package llm

import "errors"

var (
	// ErrTokenLimitExceeded is returned when the input would exceed the model's context limit
	ErrTokenLimitExceeded = errors.New("token limit exceeded for model context window")

	// ErrRateLimitExceeded is returned when the API rate limit would be exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded for API requests")

	// ErrInvalidTokenCount is returned when token counting fails
	ErrInvalidTokenCount = errors.New("failed to count tokens in input")
)
