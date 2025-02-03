package monitoring

import (
	"fmt"
	"sync"
	"time"

	"agentai/pkg/capabilities"
	"agentai/pkg/tasks"
)

// MetricType represents the type of metric being tracked
type MetricType string

const (
	MetricTypeTokens   MetricType = "tokens"
	MetricTypeLatency  MetricType = "latency"
	MetricTypeErrors   MetricType = "errors"
	MetricTypeProgress MetricType = "progress"
	MetricTypeResource MetricType = "resource"
)

// Metric represents a monitored value
type Metric struct {
	Type      MetricType
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
}

// Monitor manages task monitoring and metrics collection
type Monitor struct {
	mu sync.RWMutex

	// Active tasks being monitored
	tasks map[string]*tasks.Task

	// Metrics collected for each task
	metrics map[string][]Metric

	// Callbacks for status changes
	statusCallbacks []func(string, tasks.TaskStatus)

	// Callbacks for progress updates
	progressCallbacks []func(string, float64)

	// Callbacks for metric updates
	metricCallbacks []func(string, Metric)
}

// NewMonitor creates a new monitor instance
func NewMonitor() *Monitor {
	return &Monitor{
		tasks:   make(map[string]*tasks.Task),
		metrics: make(map[string][]Metric),
	}
}

// StartMonitoring begins monitoring a task
func (m *Monitor) StartMonitoring(task *tasks.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[task.ID]; exists {
		return fmt.Errorf("task %s is already being monitored", task.ID)
	}

	m.tasks[task.ID] = task
	m.metrics[task.ID] = make([]Metric, 0)

	// Record initial progress metric
	m.recordMetric(task.ID, Metric{
		Type:      MetricTypeProgress,
		Value:     0.0,
		Timestamp: time.Now(),
		Labels:    map[string]string{"status": string(task.Status)},
	})

	return nil
}

// StopMonitoring stops monitoring a task
func (m *Monitor) StopMonitoring(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return fmt.Errorf("task %s is not being monitored", taskID)
	}

	delete(m.tasks, taskID)
	delete(m.metrics, taskID)
	return nil
}

// UpdateTaskStatus updates task status and notifies callbacks
func (m *Monitor) UpdateTaskStatus(taskID string, status tasks.TaskStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s is not being monitored", taskID)
	}

	// Record status change metric
	m.recordMetric(taskID, Metric{
		Type:      MetricTypeProgress,
		Value:     task.Progress,
		Timestamp: time.Now(),
		Labels:    map[string]string{"status": string(status)},
	})

	// Notify callbacks
	for _, callback := range m.statusCallbacks {
		go callback(taskID, status)
	}

	return nil
}

// UpdateTaskProgress updates task progress and notifies callbacks
func (m *Monitor) UpdateTaskProgress(taskID string, progress float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return fmt.Errorf("task %s is not being monitored", taskID)
	}

	// Record progress metric
	m.recordMetric(taskID, Metric{
		Type:      MetricTypeProgress,
		Value:     progress,
		Timestamp: time.Now(),
	})

	// Notify callbacks
	for _, callback := range m.progressCallbacks {
		go callback(taskID, progress)
	}

	return nil
}

// RecordMetric records a metric for a task
func (m *Monitor) RecordMetric(taskID string, metric Metric) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return fmt.Errorf("task %s is not being monitored", taskID)
	}

	m.recordMetric(taskID, metric)
	return nil
}

// recordMetric adds a metric to the task's metrics list
func (m *Monitor) recordMetric(taskID string, metric Metric) {
	m.metrics[taskID] = append(m.metrics[taskID], metric)

	// Notify metric callbacks
	for _, callback := range m.metricCallbacks {
		go callback(taskID, metric)
	}
}

// GetTaskMetrics returns all metrics for a task
func (m *Monitor) GetTaskMetrics(taskID string) ([]Metric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s is not being monitored", taskID)
	}

	return metrics, nil
}

// GetTaskMetricsByType returns metrics of a specific type for a task
func (m *Monitor) GetTaskMetricsByType(taskID string, metricType MetricType) ([]Metric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s is not being monitored", taskID)
	}

	filtered := make([]Metric, 0)
	for _, metric := range metrics {
		if metric.Type == metricType {
			filtered = append(filtered, metric)
		}
	}

	return filtered, nil
}

// AddStatusCallback registers a callback for task status changes
func (m *Monitor) AddStatusCallback(callback func(string, tasks.TaskStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCallbacks = append(m.statusCallbacks, callback)
}

// AddProgressCallback registers a callback for progress updates
func (m *Monitor) AddProgressCallback(callback func(string, float64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCallbacks = append(m.progressCallbacks, callback)
}

// AddMetricCallback registers a callback for metric updates
func (m *Monitor) AddMetricCallback(callback func(string, Metric)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricCallbacks = append(m.metricCallbacks, callback)
}

// GetActiveTaskCount returns the number of tasks being monitored
func (m *Monitor) GetActiveTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}

// GetTaskProgress returns the current progress of a task
func (m *Monitor) GetTaskProgress(taskID string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return 0, fmt.Errorf("task %s is not being monitored", taskID)
	}

	return task.Progress, nil
}

// GetTaskStatus returns the current status of a task
func (m *Monitor) GetTaskStatus(taskID string) (tasks.TaskStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return "", fmt.Errorf("task %s is not being monitored", taskID)
	}

	return task.Status, nil
}

// TaskMonitor manages task progress tracking
type TaskMonitor struct {
	tasks map[string]*tasks.TaskProgress
	mu    sync.RWMutex

	// Callbacks for task status changes
	statusCallbacks []func(string, tasks.TaskStatus)

	// Callbacks for progress updates
	progressCallbacks []func(string, float64)
}

// NewTaskMonitor creates a new task monitor instance
func NewTaskMonitor() *TaskMonitor {
	return &TaskMonitor{
		tasks: make(map[string]*tasks.TaskProgress),
	}
}

// StartTask begins tracking a new task
func (tm *TaskMonitor) StartTask(taskID string, agentID string, reqs *capabilities.TaskRequirements) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tasks[taskID]; exists {
		return fmt.Errorf("task %s already exists", taskID)
	}

	tm.tasks[taskID] = &tasks.TaskProgress{
		TaskID:       taskID,
		AgentID:      agentID,
		Status:       tasks.TaskStatusRunning,
		Progress:     0.0,
		StartTime:    time.Now(),
		Requirements: reqs,
		Metadata:     make(map[string]interface{}),
	}

	// Notify status callbacks
	tm.notifyStatusChange(taskID, tasks.TaskStatusRunning)
	return nil
}

// UpdateProgress updates the progress of a task
func (tm *TaskMonitor) UpdateProgress(taskID string, progress float64, message string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Progress = progress
	task.Message = message

	// Notify progress callbacks
	tm.notifyProgressUpdate(taskID, progress)
	return nil
}

// CompleteTask marks a task as complete
func (tm *TaskMonitor) CompleteTask(taskID string, message string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.Status = tasks.TaskStatusComplete
	task.Progress = 1.0
	task.Message = message
	task.EndTime = &now

	// Notify status callbacks
	tm.notifyStatusChange(taskID, tasks.TaskStatusComplete)

	// Clean up completed task
	delete(tm.tasks, taskID)
	return nil
}

// FailTask marks a task as failed
func (tm *TaskMonitor) FailTask(taskID string, err error) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.Status = tasks.TaskStatusFailed
	task.Error = err
	task.EndTime = &now

	// Notify status callbacks
	tm.notifyStatusChange(taskID, tasks.TaskStatusFailed)

	// Clean up failed task
	delete(tm.tasks, taskID)
	return nil
}

// CancelTask marks a task as cancelled
func (tm *TaskMonitor) CancelTask(taskID string, reason string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.Status = tasks.TaskStatusCancelled
	task.Message = reason
	task.EndTime = &now

	// Notify status callbacks
	tm.notifyStatusChange(taskID, tasks.TaskStatusCancelled)

	// Clean up cancelled task
	delete(tm.tasks, taskID)
	return nil
}

// GetTaskProgress retrieves the current progress of a task
func (tm *TaskMonitor) GetTaskProgress(taskID string) (*tasks.TaskProgress, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// AddStatusCallback registers a callback for task status changes
func (tm *TaskMonitor) AddStatusCallback(callback func(string, tasks.TaskStatus)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.statusCallbacks = append(tm.statusCallbacks, callback)
}

// AddProgressCallback registers a callback for progress updates
func (tm *TaskMonitor) AddProgressCallback(callback func(string, float64)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.progressCallbacks = append(tm.progressCallbacks, callback)
}

// notifyStatusChange notifies all status callbacks
func (tm *TaskMonitor) notifyStatusChange(taskID string, status tasks.TaskStatus) {
	for _, callback := range tm.statusCallbacks {
		go callback(taskID, status)
	}
}

// notifyProgressUpdate notifies all progress callbacks
func (tm *TaskMonitor) notifyProgressUpdate(taskID string, progress float64) {
	for _, callback := range tm.progressCallbacks {
		go callback(taskID, progress)
	}
}

// GetActiveTasks returns all tasks that are currently running
func (tm *TaskMonitor) GetActiveTasks() []*tasks.TaskProgress {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var active []*tasks.TaskProgress
	for _, task := range tm.tasks {
		if task.Status == tasks.TaskStatusRunning {
			active = append(active, task)
		}
	}
	return active
}

// GetTasksByAgent returns all tasks assigned to a specific agent
func (tm *TaskMonitor) GetTasksByAgent(agentID string) []*tasks.TaskProgress {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var agentTasks []*tasks.TaskProgress
	for _, task := range tm.tasks {
		if task.AgentID == agentID {
			agentTasks = append(agentTasks, task)
		}
	}
	return agentTasks
}

// AddSubtask adds a subtask to a parent task
func (tm *TaskMonitor) AddSubtask(parentID string, subtaskID string, agentID string, reqs *capabilities.TaskRequirements) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	parent, exists := tm.tasks[parentID]
	if !exists {
		return fmt.Errorf("parent task %s not found", parentID)
	}

	subtask := &tasks.TaskProgress{
		TaskID:       subtaskID,
		AgentID:      agentID,
		Status:       tasks.TaskStatusRunning,
		Progress:     0.0,
		StartTime:    time.Now(),
		Requirements: reqs,
		Metadata:     make(map[string]interface{}),
	}

	parent.Subtasks = append(parent.Subtasks, subtask)
	tm.tasks[subtaskID] = subtask

	// Notify status callbacks
	tm.notifyStatusChange(subtaskID, tasks.TaskStatusRunning)
	return nil
}

// UpdateTaskMetadata updates the metadata for a task
func (tm *TaskMonitor) UpdateTaskMetadata(taskID string, metadata map[string]interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Update metadata
	for k, v := range metadata {
		task.Metadata[k] = v
	}

	return nil
}
