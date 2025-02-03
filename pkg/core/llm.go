package core

import (
	"agentai/pkg/types"
	"context"
)

// LLMConfig holds configuration for an LLM provider
type LLMConfig struct {
	Model            string
	Temperature      float64
	MaxTokens        int
	TopP             float64
	FrequencyPenalty float64
	PresencePenalty  float64
	Stop             []string
	APIKey           string
	BaseURL          string
	APIVersion       string
	Timeout          float64
}

// LLMProvider defines the interface for language model providers
type LLMProvider interface {
	// Complete generates a completion for a prompt
	Complete(ctx context.Context, prompt string) (string, error)

	// CompleteWithFunctions generates a completion with function calling support
	CompleteWithFunctions(ctx context.Context, prompt string, functions []types.Function) (string, error)

	// Chat generates a response in a chat conversation
	Chat(ctx context.Context, messages []types.Message) (string, error)

	// GetAPIKey returns the API key for the provider
	GetAPIKey() string

	// GetConfig returns the current configuration
	GetConfig() *LLMConfig

	// SupportsFunction returns whether the provider supports function calling
	SupportsFunction() bool

	// GetContextWindowSize returns the context window size for the model
	GetContextWindowSize() int
}

// NewLLMConfig creates a new LLM configuration with default values
func NewLLMConfig() *LLMConfig {
	return &LLMConfig{
		Temperature:      0.7,
		MaxTokens:        2000,
		TopP:             1.0,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
	}
}

// CompletePrompt is a helper function for completing a single prompt
func CompletePrompt(ctx context.Context, provider LLMProvider, prompt string) (string, error) {
	return provider.Complete(ctx, prompt)
}
