package team

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KotovN/agent-go/pkg/agents"
	"github.com/KotovN/agent-go/pkg/tasks"
	"github.com/KotovN/agent-go/pkg/types"
)

// ConflictType represents different types of conflicts that can occur
type ConflictType string

const (
	// TaskExecutionConflict occurs when agents disagree on task execution approach
	TaskExecutionConflict ConflictType = "task_execution"
	// ResourceConflict occurs when multiple agents compete for resources
	ResourceConflict ConflictType = "resource"
	// PriorityConflict occurs when there's disagreement about task priorities
	PriorityConflict ConflictType = "priority"
	// CapabilityConflict occurs when there's disagreement about agent capabilities
	CapabilityConflict ConflictType = "capability"
	// DecisionConflict occurs when agents disagree on decisions
	DecisionConflict ConflictType = "decision"
)

// ResolutionStrategy defines how to resolve a conflict
type ResolutionStrategy string

const (
	// ConsensusResolution uses voting to reach agreement
	ConsensusResolution ResolutionStrategy = "consensus"
	// HierarchicalResolution escalates to manager for decision
	HierarchicalResolution ResolutionStrategy = "hierarchical"
	// MediationResolution uses a third-party agent to mediate
	MediationResolution ResolutionStrategy = "mediation"
	// CompromiseResolution attempts to find middle ground
	CompromiseResolution ResolutionStrategy = "compromise"
	// FallbackResolution uses predefined fallback approach
	FallbackResolution ResolutionStrategy = "fallback"
)

// Conflict represents a conflict between agents
type Conflict struct {
	ID              string
	Type            ConflictType
	Description     string
	InvolvedAgents  []*agents.Agent
	RelatedTask     *tasks.Task
	StartTime       time.Time
	ResolutionTime  *time.Time
	Status          string
	Resolution      ResolutionStrategy
	VotingSession   *VotingSession
	Context         map[string]interface{}
	ResolutionNotes string
}

// ConflictManager handles conflict detection and resolution
type ConflictManager struct {
	activeConflicts map[string]*types.Conflict
	history         []*types.Conflict
	team            *Team
	mu              sync.RWMutex

	// Configuration
	defaultStrategy types.ResolutionStrategy
	escalationPath  []types.ResolutionStrategy
	maxAttempts     int
}

// NewConflictManager creates a new conflict manager
func NewConflictManager(team *Team) *ConflictManager {
	return &ConflictManager{
		activeConflicts: make(map[string]*types.Conflict),
		history:         make([]*types.Conflict, 0),
		team:            team,
		defaultStrategy: types.ConsensusResolution,
		escalationPath: []types.ResolutionStrategy{
			types.ConsensusResolution,
			types.MediationResolution,
			types.HierarchicalResolution,
			types.FallbackResolution,
		},
		maxAttempts: 3,
	}
}

// DetectConflicts checks for potential conflicts in the current state
func (cm *ConflictManager) DetectConflicts(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check for task execution conflicts
	for _, task := range cm.team.Tasks {
		if err := cm.detectTaskConflicts(ctx, task); err != nil {
			return fmt.Errorf("failed to detect task conflicts: %w", err)
		}
	}

	// Check for resource conflicts
	if err := cm.detectResourceConflicts(ctx); err != nil {
		return fmt.Errorf("failed to detect resource conflicts: %w", err)
	}

	// Check for priority conflicts
	if err := cm.detectPriorityConflicts(ctx); err != nil {
		return fmt.Errorf("failed to detect priority conflicts: %w", err)
	}

	return nil
}

// detectTaskConflicts checks for conflicts in task execution
func (cm *ConflictManager) detectTaskConflicts(ctx context.Context, task *tasks.Task) error {
	// Get task execution approach from each involved agent
	approaches := make(map[string]string)
	for _, agent := range cm.team.Agents {
		approach, err := agent.GetTaskApproach(ctx, task)
		if err != nil {
			return fmt.Errorf("failed to get task approach from agent %s: %w", agent.Name, err)
		}
		approaches[agent.Name] = approach
	}

	// Check for conflicting approaches
	if cm.hasConflictingApproaches(approaches) {
		agentIDs := make([]string, 0, len(cm.team.Agents))
		for _, agent := range cm.team.Agents {
			agentIDs = append(agentIDs, agent.Name)
		}

		conflict := &types.Conflict{
			ID:             fmt.Sprintf("conflict_%s_%d", task.ID, time.Now().UnixNano()),
			Type:           types.TaskExecutionConflict,
			Description:    "Conflicting approaches to task execution",
			InvolvedAgents: agentIDs,
			RelatedTaskID:  task.ID,
			StartTime:      time.Now(),
			Status:         "detected",
			Context: map[string]interface{}{
				"approaches": approaches,
			},
		}
		cm.activeConflicts[conflict.ID] = conflict
	}

	return nil
}

// detectResourceConflicts checks for conflicts in resource usage
func (cm *ConflictManager) detectResourceConflicts(ctx context.Context) error {
	// Track resource requests per agent
	resourceRequests := make(map[string]map[string]int)

	for _, agent := range cm.team.Agents {
		for _, task := range cm.team.Tasks {
			if reqs, err := agent.GetResourceRequirements(ctx, task); err == nil {
				if resourceRequests[agent.Name] == nil {
					resourceRequests[agent.Name] = make(map[string]int)
				}
				for resource, amount := range reqs {
					resourceRequests[agent.Name][resource] += amount
				}
			}
		}
	}

	// Check for resource contention
	if conflicts := cm.findResourceConflicts(resourceRequests); len(conflicts) > 0 {
		for _, conflictInfo := range conflicts {
			agentIDs := make([]string, 0, len(conflictInfo.agents))
			for _, agent := range conflictInfo.agents {
				agentIDs = append(agentIDs, agent.Name)
			}

			conflict := &types.Conflict{
				ID:             fmt.Sprintf("resource_conflict_%d", time.Now().UnixNano()),
				Type:           types.ResourceConflict,
				Description:    conflictInfo.description,
				InvolvedAgents: agentIDs,
				StartTime:      time.Now(),
				Status:         "detected",
				Context: map[string]interface{}{
					"resource_requests": resourceRequests,
					"conflict_details":  conflictInfo,
				},
			}
			cm.activeConflicts[conflict.ID] = conflict
		}
	}

	return nil
}

// detectPriorityConflicts checks for conflicts in task priorities
func (cm *ConflictManager) detectPriorityConflicts(ctx context.Context) error {
	priorityAssessments := make(map[string]map[string]tasks.TaskPriority)

	for _, agent := range cm.team.Agents {
		priorityAssessments[agent.Name] = make(map[string]tasks.TaskPriority)
		for _, task := range cm.team.Tasks {
			priority := agent.AssessTaskPriority(task)
			priorityAssessments[agent.Name][task.ID] = priority
		}
	}

	// Check for significant priority disagreements
	if conflicts := cm.findPriorityConflicts(priorityAssessments); len(conflicts) > 0 {
		for _, conflictInfo := range conflicts {
			agentIDs := make([]string, 0, len(conflictInfo.agents))
			for _, agent := range conflictInfo.agents {
				agentIDs = append(agentIDs, agent.Name)
			}

			conflict := &types.Conflict{
				ID:             fmt.Sprintf("priority_conflict_%d", time.Now().UnixNano()),
				Type:           types.PriorityConflict,
				Description:    conflictInfo.description,
				InvolvedAgents: agentIDs,
				StartTime:      time.Now(),
				Status:         "detected",
				Context: map[string]interface{}{
					"priority_assessments": priorityAssessments,
					"conflict_details":     conflictInfo,
				},
			}
			cm.activeConflicts[conflict.ID] = conflict
		}
	}

	return nil
}

// ResolveConflict attempts to resolve a conflict using the appropriate strategy
func (cm *ConflictManager) ResolveConflict(ctx context.Context, conflictID string) error {
	cm.mu.Lock()
	conflict, exists := cm.activeConflicts[conflictID]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("conflict %s not found", conflictID)
	}
	cm.mu.Unlock()

	// Try each resolution strategy in the escalation path
	for _, strategy := range cm.escalationPath {
		if err := cm.applyResolutionStrategy(ctx, conflict, strategy); err != nil {
			continue // Try next strategy if current one fails
		}

		// If resolution was successful
		cm.mu.Lock()
		now := time.Now()
		conflict.ResolutionTime = &now
		conflict.Status = "resolved"
		conflict.Resolution = strategy

		// Move to history
		cm.history = append(cm.history, conflict)
		delete(cm.activeConflicts, conflictID)
		cm.mu.Unlock()

		return nil
	}

	return fmt.Errorf("failed to resolve conflict after trying all strategies")
}

// applyResolutionStrategy applies a specific resolution strategy to a conflict
func (cm *ConflictManager) applyResolutionStrategy(ctx context.Context, conflict *types.Conflict, strategy types.ResolutionStrategy) error {
	switch strategy {
	case types.ConsensusResolution:
		return cm.resolveByConsensus(ctx, conflict)
	case types.HierarchicalResolution:
		return cm.resolveByHierarchy(ctx, conflict)
	case types.MediationResolution:
		return cm.resolveByMediation(ctx, conflict)
	case types.CompromiseResolution:
		return cm.resolveByCompromise(ctx, conflict)
	case types.FallbackResolution:
		return cm.resolveByFallback(ctx, conflict)
	default:
		return fmt.Errorf("unknown resolution strategy: %s", strategy)
	}
}

// resolveByConsensus uses voting to resolve the conflict
func (cm *ConflictManager) resolveByConsensus(ctx context.Context, conflict *types.Conflict) error {
	// Get the related task
	var task *tasks.Task
	for _, t := range cm.team.Tasks {
		if t.ID == conflict.RelatedTaskID {
			task = t
			break
		}
	}
	if task == nil {
		return fmt.Errorf("related task not found")
	}

	// Have agents vote on proposed solutions
	consensusReached, err := cm.team.HandleVotingRound(ctx, task)
	if err != nil {
		return fmt.Errorf("voting round failed: %w", err)
	}

	if consensusReached {
		conflict.ResolutionNotes = "Resolved through consensus voting"
		return nil
	}

	return fmt.Errorf("consensus not reached")
}

// resolveByHierarchy escalates to manager for resolution
func (cm *ConflictManager) resolveByHierarchy(ctx context.Context, conflict *types.Conflict) error {
	if cm.team.Manager == nil {
		return fmt.Errorf("no manager available for hierarchical resolution")
	}

	decision, err := cm.team.Manager.ResolveConflict(ctx, conflict)
	if err != nil {
		return fmt.Errorf("manager failed to resolve conflict: %w", err)
	}

	conflict.ResolutionNotes = fmt.Sprintf("Resolved by manager: %s", decision)
	return nil
}

// resolveByMediation uses a third-party agent to mediate
func (cm *ConflictManager) resolveByMediation(ctx context.Context, conflict *types.Conflict) error {
	mediator, err := cm.selectMediator(conflict)
	if err != nil {
		return fmt.Errorf("failed to select mediator: %w", err)
	}

	resolution, err := mediator.Mediate(ctx, conflict)
	if err != nil {
		return fmt.Errorf("mediation failed: %w", err)
	}

	conflict.ResolutionNotes = fmt.Sprintf("Resolved through mediation: %s", resolution)
	return nil
}

// resolveByCompromise attempts to find middle ground
func (cm *ConflictManager) resolveByCompromise(ctx context.Context, conflict *types.Conflict) error {
	compromise, err := cm.findCompromise(ctx, conflict)
	if err != nil {
		return fmt.Errorf("failed to find compromise: %w", err)
	}

	conflict.ResolutionNotes = fmt.Sprintf("Resolved through compromise: %s", compromise)
	return nil
}

// resolveByFallback uses predefined fallback approach
func (cm *ConflictManager) resolveByFallback(ctx context.Context, conflict *types.Conflict) error {
	// Use most conservative/safe approach
	switch conflict.Type {
	case types.TaskExecutionConflict:
		return cm.fallbackTaskExecution(ctx, conflict)
	case types.ResourceConflict:
		return cm.fallbackResourceAllocation(ctx, conflict)
	case types.PriorityConflict:
		return cm.fallbackPriority(ctx, conflict)
	default:
		return fmt.Errorf("no fallback defined for conflict type: %s", conflict.Type)
	}
}

// Helper methods for conflict detection and resolution
func (cm *ConflictManager) hasConflictingApproaches(approaches map[string]string) bool {
	if len(approaches) <= 1 {
		return false
	}

	// Compare approaches pairwise
	var baseline string
	for _, approach := range approaches {
		if baseline == "" {
			baseline = approach
			continue
		}
		if approach != baseline {
			return true
		}
	}
	return false
}

type conflictInfo struct {
	description string
	agents      []*agents.Agent
}

func (cm *ConflictManager) findResourceConflicts(requests map[string]map[string]int) []conflictInfo {
	conflicts := make([]conflictInfo, 0)

	// Sum total requests per resource
	totalRequests := make(map[string]int)
	for _, agentRequests := range requests {
		for resource, amount := range agentRequests {
			totalRequests[resource] += amount
		}
	}

	// Check against available resources (simplified)
	for resource, total := range totalRequests {
		if total > 100 { // Assuming 100 is max capacity
			conflict := conflictInfo{
				description: fmt.Sprintf("Resource contention for %s: total requests %d exceed capacity", resource, total),
				agents:      make([]*agents.Agent, 0),
			}

			// Find agents involved in this conflict
			for _, agent := range cm.team.Agents {
				if requests[agent.Name][resource] > 0 {
					conflict.agents = append(conflict.agents, agent)
				}
			}

			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

func (cm *ConflictManager) findPriorityConflicts(assessments map[string]map[string]tasks.TaskPriority) []conflictInfo {
	conflicts := make([]conflictInfo, 0)

	// Compare priority assessments between agents
	for taskID := range assessments[cm.team.Agents[0].Name] {
		priorityDiff := false
		baseline := assessments[cm.team.Agents[0].Name][taskID]

		for _, agent := range cm.team.Agents[1:] {
			if assessments[agent.Name][taskID] != baseline {
				priorityDiff = true
				break
			}
		}

		if priorityDiff {
			conflict := conflictInfo{
				description: fmt.Sprintf("Priority conflict for task %s: agents disagree on priority level", taskID),
				agents:      cm.team.Agents,
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// Fallback resolution methods
func (cm *ConflictManager) fallbackTaskExecution(ctx context.Context, conflict *types.Conflict) error {
	// Use most conservative approach
	conflict.ResolutionNotes = "Used fallback: most conservative task execution approach"
	return nil
}

func (cm *ConflictManager) fallbackResourceAllocation(ctx context.Context, conflict *types.Conflict) error {
	// Allocate resources equally
	conflict.ResolutionNotes = "Used fallback: equal resource allocation"
	return nil
}

func (cm *ConflictManager) fallbackPriority(ctx context.Context, conflict *types.Conflict) error {
	// Use highest priority level suggested
	conflict.ResolutionNotes = "Used fallback: highest suggested priority level"
	return nil
}

// selectMediator chooses an appropriate mediator for the conflict
func (cm *ConflictManager) selectMediator(conflict *types.Conflict) (*agents.Agent, error) {
	// Select agent with highest collaboration score who isn't involved
	var bestMediator *agents.Agent
	var bestScore float64

	for _, agent := range cm.team.Agents {
		// Skip agents involved in the conflict
		involved := false
		for _, agentID := range conflict.InvolvedAgents {
			if agent.Name == agentID {
				involved = true
				break
			}
		}
		if involved {
			continue
		}

		// Check if this agent has a better collaboration score
		if agent.CollaborationScore > bestScore {
			bestScore = agent.CollaborationScore
			bestMediator = agent
		}
	}

	if bestMediator == nil {
		return nil, fmt.Errorf("no suitable mediator found")
	}

	return bestMediator, nil
}

// findCompromise attempts to find a middle ground solution
func (cm *ConflictManager) findCompromise(ctx context.Context, conflict *types.Conflict) (string, error) {
	switch conflict.Type {
	case types.TaskExecutionConflict:
		return cm.findTaskExecutionCompromise(ctx, conflict)
	case types.ResourceConflict:
		return cm.findResourceCompromise(ctx, conflict)
	case types.PriorityConflict:
		return cm.findPriorityCompromise(ctx, conflict)
	default:
		return "", fmt.Errorf("no compromise strategy for conflict type: %s", conflict.Type)
	}
}

func (cm *ConflictManager) findTaskExecutionCompromise(ctx context.Context, conflict *types.Conflict) (string, error) {
	// Analyze approaches and find common ground
	approaches := conflict.Context["approaches"].(map[string]string)
	commonElements := cm.findCommonElements(approaches)

	if len(commonElements) > 0 {
		return fmt.Sprintf("Compromise solution incorporating common elements: %v", commonElements), nil
	}

	return "", fmt.Errorf("no common ground found in task execution approaches")
}

func (cm *ConflictManager) findResourceCompromise(ctx context.Context, conflict *types.Conflict) (string, error) {
	// Calculate fair resource allocation
	requests := conflict.Context["resource_requests"].(map[string]map[string]int)
	allocation := cm.calculateFairAllocation(requests)

	return fmt.Sprintf("Fair resource allocation: %v", allocation), nil
}

func (cm *ConflictManager) findPriorityCompromise(ctx context.Context, conflict *types.Conflict) (string, error) {
	// Find middle ground in priority assessments
	assessments := conflict.Context["priority_assessments"].(map[string]map[string]tasks.TaskPriority)
	compromise := cm.calculatePriorityCompromise(assessments)

	return fmt.Sprintf("Priority compromise: %v", compromise), nil
}

// Helper methods for compromise finding
func (cm *ConflictManager) findCommonElements(approaches map[string]string) []string {
	// Implementation would analyze approaches and extract common elements
	return []string{"common_element_1", "common_element_2"}
}

func (cm *ConflictManager) calculateFairAllocation(requests map[string]map[string]int) map[string]int {
	// Implementation would calculate fair resource allocation
	return map[string]int{"resource1": 50, "resource2": 50}
}

func (cm *ConflictManager) calculatePriorityCompromise(assessments map[string]map[string]tasks.TaskPriority) tasks.TaskPriority {
	// Implementation would calculate priority compromise
	return tasks.PriorityNormal
}
