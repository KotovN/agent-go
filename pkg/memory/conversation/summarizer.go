package conversation

import (
	"context"
	"fmt"
	"strings"
)

// Summarizer defines the interface for conversation summarization
type Summarizer interface {
	// Summarize generates a summary of the given messages
	Summarize(ctx context.Context, messages []Message) (string, error)
}

// DefaultSummarizer provides a basic implementation of the Summarizer interface
type DefaultSummarizer struct {
	maxSummaryLength int
}

// NewDefaultSummarizer creates a new DefaultSummarizer
func NewDefaultSummarizer() *DefaultSummarizer {
	return &DefaultSummarizer{
		maxSummaryLength: 500, // Default max summary length
	}
}

// Summarize implements the Summarizer interface
func (s *DefaultSummarizer) Summarize(ctx context.Context, messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Build a basic summary by concatenating role and content snippets
	var summary strings.Builder
	summary.WriteString("Conversation summary:\n")

	for i, msg := range messages {
		if i >= 10 { // Only include last 10 messages in summary
			break
		}

		// Add role and truncated content
		content := msg.Content
		if len(content) > 100 {
			content = content[:97] + "..."
		}

		summary.WriteString(fmt.Sprintf("- %s: %s\n", msg.Role, content))
	}

	// Truncate final summary if needed
	result := summary.String()
	if len(result) > s.maxSummaryLength {
		result = result[:s.maxSummaryLength-3] + "..."
	}

	return result, nil
}

// WithMaxSummaryLength sets the maximum length for summaries
func (s *DefaultSummarizer) WithMaxSummaryLength(length int) *DefaultSummarizer {
	s.maxSummaryLength = length
	return s
}
