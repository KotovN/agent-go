package vector

import (
	"context"
	"fmt"
)

// Vector represents a high-dimensional vector
type Vector []float32

// Document represents a document with its vector embedding
type Document struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Vector   Vector                 `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Document Document `json:"document"`
	Score    float32  `json:"score"`
}

// Store defines the interface for vector storage implementations
type Store interface {
	// Insert adds documents to the store
	Insert(ctx context.Context, docs ...Document) error

	// Search finds similar documents based on query vector
	Search(ctx context.Context, queryVector Vector, limit int) ([]SearchResult, error)

	// Delete removes documents from the store
	Delete(ctx context.Context, ids ...string) error

	// Update updates existing documents
	Update(ctx context.Context, docs ...Document) error

	// Get retrieves documents by their IDs
	Get(ctx context.Context, ids ...string) ([]Document, error)

	// Clear removes all documents from the store
	Clear(ctx context.Context) error

	// Close closes the store connection
	Close() error
}

// Config holds configuration for vector stores
type Config struct {
	Type       string                 `json:"type"`              // Type of vector store (e.g., "pinecone", "weaviate", "local")
	Dimension  int                    `json:"dimension"`         // Dimension of vectors
	MetricType string                 `json:"metric_type"`       // Distance metric type (e.g., "cosine", "euclidean", "dot_product")
	Options    map[string]interface{} `json:"options,omitempty"` // Additional store-specific options
}

// StoreType represents supported vector store types
type StoreType string

const (
	TypePinecone StoreType = "pinecone" // Pinecone vector store
	TypeWeaviate StoreType = "weaviate" // Weaviate vector store
	TypeLocal    StoreType = "local"    // Local in-memory vector store
)

// MetricType represents supported distance metrics
type MetricType string

const (
	MetricCosine     MetricType = "cosine"      // Cosine similarity
	MetricEuclidean  MetricType = "euclidean"   // Euclidean distance
	MetricDotProduct MetricType = "dot_product" // Dot product similarity
)

// NewStore creates a new vector store based on config
func NewStore(ctx context.Context, config Config) (Store, error) {
	switch StoreType(config.Type) {
	case TypePinecone:
		return NewPineconeStore(ctx, config)
	case TypeWeaviate:
		return NewWeaviateStore(ctx, config)
	case TypeLocal:
		return NewLocalStore(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported vector store type: %s", config.Type)
	}
}
