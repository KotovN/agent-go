package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/KotovN/agent-go/pkg/core"
	"github.com/KotovN/agent-go/pkg/tasks"
	"github.com/KotovN/agent-go/pkg/types"
)

// ExecutorOption defines functional options for configuring an executor
type ExecutorOption func(*Executor)

// Executor handles task execution and tool management
type Executor struct {
	llm              core.LLMProvider
	tools            []core.Tool
	toolsDescription string
	toolsNames       string
	maxIterations    int
	stopWords        []string
	callbacks        []func(string)
	messages         []types.Message
	iterations       int
	verbose          bool
	rolePrompt       string
	taskPrompt       string
	metadata         map[string]string
}

// NewExecutor creates a new executor instance
func NewExecutor(llm core.LLMProvider, opts ...ExecutorOption) *Executor {
	executor := &Executor{
		llm:           llm,
		maxIterations: 10,
		stopWords:     []string{"TASK COMPLETE", "DONE"},
		callbacks:     make([]func(string), 0),
		messages:      make([]types.Message, 0),
		iterations:    0,
		verbose:       false,
		metadata:      make(map[string]string),
	}

	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// WithTools adds tools to the executor
func WithTools(tools ...core.Tool) ExecutorOption {
	return func(e *Executor) {
		e.tools = append(e.tools, tools...)

		// Update tool descriptions
		var descriptions []string
		var names []string
		for _, tool := range tools {
			descriptions = append(descriptions, fmt.Sprintf("- %s: %s", tool.Name(), tool.Description()))
			names = append(names, tool.Name())
		}
		e.toolsDescription = strings.Join(descriptions, "\n")
		e.toolsNames = strings.Join(names, ", ")
	}
}

// WithMaxIterations sets the maximum number of iterations
func WithMaxIterations(max int) ExecutorOption {
	return func(e *Executor) {
		e.maxIterations = max
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) ExecutorOption {
	return func(e *Executor) {
		e.verbose = verbose
	}
}

// WithStopWords sets custom stop words
func WithStopWords(words []string) ExecutorOption {
	return func(e *Executor) {
		e.stopWords = words
	}
}

// WithRolePrompt sets a custom role prompt
func WithRolePrompt(prompt string) ExecutorOption {
	return func(e *Executor) {
		e.rolePrompt = prompt
	}
}

// WithTaskPrompt sets a custom task prompt
func WithTaskPrompt(prompt string) ExecutorOption {
	return func(e *Executor) {
		e.taskPrompt = prompt
	}
}

// WithMetadata adds metadata to the executor
func WithMetadata(metadata map[string]string) ExecutorOption {
	return func(e *Executor) {
		for k, v := range metadata {
			e.metadata[k] = v
		}
	}
}

// AddCallback adds a callback function for execution events
func (e *Executor) AddCallback(callback func(string)) {
	e.callbacks = append(e.callbacks, callback)
}

// ExecuteTask executes a task with the given description and context
func (e *Executor) ExecuteTask(ctx context.Context, task *tasks.Task) error {
	// Initialize task state
	task.Status = tasks.TaskStatusRunning
	startTime := time.Now()
	task.StartedAt = &startTime

	// Build system prompt
	systemPrompt := e.buildSystemPrompt(task)
	e.messages = []types.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Execute this task: %s", task.Description)},
	}

	if e.verbose {
		e.logMessage("Starting task execution: %s", task.Description)
	}

	// Execute task iterations
	for e.iterations < e.maxIterations {
		response, err := e.step(ctx)
		if err != nil {
			task.Status = tasks.TaskStatusFailed
			task.Error = err
			return fmt.Errorf("execution step failed: %w", err)
		}

		// Check for completion
		if e.isTaskComplete(response) {
			task.Status = tasks.TaskStatusComplete
			task.Result = response
			completedTime := time.Now()
			task.CompletedAt = &completedTime
			if e.verbose {
				e.logMessage("Task completed successfully")
			}
			return nil
		}

		e.iterations++
		if e.verbose {
			e.logMessage("Completed iteration %d", e.iterations)
		}
	}

	task.Status = tasks.TaskStatusFailed
	task.Error = errors.New("reached maximum iterations without completion")
	return fmt.Errorf("reached maximum iterations (%d) without completion", e.maxIterations)
}

// step performs a single execution step
func (e *Executor) step(ctx context.Context) (string, error) {
	// Convert messages to a format suitable for Chat
	response, err := e.llm.Chat(ctx, e.messages)
	if err != nil {
		return "", fmt.Errorf("LLM chat failed: %w", err)
	}

	// Check for tool calls
	if toolCall, err := e.parseToolCall(response); err != nil {
		return "", fmt.Errorf("failed to parse tool call: %w", err)
	} else if toolCall != nil {
		result, err := e.executeTool(ctx, toolCall)
		if err != nil {
			return "", fmt.Errorf("tool execution failed: %w", err)
		}
		e.messages = append(e.messages, types.Message{
			Role:    "assistant",
			Content: response,
		}, types.Message{
			Role:    "system",
			Content: fmt.Sprintf("Tool result: %s", result),
		})
		return "", nil
	}

	return response, nil
}

// buildSystemPrompt builds the system prompt for the LLM
func (e *Executor) buildSystemPrompt(task *tasks.Task) string {
	prompt := e.rolePrompt
	if e.toolsDescription != "" {
		prompt += fmt.Sprintf("\n\nAvailable tools:\n%s", e.toolsDescription)
	}
	if task.ExpectedOutput != "" {
		prompt += fmt.Sprintf("\n\nExpected output: %s", task.ExpectedOutput)
	}
	if e.taskPrompt != "" {
		prompt += fmt.Sprintf("\n\n%s", e.taskPrompt)
	}
	return prompt
}

// isTaskComplete checks if the task is complete based on the response
func (e *Executor) isTaskComplete(response string) bool {
	for _, word := range e.stopWords {
		if strings.Contains(response, word) {
			return true
		}
	}
	return false
}

// parseToolCall attempts to parse a tool call from the LLM response
func (e *Executor) parseToolCall(response string) (*types.ToolCall, error) {
	if !strings.Contains(response, "USE_TOOL:") {
		return nil, nil
	}

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
	var tool core.Tool
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

// logMessage logs a message if verbose mode is enabled
func (e *Executor) logMessage(format string, args ...any) {
	if e.verbose {
		message := fmt.Sprintf(format, args...)
		for _, callback := range e.callbacks {
			callback(message)
		}
	}
}

// Reset resets the executor state for a new task
func (e *Executor) Reset() {
	e.messages = make([]types.Message, 0)
	e.iterations = 0
}
