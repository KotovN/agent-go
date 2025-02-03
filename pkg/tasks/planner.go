package tasks

import (
	"fmt"
	"time"
)

// TaskNode represents a node in the task dependency graph
type TaskNode struct {
	Task          *Task
	Duration      time.Duration
	Resources     map[string]float64
	Dependencies  []*TaskNode
	Dependents    []*TaskNode
	EarliestStart time.Time
	LatestStart   time.Time
	Critical      bool
}

// TaskPlanner handles task scheduling and resource allocation
type TaskPlanner struct {
	nodes          map[string]*TaskNode
	resourceLimits map[string]float64
}

// NewTaskPlanner creates a new task planner
func NewTaskPlanner(resourceLimits map[string]float64) *TaskPlanner {
	return &TaskPlanner{
		nodes:          make(map[string]*TaskNode),
		resourceLimits: resourceLimits,
	}
}

// AddTask adds a task to the planner with its dependencies and resource requirements
func (p *TaskPlanner) AddTask(task *Task, dependencies []string, resources map[string]float64, duration time.Duration) error {
	node := &TaskNode{
		Task:      task,
		Duration:  duration,
		Resources: resources,
	}

	// Check resource limits
	for resource, usage := range resources {
		if limit, exists := p.resourceLimits[resource]; exists && usage > limit {
			return fmt.Errorf("resource %s usage %.2f exceeds limit %.2f", resource, usage, limit)
		}
	}

	// Add dependencies
	for _, depID := range dependencies {
		if dep, exists := p.nodes[depID]; exists {
			node.Dependencies = append(node.Dependencies, dep)
			dep.Dependents = append(dep.Dependents, node)
		} else {
			return fmt.Errorf("dependency not found: %s", depID)
		}
	}

	p.nodes[task.ID] = node
	return nil
}

// OptimizeResources optimizes resource allocation across tasks
func (p *TaskPlanner) OptimizeResources() error {
	// Implement resource optimization logic
	// For example, load balancing, resource leveling, etc.
	return nil
}

// EstimateCosts estimates execution costs for tasks
func (p *TaskPlanner) EstimateCosts() map[string]float64 {
	costs := make(map[string]float64)
	for id, node := range p.nodes {
		// Calculate cost based on resource usage and duration
		cost := 0.0
		for resource, usage := range node.Resources {
			// Apply resource-specific cost factors
			switch resource {
			case "compute":
				cost += usage * 0.1 * node.Duration.Hours()
			case "memory":
				cost += usage * 0.05 * node.Duration.Hours()
			case "network":
				cost += usage * 0.01 * node.Duration.Hours()
			}
		}
		costs[id] = cost
	}
	return costs
}

// GetCriticalPath identifies the critical path in the task graph
func (p *TaskPlanner) GetCriticalPath() []*Task {
	var criticalPath []*Task
	var criticalNodes []*TaskNode

	// Find tasks with no dependencies (entry points)
	var entryNodes []*TaskNode
	for _, node := range p.nodes {
		if len(node.Dependencies) == 0 {
			entryNodes = append(entryNodes, node)
		}
	}

	// Calculate earliest start times (forward pass)
	for _, node := range entryNodes {
		p.calculateEarliestStart(node, time.Time{})
	}

	// Find tasks with no dependents (exit points)
	var exitNodes []*TaskNode
	latestFinish := time.Time{}
	for _, node := range p.nodes {
		if len(node.Dependents) == 0 {
			exitNodes = append(exitNodes, node)
			finish := node.EarliestStart.Add(node.Duration)
			if finish.After(latestFinish) {
				latestFinish = finish
			}
		}
	}

	// Calculate latest start times (backward pass)
	for _, node := range exitNodes {
		p.calculateLatestStart(node, latestFinish)
	}

	// Identify critical path (nodes with zero float)
	for _, node := range p.nodes {
		if node.EarliestStart.Equal(node.LatestStart) {
			node.Critical = true
			criticalNodes = append(criticalNodes, node)
		}
	}

	// Convert critical nodes to tasks
	for _, node := range criticalNodes {
		criticalPath = append(criticalPath, node.Task)
	}

	return criticalPath
}

// DetectCycles checks for dependency cycles in the task graph
func (p *TaskPlanner) DetectCycles() [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var visit func(node *TaskNode, path []string) bool
	visit = func(node *TaskNode, path []string) bool {
		if stack[node.Task.ID] {
			// Found a cycle
			cycle := append(path, node.Task.ID)
			cycles = append(cycles, cycle)
			return true
		}
		if visited[node.Task.ID] {
			return false
		}

		visited[node.Task.ID] = true
		stack[node.Task.ID] = true
		path = append(path, node.Task.ID)

		for _, dep := range node.Dependencies {
			if visit(dep, path) {
				return true
			}
		}

		stack[node.Task.ID] = false
		return false
	}

	for _, node := range p.nodes {
		if !visited[node.Task.ID] {
			visit(node, nil)
		}
	}

	return cycles
}

func (p *TaskPlanner) calculateEarliestStart(node *TaskNode, currentTime time.Time) {
	// Find the latest finish time among dependencies
	for _, dep := range node.Dependencies {
		finish := dep.EarliestStart.Add(dep.Duration)
		if finish.After(currentTime) {
			currentTime = finish
		}
	}

	node.EarliestStart = currentTime

	// Propagate to dependents
	for _, dep := range node.Dependents {
		p.calculateEarliestStart(dep, currentTime.Add(node.Duration))
	}
}

func (p *TaskPlanner) calculateLatestStart(node *TaskNode, deadline time.Time) {
	// Calculate latest start time that still meets the deadline
	node.LatestStart = deadline.Add(-node.Duration)

	// Propagate to dependencies
	for _, dep := range node.Dependencies {
		p.calculateLatestStart(dep, node.LatestStart)
	}
}
