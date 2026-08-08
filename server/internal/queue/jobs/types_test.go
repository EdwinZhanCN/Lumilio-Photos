package jobs

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestMLProcessArgsInsertOpts(t *testing.T) {
	tests := map[string]river.InsertOpts{
		"semantic": ProcessSemanticArgs{}.InsertOpts(),
		"bioclip":  ProcessBioClipArgs{}.InsertOpts(),
		"ocr":      ProcessOcrArgs{}.InsertOpts(),
		"face":     ProcessFaceArgs{}.InsertOpts(),
		"zeroshot": ZeroshotClassifyArgs{}.InsertOpts(),
		"video":    ProcessVideoFramesArgs{}.InsertOpts(),
	}

	for name, opts := range tests {
		t.Run(name, func(t *testing.T) {
			if opts.MaxAttempts != MLProcessMaxAttempts {
				t.Fatalf("expected max attempts %d, got %d", MLProcessMaxAttempts, opts.MaxAttempts)
			}
			// ML jobs must be unique by args so that overlapping reindex/retry
			// fan-out collapses to one job per asset instead of racing the
			// non-transactional OCR/face save paths.
			if !opts.UniqueOpts.ByArgs {
				t.Fatalf("expected %s jobs to be unique by args", name)
			}
			if opts.UniqueOpts.ByPeriod != 5*time.Minute {
				t.Fatalf("expected %s jobs to use a 5-minute uniqueness period, got %s", name, opts.UniqueOpts.ByPeriod)
			}
			if slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) {
				t.Fatalf("completed %s jobs must not block explicit reprocessing", name)
			}
			for _, state := range []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			} {
				if !slices.Contains(opts.UniqueOpts.ByState, state) {
					t.Fatalf("expected %s jobs to dedupe active state %s", name, state)
				}
			}
		})
	}
}

func TestAssetRetryPayloadDedupesOnlyOverlappingRequestsPerAsset(t *testing.T) {
	t.Parallel()

	opts := AssetRetryPayload{}.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatal("retry jobs must be unique by asset arguments")
	}
	if slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) {
		t.Fatal("a completed retry must not block a later explicit retry")
	}
	if !slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateRunning) {
		t.Fatal("overlapping running retries must be deduplicated")
	}
}

func TestScanRepositoryArgsKindAndInsertOpts(t *testing.T) {
	args := ScanRepositoryArgs{
		RepositoryID: "11111111-1111-1111-1111-111111111111",
		Mode:         RepositoryScanModeManual,
		Force:        true,
	}

	if args.Kind() != "scan_repository" {
		t.Fatalf("unexpected kind: %s", args.Kind())
	}
	opts := args.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("expected scan repository jobs to be unique by args")
	}
	if opts.UniqueOpts.ByPeriod == 0 {
		t.Fatalf("expected scan repository jobs to use uniqueness by period")
	}
}

func TestDatabaseBackupArgsOnlyDedupesPeriodicTicks(t *testing.T) {
	periodic := (DatabaseBackupArgs{}).InsertOpts()
	if !periodic.UniqueOpts.ByArgs || periodic.UniqueOpts.ByPeriod != 30*time.Minute {
		t.Fatalf("periodic backup uniqueness = %+v", periodic.UniqueOpts)
	}

	forced := (DatabaseBackupArgs{Force: true}).InsertOpts()
	if forced.UniqueOpts.ByArgs || forced.UniqueOpts.ByPeriod != 0 {
		t.Fatalf("forced backup must always enqueue, got uniqueness %+v", forced.UniqueOpts)
	}
}

func TestProcessPHashArgsInsertOpts(t *testing.T) {
	args := ProcessPHashArgs{}

	if args.Kind() != "process_phash" {
		t.Fatalf("unexpected kind: %s", args.Kind())
	}

	opts := args.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("expected process pHash jobs to be unique by args")
	}
	if opts.UniqueOpts.ByPeriod == 0 {
		t.Fatalf("expected process pHash jobs to use uniqueness by period")
	}
}

func TestDiscoverAssetArgsInsertOptsAreUniqueByPath(t *testing.T) {
	args := DiscoverAssetArgs{
		RepositoryID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		StoragePath:      "album/photo.jpg",
		ScanID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		ObservationToken: "obs-v1:test",
	}

	if args.Kind() != "discover_asset" {
		t.Fatalf("unexpected kind: %s", args.Kind())
	}
	opts := args.InsertOpts()
	if !opts.UniqueOpts.ByArgs {
		t.Fatalf("expected discover asset jobs to be unique by args")
	}
	if opts.UniqueOpts.ByPeriod == 0 {
		t.Fatalf("expected discover asset jobs to use uniqueness by period")
	}
}
