package ratelimit

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLimitExceeded = errors.New("render concurrency limit reached")
	ErrAcquireTimeout = errors.New("render acquire semaphore timeout")
)

// Limiter controls maximum parallel chart rendering tasks.
type Limiter struct {
	sem chan struct{}
}

// NewLimiter creates a concurrency limiter with given capacity.
func NewLimiter(maxConcurrent int) *Limiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &Limiter{
		sem: make(chan struct{}, maxConcurrent),
	}
}

// Acquire waits for a slot or context cancellation.
func (l *Limiter) Acquire(ctx context.Context, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrAcquireTimeout
		}
		return ctx.Err()
	}
}

// TryAcquire attempts to immediately acquire a slot without blocking.
func (l *Limiter) TryAcquire() bool {
	select {
	case l.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release frees an occupied concurrency slot.
func (l *Limiter) Release() {
	select {
	case <-l.sem:
	default:
	}
}
