package embeddings

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIModel implements Model using OpenAI's API
type OpenAIModel struct {
	client    *openai.Client
	model     openai.EmbeddingModel
	dimension int
}

// NewOpenAIModel creates a new OpenAI embedding model
func NewOpenAIModel(ctx context.Context, config Config) (Model, error) {
	apiKey, ok := config.Options["api_key"].(string)
	if !ok {
		return nil, fmt.Errorf("api_key is required for OpenAI model")
	}

	var model openai.EmbeddingModel
	if modelStr, ok := config.Options["model"].(string); ok {
		model = openai.EmbeddingModel(modelStr)
	} else {
		model = openai.AdaEmbeddingV2
	}

	dimension, ok := config.Options["dimension"].(int)
	if !ok {
		dimension = 1536 // Default dimension for ada-002
	}

	client := openai.NewClient(apiKey)

	return &OpenAIModel{
		client:    client,
		model:     model,
		dimension: dimension,
	}, nil
}

// Embed implements Model.Embed
func (m *OpenAIModel) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	// Process texts in batches to avoid rate limits
	batchSize := 20 // OpenAI allows larger batches
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		batchEmbeddings, err := m.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to embed batch: %w", err)
		}

		copy(embeddings[i:end], batchEmbeddings)
	}

	return embeddings, nil
}

func (m *OpenAIModel) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	req := openai.EmbeddingRequest{
		Input: texts,
		Model: m.model,
	}

	resp, err := m.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for i, data := range resp.Data {
		if len(data.Embedding) != m.dimension {
			return nil, fmt.Errorf("unexpected embedding dimension: got %d, want %d", len(data.Embedding), m.dimension)
		}
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// Dimension implements Model.Dimension
func (m *OpenAIModel) Dimension() int {
	return m.dimension
}

// Close implements Model.Close
func (m *OpenAIModel) Close() error {
	return nil // HTTP client doesn't need explicit cleanup
}
