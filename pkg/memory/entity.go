package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-go/pkg/utils"
)

// EntityMemoryStruct represents a memory component for storing entity information
type EntityMemoryStruct struct {
	*BaseMemory
	logger *utils.Logger
}

// NewEntityMemoryStruct creates a new entity memory instance
func NewEntityMemoryStruct(storage Storage, config *MemoryConfig) *EntityMemoryStruct {
	return &EntityMemoryStruct{
		BaseMemory: NewBaseMemory(storage, config),
		logger:     utils.NewLogger(false),
	}
}

// Save stores a value in entity memory
func (m *EntityMemoryStruct) Save(value string, metadata map[string]interface{}, source string, scope MemoryScope, expiresAt *time.Time) error {
	return m.BaseMemory.Save(value, metadata, source, scope, expiresAt)
}

// Search performs a search over stored items
func (m *EntityMemoryStruct) Search(query *MemoryQuery) ([]MemoryResult, error) {
	// Add entity-specific metadata filters
	if query.Metadata == nil {
		query.Metadata = make(map[string]interface{})
	}
	query.Metadata[MetadataKeyType] = "entity"

	// If entity type is specified in metadata, add it to filter
	if entityType, ok := query.Metadata["entity_type"].(string); ok {
		query.Metadata["entity_type"] = entityType
	}

	// If entity name is specified in metadata, add it to filter
	if entityName, ok := query.Metadata["entity_name"].(string); ok {
		query.Metadata["entity_name"] = entityName
	}

	items, err := m.storage.Search(context.Background(), query.Query, query.MaxResults)
	if err != nil {
		return nil, err
	}

	var results []MemoryResult
	for _, item := range items {
		// Skip non-entity items
		if itemType, ok := item.Metadata[MetadataKeyType].(string); !ok || itemType != "entity" {
			continue
		}

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

// SaveEntity stores an entity in memory
func (m *EntityMemoryStruct) SaveEntity(ctx context.Context, item EntityMemoryItem) error {
	// Convert item to JSON for storage
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	// Create standard metadata
	metadata := map[string]interface{}{
		MetadataKeyType:      "entity",
		MetadataKeySource:    item.Agent,
		MetadataKeyTimestamp: item.Timestamp,
		"entity_type":        string(item.Type),
		"entity_name":        item.Name,
	}

	// Add entity properties and custom metadata
	for k, v := range item.Properties {
		metadata[k] = v
	}
	for k, v := range item.Metadata {
		metadata[k] = v
	}

	// Determine appropriate scope based on entity type
	scope := TaskScope
	if item.Type == EntityTypePerson || item.Type == EntityTypeOrg {
		scope = GlobalScope // Entities representing people or organizations are globally relevant
	}

	return m.Save(string(data), metadata, item.Agent, scope, nil)
}

// SearchEntities performs a search over stored entities
func (m *EntityMemoryStruct) SearchEntities(ctx context.Context, query string) ([]EntityMemoryItem, error) {
	memQuery := &MemoryQuery{
		Query:      query,
		Type:       GeneralMemory,
		MaxResults: m.config.MaxResults,
		Metadata: map[string]interface{}{
			MetadataKeyType: "entity",
		},
	}

	items, err := m.Search(memQuery)
	if err != nil {
		return nil, err
	}

	var entities []EntityMemoryItem
	for _, item := range items {
		var entity EntityMemoryItem
		if err := json.Unmarshal([]byte(item.Value), &entity); err != nil {
			// Log error but continue processing other items
			m.logger.Error("failed to unmarshal entity: %v", err)
			continue
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// SearchByType searches for entities of a specific type
func (m *EntityMemoryStruct) SearchByType(ctx context.Context, entityType EntityType, query string) ([]EntityMemoryItem, error) {
	memQuery := &MemoryQuery{
		Query:      query,
		Type:       GeneralMemory,
		MaxResults: m.config.MaxResults,
		Metadata: map[string]interface{}{
			MetadataKeyType: "entity",
			"entity_type":   string(entityType),
		},
	}

	items, err := m.Search(memQuery)
	if err != nil {
		return nil, err
	}

	var entities []EntityMemoryItem
	for _, item := range items {
		var entity EntityMemoryItem
		if err := json.Unmarshal([]byte(item.Value), &entity); err != nil {
			m.logger.Error("failed to unmarshal entity: %v", err)
			continue
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// GetEntity retrieves a specific entity by name
func (m *EntityMemoryStruct) GetEntity(ctx context.Context, name string) (*EntityMemoryItem, error) {
	memQuery := &MemoryQuery{
		Query:      name,
		Type:       GeneralMemory,
		MaxResults: 1,
		Metadata: map[string]interface{}{
			MetadataKeyType: "entity",
			"entity_name":   name,
		},
	}

	items, err := m.Search(memQuery)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("entity not found: %s", name)
	}

	var entity EntityMemoryItem
	if err := json.Unmarshal([]byte(items[0].Value), &entity); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
	}

	return &entity, nil
}

// Reset clears all stored items
func (m *EntityMemoryStruct) Reset(ctx context.Context) error {
	return m.storage.Reset(ctx)
}
