package jobs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestRuntimeJobCatalogIsClosedAndNeverUsesDefaultQueue(t *testing.T) {
	t.Parallel()

	declared := declaredKindReceivers(t)
	catalogTypes := make([]string, 0, len(RuntimeJobCatalog()))
	kinds := make(map[string]string)
	for _, job := range RuntimeJobCatalog() {
		typeName := reflect.TypeOf(job).Name()
		catalogTypes = append(catalogTypes, typeName)
		if previous, exists := kinds[job.Kind()]; exists {
			t.Fatalf("job kind %q is shared by %s and %s", job.Kind(), previous, typeName)
		}
		kinds[job.Kind()] = typeName
		queue := job.InsertOpts().Queue
		if queue == "" || queue == river.QueueDefault {
			t.Fatalf("%s (%s) routes to forbidden implicit queue %q", typeName, job.Kind(), queue)
		}
		states := job.InsertOpts().UniqueOpts.ByState
		if len(states) > 0 {
			for _, required := range []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			} {
				if !slices.Contains(states, required) {
					t.Fatalf("%s (%s) uses River-invalid uniqueness states: missing %s in %v", typeName, job.Kind(), required, states)
				}
			}
		}
	}
	sort.Strings(catalogTypes)
	if !slices.Equal(catalogTypes, declared) {
		t.Fatalf("RuntimeJobCatalog drift:\n catalog=%v\n declared Kind receivers=%v", catalogTypes, declared)
	}
	if queues := RuntimeQueueNames(); len(queues) != 23 {
		t.Fatalf("runtime queue count = %d, want 23: %v", len(queues), queues)
	}
}

func declaredKindReceivers(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "types.go", nil, 0)
	if err != nil {
		t.Fatalf("parse types.go: %v", err)
	}
	receivers := make([]string, 0)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "Kind" || function.Recv == nil || len(function.Recv.List) != 1 {
			continue
		}
		receiver := function.Recv.List[0].Type
		if pointer, ok := receiver.(*ast.StarExpr); ok {
			receiver = pointer.X
		}
		identifier, ok := receiver.(*ast.Ident)
		if !ok {
			t.Fatalf("unsupported Kind receiver syntax %T", receiver)
		}
		receivers = append(receivers, identifier.Name)
	}
	sort.Strings(receivers)
	return receivers
}

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
			if opts.UniqueOpts.ByPeriod != 0 {
				t.Fatalf("%s job uniqueness must not roll over while work is running", name)
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

func TestLocalProcessingJobsDeduplicateCompletedRevisionEffectReplay(t *testing.T) {
	t.Parallel()
	for name, opts := range map[string]river.InsertOpts{
		"metadata":  (MetadataArgs{}).InsertOpts(),
		"thumbnail": (ThumbnailArgs{}).InsertOpts(),
		"transcode": (TranscodeArgs{}).InsertOpts(),
	} {
		if !opts.UniqueOpts.ByArgs {
			t.Fatalf("%s retry jobs must deduplicate identical arguments", name)
		}
		if !slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) {
			t.Fatalf("completed %s effect replay must remain deduplicated", name)
		}
		if !slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateRunning) {
			t.Fatalf("running %s work must block duplicate retry fan-out", name)
		}
	}
}

func TestRepositoryObservationJobsCarryStableRevisionIdentity(t *testing.T) {
	controller := ObserveRepositoryArgs{
		OperationID:   "22222222-2222-2222-2222-222222222222",
		RepositoryID:  "11111111-1111-1111-1111-111111111111",
		ExpectedEpoch: 7,
	}
	if controller.Kind() != "observe_repository" || !controller.InsertOpts().UniqueOpts.ByArgs {
		t.Fatalf("controller job contract = %+v", controller.InsertOpts())
	}
	if !slices.Contains(controller.InsertOpts().UniqueOpts.ByState, rivertype.JobStateRunning) {
		t.Fatal("running controller work must participate in River uniqueness; continuation uses snooze")
	}

	hash := HashRepositoryNodeArgs{
		NodeID: "33333333-3333-3333-3333-333333333333", ExpectedRevision: 19,
	}
	if hash.Kind() != "hash_repository_node" || !hash.InsertOpts().UniqueOpts.ByArgs {
		t.Fatalf("hash job contract = %+v", hash.InsertOpts())
	}
	if !slices.Contains(hash.InsertOpts().UniqueOpts.ByState, rivertype.JobStateRunning) {
		t.Fatal("equivalent running node/revision hashes must deduplicate")
	}
}

func TestDatabaseBackupArgsOnlyDedupesPeriodicTicks(t *testing.T) {
	periodic := (DatabaseBackupArgs{}).InsertOpts()
	if !periodic.UniqueOpts.ByArgs || periodic.UniqueOpts.ByPeriod != 0 ||
		!slices.Contains(periodic.UniqueOpts.ByState, rivertype.JobStateRunning) ||
		slices.Contains(periodic.UniqueOpts.ByState, rivertype.JobStateCompleted) {
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
	if opts.UniqueOpts.ByPeriod != 0 || !slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateRunning) ||
		slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) {
		t.Fatalf("process pHash uniqueness does not converge across clock windows: %+v", opts.UniqueOpts)
	}
}

func TestRepeatableQueueWorkCannotAcquireFollowerAcrossClockWindows(t *testing.T) {
	t.Parallel()

	tests := map[string]river.InsertOpts{
		"asset retry":      (AssetRetryPayload{}).InsertOpts(),
		"OCR outbox":       (ProcessOCROutboxArgs{}).InsertOpts(),
		"Live Photo match": (LivePhotoMatchArgs{}).InsertOpts(),
		"pHash":            (ProcessPHashArgs{}).InsertOpts(),
		"periodic backup":  (DatabaseBackupArgs{}).InsertOpts(),
		"repository scans": (ScheduleRepositoryScansArgs{}).InsertOpts(),
	}
	for name, opts := range tests {
		t.Run(name, func(t *testing.T) {
			if !opts.UniqueOpts.ByArgs {
				t.Fatal("repeatable work must have stable argument identity")
			}
			if opts.UniqueOpts.ByPeriod != 0 {
				t.Fatalf("ByPeriod=%s permits a running follower in the next window", opts.UniqueOpts.ByPeriod)
			}
			if !slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateRunning) {
				t.Fatal("running work must participate in uniqueness")
			}
			if slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) {
				t.Fatal("completed repeatable work must permit the next factual run")
			}
		})
	}
}

func TestMutableProjectionQueuesCoalesceAcrossEveryActiveState(t *testing.T) {
	t.Parallel()
	for name, opts := range map[string]river.InsertOpts{
		"Event rebuild":    (EventRebuildArgs{}).InsertOpts(),
		"location rebuild": (RebuildLocationClustersArgs{}).InsertOpts(),
		"stack detection":  (DetectStacksArgs{}).InsertOpts(),
	} {
		t.Run(name, func(t *testing.T) {
			if !opts.UniqueOpts.ByArgs || opts.UniqueOpts.ByPeriod != 0 {
				t.Fatalf("mutable projection uniqueness = %+v", opts.UniqueOpts)
			}
			for _, state := range []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			} {
				if !slices.Contains(opts.UniqueOpts.ByState, state) {
					t.Fatalf("queued state %s must coalesce additional followers", state)
				}
			}
			if slices.Contains(opts.UniqueOpts.ByState, rivertype.JobStateCompleted) {
				t.Fatal("completed projection must permit a later factual rebuild")
			}
		})
	}
}

func TestEventRebuildUsesRiverValidActiveStateCoalescing(t *testing.T) {
	automatic := (EventRebuildArgs{OwnerID: 7}).InsertOpts().UniqueOpts
	if !slices.Contains(automatic.ByState, rivertype.JobStateRunning) {
		t.Fatal("Event uniqueness must include running; the worker snoozes itself when its source revision changes")
	}
	forced := (EventRebuildArgs{OwnerID: 7, Force: true}).InsertOpts().UniqueOpts
	if !slices.Contains(forced.ByState, rivertype.JobStateRunning) {
		t.Fatal("forced Event rebuilds must obey River's required active-state uniqueness")
	}
	if !slices.Contains(forced.ByState, rivertype.JobStateAvailable) {
		t.Fatal("additional forced requests must coalesce into the queued follower")
	}
}
