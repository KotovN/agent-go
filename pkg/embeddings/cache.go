package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// MemoryCache provides in-memory caching for embeddings
type MemoryCache struct {
	mu    sync.RWMutex
	cache map[string][]float32
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		cache: make(map[string][]float32),
	}
}

// Get implements Cache.Get
func (c *MemoryCache) Get(ctx context.Context, texts []string) ([][]float32, []bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	embeddings := make([][]float32, len(texts))
	found := make([]bool, len(texts))

	for i, text := range texts {
		key := c.hashText(text)
		if embedding, ok := c.cache[key]; ok {
			embeddings[i] = embedding
			found[i] = true
		}
	}

	return embeddings, found, nil
}

// Set implements Cache.Set
func (c *MemoryCache) Set(ctx context.Context, texts []string, embeddings [][]float32) error {
	if len(texts) != len(embeddings) {
		return nil // Silently ignore invalid input
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for i, text := range texts {
		key := c.hashText(text)
		c.cache[key] = embeddings[i]
	}

	return nil
}

// Clear implements Cache.Clear
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string][]float32)
	return nil
}

// Close implements Cache.Close
func (c *MemoryCache) Close() error {
	return c.Clear(context.Background())
}

// hashText generates a stable hash for a text string
func (c *MemoryCache) hashText(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
