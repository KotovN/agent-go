package types

// Message represents a chat message
type Message struct {
	Role    string
	Content string
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	Tool  string
	Input string
}

// FunctionCall represents a function call in the conversation
type FunctionCall struct {
	Name      string
	Arguments string
}

// FunctionDefinition represents an OpenAI function definition
type FunctionDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ImageContent represents an image in the conversation
type ImageContent struct {
	Path        string
	Description string
}
