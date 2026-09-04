package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRepositoryAccessCoordinatorDoesNotStarveWaitingMutation(t *testing.T) {
	coordinator := NewRepositoryAccessCoordinator()
	repositoryID := uuid.New()
	releaseInitialReader := coordinator.acquireRead(repositoryID)

	type acquisition struct {
		release func()
		err     error
	}
	writerAcquired := make(chan acquisition, 1)
	writerCtx, cancelWriter := context.WithTimeout(context.Background(), time.Second)
	defer cancelWriter()
	go func() {
		release, err := coordinator.AcquireMutationsContext(writerCtx, []uuid.UUID{repositoryID})
		writerAcquired <- acquisition{release: release, err: err}
	}()
	waitForRepositoryAccessWaiter(t, coordinator.lockFor(repositoryID), repositoryAccessWrite)
	readerAcquired := make(chan acquisition, 1)
	readerCtx, cancelReader := context.WithTimeout(context.Background(), time.Second)
	defer cancelReader()
	go func() {
		release, err := coordinator.acquireReadContext(readerCtx, repositoryID)
		readerAcquired <- acquisition{release: release, err: err}
	}()

	select {
	case result := <-readerAcquired:
		if result.release != nil {
			result.release()
		}
		releaseInitialReader()
		writer := <-writerAcquired
		if writer.release != nil {
			writer.release()
		}
		t.Fatalf("later reader bypassed a waiting mutation: %v", result.err)
	case <-time.After(30 * time.Millisecond):
	}

	releaseInitialReader()
	var writer acquisition
	select {
	case writer = <-writerAcquired:
		if writer.err != nil {
			t.Fatalf("mutation acquisition: %v", writer.err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting mutation did not acquire after the initial reader released")
	}

	select {
	case result := <-readerAcquired:
		if result.release != nil {
			result.release()
		}
		writer.release()
		t.Fatalf("later reader acquired while the mutation was active: %v", result.err)
	case <-time.After(30 * time.Millisecond):
	}

	writer.release()
	select {
	case result := <-readerAcquired:
		if result.err != nil {
			t.Fatalf("reader acquisition after mutation: %v", result.err)
		}
		result.release()
	case <-time.After(time.Second):
		t.Fatal("later reader did not acquire after the mutation released")
	}
}

func waitForRepositoryAccessWaiter(t *testing.T, lock *repositoryAccessLock, mode repositoryAccessMode) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		lock.mu.Lock()
		found := false
		for _, waiter := range lock.waiters {
			if waiter.mode == mode {
				found = true
				break
			}
		}
		lock.mu.Unlock()
		if found {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("repository access waiter did not enter the queue")
}

func TestRepositoryAccessCoordinatorCancelledMutationStopsBlockingReaders(t *testing.T) {
	coordinator := NewRepositoryAccessCoordinator()
	repositoryID := uuid.New()
	releaseInitialReader := coordinator.acquireRead(repositoryID)
	defer releaseInitialReader()

	mutationCtx, cancelMutation := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelMutation()
	if release, err := coordinator.AcquireMutationsContext(mutationCtx, []uuid.UUID{repositoryID}); err == nil {
		release()
		t.Fatal("mutation unexpectedly acquired while a reader was active")
	}

	readerCtx, cancelReader := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelReader()
	releaseReader, err := coordinator.acquireReadContext(readerCtx, repositoryID)
	if err != nil {
		t.Fatalf("reader stayed blocked after the waiting mutation was cancelled: %v", err)
	}
	releaseReader()
}

func TestRepositoryAccessCoordinatorCancelledReaderDoesNotLeakLease(t *testing.T) {
	coordinator := NewRepositoryAccessCoordinator()
	repositoryID := uuid.New()
	releaseMutation := coordinator.AcquireMutation(repositoryID)

	readerCtx, cancelReader := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelReader()
	if release, err := coordinator.acquireReadContext(readerCtx, repositoryID); err == nil {
		release()
		releaseMutation()
		t.Fatal("reader unexpectedly acquired while a mutation was active")
	}
	releaseMutation()

	nextCtx, cancelNext := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelNext()
	releaseNext, err := coordinator.AcquireMutationsContext(nextCtx, []uuid.UUID{repositoryID})
	if err != nil {
		t.Fatalf("cancelled reader leaked a lease: %v", err)
	}
	releaseNext()
}
