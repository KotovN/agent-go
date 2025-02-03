package types

// LLMConfig represents configuration for an LLM provider
type LLMConfig struct {
	Model       string
	Temperature float64
	MaxTokens   int
	TopP        float64
	TopK        int
}
