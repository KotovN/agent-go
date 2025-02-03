package tasks

import (
	"agentai/pkg/capabilities"
	"time"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusComplete  TaskStatus = "complete"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	// TaskStatusBlocked indicates task is blocked by dependencies
	TaskStatusBlocked TaskStatus = "blocked"
	// TaskStatusTimeout indicates task exceeded its timeout
	TaskStatusTimeout TaskStatus = "timeout"
	// TaskStatusRetrying indicates task is being retried after failure
	TaskStatusRetrying TaskStatus = "retrying"
)

// TaskProgress represents the progress of a task
type TaskProgress struct {
	// TaskID uniquely identifies the task
	TaskID string

	// AgentID identifies the agent executing the task
	AgentID string

	// Status indicates the current state
	Status TaskStatus

	// Progress is a number between 0 and 1
	Progress float64

	// Message provides additional context about the current state
	Message string

	// Error holds any error that occurred
	Error error

	// StartTime is when the task began
	StartTime time.Time

	// EndTime is when the task completed/failed
	EndTime *time.Time

	// Requirements specifies what the task needs
	Requirements *capabilities.TaskRequirements

	// Subtasks tracks progress of component tasks
	Subtasks []*TaskProgress

	// Metadata holds additional task-specific data
	Metadata map[string]interface{}
}
