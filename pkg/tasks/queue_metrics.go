package tasks

import (
	"sync"
	"time"
)

// QueueMetrics tracks performance and statistics of the priority queue
type QueueMetrics struct {
	mu sync.RWMutex

	// Basic metrics
	totalTasks       int64
	completedTasks   int64
	currentQueueSize int
	maxQueueSize     int

	// Timing metrics
	totalWaitTime   time.Duration
	averageWaitTime time.Duration
	peakWaitTime    time.Duration
	lastUpdateTime  time.Time

	// Performance metrics
	throughput float64 // tasks per second
	queueLoad  float64 // 0.0 to 1.0

	// Distribution metrics
	priorityDistribution map[int]int
	waitTimeDistribution map[time.Duration]int
}

// NewQueueMetrics creates a new queue metrics instance
func NewQueueMetrics() *QueueMetrics {
	return &QueueMetrics{
		priorityDistribution: make(map[int]int),
		waitTimeDistribution: make(map[time.Duration]int),
		lastUpdateTime:       time.Now(),
	}
}

// TaskEnqueued records metrics when a task is added to the queue
func (m *QueueMetrics) TaskEnqueued(priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalTasks++
	m.currentQueueSize++
	if m.currentQueueSize > m.maxQueueSize {
		m.maxQueueSize = m.currentQueueSize
	}

	m.priorityDistribution[priority]++
	m.updateQueueLoad()
}

// TaskDequeued records metrics when a task is removed from the queue
func (m *QueueMetrics) TaskDequeued(waitTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.completedTasks++
	m.currentQueueSize--

	// Update wait time metrics
	m.totalWaitTime += waitTime
	if waitTime > m.peakWaitTime {
		m.peakWaitTime = waitTime
	}

	// Round wait time to nearest second for distribution
	roundedWait := waitTime.Round(time.Second)
	m.waitTimeDistribution[roundedWait]++

	// Update average wait time
	if m.completedTasks > 0 {
		m.averageWaitTime = time.Duration(int64(m.totalWaitTime) / m.completedTasks)
	}

	m.updateThroughput()
	m.updateQueueLoad()
}

// GetQueueLoad returns the current queue load (0.0 to 1.0)
func (m *QueueMetrics) GetQueueLoad() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queueLoad
}

// GetAverageWaitTime returns the average wait time for tasks
func (m *QueueMetrics) GetAverageWaitTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.averageWaitTime
}

// GetMetricsSnapshot returns a snapshot of current metrics
func (m *QueueMetrics) GetMetricsSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_tasks":        m.totalTasks,
		"completed_tasks":    m.completedTasks,
		"current_queue_size": m.currentQueueSize,
		"max_queue_size":     m.maxQueueSize,
		"average_wait_time":  m.averageWaitTime.String(),
		"peak_wait_time":     m.peakWaitTime.String(),
		"throughput":         m.throughput,
		"queue_load":         m.queueLoad,
	}
}

// Reset resets all metrics to initial values
func (m *QueueMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalTasks = 0
	m.completedTasks = 0
	m.currentQueueSize = 0
	m.maxQueueSize = 0
	m.totalWaitTime = 0
	m.averageWaitTime = 0
	m.peakWaitTime = 0
	m.throughput = 0
	m.queueLoad = 0
	m.lastUpdateTime = time.Now()
	m.priorityDistribution = make(map[int]int)
	m.waitTimeDistribution = make(map[time.Duration]int)
}

// private methods

func (m *QueueMetrics) updateThroughput() {
	now := time.Now()
	duration := now.Sub(m.lastUpdateTime)
	if duration > 0 {
		m.throughput = float64(m.completedTasks) / duration.Seconds()
	}
}

func (m *QueueMetrics) updateQueueLoad() {
	if m.maxQueueSize > 0 {
		m.queueLoad = float64(m.currentQueueSize) / float64(m.maxQueueSize)
	} else {
		m.queueLoad = 0
	}
}
