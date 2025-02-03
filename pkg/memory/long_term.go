// This file is intentionally empty as SQLiteStorage is now defined in sqlite_storage.go

package memory

import (
	"context"
	"time"
)

// LongTermMemoryItem represents an item in long-term memory
type LongTermMemoryItem struct {
	ID        int64                  `json:"id"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata"`
	Agent     string                 `json:"agent,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// LongTermMemory manages persistent data and insights
type LongTermMemory struct {
	*BaseMemory
}

// NewLongTermMemory creates a new long-term memory instance
func NewLongTermMemory(storage Storage, config *MemoryConfig) *LongTermMemory {
	return &LongTermMemory{
		BaseMemory: NewBaseMemory(storage, config),
	}
}

// Save stores a value in long-term memory
func (m *LongTermMemory) Save(value string, metadata map[string]interface{}, source string, scope MemoryScope, expiresAt *time.Time) error {
	return m.BaseMemory.Save(value, metadata, source, scope, expiresAt)
}

// Search performs a search over stored items
func (m *LongTermMemory) Search(query *MemoryQuery) ([]MemoryResult, error) {
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
func (m *LongTermMemory) Reset(ctx context.Context) error {
	return m.storage.Reset(ctx)
}
