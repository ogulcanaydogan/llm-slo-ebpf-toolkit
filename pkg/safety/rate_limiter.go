package safety

import (
	"sync"
	"time"
)

// RateLimiter enforces a max events-per-second budget.
type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	windowSec int64
	count     int
}

// NewRateLimiter creates a limiter with a per-second cap.
func NewRateLimiter(limit int) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	return &RateLimiter{limit: limit}
}

// Allow returns true if one more event can be emitted in the current second.
func (l *RateLimiter) Allow(now time.Time) bool {
	sec := now.UTC().Unix()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.windowSec != sec {
		l.windowSec = sec
		l.count = 0
	}
	if l.count >= l.limit {
		return false
	}
	l.count++
	return true
}
