package cache

import (
	"fmt"
	"sync"
)

// CacheHandler handles caching of tool execution results
type CacheHandler struct {
	cache map[string]string
	mu    sync.RWMutex
}

// NewCacheHandler creates a new cache handler instance
func NewCacheHandler() *CacheHandler {
	return &CacheHandler{
		cache: make(map[string]string),
	}
}

// Add adds a tool execution result to the cache
func (c *CacheHandler) Add(tool, input, output string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[c.generateKey(tool, input)] = output
}

// Read retrieves a cached tool execution result
func (c *CacheHandler) Read(tool, input string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, exists := c.cache[c.generateKey(tool, input)]
	return result, exists
}

// Clear clears all cached entries
func (c *CacheHandler) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]string)
}

// generateKey generates a cache key from tool name and input
func (c *CacheHandler) generateKey(tool, input string) string {
	return fmt.Sprintf("%s-%s", tool, input)
}
