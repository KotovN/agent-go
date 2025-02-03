package memory

import (
	"context"
	"sync"
	"time"
)

// BaseMemory provides a basic implementation of the Memory interface
type BaseMemory struct {
	storage   Storage
	config    *MemoryConfig
	callbacks map[string]func(MemoryNotification)
	mu        sync.RWMutex
}

// NewBaseMemory creates a new base memory instance
func NewBaseMemory(storage Storage, config *MemoryConfig) *BaseMemory {
	if config == nil {
		config = DefaultMemoryConfig()
	}
	return &BaseMemory{
		storage:   storage,
		config:    config,
		callbacks: make(map[string]func(MemoryNotification)),
	}
}

// Save stores a memory with optional metadata and expiration
func (m *BaseMemory) Save(value string, metadata map[string]interface{}, source string, scope MemoryScope, expiresAt *time.Time) error {
	if expiresAt == nil && m.config.DefaultTTL > 0 {
		t := time.Now().Add(m.config.DefaultTTL)
		expiresAt = &t
	}

	// Ensure metadata exists
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Add standard metadata fields
	now := time.Now()
	metadata[MetadataKeySource] = source
	metadata[MetadataKeyTimestamp] = now
	metadata[MetadataKeyType] = string(GeneralMemory)

	item := &StorageItem{
		Value:     value,
		Metadata:  metadata,
		Source:    source,
		Scope:     scope,
		Type:      GeneralMemory,
		ExpiresAt: expiresAt,
		Timestamp: now,
	}

	if err := m.storage.Save(context.Background(), item); err != nil {
		return err
	}

	// Notify subscribers if enabled
	if m.config.EnableNotifications {
		m.notifySubscribers(MemoryNotification{
			Source:    source,
			Value:     value,
			Metadata:  metadata,
			Scope:     scope,
			Type:      GeneralMemory,
			Timestamp: now,
		})
	}

	return nil
}

// Search queries memories based on the provided query parameters
func (m *BaseMemory) Search(query *MemoryQuery) ([]MemoryResult, error) {
	if query.MaxResults == 0 {
		query.MaxResults = m.config.MaxResults
	}
	if query.MinScore == 0 {
		query.MinScore = m.config.RelevanceThreshold
	}

	items, err := m.storage.Search(context.Background(), query.Query, query.MaxResults)
	if err != nil {
		return nil, err
	}

	var results []MemoryResult
	for _, item := range items {
		if item.Score < query.MinScore {
			continue
		}
		results = append(results, MemoryResult{
			Value:     item.Value,
			Score:     item.Score,
			Metadata:  item.Metadata,
			Scope:     item.Scope,
			Type:      query.Type,
			ExpiresAt: item.ExpiresAt,
		})
	}

	return results, nil
}

// Subscribe registers a callback for memory notifications
func (m *BaseMemory) Subscribe(target string, callback func(MemoryNotification)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks[target] = callback
	return nil
}

// Unsubscribe removes a notification callback
func (m *BaseMemory) Unsubscribe(target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.callbacks, target)
	return nil
}

// Reset clears all stored memories
func (m *BaseMemory) Reset() error {
	return m.storage.Reset(context.Background())
}

// notifySubscribers sends notifications to all subscribers
func (m *BaseMemory) notifySubscribers(notification MemoryNotification) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for target, callback := range m.callbacks {
		notification.Target = target
		callback(notification)
	}
}
