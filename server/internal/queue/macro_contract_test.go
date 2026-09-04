package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"server/internal/queue/jobs"
	"server/internal/workqos"
)

func TestProjectionMacroCannotCompleteWithoutNoopOrCommitAcknowledgement(t *testing.T) {
	args := jobs.RebuildProjectionBatchArgs{
		ProjectionKind: "event", Scope: "1", SourceRevision: 1, ProjectionVersion: 1,
	}
	priority, err := workqos.Background.Priority()
	if err != nil {
		t.Fatal(err)
	}
	job := &river.Job[jobs.RebuildProjectionBatchArgs]{JobRow: &rivertype.JobRow{Priority: priority}, Args: args}
	var observedQoS workqos.Class
	worker := &RebuildProjectionBatchWorker{Execute: func(_ context.Context, qos workqos.Class, _ jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error) {
		observedQoS = qos
		return ProjectionExecution{}, nil
	}}
	if err := worker.Work(context.Background(), job); !errors.Is(err, ErrMacroStageUnavailable) {
		t.Fatalf("unacknowledged projection error = %v", err)
	}
	if observedQoS != workqos.Background {
		t.Fatalf("worker QoS = %s, want background", observedQoS)
	}
	worker.Execute = func(context.Context, workqos.Class, jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error) {
		return ProjectionExecution{Acknowledged: true}, nil
	}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("acknowledged projection: %v", err)
	}
	worker.Execute = func(context.Context, workqos.Class, jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error) {
		return ProjectionExecution{Noop: true}, nil
	}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("stale projection no-op: %v", err)
	}
}

// Macro work has materially different runtime shapes. Leaving any of these
// on River's one-minute client default turns normal media work into a retry
// loop. River's retry/discard state is delivery-only; Catalog remains the
// source of whether the work is still runnable.
func TestMacroWorkersDeclareExplicitTimeouts(t *testing.T) {
	tests := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"ingest", (&IngestMacroWorker{}).Timeout(nil), 30 * time.Minute},
		{"analyze", (&AnalyzeAssetWorker{}).Timeout(nil), 10 * time.Minute},
		{"derivatives", (&GenerateAssetDerivativesWorker{}).Timeout(nil), 30 * time.Minute},
		{"transcode", (&TranscodeMediaWorker{}).Timeout(nil), 2 * time.Hour},
		{"enrich", (&EnrichAssetWorker{}).Timeout(nil), 2 * time.Hour},
		{"scan", (&ScanRepositoryBatchWorker{}).Timeout(nil), 15 * time.Minute},
		{"projection", (&RebuildProjectionBatchWorker{}).Timeout(nil), 30 * time.Minute},
		{"backup", (&BackupCatalogWorker{}).Timeout(nil), 30 * time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("timeout = %s, want %s", test.got, test.want)
			}
		})
	}
}

func TestMacroJobAttemptsAreFiniteAndDiscardedJobsCanBeRecreated(t *testing.T) {
	for _, job := range jobs.RuntimeJobCatalog() {
		opts := job.InsertOpts()
		if opts.MaxAttempts < 1 {
			t.Fatalf("%s MaxAttempts = %d", job.Kind(), opts.MaxAttempts)
		}
		for _, state := range opts.UniqueOpts.ByState {
			if state == "discarded" {
				t.Fatalf("%s retains uniqueness after discard", job.Kind())
			}
		}
	}
}
