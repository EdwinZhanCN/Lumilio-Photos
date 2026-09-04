package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRepositoryVerifierRunsOnStartContinuesAfterErrorAndStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan time.Time)
	calls := make(chan int, 3)
	done := make(chan struct{})
	count := 0

	go func() {
		defer close(done)
		runRepositoryVerifierLoop(ctx, ticks, func(context.Context) error {
			count++
			calls <- count
			if count == 1 {
				return errors.New("transient catalog read failure")
			}
			return nil
		}, zap.NewNop())
	}()

	if got := awaitVerifierCall(t, calls); got != 1 {
		t.Fatalf("startup verifier call = %d, want 1", got)
	}
	ticks <- time.Now()
	if got := awaitVerifierCall(t, calls); got != 2 {
		t.Fatalf("post-error verifier call = %d, want 2", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("repository verifier did not stop after context cancellation")
	}
}

func awaitVerifierCall(t *testing.T, calls <-chan int) int {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for repository verifier")
		return 0
	}
}
