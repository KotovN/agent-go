package llm

import (
	"context"
	"fmt"
	"os"

	"agent-go/pkg/core"
	"agent-go/pkg/types"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiStudioProvider implements the Provider interface for Google's Gemini model via AI Studio
type GeminiStudioProvider struct {
	client *genai.Client
	config *core.LLMConfig
}

// NewGeminiStudioProvider creates a new Gemini provider instance using AI Studio
func NewGeminiStudioProvider(ctx context.Context, config *core.LLMConfig) (*GeminiStudioProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv("GOOGLE_API_KEY")
	}
	if config.Model == "" {
		config.Model = "gemini-pro" // Default to Gemini Pro
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiStudioProvider{
		client: client,
		config: config,
	}, nil
}

// Complete generates a completion for a prompt
func (p *GeminiStudioProvider) Complete(ctx context.Context, prompt string) (string, error) {
	model := p.client.GenerativeModel(p.config.Model)
	model.SetTemperature(float32(p.config.Temperature))
	model.SetTopP(float32(p.config.TopP))
	model.SetMaxOutputTokens(int32(p.config.MaxTokens))

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini completion error: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	parts := resp.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", fmt.Errorf("no content parts in Gemini response")
	}

	// Extract text from the response
	var result string
	for _, part := range parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	if result == "" {
		return "", fmt.Errorf("no text content in Gemini response")
	}

	return result, nil
}

// CompleteWithFunctions generates a completion with function calling support
func (p *GeminiStudioProvider) CompleteWithFunctions(ctx context.Context, prompt string, functions []types.Function) (string, error) {
	// Note: Gemini doesn't support function calling in the same way as OpenAI
	// We'll need to format the functions as part of the prompt
	functionDesc := "Available functions:\n"
	for _, f := range functions {
		functionDesc += fmt.Sprintf("- %s: %s\n", f.Name, f.Description)
	}

	fullPrompt := fmt.Sprintf("%s\n\n%s", functionDesc, prompt)
	return p.Complete(ctx, fullPrompt)
}

// Chat generates a response in a chat conversation
func (p *GeminiStudioProvider) Chat(ctx context.Context, messages []types.Message) (string, error) {
	model := p.client.GenerativeModel(p.config.Model)
	model.SetTemperature(float32(p.config.Temperature))
	model.SetTopP(float32(p.config.TopP))
	model.SetMaxOutputTokens(int32(p.config.MaxTokens))

	// Convert our messages to Gemini's chat format
	chat := model.StartChat()
	for _, msg := range messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		chat.History = append(chat.History, &genai.Content{
			Role:  role,
			Parts: []genai.Part{genai.Text(msg.Content)},
		})
	}

	resp, err := chat.SendMessage(ctx, genai.Text(""))
	if err != nil {
		return "", fmt.Errorf("gemini chat error: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	parts := resp.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", fmt.Errorf("no content parts in Gemini response")
	}

	// Extract text from the response
	var result string
	for _, part := range parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	if result == "" {
		return "", fmt.Errorf("no text content in Gemini response")
	}

	return result, nil
}

// GetAPIKey returns the API key
func (p *GeminiStudioProvider) GetAPIKey() string {
	return p.config.APIKey
}

// GetConfig returns the current configuration
func (p *GeminiStudioProvider) GetConfig() *core.LLMConfig {
	return p.config
}

// SupportsFunction returns whether function calling is supported
func (p *GeminiStudioProvider) SupportsFunction() bool {
	// Gemini doesn't support OpenAI-style function calling
	return false
}

// GetContextWindowSize returns the context window size for the model
func (p *GeminiStudioProvider) GetContextWindowSize() int {
	switch p.config.Model {
	case "gemini-pro":
		return 32768 // 32k tokens
	case "gemini-pro-vision":
		return 16384 // 16k tokens
	default:
		return 32768
	}
}
