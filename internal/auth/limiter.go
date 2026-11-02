package auth

import (
	"sync"
	"time"
)

// Limiter counts failures per key inside a sliding window and says when a
// key has used up its allowance. It lives in memory, so a restart clears it.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
	now    func() time.Time
}

// NewLimiter allows max failures per key within window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, fails: map[string][]time.Time{}, now: time.Now}
}

// Allow reports whether key may try again. When it may not, retryAfter says
// how long until the oldest failure leaves the window.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.prune(key)
	if len(recent) < l.max {
		return true, 0
	}
	return false, recent[0].Add(l.window).Sub(l.now())
}

// Fail records a failed attempt for key.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.fails) > 1024 {
		l.sweep()
	}
	l.fails[key] = append(l.prune(key), l.now())
}

// Reset forgets every failure recorded for key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// prune drops failures older than the window and returns what is left.
func (l *Limiter) prune(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	all := l.fails[key]
	i := 0
	for i < len(all) && !all[i].After(cutoff) {
		i++
	}
	recent := all[i:]
	if len(recent) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = recent
	return recent
}

// sweep removes keys whose failures have all expired. It stops the map from
// growing without bound when many different keys fail once.
func (l *Limiter) sweep() {
	for key := range l.fails {
		l.prune(key)
	}
}
