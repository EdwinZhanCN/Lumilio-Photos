package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	dbbackup "server/internal/db/backup"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
)

func TestRepositoryCutoverPreflightCreatesVerifiedBackupAndQuarantinesStaging(t *testing.T) {
	ctx := context.Background()
	database, repository, staging, appConfig := newCutoverPreflightFixture(t, ctx)
	staged := createCutoverStaging(t, staging, repository, "uncertain.jpg", "uncertain-bytes")

	if err := preflightRepositoryObservationCutover(
		ctx,
		database,
		appConfig,
		db.DestructiveMigrationPlan{FromApplicationMigration: 13, ToApplicationMigration: 14},
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repository.Path, filepath.FromSlash(staged.PrivatePath))); !os.IsNotExist(err) {
		t.Fatalf("incoming staging remains after preflight: %v", err)
	}
	failedEntries, err := os.ReadDir(filepath.Join(repository.Path, filepath.FromSlash(storage.DefaultStructure.FailedDir)))
	if err != nil {
		t.Fatal(err)
	}
	foundBytes := false
	for _, entry := range failedEntries {
		content, readErr := os.ReadFile(filepath.Join(
			repository.Path,
			filepath.FromSlash(storage.DefaultStructure.FailedDir),
			entry.Name(),
		))
		if readErr != nil {
			t.Fatal(readErr)
		}
		foundBytes = foundBytes || string(content) == "uncertain-bytes"
	}
	if !foundBytes {
		t.Fatal("uncertain staging bytes were not preserved in quarantine")
	}

	backupEntries, err := os.ReadDir(appConfig.StorageConfig.BackupsDir())
	if err != nil {
		t.Fatal(err)
	}
	var snapshotPath string
	for _, entry := range backupEntries {
		if strings.HasPrefix(entry.Name(), dbbackup.CutoverPointPrefix) &&
			strings.HasSuffix(entry.Name(), ".sqlite3") {
			snapshotPath = filepath.Join(appConfig.StorageConfig.BackupsDir(), entry.Name())
			break
		}
	}
	if snapshotPath == "" {
		t.Fatal("preflight did not create a protected cutover snapshot")
	}
	manifest, info, err := dbbackup.ValidateSnapshot(ctx, snapshotPath, dbbackup.Compatibility{
		ConfigSchemaVersion:     config.SchemaVersion,
		MaxApplicationMigration: 13,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ApplicationMigration != 13 || info.ApplicationMigration != 13 || info.QuickCheck != "ok" {
		t.Fatalf("verified cutover snapshot = manifest %+v info %+v", manifest, info)
	}
}

func TestRepositoryCutoverPreflightBackupFailureDoesNotMoveStaging(t *testing.T) {
	ctx := context.Background()
	database, repository, staging, appConfig := newCutoverPreflightFixture(t, ctx)
	staged := createCutoverStaging(t, staging, repository, "keep.jpg", "keep-bytes")
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	appConfig.StorageConfig.BackupsPath = blocked

	err := preflightRepositoryObservationCutover(
		ctx,
		database,
		appConfig,
		db.DestructiveMigrationPlan{FromApplicationMigration: 13, ToApplicationMigration: 14},
		nil,
	)
	if err == nil {
		t.Fatal("unreachable backup destination did not abort preflight")
	}
	content, readErr := os.ReadFile(filepath.Join(repository.Path, filepath.FromSlash(staged.PrivatePath)))
	if readErr != nil || string(content) != "keep-bytes" {
		t.Fatalf("staging changed before backup completed: content=%q err=%v", content, readErr)
	}
}

func newCutoverPreflightFixture(
	t *testing.T,
	ctx context.Context,
) (*db.DB, repo.Repository, *storage.DefaultStagingManager, config.AppConfig) {
	t.Helper()
	databaseDir := t.TempDir()
	if err := os.Chmod(databaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(databaseDir, "catalog.sqlite3")
	database, err := db.Open(ctx, config.DatabaseConfig{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "cutover-owner", Password: "unused", DisplayName: "Cutover Owner",
		Role: "admin", WebauthnUserHandle: []byte("cutover-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}

	rootPath := t.TempDir()
	rootID := uuid.New()
	rootConfig := rootcfg.New("Cutover root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(rootPath); err != nil {
		t.Fatal(err)
	}
	repositoryPath := filepath.Join(rootPath, "repository")
	if err := storage.NewDirectoryManager().CreateStructure(repositoryPath); err != nil {
		t.Fatal(err)
	}
	repositoryID := uuid.New()
	repositoryConfig := repocfg.NewRepositoryConfig("Cutover Repository")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := database.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "Cutover root", Path: rootPath,
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := database.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "Cutover Repository", Path: repositoryPath,
		Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle, DefaultOwnerID: &owner.UserID,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, database.Queries)
	staging := storage.NewStagingManager(files)
	backups := filepath.Join(t.TempDir(), "backups")
	return database, repository, staging, config.AppConfig{
		SchemaVersion: config.SchemaVersion,
		StorageConfig: config.StorageConfig{BackupsPath: backups},
	}
}

func createCutoverStaging(
	t *testing.T,
	staging *storage.DefaultStagingManager,
	repository repo.Repository,
	name string,
	content string,
) *storage.StagingFile {
	t.Helper()
	staged, opened, err := staging.CreateStagingFile(repository, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	return staged
}
