package tasks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

var (
	// ErrQueueFull is returned when trying to add a task to a full queue
	ErrQueueFull = errors.New("priority queue is at maximum capacity")

	// ErrInvalidTask is returned when a task is nil or invalid
	ErrInvalidTask = errors.New("invalid task")

	// ErrTaskNotFound is returned when a task cannot be found in the queue
	ErrTaskNotFound = errors.New("task not found in queue")

	// ErrInvalidPriority is returned when an invalid priority value is provided
	ErrInvalidPriority = errors.New("invalid priority value")
)

// ErrorCategory represents the broad category of an error
type ErrorCategory string

const (
	// TemporaryError represents transient errors that may resolve on retry
	TemporaryError ErrorCategory = "temporary"
	// ResourceError represents errors due to resource constraints
	ResourceError ErrorCategory = "resource"
	// TimeoutError represents deadline or timeout errors
	TimeoutError ErrorCategory = "timeout"
	// ValidationError represents errors in task input/output validation
	ValidationError ErrorCategory = "validation"
	// PermissionError represents authorization/access errors
	PermissionError ErrorCategory = "permission"
	// SystemError represents internal system errors
	SystemError ErrorCategory = "system"
	// UnknownError represents unclassified errors
	UnknownError ErrorCategory = "unknown"
)

// ErrorClassifier provides error classification functionality
type ErrorClassifier struct {
	// Maps error types to categories
	categoryMap map[string]ErrorCategory
	// Custom classification functions
	classifiers []func(error) (ErrorCategory, bool)
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	ec := &ErrorClassifier{
		categoryMap: make(map[string]ErrorCategory),
		classifiers: make([]func(error) (ErrorCategory, bool), 0),
	}

	// Register default classifiers
	ec.registerDefaultClassifiers()
	return ec
}

// RegisterClassifier adds a custom error classifier
func (ec *ErrorClassifier) RegisterClassifier(fn func(error) (ErrorCategory, bool)) {
	ec.classifiers = append(ec.classifiers, fn)
}

// Classify determines the category of an error
func (ec *ErrorClassifier) Classify(err error) ErrorCategory {
	if err == nil {
		return UnknownError
	}

	// Try custom classifiers first
	for _, classifier := range ec.classifiers {
		if category, ok := classifier(err); ok {
			return category
		}
	}

	// Check if it's a context error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return TimeoutError
	}

	// Check if it's a network error
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return TimeoutError
		}
		if netErr.Temporary() {
			return TemporaryError
		}
	}

	// Check error string for common patterns
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
		return TimeoutError
	case strings.Contains(errStr, "temporary") || strings.Contains(errStr, "retry"):
		return TemporaryError
	case strings.Contains(errStr, "resource") || strings.Contains(errStr, "capacity"):
		return ResourceError
	case strings.Contains(errStr, "permission") || strings.Contains(errStr, "unauthorized"):
		return PermissionError
	case strings.Contains(errStr, "validation") || strings.Contains(errStr, "invalid"):
		return ValidationError
	case strings.Contains(errStr, "internal") || strings.Contains(errStr, "system"):
		return SystemError
	}

	return UnknownError
}

// registerDefaultClassifiers sets up the default error classification rules
func (ec *ErrorClassifier) registerDefaultClassifiers() {
	// Register common error types
	ec.categoryMap["*net.OpError"] = TemporaryError
	ec.categoryMap["*os.PathError"] = SystemError
	ec.categoryMap["*os.SyscallError"] = SystemError

	// Add classifier for validation errors
	ec.RegisterClassifier(func(err error) (ErrorCategory, bool) {
		if _, ok := err.(interface{ IsValidation() bool }); ok {
			return ValidationError, true
		}
		return "", false
	})

	// Add classifier for resource errors
	ec.RegisterClassifier(func(err error) (ErrorCategory, bool) {
		if _, ok := err.(interface{ IsResource() bool }); ok {
			return ResourceError, true
		}
		return "", false
	})
}

// ErrorWithMetadata adds context and metadata to an error
type ErrorWithMetadata struct {
	Err      error
	Category ErrorCategory
	Metadata map[string]interface{}
	Time     time.Time
}

// Error implements the error interface
func (e *ErrorWithMetadata) Error() string {
	return fmt.Sprintf("%v [category=%s, time=%v]", e.Err, e.Category, e.Time)
}

// Unwrap returns the underlying error
func (e *ErrorWithMetadata) Unwrap() error {
	return e.Err
}

// NewErrorWithMetadata creates a new error with metadata
func NewErrorWithMetadata(err error, category ErrorCategory, metadata map[string]interface{}) *ErrorWithMetadata {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	return &ErrorWithMetadata{
		Err:      err,
		Category: category,
		Metadata: metadata,
		Time:     time.Now(),
	}
}
