package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// RAGStorage implements Storage interface using vector embeddings for semantic search
type RAGStorage struct {
	mu        sync.RWMutex
	items     []RAGItem
	embedder  Embedder
	dimension int
}

// RAGItem represents a single item in RAG storage
type RAGItem struct {
	StorageItem *StorageItem
	Embedding   []float32
}

// Embedder interface for generating embeddings
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dimension() int
}

// NewRAGStorage creates a new RAG storage instance
func NewRAGStorage(embedder Embedder) *RAGStorage {
	return &RAGStorage{
		items:     make([]RAGItem, 0),
		embedder:  embedder,
		dimension: embedder.Dimension(),
	}
}

// Save stores an item in RAG storage
func (s *RAGStorage) Save(ctx context.Context, item *StorageItem) error {
	// Generate embedding for the value
	embedding, err := s.embedder.Embed(item.Value)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	ragItem := RAGItem{
		StorageItem: item,
		Embedding:   embedding,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, ragItem)

	return nil
}

// Search finds memories matching the query
func (s *RAGStorage) Search(ctx context.Context, query string, maxResults int) ([]*StorageItem, error) {
	// Get embeddings for the query
	queryEmbedding, err := s.embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings: %w", err)
	}

	// Find similar vectors
	results := make([]*StorageItem, 0, maxResults)
	for _, item := range s.items {
		if len(results) >= maxResults {
			break
		}

		// Calculate cosine similarity
		score := cosineSimilarity(queryEmbedding, item.Embedding)

		// Create a copy of the item with the score
		result := *item.StorageItem   // Dereference to get a copy
		result.Score = float64(score) // Convert float32 to float64
		results = append(results, &result)
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// Reset clears all stored items
func (s *RAGStorage) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make([]RAGItem, 0)
	return nil
}
