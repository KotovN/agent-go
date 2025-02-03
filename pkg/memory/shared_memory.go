package memory

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SharedMemory manages memory sharing between agents
type SharedMemory struct {
	storage     Storage
	config      *MemoryConfig
	permissions map[string]*MemoryPermission // agent ID -> permissions
	mu          sync.RWMutex
}

// NewSharedMemory creates a new shared memory instance
func NewSharedMemory(storage Storage, config *MemoryConfig) *SharedMemory {
	if config == nil {
		config = DefaultMemoryConfig()
	}
	return &SharedMemory{
		storage:     storage,
		config:      config,
		permissions: make(map[string]*MemoryPermission),
	}
}

// Share stores a memory that can be accessed by other agents
func (m *SharedMemory) Share(ctx context.Context, memory *SharedMemoryContext) error {
	// Validate memory context
	if err := m.validateMemoryContext(memory); err != nil {
		return fmt.Errorf("invalid memory context: %w", err)
	}

	// Set timestamp if not set
	if memory.Timestamp.IsZero() {
		memory.Timestamp = time.Now()
	}

	// Create storage item
	item := &StorageItem{
		Value:     fmt.Sprintf("%v", memory.Value), // Convert value to string
		Metadata:  memory.Metadata,
		Source:    memory.Source,
		Scope:     TaskScope,    // Default to task scope
		Type:      memory.Scope, // Use the memory's scope as type
		Timestamp: memory.Timestamp,
	}

	// Add target to metadata
	if item.Metadata == nil {
		item.Metadata = make(map[string]interface{})
	}
	item.Metadata["target"] = memory.Target
	item.Metadata["shared"] = true

	return m.storage.Save(ctx, item)
}

// Access retrieves shared memories accessible to the given agent
func (m *SharedMemory) Access(ctx context.Context, agentID string, query string, memoryType MemoryType) ([]*SharedMemoryContext, error) {
	// Check agent permissions
	perm, err := m.getAgentPermissions(agentID)
	if err != nil {
		return nil, err
	}

	// Search storage with query
	memQuery := &MemoryQuery{
		Query:      query,
		Type:       memoryType,
		MaxResults: m.config.MaxResults,
		Metadata: map[string]interface{}{
			"shared": true,
		},
	}

	items, err := m.storage.Search(ctx, memQuery.Query, memQuery.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to search shared memories: %w", err)
	}

	// Process and filter results
	memories := make([]*SharedMemoryContext, 0)
	for _, item := range items {
		// Skip non-shared memories
		shared, ok := item.Metadata["shared"].(bool)
		if !ok || !shared {
			continue
		}

		// Create memory context
		memory := &SharedMemoryContext{
			Source:    item.Source,
			Value:     item.Value,
			Metadata:  item.Metadata,
			Scope:     item.Type, // Use type as scope
			Timestamp: item.Timestamp,
		}

		// Get target list from metadata
		if targets, ok := item.Metadata["target"].([]string); ok {
			memory.Target = targets
		}

		// Check if agent has access
		if m.canAccess(agentID, memory, perm) {
			memories = append(memories, memory)
		}
	}

	return memories, nil
}

// SetPermissions sets memory access permissions for an agent
func (m *SharedMemory) SetPermissions(agentID string, permissions *MemoryPermission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permissions[agentID] = permissions
}

// getAgentPermissions retrieves an agent's memory permissions
func (m *SharedMemory) getAgentPermissions(agentID string) (*MemoryPermission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perm, exists := m.permissions[agentID]
	if !exists {
		return nil, fmt.Errorf("no permissions set for agent: %s", agentID)
	}
	return perm, nil
}

// validateMemoryContext validates a shared memory context
func (m *SharedMemory) validateMemoryContext(memory *SharedMemoryContext) error {
	if memory == nil {
		return fmt.Errorf("memory context cannot be nil")
	}
	if memory.Source == "" {
		return fmt.Errorf("memory source cannot be empty")
	}
	if memory.Value == nil {
		return fmt.Errorf("memory value cannot be nil")
	}
	return nil
}

// canAccess checks if an agent can access a shared memory
func (m *SharedMemory) canAccess(agentID string, memory *SharedMemoryContext, perm *MemoryPermission) bool {
	// Source agent always has access
	if memory.Source == agentID {
		return true
	}

	// Check scope
	switch perm.Scope {
	case TaskScope:
		return false
	case ProcessScope:
		// Check if agent is in target list
		if len(memory.Target) > 0 {
			for _, target := range memory.Target {
				if target == agentID {
					return true
				}
			}
			return false
		}
		return true // If no targets specified, treat as global
	case GlobalScope:
		return true
	}

	return false
}

// UpdateRelevance updates the relevance score of a shared memory
func (m *SharedMemory) UpdateRelevance(ctx context.Context, memory *SharedMemoryContext, relevance float64) error {
	if relevance < 0 || relevance > 1 {
		return fmt.Errorf("relevance score must be between 0 and 1")
	}

	memory.Metadata["relevance"] = relevance
	memory.Timestamp = time.Now() // Update timestamp to reflect the change

	// Create storage item
	item := &StorageItem{
		Value:     fmt.Sprintf("%v", memory.Value), // Convert value to string
		Metadata:  memory.Metadata,
		Source:    memory.Source,
		Scope:     TaskScope,    // Default to task scope
		Type:      memory.Scope, // Use the memory's scope as type
		Score:     relevance,
		Timestamp: memory.Timestamp,
	}

	// Add target to metadata
	if item.Metadata == nil {
		item.Metadata = make(map[string]interface{})
	}
	item.Metadata["target"] = memory.Target
	item.Metadata["shared"] = true

	return m.storage.Save(ctx, item)
}
