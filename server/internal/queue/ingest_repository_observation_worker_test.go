package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
)

func TestObserveRepositoryWorkerSnoozesOneDurableJobForBoundedContinuation(t *testing.T) {
	delay := 250 * time.Millisecond
	worker := &ObserveRepositoryWorker{
		Process: func(context.Context, ObserveRepositoryArgs) (bool, time.Duration, error) {
			return true, delay, nil
		},
	}

	err := worker.Work(context.Background(), &river.Job[ObserveRepositoryArgs]{})
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) {
		t.Fatalf("worker error = %v, want River snooze", err)
	}
	if snooze.Duration != delay {
		t.Fatalf("snooze duration = %s, want %s", snooze.Duration, delay)
	}
}

func TestObserveRepositoryWorkerCompletesTerminalTurn(t *testing.T) {
	worker := &ObserveRepositoryWorker{
		Process: func(context.Context, ObserveRepositoryArgs) (bool, time.Duration, error) {
			return false, 0, nil
		},
	}

	if err := worker.Work(context.Background(), &river.Job[ObserveRepositoryArgs]{}); err != nil {
		t.Fatalf("terminal turn: %v", err)
	}
}

func TestDrainRepositoryOutboxWorkerSnoozesSameJobForFollowerPage(t *testing.T) {
	worker := &DrainRepositoryOutboxWorker{
		Drain: func(context.Context, DrainRepositoryOutboxArgs) (bool, error) {
			return true, nil
		},
	}

	err := worker.Work(context.Background(), &river.Job[DrainRepositoryOutboxArgs]{})
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) {
		t.Fatalf("worker error = %v, want River snooze", err)
	}
	if snooze.Duration <= 0 {
		t.Fatalf("snooze duration = %s, want a positive writer-yield delay", snooze.Duration)
	}
}

func TestDrainRepositoryOutboxWorkerCompletesDrainedBacklog(t *testing.T) {
	worker := &DrainRepositoryOutboxWorker{
		Drain: func(context.Context, DrainRepositoryOutboxArgs) (bool, error) {
			return false, nil
		},
	}

	if err := worker.Work(context.Background(), &river.Job[DrainRepositoryOutboxArgs]{}); err != nil {
		t.Fatalf("drained outbox: %v", err)
	}
}
