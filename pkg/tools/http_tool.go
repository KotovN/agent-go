package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/KotovN/agent-go/pkg/core"
)

// HTTPTool provides HTTP request capabilities
type HTTPTool struct {
	client *http.Client
}

// NewHTTPTool creates a new HTTP tool instance
func NewHTTPTool() *HTTPTool {
	return &HTTPTool{
		client: &http.Client{
			Timeout: time.Second * 30,
		},
	}
}

func (t *HTTPTool) Name() string {
	return "http"
}

func (t *HTTPTool) Description() string {
	return "Make HTTP requests to external APIs and web services"
}

type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

func (t *HTTPTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"method": {
			Type:        "string",
			Description: "HTTP method (GET, POST, PUT, DELETE)",
			Required:    true,
			Enum:        []string{"GET", "POST", "PUT", "DELETE"},
		},
		"url": {
			Type:        "string",
			Description: "Target URL for the request",
			Required:    true,
		},
		"headers": {
			Type:        "object",
			Description: "HTTP headers to include in the request",
			Required:    false,
		},
		"body": {
			Type:        "object",
			Description: "Request body (for POST, PUT methods)",
			Required:    false,
		},
	}
}

func (t *HTTPTool) Execute(input string) (string, error) {
	var req HTTPRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("failed to parse request: %w", err)
	}

	// Validate request
	if req.URL == "" {
		return "", fmt.Errorf("URL is required")
	}

	// Create HTTP request
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set default content type if not specified
	if req.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Format response
	response := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(body),
	}

	result, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format response: %w", err)
	}

	return string(result), nil
}
