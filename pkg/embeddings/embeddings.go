package embeddings

import (
	"context"
	"fmt"
)

// Model represents a text embedding model
type Model interface {
	// Embed generates embeddings for the given texts
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the dimension of the embeddings
	Dimension() int

	// Close releases any resources used by the model
	Close() error
}

// Config holds configuration for embedding models
type Config struct {
	Type    string                 `json:"type"`              // Type of embedding model
	Options map[string]interface{} `json:"options,omitempty"` // Model-specific options
}

// ModelType represents supported embedding model types
type ModelType string

const (
	TypeVertexAI ModelType = "vertex_ai" // Google Vertex AI embeddings
	TypeOpenAI   ModelType = "openai"    // OpenAI embeddings
	TypeLocal    ModelType = "local"     // Local embedding model
)

// NewModel creates a new embedding model based on config
func NewModel(ctx context.Context, config Config) (Model, error) {
	switch ModelType(config.Type) {
	case TypeVertexAI:
		return NewVertexAIModel(ctx, config)
	case TypeOpenAI:
		return NewOpenAIModel(ctx, config)
	case TypeLocal:
		return NewLocalModel(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported embedding model type: %s", config.Type)
	}
}

// Cache provides caching for embeddings to avoid regenerating them
type Cache interface {
	// Get retrieves cached embeddings for texts
	Get(ctx context.Context, texts []string) ([][]float32, []bool, error)

	// Set stores embeddings for texts
	Set(ctx context.Context, texts []string, embeddings [][]float32) error

	// Clear removes all cached embeddings
	Clear(ctx context.Context) error

	// Close releases any resources used by the cache
	Close() error
}

// CachedModel wraps an embedding model with caching
type CachedModel struct {
	model Model
	cache Cache
}

// NewCachedModel creates a new cached embedding model
func NewCachedModel(model Model, cache Cache) *CachedModel {
	return &CachedModel{
		model: model,
		cache: cache,
	}
}

// Embed implements Model.Embed with caching
func (m *CachedModel) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// Try to get from cache first
	embeddings, found, err := m.cache.Get(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	// Generate embeddings for texts not in cache
	var missingTexts []string
	var missingIndices []int
	for i, exists := range found {
		if !exists {
			missingTexts = append(missingTexts, texts[i])
			missingIndices = append(missingIndices, i)
		}
	}

	if len(missingTexts) > 0 {
		newEmbeddings, err := m.model.Embed(ctx, missingTexts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings: %w", err)
		}

		// Store new embeddings in cache
		if err := m.cache.Set(ctx, missingTexts, newEmbeddings); err != nil {
			return nil, fmt.Errorf("failed to store in cache: %w", err)
		}

		// Merge new embeddings with cached ones
		for i, idx := range missingIndices {
			embeddings[idx] = newEmbeddings[i]
		}
	}

	return embeddings, nil
}

// Dimension implements Model.Dimension
func (m *CachedModel) Dimension() int {
	return m.model.Dimension()
}

// Close implements Model.Close
func (m *CachedModel) Close() error {
	var errs []error

	if err := m.model.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close model: %w", err))
	}

	if err := m.cache.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close cache: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing cached model: %v", errs)
	}

	return nil
}
