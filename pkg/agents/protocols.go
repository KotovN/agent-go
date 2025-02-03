package agents

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ProtocolType defines different communication protocols
type ProtocolType string

const (
	// ReliableDelivery ensures messages are delivered with acknowledgment
	ReliableDelivery ProtocolType = "reliable"
	// BestEffortDelivery attempts delivery without guarantees
	BestEffortDelivery ProtocolType = "best-effort"
	// OrderedDelivery ensures messages are processed in order
	OrderedDelivery ProtocolType = "ordered"
	// PriorityDelivery handles messages based on priority
	PriorityDelivery ProtocolType = "priority"
)

// ProtocolHandler manages communication protocols and error recovery
type ProtocolHandler struct {
	mu sync.RWMutex

	// Track message delivery attempts and status
	deliveryAttempts map[string]int
	messageStatus    map[string]MessageStatus

	// Protocol configurations
	maxRetries      int
	retryBackoff    time.Duration
	maxBackoff      time.Duration
	deliveryTimeout time.Duration

	// Message ordering
	messageSequence map[string]uint64     // AgentID -> Last sequence number
	pendingMessages map[string][]*Message // Messages waiting for ordered delivery
}

// MessageStatus tracks message delivery state
type MessageStatus struct {
	Attempts     int
	LastAttempt  time.Time
	NextAttempt  time.Time
	Status       string
	ErrorHistory []error
}

// NewProtocolHandler creates a new protocol handler
func NewProtocolHandler(config *ProtocolConfig) *ProtocolHandler {
	if config == nil {
		config = DefaultProtocolConfig()
	}

	return &ProtocolHandler{
		deliveryAttempts: make(map[string]int),
		messageStatus:    make(map[string]MessageStatus),
		messageSequence:  make(map[string]uint64),
		pendingMessages:  make(map[string][]*Message),
		maxRetries:       config.MaxRetries,
		retryBackoff:     config.RetryBackoff,
		maxBackoff:       config.MaxBackoff,
		deliveryTimeout:  config.DeliveryTimeout,
	}
}

// HandleMessageDelivery processes message delivery with protocol-specific handling
func (ph *ProtocolHandler) HandleMessageDelivery(ctx context.Context, msg *Message, protocol ProtocolType) error {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	// Initialize or get message status
	status, exists := ph.messageStatus[msg.ID]
	if !exists {
		status = MessageStatus{
			LastAttempt: time.Now(),
			Status:      "pending",
		}
	}

	// Check delivery attempts
	if status.Attempts >= ph.maxRetries {
		return fmt.Errorf("max delivery attempts reached for message %s", msg.ID)
	}

	// Protocol-specific handling
	var err error
	switch protocol {
	case ReliableDelivery:
		err = ph.handleReliableDelivery(ctx, msg, &status)
	case OrderedDelivery:
		err = ph.handleOrderedDelivery(ctx, msg, &status)
	case PriorityDelivery:
		err = ph.handlePriorityDelivery(ctx, msg, &status)
	default:
		err = ph.handleBestEffortDelivery(ctx, msg, &status)
	}

	// Update status
	status.Attempts++
	status.LastAttempt = time.Now()
	if err != nil {
		status.ErrorHistory = append(status.ErrorHistory, err)
		status.Status = "failed"

		// Calculate next retry attempt with exponential backoff
		backoff := ph.retryBackoff * time.Duration(1<<uint(status.Attempts))
		if backoff > ph.maxBackoff {
			backoff = ph.maxBackoff
		}
		status.NextAttempt = status.LastAttempt.Add(backoff)
	} else {
		status.Status = "delivered"
	}

	ph.messageStatus[msg.ID] = status
	return err
}

// handleReliableDelivery ensures message delivery with acknowledgment
func (ph *ProtocolHandler) handleReliableDelivery(ctx context.Context, msg *Message, status *MessageStatus) error {
	// Set acknowledgment requirement
	msg.RequiresAck = true

	// Wait for acknowledgment with timeout
	done := make(chan bool)
	go func() {
		for !msg.Acked {
			time.Sleep(100 * time.Millisecond)
			if msg.Acked {
				done <- true
				return
			}
		}
	}()

	select {
	case <-done:
		return nil
	case <-time.After(ph.deliveryTimeout):
		return fmt.Errorf("acknowledgment timeout for message %s", msg.ID)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleOrderedDelivery ensures messages are delivered in sequence
func (ph *ProtocolHandler) handleOrderedDelivery(ctx context.Context, msg *Message, status *MessageStatus) error {
	// Get current sequence for the agent
	currentSeq := ph.messageSequence[msg.FromAgent]
	expectedSeq := currentSeq + 1

	// Check if this is the next expected message
	if msg.Sequence != expectedSeq {
		// Store for later delivery
		ph.pendingMessages[msg.FromAgent] = append(ph.pendingMessages[msg.FromAgent], msg)
		return fmt.Errorf("out of order message %s, expected seq %d, got %d", msg.ID, expectedSeq, msg.Sequence)
	}

	// Update sequence and check for pending messages
	ph.messageSequence[msg.FromAgent] = expectedSeq
	return ph.deliverPendingMessages(ctx, msg.FromAgent)
}

// handlePriorityDelivery manages priority-based message delivery
func (ph *ProtocolHandler) handlePriorityDelivery(ctx context.Context, msg *Message, status *MessageStatus) error {
	// Implement priority queue handling
	if msg.Priority <= 0 {
		// Use best-effort for non-priority messages
		return ph.handleBestEffortDelivery(ctx, msg, status)
	}

	// High priority messages get immediate delivery attempt
	msg.RequiresAck = true
	return ph.handleReliableDelivery(ctx, msg, status)
}

// handleBestEffortDelivery attempts delivery without guarantees
func (ph *ProtocolHandler) handleBestEffortDelivery(ctx context.Context, msg *Message, status *MessageStatus) error {
	// Simple delivery without guarantees
	msg.RequiresAck = false
	return nil
}

// deliverPendingMessages processes any pending messages for an agent
func (ph *ProtocolHandler) deliverPendingMessages(ctx context.Context, agentID string) error {
	pending := ph.pendingMessages[agentID]
	if len(pending) == 0 {
		return nil
	}

	// Sort pending messages by sequence
	// Deliver messages in order
	var delivered []*Message
	currentSeq := ph.messageSequence[agentID]

	for _, msg := range pending {
		if msg.Sequence == currentSeq+1 {
			// Deliver this message
			currentSeq++
			delivered = append(delivered, msg)
		}
	}

	// Update sequence and remove delivered messages
	if len(delivered) > 0 {
		ph.messageSequence[agentID] = currentSeq
		// Remove delivered messages from pending
		remaining := make([]*Message, 0)
		for _, msg := range pending {
			if msg.Sequence > currentSeq {
				remaining = append(remaining, msg)
			}
		}
		ph.pendingMessages[agentID] = remaining
	}

	return nil
}

// GetMessageStatus returns the current status of a message
func (ph *ProtocolHandler) GetMessageStatus(msgID string) (MessageStatus, bool) {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	status, exists := ph.messageStatus[msgID]
	return status, exists
}

// DefaultProtocolConfig returns default protocol configuration
func DefaultProtocolConfig() *ProtocolConfig {
	return &ProtocolConfig{
		MaxRetries:      3,
		RetryBackoff:    time.Second,
		MaxBackoff:      time.Minute * 5,
		DeliveryTimeout: time.Second * 30,
	}
}

// ProtocolConfig defines configuration for protocol handler
type ProtocolConfig struct {
	MaxRetries      int
	RetryBackoff    time.Duration
	MaxBackoff      time.Duration
	DeliveryTimeout time.Duration
}
