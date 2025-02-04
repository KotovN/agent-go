package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KotovN/agent-go/pkg/types"
)

// Executor handles task execution for agents
type Executor struct {
	maxIterations int
	tools         []Tool
	llm           LLMProvider
}

// NewExecutor creates a new executor instance
func NewExecutor(llm LLMProvider) *Executor {
	return &Executor{
		maxIterations: 10,
		tools:         make([]Tool, 0),
		llm:           llm,
	}
}

// WithTools adds tools to the executor
func (e *Executor) WithTools(tools ...Tool) *Executor {
	e.tools = append(e.tools, tools...)
	return e
}

// Execute executes a task with the given description
func (e *Executor) Execute(ctx context.Context, taskPrompt string) (string, error) {
	var result string

	for iteration := 0; iteration < e.maxIterations; iteration++ {
		// Get available tools
		var toolDescriptions string
		if len(e.tools) > 0 {
			toolDescriptions = "\n\nAvailable tools:\n"
			for _, tool := range e.tools {
				toolDescriptions += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
				params := tool.Parameters()
				if len(params) > 0 {
					toolDescriptions += "  Parameters:\n"
					for name, param := range params {
						toolDescriptions += fmt.Sprintf("  - %s: %s (Required: %v)\n",
							name, param.Description, param.Required)
					}
				}
			}
			toolDescriptions += "\nTo use a tool, respond with:\nUSE_TOOL: {\"tool\": \"tool_name\", \"input\": \"tool input\"}"
		}

		// Build full prompt
		fullPrompt := taskPrompt + toolDescriptions
		if result != "" {
			fullPrompt += fmt.Sprintf("\n\nPrevious step result:\n%s", result)
		}

		// Convert tools to function definitions if available
		var functions []types.Function
		if len(e.tools) > 0 {
			functions = make([]types.Function, len(e.tools))
			for i, tool := range e.tools {
				functions[i] = types.Function{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  convertParameters(tool.Parameters()),
				}
			}
		}

		// Get response from LLM
		var response string
		var err error
		if len(functions) > 0 {
			response, err = e.llm.CompleteWithFunctions(ctx, fullPrompt, functions)
		} else {
			response, err = e.llm.Complete(ctx, fullPrompt)
		}
		if err != nil {
			return "", fmt.Errorf("failed to get LLM response: %w", err)
		}

		// Check for tool usage
		if strings.Contains(response, "USE_TOOL:") {
			// Parse tool call
			toolCall, err := e.parseToolCall(response)
			if err != nil {
				return "", fmt.Errorf("failed to parse tool call: %w", err)
			}

			// Execute tool
			result, err = e.executeTool(ctx, toolCall)
			if err != nil {
				return "", fmt.Errorf("tool execution failed: %w", err)
			}

			// Continue to next iteration
			continue
		}

		// No tool usage, return final response
		return response, nil
	}

	return "", fmt.Errorf("exceeded maximum iterations")
}

// parseToolCall attempts to parse a tool call from the LLM response
func (e *Executor) parseToolCall(response string) (*types.ToolCall, error) {
	// Extract the JSON part
	parts := strings.Split(response, "USE_TOOL:")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid tool call format")
	}

	var toolCall types.ToolCall
	if err := json.Unmarshal([]byte(strings.TrimSpace(parts[1])), &toolCall); err != nil {
		return nil, fmt.Errorf("failed to parse tool call: %w", err)
	}

	return &toolCall, nil
}

// executeTool executes a tool with the given input
func (e *Executor) executeTool(ctx context.Context, toolCall *types.ToolCall) (string, error) {
	// Find the tool
	var tool Tool
	for _, t := range e.tools {
		if strings.EqualFold(t.Name(), toolCall.Tool) {
			tool = t
			break
		}
	}

	if tool == nil {
		return "", fmt.Errorf("tool not found: %s", toolCall.Tool)
	}

	// Execute the tool
	result, err := tool.Execute(toolCall.Input)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// convertParameters converts core.ParameterDefinition to types.ParameterDefinition
func convertParameters(params map[string]ParameterDefinition) map[string]types.ParameterDefinition {
	converted := make(map[string]types.ParameterDefinition)
	for name, param := range params {
		converted[name] = types.ParameterDefinition{
			Type:        param.Type,
			Description: param.Description,
			Required:    param.Required,
			Enum:        param.Enum,
		}
	}
	return converted
}
