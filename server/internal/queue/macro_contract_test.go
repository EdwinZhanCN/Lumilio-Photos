package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"server/internal/commit"
	"server/internal/queue/jobs"
)

func TestProjectionMacroCannotCompleteWithoutNoopOrCommitAcknowledgement(t *testing.T) {
	args := jobs.RebuildProjectionBatchArgs{
		ProjectionKind: "event", Scope: "1", SourceRevision: 1, ProjectionVersion: 1,
	}
	job := &river.Job[jobs.RebuildProjectionBatchArgs]{Args: args}
	worker := &RebuildProjectionBatchWorker{Execute: func(context.Context, jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error) {
		return ProjectionExecution{}, nil
	}}
	if err := worker.Work(context.Background(), job); !errors.Is(err, ErrMacroStageUnavailable) {
		t.Fatalf("unacknowledged projection error = %v", err)
	}
	worker.Execute = func(context.Context, jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error) {
		return ProjectionExecution{Acknowledged: true}, nil
	}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("acknowledged projection: %v", err)
	}
	worker.Execute = func(context.Context, jobs.RebuildProjectionBatchArgs) (ProjectionExecution, error) {
		return ProjectionExecution{Noop: true}, nil
	}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("stale projection no-op: %v", err)
	}
}

// Macro work has materially different runtime envelopes. Leaving any of these
// on River's one-minute client default turns normal media work into a retry
// loop, and eventually into a discarded job with no product terminal state.
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

func TestFinalMacroAttemptMapsToTypedTerminalCommit(t *testing.T) {
	assetID, fence, receiptID := uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name    string
		kind    string
		args    any
		family  string
		stage   string
		subject string
	}{
		{"asset", "generate_asset_derivatives", jobs.GenerateAssetDerivativesArgs{AssetID: assetID, SourceFence: fence, DesiredVersion: 3, PipelineVersion: "asset-v1"}, commit.FamilyAssetStage, "derivatives", assetID.String()},
		{"ingest", "ingest_asset", jobs.IngestAssetArgs{CommitID: fence, ReceiptID: receiptID}, commit.FamilyIngestReceipt, "ingest", receiptID.String()},
		{"backup", "backup_catalog", jobs.BackupCatalogArgs{RequestID: receiptID}, commit.FamilyOperationReceipt, "backup", receiptID.String()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.args)
			if err != nil {
				t.Fatal(err)
			}
			intent, ok := macroTerminalIntent(&rivertype.JobRow{Kind: test.kind, Attempt: 8, MaxAttempts: 8, EncodedArgs: encoded}, "attempts_exhausted")
			if !ok {
				t.Fatal("final macro attempt did not produce a terminal catalog intent")
			}
			if intent.Key.Family != test.family || intent.Key.Subject != test.subject || intent.Key.Stage != test.stage {
				t.Fatalf("terminal intent key = %+v", intent.Key)
			}
		})
	}
}

func TestMacroJobAttemptsAreFiniteAndDiscardedJobsRemainUnique(t *testing.T) {
	for _, job := range jobs.RuntimeJobCatalog() {
		opts := job.InsertOpts()
		if opts.MaxAttempts < 1 {
			t.Fatalf("%s MaxAttempts = %d", job.Kind(), opts.MaxAttempts)
		}
		foundDiscarded := false
		for _, state := range opts.UniqueOpts.ByState {
			if state == "discarded" {
				foundDiscarded = true
			}
		}
		if !foundDiscarded {
			t.Fatalf("%s does not retain uniqueness after terminal failure", job.Kind())
		}
	}
}
