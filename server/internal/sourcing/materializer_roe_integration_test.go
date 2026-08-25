package sourcing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/roe/changefeed"
	roecontroller "server/internal/storage/roe/controller"
	roematerializer "server/internal/storage/roe/materializer"
	"server/internal/storage/rootcfg"
)

type sourceMaterializerFixture struct {
	ctx            context.Context
	database       *db.DB
	repository     repo.Repository
	repositoryPath string
	files          *storage.RepositoryFSFactory
	staging        *storage.DefaultStagingManager
	publisher      *roematerializer.HashMaterializer
	materializer   *SourceMaterializer
}

func newSourceMaterializerFixture(t *testing.T) *sourceMaterializerFixture {
	t.Helper()
	ctx := context.Background()
	databaseDirectory := t.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(databaseDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "source-owner", Password: "unused", DisplayName: "Source Owner",
		Role: "admin", WebauthnUserHandle: []byte("source-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}

	rootPath := t.TempDir()
	repositoryPath := filepath.Join(rootPath, "repository")
	if err := os.Mkdir(repositoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	rootID := uuid.New()
	rootConfig := rootcfg.New("source root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(rootPath); err != nil {
		t.Fatal(err)
	}
	repositoryID := uuid.New()
	repositoryConfig := repocfg.NewRepositoryConfig(
		"source repository",
		repocfg.WithStorageStrategy("flat"),
		repocfg.WithLocalSettings("uuid"),
	)
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := database.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "source root", Path: rootPath,
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := database.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "source repository", Path: repositoryPath,
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
	publisher := roematerializer.NewHashMaterializer(database, files)
	return &sourceMaterializerFixture{
		ctx: ctx, database: database, repository: repository, repositoryPath: repositoryPath,
		files: files, staging: staging, publisher: publisher,
		materializer: NewSourceMaterializer(database, staging, publisher, zap.NewNop(), nil, files),
	}
}

func (fixture *sourceMaterializerFixture) stage(
	t *testing.T,
	kind IngestSourceKind,
	filename string,
	contents []byte,
) IngestSource {
	t.Helper()
	staged, writer, err := fixture.staging.CreateStagingFile(fixture.repository, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return IngestSource{
		RepositoryID: fixture.repository.RepoID, Kind: kind,
		StagingPath: staged.PrivatePath, OriginalFilename: filename,
		Timestamp: staged.CreatedAt, ContentType: "image/jpeg",
	}
}

func (fixture *sourceMaterializerFixture) prepareCommit(t *testing.T, source IngestSource) repo.RepositoryStagingCommit {
	t.Helper()
	handle, err := stagingHandle(fixture.repository.RepoID, source.StagingPath, source.OriginalFilename, source.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := fixture.staging.OpenStagingFile(fixture.repository, handle)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := hashOpenedStaging(opened)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := fixture.database.Queries.CreateRepositoryStagingCommit(fixture.ctx, repo.CreateRepositoryStagingCommitParams{
		CommitID: uuid.New(), RepositoryID: fixture.repository.RepoID,
		OwnerID: *fixture.repository.DefaultOwnerID, SourceKind: string(source.Kind),
		StagingPath: source.StagingPath, OriginalFilename: source.OriginalFilename,
		MimeType: source.ContentType, FullHash: verified.ContentHash, FileSize: verified.FileSize,
		QuickFingerprint:        verified.QuickFingerprint,
		QuickFingerprintVersion: verified.QuickFingerprintVersion,
		CreatedAt:               dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func (fixture *sourceMaterializerFixture) chooseTarget(t *testing.T, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
	t.Helper()
	target, err := fixture.staging.ResolveInboxPath(fixture.repository, commit.OriginalFilename, commit.FullHash)
	if err != nil {
		t.Fatal(err)
	}
	commit, err = fixture.database.Queries.SetRepositoryStagingCommitTarget(fixture.ctx, repo.SetRepositoryStagingCommitTargetParams{
		CommitID: commit.CommitID, TargetPath: &target, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func (fixture *sourceMaterializerFixture) claimCommit(t *testing.T, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
	t.Helper()
	claimed, err := fixture.database.Queries.ClaimRepositoryStagingCommit(fixture.ctx, repo.ClaimRepositoryStagingCommitParams{
		CommitID: commit.CommitID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	return claimed
}

func (fixture *sourceMaterializerFixture) moveCommitToTarget(t *testing.T, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
	t.Helper()
	if commit.TargetPath == nil {
		t.Fatal("staging commit has no target")
	}
	handle, err := stagingHandle(commit.RepositoryID, commit.StagingPath, commit.OriginalFilename, commit.CreatedAt.Time)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.staging.CommitStagingFile(fixture.repository, handle, *commit.TargetPath); err != nil {
		t.Fatal(err)
	}
	return commit
}

func TestStagingCommitReplaysEveryPersistedFilesystemCatalogBoundary(t *testing.T) {
	stages := []struct {
		name  string
		setup func(*testing.T, *sourceMaterializerFixture, repo.RepositoryStagingCommit) repo.RepositoryStagingCommit
	}{
		{name: "prepared"},
		{name: "claimed", setup: func(t *testing.T, fixture *sourceMaterializerFixture, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
			return fixture.claimCommit(t, commit)
		}},
		{name: "target chosen", setup: func(t *testing.T, fixture *sourceMaterializerFixture, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
			return fixture.chooseTarget(t, fixture.claimCommit(t, commit))
		}},
		{name: "filesystem move before receipt", setup: func(t *testing.T, fixture *sourceMaterializerFixture, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
			commit = fixture.chooseTarget(t, fixture.claimCommit(t, commit))
			return fixture.moveCommitToTarget(t, commit)
		}},
		{name: "on-disk receipt", setup: func(t *testing.T, fixture *sourceMaterializerFixture, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
			commit = fixture.chooseTarget(t, fixture.claimCommit(t, commit))
			commit = fixture.moveCommitToTarget(t, commit)
			updated, err := fixture.database.Queries.MarkRepositoryStagingCommitOnDisk(fixture.ctx, repo.MarkRepositoryStagingCommitOnDiskParams{
				CommitID: commit.CommitID, TargetPath: commit.TargetPath, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
			})
			if err != nil {
				t.Fatal(err)
			}
			return updated
		}},
		{name: "ROE publication before completion", setup: func(t *testing.T, fixture *sourceMaterializerFixture, commit repo.RepositoryStagingCommit) repo.RepositoryStagingCommit {
			commit = fixture.chooseTarget(t, fixture.claimCommit(t, commit))
			commit = fixture.moveCommitToTarget(t, commit)
			updated, err := fixture.database.Queries.MarkRepositoryStagingCommitOnDisk(fixture.ctx, repo.MarkRepositoryStagingCommitOnDiskParams{
				CommitID: commit.CommitID, TargetPath: commit.TargetPath, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
			})
			if err != nil {
				t.Fatal(err)
			}
			observation, exists, err := fixture.materializer.inspectFinal(
				fixture.ctx,
				fixture.repository,
				*updated.TargetPath,
				storage.HashNone,
			)
			if err != nil || !exists {
				t.Fatalf("inspect committed source = %t, %v", exists, err)
			}
			if _, err := fixture.publisher.PublishKnownContent(fixture.ctx, roematerializer.KnownContent{
				RepositoryID: updated.RepositoryID, OwnerID: updated.OwnerID,
				Source: updated.SourceKind, SourceEventKey: "staging:" + updated.CommitID.String(),
				RelativePath: *updated.TargetPath, OriginalFilename: updated.OriginalFilename,
				MimeType: updated.MimeType, AssetType: string(dbtypes.AssetTypePhoto),
				FullHash: updated.FullHash, FileSize: updated.FileSize,
				QuickFingerprint:        updated.QuickFingerprint,
				QuickFingerprintVersion: updated.QuickFingerprintVersion,
				Observation:             observation,
			}); err != nil {
				t.Fatal(err)
			}
			return updated
		}},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			fixture := newSourceMaterializerFixture(t)
			source := fixture.stage(t, IngestSourceUpload, "resume.jpg", []byte("resume original bytes"))
			commit := fixture.prepareCommit(t, source)
			if stage.setup != nil {
				commit = stage.setup(t, fixture, commit)
			}

			restarted := NewSourceMaterializer(
				fixture.database, fixture.staging, fixture.publisher, zap.NewNop(), nil, fixture.files,
			)
			asset, err := restarted.MaterializeCommit(fixture.ctx, commit.CommitID)
			if err != nil {
				t.Fatalf("resume %s: %v", stage.name, err)
			}
			replayed, err := restarted.MaterializeCommit(fixture.ctx, commit.CommitID)
			if err != nil {
				t.Fatalf("replay completed %s: %v", stage.name, err)
			}
			if replayed.AssetID != asset.AssetID {
				t.Fatalf("replay asset = %s, want %s", replayed.AssetID, asset.AssetID)
			}
			var commits, contents, assets, locations, processingEffects int
			if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
				SELECT
				  (SELECT count(*) FROM repository_staging_commits WHERE status = 'completed'),
				  (SELECT count(*) FROM content_objects),
				  (SELECT count(*) FROM assets),
				  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL),
				  (SELECT count(*) FROM repository_outbox WHERE effect_kind = 'process_asset')
			`).Scan(&commits, &contents, &assets, &locations, &processingEffects); err != nil {
				t.Fatal(err)
			}
			if commits != 1 || contents != 1 || assets != 1 || locations != 1 || processingEffects != 1 {
				t.Fatalf("resume counts commits=%d contents=%d assets=%d locations=%d processing=%d",
					commits, contents, assets, locations, processingEffects)
			}
		})
	}
}

func TestConcurrentScanUploadAndCloudConvergeOnContentAndOwnerAsset(t *testing.T) {
	fixture := newSourceMaterializerFixture(t)
	contents := []byte("one exact original from three source paths")
	if err := os.WriteFile(filepath.Join(fixture.repositoryPath, "scanned.jpg"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	controller := roecontroller.New(
		fixture.database,
		fixture.files,
		roecontroller.Config{BatchSize: 8, ChangeFeed: changefeed.Periodic{}},
		zap.NewNop(),
	)
	receipt, err := controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	for turn := 0; turn < 100; turn++ {
		result, runErr := controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID)
		if runErr != nil {
			t.Fatal(runErr)
		}
		if !result.HasMore {
			break
		}
		if turn == 99 {
			t.Fatal("scan controller did not finish")
		}
	}
	var scanNodeID uuid.UUID
	var scanRevision int64
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT entity_id, expected_revision
		FROM repository_outbox
		WHERE effect_kind = 'hash'
		ORDER BY created_at, outbox_id
		LIMIT 1
	`).Scan(&scanNodeID, &scanRevision); err != nil {
		t.Fatal(err)
	}
	upload := fixture.stage(t, IngestSourceUpload, "uploaded.jpg", contents)
	cloud := fixture.stage(t, IngestSourceCloud, "cloud.jpg", contents)

	type concurrentResult struct {
		assetID uuid.UUID
		err     error
	}
	results := make(chan concurrentResult, 3)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(3)
	go func() {
		ready.Done()
		<-start
		result, processErr := fixture.publisher.Process(fixture.ctx, scanNodeID, scanRevision)
		results <- concurrentResult{assetID: result.AssetID, err: processErr}
	}()
	for _, source := range []IngestSource{upload, cloud} {
		source := source
		go func() {
			ready.Done()
			<-start
			asset, materializeErr := fixture.materializer.MaterializeStaged(fixture.ctx, source)
			result := concurrentResult{err: materializeErr}
			if asset != nil {
				result.assetID = asset.AssetID
			}
			results <- result
		}()
	}
	ready.Wait()
	close(start)
	assetIDs := make(map[uuid.UUID]struct{})
	for range 3 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.assetID == uuid.Nil {
			t.Fatal("concurrent source returned no Asset")
		}
		assetIDs[result.assetID] = struct{}{}
	}
	if len(assetIDs) != 1 {
		t.Fatalf("concurrent source Asset IDs = %v, want one", assetIDs)
	}
	var contentsCount, assetsCount, locationsCount, commitsCount, processingEffects int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM content_objects),
		  (SELECT count(*) FROM assets),
		  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL),
		  (SELECT count(*) FROM repository_staging_commits WHERE status = 'completed'),
		  (SELECT count(*) FROM repository_outbox WHERE effect_kind = 'process_asset')
	`).Scan(&contentsCount, &assetsCount, &locationsCount, &commitsCount, &processingEffects); err != nil {
		t.Fatal(err)
	}
	if contentsCount != 1 || assetsCount != 1 || locationsCount != 3 || commitsCount != 2 || processingEffects != 1 {
		t.Fatalf("convergence contents=%d assets=%d locations=%d commits=%d processing=%d",
			contentsCount, assetsCount, locationsCount, commitsCount, processingEffects)
	}
}

func TestMaterializerRejectsUnclaimedInvalidSource(t *testing.T) {
	materializer := &SourceMaterializer{}
	_, err := materializer.MaterializeStaged(context.Background(), IngestSource{
		Kind: IngestSourceCloud, OriginalFilename: "malware.exe", ContentType: "application/octet-stream",
	})
	if err == nil {
		t.Fatal("invalid source unexpectedly materialized")
	}
	var ownership *StagingMaterializationError
	if !errors.As(err, &ownership) || ownership.Prepared {
		t.Fatalf("staging ownership = %#v, error = %v", ownership, err)
	}
}

func TestStagingHandleRejectsNonPrivatePaths(t *testing.T) {
	repositoryID := uuid.New()
	for _, candidate := range []string{"/tmp/upload.jpg", "../upload.jpg", "inbox/upload.jpg"} {
		if _, err := stagingHandle(repositoryID, candidate, "upload.jpg", time.Now()); err == nil {
			t.Fatalf("accepted invalid staging path %q", candidate)
		}
	}
	valid, err := stagingHandle(
		repositoryID,
		".lumilio/staging/incoming/upload.jpg",
		"upload.jpg",
		time.Now(),
	)
	if err != nil || valid.RepositoryID != repositoryID {
		t.Fatalf("valid staging handle = %+v, %v", valid, err)
	}
}
