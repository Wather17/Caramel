package ai

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestExecuteModelChainAvancaDepoisDe429Esgotado(t *testing.T) {
	oldBackoff := retryBackoffBase
	retryBackoffBase = -time.Second
	t.Cleanup(func() { retryBackoffBase = oldBackoff })

	calls := make([]string, 0, 4)
	effective, err := ExecuteModelChainContext(context.Background(), []string{"primary", "fallback"}, 3, func(model string) error {
		calls = append(calls, model)
		if model == "primary" {
			return statusError(http.StatusTooManyRequests, []byte("rate limit"))
		}
		return nil
	}, nil)
	if err != nil || effective != "fallback" {
		t.Fatalf("429 transitório deveria usar fallback: model=%s err=%v", effective, err)
	}
	if len(calls) != 4 || calls[0] != "primary" || calls[1] != "primary" || calls[2] != "primary" || calls[3] != "fallback" {
		t.Fatalf("tentativas inesperadas: %v", calls)
	}
}
