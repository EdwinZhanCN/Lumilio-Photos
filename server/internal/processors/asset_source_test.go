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
	"server/internal/storage/roe/locations"
	roematerializer "server/internal/storage/roe/materializer"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
)

func TestQueuedAssetSourceFallsThroughToCurrentExactLocation(t *testing.T) {
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

	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "processor-source", Password: "unused", DisplayName: "Processor Source", Role: "admin",
		WebauthnUserHandle: []byte("processor-source"),
	})
	if err != nil {
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
		CreatedAt: now, UpdatedAt: now, RootID: rootID, DefaultOwnerID: &owner.UserID,
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
	materializer := roematerializer.NewHashMaterializer(catalog, files)
	publish := func(eventKey, relative string, observation storage.FileObservation) roematerializer.Result {
		t.Helper()
		result, err := materializer.PublishKnownContent(ctx, roematerializer.KnownContent{
			RepositoryID: repositoryID, OwnerID: owner.UserID, Source: "upload", SourceEventKey: eventKey,
			RelativePath: relative, OriginalFilename: "queued.jpg", MimeType: "image/jpeg", AssetType: "PHOTO",
			FullHash: *observation.ContentHash, FileSize: observation.Size, Observation: observation,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	oldObservation := inspect(oldPath)
	first := publish("old-location", oldPath, oldObservation)
	if err := os.MkdirAll(filepath.Join(repositoryPath, "Trips"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repositoryPath, filepath.FromSlash(oldPath)), filepath.Join(repositoryPath, filepath.FromSlash(newPath))); err != nil {
		t.Fatal(err)
	}
	newObservation := inspect(newPath)
	second := publish("new-location", newPath, newObservation)
	if second.AssetID != first.AssetID || second.ContentID != first.ContentID || second.NewAsset {
		t.Fatalf("exact location did not reuse logical asset: first=%+v second=%+v", first, second)
	}

	processor := &AssetProcessor{queries: catalog.Queries, locationResolver: locations.NewResolver(catalog, files)}
	source, err := processor.resolveCurrentAssetSource(ctx, first.AssetID, first.ContentID)
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
