package backend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent-go/pkg/memory/conversation"
	"agent-go/pkg/memory/vector"
)

// Backend defines the interface for memory storage
type Backend interface {
	// SaveConversation stores a conversation thread and its messages
	SaveConversation(ctx context.Context, thread *conversation.Thread, messages []conversation.Message) error

	// LoadConversation retrieves a conversation thread and its messages
	LoadConversation(ctx context.Context, threadID string) (*conversation.Thread, []conversation.Message, error)

	// SearchConversations searches for conversations matching the query
	SearchConversations(ctx context.Context, query string, limit int) ([]*conversation.Thread, error)

	// DeleteConversation deletes a conversation thread and its messages
	DeleteConversation(ctx context.Context, threadID string) error

	// SaveVector stores a vector with associated metadata
	SaveVector(ctx context.Context, id string, vec vector.Vector, metadata map[string]interface{}) error

	// LoadVector retrieves a vector by ID
	LoadVector(ctx context.Context, id string) (vector.Vector, map[string]interface{}, error)

	// SearchVectors searches for similar vectors
	SearchVectors(ctx context.Context, queryVec vector.Vector, limit int) ([]vector.SearchResult, error)

	// DeleteVector deletes a vector by ID
	DeleteVector(ctx context.Context, id string) error

	// Close closes the backend
	Close() error
}

// MemoryBackend provides an in-memory implementation of the Backend interface
type MemoryBackend struct {
	mu sync.RWMutex

	conversations map[string]*conversationData
	vectors       map[string]*vectorData

	vectorStore vector.Store
}

type conversationData struct {
	thread   *conversation.Thread
	messages []conversation.Message
	updated  time.Time
}

type vectorData struct {
	vector   vector.Vector
	metadata map[string]interface{}
	updated  time.Time
}

// NewMemoryBackend creates a new in-memory backend
func NewMemoryBackend(vectorStore vector.Store) *MemoryBackend {
	return &MemoryBackend{
		conversations: make(map[string]*conversationData),
		vectors:       make(map[string]*vectorData),
		vectorStore:   vectorStore,
	}
}

// SaveConversation implements Backend.SaveConversation
func (b *MemoryBackend) SaveConversation(ctx context.Context, thread *conversation.Thread, messages []conversation.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if thread == nil {
		return fmt.Errorf("thread cannot be nil")
	}

	b.conversations[thread.ID] = &conversationData{
		thread:   thread,
		messages: messages,
		updated:  time.Now(),
	}

	return nil
}

// LoadConversation implements Backend.LoadConversation
func (b *MemoryBackend) LoadConversation(ctx context.Context, threadID string) (*conversation.Thread, []conversation.Message, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	data, exists := b.conversations[threadID]
	if !exists {
		return nil, nil, fmt.Errorf("conversation not found: %s", threadID)
	}

	return data.thread, data.messages, nil
}

// SearchConversations implements Backend.SearchConversations
func (b *MemoryBackend) SearchConversations(ctx context.Context, query string, limit int) ([]*conversation.Thread, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Simple implementation that returns most recent conversations
	threads := make([]*conversation.Thread, 0, len(b.conversations))
	for _, data := range b.conversations {
		threads = append(threads, data.thread)
	}

	// Sort by updated time (most recent first)
	// TODO: Implement proper search using vector store

	if len(threads) > limit {
		threads = threads[:limit]
	}

	return threads, nil
}

// DeleteConversation implements Backend.DeleteConversation
func (b *MemoryBackend) DeleteConversation(ctx context.Context, threadID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.conversations, threadID)
	return nil
}

// SaveVector implements Backend.SaveVector
func (b *MemoryBackend) SaveVector(ctx context.Context, id string, vec vector.Vector, metadata map[string]interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if id == "" {
		return fmt.Errorf("vector ID cannot be empty")
	}

	b.vectors[id] = &vectorData{
		vector:   vec,
		metadata: metadata,
		updated:  time.Now(),
	}

	// Store in vector store
	doc := vector.Document{
		ID:       id,
		Vector:   vec,
		Metadata: metadata,
	}
	return b.vectorStore.Insert(ctx, doc)
}

// LoadVector implements Backend.LoadVector
func (b *MemoryBackend) LoadVector(ctx context.Context, id string) (vector.Vector, map[string]interface{}, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	data, exists := b.vectors[id]
	if !exists {
		return nil, nil, fmt.Errorf("vector not found: %s", id)
	}

	return data.vector, data.metadata, nil
}

// SearchVectors implements Backend.SearchVectors
func (b *MemoryBackend) SearchVectors(ctx context.Context, queryVec vector.Vector, limit int) ([]vector.SearchResult, error) {
	return b.vectorStore.Search(ctx, queryVec, limit)
}

// DeleteVector implements Backend.DeleteVector
func (b *MemoryBackend) DeleteVector(ctx context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.vectors, id)
	return b.vectorStore.Delete(ctx, id)
}

// Close implements Backend.Close
func (b *MemoryBackend) Close() error {
	return b.vectorStore.Close()
}
