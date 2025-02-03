package core

// Message represents a chat message in the conversation
type Message struct {
	Role         string // system, user, or assistant
	Content      string
	FunctionCall *FunctionCall  `json:"function_call,omitempty"`
	Images       []ImageContent `json:"images,omitempty"`
}

// FunctionCall represents a function call in the conversation
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ImageContent represents an image in the conversation
type ImageContent struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}
