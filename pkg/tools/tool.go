package tools

import (
	"agentai/pkg/core"
)

// BaseTool provides a base implementation of the Tool interface
type BaseTool struct {
	name        string
	description string
}

// Name returns the name of the tool
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns a description of what the tool does
func (t *BaseTool) Description() string {
	return t.description
}

// Parameters returns the parameters that the tool accepts
func (t *BaseTool) Parameters() map[string]core.ParameterDefinition {
	return make(map[string]core.ParameterDefinition)
}

// Execute executes the tool with the given input
func (t *BaseTool) Execute(input string) (string, error) {
	return "", nil
}

// NewBaseTool creates a new BaseTool instance
func NewBaseTool(name, description string) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
	}
}
