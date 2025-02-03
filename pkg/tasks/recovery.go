package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RecoveryStrategy defines how to handle task failures
type RecoveryStrategy int

const (
	// RetryWithBackoff retries the task with exponential backoff
	RetryWithBackoff RecoveryStrategy = iota
	// RetryWithTimeout retries the task with a new timeout
	RetryWithTimeout
	// RetryWithFallback retries the task with fallback options
	RetryWithFallback
	// AbortExecution stops execution and reports failure
	AbortExecution
)

// RecoveryOptions configures how recovery should be handled
type RecoveryOptions struct {
	Strategy         RecoveryStrategy
	MaxRetries       int
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	TimeoutExtension time.Duration
	FallbackFunc     func(context.Context) (string, error)
}

// RecoveryManager handles task recovery and error handling
type RecoveryManager struct {
	mu sync.RWMutex

	// Active recovery attempts
	recoveries map[string]*RecoveryState

	// Default options
	defaultOptions *RecoveryOptions

	// Error classification
	classifier *ErrorClassifier

	// Recovery metrics
	metrics *RecoveryMetrics
}

// RecoveryState tracks the state of a recovery attempt
type RecoveryState struct {
	TaskID       string
	Strategy     RecoveryStrategy
	Attempts     int
	LastError    error
	LastAttempt  time.Time
	NextAttempt  time.Time
	BackoffDelay time.Duration
	FallbackUsed bool
}

// NewRecoveryManager creates a new recovery manager instance
func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		recoveries: make(map[string]*RecoveryState),
		defaultOptions: &RecoveryOptions{
			Strategy:         RetryWithBackoff,
			MaxRetries:       3,
			InitialBackoff:   time.Second,
			MaxBackoff:       time.Minute * 5,
			TimeoutExtension: time.Minute,
		},
		classifier: NewErrorClassifier(),
		metrics:    NewRecoveryMetrics(),
	}
}

// HandleError determines and executes the appropriate recovery strategy
func (r *RecoveryManager) HandleError(ctx context.Context, task *Task, err error, opts *RecoveryOptions) (RecoveryStrategy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Use default options if none provided
	if opts == nil {
		opts = r.defaultOptions
	}

	// Get or create recovery state
	state, exists := r.recoveries[task.ID]
	if !exists {
		state = &RecoveryState{
			TaskID:       task.ID,
			Strategy:     opts.Strategy,
			BackoffDelay: opts.InitialBackoff,
		}
		r.recoveries[task.ID] = state
	}

	// Update state
	state.LastError = err
	state.LastAttempt = time.Now()
	state.Attempts++

	// Check if max retries exceeded
	if state.Attempts > opts.MaxRetries {
		delete(r.recoveries, task.ID)
		return AbortExecution, fmt.Errorf("max retries exceeded: %w", err)
	}

	// Classify error and determine strategy
	errorCategory := r.classifier.Classify(err)
	strategy := r.determineStrategy(ctx, task, err, state, errorCategory)
	state.Strategy = strategy

	// Create recovery attempt for metrics
	attempt := &RecoveryAttempt{
		TaskID:        task.ID,
		Strategy:      strategy,
		ErrorCategory: errorCategory,
		StartTime:     time.Now(),
		AttemptNumber: state.Attempts,
		ErrorDetails:  err.Error(),
		MetadataSnapshot: map[string]interface{}{
			"task_priority": task.Priority,
			"retry_count":   state.Attempts,
			"backoff":       state.BackoffDelay.String(),
		},
	}

	// Execute recovery strategy
	var recoveryErr error
	switch strategy {
	case RetryWithBackoff:
		_, recoveryErr = r.handleBackoffRetry(state, opts)
	case RetryWithTimeout:
		_, recoveryErr = r.handleTimeoutRetry(ctx, task, state, opts)
	case RetryWithFallback:
		_, recoveryErr = r.handleFallback(ctx, task, state, opts)
	default:
		delete(r.recoveries, task.ID)
		recoveryErr = fmt.Errorf("unrecoverable error: %w", err)
	}

	// Record metrics
	attempt.EndTime = time.Now()
	attempt.Duration = attempt.EndTime.Sub(attempt.StartTime)
	attempt.Successful = recoveryErr == nil
	attempt.BackoffDuration = state.BackoffDelay
	r.metrics.RecordAttempt(attempt)

	if recoveryErr != nil {
		return strategy, recoveryErr
	}
	return strategy, nil
}

// determineStrategy selects the best recovery strategy based on the error
func (r *RecoveryManager) determineStrategy(ctx context.Context, task *Task, err error, state *RecoveryState, category ErrorCategory) RecoveryStrategy {
	// Check if context cancelled or deadline exceeded
	if ctx.Err() != nil {
		return RetryWithTimeout
	}

	// If we've already tried timeout extension, try fallback
	if state.Strategy == RetryWithTimeout && state.Attempts > 1 {
		return RetryWithFallback
	}

	// Choose strategy based on error category
	switch category {
	case TimeoutError:
		return RetryWithTimeout
	case TemporaryError:
		return RetryWithBackoff
	case ResourceError:
		// For resource errors, check success rate with backoff
		if r.metrics.GetCategorySuccessRate(ResourceError) > 0.5 {
			return RetryWithBackoff
		}
		return RetryWithFallback
	case ValidationError:
		// Validation errors likely need fallback
		return RetryWithFallback
	case SystemError:
		// System errors might resolve with backoff
		return RetryWithBackoff
	default:
		// For unknown errors, use fallback if available
		if !state.FallbackUsed {
			return RetryWithFallback
		}
		return AbortExecution
	}
}

// handleBackoffRetry implements exponential backoff retry
func (r *RecoveryManager) handleBackoffRetry(state *RecoveryState, opts *RecoveryOptions) (RecoveryStrategy, error) {
	// Calculate next backoff delay with exponential increase
	state.BackoffDelay *= 2
	if state.BackoffDelay > opts.MaxBackoff {
		state.BackoffDelay = opts.MaxBackoff
	}

	state.NextAttempt = state.LastAttempt.Add(state.BackoffDelay)
	return RetryWithBackoff, nil
}

// handleTimeoutRetry implements retry with extended timeout
func (r *RecoveryManager) handleTimeoutRetry(ctx context.Context, task *Task, state *RecoveryState, opts *RecoveryOptions) (RecoveryStrategy, error) {
	// Extend task timeout
	task.Timeout += opts.TimeoutExtension
	state.NextAttempt = time.Now()
	return RetryWithTimeout, nil
}

// handleFallback implements fallback execution
func (r *RecoveryManager) handleFallback(ctx context.Context, task *Task, state *RecoveryState, opts *RecoveryOptions) (RecoveryStrategy, error) {
	if opts.FallbackFunc == nil {
		return AbortExecution, fmt.Errorf("no fallback function provided")
	}

	state.FallbackUsed = true
	state.NextAttempt = time.Now()
	return RetryWithFallback, nil
}

// GetRecoveryState returns the current recovery state for a task
func (r *RecoveryManager) GetRecoveryState(taskID string) (*RecoveryState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.recoveries[taskID]
	return state, exists
}

// ClearRecoveryState removes recovery state for a task
func (r *RecoveryManager) ClearRecoveryState(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.recoveries, taskID)
}

// GetMetrics returns the recovery metrics
func (r *RecoveryManager) GetMetrics() *RecoveryMetrics {
	return r.metrics
}

// RegisterErrorClassifier adds a custom error classifier
func (r *RecoveryManager) RegisterErrorClassifier(fn func(error) (ErrorCategory, bool)) {
	r.classifier.RegisterClassifier(fn)
}

// isTemporaryError determines if an error is temporary and retriable
func isTemporaryError(err error) bool {
	// Add logic to identify temporary errors
	return false
}

// SetDefaultOptions updates the default recovery options
func (r *RecoveryManager) SetDefaultOptions(opts *RecoveryOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaultOptions = opts
}

// GetDefaultOptions returns the current default recovery options
func (r *RecoveryManager) GetDefaultOptions() *RecoveryOptions {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.defaultOptions
}
