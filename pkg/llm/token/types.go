package token

// UsageMetrics tracks token usage statistics
type UsageMetrics struct {
	TotalTokens        int `json:"total_tokens"`
	PromptTokens       int `json:"prompt_tokens"`
	CachedPromptTokens int `json:"cached_prompt_tokens"`
	CompletionTokens   int `json:"completion_tokens"`
	SuccessfulRequests int `json:"successful_requests"`
}

// TokenProcess manages token counting and tracking
type TokenProcess struct {
	metrics UsageMetrics
}

// NewTokenProcess creates a new token process instance
func NewTokenProcess() *TokenProcess {
	return &TokenProcess{
		metrics: UsageMetrics{},
	}
}

// AddPromptTokens adds tokens used in prompts
func (tp *TokenProcess) AddPromptTokens(tokens int) {
	tp.metrics.PromptTokens += tokens
	tp.metrics.TotalTokens += tokens
}

// AddCompletionTokens adds tokens used in completions
func (tp *TokenProcess) AddCompletionTokens(tokens int) {
	tp.metrics.CompletionTokens += tokens
	tp.metrics.TotalTokens += tokens
}

// AddCachedPromptTokens adds cached prompt tokens
func (tp *TokenProcess) AddCachedPromptTokens(tokens int) {
	tp.metrics.CachedPromptTokens += tokens
}

// AddSuccessfulRequest increments the successful requests counter
func (tp *TokenProcess) AddSuccessfulRequest() {
	tp.metrics.SuccessfulRequests++
}

// GetMetrics returns the current usage metrics
func (tp *TokenProcess) GetMetrics() UsageMetrics {
	return tp.metrics
}

// Reset resets all metrics to zero
func (tp *TokenProcess) Reset() {
	tp.metrics = UsageMetrics{}
}
