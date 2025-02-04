package llm

import (
	"context"
	"fmt"

	"github.com/KotovN/agent-go/pkg/core"
	"github.com/KotovN/agent-go/pkg/types"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider interface using OpenAI's API
type OpenAIProvider struct {
	client  *openai.Client
	model   string
	apiKey  string
	verbose bool
}

// NewOpenAIProvider creates a new OpenAI provider instance
func NewOpenAIProvider(apiKey string, model string, verbose bool) *OpenAIProvider {
	client := openai.NewClient(apiKey)
	return &OpenAIProvider{
		client:  client,
		model:   model,
		apiKey:  apiKey,
		verbose: verbose,
	}
}

// Complete generates a completion for the given messages
func (p *OpenAIProvider) Complete(ctx context.Context, messages []types.Message) (string, error) {
	return p.CompleteWithFunctions(ctx, messages, nil)
}

// CompleteWithFunctions generates a completion with function calling support
func (p *OpenAIProvider) CompleteWithFunctions(ctx context.Context, messages []types.Message, functions []core.FunctionDefinition) (string, error) {
	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Create chat completion request
	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: openaiMessages,
	}

	// Add functions if provided
	if len(functions) > 0 {
		openaiFunc := make([]openai.FunctionDefinition, len(functions))
		for i, fn := range functions {
			params := make(map[string]interface{})
			for name, param := range fn.Parameters {
				params[name] = map[string]interface{}{
					"type":        param.Type,
					"description": param.Description,
					"required":    param.Required,
				}
			}
			openaiFunc[i] = openai.FunctionDefinition{
				Name:        fn.Name,
				Description: fn.Description,
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": params,
				},
			}
		}
		req.Functions = openaiFunc
		req.FunctionCall = "auto"
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// GetAPIKey returns the API key used by the provider
func (p *OpenAIProvider) GetAPIKey() string {
	return p.apiKey
}
