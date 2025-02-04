package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent-go/pkg/utils"
)

// AsyncResult represents the result of an async task execution
type AsyncResult struct {
	TaskID     string
	Result     string
	Error      error
	StartTime  time.Time
	EndTime    time.Time
	RetryCount int
}

// AsyncExecutor handles asynchronous task execution
type AsyncExecutor struct {
	pipeline   *Pipeline
	maxRetries int
	resultChan chan AsyncResult
	lifecycle  *TaskLifecycle
	mu         sync.RWMutex
	logger     *utils.Logger
	channels   *TaskChannels
	recovery   *RecoveryManager
}

// NewAsyncExecutor creates a new async executor instance
func NewAsyncExecutor(pipeline *Pipeline, maxRetries int) *AsyncExecutor {
	return &AsyncExecutor{
		pipeline:   pipeline,
		maxRetries: maxRetries,
		resultChan: make(chan AsyncResult, 100),
		lifecycle:  NewTaskLifecycle(),
		logger:     utils.NewLogger(false),
		channels:   NewTaskChannels(),
		recovery:   NewRecoveryManager(),
	}
}

// ExecuteAsync executes a task asynchronously
func (e *AsyncExecutor) ExecuteAsync(ctx context.Context, task *Task) (<-chan AsyncResult, error) {
	if task.ExecuteFunc == nil {
		return nil, fmt.Errorf("task %s has no execution function", task.ID)
	}

	// Register task with lifecycle manager
	if err := e.lifecycle.RegisterTask(task, task.Timeout, e.maxRetries); err != nil {
		return nil, fmt.Errorf("failed to register task: %w", err)
	}

	// Create channels for the task
	e.channels.CreateChannels(task.ID)

	// Start execution in goroutine
	go e.executeWithRetry(ctx, task)

	// Return the result channel
	return e.channels.GetResultChannel(task.ID)
}

// executeWithRetry handles task execution with retry logic
func (e *AsyncExecutor) executeWithRetry(ctx context.Context, task *Task) {
	var result AsyncResult
	result.TaskID = task.ID
	result.StartTime = time.Now()

	// Send initial status
	e.channels.SendStatus(task.ID, task.Status)
	e.channels.SendProgress(task.ID, 0.0)

	// Configure recovery options
	recoveryOpts := &RecoveryOptions{
		Strategy:         RetryWithBackoff,
		MaxRetries:       e.maxRetries,
		InitialBackoff:   time.Second,
		MaxBackoff:       time.Minute * 5,
		TimeoutExtension: time.Minute,
		FallbackFunc:     nil, // Can be set based on task requirements
	}

	for {
		// Get task context from lifecycle manager
		taskCtx, err := e.lifecycle.GetContext(task.ID)
		if err != nil {
			result.Error = fmt.Errorf("failed to get task context: %w", err)
			break
		}

		// Check if context is cancelled
		if taskCtx.Err() != nil {
			result.Error = taskCtx.Err()
			break
		}

		// Execute task
		taskResult, err := e.executeAttempt(taskCtx, task)
		if err == nil {
			result.Result = taskResult
			result.EndTime = time.Now()
			e.channels.SendResult(task.ID, result)
			e.channels.SendStatus(task.ID, task.Status)
			e.channels.SendProgress(task.ID, 1.0)
			break
		}

		// Handle error with recovery manager
		strategy, recoveryErr := e.recovery.HandleError(taskCtx, task, err, recoveryOpts)
		if recoveryErr != nil {
			result.Error = fmt.Errorf("recovery failed: %w", recoveryErr)
			break
		}

		// Get recovery state for logging
		state, _ := e.recovery.GetRecoveryState(task.ID)
		e.channels.SendLog(task.ID, fmt.Sprintf("Recovery attempt %d using strategy: %v",
			state.Attempts, strategy))

		// Handle different recovery strategies
		switch strategy {
		case RetryWithBackoff:
			backoff := state.NextAttempt.Sub(time.Now())
			e.channels.SendLog(task.ID, fmt.Sprintf("Retrying with backoff: %v", backoff))
			select {
			case <-taskCtx.Done():
				result.Error = taskCtx.Err()
				return
			case <-time.After(backoff):
				continue
			}

		case RetryWithTimeout:
			e.channels.SendLog(task.ID, fmt.Sprintf("Retrying with extended timeout: %v", task.Timeout))
			continue

		case RetryWithFallback:
			if recoveryOpts.FallbackFunc != nil {
				e.channels.SendLog(task.ID, "Attempting fallback execution")
				fallbackResult, fallbackErr := recoveryOpts.FallbackFunc(taskCtx)
				if fallbackErr == nil {
					result.Result = fallbackResult
					result.EndTime = time.Now()
					e.channels.SendResult(task.ID, result)
					e.channels.SendStatus(task.ID, task.Status)
					return
				}
			}
			result.Error = fmt.Errorf("fallback execution failed")
			break

		default:
			result.Error = fmt.Errorf("task execution failed: %w", err)
			break
		}

		if result.Error != nil {
			break
		}
	}

	// Send final result and cleanup
	if result.Error != nil {
		task.Fail(result.Error)
		e.channels.SendStatus(task.ID, task.Status)
		e.channels.SendResult(task.ID, result)
	}

	e.lifecycle.UnregisterTask(task.ID)
	e.recovery.ClearRecoveryState(task.ID)
	e.channels.CloseChannels(task.ID)
}

// executeAttempt executes a single attempt of the task
func (e *AsyncExecutor) executeAttempt(ctx context.Context, task *Task) (string, error) {
	// Create execution context with timeout if specified
	execCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Execute the task
	if err := Execute(execCtx, task); err != nil {
		e.channels.SendLog(task.ID, fmt.Sprintf("Execution failed: %v", err))
		return "", err
	}

	return task.Result, nil
}

// CancelTask cancels a running task
func (e *AsyncExecutor) CancelTask(taskID string) error {
	return e.lifecycle.CancelTask(taskID, "Task cancelled by user")
}

// Close cleans up resources
func (e *AsyncExecutor) Close() {
	e.lifecycle.Close()
	e.channels.CloseChannels("")
	close(e.resultChan)
}

// GetResultChannel returns the channel for receiving task results
func (e *AsyncExecutor) GetResultChannel() <-chan AsyncResult {
	return e.resultChan
}

// GetTaskChannels returns the task channels manager
func (e *AsyncExecutor) GetTaskChannels() *TaskChannels {
	return e.channels
}
