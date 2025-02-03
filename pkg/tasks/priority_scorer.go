package tasks

import (
	"math"
	"time"
)

// TaskContext provides context for priority calculation
type TaskContext struct {
	WaitingTime  time.Duration
	Dependencies []*Task
	QueueMetrics *QueueMetrics
	CreatedAt    time.Time
}

// PriorityScorer defines how task priority is calculated
type PriorityScorer interface {
	CalculatePriority(task *Task, context TaskContext) float64
}

// WeightedPriorityScorer implements weighted priority calculation
type WeightedPriorityScorer struct {
	PriorityWeight   float64
	DeadlineWeight   float64
	WaitTimeWeight   float64
	DependencyWeight float64
}

// NewWeightedPriorityScorer creates a new weighted priority scorer with default weights
func NewWeightedPriorityScorer() *WeightedPriorityScorer {
	return &WeightedPriorityScorer{
		PriorityWeight:   1.0,
		DeadlineWeight:   1.0,
		WaitTimeWeight:   0.5,
		DependencyWeight: 0.8,
	}
}

// CalculatePriority implements PriorityScorer interface
func (w *WeightedPriorityScorer) CalculatePriority(task *Task, context TaskContext) float64 {
	var score float64

	// Base priority score
	score += w.PriorityWeight * float64(task.Priority)

	// Deadline score
	if !task.Deadline.IsZero() {
		timeUntilDeadline := time.Until(task.Deadline)
		// Convert to hours and normalize
		deadlineScore := math.Max(0, 24-timeUntilDeadline.Hours()) / 24
		score += w.DeadlineWeight * deadlineScore
	}

	// Waiting time score
	if context.WaitingTime > 0 {
		// Normalize waiting time (max 1 hour)
		waitScore := math.Min(1.0, context.WaitingTime.Hours())
		score += w.WaitTimeWeight * waitScore
	}

	// Dependency score
	if len(context.Dependencies) > 0 {
		depScore := 0.0
		completedDeps := 0
		for _, dep := range context.Dependencies {
			if dep.Status == TaskStatusComplete {
				completedDeps++
			}
		}
		if len(context.Dependencies) > 0 {
			depScore = float64(completedDeps) / float64(len(context.Dependencies))
		}
		score += w.DependencyWeight * depScore
	}

	return score
}

// DynamicPriorityScorer adjusts priority based on queue metrics
type DynamicPriorityScorer struct {
	baseScorer PriorityScorer
}

// NewDynamicPriorityScorer creates a new dynamic priority scorer
func NewDynamicPriorityScorer(baseScorer PriorityScorer) *DynamicPriorityScorer {
	return &DynamicPriorityScorer{
		baseScorer: baseScorer,
	}
}

// CalculatePriority implements PriorityScorer interface with dynamic adjustments
func (d *DynamicPriorityScorer) CalculatePriority(task *Task, context TaskContext) float64 {
	baseScore := d.baseScorer.CalculatePriority(task, context)

	if context.QueueMetrics == nil {
		return baseScore
	}

	// Adjust based on queue metrics
	var adjustment float64

	// Adjust for queue load
	queueLoad := context.QueueMetrics.GetQueueLoad()
	if queueLoad > 0.8 { // High load
		adjustment += 0.2
	}

	// Adjust for waiting time relative to average
	if context.WaitingTime > context.QueueMetrics.GetAverageWaitTime()*2 {
		adjustment += 0.3
	}

	return baseScore * (1 + adjustment)
}
