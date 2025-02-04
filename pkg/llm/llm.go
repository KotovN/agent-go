package llm

import (
	"context"

	"agent-go/pkg/llm/token"
	"agent-go/pkg/types"
)

// Role represents a message role in a chat conversation
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message represents a message in a chat conversation
type Message struct {
	Role    Role
	Content string
}

// Config holds configuration for an LLM provider
type Config struct {
	Model            string
	Temperature      float32
	MaxTokens        int
	TopP             float32
	FrequencyPenalty float32
	PresencePenalty  float32
	Stop             []string
	APIKey           string
	BaseURL          string
	APIVersion       string
	Timeout          float32
	MaxRPM           int  // Maximum requests per minute
	EnableTokenCheck bool // Whether to enable token counting and limits
}

// Provider defines the interface for language model providers
type Provider interface {
	// Complete generates a completion for a prompt
	Complete(ctx context.Context, prompt string) (string, error)

	// CompleteWithFunctions generates a completion with function calling support
	CompleteWithFunctions(ctx context.Context, prompt string, functions []Function) (string, error)

	// Chat generates a response in a chat conversation
	Chat(ctx context.Context, messages []Message) (string, error)

	// GetAPIKey returns the API key for the provider
	GetAPIKey() string

	// GetConfig returns the current configuration
	GetConfig() *Config

	// SupportsFunction returns whether the provider supports function calling
	SupportsFunction() bool

	// GetContextWindowSize returns the context window size for the model
	GetContextWindowSize() int

	// GetTokenUsage returns the current token usage metrics
	GetTokenUsage() token.UsageMetrics

	// ResetTokenUsage resets the token usage metrics
	ResetTokenUsage()
}

// Function represents a function that can be called by the LLM
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// NewConfig creates a new LLM configuration with default values
func NewConfig() *Config {
	return &Config{
		Temperature:      0.7,
		MaxTokens:        2000,
		TopP:             1.0,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
		MaxRPM:           60,
		EnableTokenCheck: true,
	}
}

// BaseProvider provides common functionality for LLM providers
type BaseProvider struct {
	config      *Config
	tokenProc   *token.TokenProcess
	rateLimiter *token.RateLimiter
	counter     *token.Counter
}

// NewBaseProvider creates a new base provider with token management
func NewBaseProvider(config *Config) (*BaseProvider, error) {
	if config == nil {
		config = NewConfig()
	}

	counter, err := token.NewCounter(config.Model)
	if err != nil {
		return nil, err
	}

	return &BaseProvider{
		config:      config,
		tokenProc:   token.NewTokenProcess(),
		rateLimiter: token.NewRateLimiter(config.MaxRPM),
		counter:     counter,
	}, nil
}

// GetTokenUsage returns the current token usage metrics
func (p *BaseProvider) GetTokenUsage() token.UsageMetrics {
	return p.tokenProc.GetMetrics()
}

// ResetTokenUsage resets the token usage metrics
func (p *BaseProvider) ResetTokenUsage() {
	p.tokenProc.Reset()
}

// CheckTokenLimit checks if the input would exceed the model's context limit
func (p *BaseProvider) CheckTokenLimit(messages []Message) error {
	if !p.config.EnableTokenCheck {
		return nil
	}

	// Convert messages to types.Message
	typeMessages := make([]types.Message, len(messages))
	for i, msg := range messages {
		typeMessages[i] = types.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	count, err := p.counter.CountMessagesTokens(typeMessages)
	if err != nil {
		return err
	}

	if count > p.counter.GetContextSize() {
		return ErrTokenLimitExceeded
	}

	return nil
}

// WaitForRateLimit waits if necessary to respect the rate limit
func (p *BaseProvider) WaitForRateLimit() {
	p.rateLimiter.CheckOrWait()
}

// Stop stops the provider's background processes
func (p *BaseProvider) Stop() {
	p.rateLimiter.Stop()
}
