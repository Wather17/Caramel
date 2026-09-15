package ai

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BatchWork executes one indexed item from a batch.
type BatchWork func(context.Context, int) error

// BatchProgressEvent reports a serialized batch state transition.
type BatchProgressEvent struct {
	Index     int
	Total     int
	Completed int
	Succeeded int
	Failed    int
	State     string // started, completed, failed, done
	Err       error
}

// BatchProgressFunc receives batch progress without concurrent callback calls.
type BatchProgressFunc func(BatchProgressEvent)

// ExecuteBatchContext runs indexed work with the same adaptive worker policy
// used by image generation. Per-item errors are returned in input order; the
// returned error is reserved for cancellation or a structural batch failure.
func ExecuteBatchContext(ctx context.Context, total int, maxWorkers int, work BatchWork, onProgress BatchProgressFunc) ([]error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if total <= 0 {
		return []error{}, nil
	}
	if work == nil {
		return nil, fmt.Errorf("batch work is nil")
	}

	workers, dispatchDelay := CalculateConcurrencyDecision(total, maxWorkers)
	if workers > total {
		workers = total
	}

	errorsByIndex := make([]error, total)
	jobs := make(chan int, total)
	done := make(chan struct{})

	var stateMu sync.Mutex
	var progressMu sync.Mutex
	completed, succeeded, failed := 0, 0, 0

	emit := func(event BatchProgressEvent) {
		if onProgress == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		onProgress(event)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				var index int
				select {
				case <-ctx.Done():
					return
				case next, ok := <-jobs:
					if !ok {
						return
					}
					index = next
				}

				if err := ctx.Err(); err != nil {
					errorsByIndex[index] = err
					return
				}

				emit(BatchProgressEvent{Index: index, Total: total, State: "started"})
				err := work(ctx, index)
				errorsByIndex[index] = err

				stateMu.Lock()
				completed++
				if err != nil {
					failed++
				} else {
					succeeded++
				}
				event := BatchProgressEvent{
					Index:     index,
					Total:     total,
					Completed: completed,
					Succeeded: succeeded,
					Failed:    failed,
					State:     "completed",
					Err:       err,
				}
				if err != nil {
					event.State = "failed"
				}
				stateMu.Unlock()
				emit(event)
			}
		}()
	}

	go func() {
		defer close(done)
		for index := 0; index < total; index++ {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- index:
			}
			if dispatchDelay > 0 && index < total-1 {
				timer := time.NewTimer(dispatchDelay)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					close(jobs)
					return
				case <-timer.C:
				}
			}
		}
		close(jobs)
	}()

	<-done
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return errorsByIndex, err
	}
	stateMu.Lock()
	finalEvent := BatchProgressEvent{Total: total, Completed: completed, Succeeded: succeeded, Failed: failed, State: "done"}
	stateMu.Unlock()
	emit(finalEvent)
	return errorsByIndex, nil
}
