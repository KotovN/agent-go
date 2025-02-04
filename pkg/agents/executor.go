package agents

import (
	"context"
	"fmt"
	"strings"

	"agent-go/pkg/core"
	"agent-go/pkg/execution"
	"agent-go/pkg/tasks"
)

// AgentExecutor wraps the core executor with agent-specific functionality
type AgentExecutor struct {
	agent    *Agent
	executor *execution.Executor
}

// NewAgentExecutor creates a new agent executor instance
func NewAgentExecutor(agent *Agent, tools []core.Tool, opts ...execution.ExecutorOption) *AgentExecutor {
	// Build agent-specific role prompt
	rolePrompt := fmt.Sprintf(`You are %s, with the role of %s.
Your goals are:
%s

Your background knowledge:
%s`, agent.Name, agent.Role, formatGoals(agent.Goals), agent.KnowledgeSources)

	// Create base options
	baseOpts := []execution.ExecutorOption{
		execution.WithRolePrompt(rolePrompt),
		execution.WithTools(tools...),
		execution.WithMetadata(map[string]string{
			"agent_name": agent.Name,
			"agent_role": agent.Role,
		}),
	}

	// Combine with user options
	allOpts := append(baseOpts, opts...)

	// Create core executor
	executor := execution.NewExecutor(agent.LLMProvider, allOpts...)

	return &AgentExecutor{
		agent:    agent,
		executor: executor,
	}
}

// ExecuteTask executes a specific task with context
func (e *AgentExecutor) ExecuteTask(ctx context.Context, taskDescription string, expectedOutput string, taskContext string) (string, error) {
	// Create task
	task := &tasks.Task{
		Description:    taskDescription,
		ExpectedOutput: expectedOutput,
		Metadata: map[string]interface{}{
			"context": taskContext,
		},
	}

	// Execute task
	err := e.executor.ExecuteTask(ctx, task)
	if err != nil {
		return "", err
	}

	return task.Result, nil
}

// formatGoals formats agent goals for the prompt
func formatGoals(goals []string) string {
	var formatted string
	for i, goal := range goals {
		formatted += fmt.Sprintf("%d. %s\n", i+1, goal)
	}
	return formatted
}

// formatTools formats tool descriptions for the prompt
func formatTools(tools []core.Tool) string {
	var formatted string
	for _, tool := range tools {
		formatted += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
	}
	return formatted
}

// formatToolNames formats tool names for the prompt
func formatToolNames(tools []core.Tool) string {
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	return strings.Join(names, ", ")
}
