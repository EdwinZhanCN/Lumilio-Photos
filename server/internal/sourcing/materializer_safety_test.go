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
	"server/internal/utils/file"
	"server/internal/utils/hash"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type failingStagingManager struct {
	commitErr     error
	quarantineErr error
	installTarget bool
}

func (*failingStagingManager) CreateStagingFile(string, string) (*storage.StagingFile, error) {
	return nil, errors.New("unused")
}

func (manager *failingStagingManager) CommitStagingFile(stagingFile *storage.StagingFile, finalPath string) error {
	if manager.installTarget {
		target := filepath.Join(stagingFile.RepoPath, filepath.FromSlash(finalPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return errors.Join(manager.commitErr, err)
		}
		if err := os.Link(stagingFile.Path, target); err != nil {
			return errors.Join(manager.commitErr, err)
		}
	}
	return manager.commitErr
}

func (*failingStagingManager) CommitStagingFileToInbox(*storage.StagingFile, string) (string, error) {
	return "", errors.New("unused")
}

func (*failingStagingManager) ResolveInboxPath(string, string, string) (string, error) {
	return "inbox/original.jpg", nil
}

func (manager *failingStagingManager) MoveStagingToFailed(*storage.StagingFile) error {
	return manager.quarantineErr
}

func (*failingStagingManager) CleanupStaging(string, time.Duration) error {
	return nil
}

func TestResolveInPlaceSourceRejectsRepositoryEscape(t *testing.T) {
	repositoryPath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "outside.jpg")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, sourcePath := range []string{
		"../outside.jpg",
		`..\outside.jpg`,
		"/absolute.jpg",
		`C:\absolute.jpg`,
		`C:drive-relative.jpg`,
	} {
		t.Run(sourcePath, func(t *testing.T) {
			if _, _, err := resolveInPlaceSource(repositoryPath, sourcePath); err == nil {
				t.Fatalf("resolveInPlaceSource(%q) accepted an escaping path", sourcePath)
			}
		})
	}

	linkPath := filepath.Join(repositoryPath, "outside-link.jpg")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	if _, _, err := resolveInPlaceSource(repositoryPath, "outside-link.jpg"); err == nil {
		t.Fatal("resolveInPlaceSource accepted a symlink escaping the repository")
	}
}

func TestResolveInPlaceSourceAllowsContainedFileAndSymlink(t *testing.T) {
	repositoryPath := t.TempDir()
	filePath := filepath.Join(repositoryPath, "photos", "inside.jpg")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	storagePath, fullPath, err := resolveInPlaceSource(repositoryPath, "photos/inside.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if storagePath != "photos/inside.jpg" || fullPath != filePath {
		t.Fatalf("resolved path = %q/%q", storagePath, fullPath)
	}

	linkPath := filepath.Join(repositoryPath, "inside-link.jpg")
	if err := os.Symlink(filePath, linkPath); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	if _, _, err := resolveInPlaceSource(repositoryPath, "inside-link.jpg"); err != nil {
		t.Fatalf("contained symlink rejected: %v", err)
	}
}

func TestExistingFinalMustMatchBeforeStagingRemoval(t *testing.T) {
	root := t.TempDir()
	finalPath := filepath.Join(root, "final.jpg")
	stagingPath := filepath.Join(root, "staging.jpg")
	content := []byte("same original bytes")
	if err := os.WriteFile(finalPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagingPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	contentHash, err := hash.CalculateBLAKE3(stagingPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyFinalAndRemoveStaging(stagingPath, finalPath, contentHash, int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stagingPath); !os.IsNotExist(err) {
		t.Fatalf("verified duplicate staging file remains: %v", err)
	}

	conflictPath := filepath.Join(root, "conflict-staging.jpg")
	conflictContent := []byte("different original")
	if err := os.WriteFile(conflictPath, conflictContent, 0o600); err != nil {
		t.Fatal(err)
	}
	conflictHash, err := hash.CalculateBLAKE3(conflictPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyFinalAndRemoveStaging(conflictPath, finalPath, conflictHash, int64(len(conflictContent))); err == nil {
		t.Fatal("content conflict unexpectedly succeeded")
	}
	for _, path := range []string{finalPath, conflictPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("conflict removed %s: %v", path, err)
		}
	}
}

func TestRecoverableStagingStateDoesNotDependOnMessage(t *testing.T) {
	status := statusdb.NewTrackedProcessingStatus("copy changed by localization", pipelineTaskNames(dbtypes.AssetTypePhoto))
	status.SetIngestState(statusdb.IngestPhasePrepared, "", ".lumilio/staging/incoming/file.jpg", true)
	statusJSON, err := status.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !isRecoverableStagingAsset(&repo.Asset{Status: statusJSON}) {
		t.Fatal("structured prepared state was not recoverable")
	}
}

func TestCommitAndQuarantineFailureReturnsErrorAndPreservesSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	catalogDir := t.TempDir()
	if err := os.Chmod(catalogDir, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close(context.Background())
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repositoryPath := t.TempDir()
	repositoryID := uuid.New()
	repositoryConfig := repocfg.DefaultRepositoryConfig()
	repositoryConfig.ID = repositoryID.String()
	repositoryConfig.Name = "Safety"
	repositoryConfig.CreatedAt = time.Now().UTC()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID:    repositoryID,
		Name:      "Safety",
		Path:      repositoryPath,
		Config:    *repositoryConfig,
		Role:      dbtypes.RepoRolePrimary,
		Status:    dbtypes.RepoStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	stagingPath := filepath.Join(repositoryPath, ".lumilio", "staging", "incoming", "original.jpg")
	if err := os.MkdirAll(filepath.Dir(stagingPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagingPath, []byte("only original media"), 0o600); err != nil {
		t.Fatal(err)
	}

	commitErr := errors.New("simulated commit failure")
	manager := &failingStagingManager{
		commitErr:     commitErr,
		quarantineErr: errors.New("simulated quarantine failure"),
		installTarget: true,
	}
	materializer := NewSourceMaterializer(
		catalog,
		manager,
		nil,
		zap.NewNop(),
		logging.NewRepositoryAuditProvider(zap.NewNop(), false),
	)
	asset, err := materializer.materializeFromStaging(
		ctx,
		IngestSource{
			RepositoryID:     repositoryID,
			Kind:             IngestSourceUpload,
			SourcePath:       stagingPath,
			OriginalFilename: "original.jpg",
			Timestamp:        time.Now().UTC(),
			ContentType:      "image/jpeg",
		},
		repository,
		&file.ValidationResult{
			Valid:     true,
			AssetType: dbtypes.AssetTypePhoto,
			MimeType:  "image/jpeg",
		},
	)
	if asset != nil || err == nil || !errors.Is(err, commitErr) {
		t.Fatalf("materializeFromStaging = %+v/%v", asset, err)
	}
	if _, err := os.Stat(stagingPath); err != nil {
		t.Fatalf("unique staging source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, "inbox", "original.jpg")); err != nil {
		t.Fatalf("partial commit target was not retained: %v", err)
	}

	hashResult, err := hash.CalculateLayeredBLAKE3(stagingPath)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := materializer.findExistingContent(ctx, repositoryID, hashResult.ContentHash, int64(len("only original media")))
	if err != nil || prepared == nil {
		t.Fatalf("load recoverable asset = %+v/%v", prepared, err)
	}
	status, err := statusdb.FromJSON(prepared.Status)
	if err != nil {
		t.Fatal(err)
	}
	if status.Ingest == nil ||
		status.Ingest.Phase != statusdb.IngestPhaseCommitFailed ||
		status.Ingest.Code != ingestCodeCommitFailed ||
		!status.Ingest.Recoverable {
		t.Fatalf("recoverable ingest status = %+v", status.Ingest)
	}
}
