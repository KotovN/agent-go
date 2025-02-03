package types

// ParameterDefinition defines a parameter for function calling
type ParameterDefinition struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"`
}

// Function represents a callable function definition for LLM function calling
type Function struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Parameters  map[string]ParameterDefinition `json:"parameters"`
	Required    []string                       `json:"required,omitempty"`
}
