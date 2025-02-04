package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/KotovN/agent-go/pkg/core"
	"github.com/KotovN/agent-go/pkg/types"
)

// Provider implements the LLM provider interface for OpenAI
type Provider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewProvider creates a new OpenAI provider instance
func NewProvider(apiKey string, model string) *Provider {
	return &Provider{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

type chatRequest struct {
	Model            string           `json:"model"`
	Messages         []types.Message  `json:"messages"`
	Functions        []types.Function `json:"functions,omitempty"`
	FunctionCall     string           `json:"function_call,omitempty"`
	Temperature      float64          `json:"temperature"`
	MaxTokens        int              `json:"max_tokens,omitempty"`
	TopP             float64          `json:"top_p,omitempty"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete generates a completion for a prompt
func (p *Provider) Complete(ctx context.Context, prompt string) (string, error) {
	return p.CompleteWithFunctions(ctx, prompt, nil)
}

// CompleteWithFunctions generates a completion with function calling support
func (p *Provider) CompleteWithFunctions(ctx context.Context, prompt string, functions []types.Function) (string, error) {
	messages := []types.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	req := chatRequest{
		Model:    p.model,
		Messages: messages,
	}

	if len(functions) > 0 {
		req.Functions = functions
		req.FunctionCall = "auto"
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

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

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// Chat generates a response in a chat conversation
func (p *Provider) Chat(ctx context.Context, messages []types.Message) (string, error) {
	req := chatRequest{
		Model:    p.model,
		Messages: messages,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

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

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
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
		BaseURL: "https://api.openai.com/v1",
	}
}

// SupportsFunction returns whether the provider supports function calling
func (p *Provider) SupportsFunction() bool {
	return true
}

// GetContextWindowSize returns the context window size for the model
func (p *Provider) GetContextWindowSize() int {
	// Default to GPT-4's context window
	return 8192
}
