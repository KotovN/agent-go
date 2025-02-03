package token

import (
	"sync"
	"time"
)

// RateLimiter manages request rate limiting for API calls
type RateLimiter struct {
	maxRPM       int
	currentRPM   int
	mu           sync.Mutex
	resetTimer   *time.Timer
	shutdownChan chan struct{}
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(maxRPM int) *RateLimiter {
	if maxRPM <= 0 {
		maxRPM = 60 // default to 60 RPM
	}

	rl := &RateLimiter{
		maxRPM:       maxRPM,
		shutdownChan: make(chan struct{}),
	}

	rl.startResetTimer()
	return rl
}

// CheckOrWait checks if a request can be made, waiting if necessary
func (rl *RateLimiter) CheckOrWait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.currentRPM >= rl.maxRPM {
		// Wait for the next minute to start
		rl.mu.Unlock()
		time.Sleep(time.Second * 60)
		rl.mu.Lock()
		rl.currentRPM = 0
	}

	rl.currentRPM++
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.shutdownChan)
	if rl.resetTimer != nil {
		rl.resetTimer.Stop()
	}
}

func (rl *RateLimiter) startResetTimer() {
	rl.resetTimer = time.NewTimer(time.Minute)
	go func() {
		for {
			select {
			case <-rl.resetTimer.C:
				rl.mu.Lock()
				rl.currentRPM = 0
				rl.resetTimer.Reset(time.Minute)
				rl.mu.Unlock()
			case <-rl.shutdownChan:
				return
			}
		}
	}()
}
