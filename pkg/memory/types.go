package memory

import (
	"context"
	"time"
)

// MemoryScope defines the scope of memory storage
type MemoryScope string

const (
	// ScopeShortTerm represents short-term memory storage
	ScopeShortTerm MemoryScope = "short_term"
	// ScopeLongTerm represents long-term memory storage
	ScopeLongTerm MemoryScope = "long_term"
	// ScopeEntity represents entity memory storage
	ScopeEntity MemoryScope = "entity"
	// TaskScope indicates memory is only relevant to a specific task
	TaskScope MemoryScope = "task"
	// ProcessScope indicates memory is relevant to the entire process/workflow
	ProcessScope MemoryScope = "process"
	// GlobalScope indicates memory is globally accessible
	GlobalScope MemoryScope = "global"
)

// MemoryType defines the type of memory storage
type MemoryType string

const (
	// TaskMemory represents memory related to task execution
	TaskMemory MemoryType = "task"
	// AgentMemory represents memory specific to an agent
	AgentMemory MemoryType = "agent"
	// ProcessMemory represents memory related to process execution
	ProcessMemory MemoryType = "process"
	// GeneralMemory represents general purpose memory
	GeneralMemory MemoryType = "general"
)

// MemoryQuery represents a query for retrieving memories
type MemoryQuery struct {
	Query       string                 `json:"query"`
	Type        MemoryType             `json:"type"`
	Scope       MemoryScope            `json:"scope"`
	TaskID      string                 `json:"task_id,omitempty"`
	AgentName   string                 `json:"agent_name,omitempty"`
	ProcessType string                 `json:"process_type,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	MaxResults  int                    `json:"max_results"`
	MinScore    float64                `json:"min_score"`
}

// MemoryResult represents a result from a memory query
type MemoryResult struct {
	Value     string                 `json:"value"`
	Score     float64                `json:"score"`
	Metadata  map[string]interface{} `json:"metadata"`
	Scope     MemoryScope            `json:"scope"`
	Type      MemoryType             `json:"type"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
}

// MemoryNotification represents a notification about memory updates
type MemoryNotification struct {
	Source    string                 `json:"source"`
	Target    string                 `json:"target"`
	Value     string                 `json:"value"`
	Metadata  map[string]interface{} `json:"metadata"`
	Scope     MemoryScope            `json:"scope"`
	Type      MemoryType             `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
}

// Memory defines the interface for memory operations
type Memory interface {
	// Save stores a memory with optional metadata and expiration
	Save(value string, metadata map[string]interface{}, source string, scope MemoryScope, expiresAt *time.Time) error

	// Search queries memories based on the provided query parameters
	Search(query *MemoryQuery) ([]MemoryResult, error)

	// Subscribe registers a callback for memory notifications
	Subscribe(target string, callback func(MemoryNotification)) error

	// Unsubscribe removes a notification callback
	Unsubscribe(target string) error

	// Reset clears all stored memories
	Reset() error
}

// StorageProvider defines the type of storage provider
type StorageProvider string

const (
	// ProviderLocal uses local storage (SQLite/files)
	ProviderLocal StorageProvider = "local"
	// ProviderRAG uses RAG storage with vector embeddings
	ProviderRAG StorageProvider = "rag"
	// ProviderMem0 uses Mem0 external service
	ProviderMem0 StorageProvider = "mem0"
)

// StorageConfig defines configuration options for storage backends
type StorageConfig struct {
	// Provider specifies the storage provider to use
	Provider StorageProvider `json:"provider"`

	// Path is the storage location (e.g. file path, connection string)
	Path string `json:"path"`

	// MaxSize is the maximum number of items to store
	MaxSize int `json:"max_size"`

	// CleanupInterval is how often to remove expired items
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// Additional provider-specific options
	Options map[string]interface{} `json:"options"`
}

// MemoryConfig defines configuration options for memory components
type MemoryConfig struct {
	MaxResults          int           `json:"max_results"`
	RelevanceThreshold  float64       `json:"relevance_threshold"`
	DefaultTTL          time.Duration `json:"default_ttl"`
	EnableNotifications bool          `json:"enable_notifications"`

	// Storage configuration for different memory types
	ShortTermStorage *StorageConfig `json:"short_term_storage,omitempty"`
	LongTermStorage  *StorageConfig `json:"long_term_storage,omitempty"`
	EntityStorage    *StorageConfig `json:"entity_storage,omitempty"`
}

// DefaultMemoryConfig returns the default memory configuration
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MaxResults:          5,
		RelevanceThreshold:  0.5,
		DefaultTTL:          time.Hour,
		EnableNotifications: true,
		ShortTermStorage: &StorageConfig{
			Provider: ProviderRAG,
			MaxSize:  1000,
		},
		LongTermStorage: &StorageConfig{
			Provider: ProviderLocal,
			MaxSize:  10000,
		},
		EntityStorage: &StorageConfig{
			Provider: ProviderRAG,
			MaxSize:  1000,
		},
	}
}

// EntityType represents the type of an entity in memory
type EntityType string

const (
	EntityTypePerson   EntityType = "person"
	EntityTypePlace    EntityType = "place"
	EntityTypeConcept  EntityType = "concept"
	EntityTypeOrg      EntityType = "organization"
	EntityTypeArtifact EntityType = "artifact"
)

// EntityMemoryItem represents an entity in memory with relationships
type EntityMemoryItem struct {
	// Name uniquely identifies the entity
	Name string

	// Type indicates the entity type
	Type EntityType

	// Description provides entity details
	Description string

	// Properties contains entity properties
	Properties map[string]interface{}

	// Relationships maps relationship types to related entity names
	Relationships map[string][]string

	// Metadata contains additional metadata
	Metadata map[string]interface{}

	// Agent indicates which agent created/owns this entity
	Agent string

	// Timestamp indicates when the entity was created/updated
	Timestamp time.Time
}

// StorageItem represents a stored memory item
type StorageItem struct {
	Value     string                 `json:"value"`
	Metadata  map[string]interface{} `json:"metadata"`
	Source    string                 `json:"source"`
	Scope     MemoryScope            `json:"scope"`
	Type      MemoryType             `json:"type"`
	Score     float64                `json:"score,omitempty"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Storage defines the interface for memory storage backends
type Storage interface {
	// Save stores a memory item
	Save(ctx context.Context, item *StorageItem) error

	// Search finds memories matching the query
	Search(ctx context.Context, query string, maxResults int) ([]*StorageItem, error)

	// Reset clears all stored memories
	Reset(ctx context.Context) error
}

// SharedMemoryContext represents shared memory between agents
type SharedMemoryContext struct {
	// Source indicates which agent shared this memory
	Source string `json:"source"`

	// Target specifies which agents can access this memory (empty means all)
	Target []string `json:"target,omitempty"`

	// Scope defines the context scope (task, process, general)
	Scope MemoryType `json:"scope"`

	// Value is the actual memory content
	Value interface{} `json:"value"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata"`

	// Timestamp indicates when this was shared
	Timestamp time.Time `json:"timestamp"`

	// Relevance indicates how relevant this context is (0-1)
	Relevance float64 `json:"relevance,omitempty"`
}

// MemoryAccessLevel defines how agents can interact with shared memories
type MemoryAccessLevel string

const (
	// AccessReadOnly means agent can only read the memory
	AccessReadOnly MemoryAccessLevel = "read"

	// AccessReadWrite means agent can read and modify the memory
	AccessReadWrite MemoryAccessLevel = "write"
)

// MemoryPermission defines access control for shared memories
type MemoryPermission struct {
	// AgentID identifies the agent
	AgentID string `json:"agent_id"`

	// Scope defines visibility scope
	Scope MemoryScope `json:"scope"`

	// AccessLevel defines interaction level
	AccessLevel MemoryAccessLevel `json:"access_level"`

	// AllowedTypes specifies which memory types this agent can access
	AllowedTypes []MemoryType `json:"allowed_types,omitempty"`
}

// Standard metadata keys
const (
	// MetadataKeySource indicates the source/creator of the memory
	MetadataKeySource = "source"
	// MetadataKeyTimestamp indicates when the memory was created
	MetadataKeyTimestamp = "timestamp"
	// MetadataKeyType indicates the type of memory (e.g., "entity", "task", "general")
	MetadataKeyType = "type"
	// MetadataKeyShared indicates if the memory is shared between agents
	MetadataKeyShared = "shared"
	// MetadataKeyTaskID links memory to a specific task
	MetadataKeyTaskID = "task_id"
	// MetadataKeyAgentID links memory to a specific agent
	MetadataKeyAgentID = "agent_id"
	// MetadataKeyProcessType indicates the process type (sequential, hierarchical)
	MetadataKeyProcessType = "process_type"
)
