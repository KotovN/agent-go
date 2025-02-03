package tasks

import (
	"sync/atomic"
	"time"
)

// ExecutionMetrics tracks task execution performance metrics
type ExecutionMetrics struct {
	TaskID        string
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	CPUUsage      float64
	MemoryUsage   float64
	NetworkUsage  float64
	ErrorCount    uint32
	RetryCount    uint32
	Status        string
	ResourceUsage map[string]float64
}

// NewExecutionMetrics creates a new metrics tracker for a task
func NewExecutionMetrics(taskID string) *ExecutionMetrics {
	return &ExecutionMetrics{
		TaskID:        taskID,
		StartTime:     time.Now(),
		ResourceUsage: make(map[string]float64),
	}
}

// Complete marks the task as complete and calculates final metrics
func (m *ExecutionMetrics) Complete() {
	m.EndTime = time.Now()
	m.Duration = m.EndTime.Sub(m.StartTime)
}

// IncrementErrors increments the error counter
func (m *ExecutionMetrics) IncrementErrors() uint32 {
	return atomic.AddUint32(&m.ErrorCount, 1)
}

// IncrementRetries increments the retry counter
func (m *ExecutionMetrics) IncrementRetries() uint32 {
	return atomic.AddUint32(&m.RetryCount, 1)
}

// UpdateResourceUsage updates resource usage metrics
func (m *ExecutionMetrics) UpdateResourceUsage(resource string, usage float64) {
	m.ResourceUsage[resource] = usage
}

// GetResourceUsage gets the current resource usage
func (m *ExecutionMetrics) GetResourceUsage() map[string]float64 {
	return m.ResourceUsage
}

// SetStatus updates the task status
func (m *ExecutionMetrics) SetStatus(status string) {
	m.Status = status
}

// GetDuration returns the task duration
func (m *ExecutionMetrics) GetDuration() time.Duration {
	if m.EndTime.IsZero() {
		return time.Since(m.StartTime)
	}
	return m.Duration
}
