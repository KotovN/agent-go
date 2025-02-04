package conversation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KotovN/agent-go/pkg/embeddings"
	"github.com/KotovN/agent-go/pkg/memory/vector"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleFunction  MessageRole = "function"
)

// Message represents a conversation message
type Message struct {
	ID        string                 `json:"id"`
	Role      MessageRole            `json:"role"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	ThreadID  string                 `json:"thread_id"`
	ParentID  string                 `json:"parent_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Thread represents a conversation thread
type Thread struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Summary      string    `json:"summary,omitempty"`
	MessageCount int       `json:"message_count"`
}

// History manages conversation history with vector storage
type History struct {
	mu          sync.RWMutex
	vectorStore vector.Store
	threads     map[string]*Thread
	summarizer  Summarizer
	embedder    embeddings.Model
}

// Config holds configuration for conversation history
type Config struct {
	VectorStore vector.Store
	Summarizer  Summarizer
	Embedder    embeddings.Model
}

// NewHistory creates a new conversation history manager
func NewHistory(config Config) (*History, error) {
	if config.VectorStore == nil {
		return nil, fmt.Errorf("vector store is required")
	}

	if config.Summarizer == nil {
		config.Summarizer = NewDefaultSummarizer()
	}

	if config.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}

	return &History{
		vectorStore: config.VectorStore,
		threads:     make(map[string]*Thread),
		summarizer:  config.Summarizer,
		embedder:    config.Embedder,
	}, nil
}

// AddMessage adds a message to the conversation history
func (h *History) AddMessage(ctx context.Context, msg Message) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Validate message
	if msg.ID == "" || msg.ThreadID == "" {
		return fmt.Errorf("message ID and thread ID are required")
	}

	// Get or create thread
	thread, exists := h.threads[msg.ThreadID]
	if !exists {
		thread = &Thread{
			ID:        msg.ThreadID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		h.threads[msg.ThreadID] = thread
	}

	// Update thread
	thread.MessageCount++
	thread.UpdatedAt = time.Now()

	// Generate embedding for message content
	embeddings, err := h.embedder.Embed(ctx, []string{msg.Content})
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Store message in vector store
	doc := vector.Document{
		ID:      msg.ID,
		Content: msg.Content,
		Vector:  embeddings[0],
		Metadata: map[string]interface{}{
			"role":      msg.Role,
			"thread_id": msg.ThreadID,
			"parent_id": msg.ParentID,
			"timestamp": msg.Timestamp,
		},
	}

	if err := h.vectorStore.Insert(ctx, doc); err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	// Update thread summary if needed
	if thread.MessageCount%10 == 0 { // Update summary every 10 messages
		if err := h.updateThreadSummary(ctx, thread.ID); err != nil {
			return fmt.Errorf("failed to update thread summary: %w", err)
		}
	}

	return nil
}

// GetThread retrieves a conversation thread
func (h *History) GetThread(threadID string) (*Thread, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	thread, exists := h.threads[threadID]
	if !exists {
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}

	return thread, nil
}

// GetThreadMessages retrieves messages from a thread
func (h *History) GetThreadMessages(ctx context.Context, threadID string, limit int) ([]Message, error) {
	// Get thread messages by ID
	docs, err := h.vectorStore.Get(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]Message, 0, len(docs))
	for _, doc := range docs {
		msg := Message{
			ID:      doc.ID,
			Content: doc.Content,
		}

		// Extract metadata
		if doc.Metadata != nil {
			if role, ok := doc.Metadata["role"].(string); ok {
				msg.Role = MessageRole(role)
			}
			if threadID, ok := doc.Metadata["thread_id"].(string); ok {
				msg.ThreadID = threadID
			}
			if parentID, ok := doc.Metadata["parent_id"].(string); ok {
				msg.ParentID = parentID
			}
			if timestamp, ok := doc.Metadata["timestamp"].(time.Time); ok {
				msg.Timestamp = timestamp
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// SearchMessages searches for messages across all threads
func (h *History) SearchMessages(ctx context.Context, query string, limit int) ([]Message, error) {
	// Generate embedding for query
	embeddings, err := h.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search vector store
	results, err := h.vectorStore.Search(ctx, embeddings[0], limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	messages := make([]Message, 0, len(results))
	for _, result := range results {
		msg := Message{
			ID:      result.Document.ID,
			Content: result.Document.Content,
		}

		// Extract metadata
		if result.Document.Metadata != nil {
			if role, ok := result.Document.Metadata["role"].(string); ok {
				msg.Role = MessageRole(role)
			}
			if threadID, ok := result.Document.Metadata["thread_id"].(string); ok {
				msg.ThreadID = threadID
			}
			if parentID, ok := result.Document.Metadata["parent_id"].(string); ok {
				msg.ParentID = parentID
			}
			if timestamp, ok := result.Document.Metadata["timestamp"].(time.Time); ok {
				msg.Timestamp = timestamp
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// DeleteThread deletes a thread and all its messages
func (h *History) DeleteThread(ctx context.Context, threadID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get thread messages
	messages, err := h.GetThreadMessages(ctx, threadID, -1)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// Delete messages from vector store
	var ids []string
	for _, msg := range messages {
		ids = append(ids, msg.ID)
	}

	if err := h.vectorStore.Delete(ctx, ids...); err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// Delete thread
	delete(h.threads, threadID)
	return nil
}

func (h *History) updateThreadSummary(ctx context.Context, threadID string) error {
	// Get recent messages
	messages, err := h.GetThreadMessages(ctx, threadID, 10)
	if err != nil {
		return err
	}

	// Generate summary
	summary, err := h.summarizer.Summarize(ctx, messages)
	if err != nil {
		return err
	}

	// Update thread
	thread, exists := h.threads[threadID]
	if !exists {
		return fmt.Errorf("thread not found: %s", threadID)
	}

	thread.Summary = summary
	return nil
}

// Close closes the history manager
func (h *History) Close() error {
	var errs []error

	if err := h.vectorStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close vector store: %w", err))
	}

	if err := h.embedder.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close embedder: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing history: %v", errs)
	}

	return nil
}
