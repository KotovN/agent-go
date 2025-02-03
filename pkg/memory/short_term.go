package memory

import (
	"context"
	"time"
)

// ShortTermMemoryItem represents an item in short-term memory
type ShortTermMemoryItem struct {
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata"`
	Agent     string                 `json:"agent,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ShortTermMemory represents a memory component for transient data
type ShortTermMemory struct {
	*BaseMemory
}

// NewShortTermMemory creates a new short-term memory instance
func NewShortTermMemory(storage Storage, config *MemoryConfig) *ShortTermMemory {
	return &ShortTermMemory{
		BaseMemory: NewBaseMemory(storage, config),
	}
}

// Save stores a value in short-term memory
func (m *ShortTermMemory) Save(value string, metadata map[string]interface{}, source string, scope MemoryScope, expiresAt *time.Time) error {
	return m.BaseMemory.Save(value, metadata, source, scope, expiresAt)
}

// Search performs a search over stored items
func (m *ShortTermMemory) Search(query *MemoryQuery) ([]MemoryResult, error) {
	items, err := m.storage.Search(context.Background(), query.Query, query.MaxResults)
	if err != nil {
		return nil, err
	}

	var results []MemoryResult
	for _, item := range items {
		results = append(results, MemoryResult{
			Value:     item.Value,
			Score:     item.Score,
			Metadata:  item.Metadata,
			Scope:     item.Scope,
			Type:      item.Type,
			ExpiresAt: item.ExpiresAt,
		})
	}

	return results, nil
}

// Reset clears all stored items
func (m *ShortTermMemory) Reset(ctx context.Context) error {
	return m.storage.Reset(ctx)
}
