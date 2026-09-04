package commit

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/artifact"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
)

func TestArtifactCatalogFailureWithholdsACKAndCommittedRetryIsNoop(t *testing.T) {
	writer, database := testWriter(t)
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

	store, files := newCommitArtifactStore(t, repositoryID)
	body := []byte("immutable thumbnail")
	identity := artifact.Identity{SourceFence: sourceFence.String(), Stage: "derivatives", PipelineVersion: "asset-v1", Name: "small.webp"}
	published, err := store.Publish(context.Background(), identity, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	payload := AssetDerivativesApplied{
		AssetID: assetID, SourceFence: sourceFence, PipelineVersion: "asset-v1", DesiredVersion: 1,
		Artifacts: []ThumbnailArtifact{{RepositoryID: repositoryID, Size: "small", StoragePath: published.Path, MimeType: "image/webp"}},
	}
	injected := errors.New("injected catalog failure after artifact publication")
	failBeforeCommit := true
	derivativeOperation := func() Operation {
		return Operation{Kind: OperationKindCatalogAssetDerivatives, Apply: func(ctx context.Context, tx *sql.Tx) (Result, error) {
			result, applyErr := applyAssetDerivatives(ctx, tx, payload)
			if applyErr == nil && failBeforeCommit {
				failBeforeCommit = false
				return Result{}, injected
			}
			return Result{Outcome: result}, applyErr
		}}
	}
	coordinator, err := New(writer, Config{Capacity: 2, MaxBatch: 1, OldestWait: time.Millisecond}, CatalogDependencies{})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Start()
	defer coordinator.Stop(context.Background())

	failedResult, err := coordinator.SubmitOperation(context.Background(), derivativeOperation())
	if !errors.Is(err, injected) || failedResult != (Result{}) {
		t.Fatalf("failed commit result = %+v, error = %v", failedResult, err)
	}
	var thumbnailCount int
	if err := database.QueryRow(`SELECT count(*) FROM thumbnails`).Scan(&thumbnailCount); err != nil || thumbnailCount != 0 {
		t.Fatalf("thumbnail rows after rollback = %d, error = %v", thumbnailCount, err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.Acknowledgements != 0 || snapshot.Failures != 1 {
		t.Fatalf("failed commit emitted an acknowledgement: %+v", snapshot)
	}
	privatePath, err := storage.ParsePrivateRepositoryPath(published.Path)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := files.ReadPrivateFile(privatePath)
	if err != nil || !bytes.Equal(persisted, body) {
		t.Fatalf("published artifact after catalog rollback = %q, error = %v", persisted, err)
	}

	recomputedBody := []byte("byte-different equivalent thumbnail")
	republished, err := store.Publish(context.Background(), identity, bytes.NewReader(recomputedBody))
	if err != nil || republished != published {
		t.Fatalf("immutable retry = %+v, error = %v, want %+v", republished, err, published)
	}
	if result, err := coordinator.SubmitOperation(context.Background(), derivativeOperation()); err != nil || result.Outcome != OutcomeApplied {
		t.Fatalf("derivative retry result = %+v, error = %v", result, err)
	}
	if result, err := coordinator.ApplyAssetStage(context.Background(), AssetStageApplied{AssetID: assetID, SourceFence: sourceFence, Stage: "derivatives", PipelineVersion: "asset-v1", DesiredVersion: 1}); err != nil || result.Outcome != OutcomeApplied {
		t.Fatalf("stage ACK result = %+v, error = %v", result, err)
	}

	// Simulate a crash after both catalog ACKs but before River records macro
	// completion. Redelivery repeats publication and both commits. Every step is
	// a no-op success against externally re-read filesystem and catalog state.
	if republished, err = store.Publish(context.Background(), identity, bytes.NewReader(recomputedBody)); err != nil || republished != published {
		t.Fatalf("post-ACK artifact retry = %+v, error = %v", republished, err)
	}
	if result, err := coordinator.SubmitOperation(context.Background(), derivativeOperation()); err != nil || result.Outcome != OutcomeDuplicate {
		t.Fatalf("post-ACK derivative result = %+v, error = %v", result, err)
	}
	if result, err := coordinator.ApplyAssetStage(context.Background(), AssetStageApplied{AssetID: assetID, SourceFence: sourceFence, Stage: "derivatives", PipelineVersion: "asset-v1", DesiredVersion: 1}); err != nil || result.Outcome != OutcomeDuplicate {
		t.Fatalf("post-ACK stage result = %+v, error = %v", result, err)
	}
	var appliedVersion uint64
	if err := database.QueryRow(`SELECT count(*) FROM thumbnails`).Scan(&thumbnailCount); err != nil || thumbnailCount != 1 {
		t.Fatalf("thumbnail rows after redelivery = %d, error = %v", thumbnailCount, err)
	}
	if err := database.QueryRow(`SELECT applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='derivatives'`, assetID.String()).Scan(&appliedVersion); err != nil || appliedVersion != 1 {
		t.Fatalf("applied version after redelivery = %d, error = %v", appliedVersion, err)
	}
}

func newCommitArtifactStore(t *testing.T, repositoryID uuid.UUID) (*artifact.Store, *storage.RepositoryFS) {
	t.Helper()
	root := t.TempDir()
	config := repocfg.NewRepositoryConfig("commit artifact protocol")
	config.ID = repositoryID.String()
	if err := config.SaveConfigToFile(root); err != nil {
		t.Fatal(err)
	}
	repository := repo.Repository{
		RepoID: repositoryID, Path: root,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle,
		Config:       *config,
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
