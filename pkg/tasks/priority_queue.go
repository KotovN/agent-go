package tasks

import (
	"container/heap"
	"sync"
	"time"
)

// PriorityQueue implements a thread-safe priority queue for tasks
type PriorityQueue struct {
	mu          sync.RWMutex
	items       []*QueueItem
	metrics     *QueueMetrics
	scorer      PriorityScorer
	maxCapacity int
	taskIndex   map[string]*QueueItem // For O(1) lookups by task ID
}

// QueueItem represents an item in the priority queue
type QueueItem struct {
	Task       *Task
	Priority   float64
	Deadline   time.Time
	Index      int // Used by heap.Interface
	InsertedAt time.Time
}

// NewPriorityQueue creates a new priority queue instance
func NewPriorityQueue(maxCapacity int) *PriorityQueue {
	if maxCapacity <= 0 {
		maxCapacity = 1000 // Default capacity
	}

	return &PriorityQueue{
		items:       make([]*QueueItem, 0),
		metrics:     NewQueueMetrics(),
		scorer:      NewWeightedPriorityScorer(),
		maxCapacity: maxCapacity,
		taskIndex:   make(map[string]*QueueItem),
	}
}

// SetScorer sets a custom priority scorer
func (pq *PriorityQueue) SetScorer(scorer PriorityScorer) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.scorer = scorer
}

// Len returns the number of items in the queue
func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

// Less compares two items for priority ordering
func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.items[i].Priority > pq.items[j].Priority
}

// Swap swaps two items in the queue
func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].Index = i
	pq.items[j].Index = j
}

// Push adds an item to the queue
func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*QueueItem)
	item.Index = len(pq.items)
	item.InsertedAt = time.Now()
	pq.items = append(pq.items, item)
	pq.taskIndex[item.Task.ID] = item
}

// Pop removes and returns the highest priority item
func (pq *PriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	pq.items = old[0 : n-1]
	delete(pq.taskIndex, item.Task.ID)
	return item
}

// Enqueue adds a task to the queue
func (pq *PriorityQueue) Enqueue(task *Task) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) >= pq.maxCapacity {
		return ErrQueueFull
	}

	// Calculate priority score
	context := TaskContext{
		WaitingTime:  0,
		Dependencies: pq.getDependencyTasks(task.Dependencies),
		QueueMetrics: pq.metrics,
		CreatedAt:    task.CreatedAt,
	}

	priority := pq.scorer.CalculatePriority(task, context)

	item := &QueueItem{
		Task:     task,
		Priority: priority,
		Deadline: task.Deadline,
	}

	heap.Push(pq, item)
	pq.metrics.TaskEnqueued(int(task.Priority))
	return nil
}

// EnqueueBatch adds multiple tasks to the queue
func (pq *PriorityQueue) EnqueueBatch(tasks []*Task) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items)+len(tasks) > pq.maxCapacity {
		return ErrQueueFull
	}

	for _, task := range tasks {
		context := TaskContext{
			WaitingTime:  0,
			Dependencies: pq.getDependencyTasks(task.Dependencies),
			QueueMetrics: pq.metrics,
			CreatedAt:    task.CreatedAt,
		}

		priority := pq.scorer.CalculatePriority(task, context)

		item := &QueueItem{
			Task:     task,
			Priority: priority,
			Deadline: task.Deadline,
		}

		heap.Push(pq, item)
		pq.metrics.TaskEnqueued(int(task.Priority))
	}

	return nil
}

// Dequeue removes and returns the highest priority task
func (pq *PriorityQueue) Dequeue() (*Task, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return nil, false
	}

	item := heap.Pop(pq).(*QueueItem)
	waitTime := time.Since(item.InsertedAt)
	pq.metrics.TaskDequeued(waitTime)

	return item.Task, true
}

// DequeueBatch removes and returns multiple high priority tasks
func (pq *PriorityQueue) DequeueBatch(count int) []*Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if count <= 0 {
		return nil
	}

	// Limit count to available items
	if count > len(pq.items) {
		count = len(pq.items)
	}

	tasks := make([]*Task, count)
	for i := 0; i < count; i++ {
		item := heap.Pop(pq).(*QueueItem)
		waitTime := time.Since(item.InsertedAt)
		pq.metrics.TaskDequeued(waitTime)
		tasks[i] = item.Task
	}

	return tasks
}

// Peek returns the highest priority task without removing it
func (pq *PriorityQueue) Peek() (*Task, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if len(pq.items) == 0 {
		return nil, false
	}

	return pq.items[0].Task, true
}

// UpdatePriority updates the priority of a task
func (pq *PriorityQueue) UpdatePriority(taskID string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	item, exists := pq.taskIndex[taskID]
	if !exists {
		return false
	}

	context := TaskContext{
		WaitingTime:  time.Since(item.InsertedAt),
		Dependencies: pq.getDependencyTasks(item.Task.Dependencies),
		QueueMetrics: pq.metrics,
		CreatedAt:    item.Task.CreatedAt,
	}

	item.Priority = pq.scorer.CalculatePriority(item.Task, context)
	heap.Fix(pq, item.Index)
	return true
}

// Remove removes a specific task from the queue
func (pq *PriorityQueue) Remove(taskID string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	item, exists := pq.taskIndex[taskID]
	if !exists {
		return false
	}

	heap.Remove(pq, item.Index)
	return true
}

// GetMetrics returns the queue metrics
func (pq *PriorityQueue) GetMetrics() map[string]interface{} {
	return pq.metrics.GetMetricsSnapshot()
}

// Clear removes all items from the queue
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.items = make([]*QueueItem, 0)
	pq.taskIndex = make(map[string]*QueueItem)
	pq.metrics.Reset()
	heap.Init(pq)
}

// getDependencyTasks retrieves task objects for the given dependency IDs
func (pq *PriorityQueue) getDependencyTasks(depIDs []string) []*Task {
	deps := make([]*Task, 0, len(depIDs))
	for _, id := range depIDs {
		if item, exists := pq.taskIndex[id]; exists {
			deps = append(deps, item.Task)
		}
	}
	return deps
}
