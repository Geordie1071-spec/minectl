package auth

import (
	"sync"
	"time"
)

// Limiter counts failures per key in a sliding window.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
	now    func() time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, fails: map[string][]time.Time{}, now: time.Now}
}

func (l *Limiter) prune(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	fs := l.fails[key]
	i := 0
	for i < len(fs) && !fs[i].After(cutoff) {
		i++
	}
	fs = fs[i:]
	if len(fs) == 0 {
		delete(l.fails, key)
	} else {
		l.fails[key] = fs
	}
	return fs
}

// Blocked reports whether key has used up its failures, and when it frees up.
func (l *Limiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fs := l.prune(key)
	if len(fs) < l.max {
		return false, 0
	}
	return true, fs[0].Add(l.window).Sub(l.now())
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key)
	l.fails[key] = append(l.fails[key], l.now())
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
