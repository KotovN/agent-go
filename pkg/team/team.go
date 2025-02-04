package team

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-go/pkg/agents"
	"agent-go/pkg/core"
	"agent-go/pkg/embeddings"
	"agent-go/pkg/memory"
	"agent-go/pkg/tasks"
	"agent-go/pkg/utils"
)

// ProcessType defines how agents in a team work together
type ProcessType string

const (
	// Sequential - agents work one after another, passing results forward
	Sequential ProcessType = "sequential"
	// Hierarchical - agents work in a manager-worker relationship
	Hierarchical ProcessType = "hierarchical"
	// Consensus - agents work together to reach agreement through voting
	Consensus ProcessType = "consensus"
)

// RecoveryStrategy defines how to handle task failures
type RecoveryStrategy string

const (
	// RetryWithSameAgent retries the failed task with the same agent
	RetryWithSameAgent RecoveryStrategy = "retry_same"
	// RetryWithDifferentAgent retries the task with a different agent
	RetryWithDifferentAgent RecoveryStrategy = "retry_different"
	// EscalateToManager sends the failed task to manager for review
	EscalateToManager RecoveryStrategy = "escalate"
)

// TaskError represents a task execution error with context
type TaskError struct {
	TaskID      string
	AgentName   string
	err         error
	Context     string
	Recoverable bool
}

// Error implements the error interface
func (e *TaskError) Error() string {
	return fmt.Sprintf("task %s failed by agent %s: %v", e.TaskID, e.AgentName, e.err)
}

// Unwrap returns the underlying error
func (e *TaskError) Unwrap() error {
	return e.err
}

// NewTaskError creates a new TaskError
func NewTaskError(taskID, agentName string, err error, context string, recoverable bool) *TaskError {
	return &TaskError{
		TaskID:      taskID,
		AgentName:   agentName,
		err:         err,
		Context:     context,
		Recoverable: recoverable,
	}
}

// ManagerConfig defines custom manager configuration
type ManagerConfig struct {
	// Format for task analysis output
	AnalysisFormat string
	// Format for task review output
	ReviewFormat string
	// Whether to require explicit approval/rejection
	RequireApproval bool
}

// Default manager configuration
var DefaultManagerConfig = ManagerConfig{
	AnalysisFormat:  "A detailed analysis including: 1) Key requirements and constraints 2) Potential challenges and risks 3) Recommended execution approach 4) Success criteria",
	ReviewFormat:    "APPROVE or REJECT with detailed explanation",
	RequireApproval: true,
}

// VotingConfig defines configuration for consensus voting
type VotingConfig struct {
	// Minimum percentage of votes needed to reach consensus (0-100)
	ConsensusThreshold int
	// Maximum number of voting rounds before escalating
	MaxRounds int
	// Whether to require unanimous agreement
	RequireUnanimous bool
	// How to handle ties
	TieBreaker string
	// Format for vote responses
	VoteFormat string
}

// DefaultVotingConfig provides default voting configuration
var DefaultVotingConfig = VotingConfig{
	ConsensusThreshold: 70,
	MaxRounds:          3,
	RequireUnanimous:   false,
	TieBreaker:         "manager",
	VoteFormat:         "APPROVE or REJECT with detailed explanation",
}

// EmbeddingAdapter adapts embeddings.Model to memory.Embedder
type EmbeddingAdapter struct {
	model embeddings.Model
}

func (a *EmbeddingAdapter) Embed(text string) ([]float32, error) {
	vectors, err := a.model.Embed(context.Background(), []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}
	return vectors[0], nil
}

func (a *EmbeddingAdapter) Dimension() int {
	return a.model.Dimension()
}

// SimpleEmbedder provides basic local embeddings without external API
type SimpleEmbedder struct {
	dimension int
}

func NewSimpleEmbedder(dimension int) *SimpleEmbedder {
	return &SimpleEmbedder{dimension: dimension}
}

func (e *SimpleEmbedder) Embed(text string) ([]float32, error) {
	// Create a simple deterministic vector based on the text
	vector := make([]float32, e.dimension)
	for i := 0; i < e.dimension && i < len(text); i++ {
		// Use character codes to generate vector components
		if i < len(text) {
			vector[i] = float32(text[i]) / 255.0 // Normalize to [0,1]
		}
	}
	return vector, nil
}

func (e *SimpleEmbedder) Dimension() int {
	return e.dimension
}

// Team represents a team of agents working together
type Team struct {
	Name          string
	Description   string
	Agents        []*agents.Agent
	Tasks         []*tasks.Task
	ProcessType   ProcessType
	Manager       *agents.Agent // Used in hierarchical process
	Memory        bool
	pipeline      *tasks.Pipeline
	logger        *utils.Logger
	workDir       string
	managerConfig *ManagerConfig

	// Memory components
	shortTermMemory  *memory.ShortTermMemory
	longTermMemory   *memory.LongTermMemory
	entityMemory     memory.EntityMemory
	contextualMemory *memory.ContextualMemory
	sharedMemory     *memory.SharedMemory
	memoryConfig     *memory.MemoryConfig

	// RBAC components
	rbacManager    *core.RBACManager
	rbacMiddleware *core.RBACMiddleware
}

// TeamOption defines functional options for configuring a team
type TeamOption func(*Team)

// WithMemory enables memory capabilities for the team
func WithMemory(enabled bool) TeamOption {
	return func(t *Team) {
		t.Memory = enabled
	}
}

// WithManagerConfig sets custom manager configuration
func WithManagerConfig(config ManagerConfig) TeamOption {
	return func(t *Team) {
		t.managerConfig = &config
	}
}

// WithManager sets the manager for the team
func WithManager(manager *agents.Agent) TeamOption {
	return func(t *Team) {
		t.Manager = manager
	}
}

// NewTeam creates a new team instance
func NewTeam(name, description string, processType ProcessType, workDir string, memoryConfig memory.MemoryConfig, opts ...TeamOption) (*Team, error) {
	pipeline, err := tasks.NewPipeline(filepath.Join(workDir, "outputs"))
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	// Initialize RBAC
	rbacManager := core.NewRBACManager()
	if err := core.InitializeDefaultRoles(rbacManager); err != nil {
		return nil, fmt.Errorf("failed to initialize RBAC roles: %w", err)
	}
	rbacMiddleware := core.NewRBACMiddleware(rbacManager)

	team := &Team{
		Name:           name,
		Description:    description,
		ProcessType:    processType,
		Agents:         make([]*agents.Agent, 0),
		Tasks:          make([]*tasks.Task, 0),
		pipeline:       pipeline,
		logger:         utils.NewLogger(false),
		workDir:        workDir,
		memoryConfig:   &memoryConfig,
		managerConfig:  &DefaultManagerConfig,
		rbacManager:    rbacManager,
		rbacMiddleware: rbacMiddleware,
	}

	// Apply options
	for _, opt := range opts {
		opt(team)
	}

	return team, nil
}

// initializeMemory sets up memory components for the team
func (t *Team) initializeMemory() error {
	if len(t.Agents) == 0 {
		return fmt.Errorf("at least one agent is required for memory initialization")
	}

	// Use simple local embedder that doesn't require API
	embedder := NewSimpleEmbedder(64) // 64-dimensional vectors

	// Initialize short-term memory with RAG storage
	stmStorage := memory.NewRAGStorage(embedder)
	t.shortTermMemory = memory.NewShortTermMemory(stmStorage, t.memoryConfig)

	// Initialize long-term memory with RAG storage
	ltmStorage := memory.NewRAGStorage(embedder)
	t.longTermMemory = memory.NewLongTermMemory(ltmStorage, t.memoryConfig)

	// Initialize entity memory with RAG storage
	entityStorage := memory.NewRAGStorage(embedder)
	t.entityMemory = memory.NewEntityMemoryStruct(entityStorage, t.memoryConfig)

	// Initialize contextual memory
	t.contextualMemory = memory.NewContextualMemory(
		t.shortTermMemory,
		t.longTermMemory,
		t.entityMemory,
	)

	// Initialize shared memory with RAG storage
	sharedStorage := memory.NewRAGStorage(embedder)
	t.sharedMemory = memory.NewSharedMemory(sharedStorage, t.memoryConfig)

	// Set up memory permissions for agents
	for _, agent := range t.Agents {
		// Manager gets global read/write access
		if agent == t.Manager {
			t.sharedMemory.SetPermissions(agent.Name, &memory.MemoryPermission{
				AgentID:     agent.Name,
				Scope:       memory.GlobalScope,
				AccessLevel: memory.AccessReadWrite,
			})
		} else {
			// Workers get shared access by default
			t.sharedMemory.SetPermissions(agent.Name, &memory.MemoryPermission{
				AgentID:     agent.Name,
				Scope:       memory.ProcessScope,
				AccessLevel: memory.AccessReadOnly,
			})
		}
	}

	return nil
}

// AddAgent adds an agent to the team
func (t *Team) AddAgent(agent *agents.Agent) error {
	// Initialize agent memory if team has memory enabled
	if t.Memory {
		if err := agent.InitializeMemory(t.workDir, t.memoryConfig); err != nil {
			return fmt.Errorf("failed to initialize agent memory: %w", err)
		}
	}

	// Assign appropriate role based on agent type and team process type
	var roleName string
	switch {
	case agent == t.Manager:
		roleName = core.RoleManager
	case t.ProcessType == Hierarchical && agent.DelegationPrefs != nil && len(agent.DelegationPrefs.SupervisorRoles) > 0:
		roleName = core.RoleTaskCreator
	default:
		roleName = core.RoleWorker
	}

	// Assign role to agent
	if err := t.rbacManager.AssignRole(agent.Name, roleName); err != nil {
		return fmt.Errorf("failed to assign role to agent: %w", err)
	}

	t.Agents = append(t.Agents, agent)

	// Initialize team memory after first agent is added
	if t.Memory && len(t.Agents) == 1 {
		if err := t.initializeMemory(); err != nil {
			return fmt.Errorf("failed to initialize team memory: %w", err)
		}
	}

	return nil
}

// Remember stores information in the team's memory
func (t *Team) Remember(ctx context.Context, value interface{}, metadata map[string]interface{}) error {
	if !t.Memory {
		return nil
	}

	// Calculate expiration based on config
	var expiresAt *time.Time
	if t.memoryConfig.DefaultTTL > 0 {
		t := time.Now().Add(t.memoryConfig.DefaultTTL)
		expiresAt = &t
	}

	// Convert value to string if needed
	valueStr := fmt.Sprintf("%v", value)

	// Store in both short-term and long-term memory
	if err := t.shortTermMemory.Save(valueStr, metadata, "team", memory.TaskScope, expiresAt); err != nil {
		return fmt.Errorf("failed to save to short-term memory: %w", err)
	}

	if err := t.longTermMemory.Save(valueStr, metadata, "team", memory.TaskScope, expiresAt); err != nil {
		return fmt.Errorf("failed to save to long-term memory: %w", err)
	}

	return nil
}

// RememberEntity stores entity information in memory
func (t *Team) RememberEntity(ctx context.Context, name string, entityType memory.EntityType, description string, metadata map[string]interface{}) error {
	if !t.Memory {
		return nil
	}

	item := memory.EntityMemoryItem{
		Name:        name,
		Type:        entityType,
		Description: description,
		Metadata:    metadata,
		Agent:       "team",
	}

	return t.entityMemory.SaveEntity(ctx, item)
}

// Recall searches the team's memory for relevant information
func (t *Team) Recall(ctx context.Context, query string) (map[string]interface{}, error) {
	if !t.Memory {
		return nil, nil
	}

	return t.contextualMemory.GetRelevantContext(ctx, &memory.MemoryQuery{
		Query: query,
		Metadata: map[string]interface{}{
			"team_name": t.Name,
			"process":   string(t.ProcessType),
		},
	})
}

// ValidateProcess checks if the team is properly configured for its process type
func (t *Team) ValidateProcess() error {
	// Basic validation
	if len(t.Agents) == 0 {
		return fmt.Errorf("team must have at least one agent")
	}

	if len(t.Tasks) == 0 {
		return fmt.Errorf("team must have at least one task")
	}

	// Process-specific validation
	switch t.ProcessType {
	case Sequential:
		if err := t.validateSequentialProcess(); err != nil {
			return fmt.Errorf("sequential process validation failed: %w", err)
		}
	case Hierarchical:
		if err := t.validateHierarchicalProcess(); err != nil {
			return fmt.Errorf("hierarchical process validation failed: %w", err)
		}
	case Consensus:
		if err := t.validateConsensusProcess(); err != nil {
			return fmt.Errorf("consensus process validation failed: %w", err)
		}
	default:
		return fmt.Errorf("unsupported process type: %s", t.ProcessType)
	}

	// Validate agent configuration
	for _, agent := range t.Agents {
		if agent.LLMProvider == nil {
			return fmt.Errorf("agent %s has no LLM provider configured", agent.Name)
		}
	}

	return nil
}

// validateSequentialProcess validates sequential process requirements
func (t *Team) validateSequentialProcess() error {
	if len(t.Tasks) != len(t.Agents) {
		return fmt.Errorf("sequential process requires equal number of tasks (%d) and agents (%d)", len(t.Tasks), len(t.Agents))
	}

	// Check task dependencies
	for i, task := range t.Tasks {
		if i > 0 && task.Context == "" {
			t.logger.Warning("Task %s might need context from previous task for sequential execution", task.ID)
		}
	}

	return nil
}

// validateHierarchicalProcess validates hierarchical process requirements
func (t *Team) validateHierarchicalProcess() error {
	if t.Manager == nil {
		return fmt.Errorf("hierarchical process requires a manager")
	}

	// Verify manager has delegation tool
	hasDelegationTool := false
	for _, tool := range t.Manager.Tools {
		if tool.Name() == "delegate" {
			hasDelegationTool = true
			break
		}
	}
	if !hasDelegationTool {
		return fmt.Errorf("manager requires delegation tool for hierarchical process")
	}

	// Check if there are enough agents for delegation
	if len(t.Agents) < 2 { // At least manager and one worker
		return fmt.Errorf("hierarchical process requires at least 2 agents (manager and worker)")
	}

	return nil
}

// validateConsensusProcess validates consensus process requirements
func (t *Team) validateConsensusProcess() error {
	if len(t.Tasks) < 2 {
		return fmt.Errorf("consensus process requires at least two tasks")
	}

	// Check task dependencies
	for i, task := range t.Tasks {
		if i > 0 && task.Context == "" {
			t.logger.Warning("Task %s might need context from previous task for consensus execution", task.ID)
		}
	}

	return nil
}

// Execute runs the team's tasks with validation and error recovery
func (t *Team) Execute(ctx context.Context) ([]string, error) {
	// Validate process configuration
	if err := t.ValidateProcess(); err != nil {
		return nil, fmt.Errorf("process validation failed: %w", err)
	}

	if t.Memory {
		// Store team execution start
		metadata := map[string]interface{}{
			"event_type": "execution_start",
			"team_name":  t.Name,
			"process":    string(t.ProcessType),
		}
		if err := t.Remember(ctx, "Starting team execution", metadata); err != nil {
			t.logger.Error("Failed to store execution start: %v", err)
		}
	}

	results, err := t.executeWithProcess(ctx)

	if t.Memory && err == nil {
		// Store execution results
		metadata := map[string]interface{}{
			"event_type": "execution_complete",
			"team_name":  t.Name,
			"process":    string(t.ProcessType),
		}
		if err := t.Remember(ctx, results, metadata); err != nil {
			t.logger.Error("Failed to store execution results: %v", err)
		}
	}

	return results, err
}

// executeWithProcess handles execution based on process type with error recovery and memory sharing
func (t *Team) executeWithProcess(ctx context.Context) ([]string, error) {
	var results []string
	var err error

	// Share process start context
	if t.Memory {
		metadata := map[string]interface{}{
			"event":     "process_start",
			"process":   string(t.ProcessType),
			"timestamp": time.Now(),
		}
		if err := t.ShareMemory(ctx, "team", metadata, nil, metadata); err != nil {
			t.logger.Warning("Failed to share process start context: %v", err)
		}
	}

	switch t.ProcessType {
	case Sequential:
		results, err = t.executeSequential(ctx)
	case Hierarchical:
		results, err = t.executeHierarchical(ctx)
	case Consensus:
		results, err = t.executeConsensus(ctx)
	default:
		return nil, fmt.Errorf("unsupported process type: %s", t.ProcessType)
	}

	if err != nil {
		// Try to recover from error
		if taskErr, ok := err.(*TaskError); ok && taskErr.Recoverable {
			strategy := t.determineRecoveryStrategy(taskErr)
			if recoverErr := t.RecoverTask(ctx, taskErr, strategy); recoverErr == nil {
				// Share recovery context
				if t.Memory {
					metadata := map[string]interface{}{
						"event":    "task_recovery",
						"task_id":  taskErr.TaskID,
						"strategy": string(strategy),
					}
					if err := t.ShareMemory(ctx, "team", metadata, nil, metadata); err != nil {
						t.logger.Warning("Failed to share recovery context: %v", err)
					}
				}
				// Retry execution
				return t.executeWithProcess(ctx)
			}
		}
	}

	// Share process completion context
	if t.Memory {
		metadata := map[string]interface{}{
			"event":     "process_complete",
			"process":   string(t.ProcessType),
			"timestamp": time.Now(),
			"success":   err == nil,
		}
		if err := t.ShareMemory(ctx, "team", metadata, nil, metadata); err != nil {
			t.logger.Warning("Failed to share process completion context: %v", err)
		}
	}

	return results, err
}

// executeSequential executes tasks in sequence
func (t *Team) executeSequential(ctx context.Context) ([]string, error) {
	results := make([]string, 0, len(t.Tasks))
	for i, task := range t.Tasks {
		// Build context from previous tasks and memory
		var taskContext string
		if i > 0 {
			prevTask := t.Tasks[i-1]
			prevAgent := t.Agents[i-1]

			// Get previous task's result
			prevResult, exists := t.pipeline.GetTaskContext(prevTask.ID, "result")
			if exists {
				if str, ok := prevResult.(string); ok {
					taskContext = fmt.Sprintf("Previous Task: %s\nExecuted by: %s (%s)\nResult: %s",
						prevTask.Description,
						prevAgent.Name,
						prevAgent.Role,
						str)
				}
			}

			// Add execution metadata
			if taskContext != "" {
				taskContext += "\n\nExecution Details:"
				if prevAgentName, exists := t.pipeline.GetTaskContext(prevTask.ID, "agent"); exists {
					taskContext += fmt.Sprintf("\nExecuted by: %v", prevAgentName)
				}
				if prevAgentRole, exists := t.pipeline.GetTaskContext(prevTask.ID, "agent_role"); exists {
					taskContext += fmt.Sprintf("\nAgent Role: %v", prevAgentRole)
				}
				if taskNum, exists := t.pipeline.GetTaskContext(prevTask.ID, "task_number"); exists {
					taskContext += fmt.Sprintf("\nTask Number: %v", taskNum)
				}
			}
		}

		// Add memory context if available
		if t.Memory {
			// Get task-specific context
			if t.contextualMemory != nil {
				memoryContext, err := t.contextualMemory.GetTaskContext(ctx, task.ID, task.Description)
				if err == nil && len(memoryContext) > 0 {
					if items, ok := memoryContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
						taskContext += "\n\nRelevant Memory Context:"
						for _, item := range items {
							if str, ok := item.Data.(string); ok {
								taskContext += fmt.Sprintf("\n- %s", str)
							}
						}
					}
				}
			}

			// Get agent-specific context
			if t.contextualMemory != nil {
				agentContext, err := t.contextualMemory.GetAgentContext(ctx, t.Agents[i].Name, task.Description)
				if err == nil && len(agentContext) > 0 {
					if items, ok := agentContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
						taskContext += "\n\nAgent's Previous Experience:"
						for _, item := range items {
							if str, ok := item.Data.(string); ok {
								taskContext += fmt.Sprintf("\n- %s", str)
							}
						}
					}
				}
			}

			// Get shared context from other agents
			if t.sharedMemory != nil {
				sharedMemories, err := t.GetSharedMemories(ctx, t.Agents[i].Name, task.Description)
				if err == nil && len(sharedMemories) > 0 {
					taskContext += "\n\nShared Knowledge from Other Agents:"
					for _, shared := range sharedMemories {
						if str, ok := shared.Value.(string); ok {
							taskContext += fmt.Sprintf("\n- From %s: %s", shared.Source, str)
						}
					}
				}
			}
		}

		// Execute task with agent
		result, err := t.Agents[i].ExecuteTask(ctx, task.Description, task.ExpectedOutput, taskContext)
		if err != nil {
			return nil, NewTaskError(
				task.ID,
				t.Agents[i].Name,
				err,
				taskContext,
				true, // Most sequential task errors are recoverable
			)
		}

		results = append(results, result)

		// Store comprehensive context
		t.pipeline.SetTaskContext(task.ID, "result", result)
		t.pipeline.SetTaskContext(task.ID, "agent", t.Agents[i].Name)
		t.pipeline.SetTaskContext(task.ID, "agent_role", t.Agents[i].Role)
		t.pipeline.SetTaskContext(task.ID, "task_number", i+1)
		t.pipeline.SetTaskContext(task.ID, "total_tasks", len(t.Tasks))

		// Save to memory if enabled
		if t.Memory {
			metadata := map[string]interface{}{
				"task_id":          task.ID,
				"agent":            t.Agents[i].Name,
				"role":             t.Agents[i].Role,
				"task_description": task.Description,
				"task_number":      i + 1,
				"total_tasks":      len(t.Tasks),
				"timestamp":        time.Now(),
				"status":           "completed",
			}

			// Save to different memory types
			if t.shortTermMemory != nil {
				if err := t.shortTermMemory.Save(result, metadata, t.Agents[i].Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save to short-term memory", "error", err)
				}
			}

			if t.longTermMemory != nil {
				if err := t.longTermMemory.Save(result, metadata, t.Agents[i].Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save to long-term memory", "error", err)
				}
			}

			// Share with other agents
			if t.sharedMemory != nil {
				sharedContext := &memory.SharedMemoryContext{
					Source:    t.Agents[i].Name,
					Value:     result,
					Metadata:  metadata,
					Scope:     memory.TaskMemory,
					Timestamp: time.Now(),
				}
				if err := t.sharedMemory.Share(ctx, sharedContext); err != nil {
					t.logger.Error("Failed to share memory", "error", err)
				}
			}
		}
	}

	return results, nil
}

// executeHierarchical executes tasks with manager oversight
func (t *Team) executeHierarchical(ctx context.Context) ([]string, error) {
	if t.Manager == nil {
		return nil, fmt.Errorf("hierarchical process requires a manager")
	}

	results := make([]string, 0, len(t.Tasks))
	for _, task := range t.Tasks {
		// Build analysis context
		analysisContext := fmt.Sprintf("Current Team State:\nName: %s\nDescription: %s\nProcess Type: %s\n\nAvailable Agents:\n%s",
			t.Name, t.Description, t.ProcessType, t.formatAgentList())

		// Add memory context if available
		if t.Memory {
			// Get task-specific context
			if t.contextualMemory != nil {
				memoryContext, err := t.contextualMemory.GetTaskContext(ctx, task.ID, task.Description)
				if err == nil && len(memoryContext) > 0 {
					if items, ok := memoryContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
						analysisContext += "\n\nRelevant Task History:"
						for _, item := range items {
							if str, ok := item.Data.(string); ok {
								analysisContext += fmt.Sprintf("\n- %s", str)
							}
						}
					}
				}
			}

			// Get manager's previous experiences
			if t.contextualMemory != nil {
				managerContext, err := t.contextualMemory.GetAgentContext(ctx, t.Manager.Name, task.Description)
				if err == nil && len(managerContext) > 0 {
					if items, ok := managerContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
						analysisContext += "\n\nManager's Previous Experience:"
						for _, item := range items {
							if str, ok := item.Data.(string); ok {
								analysisContext += fmt.Sprintf("\n- %s", str)
							}
						}
					}
				}
			}

			// Get shared knowledge
			if t.sharedMemory != nil {
				sharedMemories, err := t.GetSharedMemories(ctx, t.Manager.Name, task.Description)
				if err == nil && len(sharedMemories) > 0 {
					analysisContext += "\n\nShared Knowledge:"
					for _, shared := range sharedMemories {
						if str, ok := shared.Value.(string); ok {
							analysisContext += fmt.Sprintf("\n- From %s: %s", shared.Source, str)
						}
					}
				}
			}
		}

		// Have manager analyze task first
		analysisTask, err := tasks.NewTask(
			fmt.Sprintf("Analyze this task and provide insights:\nTask: %s\nExpected Output: %s\n\nProvide key considerations, potential challenges, and recommended approach for executing this task.",
				task.Description, task.ExpectedOutput),
			t.managerConfig.AnalysisFormat,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create analysis task: %w", err)
		}

		analysis, err := t.Manager.ExecuteTask(ctx, analysisTask.Description, t.managerConfig.AnalysisFormat, analysisContext)
		if err != nil {
			return nil, NewTaskError(
				analysisTask.ID,
				t.Manager.Name,
				err,
				"Task analysis",
				false, // Manager analysis failures are critical
			)
		}

		// Save manager's analysis to memory
		if t.Memory {
			metadata := map[string]interface{}{
				"task_id":          task.ID,
				"agent":            t.Manager.Name,
				"role":             t.Manager.Role,
				"type":             "analysis",
				"task_description": task.Description,
				"timestamp":        time.Now(),
			}

			if t.shortTermMemory != nil {
				if err := t.shortTermMemory.Save(analysis, metadata, t.Manager.Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save analysis to short-term memory", "error", err)
				}
			}

			if t.longTermMemory != nil {
				if err := t.longTermMemory.Save(analysis, metadata, t.Manager.Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save analysis to long-term memory", "error", err)
				}
			}
		}

		// Have manager select an agent
		selectedAgent, err := t.selectAgentForTask(ctx, task)
		if err != nil {
			return nil, NewTaskError(
				task.ID,
				t.Manager.Name,
				err,
				"Agent selection",
				true, // Can retry agent selection
			)
		}

		// Build task context with manager's insights and memory
		taskContext := fmt.Sprintf("Manager's Analysis:\n%s\n\nSelected Agent: %s\nReason: Best suited for this task based on capabilities and role.",
			analysis, selectedAgent.Name)

		// Add agent-specific memory context
		if t.Memory && t.contextualMemory != nil {
			agentContext, err := t.contextualMemory.GetAgentContext(ctx, selectedAgent.Name, task.Description)
			if err == nil && len(agentContext) > 0 {
				if items, ok := agentContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
					taskContext += "\n\nYour Previous Experience:"
					for _, item := range items {
						if str, ok := item.Data.(string); ok {
							taskContext += fmt.Sprintf("\n- %s", str)
						}
					}
				}
			}
		}

		// Execute task with selected agent
		result, err := selectedAgent.ExecuteTask(ctx, task.Description, task.ExpectedOutput, taskContext)
		if err != nil {
			return nil, NewTaskError(
				task.ID,
				selectedAgent.Name,
				err,
				taskContext,
				true, // Worker task failures are recoverable
			)
		}

		// Save task result to memory
		if t.Memory {
			metadata := map[string]interface{}{
				"task_id":          task.ID,
				"agent":            selectedAgent.Name,
				"role":             selectedAgent.Role,
				"manager":          t.Manager.Name,
				"task_description": task.Description,
				"timestamp":        time.Now(),
				"status":           "completed",
			}

			if t.shortTermMemory != nil {
				if err := t.shortTermMemory.Save(result, metadata, selectedAgent.Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save to short-term memory", "error", err)
				}
			}

			if t.longTermMemory != nil {
				if err := t.longTermMemory.Save(result, metadata, selectedAgent.Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save to long-term memory", "error", err)
				}
			}

			// Share with other agents
			if t.sharedMemory != nil {
				sharedContext := &memory.SharedMemoryContext{
					Source:    selectedAgent.Name,
					Value:     result,
					Metadata:  metadata,
					Scope:     memory.TaskMemory,
					Timestamp: time.Now(),
				}
				if err := t.sharedMemory.Share(ctx, sharedContext); err != nil {
					t.logger.Error("Failed to share memory", "error", err)
				}
			}
		}

		// Build review context
		reviewContext := fmt.Sprintf("Original Analysis:\n%s\n\nExecution Context:\nSelected Agent: %s\nAgent Role: %s\n\nTask Execution:\nStart Time: %s\nCompletion Time: %s",
			analysis,
			selectedAgent.Name,
			selectedAgent.Role,
			task.StartedAt.Format(time.RFC3339),
			time.Now().Format(time.RFC3339))

		// Add task history and memory context for review
		if t.Memory {
			if taskHistory, err := t.GetTaskHistory(task.ID); err == nil {
				reviewContext += "\n\nTask History:"
				for key, value := range taskHistory.Context {
					reviewContext += fmt.Sprintf("\n%s: %v", key, value)
				}
			}

			// Get relevant memories for review
			if t.contextualMemory != nil {
				memoryContext, err := t.contextualMemory.GetTaskContext(ctx, task.ID, result)
				if err == nil && len(memoryContext) > 0 {
					if items, ok := memoryContext["items"].([]memory.ContextualMemoryItem); ok && len(items) > 0 {
						reviewContext += "\n\nRelevant Memory Context:"
						for _, item := range items {
							if str, ok := item.Data.(string); ok {
								reviewContext += fmt.Sprintf("\n- %s", str)
							}
						}
					}
				}
			}
		}

		reviewPrompt := fmt.Sprintf("Review this task result:\nTask: %s\nResult: %s\nDoes this meet the expected output: %s\n\nProvide a detailed review",
			task.Description, result, task.ExpectedOutput)
		if t.managerConfig.RequireApproval {
			reviewPrompt += " and respond with APPROVE or REJECT with explanation"
		}
		reviewPrompt += "."

		reviewTask, err := tasks.NewTask(reviewPrompt, t.managerConfig.ReviewFormat)
		if err != nil {
			return nil, fmt.Errorf("failed to create review task: %w", err)
		}

		reviewResult, err := t.Manager.ExecuteTask(ctx, reviewTask.Description, t.managerConfig.ReviewFormat, reviewContext)
		if err != nil {
			return nil, NewTaskError(
				reviewTask.ID,
				t.Manager.Name,
				err,
				"Result review",
				true, // Can retry review
			)
		}

		// Save review to memory
		if t.Memory {
			metadata := map[string]interface{}{
				"task_id":          task.ID,
				"agent":            t.Manager.Name,
				"role":             t.Manager.Role,
				"type":             "review",
				"task_description": task.Description,
				"timestamp":        time.Now(),
			}

			if t.shortTermMemory != nil {
				if err := t.shortTermMemory.Save(reviewResult, metadata, t.Manager.Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save review to short-term memory", "error", err)
				}
			}

			if t.longTermMemory != nil {
				if err := t.longTermMemory.Save(reviewResult, metadata, t.Manager.Name, memory.TaskScope, nil); err != nil {
					t.logger.Error("Failed to save review to long-term memory", "error", err)
				}
			}
		}

		// Check if result is approved (if required)
		if t.managerConfig.RequireApproval {
			if !strings.HasPrefix(strings.ToUpper(reviewResult), "APPROVE") {
				return nil, NewTaskError(
					task.ID,
					selectedAgent.Name,
					fmt.Errorf("manager rejected result: %s", reviewResult),
					taskContext,
					true, // Rejected tasks can be retried
				)
			}
		}

		results = append(results, result)

		// Store comprehensive context
		t.pipeline.SetTaskContext(task.ID, "result", result)
		t.pipeline.SetTaskContext(task.ID, "manager_analysis", analysis)
		t.pipeline.SetTaskContext(task.ID, "selected_agent", selectedAgent.Name)
		t.pipeline.SetTaskContext(task.ID, "manager_review", reviewResult)
	}

	return results, nil
}

// executeConsensus executes tasks with consensus-based execution
func (t *Team) executeConsensus(ctx context.Context) ([]string, error) {
	if len(t.Tasks) < 2 {
		return nil, fmt.Errorf("consensus process requires at least two tasks")
	}

	results := make([]string, 0, len(t.Tasks))
	for _, task := range t.Tasks {
		// Build consensus context
		consensusContext := fmt.Sprintf("Current Team State:\nName: %s\nDescription: %s\nProcess Type: %s\n\nAvailable Agents:\n%s",
			t.Name, t.Description, t.ProcessType, t.formatAgentList())

		// Add memory context if available
		if t.Memory {
			if relevantContext, err := t.Recall(ctx, task.Description); err == nil && len(relevantContext) > 0 {
				consensusContext += "\n\nRelevant Memory Context:"
				for key, value := range relevantContext {
					consensusContext += fmt.Sprintf("\n%s: %v", key, value)
				}
			}
		}

		// Have consensus analysis
		analysis, err := t.analyzeConsensus(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze consensus: %w", err)
		}

		// Have consensus select an agent
		selectedAgent, err := t.selectConsensusAgent(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("failed to select consensus agent: %w", err)
		}

		// Build task context with consensus insights
		taskContext := fmt.Sprintf("Consensus Analysis:\n%s\n\nSelected Agent: %s\nReason: Best suited for this task based on consensus",
			analysis, selectedAgent.Name)

		// Execute task with selected agent
		result, err := selectedAgent.ExecuteTask(ctx, task.Description, task.ExpectedOutput, taskContext)
		if err != nil {
			return nil, NewTaskError(
				task.ID,
				selectedAgent.Name,
				err,
				taskContext,
				true, // Worker task failures are recoverable
			)
		}

		results = append(results, result)

		// Store comprehensive context
		t.pipeline.SetTaskContext(task.ID, "result", result)
		t.pipeline.SetTaskContext(task.ID, "consensus_analysis", analysis)
		t.pipeline.SetTaskContext(task.ID, "selected_agent", selectedAgent.Name)
	}

	return results, nil
}

// analyzeConsensus analyzes consensus-based task requirements
func (t *Team) analyzeConsensus(ctx context.Context, task *tasks.Task) (string, error) {
	// Implementation of consensus analysis logic
	// This is a placeholder and should be replaced with actual implementation
	return "Consensus analysis result", nil
}

// selectConsensusAgent selects an agent based on consensus-based criteria
func (t *Team) selectConsensusAgent(ctx context.Context, task *tasks.Task) (*agents.Agent, error) {
	// Implementation of consensus agent selection logic
	// This is a placeholder and should be replaced with actual implementation
	return t.Agents[0], nil
}

// selectAgentForTask has the manager choose the best agent for a task
func (t *Team) selectAgentForTask(ctx context.Context, task *tasks.Task) (*agents.Agent, error) {
	// Get task priority from metadata or default to medium
	taskPriority := tasks.PriorityNormal
	if priority, ok := task.Metadata["priority"].(tasks.TaskPriority); ok {
		taskPriority = priority
	}

	// Filter available agents based on priority, workload, and delegation compatibility
	availableAgents := make([]*agents.Agent, 0)
	for _, agent := range t.Agents {
		if agent != t.Manager && agent.CanAcceptTask(taskPriority) {
			// Check if manager can delegate to this agent
			if canDelegate, delegationType, reasons := t.Manager.CanDelegate(task, agent); canDelegate {
				availableAgents = append(availableAgents, agent)

				// Store delegation info in task metadata
				if task.Metadata == nil {
					task.Metadata = make(map[string]interface{})
				}
				task.Metadata["delegation_type"] = delegationType
				task.Metadata["delegation_reasons"] = reasons
			}
		}
	}

	if len(availableAgents) == 0 {
		return nil, fmt.Errorf("no available agents for task with priority %v", taskPriority)
	}

	// Create agent selection criteria for manager
	var selectionInfo string
	for i, agent := range availableAgents {
		selectionInfo += fmt.Sprintf("%d. %s (Role: %s)\n", i, agent.Name, agent.Role)
		selectionInfo += fmt.Sprintf("   Priority: %v, Current Tasks: %d/%d\n", agent.Priority, agent.CurrentTasks, agent.MaxWorkload)
		selectionInfo += fmt.Sprintf("   Performance Score: %.2f\n", agent.GetPerformanceScore())
		selectionInfo += fmt.Sprintf("   Delegation Score: %.2f\n", agent.GetDelegationScore())
		selectionInfo += fmt.Sprintf("   Collaboration Score: %.2f\n", agent.CollaborationScore)

		// Add recent collaborations
		if len(agent.LastCollaborators) > 0 {
			selectionInfo += "   Recent Collaborations:\n"
			for _, collaborator := range agent.LastCollaborators {
				selectionInfo += fmt.Sprintf("   - %s\n", collaborator)
			}
		}

		// Add delegation type if available
		if delegationType, ok := task.Metadata["delegation_type"].(agents.DelegationType); ok {
			selectionInfo += fmt.Sprintf("   Delegation Type: %s\n", delegationType)
		}

		// Add relevant performance metrics
		selectionInfo += "   Recent Performance:\n"
		for metric, value := range agent.Performance {
			selectionInfo += fmt.Sprintf("   - %s: %.2f\n", metric, value)
		}
		selectionInfo += "\n"
	}

	// Create selection task for manager
	selectionTask, err := tasks.NewTask(
		fmt.Sprintf(`Select the best agent for this task:
Task Description: %s
Expected Output: %s
Priority Level: %v

Available Agents:
%s

Consider the following criteria:
1. Agent's current workload and capacity
2. Performance history and expertise
3. Priority level match
4. Role suitability
5. Delegation compatibility
6. Collaboration history
7. Supervision requirements

Respond with the index (0-%d) of the chosen agent and a brief explanation of your choice.`,
			task.Description,
			task.ExpectedOutput,
			taskPriority,
			selectionInfo,
			len(availableAgents)-1),
		"Agent index and selection rationale",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create selection task: %w", err)
	}

	// Have manager execute selection task
	result, err := t.Manager.ExecuteTask(ctx, selectionTask.Description, "Agent index and selection rationale", "")
	if err != nil {
		return nil, fmt.Errorf("manager failed to select agent: %w", err)
	}

	// Parse the agent index from the result
	var index int
	if _, err := fmt.Sscanf(result, "%d", &index); err != nil {
		return nil, fmt.Errorf("invalid agent selection result: %s", result)
	}

	if index < 0 || index >= len(availableAgents) {
		return nil, fmt.Errorf("invalid agent index: %d", index)
	}

	selectedAgent := availableAgents[index]

	// Update task metadata with selection info
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	task.Metadata["selected_agent_priority"] = selectedAgent.Priority
	task.Metadata["selected_agent_workload"] = selectedAgent.CurrentTasks
	task.Metadata["selected_agent_performance"] = selectedAgent.GetPerformanceScore()
	task.Metadata["selected_agent_delegation_score"] = selectedAgent.GetDelegationScore()
	task.Metadata["selected_agent_collaboration_score"] = selectedAgent.CollaborationScore

	// Increment the selected agent's task count
	selectedAgent.IncrementTasks()

	// Record collaboration if needed
	if delegationType, ok := task.Metadata["delegation_type"].(agents.DelegationType); ok {
		if delegationType == agents.CollaborativeDelegation {
			selectedAgent.AddCollaborator(t.Manager.Name)
			t.Manager.AddCollaborator(selectedAgent.Name)
		}
	}

	return selectedAgent, nil
}

// formatAgentList creates a formatted string of agent descriptions
func (t *Team) formatAgentList() string {
	var result string
	for i, agent := range t.Agents {
		result += fmt.Sprintf("%d. %s (Role: %s)\n", i, agent.Name, agent.Role)
	}
	return result
}

// GetTaskOutput retrieves a task's output from storage
func (t *Team) GetTaskOutput(taskID string) (*tasks.TaskOutput, error) {
	return t.pipeline.GetTaskOutput(taskID)
}

// ListTaskOutputs returns all stored task outputs
func (t *Team) ListTaskOutputs() []*tasks.TaskOutput {
	return t.pipeline.ListTaskOutputs()
}

// QueryTaskOutputs searches for outputs matching the given criteria
func (t *Team) QueryTaskOutputs(criteria map[string]interface{}) []*tasks.TaskOutput {
	return t.pipeline.QueryTaskOutputs(criteria)
}

// DeleteTaskOutput removes a task's output from storage
func (t *Team) DeleteTaskOutput(taskID string) error {
	return t.pipeline.DeleteTaskOutput(taskID)
}

// GetTaskHistory returns the execution history of a task
func (t *Team) GetTaskHistory(taskID string) (*tasks.TaskOutput, error) {
	output, err := t.pipeline.GetTaskOutput(taskID)
	if err != nil {
		return nil, err
	}

	// Add agent information to metadata
	if agentName, exists := output.Context["agent"]; exists {
		output.Metadata["agent_name"] = agentName
	}
	if agentRole, exists := output.Context["agent_role"]; exists {
		output.Metadata["agent_role"] = agentRole
	}

	// Add manager information for hierarchical tasks
	if t.ProcessType == Hierarchical {
		if selectedAgent, exists := output.Context["selected_agent"]; exists {
			output.Metadata["selected_agent"] = selectedAgent
		}
		if managerReview, exists := output.Context["manager_review"]; exists {
			output.Metadata["manager_review"] = managerReview
		}
	}

	return output, nil
}

// CreateTask creates a new task in the team
func (t *Team) CreateTask(description string, expectedOutput string, opts ...tasks.TaskOption) (*tasks.Task, error) {
	task, err := tasks.NewTask(description, expectedOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Apply any additional options
	for _, opt := range opts {
		opt(task)
	}

	t.Tasks = append(t.Tasks, task)
	return task, nil
}

// AssignTask assigns a task to an agent and sets up the execution function
func (t *Team) AssignTask(ctx context.Context, task *tasks.Task, agent *agents.Agent) error {
	if task == nil || agent == nil {
		return fmt.Errorf("task and agent cannot be nil")
	}

	// Set up the execution function that will use the agent
	task.ExecuteFunc = func(execCtx context.Context) (string, error) {
		result, err := agent.ExecuteTask(execCtx, task.Description, task.ExpectedOutput, task.Context)
		if err != nil {
			return "", fmt.Errorf("agent execution failed: %w", err)
		}
		return result, nil
	}

	// Store assignment in pipeline
	t.pipeline.SetTaskContext(task.ID, "assigned_agent", agent.Name)

	return nil
}

// Pipeline returns the task pipeline
func (t *Team) Pipeline() *tasks.Pipeline {
	return t.pipeline
}

// RecoverTask attempts to recover from a task error
func (t *Team) RecoverTask(ctx context.Context, taskErr *TaskError, strategy RecoveryStrategy) error {
	task := t.findTask(taskErr.TaskID)
	if task == nil {
		return fmt.Errorf("task not found: %s", taskErr.TaskID)
	}

	t.logger.Warning("Attempting to recover task %s using strategy: %s", task.ID, strategy)

	switch strategy {
	case RetryWithSameAgent:
		agent := t.findAgent(taskErr.AgentName)
		if agent == nil {
			return fmt.Errorf("agent not found: %s", taskErr.AgentName)
		}
		return t.retryTask(ctx, task, agent)

	case RetryWithDifferentAgent:
		// Have manager select a different agent
		newAgent, err := t.selectAgentForTask(ctx, task)
		if err != nil {
			return fmt.Errorf("failed to select new agent: %w", err)
		}
		if newAgent.Name == taskErr.AgentName {
			return fmt.Errorf("selected same agent for retry")
		}
		return t.retryTask(ctx, task, newAgent)

	case EscalateToManager:
		return t.escalateTask(ctx, task, taskErr)

	default:
		return fmt.Errorf("unknown recovery strategy: %s", strategy)
	}
}

// retryTask attempts to retry a failed task
func (t *Team) retryTask(ctx context.Context, task *tasks.Task, agent *agents.Agent) error {
	// Update task context with error information
	if task.Context != "" {
		task.Context += "\n\n"
	}
	task.Context += "Previous attempt failed. Please review and avoid similar issues."

	// Reset task status
	task.Status = tasks.TaskStatusPending
	task.Error = nil
	task.Result = ""

	// Reassign task
	return t.AssignTask(ctx, task, agent)
}

// escalateTask escalates a failed task to the manager
func (t *Team) escalateTask(ctx context.Context, task *tasks.Task, taskErr *TaskError) error {
	if t.Manager == nil {
		return fmt.Errorf("no manager available for escalation")
	}

	// Create analysis task for manager
	analysisTask, err := tasks.NewTask(
		fmt.Sprintf("Analyze this failed task and provide solution:\nTask: %s\nError: %s\nContext: %s",
			task.Description, taskErr.err.Error(), taskErr.Context),
		"Analysis and solution for failed task",
	)
	if err != nil {
		return fmt.Errorf("failed to create analysis task: %w", err)
	}

	// Have manager analyze the failure
	analysis, err := t.Manager.ExecuteTask(ctx, analysisTask.Description, "Analysis and solution for failed task", "")
	if err != nil {
		return fmt.Errorf("manager analysis failed: %w", err)
	}

	// Update task context with manager's analysis
	if task.Context != "" {
		task.Context += "\n\n"
	}
	task.Context += fmt.Sprintf("Manager's Analysis:\n%s", analysis)

	// Have manager select new agent
	newAgent, err := t.selectAgentForTask(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to select new agent: %w", err)
	}

	// Retry task with new agent
	return t.retryTask(ctx, task, newAgent)
}

// findTask finds a task by ID
func (t *Team) findTask(taskID string) *tasks.Task {
	for _, task := range t.Tasks {
		if task.ID == taskID {
			return task
		}
	}
	return nil
}

// findAgent finds an agent by name
func (t *Team) findAgent(name string) *agents.Agent {
	for _, agent := range t.Agents {
		if agent.Name == name {
			return agent
		}
	}
	return nil
}

// determineRecoveryStrategy determines the best recovery strategy for a task error
func (t *Team) determineRecoveryStrategy(taskErr *TaskError) RecoveryStrategy {
	errStr := taskErr.err.Error()

	// If it's a simple error, retry with same agent
	if strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "timeout") {
		return RetryWithSameAgent
	}

	// If it's a capability error, try different agent
	if strings.Contains(errStr, "capability") ||
		strings.Contains(errStr, "unable to complete") {
		return RetryWithDifferentAgent
	}

	// For complex errors, escalate to manager
	return EscalateToManager
}

// SaveMemory saves a memory item to both short-term and long-term storage
func (t *Team) SaveMemory(value string, metadata map[string]interface{}, source string, scope memory.MemoryScope) error {
	if !t.Memory {
		return nil
	}

	// Calculate expiration based on config
	var expiresAt *time.Time
	if t.memoryConfig.DefaultTTL > 0 {
		t := time.Now().Add(t.memoryConfig.DefaultTTL)
		expiresAt = &t
	}

	switch scope {
	case memory.ScopeShortTerm:
		// Save to short-term memory
		if err := t.shortTermMemory.Save(value, metadata, source, scope, expiresAt); err != nil {
			return fmt.Errorf("failed to save to short-term memory: %w", err)
		}
	case memory.ScopeLongTerm:
		// Save to long-term memory
		if err := t.longTermMemory.Save(value, metadata, source, scope, expiresAt); err != nil {
			return fmt.Errorf("failed to save to long-term memory: %w", err)
		}
	}

	return nil
}

// ShareMemory shares a memory context with other agents
func (t *Team) ShareMemory(ctx context.Context, source string, value interface{}, targets []string, metadata map[string]interface{}) error {
	if !t.Memory {
		return nil
	}

	// Convert value to string if needed
	valueStr := fmt.Sprintf("%v", value)

	// Save to memory storage
	if err := t.SaveMemory(valueStr, metadata, source, memory.TaskScope); err != nil {
		return err
	}

	// Share through shared memory
	memory := &memory.SharedMemoryContext{
		Source:    source,
		Target:    targets,
		Value:     value,
		Metadata:  metadata,
		Scope:     memory.TaskMemory,
		Timestamp: time.Now(),
	}

	return t.sharedMemory.Share(ctx, memory)
}

// GetSharedMemories retrieves shared memories accessible to an agent
func (t *Team) GetSharedMemories(ctx context.Context, agentID string, query string) ([]*memory.SharedMemoryContext, error) {
	if !t.Memory {
		return nil, nil
	}

	return t.sharedMemory.Access(ctx, agentID, query, memory.TaskMemory)
}

// ExecuteTask executes a task with the team
func (t *Team) ExecuteTask(ctx context.Context, task *tasks.Task) error {
	// Ensure task has an assigned agent
	if task.AssignedAgent == "" {
		// If no agent assigned and we have a manager, assign to manager
		if t.Manager != nil {
			task.AssignedAgent = t.Manager.Name
		} else if len(t.Agents) > 0 {
			// Otherwise assign to first available agent
			task.AssignedAgent = t.Agents[0].Name
		} else {
			return fmt.Errorf("no agents available to execute task")
		}
	}

	// Find assigned agent
	var agent *agents.Agent
	for _, a := range t.Agents {
		if a.Name == task.AssignedAgent {
			agent = a
			break
		}
	}

	if agent == nil {
		return fmt.Errorf("assigned agent %s not found in team", task.AssignedAgent)
	}

	// Set up task execution function
	task.ExecuteFunc = func(ctx context.Context) (string, error) {
		output, err := agent.ExecuteTask(ctx, task.Description, task.ExpectedOutput, task.Context)
		if err != nil {
			return "", err
		}
		task.Result = output // Set the task result
		return output, nil
	}

	// Set up task timing if not already set
	if task.StartedAt == nil {
		now := time.Now()
		task.StartedAt = &now
	}

	// Execute the task with the assigned agent
	output, err := task.ExecuteFunc(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Save task result to memory if enabled
	if t.Memory {
		metadata := map[string]interface{}{
			"task_id": task.ID,
			"agent":   task.AssignedAgent,
			"status":  "completed",
			"type":    memory.TaskMemory,
		}

		// Save to short-term memory
		if err := t.SaveMemory(output, metadata, task.AssignedAgent, memory.ScopeShortTerm); err != nil {
			t.logger.Warning("Failed to save to short-term memory", "error", err)
		}

		// Save to long-term memory
		if err := t.SaveMemory(output, metadata, task.AssignedAgent, memory.ScopeLongTerm); err != nil {
			t.logger.Warning("Failed to save to long-term memory", "error", err)
		}
	}

	return nil
}
