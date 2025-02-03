package memory

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIEmbedder implements Embedder interface using OpenAI's embeddings API
type OpenAIEmbedder struct {
	client    *openai.Client
	model     openai.EmbeddingModel
	dimension int
}

// NewOpenAIEmbedder creates a new OpenAI embedder instance
func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
	client := openai.NewClient(apiKey)
	return &OpenAIEmbedder{
		client:    client,
		model:     openai.AdaEmbeddingV2,
		dimension: 1536, // Ada v2 embedding dimension
	}
}

// Embed generates embeddings for the given text using OpenAI's API
func (e *OpenAIEmbedder) Embed(text string) ([]float32, error) {
	resp, err := e.client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: []string{text},
		Model: e.model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// Dimension returns the dimension of the embeddings
func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}
