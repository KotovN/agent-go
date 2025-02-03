package capabilities

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Capability represents a specific skill or ability an agent has
type Capability string

const (
	// Core capabilities
	CapabilityTextProcessing  Capability = "text_processing"
	CapabilityCodeAnalysis    Capability = "code_analysis"
	CapabilityMathComputation Capability = "math_computation"

	// Task-specific capabilities
	CapabilityTaskPlanning   Capability = "task_planning"
	CapabilityTaskDelegation Capability = "task_delegation"
	CapabilityTaskMonitoring Capability = "task_monitoring"

	// Domain-specific capabilities
	CapabilityDataAnalysis    Capability = "data_analysis"
	CapabilityImageProcessing Capability = "image_processing"
	CapabilityNLPProcessing   Capability = "nlp_processing"
)

// CapabilityLevel represents how proficient an agent is at a capability
type CapabilityLevel float64

const (
	CapabilityLevelNone     CapabilityLevel = 0.0
	CapabilityLevelBasic    CapabilityLevel = 0.3
	CapabilityLevelModerate CapabilityLevel = 0.6
	CapabilityLevelAdvanced CapabilityLevel = 0.9
	CapabilityLevelExpert   CapabilityLevel = 1.0
)

// AgentCapabilities represents the set of capabilities an agent has
type AgentCapabilities struct {
	// Capabilities maps capability names to proficiency levels
	Capabilities map[Capability]CapabilityLevel

	// Priority indicates the agent's priority level for task assignment
	Priority int

	// MaxConcurrentTasks is the maximum number of tasks this agent can handle
	MaxConcurrentTasks int

	// CurrentTasks is the number of tasks currently assigned
	CurrentTasks int
}

// NewAgentCapabilities creates a new capabilities instance with defaults
func NewAgentCapabilities() *AgentCapabilities {
	return &AgentCapabilities{
		Capabilities:       make(map[Capability]CapabilityLevel),
		Priority:           1,
		MaxConcurrentTasks: 5,
	}
}

// HasCapability checks if the agent has the required capability at minimum level
func (ac *AgentCapabilities) HasCapability(cap Capability, minLevel CapabilityLevel) bool {
	level, exists := ac.Capabilities[cap]
	return exists && level >= minLevel
}

// CanAcceptTask checks if the agent can take on another task
func (ac *AgentCapabilities) CanAcceptTask() bool {
	return ac.CurrentTasks < ac.MaxConcurrentTasks
}

// TaskRequirements represents the capabilities required for a task
// TaskRequirements defines what capabilities are needed for a task
type TaskRequirements struct {
	Required map[Capability]CapabilityLevel
	Optional map[Capability]CapabilityLevel
	Priority int
	Deadline *time.Time
}

// NewTaskRequirements creates a new requirements instance
func NewTaskRequirements() *TaskRequirements {
	return &TaskRequirements{
		Required: make(map[Capability]CapabilityLevel),
		Optional: make(map[Capability]CapabilityLevel),
		Priority: 1,
	}
}

// AgentMatch represents how well an agent matches task requirements
type AgentMatch struct {
	AgentID string
	Score   float64
	Reasons []string
}

// CalculateMatch determines how well an agent matches task requirements
func CalculateMatch(agentID string, caps *AgentCapabilities, reqs *TaskRequirements) *AgentMatch {
	match := &AgentMatch{
		AgentID: agentID,
		Score:   0.0,
		Reasons: make([]string, 0),
	}

	if !caps.CanAcceptTask() {
		match.Reasons = append(match.Reasons, "Agent at maximum task capacity")
		return match
	}

	// Check required capabilities
	for cap, minLevel := range reqs.Required {
		if level, exists := caps.Capabilities[cap]; !exists {
			match.Reasons = append(match.Reasons, fmt.Sprintf("Missing required capability: %s", cap))
			return match
		} else if level < minLevel {
			match.Reasons = append(match.Reasons, fmt.Sprintf("Insufficient level for %s: %.2f < %.2f", cap, level, minLevel))
			return match
		}
	}

	// Calculate base score from required capabilities
	var totalScore float64
	var numFactors int

	for cap, minLevel := range reqs.Required {
		level := caps.Capabilities[cap]
		totalScore += float64(level) / float64(minLevel)
		numFactors++
	}

	// Add bonus for optional capabilities
	for cap, desiredLevel := range reqs.Optional {
		if level, exists := caps.Capabilities[cap]; exists {
			bonus := float64(level) / float64(desiredLevel) * 0.5 // Optional caps worth 50% of required
			totalScore += bonus
			numFactors++
			match.Reasons = append(match.Reasons, fmt.Sprintf("Has optional capability %s at level %.2f", cap, level))
		}
	}

	// Factor in priority alignment
	priorityAlignment := float64(caps.Priority) / float64(reqs.Priority)
	totalScore += priorityAlignment
	numFactors++

	// Calculate final score
	match.Score = totalScore / float64(numFactors)
	match.Reasons = append(match.Reasons, fmt.Sprintf("Priority alignment: %.2f", priorityAlignment))

	return match
}

// FindBestAgent selects the most suitable agent for a task
func FindBestAgent(agents map[string]*AgentCapabilities, reqs *TaskRequirements) (string, error) {
	if len(agents) == 0 {
		return "", fmt.Errorf("no agents available")
	}

	var matches []*AgentMatch
	for agentID, caps := range agents {
		match := CalculateMatch(agentID, caps, reqs)
		if match.Score > 0 {
			matches = append(matches, match)
		}
	}

	if len(matches) == 0 {
		var missingCaps []string
		for cap := range reqs.Required {
			missingCaps = append(missingCaps, string(cap))
		}
		return "", fmt.Errorf("no suitable agents found for capabilities: %s", strings.Join(missingCaps, ", "))
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches[0].AgentID, nil
}
