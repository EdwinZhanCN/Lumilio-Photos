package roe_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

type catalogFixture struct {
	ctx          context.Context
	database     *db.DB
	repositoryID uuid.UUID
	ownerID      int32
	rootNodeID   uuid.UUID
	now          dbtypes.Timestamp
}

func newCatalogFixture(t *testing.T) *catalogFixture {
	t.Helper()
	ctx := context.Background()
	databaseDirectory := t.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(databaseDirectory, "catalog.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username:           "catalog-owner",
		Password:           "not-used-by-test",
		DisplayName:        "Catalog Owner",
		Role:               "admin",
		WebauthnUserHandle: []byte("catalog-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	repositoryID := uuid.New()
	rootID := uuid.New()
	if _, err := database.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "ROE catalog root", Path: t.TempDir(),
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repositoryConfig := repocfg.NewRepositoryConfig("ROE catalog repository")
	repositoryConfig.ID = repositoryID.String()
	if _, err := database.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "ROE catalog repository", Path: t.TempDir(),
		Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability:   dbtypes.RepositoryReachabilityActive,
		Activity:       dbtypes.RepositoryActivityIdle,
		DefaultOwnerID: &owner.UserID,
		CreatedAt:      now, UpdatedAt: now, RootID: rootID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Queries.EnsureRepositoryObservationState(ctx, repo.EnsureRepositoryObservationStateParams{
		RepositoryID: repositoryID, AdapterKind: "periodic", VolumeKind: "local",
		PathCaseMode: "sensitive", PathNormalization: "nfc",
		CursorHealth: "unavailable", FullVerificationRequired: 1, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	rootNodeID := uuid.New()
	if _, err := database.Queries.InsertRepositoryRootNode(ctx, repo.InsertRepositoryRootNodeParams{
		NodeID: rootNodeID, RepositoryID: repositoryID,
		ObservationRevision: 1, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	return &catalogFixture{
		ctx: ctx, database: database, repositoryID: repositoryID,
		ownerID: owner.UserID, rootNodeID: rootNodeID, now: now,
	}
}

func (f *catalogFixture) upsertFile(t *testing.T, nodeID uuid.UUID, name, nameKey string, revision int64) repo.RepositoryNode {
	t.Helper()
	size := int64(12)
	result, err := f.database.Queries.UpsertRepositoryNodeObservation(f.ctx, repo.UpsertRepositoryNodeObservationParams{
		NodeID: nodeID, RepositoryID: f.repositoryID,
		ParentNodeID: uuid.NullUUID{UUID: f.rootNodeID, Valid: true},
		Name:         name, NameKey: nameKey, Kind: "file",
		ObservationRevision: revision, FileSize: &size, CreatedAt: f.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCatalogRejectsDuplicateActiveNamesAndStaleRevisions(t *testing.T) {
	fixture := newCatalogFixture(t)
	nodeID := uuid.New()
	fixture.upsertFile(t, nodeID, "photo.jpg", "photo.jpg", 4)

	_, err := fixture.database.Queries.UpsertRepositoryNodeObservation(fixture.ctx, repo.UpsertRepositoryNodeObservationParams{
		NodeID: nodeID, RepositoryID: fixture.repositoryID,
		ParentNodeID: uuid.NullUUID{UUID: fixture.rootNodeID, Valid: true},
		Name:         "stale.jpg", NameKey: "stale.jpg", Kind: "file",
		ObservationRevision: 3, CreatedAt: fixture.now,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale revision error = %v, want sql.ErrNoRows", err)
	}
	current, err := fixture.database.Queries.GetRepositoryNode(fixture.ctx, repo.GetRepositoryNodeParams{
		RepositoryID: fixture.repositoryID, NodeID: nodeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if current.Name != "photo.jpg" || current.ObservationRevision != 4 {
		t.Fatalf("stale update changed current node: %+v", current)
	}

	_, err = fixture.database.Queries.UpsertRepositoryNodeObservation(fixture.ctx, repo.UpsertRepositoryNodeObservationParams{
		NodeID: uuid.New(), RepositoryID: fixture.repositoryID,
		ParentNodeID: uuid.NullUUID{UUID: fixture.rootNodeID, Valid: true},
		Name:         "PHOTO.JPG", NameKey: "photo.jpg", Kind: "file",
		ObservationRevision: 5, CreatedAt: fixture.now,
	})
	if err == nil {
		t.Fatal("duplicate active normalized name was accepted")
	}
}

func TestCatalogConcurrentOwnerContentInsertSelectAndLocationConstraints(t *testing.T) {
	fixture := newCatalogFixture(t)
	content, err := fixture.database.Queries.InsertContentObject(fixture.ctx, repo.InsertContentObjectParams{
		ContentID: uuid.New(), HashAlgorithm: "blake3-v1",
		FullHash: "a141f62b558fd2b82e5419123f895e270928c7e912d1b03a2239c5f52d3f2405",
		FileSize: 12, CreatedAt: fixture.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := fixture.database.Queries.InsertContentObject(fixture.ctx, repo.InsertContentObjectParams{
		ContentID: uuid.New(), HashAlgorithm: "blake3-v1", FullHash: content.FullHash,
		FileSize: content.FileSize, CreatedAt: fixture.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ContentID != content.ContentID {
		t.Fatalf("content insert-or-select IDs = %s and %s", content.ContentID, selected.ContentID)
	}

	const contenders = 12
	assets := make(chan repo.Asset, contenders)
	errs := make(chan error, contenders)
	start := make(chan struct{})
	var group sync.WaitGroup
	for range contenders {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			asset, insertErr := fixture.database.Queries.InsertOwnerContentAsset(fixture.ctx, repo.InsertOwnerContentAssetParams{
				AssetID: uuid.New(), OwnerID: &fixture.ownerID, ContentID: content.ContentID,
				Type: "PHOTO", OriginalFilename: "photo.jpg", MimeType: "image/jpeg",
				UploadTime: fixture.now, TakenTime: fixture.now,
				Status:    dbtypes.JSON(`{"state":"processing","message":"Pending processing"}`),
				UpdatedAt: fixture.now,
			})
			if insertErr != nil {
				errs <- insertErr
				return
			}
			assets <- asset
		}()
	}
	close(start)
	group.Wait()
	close(assets)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent insert-or-select failed: %v", err)
	}
	var canonicalAsset uuid.UUID
	for asset := range assets {
		if canonicalAsset == uuid.Nil {
			canonicalAsset = asset.AssetID
		}
		if asset.AssetID != canonicalAsset {
			t.Fatalf("owner/content produced assets %s and %s", canonicalAsset, asset.AssetID)
		}
	}
	var assetCount int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM assets WHERE owner_id = ? AND content_id = ?",
		fixture.ownerID, content.ContentID,
	).Scan(&assetCount); err != nil {
		t.Fatal(err)
	}
	if assetCount != 1 {
		t.Fatalf("owner/content asset count = %d, want 1", assetCount)
	}

	nodeID := uuid.New()
	fixture.upsertFile(t, nodeID, "copy.jpg", "copy.jpg", 6)
	if _, err := fixture.database.Queries.BindAssetLocation(fixture.ctx, repo.BindAssetLocationParams{
		LocationID: uuid.New(), NodeID: nodeID, AssetID: canonicalAsset,
		BoundObservationRevision: 6, CreatedAt: fixture.now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.Queries.BindAssetLocation(fixture.ctx, repo.BindAssetLocationParams{
		LocationID: uuid.New(), NodeID: nodeID, AssetID: canonicalAsset,
		BoundObservationRevision: 7, CreatedAt: fixture.now,
	}); err == nil {
		t.Fatal("second active location for one node was accepted")
	}
}
