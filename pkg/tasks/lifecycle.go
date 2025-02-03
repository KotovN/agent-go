package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskLifecycle manages task lifecycle events and states
type TaskLifecycle struct {
	mu sync.RWMutex

	// Active tasks and their contexts
	tasks       map[string]*Task
	contexts    map[string]context.Context
	cancels     map[string]context.CancelFunc
	timeouts    map[string]time.Duration
	deadlines   map[string]time.Time
	startTimes  map[string]time.Time
	retryStates map[string]*RetryState
}

// RetryState tracks retry-related information for a task
type RetryState struct {
	Attempts     int
	MaxAttempts  int
	LastAttempt  time.Time
	NextAttempt  time.Time
	BackoffDelay time.Duration
}

// NewTaskLifecycle creates a new task lifecycle manager
func NewTaskLifecycle() *TaskLifecycle {
	return &TaskLifecycle{
		tasks:       make(map[string]*Task),
		contexts:    make(map[string]context.Context),
		cancels:     make(map[string]context.CancelFunc),
		timeouts:    make(map[string]time.Duration),
		deadlines:   make(map[string]time.Time),
		startTimes:  make(map[string]time.Time),
		retryStates: make(map[string]*RetryState),
	}
}

// RegisterTask registers a task for lifecycle management
func (l *TaskLifecycle) RegisterTask(task *Task, timeout time.Duration, maxRetries int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.tasks[task.ID]; exists {
		return fmt.Errorf("task %s is already registered", task.ID)
	}

	// Create context with timeout if specified
	ctx := context.Background()
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	// Store task information
	l.tasks[task.ID] = task
	l.contexts[task.ID] = ctx
	l.cancels[task.ID] = cancel
	l.timeouts[task.ID] = timeout
	l.startTimes[task.ID] = time.Now()

	// Initialize retry state
	l.retryStates[task.ID] = &RetryState{
		MaxAttempts:  maxRetries,
		BackoffDelay: time.Second, // Initial backoff delay
	}

	// Set deadline if timeout is specified
	if timeout > 0 {
		l.deadlines[task.ID] = time.Now().Add(timeout)
	}

	return nil
}

// GetContext returns the context for a task
func (l *TaskLifecycle) GetContext(taskID string) (context.Context, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	ctx, exists := l.contexts[taskID]
	if !exists {
		return nil, fmt.Errorf("no context found for task %s", taskID)
	}
	return ctx, nil
}

// CancelTask cancels a task and its context
func (l *TaskLifecycle) CancelTask(taskID string, reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	cancel, exists := l.cancels[taskID]
	if !exists {
		return fmt.Errorf("no cancel function found for task %s", taskID)
	}

	// Cancel the context
	cancel()

	// Update task status
	if task, exists := l.tasks[taskID]; exists {
		task.Status = TaskStatusCancelled
		if task.Error == nil {
			task.Error = fmt.Errorf(reason)
		}
	}

	return nil
}

// ExtendTimeout extends the timeout for a task
func (l *TaskLifecycle) ExtendTimeout(taskID string, extension time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	task, exists := l.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Create new context with extended timeout
	oldCancel := l.cancels[taskID]

	newCtx, newCancel := context.WithTimeout(context.Background(), extension)
	l.contexts[taskID] = newCtx
	l.cancels[taskID] = newCancel
	l.timeouts[taskID] += extension
	l.deadlines[taskID] = time.Now().Add(extension)

	// Cancel old context and cleanup
	oldCancel()
	task.Timeout = l.timeouts[taskID]

	return nil
}

// ShouldRetry determines if a task should be retried
func (l *TaskLifecycle) ShouldRetry(taskID string, err error) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state, exists := l.retryStates[taskID]
	if !exists {
		return false, 0
	}

	// Check if max attempts reached
	if state.Attempts >= state.MaxAttempts {
		return false, 0
	}

	// Check if deadline exceeded
	if deadline, hasDeadline := l.deadlines[taskID]; hasDeadline && time.Now().After(deadline) {
		return false, 0
	}

	// Update retry state
	state.Attempts++
	state.LastAttempt = time.Now()
	state.BackoffDelay *= 2 // Exponential backoff
	state.NextAttempt = state.LastAttempt.Add(state.BackoffDelay)

	return true, state.BackoffDelay
}

// GetRetryState returns the retry state for a task
func (l *TaskLifecycle) GetRetryState(taskID string) (*RetryState, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state, exists := l.retryStates[taskID]
	if !exists {
		return nil, fmt.Errorf("no retry state found for task %s", taskID)
	}
	return state, nil
}

// IsExpired checks if a task has expired (exceeded its timeout)
func (l *TaskLifecycle) IsExpired(taskID string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	deadline, hasDeadline := l.deadlines[taskID]
	if !hasDeadline {
		return false
	}
	return time.Now().After(deadline)
}

// GetTimeRemaining returns the remaining time for a task
func (l *TaskLifecycle) GetTimeRemaining(taskID string) (time.Duration, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	deadline, hasDeadline := l.deadlines[taskID]
	if !hasDeadline {
		return 0, fmt.Errorf("no deadline set for task %s", taskID)
	}

	remaining := time.Until(deadline)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// UnregisterTask removes a task from lifecycle management
func (l *TaskLifecycle) UnregisterTask(taskID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Cancel context if it exists
	if cancel, exists := l.cancels[taskID]; exists {
		cancel()
	}

	// Clean up all task-related data
	delete(l.tasks, taskID)
	delete(l.contexts, taskID)
	delete(l.cancels, taskID)
	delete(l.timeouts, taskID)
	delete(l.deadlines, taskID)
	delete(l.startTimes, taskID)
	delete(l.retryStates, taskID)
}

// Close cleans up all resources
func (l *TaskLifecycle) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Cancel all contexts
	for _, cancel := range l.cancels {
		cancel()
	}

	// Clear all maps
	l.tasks = make(map[string]*Task)
	l.contexts = make(map[string]context.Context)
	l.cancels = make(map[string]context.CancelFunc)
	l.timeouts = make(map[string]time.Duration)
	l.deadlines = make(map[string]time.Time)
	l.startTimes = make(map[string]time.Time)
	l.retryStates = make(map[string]*RetryState)
}
