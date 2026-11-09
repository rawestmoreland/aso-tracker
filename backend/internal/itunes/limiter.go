package itunes

import (
	"context"
	"sync"
	"time"
)

// Limiter spaces requests at least `interval` apart across all goroutines.
// It can be slowed down at runtime when the API starts refusing requests.
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	max      time.Duration
	next     time.Time
}

// NewLimiter returns a limiter allowing one request per interval.
func NewLimiter(interval time.Duration) *Limiter {
	return &Limiter{interval: interval, max: 60 * time.Second}
}

// Wait blocks until the caller may send a request, or ctx is done.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	wait := l.next.Sub(now)
	l.next = l.next.Add(l.interval)
	l.mu.Unlock()

	if wait <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// SlowDown doubles the interval (up to a cap). Call it on 403/429 responses.
func (l *Limiter) SlowDown() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.interval *= 2
	if l.interval > l.max {
		l.interval = l.max
	}
}

// Interval returns the current spacing between requests.
func (l *Limiter) Interval() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.interval
}
