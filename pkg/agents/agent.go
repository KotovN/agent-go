package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agentai/pkg/capabilities"
	"agentai/pkg/core"
	"agentai/pkg/memory"
	"agentai/pkg/tasks"
	"agentai/pkg/types"
	"agentai/pkg/utils"
)

var slog = utils.NewLogger(true)

// AgentConfig holds configuration for an agent
type AgentConfig struct {
	AllowDelegation    bool
	AllowCodeExecution bool
	CodeExecutionMode  string // "safe" or "unsafe"
	Multimodal         bool
	MaxRetryLimit      int
}

// KnowledgeSource represents a source of background knowledge
type KnowledgeSource struct {
	Content  string
	Metadata map[string]interface{}
}

// String returns a string representation of the knowledge source
func (k KnowledgeSource) String() string {
	var result string

	// Add metadata if present
	if len(k.Metadata) > 0 {
		var metadataParts []string
		for key, value := range k.Metadata {
			metadataParts = append(metadataParts, fmt.Sprintf("%s: %v", key, value))
		}
		result += fmt.Sprintf("[%s] ", strings.Join(metadataParts, ", "))
	}

	result += k.Content
	return result
}

// DelegationType represents how a task can be delegated
type DelegationType string

const (
	// DirectDelegation - agent can directly execute the task
	DirectDelegation DelegationType = "direct"
	// CollaborativeDelegation - agent needs to collaborate with others
	CollaborativeDelegation DelegationType = "collaborative"
	// SupervisedDelegation - agent needs supervision
	SupervisedDelegation DelegationType = "supervised"
)

// DelegationPreference represents an agent's delegation preferences
type DelegationPreference struct {
	PreferredTypes    []DelegationType
	CollaborationTags []string
	SupervisorRoles   []string
	ExcludedTasks     []string
}

// AssignedTaskPriority represents the priority level of a task
type AssignedTaskPriority struct {
	Priority     tasks.TaskPriority
	Deadline     time.Time
	Importance   float64
	Urgency      float64
	Dependencies []string
}

// PriorityQueue manages tasks based on priority
type PriorityQueue struct {
	Tasks    []*tasks.Task
	Capacity int
	Strategy string // "fifo", "deadline", "importance", "balanced"
}

// DelegationResult represents the result of a task delegation
type DelegationResult struct {
	Success      bool
	AgentID      string
	TaskID       string
	ErrorMessage string
	Feedback     string
}

// DelegationStats tracks delegation performance
type DelegationStats struct {
	TotalDelegations      int
	SuccessfulDelegations int
	FailedDelegations     int
	AverageSuccessRate    float64
	PreferredAgents       map[string]int
	TaskTypes             map[string]int
}

// FeedbackMetrics tracks agent performance metrics
type FeedbackMetrics struct {
	TasksCompleted        int
	TasksFailed           int
	AverageCompletionTime time.Duration
	SuccessRate           float64
	TaskTypePerformance   map[string]float64
	CapabilityScores      map[capabilities.Capability]float64
	RecentFeedback        []string
}

// Agent represents an AI agent that can execute tasks
type Agent struct {
	Name                  string
	Role                  string
	Goals                 []string
	Backstory             string
	KnowledgeSources      []KnowledgeSource
	Memory                *memory.BaseMemory
	Tools                 []core.Tool
	LLMProvider           core.LLMProvider
	Executor              *core.Executor
	Config                *AgentConfig
	Capabilities          *AgentCapabilities
	TaskMonitor           *TaskMonitor
	Priority              tasks.TaskPriority
	MaxWorkload           int
	CurrentTasks          int
	Performance           map[string]float64
	DelegationPrefs       *DelegationPreference
	CollaborationScore    float64
	SupervisionNeeded     bool
	LastCollaborators     []string
	SuccessfulDelegations int
	FailedDelegations     int

	// Memory components
	shortTermMemory  *memory.ShortTermMemory
	longTermMemory   *memory.LongTermMemory
	entityMemory     memory.EntityMemory
	contextualMemory *memory.ContextualMemory
	sharedMemory     *memory.SharedMemory
	memoryConfig     *memory.MemoryConfig

	PriorityQueue    *PriorityQueue
	PriorityStrategy string
	PriorityWeights  map[string]float64 // Weights for different priority factors

	DelegationStats    *DelegationStats
	DelegationHistory  []DelegationResult
	PreferredDelegates map[string]float64 // Agent ID -> success rate

	FeedbackMetrics   *FeedbackMetrics
	LearningRate      float64
	AdaptationHistory []string

	mu sync.RWMutex // Protects concurrent access to agent state
}

// AgentOption defines functional options for configuring an agent
type AgentOption func(*Agent)

// WithMemory adds memory capabilities to the agent
func (a *Agent) WithMemory(storage memory.Storage, config *memory.MemoryConfig) error {
	if storage == nil {
		return fmt.Errorf("storage cannot be nil")
	}
	if config == nil {
		return fmt.Errorf("memory config cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	memory := memory.NewBaseMemory(storage, config)
	a.Memory = memory
	return nil
}

// NewAgent creates a new agent instance
func NewAgent(name string, role string, goals []string, provider core.LLMProvider) *Agent {
	agent := &Agent{
		Name:             name,
		Role:             role,
		Goals:            goals,
		KnowledgeSources: make([]KnowledgeSource, 0),
		Tools:            make([]core.Tool, 0),
		LLMProvider:      provider,
		Executor:         core.NewExecutor(provider),
		Config: &AgentConfig{
			MaxRetryLimit:     3,
			CodeExecutionMode: "safe",
		},
		Capabilities: NewAgentCapabilities(),
		TaskMonitor:  NewTaskMonitor(),
		DelegationStats: &DelegationStats{
			PreferredAgents: make(map[string]int),
			TaskTypes:       make(map[string]int),
		},
		PreferredDelegates: make(map[string]float64),
		FeedbackMetrics: &FeedbackMetrics{
			TaskTypePerformance: make(map[string]float64),
			CapabilityScores:    make(map[capabilities.Capability]float64),
			RecentFeedback:      make([]string, 0),
		},
		LearningRate: 0.1, // Default learning rate
	}
	return agent
}

// WithKnowledge adds knowledge sources to the agent
func (a *Agent) WithKnowledge(sources ...KnowledgeSource) error {
	if len(sources) == 0 {
		return fmt.Errorf("no knowledge sources provided")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.KnowledgeSources = append(a.KnowledgeSources, sources...)
	return nil
}

// WithCapability adds or updates a capability for the agent
func (a *Agent) WithCapability(cap capabilities.Capability, level capabilities.CapabilityLevel) error {
	if cap == "" {
		return fmt.Errorf("capability cannot be empty")
	}
	if level <= 0 {
		return fmt.Errorf("capability level must be positive")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Capabilities == nil {
		a.Capabilities = NewAgentCapabilities()
	}

	a.Capabilities.Capabilities[cap] = level
	return nil
}

// WithPriority sets the agent's priority level
func (a *Agent) WithPriority(priority tasks.TaskPriority) error {
	if priority < tasks.PriorityLow || priority > tasks.PriorityHighest {
		return fmt.Errorf("invalid priority level: %d", priority)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.Priority = priority
	return nil
}

// WithMaxWorkload sets the agent's maximum concurrent tasks
func (a *Agent) WithMaxWorkload(max int) error {
	if max <= 0 {
		return fmt.Errorf("max workload must be positive")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.MaxWorkload = max
	return nil
}

// CanHandleTask checks if the agent can handle the given task requirements
func (a *Agent) CanHandleTask(reqs *capabilities.TaskRequirements) (bool, []string, error) {
	if reqs == nil {
		return false, nil, fmt.Errorf("task requirements cannot be nil")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.Capabilities == nil {
		return false, []string{"agent has no capabilities"}, nil
	}

	match := CalculateMatch(a.Name, a.Capabilities, reqs)
	return match.Score > 0, match.Reasons, nil
}

// CanAcceptTask checks if agent can take on new task based on priority and workload
func (a *Agent) CanAcceptTask(taskPriority tasks.TaskPriority) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Critical priority tasks can always be accepted if no current critical tasks
	if taskPriority == tasks.PriorityHighest {
		return true
	}

	// Check workload capacity
	if a.MaxWorkload > 0 && a.CurrentTasks >= a.MaxWorkload {
		return false
	}

	return true
}

// IncrementTasks increases current task count
func (a *Agent) IncrementTasks() {
	a.CurrentTasks++
}

// DecrementTasks decreases current task count
func (a *Agent) DecrementTasks() {
	if a.CurrentTasks > 0 {
		a.CurrentTasks--
	}
}

// ExecuteTask executes a task with the given description
func (a *Agent) ExecuteTask(ctx context.Context, taskDescription string, expectedOutput string, taskContext string) (string, error) {
	slog.Info("Starting task execution",
		"agent", a.Name,
		"description", taskDescription)

	// Increment task count at start
	if !a.Capabilities.CanAcceptTask() {
		return "", fmt.Errorf("agent %s is at maximum task capacity", a.Name)
	}
	a.IncrementTasks()
	defer a.DecrementTasks() // Ensure task count is decremented when done

	// Generate task ID and start monitoring
	taskID := fmt.Sprintf("task_%s_%d", a.Name, time.Now().UnixNano())
	slog.Debug("Generated task ID", "taskID", taskID)

	// Create task requirements based on capabilities needed
	reqs := NewTaskRequirements()
	reqs.Required[CapabilityTaskPlanning] = CapabilityLevelBasic

	slog.Debug("Starting task monitoring", "taskID", taskID)
	if err := a.TaskMonitor.StartTask(taskID, a.Name, reqs); err != nil {
		return "", fmt.Errorf("failed to start task monitoring: %w", err)
	}

	// Retrieve relevant context from memory
	var relevantContext string
	if a.contextualMemory != nil {
		memoryContext, err := a.contextualMemory.GetTaskContext(ctx, taskID, taskDescription)
		if err == nil && len(memoryContext) > 0 {
			if items, ok := memoryContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
				// Build context from memory items
				var contextParts []string
				for _, item := range items {
					if str, ok := item.Data.(string); ok {
						contextParts = append(contextParts, str)
					}
				}
				relevantContext = strings.Join(contextParts, "\n")
				slog.Debug("Retrieved context from memory", "context_length", len(relevantContext))
			}
		}
	}

	// Update progress as we go through stages
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Task panicked",
				"taskID", taskID,
				"error", r)
			a.TaskMonitor.FailTask(taskID, fmt.Errorf("task panicked: %v", r))
			panic(r) // Re-panic after logging
		}
	}()

	// Build task prompt
	slog.Debug("Building task prompt", "taskID", taskID)
	a.TaskMonitor.UpdateProgress(taskID, 0.1, "Building task prompt")
	taskPrompt := fmt.Sprintf("You are %s.\nRole: %s\n", a.Name, a.Role)
	if len(a.Goals) > 0 {
		taskPrompt += "Goals:\n"
		for _, goal := range a.Goals {
			taskPrompt += fmt.Sprintf("- %s\n", goal)
		}
	}

	taskPrompt += fmt.Sprintf("\nTask: %s", taskDescription)
	if expectedOutput != "" {
		taskPrompt += fmt.Sprintf("\n\nExpected Output: %s", expectedOutput)
	}

	// Add context from previous steps and memory
	if taskContext != "" {
		taskPrompt += fmt.Sprintf("\n\nContext from previous steps:\n%s", taskContext)
	}
	if relevantContext != "" {
		taskPrompt += fmt.Sprintf("\n\nRelevant context from memory:\n%s", relevantContext)
	}

	// Execute task with LLM
	slog.Debug("Executing task with LLM", "taskID", taskID)
	a.TaskMonitor.UpdateProgress(taskID, 0.5, "Executing with LLM")
	result, err := a.LLMProvider.Complete(ctx, taskPrompt)
	if err != nil {
		slog.Error("LLM execution failed",
			"taskID", taskID,
			"error", err)
		a.TaskMonitor.FailTask(taskID, fmt.Errorf("llm execution failed: %w", err))
		return "", err
	}

	slog.Debug("Task completed successfully",
		"taskID", taskID,
		"result_length", len(result))

	// Save task result to memory with metadata
	metadata := map[string]interface{}{
		"task_id":          taskID,
		"agent":            a.Name,
		"role":             a.Role,
		"task_description": taskDescription,
		"timestamp":        time.Now(),
		"status":           "completed",
	}

	// Save to different memory types
	if a.shortTermMemory != nil {
		if err := a.shortTermMemory.Save(result, metadata, a.Name, memory.TaskScope, nil); err != nil {
			slog.Error("Failed to save to short-term memory", "error", err)
		}
	}

	if a.longTermMemory != nil {
		if err := a.longTermMemory.Save(result, metadata, a.Name, memory.TaskScope, nil); err != nil {
			slog.Error("Failed to save to long-term memory", "error", err)
		}
	}

	// Share result with other agents if shared memory is available
	if a.sharedMemory != nil {
		sharedContext := &memory.SharedMemoryContext{
			Source:    a.Name,
			Value:     result,
			Metadata:  metadata,
			Scope:     memory.TaskMemory,
			Timestamp: time.Now(),
		}
		if err := a.sharedMemory.Share(ctx, sharedContext); err != nil {
			slog.Error("Failed to share memory", "error", err)
		}
	}

	// Mark task as complete
	a.TaskMonitor.CompleteTask(taskID, "Task completed successfully")
	return result, nil
}

// WithTools adds tools to the agent
func (a *Agent) WithTools(tools ...core.Tool) *Agent {
	a.Tools = append(a.Tools, tools...)
	a.Executor.WithTools(tools...)
	return a
}

// WithMaxRetryLimit sets the maximum retry limit for task execution
func (a *Agent) WithMaxRetryLimit(limit int) *Agent {
	a.Config.MaxRetryLimit = limit
	return a
}

// WithCodeExecution adds code execution capabilities to the agent
func (a *Agent) WithCodeExecution(allow bool, mode string) *Agent {
	a.Config.AllowCodeExecution = allow
	a.Config.CodeExecutionMode = mode
	return a
}

// WithMultimodal adds multimodal capabilities to the agent
func (a *Agent) WithMultimodal(enabled bool) *Agent {
	a.Config.Multimodal = enabled
	return a
}

// WithDelegationPreferences sets the agent's delegation preferences
func (a *Agent) WithDelegationPreferences(prefs *DelegationPreference) *Agent {
	a.DelegationPrefs = prefs
	return a
}

// parseToolCall attempts to parse a tool call from the LLM response
func (a *Agent) parseToolCall(response string) (*types.ToolCall, error) {
	// Check if response contains a tool call marker
	if !strings.Contains(response, "USE_TOOL:") {
		return nil, nil
	}

	// Extract the JSON part
	parts := strings.Split(response, "USE_TOOL:")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid tool call format")
	}

	var toolCall types.ToolCall
	if err := json.Unmarshal([]byte(strings.TrimSpace(parts[1])), &toolCall); err != nil {
		return nil, fmt.Errorf("failed to parse tool call: %w", err)
	}

	return &toolCall, nil
}

// executeTool executes a tool with the given input
func (a *Agent) executeTool(ctx context.Context, toolCall *types.ToolCall) (string, error) {
	// Find the tool
	var tool core.Tool
	for _, t := range a.Tools {
		if strings.EqualFold(t.Name(), toolCall.Tool) {
			tool = t
			break
		}
	}

	if tool == nil {
		return "", fmt.Errorf("tool not found: %s", toolCall.Tool)
	}

	// Execute the tool
	result, err := tool.Execute(toolCall.Input)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// validateDocker checks if Docker is installed and running
func (a *Agent) validateDocker() error {
	// Check if Docker is installed
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed")
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not running")
	}

	return nil
}

// InitializeMemory initializes memory components for the agent
func (a *Agent) InitializeMemory(workDir string, config *memory.MemoryConfig) error {
	ctx := context.Background()
	agentDir := filepath.Join(workDir, a.Name)

	// Initialize short-term memory
	stmStorage, err := memory.NewStorage(ctx, config.ShortTermStorage, agentDir)
	if err != nil {
		return fmt.Errorf("failed to create short-term memory storage: %w", err)
	}
	a.shortTermMemory = memory.NewShortTermMemory(stmStorage, config)

	// Initialize long-term memory
	ltmStorage, err := memory.NewStorage(ctx, config.LongTermStorage, agentDir)
	if err != nil {
		return fmt.Errorf("failed to create long-term memory storage: %w", err)
	}
	a.longTermMemory = memory.NewLongTermMemory(ltmStorage, config)

	// Initialize entity memory
	entityStorage, err := memory.NewStorage(ctx, config.EntityStorage, agentDir)
	if err != nil {
		return fmt.Errorf("failed to create entity memory storage: %w", err)
	}
	a.entityMemory = memory.NewEntityMemoryStruct(entityStorage, config)

	// Initialize contextual memory
	a.contextualMemory = memory.NewContextualMemory(
		a.shortTermMemory,
		a.longTermMemory,
		a.entityMemory,
	)

	return nil
}

// UpdatePerformance records agent's performance for a task
func (a *Agent) UpdatePerformance(metric string, value float64) {
	if a.Performance == nil {
		a.Performance = make(map[string]float64)
	}
	// Use exponential moving average to update metrics
	if oldValue, exists := a.Performance[metric]; exists {
		a.Performance[metric] = 0.7*oldValue + 0.3*value
	} else {
		a.Performance[metric] = value
	}
}

// GetPerformanceScore returns overall performance score
func (a *Agent) GetPerformanceScore() float64 {
	if len(a.Performance) == 0 {
		return 0.5 // Default score for new agents
	}

	var total float64
	for _, score := range a.Performance {
		total += score
	}
	return total / float64(len(a.Performance))
}

// CanDelegate checks if the agent can delegate a specific task
func (a *Agent) CanDelegate(task *tasks.Task, targetAgent *Agent) (bool, DelegationType, []string) {
	if a.DelegationPrefs == nil {
		return false, "", nil
	}

	// Check if task is explicitly excluded
	for _, excluded := range a.DelegationPrefs.ExcludedTasks {
		if strings.Contains(strings.ToLower(task.Description), strings.ToLower(excluded)) {
			return false, "", []string{"Task type is excluded from delegation"}
		}
	}

	// Check collaboration requirements
	needsCollaboration := false
	for _, tag := range a.DelegationPrefs.CollaborationTags {
		if strings.Contains(strings.ToLower(task.Description), strings.ToLower(tag)) {
			needsCollaboration = true
			break
		}
	}

	// Check supervision requirements
	needsSupervision := false
	if targetAgent.SupervisionNeeded {
		needsSupervision = true
		for _, role := range a.DelegationPrefs.SupervisorRoles {
			if role == a.Role {
				needsSupervision = false
				break
			}
		}
	}

	reasons := make([]string, 0)
	var delegationType DelegationType

	switch {
	case needsSupervision:
		delegationType = SupervisedDelegation
		reasons = append(reasons, "Agent requires supervision")
	case needsCollaboration:
		delegationType = CollaborativeDelegation
		reasons = append(reasons, "Task requires collaboration")
	default:
		delegationType = DirectDelegation
		reasons = append(reasons, "Direct delegation possible")
	}

	// Check if this delegation type is preferred
	isPreferred := false
	for _, preferred := range a.DelegationPrefs.PreferredTypes {
		if preferred == delegationType {
			isPreferred = true
			break
		}
	}

	if !isPreferred {
		reasons = append(reasons, "Delegation type not preferred")
		return false, delegationType, reasons
	}

	return true, delegationType, reasons
}

// UpdateDelegationStats updates delegation success/failure statistics
func (a *Agent) UpdateDelegationStats(success bool) {
	if success {
		a.SuccessfulDelegations++
	} else {
		a.FailedDelegations++
	}
}

// GetDelegationScore returns a score indicating how well the agent handles delegations
func (a *Agent) GetDelegationScore() float64 {
	total := a.SuccessfulDelegations + a.FailedDelegations
	if total == 0 {
		return 0.5 // Default score
	}
	return float64(a.SuccessfulDelegations) / float64(total)
}

// UpdateCollaborationScore updates the agent's collaboration effectiveness score
func (a *Agent) UpdateCollaborationScore(score float64) {
	// Use exponential moving average
	a.CollaborationScore = 0.7*a.CollaborationScore + 0.3*score
}

// AddCollaborator records a successful collaboration with another agent
func (a *Agent) AddCollaborator(collaborator string) {
	// Keep only recent collaborators (last 5)
	if len(a.LastCollaborators) >= 5 {
		a.LastCollaborators = a.LastCollaborators[1:]
	}
	a.LastCollaborators = append(a.LastCollaborators, collaborator)
}

// AgentCapabilities represents an agent's capabilities and limitations
type AgentCapabilities struct {
	Capabilities       map[capabilities.Capability]capabilities.CapabilityLevel
	Priority           int
	MaxConcurrentTasks int
	CurrentTasks       int
}

// NewAgentCapabilities creates a new capabilities instance
func NewAgentCapabilities() *AgentCapabilities {
	return &AgentCapabilities{
		Capabilities: make(map[capabilities.Capability]capabilities.CapabilityLevel),
	}
}

// TaskMonitor handles task progress tracking
type TaskMonitor struct {
	activeTasks map[string]*TaskProgress
	callbacks   map[string][]func(string, float64, string)
}

// TaskProgress tracks the progress of a task
type TaskProgress struct {
	TaskID       string
	AgentID      string
	Status       string
	Progress     float64
	Messages     []string
	StartTime    time.Time
	EndTime      *time.Time
	Requirements *capabilities.TaskRequirements
}

// NewTaskMonitor creates a new task monitor
func NewTaskMonitor() *TaskMonitor {
	return &TaskMonitor{
		activeTasks: make(map[string]*TaskProgress),
		callbacks:   make(map[string][]func(string, float64, string)),
	}
}

// StartTask begins monitoring a new task
func (tm *TaskMonitor) StartTask(taskID, agentID string, reqs *capabilities.TaskRequirements) error {
	if _, exists := tm.activeTasks[taskID]; exists {
		return fmt.Errorf("task %s is already being monitored", taskID)
	}

	tm.activeTasks[taskID] = &TaskProgress{
		TaskID:       taskID,
		AgentID:      agentID,
		Status:       "started",
		Progress:     0.0,
		StartTime:    time.Now(),
		Requirements: reqs,
	}

	return nil
}

// UpdateProgress updates a task's progress
func (tm *TaskMonitor) UpdateProgress(taskID string, progress float64, message string) error {
	task, exists := tm.activeTasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Progress = progress
	if message != "" {
		task.Messages = append(task.Messages, message)
	}

	// Notify callbacks
	if callbacks, exists := tm.callbacks[taskID]; exists {
		for _, callback := range callbacks {
			callback(taskID, progress, message)
		}
	}

	return nil
}

// CompleteTask marks a task as complete
func (tm *TaskMonitor) CompleteTask(taskID string, message string) error {
	task, exists := tm.activeTasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.EndTime = &now
	task.Status = "completed"
	task.Progress = 1.0
	if message != "" {
		task.Messages = append(task.Messages, message)
	}

	return nil
}

// FailTask marks a task as failed
func (tm *TaskMonitor) FailTask(taskID string, err error) error {
	task, exists := tm.activeTasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.EndTime = &now
	task.Status = "failed"
	if err != nil {
		task.Messages = append(task.Messages, err.Error())
	}

	return nil
}

// GetTaskProgress retrieves the current progress of a task
func (tm *TaskMonitor) GetTaskProgress(taskID string) (*TaskProgress, error) {
	task, exists := tm.activeTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// AddProgressCallback adds a callback for task progress updates
func (tm *TaskMonitor) AddProgressCallback(taskID string, callback func(string, float64, string)) {
	if _, exists := tm.callbacks[taskID]; !exists {
		tm.callbacks[taskID] = make([]func(string, float64, string), 0)
	}
	tm.callbacks[taskID] = append(tm.callbacks[taskID], callback)
}

// CanAcceptTask checks if agent can accept more tasks based on capabilities
func (ac *AgentCapabilities) CanAcceptTask() bool {
	return ac.MaxConcurrentTasks == 0 || ac.CurrentTasks < ac.MaxConcurrentTasks
}

// CalculateMatch calculates how well an agent matches task requirements
type CapabilityMatch struct {
	Score   float64
	Reasons []string
}

func CalculateMatch(agentName string, capabilities *AgentCapabilities, reqs *capabilities.TaskRequirements) *CapabilityMatch {
	match := &CapabilityMatch{
		Score:   0.0,
		Reasons: make([]string, 0),
	}

	// Check required capabilities
	for cap, level := range reqs.Required {
		if agentLevel, ok := capabilities.Capabilities[cap]; !ok {
			match.Reasons = append(match.Reasons, fmt.Sprintf("Missing required capability: %s", cap))
			return match
		} else if agentLevel < level {
			match.Reasons = append(match.Reasons, fmt.Sprintf("Insufficient level for %s: %d < %d", cap, agentLevel, level))
			return match
		}
	}

	// Calculate score based on capabilities
	totalCaps := len(reqs.Required) + len(reqs.Optional)
	matchedCaps := len(reqs.Required) // We've already verified required caps

	// Check optional capabilities
	for cap, level := range reqs.Optional {
		if agentLevel, ok := capabilities.Capabilities[cap]; ok && agentLevel >= level {
			matchedCaps++
		}
	}

	match.Score = float64(matchedCaps) / float64(totalCaps)
	match.Reasons = append(match.Reasons, fmt.Sprintf("Matched %d/%d capabilities", matchedCaps, totalCaps))

	return match
}

// Capability constants
const (
	CapabilityTaskPlanning = "task_planning"
	CapabilityLevelBasic   = 1
)

// NewTaskRequirements creates a new task requirements instance
func NewTaskRequirements() *capabilities.TaskRequirements {
	return &capabilities.TaskRequirements{
		Required: make(map[capabilities.Capability]capabilities.CapabilityLevel),
		Optional: make(map[capabilities.Capability]capabilities.CapabilityLevel),
	}
}

// WithPriorityStrategy sets the agent's priority handling strategy
func (a *Agent) WithPriorityStrategy(strategy string, weights map[string]float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.PriorityStrategy = strategy
	a.PriorityWeights = weights
	a.PriorityQueue = &PriorityQueue{
		Tasks:    make([]*tasks.Task, 0),
		Capacity: a.MaxWorkload,
		Strategy: strategy,
	}
	return nil
}

// CalculateTaskPriority computes a priority score for a task
func (a *Agent) CalculateTaskPriority(task *tasks.Task) float64 {
	if a.PriorityWeights == nil {
		// Default weights if none specified
		a.PriorityWeights = map[string]float64{
			"level":      0.4,
			"deadline":   0.3,
			"importance": 0.2,
			"urgency":    0.1,
		}
	}

	var score float64

	// Base priority level score
	levelScore := float64(task.Priority) / float64(tasks.PriorityHighest)
	score += levelScore * a.PriorityWeights["level"]

	// Deadline score - closer deadline = higher priority
	if !task.Deadline.IsZero() {
		timeUntilDeadline := time.Until(task.Deadline)
		if timeUntilDeadline <= 0 {
			score += a.PriorityWeights["deadline"] // Max deadline score
		} else {
			// Normalize deadline score (24 hours max)
			deadlineScore := math.Max(0, 24-timeUntilDeadline.Hours()) / 24
			score += deadlineScore * a.PriorityWeights["deadline"]
		}
	}

	// Get importance and urgency from metadata if available
	if task.Metadata != nil {
		if importance, ok := task.Metadata["importance"].(float64); ok {
			score += importance * a.PriorityWeights["importance"]
		}
		if urgency, ok := task.Metadata["urgency"].(float64); ok {
			score += urgency * a.PriorityWeights["urgency"]
		}
	}

	return score
}

// AddTask adds a task to the priority queue
func (a *Agent) AddTask(task *tasks.Task) error {
	if len(a.PriorityQueue.Tasks) >= a.PriorityQueue.Capacity {
		return fmt.Errorf("priority queue is at capacity")
	}

	task.PriorityScore = a.CalculateTaskPriority(task)
	a.PriorityQueue.Tasks = append(a.PriorityQueue.Tasks, task)

	// Sort based on priority strategy
	a.sortPriorityQueue()
	return nil
}

// sortPriorityQueue sorts tasks based on the chosen strategy
func (a *Agent) sortPriorityQueue() {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.PriorityStrategy {
	case "fifo":
		// Already in order
		return
	case "deadline":
		sort.Slice(a.PriorityQueue.Tasks, func(i, j int) bool {
			if a.PriorityQueue.Tasks[i].Deadline.IsZero() {
				return false
			}
			if a.PriorityQueue.Tasks[j].Deadline.IsZero() {
				return true
			}
			return a.PriorityQueue.Tasks[i].Deadline.Before(
				a.PriorityQueue.Tasks[j].Deadline)
		})
	default: // "balanced" or any other strategy
		sort.Slice(a.PriorityQueue.Tasks, func(i, j int) bool {
			return a.PriorityQueue.Tasks[i].PriorityScore >
				a.PriorityQueue.Tasks[j].PriorityScore
		})
	}
}

// GetNextTask gets the next task from the priority queue
func (a *Agent) GetNextTask() *tasks.Task {
	if len(a.PriorityQueue.Tasks) == 0 {
		return nil
	}

	task := a.PriorityQueue.Tasks[0]
	a.PriorityQueue.Tasks = a.PriorityQueue.Tasks[1:]
	return task
}

// DelegateTask attempts to delegate a task to another agent
func (a *Agent) DelegateTask(ctx context.Context, task *tasks.Task, targetAgent *Agent) (*DelegationResult, error) {
	// Check if delegation is allowed
	if !a.Config.AllowDelegation {
		return nil, fmt.Errorf("delegation not allowed for agent %s", a.Name)
	}

	// Check if target agent can handle the task
	match := CalculateMatch(targetAgent.Name, targetAgent.Capabilities, task.Requirements)
	if match.Score == 0 {
		return &DelegationResult{
			Success:      false,
			AgentID:      targetAgent.Name,
			TaskID:       task.ID,
			ErrorMessage: fmt.Sprintf("agent cannot handle task: %v", match.Reasons),
		}, nil
	}

	// Check target agent's workload
	if !targetAgent.CanAcceptTask(task.Priority) {
		return &DelegationResult{
			Success:      false,
			AgentID:      targetAgent.Name,
			TaskID:       task.ID,
			ErrorMessage: "agent at maximum workload capacity",
		}, nil
	}

	// Attempt to delegate
	err := targetAgent.AddTask(task)
	if err != nil {
		a.DelegationStats.FailedDelegations++
		return &DelegationResult{
			Success:      false,
			AgentID:      targetAgent.Name,
			TaskID:       task.ID,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Update delegation stats
	a.DelegationStats.TotalDelegations++
	a.DelegationStats.SuccessfulDelegations++
	a.DelegationStats.PreferredAgents[targetAgent.Name]++

	// Update success rate
	a.PreferredDelegates[targetAgent.Name] = float64(a.DelegationStats.SuccessfulDelegations) /
		float64(a.DelegationStats.TotalDelegations)

	return &DelegationResult{
		Success:  true,
		AgentID:  targetAgent.Name,
		TaskID:   task.ID,
		Feedback: "Task successfully delegated",
	}, nil
}

// FindBestDelegate finds the most suitable agent for delegation
func (a *Agent) FindBestDelegate(task *tasks.Task, agents []*Agent) *Agent {
	var bestAgent *Agent
	var bestScore float64

	for _, agent := range agents {
		if agent.Name == a.Name {
			continue // Skip self
		}

		// Calculate delegation score
		score := a.calculateDelegationScore(task, agent)
		if score > bestScore {
			bestScore = score
			bestAgent = agent
		}
	}

	return bestAgent
}

// calculateDelegationScore computes a score for delegating a task to an agent
func (a *Agent) calculateDelegationScore(task *tasks.Task, agent *Agent) float64 {
	score := 0.0

	// Factor 1: Past success rate with this agent (40%)
	if successRate, exists := a.PreferredDelegates[agent.Name]; exists {
		score += successRate * 0.4
	}

	// Factor 2: Agent's current workload (30%)
	workloadScore := 1.0 - (float64(agent.CurrentTasks) / float64(agent.MaxWorkload))
	score += workloadScore * 0.3

	// Factor 3: Capability match (30%)
	match := CalculateMatch(agent.Name, agent.Capabilities, task.Requirements)
	score += match.Score * 0.3

	return score
}

// GetDelegationStats returns current delegation statistics
func (a *Agent) GetDelegationStats() *DelegationStats {
	return a.DelegationStats
}

// UpdateDelegationFeedback updates delegation success metrics
func (a *Agent) UpdateDelegationFeedback(result *DelegationResult) {
	a.DelegationHistory = append(a.DelegationHistory, *result)

	// Update preferred delegates based on recent history
	successCount := 0
	recentHistory := a.DelegationHistory
	if len(recentHistory) > 10 {
		recentHistory = recentHistory[len(recentHistory)-10:]
	}

	for _, res := range recentHistory {
		if res.Success {
			successCount++
		}
	}

	// Update success rate for the agent
	if len(recentHistory) > 0 {
		a.PreferredDelegates[result.AgentID] = float64(successCount) / float64(len(recentHistory))
	}
}

// ProcessTaskFeedback processes feedback after task completion
func (a *Agent) ProcessTaskFeedback(task *tasks.Task, success bool, executionTime time.Duration, feedback string) {
	// Update basic metrics
	if success {
		a.FeedbackMetrics.TasksCompleted++
	} else {
		a.FeedbackMetrics.TasksFailed++
	}

	// Update average completion time
	totalTasks := float64(a.FeedbackMetrics.TasksCompleted + a.FeedbackMetrics.TasksFailed)
	newAvg := (a.FeedbackMetrics.AverageCompletionTime.Seconds()*float64(totalTasks-1) +
		executionTime.Seconds()) / totalTasks
	a.FeedbackMetrics.AverageCompletionTime = time.Duration(newAvg) * time.Second

	// Update success rate
	a.FeedbackMetrics.SuccessRate = float64(a.FeedbackMetrics.TasksCompleted) / totalTasks

	// Store recent feedback
	a.FeedbackMetrics.RecentFeedback = append(a.FeedbackMetrics.RecentFeedback, feedback)
	if len(a.FeedbackMetrics.RecentFeedback) > 10 {
		a.FeedbackMetrics.RecentFeedback = a.FeedbackMetrics.RecentFeedback[1:]
	}

	// Update task type performance
	taskType := task.Metadata["type"].(string)
	if taskType != "" {
		currentScore := a.FeedbackMetrics.TaskTypePerformance[taskType]
		if success {
			a.FeedbackMetrics.TaskTypePerformance[taskType] = currentScore + (1-currentScore)*a.LearningRate
		} else {
			a.FeedbackMetrics.TaskTypePerformance[taskType] = currentScore - currentScore*a.LearningRate
		}
	}

	// Update capability scores based on task requirements
	for capability, level := range task.Requirements.Required {
		currentScore := a.FeedbackMetrics.CapabilityScores[capability]
		if success {
			// Increase score based on successful use of capability
			newScore := currentScore + (float64(level)-currentScore)*a.LearningRate
			a.FeedbackMetrics.CapabilityScores[capability] = newScore
		} else {
			// Decrease score based on failed use of capability
			newScore := currentScore - currentScore*a.LearningRate
			a.FeedbackMetrics.CapabilityScores[capability] = newScore
		}
	}

	// Record adaptation
	adaptation := fmt.Sprintf("Task %s completed with success=%v, updating metrics", task.ID, success)
	a.AdaptationHistory = append(a.AdaptationHistory, adaptation)
}

// GetPerformanceReport generates a performance report
func (a *Agent) GetPerformanceReport() map[string]interface{} {
	return map[string]interface{}{
		"total_tasks":           a.FeedbackMetrics.TasksCompleted + a.FeedbackMetrics.TasksFailed,
		"success_rate":          a.FeedbackMetrics.SuccessRate,
		"avg_completion_time":   a.FeedbackMetrics.AverageCompletionTime,
		"task_type_performance": a.FeedbackMetrics.TaskTypePerformance,
		"capability_scores":     a.FeedbackMetrics.CapabilityScores,
		"recent_feedback":       a.FeedbackMetrics.RecentFeedback,
		"adaptations":           a.AdaptationHistory,
	}
}

// AdjustLearningRate adjusts the learning rate based on performance
func (a *Agent) AdjustLearningRate(performance float64) {
	// Adjust learning rate based on recent performance
	// Higher performance = lower learning rate (fine-tuning)
	// Lower performance = higher learning rate (more adaptation)
	if performance > 0.8 {
		a.LearningRate *= 0.9 // Reduce learning rate
	} else if performance < 0.5 {
		a.LearningRate *= 1.1 // Increase learning rate
	}

	// Keep learning rate within reasonable bounds
	if a.LearningRate < 0.01 {
		a.LearningRate = 0.01
	} else if a.LearningRate > 0.5 {
		a.LearningRate = 0.5
	}
}

// GetVoteDecision asks the agent to vote on a task with explanation
func (a *Agent) GetVoteDecision(ctx context.Context, voteContext string) (string, string, error) {
	prompt := fmt.Sprintf(`Based on the following context, please vote APPROVE or REJECT with a detailed explanation:

%s

Your response should be in the format:
DECISION: [APPROVE/REJECT]
RATIONALE: [Your detailed explanation]`, voteContext)

	response, err := a.ExecuteTask(ctx, prompt, "DECISION and RATIONALE", "")
	if err != nil {
		return "", "", err
	}

	// Parse response to extract decision and rationale
	// This is a simplified implementation - you may want to add more robust parsing
	decision := "REJECT" // Default to reject
	if len(response) >= 7 && response[:7] == "APPROVE" {
		decision = "APPROVE"
	}

	return decision, response, nil
}

// GetTaskApproach returns the agent's approach to executing a task
func (a *Agent) GetTaskApproach(ctx context.Context, task *tasks.Task) (string, error) {
	prompt := fmt.Sprintf(`Given this task:
Description: %s
Expected Output: %s

Explain your approach to executing this task. Be specific about:
1. Steps you would take
2. Tools you would use
3. Key considerations
4. Success criteria`, task.Description, task.ExpectedOutput)

	approach, err := a.ExecuteTask(ctx, prompt, "Detailed task execution approach", "")
	if err != nil {
		return "", fmt.Errorf("failed to get task approach: %w", err)
	}

	return approach, nil
}

// GetResourceRequirements returns the agent's resource needs for a task
func (a *Agent) GetResourceRequirements(ctx context.Context, task *tasks.Task) (map[string]int, error) {
	// Default resource requirements
	requirements := map[string]int{
		"cpu":    10,  // Base CPU requirement
		"memory": 100, // Base memory requirement in MB
	}

	// Adjust based on task complexity and agent capabilities
	if task.Priority == tasks.PriorityHighest {
		requirements["cpu"] *= 2
		requirements["memory"] *= 2
	}

	// Consider agent's current workload
	if a.CurrentTasks > 0 {
		requirements["cpu"] += a.CurrentTasks * 5
		requirements["memory"] += a.CurrentTasks * 50
	}

	return requirements, nil
}

// AssessTaskPriority evaluates and returns the agent's assessment of task priority
func (a *Agent) AssessTaskPriority(task *tasks.Task) tasks.TaskPriority {
	// Start with the task's defined priority
	priority := task.Priority

	// Adjust based on agent's expertise and task requirements
	if reqs := task.Requirements; reqs != nil {
		for cap, level := range reqs.Required {
			if agentLevel, ok := a.Capabilities.Capabilities[cap]; ok {
				if agentLevel < level {
					// Increase priority if agent lacks required capability level
					priority++
				}
			}
		}
	}

	// Consider deadline if present
	if !task.Deadline.IsZero() && time.Until(task.Deadline) < 24*time.Hour {
		priority = tasks.PriorityHighest
	}

	return priority
}

// ResolveConflict has the agent make a decision about a conflict
func (a *Agent) ResolveConflict(ctx context.Context, conflict *types.Conflict) (string, error) {
	// Build context for decision
	conflictContext := fmt.Sprintf(`Conflict Details:
Type: %s
Description: %s
Status: %s

Please analyze this conflict and provide a clear resolution decision.
Consider:
1. Impact on task execution
2. Resource efficiency
3. Team dynamics
4. Long-term implications

Provide your decision and detailed rationale.`, conflict.Type, conflict.Description, conflict.Status)

	decision, err := a.ExecuteTask(ctx, conflictContext, "Resolution decision with rationale", "")
	if err != nil {
		return "", fmt.Errorf("failed to resolve conflict: %w", err)
	}

	return decision, nil
}

// Mediate has the agent act as a mediator for a conflict
func (a *Agent) Mediate(ctx context.Context, conflict *types.Conflict) (string, error) {
	// Build mediation context
	mediationContext := fmt.Sprintf(`As a mediator, please help resolve this conflict:
Type: %s
Description: %s
Status: %s

Involved Agents:
%s

Please:
1. Analyze each perspective
2. Identify common ground
3. Propose a fair resolution
4. Explain your reasoning

Provide a detailed mediation proposal.`,
		conflict.Type,
		conflict.Description,
		conflict.Status,
		formatAgentList(a.GetInvolvedAgents(conflict)))

	proposal, err := a.ExecuteTask(ctx, mediationContext, "Mediation proposal with rationale", "")
	if err != nil {
		return "", fmt.Errorf("failed to mediate conflict: %w", err)
	}

	return proposal, nil
}

// GetInvolvedAgents returns the Agent objects for the agent IDs in the conflict
func (a *Agent) GetInvolvedAgents(conflict *types.Conflict) []*Agent {
	// This is a placeholder - in a real implementation, you would look up
	// the agents from some registry or the team
	agents := make([]*Agent, 0, len(conflict.InvolvedAgents))
	return agents
}

// Helper function to format agent list
func formatAgentList(agents []*Agent) string {
	var result string
	for i, agent := range agents {
		result += fmt.Sprintf("%d. %s (Role: %s)\n", i+1, agent.Name, agent.Role)
	}
	return result
}
