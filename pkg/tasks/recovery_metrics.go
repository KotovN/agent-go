package tasks

import (
	"sync"
	"time"
)

// RecoveryMetrics tracks statistics about error recovery attempts
type RecoveryMetrics struct {
	mu sync.RWMutex

	// Total attempts per strategy
	AttemptsByStrategy map[RecoveryStrategy]int
	// Successful recoveries per strategy
	SuccessesByStrategy map[RecoveryStrategy]int
	// Failed recoveries per strategy
	FailuresByStrategy map[RecoveryStrategy]int

	// Attempts by error category
	AttemptsByCategory map[ErrorCategory]int
	// Successes by error category
	SuccessesByCategory map[ErrorCategory]int
	// Failures by error category
	FailuresByCategory map[ErrorCategory]int

	// Average recovery time per strategy
	AvgRecoveryTime map[RecoveryStrategy]time.Duration
	// Total recovery time per strategy
	TotalRecoveryTime map[RecoveryStrategy]time.Duration
	// Number of timing samples per strategy
	TimingSamples map[RecoveryStrategy]int

	// Historical recovery attempts
	History []*RecoveryAttempt
}

// RecoveryAttempt represents a single recovery attempt
type RecoveryAttempt struct {
	TaskID           string
	Strategy         RecoveryStrategy
	ErrorCategory    ErrorCategory
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	Successful       bool
	ErrorDetails     string
	AttemptNumber    int
	BackoffDuration  time.Duration
	MetadataSnapshot map[string]interface{}
}

// NewRecoveryMetrics creates a new recovery metrics tracker
func NewRecoveryMetrics() *RecoveryMetrics {
	return &RecoveryMetrics{
		AttemptsByStrategy:  make(map[RecoveryStrategy]int),
		SuccessesByStrategy: make(map[RecoveryStrategy]int),
		FailuresByStrategy:  make(map[RecoveryStrategy]int),
		AttemptsByCategory:  make(map[ErrorCategory]int),
		SuccessesByCategory: make(map[ErrorCategory]int),
		FailuresByCategory:  make(map[ErrorCategory]int),
		AvgRecoveryTime:     make(map[RecoveryStrategy]time.Duration),
		TotalRecoveryTime:   make(map[RecoveryStrategy]time.Duration),
		TimingSamples:       make(map[RecoveryStrategy]int),
		History:             make([]*RecoveryAttempt, 0),
	}
}

// RecordAttempt records a recovery attempt
func (rm *RecoveryMetrics) RecordAttempt(attempt *RecoveryAttempt) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Update strategy metrics
	rm.AttemptsByStrategy[attempt.Strategy]++
	if attempt.Successful {
		rm.SuccessesByStrategy[attempt.Strategy]++
	} else {
		rm.FailuresByStrategy[attempt.Strategy]++
	}

	// Update category metrics
	rm.AttemptsByCategory[attempt.ErrorCategory]++
	if attempt.Successful {
		rm.SuccessesByCategory[attempt.ErrorCategory]++
	} else {
		rm.FailuresByCategory[attempt.ErrorCategory]++
	}

	// Update timing metrics
	rm.TotalRecoveryTime[attempt.Strategy] += attempt.Duration
	rm.TimingSamples[attempt.Strategy]++
	rm.AvgRecoveryTime[attempt.Strategy] = rm.TotalRecoveryTime[attempt.Strategy] /
		time.Duration(rm.TimingSamples[attempt.Strategy])

	// Add to history
	rm.History = append(rm.History, attempt)
}

// GetSuccessRate returns the success rate for a strategy
func (rm *RecoveryMetrics) GetSuccessRate(strategy RecoveryStrategy) float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	attempts := rm.AttemptsByStrategy[strategy]
	if attempts == 0 {
		return 0
	}
	return float64(rm.SuccessesByStrategy[strategy]) / float64(attempts)
}

// GetCategorySuccessRate returns the success rate for an error category
func (rm *RecoveryMetrics) GetCategorySuccessRate(category ErrorCategory) float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	attempts := rm.AttemptsByCategory[category]
	if attempts == 0 {
		return 0
	}
	return float64(rm.SuccessesByCategory[category]) / float64(attempts)
}

// GetAverageRecoveryTime returns the average recovery time for a strategy
func (rm *RecoveryMetrics) GetAverageRecoveryTime(strategy RecoveryStrategy) time.Duration {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.AvgRecoveryTime[strategy]
}

// GetMetricsSummary returns a summary of all metrics
func (rm *RecoveryMetrics) GetMetricsSummary() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"attempts_by_strategy":  rm.AttemptsByStrategy,
		"successes_by_strategy": rm.SuccessesByStrategy,
		"failures_by_strategy":  rm.FailuresByStrategy,
		"attempts_by_category":  rm.AttemptsByCategory,
		"successes_by_category": rm.SuccessesByCategory,
		"failures_by_category":  rm.FailuresByCategory,
		"avg_recovery_time":     rm.AvgRecoveryTime,
		"total_recovery_time":   rm.TotalRecoveryTime,
		"total_attempts":        len(rm.History),
		"overall_success_rate":  rm.getOverallSuccessRate(),
		"recent_attempts":       rm.getRecentAttempts(10),
	}
}

// getOverallSuccessRate calculates the overall success rate
func (rm *RecoveryMetrics) getOverallSuccessRate() float64 {
	var totalSuccesses, totalAttempts int
	for _, successes := range rm.SuccessesByStrategy {
		totalSuccesses += successes
	}
	for _, attempts := range rm.AttemptsByStrategy {
		totalAttempts += attempts
	}
	if totalAttempts == 0 {
		return 0
	}
	return float64(totalSuccesses) / float64(totalAttempts)
}

// getRecentAttempts returns the n most recent recovery attempts
func (rm *RecoveryMetrics) getRecentAttempts(n int) []*RecoveryAttempt {
	if len(rm.History) <= n {
		return rm.History
	}
	return rm.History[len(rm.History)-n:]
}

// PruneHistory removes old history entries beyond a certain age
func (rm *RecoveryMetrics) PruneHistory(maxAge time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	newHistory := make([]*RecoveryAttempt, 0)

	for _, attempt := range rm.History {
		if attempt.EndTime.After(cutoff) {
			newHistory = append(newHistory, attempt)
		}
	}

	rm.History = newHistory
}
