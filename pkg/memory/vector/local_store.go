package vector

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
)

// LocalStore implements an in-memory vector store
type LocalStore struct {
	mu        sync.RWMutex
	docs      map[string]Document
	dimension int
	metric    MetricType
}

// NewLocalStore creates a new local vector store
func NewLocalStore(ctx context.Context, config Config) (Store, error) {
	if config.Dimension <= 0 {
		return nil, fmt.Errorf("dimension must be positive")
	}

	return &LocalStore{
		docs:      make(map[string]Document),
		dimension: config.Dimension,
		metric:    MetricType(config.MetricType),
	}, nil
}

func (s *LocalStore) Insert(ctx context.Context, docs ...Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, doc := range docs {
		if len(doc.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(doc.Vector))
		}
		s.docs[doc.ID] = doc
	}
	return nil
}

func (s *LocalStore) Search(ctx context.Context, queryVector Vector, limit int) ([]SearchResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(queryVector))
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]SearchResult, 0, len(s.docs))
	for _, doc := range s.docs {
		score := s.calculateSimilarity(queryVector, doc.Vector)
		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	// Sort by score in descending order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}

func (s *LocalStore) Delete(ctx context.Context, ids ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.docs, id)
	}
	return nil
}

func (s *LocalStore) Update(ctx context.Context, docs ...Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, doc := range docs {
		if len(doc.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(doc.Vector))
		}
		if _, exists := s.docs[doc.ID]; !exists {
			return fmt.Errorf("document not found: %s", doc.ID)
		}
		s.docs[doc.ID] = doc
	}
	return nil
}

func (s *LocalStore) Get(ctx context.Context, ids ...string) ([]Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := make([]Document, 0, len(ids))
	for _, id := range ids {
		if doc, exists := s.docs[id]; exists {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func (s *LocalStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.docs = make(map[string]Document)
	return nil
}

func (s *LocalStore) Close() error {
	return nil // Nothing to close for in-memory store
}

func (s *LocalStore) calculateSimilarity(a, b Vector) float32 {
	switch s.metric {
	case MetricCosine:
		return s.cosineSimilarity(a, b)
	case MetricEuclidean:
		return s.euclideanSimilarity(a, b)
	case MetricDotProduct:
		return s.dotProduct(a, b)
	default:
		return s.cosineSimilarity(a, b) // Default to cosine similarity
	}
}

func (s *LocalStore) cosineSimilarity(a, b Vector) float32 {
	dot := float64(0)
	normA := float64(0)
	normB := float64(0)

	for i := 0; i < len(a); i++ {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func (s *LocalStore) euclideanSimilarity(a, b Vector) float32 {
	sum := float64(0)
	for i := 0; i < len(a); i++ {
		diff := float64(a[i] - b[i])
		sum += diff * diff
	}
	// Convert distance to similarity (1 / (1 + distance))
	return float32(1 / (1 + math.Sqrt(sum)))
}

func (s *LocalStore) dotProduct(a, b Vector) float32 {
	dot := float32(0)
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
	}
	return dot
}
