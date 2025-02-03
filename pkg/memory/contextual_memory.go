package memory

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// ContextualMemoryItem represents a combined memory item
type ContextualMemoryItem struct {
	Data      interface{}            `json:"data"`
	Source    string                 `json:"source"` // short_term, long_term, entity
	Metadata  map[string]interface{} `json:"metadata"`
	Agent     string                 `json:"agent,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Relevance float64                `json:"relevance,omitempty"`
}

// ContextualMemory provides context-aware memory access
type ContextualMemory struct {
	shortTerm  *ShortTermMemory
	longTerm   *LongTermMemory
	entity     EntityMemory
	shared     *SharedMemory
	maxResults int
}

// NewContextualMemory creates a new contextual memory instance
func NewContextualMemory(stm *ShortTermMemory, ltm *LongTermMemory, em EntityMemory) *ContextualMemory {
	// Create shared memory with same storage as short-term memory
	shared := NewSharedMemory(stm.BaseMemory.storage, stm.BaseMemory.config)

	return &ContextualMemory{
		shortTerm:  stm,
		longTerm:   ltm,
		entity:     em,
		shared:     shared,
		maxResults: 5,
	}
}

// GetTaskContext retrieves context relevant to a specific task
func (m *ContextualMemory) GetTaskContext(ctx context.Context, taskID string, query string) (map[string]interface{}, error) {
	return m.GetRelevantContext(ctx, &MemoryQuery{
		Query:      query,
		Type:       TaskMemory,
		TaskID:     taskID,
		MaxResults: m.maxResults,
	})
}

// GetAgentContext retrieves context relevant to a specific agent
func (m *ContextualMemory) GetAgentContext(ctx context.Context, agentName string, query string) (map[string]interface{}, error) {
	return m.GetRelevantContext(ctx, &MemoryQuery{
		Query:      query,
		Type:       AgentMemory,
		AgentName:  agentName,
		MaxResults: m.maxResults,
	})
}

// GetProcessContext retrieves context relevant to a specific process type
func (m *ContextualMemory) GetProcessContext(ctx context.Context, processType string, query string) (map[string]interface{}, error) {
	return m.GetRelevantContext(ctx, &MemoryQuery{
		Query:       query,
		Type:        ProcessMemory,
		ProcessType: processType,
		MaxResults:  m.maxResults,
	})
}

// GetRelevantContext retrieves context based on a structured query
func (m *ContextualMemory) GetRelevantContext(ctx context.Context, query *MemoryQuery) (map[string]interface{}, error) {
	var allResults []ContextualMemoryItem

	// Build metadata filter
	metadata := query.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	// Add type-specific filters
	switch query.Type {
	case TaskMemory:
		metadata["task_id"] = query.TaskID
	case AgentMemory:
		metadata["agent"] = query.AgentName
	case ProcessMemory:
		metadata["process_type"] = query.ProcessType
	}

	// Query short-term memory
	if m.shortTerm != nil {
		results, err := m.shortTerm.Search(&MemoryQuery{
			Query:      query.Query,
			Type:       query.Type,
			MaxResults: query.MaxResults,
			Metadata:   metadata,
		})
		if err != nil {
			return nil, fmt.Errorf("short-term memory query failed: %w", err)
		}
		for _, result := range results {
			allResults = append(allResults, ContextualMemoryItem{
				Data:      result.Value,
				Source:    "short_term",
				Metadata:  result.Metadata,
				Agent:     result.Metadata["agent"].(string),
				Timestamp: time.Now(),
			})
		}
	}

	// Query long-term memory
	if m.longTerm != nil {
		results, err := m.longTerm.Search(&MemoryQuery{
			Query:      query.Query,
			Type:       query.Type,
			MaxResults: query.MaxResults,
			Metadata:   metadata,
		})
		if err != nil {
			return nil, fmt.Errorf("long-term memory query failed: %w", err)
		}
		for _, result := range results {
			allResults = append(allResults, ContextualMemoryItem{
				Data:      result.Value,
				Source:    "long_term",
				Metadata:  result.Metadata,
				Agent:     result.Metadata["agent"].(string),
				Timestamp: time.Now(),
			})
		}
	}

	// Query entity memory if relevant
	if m.entity != nil && (query.Type == TaskMemory || query.Type == AgentMemory) {
		entities, err := m.entity.SearchEntities(ctx, query.Query)
		if err != nil {
			return nil, fmt.Errorf("entity memory query failed: %w", err)
		}
		for _, entity := range entities {
			allResults = append(allResults, ContextualMemoryItem{
				Data: map[string]interface{}{
					"name":        entity.Name,
					"type":        entity.Type,
					"description": entity.Description,
				},
				Source:    "entity",
				Metadata:  entity.Metadata,
				Agent:     entity.Agent,
				Timestamp: entity.Timestamp,
			})
		}
	}

	// Sort results by timestamp (most recent first)
	sortMemoryItems(allResults)

	// Build final result map
	results := map[string]interface{}{
		"items":     allResults,
		"query":     query.Query,
		"type":      query.Type,
		"timestamp": time.Now(),
		"total":     len(allResults),
	}

	return results, nil
}

// SaveWithContext stores information with contextual metadata
func (m *ContextualMemory) SaveWithContext(ctx context.Context, value interface{}, query *MemoryQuery) error {
	metadata := query.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	metadata["memory_type"] = string(query.Type)
	switch query.Type {
	case TaskMemory:
		metadata["task_id"] = query.TaskID
	case AgentMemory:
		metadata["agent"] = query.AgentName
	case ProcessMemory:
		metadata["process_type"] = query.ProcessType
	}

	// Convert value to string
	valueStr := fmt.Sprintf("%v", value)

	// Store in both short-term and long-term memory
	if m.shortTerm != nil {
		if err := m.shortTerm.Save(valueStr, metadata, metadata["agent"].(string), TaskScope, nil); err != nil {
			return fmt.Errorf("failed to save to short-term memory: %w", err)
		}
	}

	if m.longTerm != nil {
		if err := m.longTerm.Save(valueStr, metadata, metadata["agent"].(string), TaskScope, nil); err != nil {
			return fmt.Errorf("failed to save to long-term memory: %w", err)
		}
	}

	return nil
}

// Reset clears all memory components
func (m *ContextualMemory) Reset(ctx context.Context) error {
	if m.shortTerm != nil {
		if err := m.shortTerm.storage.Reset(ctx); err != nil {
			return fmt.Errorf("failed to reset short-term memory: %w", err)
		}
	}

	if m.longTerm != nil {
		if err := m.longTerm.storage.Reset(ctx); err != nil {
			return fmt.Errorf("failed to reset long-term memory: %w", err)
		}
	}

	return nil
}

// sortMemoryItems sorts memory items by timestamp (most recent first)
func sortMemoryItems(items []ContextualMemoryItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp.After(items[j].Timestamp)
	})
}

// GetSharedMemories retrieves shared memories accessible to an agent
func (m *ContextualMemory) GetSharedMemories(ctx context.Context, agentID string, query string) ([]*SharedMemoryContext, error) {
	if m.shared == nil {
		return nil, nil
	}
	return m.shared.Access(ctx, agentID, query, TaskMemory)
}

// ShareMemory shares a memory context with other agents
func (m *ContextualMemory) ShareMemory(ctx context.Context, source string, value interface{}, targets []string, metadata map[string]interface{}) error {
	if m.shared == nil {
		return nil
	}

	memory := &SharedMemoryContext{
		Source:    source,
		Target:    targets,
		Value:     value,
		Metadata:  metadata,
		Scope:     TaskMemory,
		Timestamp: time.Now(),
	}

	return m.shared.Share(ctx, memory)
}
