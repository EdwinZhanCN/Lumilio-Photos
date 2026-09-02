package queue

import "server/internal/db/catalogtx"

// SchedulerWake turns committed catalog mutations into a coalesced,
// process-local scheduling hint. The periodic scheduler pass remains the
// correctness path after crashes, missed hints, or QueueDB replacement.
type SchedulerWake struct {
	signal chan struct{}
}

func NewSchedulerWake() *SchedulerWake {
	return &SchedulerWake{signal: make(chan struct{}, 1)}
}

func (wake *SchedulerWake) ObserveTransaction(sample catalogtx.TransactionSample) {
	if wake == nil || sample.Role != catalogtx.RoleWriter || sample.Outcome != catalogtx.OutcomeCommitted || sample.Operation == catalogtx.OperationCatalogWorkStateRepair {
		return
	}
	wake.Notify()
}

func (wake *SchedulerWake) Notify() {
	if wake == nil {
		return
	}
	select {
	case wake.signal <- struct{}{}:
	default:
	}
}

func (wake *SchedulerWake) Signals() <-chan struct{} {
	if wake == nil {
		return nil
	}
	return wake.signal
}
