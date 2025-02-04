package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/KotovN/agent-go/pkg/memory/backend"
	"github.com/KotovN/agent-go/pkg/memory/conversation"
	"github.com/KotovN/agent-go/pkg/memory/vector"
)

// Manager coordinates memory components and provides a high-level interface
type Manager struct {
	mu sync.RWMutex

	backend     backend.Backend
	history     *conversation.History
	vectorStore vector.Store

	config Config
}

// Config holds configuration for the memory manager
type Config struct {
	Backend     backend.Backend
	VectorStore vector.Store
	History     *conversation.History
}

// NewManager creates a new memory manager
func NewManager(config Config) (*Manager, error) {
	if config.Backend == nil {
		return nil, fmt.Errorf("backend is required")
	}

	if config.VectorStore == nil {
		return nil, fmt.Errorf("vector store is required")
	}

	if config.History == nil {
		history, err := conversation.NewHistory(conversation.Config{
			VectorStore: config.VectorStore,
			Summarizer:  conversation.NewDefaultSummarizer(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create history: %w", err)
		}
		config.History = history
	}

	return &Manager{
		backend:     config.Backend,
		history:     config.History,
		vectorStore: config.VectorStore,
		config:      config,
	}, nil
}

// AddMessage adds a message to conversation history
func (m *Manager) AddMessage(ctx context.Context, msg conversation.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to history
	if err := m.history.AddMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to add message to history: %w", err)
	}

	// Get thread
	thread, err := m.history.GetThread(msg.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// Get thread messages
	messages, err := m.history.GetThreadMessages(ctx, msg.ThreadID, 100)
	if err != nil {
		return fmt.Errorf("failed to get thread messages: %w", err)
	}

	// Save to backend
	if err := m.backend.SaveConversation(ctx, thread, messages); err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	return nil
}

// GetThread retrieves a conversation thread
func (m *Manager) GetThread(threadID string) (*conversation.Thread, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.history.GetThread(threadID)
}

// GetThreadMessages retrieves messages from a thread
func (m *Manager) GetThreadMessages(ctx context.Context, threadID string, limit int) ([]conversation.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.history.GetThreadMessages(ctx, threadID, limit)
}

// SearchConversations searches for conversations matching the query
func (m *Manager) SearchConversations(ctx context.Context, query string, limit int) ([]*conversation.Thread, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.backend.SearchConversations(ctx, query, limit)
}

// DeleteThread deletes a conversation thread
func (m *Manager) DeleteThread(ctx context.Context, threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Delete from backend
	if err := m.backend.DeleteConversation(ctx, threadID); err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// StoreVector stores a vector with metadata
func (m *Manager) StoreVector(ctx context.Context, id string, vec vector.Vector, metadata map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backend.SaveVector(ctx, id, vec, metadata)
}

// LoadVector retrieves a vector by ID
func (m *Manager) LoadVector(ctx context.Context, id string) (vector.Vector, map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.backend.LoadVector(ctx, id)
}

// SearchVectors searches for similar vectors
func (m *Manager) SearchVectors(ctx context.Context, queryVec vector.Vector, limit int) ([]vector.SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.backend.SearchVectors(ctx, queryVec, limit)
}

// DeleteVector deletes a vector by ID
func (m *Manager) DeleteVector(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backend.DeleteVector(ctx, id)
}

// Close closes the memory manager and its components
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if err := m.history.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close history: %w", err))
	}

	if err := m.vectorStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close vector store: %w", err))
	}

	if err := m.backend.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close backend: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing memory manager: %v", errs)
	}

	return nil
}
