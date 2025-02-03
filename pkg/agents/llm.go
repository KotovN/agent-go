package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"agentai/pkg/core"
	"agentai/pkg/types"

	"github.com/sashabaranov/go-openai"
)

// LLMProvider handles interactions with language models
type LLMProvider struct {
	client *openai.Client
	config types.LLMConfig
}

// NewLLMProvider creates a new LLM provider instance
func NewLLMProvider(apiKey string, config types.LLMConfig) *LLMProvider {
	return &LLMProvider{
		client: openai.NewClient(apiKey),
		config: config,
	}
}

// Complete generates a completion using the configured LLM
func (l *LLMProvider) Complete(ctx context.Context, prompt string) (string, error) {
	return l.CompleteWithFunctions(ctx, prompt, nil)
}

// CompleteWithFunctions generates a completion with function calling support
func (l *LLMProvider) CompleteWithFunctions(ctx context.Context, prompt string, functions []core.FunctionDefinition) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	req := openai.ChatCompletionRequest{
		Model:       l.config.Model,
		Messages:    messages,
		Temperature: float32(l.config.Temperature),
		MaxTokens:   l.config.MaxTokens,
	}

	// Add functions if provided
	if len(functions) > 0 {
		req.Functions = l.convertFunctionDefinitions(functions)
		req.FunctionCall = "auto"
	}

	resp, err := l.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	choice := resp.Choices[0].Message

	// Handle function calls
	if choice.FunctionCall != nil {
		return l.formatFunctionCall(choice.FunctionCall)
	}

	return choice.Content, nil
}

// convertFunctionDefinitions converts our function definitions to OpenAI format
func (l *LLMProvider) convertFunctionDefinitions(functions []core.FunctionDefinition) []openai.FunctionDefinition {
	result := make([]openai.FunctionDefinition, len(functions))
	for i, fn := range functions {
		// Convert parameters to OpenAI format
		parameters := make(map[string]interface{})
		required := make([]string, 0)

		// Add type and properties
		properties := make(map[string]interface{})
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

		result[i] = openai.FunctionDefinition{
			Name:        fn.Name,
			Description: fn.Description,
			Parameters:  parameters,
		}
	}
	return result
}

// formatFunctionCall formats an OpenAI function call into our tool call format
func (l *LLMProvider) formatFunctionCall(call *openai.FunctionCall) (string, error) {
	toolCall := struct {
		Tool  string `json:"tool"`
		Input string `json:"input"`
	}{
		Tool:  call.Name,
		Input: call.Arguments,
	}

	result, err := json.Marshal(toolCall)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool call: %w", err)
	}

	return fmt.Sprintf("USE_TOOL: %s", string(result)), nil
}
