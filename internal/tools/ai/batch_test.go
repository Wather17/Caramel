package ai_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"caramel/internal/tools/ai"
)

func TestExecuteBatchContextRunsConcurrentlyAndKeepsIndexedErrors(t *testing.T) {
	var inFlight int32
	var maxInFlight int32
	var callbackInFlight int32
	var callbackMu sync.Mutex
	var states []string

	errs, err := ai.ExecuteBatchContext(context.Background(), 3, 0, func(_ context.Context, index int) error {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			previous := atomic.LoadInt32(&maxInFlight)
			if current <= previous || atomic.CompareAndSwapInt32(&maxInFlight, previous, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		if index == 1 {
			return errors.New("falha do item 1")
		}
		return nil
	}, func(event ai.BatchProgressEvent) {
		if atomic.AddInt32(&callbackInFlight, 1) != 1 {
			t.Errorf("callbacks de progresso não deveriam executar em paralelo")
		}
		callbackMu.Lock()
		states = append(states, event.State)
		callbackMu.Unlock()
		time.Sleep(time.Millisecond)
		atomic.AddInt32(&callbackInFlight, -1)
	})
	if err != nil {
		t.Fatalf("ExecuteBatchContext falhou: %v", err)
	}
	if len(errs) != 3 || errs[0] != nil || errs[1] == nil || errs[2] != nil {
		t.Fatalf("erros deveriam permanecer indexados, obtido: %#v", errs)
	}
	if atomic.LoadInt32(&maxInFlight) < 2 {
		t.Fatalf("esperava execução concorrente, máximo observado: %d", maxInFlight)
	}
	if len(states) == 0 || states[len(states)-1] != "done" {
		t.Fatalf("evento final deveria ser done, obtido: %#v", states)
	}
}

func TestExecuteBatchContextCancelsPendingWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})

	results, err := ai.ExecuteBatchContext(ctx, 10, 1, func(workCtx context.Context, index int) error {
		if index == 0 {
			close(started)
			cancel()
		}
		return workCtx.Err()
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava cancelamento do lote, obtido: %v", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("o primeiro item deveria ter iniciado")
	}
	if len(results) != 10 {
		t.Fatalf("esperava resultados indexados para todos os itens, obtido %d", len(results))
	}
}
