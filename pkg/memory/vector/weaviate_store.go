package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WeaviateStore implements vector store using Weaviate
type WeaviateStore struct {
	client    *http.Client
	baseURL   string
	apiKey    string
	className string
	dimension int
}

type weaviateObject struct {
	Class      string                 `json:"class"`
	ID         string                 `json:"id,omitempty"`
	Vector     []float32              `json:"vector"`
	Properties map[string]interface{} `json:"properties"`
}

type weaviateResponse struct {
	Objects []struct {
		ID         string                 `json:"id"`
		Vector     []float32              `json:"vector"`
		Properties map[string]interface{} `json:"properties"`
		Score      float32                `json:"score,omitempty"`
	} `json:"objects"`
}

// NewWeaviateStore creates a new Weaviate vector store
func NewWeaviateStore(ctx context.Context, config Config) (Store, error) {
	apiKey, ok := config.Options["api_key"].(string)
	if !ok {
		return nil, fmt.Errorf("api_key is required for Weaviate store")
	}

	baseURL, ok := config.Options["base_url"].(string)
	if !ok {
		return nil, fmt.Errorf("base_url is required for Weaviate store")
	}

	className, ok := config.Options["class_name"].(string)
	if !ok {
		return nil, fmt.Errorf("class_name is required for Weaviate store")
	}

	store := &WeaviateStore{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		className: className,
		dimension: config.Dimension,
	}

	// Test connection and create class if needed
	if err := store.initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize Weaviate store: %w", err)
	}

	return store, nil
}

func (s *WeaviateStore) initialize(ctx context.Context) error {
	// Check if class exists
	req, err := s.newRequest(ctx, "GET", fmt.Sprintf("/v1/schema/%s", s.className), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Create class
		class := map[string]interface{}{
			"class":      s.className,
			"vectorizer": "none", // We provide vectors manually
			"properties": []map[string]interface{}{
				{
					"name":     "content",
					"dataType": []string{"text"},
				},
				{
					"name":     "metadata",
					"dataType": []string{"object"},
				},
			},
		}

		req, err = s.newRequest(ctx, "POST", "/v1/schema", class)
		if err != nil {
			return err
		}

		resp, err = s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to create class: %d", resp.StatusCode)
		}
	}

	return nil
}

func (s *WeaviateStore) Insert(ctx context.Context, docs ...Document) error {
	objects := make([]weaviateObject, len(docs))
	for i, doc := range docs {
		if len(doc.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(doc.Vector))
		}

		objects[i] = weaviateObject{
			Class:  s.className,
			ID:     doc.ID,
			Vector: doc.Vector,
			Properties: map[string]interface{}{
				"content":  doc.Content,
				"metadata": doc.Metadata,
			},
		}
	}

	req, err := s.newRequest(ctx, "POST", "/v1/batch/objects", map[string]interface{}{
		"objects": objects,
	})
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to insert objects: %d", resp.StatusCode)
	}

	return nil
}

func (s *WeaviateStore) Search(ctx context.Context, queryVector Vector, limit int) ([]SearchResult, error) {
	if len(queryVector) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(queryVector))
	}

	query := map[string]interface{}{
		"vector": queryVector,
		"limit":  limit,
	}

	req, err := s.newRequest(ctx, "POST", fmt.Sprintf("/v1/objects/%s/search", s.className), query)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: %d", resp.StatusCode)
	}

	var response weaviateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(response.Objects))
	for i, obj := range response.Objects {
		content, _ := obj.Properties["content"].(string)
		metadata, _ := obj.Properties["metadata"].(map[string]interface{})

		results[i] = SearchResult{
			Document: Document{
				ID:       obj.ID,
				Content:  content,
				Vector:   obj.Vector,
				Metadata: metadata,
			},
			Score: obj.Score,
		}
	}

	return results, nil
}

func (s *WeaviateStore) Delete(ctx context.Context, ids ...string) error {
	for _, id := range ids {
		req, err := s.newRequest(ctx, "DELETE", fmt.Sprintf("/v1/objects/%s/%s", s.className, id), nil)
		if err != nil {
			return err
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("failed to delete object %s: %d", id, resp.StatusCode)
		}
	}

	return nil
}

func (s *WeaviateStore) Update(ctx context.Context, docs ...Document) error {
	for _, doc := range docs {
		if len(doc.Vector) != s.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(doc.Vector))
		}

		obj := weaviateObject{
			Class:  s.className,
			Vector: doc.Vector,
			Properties: map[string]interface{}{
				"content":  doc.Content,
				"metadata": doc.Metadata,
			},
		}

		req, err := s.newRequest(ctx, "PUT", fmt.Sprintf("/v1/objects/%s/%s", s.className, doc.ID), obj)
		if err != nil {
			return err
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to update object %s: %d", doc.ID, resp.StatusCode)
		}
	}

	return nil
}

func (s *WeaviateStore) Get(ctx context.Context, ids ...string) ([]Document, error) {
	var docs []Document

	for _, id := range ids {
		req, err := s.newRequest(ctx, "GET", fmt.Sprintf("/v1/objects/%s/%s", s.className, id), nil)
		if err != nil {
			return nil, err
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusNotFound {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get object %s: %d", id, resp.StatusCode)
		}

		var obj struct {
			ID         string                 `json:"id"`
			Vector     []float32              `json:"vector"`
			Properties map[string]interface{} `json:"properties"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		content, _ := obj.Properties["content"].(string)
		metadata, _ := obj.Properties["metadata"].(map[string]interface{})

		docs = append(docs, Document{
			ID:       obj.ID,
			Content:  content,
			Vector:   obj.Vector,
			Metadata: metadata,
		})
	}

	return docs, nil
}

func (s *WeaviateStore) Clear(ctx context.Context) error {
	req, err := s.newRequest(ctx, "DELETE", fmt.Sprintf("/v1/schema/%s", s.className), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to clear store: %d", resp.StatusCode)
	}

	return s.initialize(ctx)
}

func (s *WeaviateStore) Close() error {
	return nil // HTTP client doesn't need explicit cleanup
}

func (s *WeaviateStore) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var bodyReader strings.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = *strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, &bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
