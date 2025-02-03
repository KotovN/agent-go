package tasks

import (
	"fmt"
	"sync"
	"time"
)

// TaskMessage represents a message in task communication
type TaskMessage struct {
	Type      MessageType
	TaskID    string
	Content   interface{}
	Timestamp time.Time
}

// MessageType represents different types of task messages
type MessageType int

const (
	// MessageTypeResult represents a task result
	MessageTypeResult MessageType = iota
	// MessageTypeProgress represents task progress update
	MessageTypeProgress
	// MessageTypeLog represents a log message
	MessageTypeLog
	// MessageTypeError represents an error message
	MessageTypeError
	// MessageTypeStatus represents a status update
	MessageTypeStatus
)

// TaskChannels manages communication channels for tasks
type TaskChannels struct {
	mu sync.RWMutex

	// Channels for different message types
	resultChannels   map[string]chan AsyncResult
	progressChannels map[string]chan float64
	logChannels      map[string]chan string
	statusChannels   map[string]chan TaskStatus
	messageChannels  map[string]chan TaskMessage

	// Channel subscriptions
	subscribers map[string][]chan TaskMessage
}

// NewTaskChannels creates a new task channels manager
func NewTaskChannels() *TaskChannels {
	return &TaskChannels{
		resultChannels:   make(map[string]chan AsyncResult),
		progressChannels: make(map[string]chan float64),
		logChannels:      make(map[string]chan string),
		statusChannels:   make(map[string]chan TaskStatus),
		messageChannels:  make(map[string]chan TaskMessage),
		subscribers:      make(map[string][]chan TaskMessage),
	}
}

// CreateChannels creates all necessary channels for a task
func (tc *TaskChannels) CreateChannels(taskID string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.resultChannels[taskID] = make(chan AsyncResult, 1)
	tc.progressChannels[taskID] = make(chan float64, 10)
	tc.logChannels[taskID] = make(chan string, 100)
	tc.statusChannels[taskID] = make(chan TaskStatus, 1)
	tc.messageChannels[taskID] = make(chan TaskMessage, 100)
}

// CloseChannels closes all channels for a task
func (tc *TaskChannels) CloseChannels(taskID string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if ch, exists := tc.resultChannels[taskID]; exists {
		close(ch)
		delete(tc.resultChannels, taskID)
	}
	if ch, exists := tc.progressChannels[taskID]; exists {
		close(ch)
		delete(tc.progressChannels, taskID)
	}
	if ch, exists := tc.logChannels[taskID]; exists {
		close(ch)
		delete(tc.logChannels, taskID)
	}
	if ch, exists := tc.statusChannels[taskID]; exists {
		close(ch)
		delete(tc.statusChannels, taskID)
	}
	if ch, exists := tc.messageChannels[taskID]; exists {
		close(ch)
		delete(tc.messageChannels, taskID)
	}

	// Close and remove subscriber channels
	if subs, exists := tc.subscribers[taskID]; exists {
		for _, ch := range subs {
			close(ch)
		}
		delete(tc.subscribers, taskID)
	}
}

// SendResult sends a task result
func (tc *TaskChannels) SendResult(taskID string, result AsyncResult) error {
	tc.mu.RLock()
	ch, exists := tc.resultChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no result channel found for task %s", taskID)
	}

	ch <- result
	tc.broadcast(taskID, TaskMessage{
		Type:      MessageTypeResult,
		TaskID:    taskID,
		Content:   result,
		Timestamp: time.Now(),
	})
	return nil
}

// SendProgress sends a progress update
func (tc *TaskChannels) SendProgress(taskID string, progress float64) error {
	tc.mu.RLock()
	ch, exists := tc.progressChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no progress channel found for task %s", taskID)
	}

	ch <- progress
	tc.broadcast(taskID, TaskMessage{
		Type:      MessageTypeProgress,
		TaskID:    taskID,
		Content:   progress,
		Timestamp: time.Now(),
	})
	return nil
}

// SendLog sends a log message
func (tc *TaskChannels) SendLog(taskID string, message string) error {
	tc.mu.RLock()
	ch, exists := tc.logChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no log channel found for task %s", taskID)
	}

	ch <- message
	tc.broadcast(taskID, TaskMessage{
		Type:      MessageTypeLog,
		TaskID:    taskID,
		Content:   message,
		Timestamp: time.Now(),
	})
	return nil
}

// SendStatus sends a status update
func (tc *TaskChannels) SendStatus(taskID string, status TaskStatus) error {
	tc.mu.RLock()
	ch, exists := tc.statusChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no status channel found for task %s", taskID)
	}

	ch <- status
	tc.broadcast(taskID, TaskMessage{
		Type:      MessageTypeStatus,
		TaskID:    taskID,
		Content:   status,
		Timestamp: time.Now(),
	})
	return nil
}

// Subscribe creates a new subscription for task messages
func (tc *TaskChannels) Subscribe(taskID string) (<-chan TaskMessage, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	ch := make(chan TaskMessage, 100)
	if _, exists := tc.subscribers[taskID]; !exists {
		tc.subscribers[taskID] = make([]chan TaskMessage, 0)
	}
	tc.subscribers[taskID] = append(tc.subscribers[taskID], ch)
	return ch, nil
}

// Unsubscribe removes a subscription
func (tc *TaskChannels) Unsubscribe(taskID string, ch <-chan TaskMessage) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if subs, exists := tc.subscribers[taskID]; exists {
		for i, sub := range subs {
			if sub == ch {
				close(sub)
				tc.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// broadcast sends a message to all subscribers
func (tc *TaskChannels) broadcast(taskID string, msg TaskMessage) {
	tc.mu.RLock()
	subs := tc.subscribers[taskID]
	tc.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
			// Skip if channel is full
		}
	}
}

// GetResultChannel returns the result channel for a task
func (tc *TaskChannels) GetResultChannel(taskID string) (<-chan AsyncResult, error) {
	tc.mu.RLock()
	ch, exists := tc.resultChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no result channel found for task %s", taskID)
	}
	return ch, nil
}

// GetProgressChannel returns the progress channel for a task
func (tc *TaskChannels) GetProgressChannel(taskID string) (<-chan float64, error) {
	tc.mu.RLock()
	ch, exists := tc.progressChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no progress channel found for task %s", taskID)
	}
	return ch, nil
}

// GetLogChannel returns the log channel for a task
func (tc *TaskChannels) GetLogChannel(taskID string) (<-chan string, error) {
	tc.mu.RLock()
	ch, exists := tc.logChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no log channel found for task %s", taskID)
	}
	return ch, nil
}

// GetStatusChannel returns the status channel for a task
func (tc *TaskChannels) GetStatusChannel(taskID string) (<-chan TaskStatus, error) {
	tc.mu.RLock()
	ch, exists := tc.statusChannels[taskID]
	tc.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no status channel found for task %s", taskID)
	}
	return ch, nil
}
