package queue

import (
	"testing"

	"server/internal/db/catalogtx"
)

func TestSchedulerWakeOnlySignalsRelevantCommittedWrites(t *testing.T) {
	wake := NewSchedulerWake()
	assertQuiet := func(label string, sample catalogtx.TransactionSample) {
		t.Helper()
		wake.ObserveTransaction(sample)
		select {
		case <-wake.Signals():
			t.Fatalf("%s unexpectedly woke the scheduler", label)
		default:
		}
	}
	assertQuiet("reader", catalogtx.TransactionSample{Role: catalogtx.RoleReader, Outcome: catalogtx.OutcomeCommitted})
	assertQuiet("rollback", catalogtx.TransactionSample{Role: catalogtx.RoleWriter, Outcome: catalogtx.OutcomeRolledBack})
	assertQuiet("scheduler repair", catalogtx.TransactionSample{Role: catalogtx.RoleWriter, Outcome: catalogtx.OutcomeCommitted, Operation: catalogtx.OperationCatalogWorkStateRepair})

	wake.ObserveTransaction(catalogtx.TransactionSample{Role: catalogtx.RoleWriter, Outcome: catalogtx.OutcomeCommitted, Operation: catalogtx.OperationAssetReprocess})
	select {
	case <-wake.Signals():
	default:
		t.Fatal("committed catalog write did not wake the scheduler")
	}
}

func TestSchedulerWakeCoalescesBursts(t *testing.T) {
	wake := NewSchedulerWake()
	for range 10 {
		wake.Notify()
	}
	select {
	case <-wake.Signals():
	default:
		t.Fatal("wake burst produced no signal")
	}
	select {
	case <-wake.Signals():
		t.Fatal("wake burst was not coalesced")
	default:
	}
}
