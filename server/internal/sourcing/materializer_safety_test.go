package sourcing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/storage"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type failingCommitStagingManager struct {
	storage.StagingManager
	commitErr     error
	quarantineErr error
}

func (manager *failingCommitStagingManager) CommitStagingFile(repo.Repository, *storage.StagingFile, string) error {
	return manager.commitErr
}

func (manager *failingCommitStagingManager) MoveStagingToFailed(repo.Repository, *storage.StagingFile) error {
	return manager.quarantineErr
}

func TestStagingHandleAcceptsOnlyPrivateRelativePath(t *testing.T) {
	repositoryID := uuid.New()
	valid, err := stagingHandle(repositoryID, IngestSource{
		StagingPath:      ".lumilio/staging/incoming/upload.jpg",
		OriginalFilename: "upload.jpg",
		Timestamp:        time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if valid.RepositoryID != repositoryID || valid.PrivatePath != ".lumilio/staging/incoming/upload.jpg" {
		t.Fatalf("unexpected staging handle: %+v", valid)
	}
	for _, candidate := range []string{"/tmp/upload.jpg", "../upload.jpg", "inbox/upload.jpg"} {
		if _, err := stagingHandle(repositoryID, IngestSource{StagingPath: candidate, OriginalFilename: "upload.jpg"}); err == nil {
			t.Fatalf("accepted invalid staging path %q", candidate)
		}
	}
}

func TestRecoverableStagingStateUsesStructuredFields(t *testing.T) {
	status := statusdb.NewTrackedProcessingStatus("localized message", pipelineTaskNames(dbtypes.AssetTypePhoto))
	status.SetIngestState(statusdb.IngestPhasePrepared, "", ".lumilio/staging/incoming/file.jpg", true)
	statusJSON, err := status.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	asset := &repo.Asset{Status: statusJSON}
	if !isRecoverableStagingAsset(asset) {
		t.Fatal("structured prepared state was not recoverable")
	}
	if got := recoverableStagingPath(statusJSON); got != ".lumilio/staging/incoming/file.jpg" {
		t.Fatalf("recovery path = %q", got)
	}
}

func TestCommitAndQuarantineFailurePreservesRecoverableEvidence(t *testing.T) {
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	repositoryConfig := repocfg.NewRepositoryConfig("failure evidence")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "failure evidence", Path: repositoryPath, Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Status: dbtypes.RepoStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, catalog.Queries)
	actualStaging := storage.NewStagingManager(files)
	staged, opened, err := actualStaging.CreateStagingFile(repository, "original.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.WriteString("only original media"); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	commitErr := errors.New("simulated commit failure")
	staging := &failingCommitStagingManager{
		StagingManager: actualStaging,
		commitErr:      commitErr,
		quarantineErr:  errors.New("simulated quarantine failure"),
	}
	materializer := NewSourceMaterializer(
		catalog, staging, newTestPipelineClient(t, catalog), zap.NewNop(),
		logging.NewRepositoryAuditProvider(zap.NewNop(), false), files,
	)
	asset, err := materializer.MaterializeStaged(ctx, IngestSource{
		RepositoryID: repositoryID, Kind: IngestSourceUpload, StagingPath: staged.PrivatePath,
		OriginalFilename: "original.jpg", Timestamp: staged.CreatedAt, ContentType: "image/jpeg",
	})
	if asset != nil || !errors.Is(err, commitErr) {
		t.Fatalf("materialize = %+v/%v", asset, err)
	}
	repositoryFS, err := files.Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	privatePath, _ := storage.ParsePrivateRepositoryPath(staged.PrivatePath)
	if _, err := repositoryFS.StatPrivate(privatePath); err != nil {
		t.Fatalf("staging evidence was lost: %v", err)
	}
	_ = repositoryFS.Close()

	assets, err := catalog.Queries.ListAssetsByRepositoryAny(ctx, uuid.NullUUID{UUID: repositoryID, Valid: true})
	if err != nil || len(assets) != 1 {
		t.Fatalf("recoverable assets = %d/%v", len(assets), err)
	}
	status, err := statusdb.FromJSON(assets[0].Status)
	if err != nil {
		t.Fatal(err)
	}
	if status.Ingest == nil || status.Ingest.Phase != statusdb.IngestPhaseCommitFailed || !status.Ingest.Recoverable || status.Ingest.StagingPath != staged.PrivatePath {
		t.Fatalf("recoverable status = %+v", status.Ingest)
	}
	restarted := NewSourceMaterializer(
		catalog, actualStaging, newTestPipelineClient(t, catalog), zap.NewNop(),
		logging.NewRepositoryAuditProvider(zap.NewNop(), false), files,
	)
	recovered, err := restarted.MaterializeStaged(ctx, IngestSource{
		RepositoryID: repositoryID, Kind: IngestSourceUpload, StagingPath: staged.PrivatePath,
		OriginalFilename: "original.jpg", Timestamp: staged.CreatedAt, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("recover prepared ingest after restart: %v", err)
	}
	if recovered.AssetID != assets[0].AssetID {
		t.Fatalf("recovery replaced asset %s with %s", assets[0].AssetID, recovered.AssetID)
	}
	indexed, err := catalog.Queries.GetRepositoryFileIndexEntry(ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: repositoryID, StoragePath: *recovered.StoragePath,
	})
	if err != nil || !indexed.AssetID.Valid || indexed.AssetID.UUID != recovered.AssetID {
		t.Fatalf("recovered index binding = %+v/%v", indexed, err)
	}
}
