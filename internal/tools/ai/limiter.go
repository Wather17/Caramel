package ai

import (
	"context"
	"errors"
)

// DefaultMaxConcurrentRequests é o teto compartilhado por cliente para que
// --workers alto não bypass a política conservadora do provedor.
const DefaultMaxConcurrentRequests = 4

var errRequestLimitUnavailable = errors.New("limitador de requests indisponível")

type requestLimiter struct {
	slots chan struct{}
}

func newRequestLimiter(maxConcurrent int) *requestLimiter {
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrentRequests
	}
	return &requestLimiter{slots: make(chan struct{}, maxConcurrent)}
}

func (l *requestLimiter) acquire(ctx context.Context) error {
	if l == nil || l.slots == nil {
		return errRequestLimitUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *requestLimiter) release() {
	if l == nil || l.slots == nil {
		return
	}
	select {
	case <-l.slots:
	default:
	}
}
