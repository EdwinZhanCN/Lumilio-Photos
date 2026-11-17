package execution

import (
	"context"
	"sync"
	"testing"
	"time"
)

// This qualification lock uses the same Budget-derived River width and the
// production Demand catalog as app startup. remainingJobs models River's
// observable backlog after it hands one job to each macro worker.
func TestBudgetWidthPopulatesGovernorWhileRiverStillHasRemainingJobs(t *testing.T) {
	budget := Budget{
		CPU: 4, DiskIO: 4, ImageCodec: 2, VideoCodec: 1, Inference: 1,
		MemoryBytes: 768 << 20,
		ToolSession: ToolSession{Threads: 1, SoftwarePreset: "veryfast", HardwareAccel: "none"},
	}
	governor, err := budget.Governor()
	if err != nil {
		t.Fatal(err)
	}
	engine, catalog := NewEngine(governor), budget.DemandCatalog()
	macroWorkers := budget.DerivedMacroWorkers()
	if macroWorkers <= 2 {
		t.Fatalf("Budget-derived River width = %d, want > 2", macroWorkers)
	}

	type demandKey struct {
		step  Step
		media MediaType
	}
	work := []demandKey{
		{StepDerivativesComputeThumb, MediaPhoto},
		{StepDerivativesComputeThumb, MediaPhoto},
		{StepTranscodeCompute, MediaVideo},
	}
	for len(work) < macroWorkers+3 {
		work = append(work, demandKey{StepDerivativesComputeThumb, MediaPhoto})
	}
	remainingJobs := len(work) - macroWorkers

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	release := make(chan struct{})
	var group sync.WaitGroup
	start := func(items []demandKey) {
		for _, item := range items {
			item := item
			group.Add(1)
			go func() {
				defer group.Done()
				_ = engine.Run(ctx, ClassBackground, catalog.Demand(item.step, item.media), func(context.Context) error {
					<-release
					return nil
				})
			}()
		}
	}
	start(work[:3])
	awaitMixedPressure(t, ctx, governor, func(snapshot Snapshot) bool {
		return snapshot.InUse.ImageCodec == budget.ImageCodec && snapshot.InUse.VideoCodec == budget.VideoCodec
	})
	start(work[3:macroWorkers])

	awaitMixedPressure(t, ctx, governor, func(snapshot Snapshot) bool {
		return remainingJobs > 0 && snapshot.Waiting > 0 &&
			snapshot.InUse.ImageCodec == budget.ImageCodec &&
			snapshot.InUse.VideoCodec == budget.VideoCodec
	})
	close(release)
	group.Wait()
	if snapshot := governor.Snapshot(); snapshot.InUse != (Resources{}) || snapshot.Waiting != 0 {
		t.Fatalf("mixed workload did not drain: %+v", snapshot)
	}
}

func awaitMixedPressure(t *testing.T, ctx context.Context, governor *Governor, ready func(Snapshot) bool) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if ready(governor.Snapshot()) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("mixed workload never reached production pressure: %+v", governor.Snapshot())
		case <-ticker.C:
		}
	}
}
