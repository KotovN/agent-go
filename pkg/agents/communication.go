package agents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KotovN/agent-go/pkg/memory"
)

// MessageType defines the type of communication between agents
type MessageType string

const (
	// Direct message between agents
	MessageTypeDirect MessageType = "direct"
	// Broadcast message to all agents in team
	MessageTypeBroadcast MessageType = "broadcast"
	// Request for collaboration
	MessageTypeCollaboration MessageType = "collaboration"
	// Response to a collaboration request
	MessageTypeResponse MessageType = "response"
	// Feedback on task or collaboration
	MessageTypeFeedback MessageType = "feedback"
)

// Message represents a communication unit between agents
type Message struct {
	ID          string
	Type        MessageType
	FromAgent   string
	ToAgent     string // Empty for broadcast
	Content     string
	Context     map[string]interface{}
	Priority    int
	Timestamp   time.Time
	ExpiresAt   *time.Time
	RequiresAck bool
	Acked       bool
	// Sequence number for ordered delivery
	Sequence uint64
}

// CommunicationChannel manages message exchange between agents
type CommunicationChannel struct {
	mu       sync.RWMutex
	messages map[string]*Message   // Message ID -> Message
	inbox    map[string][]*Message // Agent ID -> Messages
	outbox   map[string][]*Message // Agent ID -> Messages
	memory   *memory.SharedMemory

	// Protocol handling
	protocolHandler *ProtocolHandler
	defaultProtocol ProtocolType
}

// NewCommunicationChannel creates a new communication channel
func NewCommunicationChannel(memory *memory.SharedMemory) *CommunicationChannel {
	return &CommunicationChannel{
		messages:        make(map[string]*Message),
		inbox:           make(map[string][]*Message),
		outbox:          make(map[string][]*Message),
		memory:          memory,
		protocolHandler: NewProtocolHandler(nil),
		defaultProtocol: BestEffortDelivery,
	}
}

// SendMessage sends a message from one agent to another
func (cc *CommunicationChannel) SendMessage(ctx context.Context, msg *Message) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Handle message delivery using protocol handler
	if err := cc.protocolHandler.HandleMessageDelivery(ctx, msg, cc.defaultProtocol); err != nil {
		return fmt.Errorf("protocol handler error: %w", err)
	}

	// Store message
	cc.messages[msg.ID] = msg

	// Add to recipient's inbox
	if msg.Type == MessageTypeBroadcast {
		// For broadcast, message goes to all agents
		for agentID := range cc.inbox {
			cc.inbox[agentID] = append(cc.inbox[agentID], msg)
		}
	} else {
		// For direct messages
		cc.inbox[msg.ToAgent] = append(cc.inbox[msg.ToAgent], msg)
	}

	// Add to sender's outbox
	cc.outbox[msg.FromAgent] = append(cc.outbox[msg.FromAgent], msg)

	// Store in shared memory if available
	if cc.memory != nil {
		memory := &memory.SharedMemoryContext{
			Source:    msg.FromAgent,
			Target:    []string{msg.ToAgent},
			Value:     msg.Content,
			Metadata:  msg.Context,
			Scope:     memory.AgentMemory,
			Timestamp: msg.Timestamp,
		}
		if err := cc.memory.Share(ctx, memory); err != nil {
			return fmt.Errorf("failed to store message in shared memory: %w", err)
		}
	}

	return nil
}

// SendMessageWithProtocol sends a message using a specific protocol
func (cc *CommunicationChannel) SendMessageWithProtocol(ctx context.Context, msg *Message, protocol ProtocolType) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Handle message delivery using specified protocol
	if err := cc.protocolHandler.HandleMessageDelivery(ctx, msg, protocol); err != nil {
		return fmt.Errorf("protocol handler error: %w", err)
	}

	// Store message
	cc.messages[msg.ID] = msg

	// Add to recipient's inbox
	if msg.Type == MessageTypeBroadcast {
		// For broadcast, message goes to all agents
		for agentID := range cc.inbox {
			cc.inbox[agentID] = append(cc.inbox[agentID], msg)
		}
	} else {
		// For direct messages
		cc.inbox[msg.ToAgent] = append(cc.inbox[msg.ToAgent], msg)
	}

	// Add to sender's outbox
	cc.outbox[msg.FromAgent] = append(cc.outbox[msg.FromAgent], msg)

	// Store in shared memory if available
	if cc.memory != nil {
		memory := &memory.SharedMemoryContext{
			Source:    msg.FromAgent,
			Target:    []string{msg.ToAgent},
			Value:     msg.Content,
			Metadata:  msg.Context,
			Scope:     memory.AgentMemory,
			Timestamp: msg.Timestamp,
		}
		if err := cc.memory.Share(ctx, memory); err != nil {
			return fmt.Errorf("failed to store message in shared memory: %w", err)
		}
	}

	return nil
}

// GetMessages retrieves messages for an agent
func (cc *CommunicationChannel) GetMessages(agentID string, unreadOnly bool) []*Message {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	messages := cc.inbox[agentID]
	if !unreadOnly {
		return messages
	}

	// Filter unread messages
	var unread []*Message
	for _, msg := range messages {
		if msg.RequiresAck && !msg.Acked {
			unread = append(unread, msg)
		}
	}
	return unread
}

// AcknowledgeMessage marks a message as read
func (cc *CommunicationChannel) AcknowledgeMessage(msgID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	msg, exists := cc.messages[msgID]
	if !exists {
		return fmt.Errorf("message %s not found", msgID)
	}

	msg.Acked = true
	return nil
}

// ClearExpiredMessages removes expired messages
func (cc *CommunicationChannel) ClearExpiredMessages() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()

	// Helper function to filter unexpired messages
	filterUnexpired := func(messages []*Message) []*Message {
		var unexpired []*Message
		for _, msg := range messages {
			if msg.ExpiresAt == nil || msg.ExpiresAt.After(now) {
				unexpired = append(unexpired, msg)
			}
		}
		return unexpired
	}

	// Clear expired messages from all collections
	for agentID := range cc.inbox {
		cc.inbox[agentID] = filterUnexpired(cc.inbox[agentID])
	}
	for agentID := range cc.outbox {
		cc.outbox[agentID] = filterUnexpired(cc.outbox[agentID])
	}

	// Clear from messages map
	for id, msg := range cc.messages {
		if msg.ExpiresAt != nil && msg.ExpiresAt.Before(now) {
			delete(cc.messages, id)
		}
	}
}

// GetMessageStatus returns the status of a message
func (cc *CommunicationChannel) GetMessageStatus(msgID string) (MessageStatus, bool) {
	return cc.protocolHandler.GetMessageStatus(msgID)
}

// SetDefaultProtocol sets the default protocol for message delivery
func (cc *CommunicationChannel) SetDefaultProtocol(protocol ProtocolType) {
	cc.defaultProtocol = protocol
}
