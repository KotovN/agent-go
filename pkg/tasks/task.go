package tasks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/KotovN/agent-go/pkg/capabilities"
)

// TaskPriority represents the priority level of a task
type TaskPriority int

const (
	PriorityLowest  TaskPriority = -2
	PriorityLow     TaskPriority = -1
	PriorityNormal  TaskPriority = 0
	PriorityHigh    TaskPriority = 1
	PriorityHighest TaskPriority = 2
)

// TaskOption represents a function that modifies a task
type TaskOption func(*Task)

// WithTimeout sets a timeout for task execution
func WithTimeout(timeout time.Duration) TaskOption {
	return func(t *Task) {
		t.Timeout = timeout
	}
}

// WithContext sets the context for task execution
func WithContext(context string) TaskOption {
	return func(t *Task) {
		t.Context = context
	}
}

// WithExpectedOutput sets the expected output for task validation
func WithExpectedOutput(output string) TaskOption {
	return func(t *Task) {
		t.ExpectedOutput = output
	}
}

// generateID generates a random ID for tasks
func generateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// If random generation fails, use timestamp as fallback
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// Task represents a unit of work to be executed by an agent
type Task struct {
	// Core fields
	ID             string
	Description    string
	ExpectedOutput string
	Status         TaskStatus
	Result         string
	Error          error
	ExecuteFunc    func(context.Context) (string, error)
	Context        string
	Timeout        time.Duration

	// Assignment and delegation
	AssignedAgent  string
	ParentID       string
	AsyncExecution bool

	// Timing
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time

	// Progress tracking
	Progress float64
	Messages []string

	// Additional data
	Metadata map[string]interface{}

	// Priority handling fields
	Priority      TaskPriority
	Deadline      time.Time
	Dependencies  []string // IDs of tasks that must complete before this one
	PriorityScore float64

	// Task requirements
	Requirements *capabilities.TaskRequirements
}

// NewTask creates a new task instance
func NewTask(description string, expectedOutput string) (*Task, error) {
	if description == "" {
		return nil, fmt.Errorf("task description cannot be empty")
	}

	return &Task{
		ID:             fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Description:    description,
		ExpectedOutput: expectedOutput,
		Status:         TaskStatusPending,
		Requirements:   capabilities.NewTaskRequirements(),
		Progress:       0.0,
		Metadata:       make(map[string]interface{}),
		CreatedAt:      time.Now(),
		Priority:       PriorityNormal,
		Messages:       make([]string, 0),
	}, nil
}

// WithRequirements sets task requirements
func (t *Task) WithRequirements(reqs *capabilities.TaskRequirements) *Task {
	t.Requirements = reqs
	return t
}

// WithAsyncExecution sets async execution flag
func (t *Task) WithAsyncExecution(async bool) *Task {
	t.AsyncExecution = async
	return t
}

// WithTimeout sets a timeout for task execution
func (t *Task) WithTimeout(timeout time.Duration) *Task {
	t.Timeout = timeout
	return t
}

// WithPriority sets the task priority
func (t *Task) WithPriority(priority TaskPriority) *Task {
	t.Priority = priority
	return t
}

// WithDeadline sets the task deadline
func (t *Task) WithDeadline(deadline time.Time) *Task {
	t.Deadline = deadline
	return t
}

// WithDependencies sets task dependencies
func (t *Task) WithDependencies(deps []string) *Task {
	t.Dependencies = deps
	return t
}

// WithMetadata adds metadata to the task
func (t *Task) WithMetadata(metadata map[string]interface{}) *Task {
	for k, v := range metadata {
		t.Metadata[k] = v
	}
	return t
}

// Start marks the task as started
func (t *Task) Start(agentID string) error {
	if t.Status != TaskStatusPending {
		return fmt.Errorf("task %s is not in pending state", t.ID)
	}

	now := time.Now()
	t.Status = TaskStatusRunning
	t.AssignedAgent = agentID
	t.StartedAt = &now
	return nil
}

// Complete marks the task as complete
func (t *Task) Complete(result string) error {
	if t.Status != TaskStatusRunning {
		return fmt.Errorf("task %s is not in running state", t.ID)
	}

	now := time.Now()
	t.Status = TaskStatusComplete
	t.Result = result
	t.Progress = 1.0
	t.CompletedAt = &now
	return nil
}

// Fail marks the task as failed
func (t *Task) Fail(err error) error {
	if t.Status != TaskStatusRunning {
		return fmt.Errorf("task %s is not in running state", t.ID)
	}

	now := time.Now()
	t.Status = TaskStatusFailed
	t.Error = err
	t.CompletedAt = &now
	return nil
}

// Cancel marks the task as cancelled
func (t *Task) Cancel() error {
	if t.Status == TaskStatusComplete || t.Status == TaskStatusFailed {
		return fmt.Errorf("task %s is already finished", t.ID)
	}

	now := time.Now()
	t.Status = TaskStatusCancelled
	t.CompletedAt = &now
	return nil
}

// AddMessage adds a progress message
func (t *Task) AddMessage(msg string) {
	t.Messages = append(t.Messages, msg)
}

// UpdateProgress updates task progress
func (t *Task) UpdateProgress(progress float64) error {
	if progress < 0 || progress > 1 {
		return fmt.Errorf("progress must be between 0 and 1")
	}

	t.Progress = progress
	return nil
}

// Duration returns the task duration
func (t *Task) Duration() time.Duration {
	if t.CompletedAt == nil {
		if t.StartedAt == nil {
			return 0
		}
		return time.Since(*t.StartedAt)
	}
	return t.CompletedAt.Sub(*t.StartedAt)
}

// IsComplete checks if task is complete
func (t *Task) IsComplete() bool {
	return t.Status == TaskStatusComplete
}

// IsFailed checks if task failed
func (t *Task) IsFailed() bool {
	return t.Status == TaskStatusFailed
}

// IsCancelled checks if task was cancelled
func (t *Task) IsCancelled() bool {
	return t.Status == TaskStatusCancelled
}

// IsFinished checks if task is in a terminal state
func (t *Task) IsFinished() bool {
	return t.Status == TaskStatusComplete ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusCancelled
}

// IsAsync checks if task can run asynchronously
func (t *Task) IsAsync() bool {
	return t.AsyncExecution
}

// GetResult returns task result if complete
func (t *Task) GetResult() (string, error) {
	if !t.IsComplete() {
		return "", fmt.Errorf("task %s is not complete", t.ID)
	}
	return t.Result, nil
}

// GetError returns task error if failed
func (t *Task) GetError() error {
	if !t.IsFailed() {
		return nil
	}
	return t.Error
}

// Execute executes the task using the execute function
func Execute(ctx context.Context, task *Task) error {
	if task.ExecuteFunc == nil {
		return fmt.Errorf("no execute function assigned to task")
	}

	if err := task.Start(task.AssignedAgent); err != nil {
		return err
	}

	result, err := task.ExecuteFunc(ctx)
	if err != nil {
		task.Fail(err)
		return err
	}

	return task.Complete(result)
}

// GetStatus returns the current status of the task
func GetStatus(task *Task) TaskStatus {
	return task.Status
}

// GetDuration returns the duration of task execution
func GetDuration(task *Task) time.Duration {
	return task.Duration()
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// updateStatus updates the task's status
func (t *Task) updateStatus(status TaskStatus) error {
	switch status {
	case TaskStatusPending:
		return fmt.Errorf("cannot set task back to pending state")
	case TaskStatusRunning:
		if t.Status != TaskStatusPending {
			return fmt.Errorf("task must be pending to start running")
		}
		t.StartedAt = timePtr(time.Now())
	case TaskStatusComplete:
		if t.Status != TaskStatusRunning {
			return fmt.Errorf("task must be running to complete")
		}
		now := time.Now()
		t.CompletedAt = &now
		t.Progress = 1.0
	case TaskStatusFailed:
		if t.Status != TaskStatusRunning {
			return fmt.Errorf("task must be running to fail")
		}
		now := time.Now()
		t.CompletedAt = &now
	case TaskStatusCancelled:
		if t.IsFinished() {
			return fmt.Errorf("cannot cancel finished task")
		}
		now := time.Now()
		t.CompletedAt = &now
	default:
		return fmt.Errorf("invalid task status: %s", status)
	}

	t.Status = status
	return nil
}
