package llm

import (
	"fmt"
	"strings"

	"github.com/KotovN/agent-go/pkg/core"
	"github.com/KotovN/agent-go/pkg/llm/gemini"
	"github.com/KotovN/agent-go/pkg/llm/openai"
)

// NewProvider creates a new LLM provider based on the configuration
func NewProvider(config core.LLMConfig) (core.LLMProvider, error) {
	switch {
	case config.Model == "":
		return nil, fmt.Errorf("model name is required")
	case config.APIKey == "":
		return nil, fmt.Errorf("API key is required")
	}

	// Check provider based on model prefix
	if strings.HasPrefix(config.Model, "gemini-") {
		return gemini.NewProvider(config.APIKey, config.Model), nil
	}

	// Default to OpenAI
	return openai.NewProvider(config.APIKey, config.Model), nil
}

// GetDefaultModel returns the default model for a provider
func GetDefaultModel(provider string) string {
	switch provider {
	case "gemini":
		return "gemini-pro"
	default:
		return "gpt-4"
	}
}
