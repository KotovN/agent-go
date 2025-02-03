package core

// Tool represents a capability that can be used by agents
type Tool interface {
	// Name returns the name of the tool
	Name() string

	// Description returns a description of what the tool does
	Description() string

	// Parameters returns the parameters that the tool accepts
	Parameters() map[string]ParameterDefinition

	// Execute executes the tool with the given input
	Execute(input string) (string, error)
}

// ParameterDefinition defines a parameter for function calling
type ParameterDefinition struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"`
}

// FunctionDefinition represents an OpenAI function definition
type FunctionDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Parameters  map[string]ParameterDefinition `json:"parameters"`
}

// ToFunctionDefinition converts a tool to a function definition
func ToFunctionDefinition(tool Tool) FunctionDefinition {
	return FunctionDefinition{
		Name:        tool.Name(),
		Description: tool.Description(),
		Parameters:  tool.Parameters(),
	}
}
