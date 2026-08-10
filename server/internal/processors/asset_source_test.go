package processors

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
)

func TestQueuedAssetSourceFollowsVerifiedMove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	catalogDir := t.TempDir()
	if err := os.Chmod(catalogDir, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	repositoryConfig := repocfg.NewRepositoryConfig("moved job")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rootID := uuid.New()
	rootConfig := rootcfg.New("processor root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(filepath.Dir(repositoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "processor root", Path: filepath.Dir(repositoryPath),
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "moved job", Path: repositoryPath, Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		t.Fatal(err)
	}

	oldPath := "inbox/queued.jpg"
	newPath := "Trips/queued.jpg"
	content := []byte("original media bytes")
	if err := os.MkdirAll(filepath.Join(repositoryPath, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, filepath.FromSlash(oldPath)), content, 0o644); err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, catalog.Queries)
	inspect := func(relative string) storage.FileObservation {
		t.Helper()
		repositoryFS, err := files.Open(repository)
		if err != nil {
			t.Fatal(err)
		}
		defer repositoryFS.Close()
		parsed, err := storage.ParseUserMediaPath(relative)
		if err != nil {
			t.Fatal(err)
		}
		observation, err := repositoryFS.InspectMedia(ctx, parsed, storage.HashFull)
		if err != nil {
			t.Fatal(err)
		}
		return observation
	}
	upsert := func(relative string, assetID uuid.UUID, observation storage.FileObservation) {
		t.Helper()
		_, err := catalog.Queries.UpsertRepositoryFileObservation(ctx, repo.UpsertRepositoryFileObservationParams{
			RepositoryID: repositoryID, StoragePath: relative,
			AssetID: uuid.NullUUID{UUID: assetID, Valid: true}, EntryKind: string(observation.EntryKind),
			FileSize: observation.Size, ModifiedAtNs: observation.ModTimeNS, ChangedAtNs: observation.ChangeTimeNS,
			FileIdentityKind: observation.FileIdentityKind, FileIdentityValue: observation.FileIdentity,
			ObservationToken: observation.ObservationToken, ContentHash: observation.ContentHash,
			State: "present", UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	oldObservation := inspect(oldPath)
	assetID := uuid.New()
	rating := int64(0)
	asset, err := catalog.Queries.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID: assetID, Type: string(dbtypes.AssetTypePhoto), OriginalFilename: "queued.jpg",
		StoragePath: &oldPath, MimeType: "image/jpeg", FileSize: int64(len(content)), ContentHash: *oldObservation.ContentHash,
		TakenTime: now, SpecificMetadata: dbtypes.SpecificMetadata([]byte("{}")), Rating: &rating,
		RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true}, Status: dbtypes.JSON([]byte("{}")),
	})
	if err != nil {
		t.Fatal(err)
	}
	upsert(oldPath, asset.AssetID, oldObservation)

	if err := os.MkdirAll(filepath.Join(repositoryPath, "Trips"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(
		filepath.Join(repositoryPath, filepath.FromSlash(oldPath)),
		filepath.Join(repositoryPath, filepath.FromSlash(newPath)),
	); err != nil {
		t.Fatal(err)
	}
	newObservation := inspect(newPath)
	if _, err := catalog.Queries.MoveAssetWithinRepository(ctx, repo.MoveAssetWithinRepositoryParams{
		StoragePath: &newPath, OriginalFilename: "queued.jpg", AssetID: asset.AssetID,
		RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Queries.DeleteRepositoryFileIndexEntry(ctx, repo.DeleteRepositoryFileIndexEntryParams{
		RepositoryID: repositoryID, StoragePath: oldPath,
	}); err != nil {
		t.Fatal(err)
	}
	upsert(newPath, asset.AssetID, newObservation)

	processor := &AssetProcessor{queries: catalog.Queries, files: files}
	source, err := processor.resolveCurrentAssetSource(ctx, asset.AssetID, oldObservation.ObservationToken, asset.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if source.path.String() != newPath || source.observation.ObservationToken != newObservation.ObservationToken {
		t.Fatalf("resolved source = %s/%s, want %s/%s", source.path.String(), source.observation.ObservationToken, newPath, newObservation.ObservationToken)
	}
	opened, err := source.files.OpenMedia(source.path)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(opened)
	closeErr := opened.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal(readErr, closeErr)
	}
	if string(got) != string(content) {
		t.Fatalf("resolved media = %q, want %q", got, content)
	}
}
