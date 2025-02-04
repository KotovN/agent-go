package token

import (
	"fmt"
	"strings"

	"github.com/KotovN/agent-go/pkg/types"

	"github.com/pkoukk/tiktoken-go"
)

// ModelTokenizer defines the interface for model-specific token counting
type ModelTokenizer interface {
	CountTokens(text string) (int, error)
	CountMessagesTokens(messages []types.Message) (int, error)
	GetContextSize() int
}

// Counter handles token counting for different models
type Counter struct {
	tokenizer ModelTokenizer
}

// NewCounter creates a new token counter for the specified model
func NewCounter(model string) (*Counter, error) {
	tokenizer, err := getTokenizer(model)
	if err != nil {
		return nil, err
	}
	return &Counter{tokenizer: tokenizer}, nil
}

// CountTokens counts the number of tokens in a text
func (c *Counter) CountTokens(text string) (int, error) {
	return c.tokenizer.CountTokens(text)
}

// CountMessagesTokens counts the total number of tokens in a list of messages
func (c *Counter) CountMessagesTokens(messages []types.Message) (int, error) {
	return c.tokenizer.CountMessagesTokens(messages)
}

// GetContextSize returns the maximum context size for the model
func (c *Counter) GetContextSize() int {
	return c.tokenizer.GetContextSize()
}

// getTokenizer returns the appropriate tokenizer for the model
func getTokenizer(model string) (ModelTokenizer, error) {
	switch {
	case strings.HasPrefix(model, "gpt-4"):
		return newGPTTokenizer("gpt-4"), nil
	case strings.HasPrefix(model, "gpt-3.5"):
		return newGPTTokenizer("gpt-3.5-turbo"), nil
	case strings.HasPrefix(model, "gemini"):
		return newGeminiTokenizer(), nil
	default:
		return nil, fmt.Errorf("unsupported model: %s", model)
	}
}

// GPTTokenizer implements ModelTokenizer for GPT models
type GPTTokenizer struct {
	encoding    *tiktoken.Tiktoken
	modelName   string
	contextSize int
}

func newGPTTokenizer(model string) *GPTTokenizer {
	encoding, _ := tiktoken.GetEncoding("cl100k_base")
	contextSize := 8192 // default for gpt-4
	if model == "gpt-3.5-turbo" {
		contextSize = 4096
	}
	return &GPTTokenizer{
		encoding:    encoding,
		modelName:   model,
		contextSize: contextSize,
	}
}

func (t *GPTTokenizer) CountTokens(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens), nil
}

func (t *GPTTokenizer) CountMessagesTokens(messages []types.Message) (int, error) {
	tokensPerMessage := 3 // every message follows <im_start>{role/name}\n{content}<im_end>\n
	tokensPerName := 1    // role is always required and is 1 token

	totalTokens := 0
	for _, message := range messages {
		totalTokens += tokensPerMessage
		totalTokens += tokensPerName

		tokens, err := t.CountTokens(message.Content)
		if err != nil {
			return 0, err
		}
		totalTokens += tokens
	}
	totalTokens += 3 // every reply is primed with <im_start>assistant

	return totalTokens, nil
}

func (t *GPTTokenizer) GetContextSize() int {
	return t.contextSize
}

// GeminiTokenizer implements ModelTokenizer for Gemini models
type GeminiTokenizer struct {
	contextSize int
}

func newGeminiTokenizer() *GeminiTokenizer {
	return &GeminiTokenizer{
		contextSize: 32768, // Gemini Pro default
	}
}

func (t *GeminiTokenizer) CountTokens(text string) (int, error) {
	// Gemini uses BPE tokenization similar to GPT
	// As a rough approximation, we'll use 100 tokens per 75 words
	words := len(strings.Fields(text))
	return (words * 100) / 75, nil
}

func (t *GeminiTokenizer) CountMessagesTokens(messages []types.Message) (int, error) {
	totalTokens := 0
	for _, message := range messages {
		tokens, err := t.CountTokens(message.Content)
		if err != nil {
			return 0, err
		}
		totalTokens += tokens + 4 // Add 4 tokens for message formatting
	}
	return totalTokens, nil
}

func (t *GeminiTokenizer) GetContextSize() int {
	return t.contextSize
}
