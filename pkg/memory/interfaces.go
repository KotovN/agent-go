package memory

import (
	"context"
)

// EntityMemory defines the interface for entity-based memory
type EntityMemory interface {
	// SaveEntity stores an entity with its relationships
	SaveEntity(ctx context.Context, item EntityMemoryItem) error

	// SearchEntities performs a search over stored entities
	SearchEntities(ctx context.Context, query string) ([]EntityMemoryItem, error)

	// SearchByType searches for entities of a specific type
	SearchByType(ctx context.Context, entityType EntityType, query string) ([]EntityMemoryItem, error)

	// GetEntity retrieves a specific entity by name
	GetEntity(ctx context.Context, name string) (*EntityMemoryItem, error)
}
