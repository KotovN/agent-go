package types

import (
	"time"
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
	InvolvedAgents  []string // List of agent IDs
	RelatedTaskID   string
	StartTime       time.Time
	ResolutionTime  *time.Time
	Status          string
	Resolution      ResolutionStrategy
	Context         map[string]interface{}
	ResolutionNotes string
}
