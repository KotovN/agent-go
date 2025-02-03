package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VertexAIModel implements Model using Google's Vertex AI
type VertexAIModel struct {
	client    *http.Client
	baseURL   string
	model     string
	dimension int
}

type vertexAIRequest struct {
	Instances []struct {
		Content string `json:"content"`
	} `json:"instances"`
}

type vertexAIResponse struct {
	Predictions []struct {
		Embeddings []float32 `json:"embeddings"`
	} `json:"predictions"`
}

// NewVertexAIModel creates a new Vertex AI embedding model
func NewVertexAIModel(ctx context.Context, config Config) (Model, error) {
	projectID, ok := config.Options["project_id"].(string)
	if !ok {
		return nil, fmt.Errorf("project_id is required for Vertex AI model")
	}

	location, ok := config.Options["location"].(string)
	if !ok {
		location = "us-central1" // Default location
	}

	model, ok := config.Options["model"].(string)
	if !ok {
		model = "textembedding-gecko@001" // Default model
	}

	dimension, ok := config.Options["dimension"].(int)
	if !ok {
		dimension = 768 // Default dimension for gecko model
	}

	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		location, projectID, location, model)

	return &VertexAIModel{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   baseURL,
		model:     model,
		dimension: dimension,
	}, nil
}

// Embed implements Model.Embed
func (m *VertexAIModel) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	// Process texts in batches to avoid rate limits
	batchSize := 5
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

func (m *VertexAIModel) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Prepare request
	req := vertexAIRequest{
		Instances: make([]struct {
			Content string `json:"content"`
		}, len(texts)),
	}

	for i, text := range texts {
		req.Instances[i].Content = text
	}

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// Note: Authentication should be handled by Application Default Credentials

	// Send request
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var response vertexAIResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract embeddings
	embeddings := make([][]float32, len(texts))
	for i, pred := range response.Predictions {
		if len(pred.Embeddings) != m.dimension {
			return nil, fmt.Errorf("unexpected embedding dimension: got %d, want %d", len(pred.Embeddings), m.dimension)
		}
		embeddings[i] = pred.Embeddings
	}

	return embeddings, nil
}

// Dimension implements Model.Dimension
func (m *VertexAIModel) Dimension() int {
	return m.dimension
}

// Close implements Model.Close
func (m *VertexAIModel) Close() error {
	return nil // HTTP client doesn't need explicit cleanup
}
