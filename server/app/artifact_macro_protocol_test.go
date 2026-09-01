package app

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"server/internal/artifact"
	"server/internal/commit"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/execution"
	"server/internal/queue"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/workqos"
)

func TestDerivativeMacroWithholdsRiverSuccessAcrossArtifactCommitCrashWindows(t *testing.T) {
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "artifact-macro.sqlite3")+"?_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	for _, statement := range []string{
		`CREATE TABLE assets(asset_id TEXT PRIMARY KEY,type TEXT,status TEXT,updated_at INTEGER)`,
		`CREATE TABLE asset_pipeline_state(asset_id TEXT,source_content_id TEXT,stage TEXT,pipeline_version TEXT,desired_version INTEGER,applied_version INTEGER,terminal_error TEXT,updated_at INTEGER,PRIMARY KEY(asset_id,stage))`,
		`CREATE TABLE thumbnails(thumbnail_id INTEGER PRIMARY KEY AUTOINCREMENT,asset_id TEXT,size TEXT,storage_path TEXT,mime_type TEXT,created_at INTEGER,repository_id TEXT,UNIQUE(asset_id,size))`,
		`CREATE TABLE catalog_operation_receipts(receipt_id TEXT PRIMARY KEY,kind TEXT,subject_id TEXT,desired_version INTEGER,applied_version INTEGER,state TEXT,terminal_error TEXT,created_at INTEGER,updated_at INTEGER)`,
		`CREATE TABLE asset_pipeline_receipt_stages(receipt_id TEXT,asset_id TEXT,stage TEXT,desired_version INTEGER,PRIMARY KEY(receipt_id,asset_id,stage))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	assetID, sourceFence, repositoryID := uuid.New(), uuid.New(), uuid.New()
	if _, err := database.Exec(`INSERT INTO assets VALUES(?, 'PHOTO', '{"state":"processing"}', 1)`, assetID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO asset_pipeline_state VALUES(?, ?, 'derivatives', 'asset-v1', 1, 0, NULL, 1)`, assetID.String(), sourceFence.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TRIGGER inject_thumbnail_failure BEFORE INSERT ON thumbnails BEGIN SELECT RAISE(ABORT, 'injected catalog failure'); END`); err != nil {
		t.Fatal(err)
	}

	store, files := newMacroArtifactStore(t, repositoryID)
	body := []byte("immutable thumbnail")
	candidateBody := body
	identity := artifact.Identity{SourceFence: sourceFence.String(), Stage: "derivatives", PipelineVersion: "asset-v1", Name: "small.webp"}
	coordinator, err := commit.New(
		catalogtx.NewWriter(database, nil),
		commit.Config{Capacity: 4, MaxBatch: 1, OldestWait: time.Millisecond},
		commit.CatalogDependencies{},
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	t.Cleanup(func() { _ = coordinator.Stop(context.Background()) })
	budget := execution.Budget{
		CPU: 1, DiskIO: 1, ImageCodec: 1, VideoCodec: 1, MemoryBytes: 256 << 20, MaxWaiting: 2, ToolSession: execution.ToolSession{Threads: 1, SoftwarePreset: "veryfast", HardwareAccel: "none"},
	}
	governor, err := budget.Governor()
	if err != nil {
		t.Fatal(err)
	}
	runtime := &pipelineRuntime{engine: execution.NewEngine(governor), demand: budget.DemandCatalog(), commits: coordinator}
	args := jobs.GenerateAssetDerivativesArgs{
		AssetID: assetID, SourceFence: sourceFence, DesiredVersion: 1, PipelineVersion: "asset-v1",
	}
	worker := queue.NewGenerateAssetDerivativesWorker(func(ctx context.Context, _ workqos.Class, args jobs.GenerateAssetDerivativesArgs) error {
		return runtime.engine.Run(ctx, execution.ClassBackground, runtime.demand.Demand(execution.StepDerivativesComputeThumb, execution.MediaPhoto), func(stepCtx context.Context) error {
			published, err := store.Publish(stepCtx, identity, bytes.NewReader(candidateBody))
			if err != nil {
				return err
			}
			if _, err := runtime.commits.ApplyAssetDerivatives(stepCtx, commit.AssetDerivativesApplied{
				AssetID: args.AssetID, SourceFence: args.SourceFence,
				PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion,
				Artifacts: []commit.ThumbnailArtifact{{
					RepositoryID: repositoryID, Size: "small", StoragePath: published.Path, MimeType: "image/webp",
				}},
			}); err != nil {
				return err
			}
			return runtime.submitAssetStage(stepCtx, args.AssetID, args.SourceFence, "derivatives", args.PipelineVersion, args.DesiredVersion)
		})
	})
	priority, err := workqos.Background.Priority()
	if err != nil {
		t.Fatal(err)
	}
	job := &river.Job[jobs.GenerateAssetDerivativesArgs]{JobRow: &rivertype.JobRow{Priority: priority}, Args: args}

	if err := worker.Work(context.Background(), job); err == nil {
		t.Fatal("macro reported River success after injected catalog rollback")
	}
	privatePath, err := storage.ParsePrivateRepositoryPath(identityMustPath(t, identity))
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := files.ReadPrivateFile(privatePath)
	if err != nil || !bytes.Equal(persisted, body) {
		t.Fatalf("published artifact after catalog rollback = %q, error = %v", persisted, err)
	}
	var thumbnails int
	var applied uint64
	if err := database.QueryRow(`SELECT count(*) FROM thumbnails`).Scan(&thumbnails); err != nil || thumbnails != 0 {
		t.Fatalf("thumbnail rows after rollback = %d, error = %v", thumbnails, err)
	}
	if err := database.QueryRow(`SELECT applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='derivatives'`, assetID.String()).Scan(&applied); err != nil || applied != 0 {
		t.Fatalf("applied version after rollback = %d, error = %v", applied, err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Acknowledgements != 0 || snapshot.Failures != 1 {
		t.Fatalf("rollback acknowledgement snapshot = %+v", snapshot)
	}

	if _, err := database.Exec(`DROP TRIGGER inject_thumbnail_failure`); err != nil {
		t.Fatal(err)
	}
	candidateBody = []byte("byte-different equivalent thumbnail")
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("macro retry after artifact publication: %v", err)
	}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("macro redelivery after catalog ACK: %v", err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM thumbnails`).Scan(&thumbnails); err != nil || thumbnails != 1 {
		t.Fatalf("thumbnail rows after redelivery = %d, error = %v", thumbnails, err)
	}
	if err := database.QueryRow(`SELECT applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='derivatives'`, assetID.String()).Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("applied version after redelivery = %d, error = %v", applied, err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Acknowledgements != 4 || snapshot.Duplicate != 2 {
		t.Fatalf("post-ACK redelivery snapshot = %+v", snapshot)
	}
}

func newMacroArtifactStore(t *testing.T, repositoryID uuid.UUID) (*artifact.Store, *storage.RepositoryFS) {
	t.Helper()
	root := t.TempDir()
	configuration := repocfg.NewRepositoryConfig("macro artifact protocol")
	configuration.ID = repositoryID.String()
	if err := configuration.SaveConfigToFile(root); err != nil {
		t.Fatal(err)
	}
	repository := repo.Repository{
		RepoID: repositoryID, Path: root,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle,
		Config:       *configuration,
	}
	files, err := storage.NewRepositoryFSFactory(nil, nil).Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })
	store, err := artifact.NewStore(files)
	if err != nil {
		t.Fatal(err)
	}
	return store, files
}

func identityMustPath(t *testing.T, identity artifact.Identity) string {
	t.Helper()
	path, err := identity.Path()
	if err != nil {
		t.Fatal(err)
	}
	return path.String()
}
