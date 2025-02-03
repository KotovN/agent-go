package tools

import (
	"agentai/pkg/agents"
	"agentai/pkg/core"
	"agentai/pkg/tasks"
	"agentai/pkg/team"
	"context"
	"encoding/json"
	"fmt"
)

// DelegationTool allows agents to delegate tasks to other agents
type DelegationTool struct {
	agents    map[string]*agents.Agent
	team      *team.Team
	delegator *agents.Agent // The agent using this tool
}

// NewDelegationTool creates a new delegation tool instance
func NewDelegationTool(team *team.Team, delegator *agents.Agent) *DelegationTool {
	// Create map of agents by name for quick lookup
	agentMap := make(map[string]*agents.Agent)
	for _, agent := range team.Agents {
		agentMap[agent.Name] = agent
	}

	return &DelegationTool{
		agents:    agentMap,
		team:      team,
		delegator: delegator,
	}
}

func (t *DelegationTool) Name() string {
	return "delegate"
}

func (t *DelegationTool) Description() string {
	return "Delegate a task to another agent with specific expertise. The delegated agent will execute the task and return the result."
}

type DelegationInput struct {
	AgentName      string `json:"agent_name"`
	Task           string `json:"task"`
	Context        string `json:"context,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
}

func (t *DelegationTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"agent_name": {
			Type:        "string",
			Description: "The name of the agent to delegate the task to",
			Required:    true,
		},
		"task": {
			Type:        "string",
			Description: "The task to delegate to the agent",
			Required:    true,
		},
		"context": {
			Type:        "string",
			Description: "Additional context for the task",
			Required:    false,
		},
		"expected_output": {
			Type:        "string",
			Description: "Expected output format or requirements",
			Required:    false,
		},
	}
}

func (t *DelegationTool) Execute(input string) (string, error) {
	var params DelegationInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %v", err)
	}

	if params.AgentName == "" || params.Task == "" {
		return "", fmt.Errorf("both agent_name and task parameters are required")
	}

	// Find the target agent
	targetAgent, exists := t.agents[params.AgentName]
	if !exists {
		return "", fmt.Errorf("agent '%s' not found", params.AgentName)
	}

	// Create delegation context
	delegationContext := fmt.Sprintf("Delegated by: %s (%s)", t.delegator.Name, t.delegator.Role)
	if params.Context != "" {
		delegationContext += fmt.Sprintf("\n\nAdditional Context:\n%s", params.Context)
	}

	// Create and assign the task
	task, err := t.team.CreateTask(params.Task, params.ExpectedOutput)
	if err != nil {
		return "", fmt.Errorf("failed to create delegated task: %v", err)
	}

	// Set delegation metadata
	t.team.Pipeline().SetTaskContext(task.ID, "delegated_by", t.delegator.Name)
	t.team.Pipeline().SetTaskContext(task.ID, "delegator_role", t.delegator.Role)
	t.team.Pipeline().SetTaskContext(task.ID, "delegation_context", delegationContext)

	// Assign and execute the task
	if err := t.team.AssignTask(context.Background(), task, targetAgent); err != nil {
		return "", fmt.Errorf("failed to assign task to agent: %v", err)
	}

	// Execute the task
	if err := tasks.Execute(context.Background(), task); err != nil {
		return "", fmt.Errorf("delegated task execution failed: %v", err)
	}

	if task.Error != nil {
		return "", fmt.Errorf("delegated task failed: %s", task.Error)
	}

	return fmt.Sprintf("Task successfully delegated and executed by %s. Result: %s", targetAgent.Name, task.Result), nil
}
