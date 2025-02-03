package memory

import (
	"context"
	"fmt"
	"path/filepath"
)

// NewStorage creates a new storage instance based on the provided configuration
func NewStorage(ctx context.Context, config *StorageConfig, baseDir string) (Storage, error) {
	if config == nil {
		return nil, fmt.Errorf("storage config is required")
	}

	switch config.Provider {
	case ProviderLocal:
		// Use SQLite storage for local persistence
		if config.Path == "" {
			config.Path = filepath.Join(baseDir, "memory.db")
		}
		return NewSQLiteStorage(config.Path)

	case ProviderRAG:
		// Use RAG storage with vector embeddings
		embedder := NewSimpleEmbedder(64) // TODO: Make dimension configurable
		return NewRAGStorage(embedder), nil

	case ProviderMem0:
		// TODO: Implement Mem0 storage provider
		return nil, fmt.Errorf("mem0 storage provider not implemented yet")

	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", config.Provider)
	}
}
