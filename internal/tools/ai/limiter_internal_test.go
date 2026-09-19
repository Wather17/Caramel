package ai

import (
	"context"
	"testing"
)

func TestRequestLimiterRespeitaCancelamentoEnquantoAguarda(t *testing.T) {
	limiter := newRequestLimiter(1)
	if err := limiter.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer limiter.release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.acquire(ctx); err != context.Canceled {
		t.Fatalf("esperava context.Canceled, obtido %v", err)
	}
}
