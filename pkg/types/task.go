package types

import (
	"agentai/pkg/tasks"
	"time"
)

// TaskProgress tracks the progress of a task
type TaskProgress struct {
	TaskID       string
	AgentID      string
	Status       tasks.TaskStatus
	Progress     float64
	Messages     []string
	StartTime    time.Time
	EndTime      *time.Time
	Requirements *TaskRequirements
}

// TaskRequirements defines what capabilities are needed for a task
type TaskRequirements struct {
	Required map[string]int
	Optional map[string]int
	Priority int
	Deadline *time.Time
}

// TaskOutput represents the complete output of a task execution
type TaskOutput struct {
	TaskID      string
	Description string
	Status      tasks.TaskStatus
	Result      string
	Context     map[string]interface{}
	Metadata    map[string]interface{}
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string
}

// NewTaskRequirements creates a new task requirements instance
func NewTaskRequirements() *TaskRequirements {
	return &TaskRequirements{
		Required: make(map[string]int),
		Optional: make(map[string]int),
	}
}
