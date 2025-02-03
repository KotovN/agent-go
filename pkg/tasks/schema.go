package tasks

import (
	"fmt"
	"time"
)

// TaskSchema defines validation rules for tasks
type TaskSchema struct {
	RequiredFields    []string
	AllowedPriorities []TaskPriority
	MaxDescriptionLen int
	MaxTimeout        time.Duration
	ResourceLimits    map[string]float64
}

// TaskValidator handles task validation
type TaskValidator struct {
	schema *TaskSchema
}

// NewTaskValidator creates a new task validator
func NewTaskValidator(schema *TaskSchema) *TaskValidator {
	return &TaskValidator{
		schema: schema,
	}
}

// ValidateSchema checks if a task meets schema requirements
func (v *TaskValidator) ValidateSchema(task *Task) []error {
	var errs []error

	// Check required fields
	for _, field := range v.schema.RequiredFields {
		switch field {
		case "id":
			if task.ID == "" {
				errs = append(errs, fmt.Errorf("task ID is required"))
			}
		case "description":
			if task.Description == "" {
				errs = append(errs, fmt.Errorf("task description is required"))
			}
		}
	}

	// Check description length
	if len(task.Description) > v.schema.MaxDescriptionLen {
		errs = append(errs, fmt.Errorf("description exceeds maximum length of %d", v.schema.MaxDescriptionLen))
	}

	// Check timeout
	if task.Timeout > v.schema.MaxTimeout {
		errs = append(errs, fmt.Errorf("timeout exceeds maximum of %v", v.schema.MaxTimeout))
	}

	// Check priority
	if task.Priority != TaskPriority(PriorityNormal) {
		validPriority := false
		for _, p := range v.schema.AllowedPriorities {
			if TaskPriority(p) == task.Priority {
				validPriority = true
				break
			}
		}
		if !validPriority {
			errs = append(errs, fmt.Errorf("invalid priority: %v", task.Priority))
		}
	}

	return errs
}

// ValidateResourceUsage checks if resource usage is within limits
func (v *TaskValidator) ValidateResourceUsage(resources map[string]float64) []error {
	var errs []error

	for resource, usage := range resources {
		if limit, exists := v.schema.ResourceLimits[resource]; exists {
			if usage > limit {
				errs = append(errs, fmt.Errorf("resource %s usage %.2f exceeds limit %.2f", resource, usage, limit))
			}
		}
	}

	return errs
}

// ValidatePerformance checks if task performance meets requirements
func (v *TaskValidator) ValidatePerformance(metrics *ExecutionMetrics) []error {
	var errs []error

	// Add performance validation logic here
	// For example, check execution time, resource usage, etc.

	return errs
}

// ValidateIOContract checks if task input/output meets requirements
func (v *TaskValidator) ValidateIOContract(taskID string, context interface{}, result string) []error {
	var errs []error

	// Add I/O contract validation logic here
	// For example, check result format, required fields, etc.

	return errs
}
