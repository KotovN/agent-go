package tasks

import (
	"agentai/pkg/utils"
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Pipeline manages the execution flow of tasks
type Pipeline struct {
	tasks     []*Task
	results   map[string]string
	statuses  map[string]TaskStatus
	mutex     sync.RWMutex
	callbacks []PipelineCallback
	context   *ContextManager
	store     *OutputStore
	logger    *utils.Logger
	async     *AsyncExecutor
	planner   *TaskPlanner
	validator *TaskValidator               // New field for validation
	metrics   map[string]*ExecutionMetrics // New field for metrics
}

// PipelineCallback is called when task status changes
type PipelineCallback func(task *Task, status TaskStatus, result string)

// NewPipeline creates a new task execution pipeline
func NewPipeline(outputDir string) (*Pipeline, error) {
	store, err := NewOutputStore(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create output store: %w", err)
	}

	// Default resource limits
	resourceLimits := map[string]float64{
		"compute": 100.0,  // CPU units
		"memory":  1024.0, // Memory in MB
		"network": 1000.0, // Network ops/sec
	}

	// Create default task schema
	schema := &TaskSchema{
		RequiredFields: []string{"id", "description"},
		AllowedPriorities: []TaskPriority{
			PriorityLowest,
			PriorityLow,
			PriorityNormal,
			PriorityHigh,
			PriorityHighest,
		},
		MaxDescriptionLen: 1000,
		MaxTimeout:        time.Hour * 24,
		ResourceLimits:    resourceLimits,
	}

	p := &Pipeline{
		tasks:     make([]*Task, 0),
		results:   make(map[string]string),
		statuses:  make(map[string]TaskStatus),
		callbacks: make([]PipelineCallback, 0),
		context:   NewContextManager(),
		store:     store,
		logger:    utils.NewLogger(false),
		planner:   NewTaskPlanner(resourceLimits),
		validator: NewTaskValidator(schema),
		metrics:   make(map[string]*ExecutionMetrics),
	}

	// Initialize async executor
	p.async = NewAsyncExecutor(p, 3) // Default to 3 retries
	return p, nil
}

// AddTask adds a task to the pipeline with validation
func (p *Pipeline) AddTask(task *Task) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Validate task schema
	if errs := p.validator.ValidateSchema(task); len(errs) > 0 {
		return fmt.Errorf("task validation failed: %v", errs)
	}

	p.tasks = append(p.tasks, task)
	p.statuses[task.ID] = TaskStatusPending

	// Initialize metrics tracking
	p.metrics[task.ID] = NewExecutionMetrics(task.ID)

	// Add task to planner with estimated resources
	resources := estimateTaskResources(task)
	duration := estimateTaskDuration(task)
	dependencies := getTaskDependencies(task)

	// Validate resource usage
	if errs := p.validator.ValidateResourceUsage(resources); len(errs) > 0 {
		return fmt.Errorf("resource validation failed: %v", errs)
	}

	if err := p.planner.AddTask(task, dependencies, resources, duration); err != nil {
		p.logger.Warning("Failed to add task to planner: %v", err)
		return err
	}

	return nil
}

// AddCallback adds a callback for task status changes
func (p *Pipeline) AddCallback(callback PipelineCallback) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.callbacks = append(p.callbacks, callback)
}

// GetTaskStatus returns the current status of a task
func (p *Pipeline) GetTaskStatus(taskID string) TaskStatus {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.statuses[taskID]
}

// GetTaskResult returns the result of a completed task
func (p *Pipeline) GetTaskResult(taskID string) (string, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	result, exists := p.results[taskID]
	return result, exists
}

// updateTaskStatus updates task status and triggers callbacks
func (p *Pipeline) updateTaskStatus(task *Task, status TaskStatus, result string) {
	p.mutex.Lock()
	p.statuses[task.ID] = status
	if status == TaskStatusComplete {
		p.results[task.ID] = result
	}
	p.mutex.Unlock()

	// Trigger callbacks
	for _, callback := range p.callbacks {
		callback(task, status, result)
	}
}

// Execute runs all tasks in the pipeline with optimization
func (p *Pipeline) Execute(ctx context.Context) error {
	// Detect any dependency cycles
	if cycles := p.planner.DetectCycles(); len(cycles) > 0 {
		return fmt.Errorf("dependency cycles detected: %v", cycles)
	}

	// Optimize resource allocation
	if err := p.planner.OptimizeResources(); err != nil {
		return fmt.Errorf("resource optimization failed: %w", err)
	}

	// Get cost estimates
	costs := p.planner.EstimateCosts()
	for id, cost := range costs {
		p.logger.Info("Estimated cost for task %s: %.2f", id, cost)
	}

	// Get critical path
	criticalPath := p.planner.GetCriticalPath()
	p.logger.Info("Critical path tasks: %v", formatCriticalPath(criticalPath))

	// Execute tasks based on optimized schedule
	for _, task := range p.tasks {
		if err := p.ExecuteTask(ctx, task); err != nil {
			return fmt.Errorf("failed to execute task %s: %w", task.ID, err)
		}
	}
	return nil
}

// ExecuteAsync runs all tasks in the pipeline asynchronously
func (p *Pipeline) ExecuteAsync(ctx context.Context) (<-chan AsyncResult, error) {
	results := make(chan AsyncResult, len(p.tasks))

	// Start a goroutine to collect results
	go func() {
		defer close(results)

		// Create wait group to track task completion
		var wg sync.WaitGroup
		wg.Add(len(p.tasks))

		// Execute each task
		for _, task := range p.tasks {
			go func(t *Task) {
				defer wg.Done()

				// Execute task and forward results
				taskChan, err := p.async.ExecuteAsync(ctx, t)
				if err != nil {
					results <- AsyncResult{
						TaskID: t.ID,
						Error:  err,
					}
					return
				}

				// Forward the result
				if result := <-taskChan; result.Error != nil || result.Result != "" {
					results <- result
				}
			}(task)
		}

		// Wait for all tasks to complete
		wg.Wait()
	}()

	return results, nil
}

// ExecuteTask executes a single task with validation and metrics
func (p *Pipeline) ExecuteTask(ctx context.Context, task *Task) error {
	// Update status to in progress
	p.updateTaskStatus(task, TaskStatusRunning, "")

	// Get metrics tracker
	metrics := p.metrics[task.ID]
	if metrics == nil {
		metrics = NewExecutionMetrics(task.ID)
		p.metrics[task.ID] = metrics
	}

	// Create execution context with timeout if specified
	execCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Execute the task
	if err := Execute(execCtx, task); err != nil {
		p.updateTaskStatus(task, TaskStatusFailed, err.Error())
		return err
	}

	p.updateTaskStatus(task, TaskStatusComplete, task.Result)
	return nil
}

// ExecuteTaskAsync executes a single task asynchronously
func (p *Pipeline) ExecuteTaskAsync(ctx context.Context, task *Task) (<-chan AsyncResult, error) {
	return p.async.ExecuteAsync(ctx, task)
}

// CancelTask cancels a running task
func (p *Pipeline) CancelTask(taskID string) error {
	return p.async.CancelTask(taskID)
}

// Close cleans up pipeline resources
func (p *Pipeline) Close() error {
	p.async.Close()
	return nil
}

// GetContextManager returns the pipeline's context manager
func (p *Pipeline) GetContextManager() *ContextManager {
	return p.context
}

// ShareContext shares context between tasks
func (p *Pipeline) ShareContext(sourceTaskID, targetTaskID string, keys ...string) error {
	return p.context.ShareTaskContext(sourceTaskID, targetTaskID, keys...)
}

// GetTaskContext gets a value from a task's context
func (p *Pipeline) GetTaskContext(taskID, key string) (interface{}, bool) {
	return p.context.GetTaskContext(taskID, key)
}

// SetTaskContext sets a value in a task's context
func (p *Pipeline) SetTaskContext(taskID, key string, value interface{}) {
	p.context.SetTaskContext(taskID, key, value)
}

// GetGlobalContext gets a value from the global context
func (p *Pipeline) GetGlobalContext(key string) (interface{}, bool) {
	return p.context.GetGlobalContext(key)
}

// SetGlobalContext sets a value in the global context
func (p *Pipeline) SetGlobalContext(key string, value interface{}) {
	p.context.SetGlobalContext(key, value)
}

// GetTaskOutput retrieves a task's output from storage
func (p *Pipeline) GetTaskOutput(taskID string) (*TaskOutput, error) {
	return p.store.GetOutput(taskID)
}

// ListTaskOutputs returns all stored task outputs
func (p *Pipeline) ListTaskOutputs() []*TaskOutput {
	return p.store.ListOutputs()
}

// QueryTaskOutputs searches for outputs matching the given criteria
func (p *Pipeline) QueryTaskOutputs(criteria map[string]interface{}) []*TaskOutput {
	return p.store.QueryOutputs(criteria)
}

// DeleteTaskOutput removes a task's output from storage
func (p *Pipeline) DeleteTaskOutput(taskID string) error {
	return p.store.DeleteOutput(taskID)
}

// Helper functions

func estimateTaskResources(task *Task) map[string]float64 {
	// Basic resource estimation based on task properties
	resources := make(map[string]float64)

	// Estimate CPU usage
	complexity := calculateTaskComplexity(task)
	resources["compute"] = math.Min(100.0, complexity*10.0)

	// Estimate memory usage (in MB)
	resources["memory"] = math.Min(1024.0, float64(len(task.Description))*0.5)

	// Estimate network usage
	if requiresNetwork(task) {
		resources["network"] = 100.0
	}

	return resources
}

func estimateTaskDuration(task *Task) time.Duration {
	// Basic duration estimation based on task properties
	complexity := calculateTaskComplexity(task)

	// Convert complexity score to minutes
	minutes := math.Max(1.0, complexity*5.0)

	return time.Duration(minutes) * time.Minute
}

func getTaskDependencies(task *Task) []string {
	return task.Dependencies
}

func calculateTaskComplexity(task *Task) float64 {
	// Basic complexity scoring based on various factors
	score := 1.0

	// Factor in description length
	score += float64(len(task.Description)) * 0.01

	// Factor in priority
	score += float64(task.Priority) * 0.5

	// Factor in dependencies
	score += float64(len(task.Dependencies)) * 0.3

	// Factor in expected output complexity
	score += float64(len(task.ExpectedOutput)) * 0.01

	return score
}

func requiresNetwork(task *Task) bool {
	// Basic check for network requirements
	networkKeywords := []string{"http", "api", "url", "download", "fetch"}
	description := strings.ToLower(task.Description)

	for _, keyword := range networkKeywords {
		if strings.Contains(description, keyword) {
			return true
		}
	}

	return false
}

func formatCriticalPath(tasks []*Task) string {
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}
	return strings.Join(ids, " -> ")
}
