package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PineconeStore implements vector store using Pinecone
type PineconeStore struct {
	apiKey      string
	projectID   string
	indexName   string
	environment string
	dimension   int
	client      *http.Client
	baseURL     string
}

type pineconeVector struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type pineconeUpsertRequest struct {
	Vectors []pineconeVector `json:"vectors"`
}

type pineconeQueryRequest struct {
	Vector    []float32 `json:"vector"`
	TopK      int       `json:"topK"`
	Namespace string    `json:"namespace,omitempty"`
}

type pineconeQueryResponse struct {
	Matches []struct {
		ID       string                 `json:"id"`
		Score    float32                `json:"score"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
		Values   []float32              `json:"values,omitempty"`
	} `json:"matches"`
}

// NewPineconeStore creates a new Pinecone vector store
func NewPineconeStore(ctx context.Context, config Config) (Store, error) {
	apiKey, ok := config.Options["api_key"].(string)
	if !ok {
		return nil, fmt.Errorf("api_key is required for Pinecone store")
	}

	projectID, ok := config.Options["project_id"].(string)
	if !ok {
		return nil, fmt.Errorf("project_id is required for Pinecone store")
	}

	indexName, ok := config.Options["index_name"].(string)
	if !ok {
		return nil, fmt.Errorf("index_name is required for Pinecone store")
	}

	environment, ok := config.Options["environment"].(string)
	if !ok {
		return nil, fmt.Errorf("environment is required for Pinecone store")
	}

	store := &PineconeStore{
		apiKey:      apiKey,
		projectID:   projectID,
		indexName:   indexName,
		environment: environment,
		dimension:   config.Dimension,
		client:      &http.Client{Timeout: 30 * time.Second},
		baseURL:     fmt.Sprintf("https://%s-%s.svc.%s.pinecone.io", indexName, projectID, environment),
	}

	// Test connection
	if err := store.ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Pinecone: %w", err)
	}

	return store, nil
}

func (s *PineconeStore) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/describe_index_stats", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Pinecone API returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *PineconeStore) Insert(ctx context.Context, docs ...Document) error {
	vectors := make([]pineconeVector, len(docs))
	for i, doc := range docs {
		if len(doc.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(doc.Vector))
		}
		vectors[i] = pineconeVector{
			ID:       doc.ID,
			Values:   doc.Vector,
			Metadata: doc.Metadata,
		}
		if doc.Content != "" {
			if vectors[i].Metadata == nil {
				vectors[i].Metadata = make(map[string]interface{})
			}
			vectors[i].Metadata["content"] = doc.Content
		}
	}

	payload := pineconeUpsertRequest{Vectors: vectors}
	return s.makeRequest(ctx, "POST", "/vectors/upsert", payload, nil)
}

func (s *PineconeStore) Search(ctx context.Context, queryVector Vector, limit int) ([]SearchResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(queryVector))
	}

	payload := pineconeQueryRequest{
		Vector: queryVector,
		TopK:   limit,
	}

	var response pineconeQueryResponse
	if err := s.makeRequest(ctx, "POST", "/query", payload, &response); err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(response.Matches))
	for i, match := range response.Matches {
		content, _ := match.Metadata["content"].(string)
		delete(match.Metadata, "content")

		results[i] = SearchResult{
			Document: Document{
				ID:       match.ID,
				Content:  content,
				Vector:   match.Values,
				Metadata: match.Metadata,
			},
			Score: match.Score,
		}
	}

	return results, nil
}

func (s *PineconeStore) Delete(ctx context.Context, ids ...string) error {
	payload := map[string]interface{}{
		"ids": ids,
	}
	return s.makeRequest(ctx, "POST", "/vectors/delete", payload, nil)
}

func (s *PineconeStore) Update(ctx context.Context, docs ...Document) error {
	// Pinecone upsert handles updates
	return s.Insert(ctx, docs...)
}

func (s *PineconeStore) Get(ctx context.Context, ids ...string) ([]Document, error) {
	payload := map[string]interface{}{
		"ids": ids,
	}

	var response struct {
		Vectors map[string]pineconeVector `json:"vectors"`
	}

	if err := s.makeRequest(ctx, "GET", "/vectors/fetch", payload, &response); err != nil {
		return nil, err
	}

	docs := make([]Document, 0, len(response.Vectors))
	for _, vec := range response.Vectors {
		content, _ := vec.Metadata["content"].(string)
		delete(vec.Metadata, "content")

		docs = append(docs, Document{
			ID:       vec.ID,
			Content:  content,
			Vector:   vec.Values,
			Metadata: vec.Metadata,
		})
	}

	return docs, nil
}

func (s *PineconeStore) Clear(ctx context.Context) error {
	return s.makeRequest(ctx, "POST", "/vectors/delete", map[string]interface{}{"deleteAll": true}, nil)
}

func (s *PineconeStore) Close() error {
	return nil // HTTP client doesn't need explicit cleanup
}

func (s *PineconeStore) makeRequest(ctx context.Context, method, path string, payload interface{}, response interface{}) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Api-Key", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
