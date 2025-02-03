package embeddings

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
)

// LocalModel implements Model using a simple hashing-based approach
// This is only for testing and should not be used in production
type LocalModel struct {
	dimension int
}

// NewLocalModel creates a new local embedding model
func NewLocalModel(ctx context.Context, config Config) (Model, error) {
	dimension, ok := config.Options["dimension"].(int)
	if !ok {
		dimension = 64 // Default dimension for testing
	}

	if dimension <= 0 {
		return nil, fmt.Errorf("dimension must be positive")
	}

	return &LocalModel{
		dimension: dimension,
	}, nil
}

// Embed implements Model.Embed
func (m *LocalModel) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, err := m.hashToVector(text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

// hashToVector generates a deterministic vector from text using hashing
func (m *LocalModel) hashToVector(text string) ([]float32, error) {
	h := fnv.New64a()
	if _, err := h.Write([]byte(text)); err != nil {
		return nil, err
	}
	hash := h.Sum64()

	// Generate vector components using the hash
	vec := make([]float32, m.dimension)
	for i := 0; i < m.dimension; i++ {
		// Use different bits of the hash for each dimension
		shift := uint(i % 64)
		val := float64((hash >> shift) & 0xFF)

		// Scale to [-1, 1] range
		val = (val / 127.5) - 1.0

		// Add some variation based on position
		val *= math.Sin(float64(i) * math.Pi / float64(m.dimension))

		vec[i] = float32(val)
	}

	// Normalize the vector
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	norm := float32(math.Sqrt(sum))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec, nil
}

// Dimension implements Model.Dimension
func (m *LocalModel) Dimension() int {
	return m.dimension
}

// Close implements Model.Close
func (m *LocalModel) Close() error {
	return nil // Nothing to close
}
