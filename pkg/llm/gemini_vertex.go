package llm

import (
	"context"
	"fmt"

	"github.com/KotovN/agent-go/pkg/types"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/api/option"
)

// GeminiVertexProvider implements Provider interface using Google Vertex AI's Gemini model
type GeminiVertexProvider struct {
	client      *genai.Client
	model       *genai.GenerativeModel
	config      *types.LLMConfig
	projectID   string
	location    string
	credentials string
}

// NewGeminiVertexProvider creates a new Vertex AI Gemini provider instance
func NewGeminiVertexProvider(ctx context.Context, projectID, location, credentials string, config *types.LLMConfig) (*GeminiVertexProvider, error) {
	if config == nil {
		config = &types.LLMConfig{
			Model:       "gemini-1.5-pro",
			Temperature: 0.7,
			MaxTokens:   2048,
			TopP:        0.95,
		}
	}

	client, err := genai.NewClient(ctx, projectID, location, option.WithCredentialsFile(credentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	model := client.GenerativeModel(config.Model)

	return &GeminiVertexProvider{
		client:      client,
		model:       model,
		config:      config,
		projectID:   projectID,
		location:    location,
		credentials: credentials,
	}, nil
}

// Complete generates a completion for the given prompt
func (p *GeminiVertexProvider) Complete(ctx context.Context, prompt string) (string, error) {
	// Create prompt content
	prompt_parts := []genai.Part{
		genai.Text(prompt),
	}

	// Generate content
	resp, err := p.model.GenerateContent(ctx, prompt_parts...)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned from model")
	}

	// Extract text from response
	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	if result == "" {
		return "", fmt.Errorf("no text content in response")
	}

	return result, nil
}

// Chat generates a response in a chat conversation
func (p *GeminiVertexProvider) Chat(ctx context.Context, messages []types.Message) (string, error) {
	// Convert messages to Gemini's format
	var chat_parts []genai.Part
	for _, msg := range messages {
		role := msg.Role
		if role == "system" {
			role = "user" // Gemini doesn't support system role, use user instead
		}

		chat_parts = append(chat_parts, genai.Text(msg.Content))
	}

	// Generate content
	resp, err := p.model.GenerateContent(ctx, chat_parts...)
	if err != nil {
		return "", fmt.Errorf("failed to generate chat response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned from model")
	}

	// Extract text from response
	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	if result == "" {
		return "", fmt.Errorf("no text content in response")
	}

	return result, nil
}

// CompleteWithFunctions generates a completion with function calling support
func (p *GeminiVertexProvider) CompleteWithFunctions(ctx context.Context, prompt string, functions []types.Function) (string, error) {
	// Gemini doesn't support function calling yet, fall back to regular completion
	return p.Complete(ctx, prompt)
}

// GetAPIKey returns the API key
func (p *GeminiVertexProvider) GetAPIKey() string {
	return p.credentials
}

// GetConfig returns the provider's configuration
func (p *GeminiVertexProvider) GetConfig() *types.LLMConfig {
	return p.config
}

// SupportsFunction returns whether the provider supports function calling
func (p *GeminiVertexProvider) SupportsFunction() bool {
	return false
}

// GetContextWindowSize returns the context window size for the model
func (p *GeminiVertexProvider) GetContextWindowSize() int {
	switch p.config.Model {
	case "gemini-1.5-pro":
		return 2000000 // 2M tokens
	case "gemini-1.5-flash", "gemini-1.5-flash-8B", "gemini-2.0-flash-exp":
		return 1000000 // 1M tokens
	default:
		return 32000 // Safe default
	}
}
