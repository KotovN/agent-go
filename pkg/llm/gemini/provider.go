package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"agent-go/pkg/core"
	"agent-go/pkg/types"
)

// Provider implements the LLM provider interface for Google's Gemini API
type Provider struct {
	apiKey     string
	model      string
	httpClient *http.Client
	baseURL    string
}

// NewProvider creates a new Gemini provider instance
func NewProvider(apiKey string, model string) *Provider {
	return &Provider{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
		baseURL:    "https://generativelanguage.googleapis.com/v1",
	}
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
	Tools    []geminiTool    `json:"tools,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"function_declarations"`
}

type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

// Complete generates a completion for a prompt
func (p *Provider) Complete(ctx context.Context, prompt string) (string, error) {
	return p.CompleteWithFunctions(ctx, prompt, nil)
}

// CompleteWithFunctions generates a completion with function calling support
func (p *Provider) CompleteWithFunctions(ctx context.Context, prompt string, functions []types.Function) (string, error) {
	// Convert prompt to Gemini format
	contents := []geminiContent{
		{
			Parts: []geminiPart{{Text: prompt}},
			Role:  "user",
		},
	}

	req := geminiRequest{
		Contents: contents,
	}

	if len(functions) > 0 {
		tools := []geminiTool{{
			FunctionDeclarations: p.convertFunctions(functions),
		}}
		req.Tools = tools
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no parts in response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// Chat generates a response in a chat conversation
func (p *Provider) Chat(ctx context.Context, messages []types.Message) (string, error) {
	contents := make([]geminiContent, len(messages))
	for i, msg := range messages {
		contents[i] = geminiContent{
			Parts: []geminiPart{{Text: msg.Content}},
			Role:  msg.Role,
		}
	}

	req := geminiRequest{
		Contents: contents,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no parts in response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// convertFunctions converts core function definitions to Gemini format
func (p *Provider) convertFunctions(functions []types.Function) []geminiFunctionDeclaration {
	result := make([]geminiFunctionDeclaration, len(functions))
	for i, fn := range functions {
		parameters := make(map[string]interface{})
		properties := make(map[string]interface{})
		var required []string

		for name, param := range fn.Parameters {
			paramDef := map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			if len(param.Enum) > 0 {
				paramDef["enum"] = param.Enum
			}
			properties[name] = paramDef

			if param.Required {
				required = append(required, name)
			}
		}

		parameters["type"] = "object"
		parameters["properties"] = properties
		if len(required) > 0 {
			parameters["required"] = required
		}

		result[i] = geminiFunctionDeclaration{
			Name:        fn.Name,
			Description: fn.Description,
			Parameters:  parameters,
		}
	}
	return result
}

// GetAPIKey returns the API key for the provider
func (p *Provider) GetAPIKey() string {
	return p.apiKey
}

// GetConfig returns the current configuration
func (p *Provider) GetConfig() *core.LLMConfig {
	return &core.LLMConfig{
		Model:   p.model,
		APIKey:  p.apiKey,
		BaseURL: p.baseURL,
	}
}

// SupportsFunction returns whether the provider supports function calling
func (p *Provider) SupportsFunction() bool {
	return true
}

// GetContextWindowSize returns the context window size for the model
func (p *Provider) GetContextWindowSize() int {
	// Gemini Pro has a context window of 32k tokens
	return 32768
}
