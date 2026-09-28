package processors

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/locations"
	"server/internal/storage/pathsemantics"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	"server/internal/storage/scan"

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
	storageLocationID := uuid.New()
	rootConfig := rootcfg.New("processor root")
	rootConfig.ID = storageLocationID.String()
	if err := rootConfig.Save(filepath.Dir(repositoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertStorageLocation(ctx, repo.UpsertStorageLocationParams{
		StorageLocationID: storageLocationID, Name: "processor root", Path: filepath.Dir(repositoryPath),
		Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "moved job", Path: repositoryPath, Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		CreatedAt: now, UpdatedAt: now, StorageLocationID: storageLocationID, DefaultOwnerID: &owner.UserID,
	})
	if err != nil {
		t.Fatal(err)
	}

	oldPath := "inbox/queued.jpg"
	// The new path sorts after the old one, so resolution must fall through
	// the stale entry to the current file.
	newPath := "trips/queued.jpg"
	content := []byte("original media bytes")
	if err := os.MkdirAll(filepath.Join(repositoryPath, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, filepath.FromSlash(oldPath)), content, 0o644); err != nil {
		t.Fatal(err)
	}
	access := storage.NewRepositoryAccessCoordinator()
	files := storage.NewRepositoryFSFactory(access, catalog.Queries)
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
	scanner, err := scan.New(catalog.ReaderQueries, catalog, files, scan.Config{
		Semantics: pathsemantics.HostDefault(),
		Activate: func(ctx context.Context, tx *sql.Tx, queries *repo.Queries, repositoryID, entryID, assetID, contentID uuid.UUID) error {
			return service.ApplyAssetActivationTx(ctx, tx, queries, repositoryID, entryID, assetID, contentID)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	publish := func(relative string, observation storage.FileObservation) scan.KnownBinding {
		t.Helper()
		binding, err := scanner.BindKnownContent(ctx, scan.KnownContent{
			RepositoryID: repositoryID, OwnerID: owner.UserID, RelativePath: relative,
			MimeType: "image/jpeg", AssetType: "PHOTO", FullHash: *observation.ContentHash, Observation: observation,
		})
		if err != nil {
			t.Fatal(err)
		}
		return binding
	}

	oldObservation := inspect(oldPath)
	first := publish(oldPath, oldObservation)
	if err := os.MkdirAll(filepath.Join(repositoryPath, "trips"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repositoryPath, filepath.FromSlash(oldPath)), filepath.Join(repositoryPath, filepath.FromSlash(newPath))); err != nil {
		t.Fatal(err)
	}
	newObservation := inspect(newPath)
	second := publish(newPath, newObservation)
	if second.AssetID != first.AssetID || second.ContentID != first.ContentID || second.EntryID == first.EntryID {
		t.Fatalf("exact location did not reuse logical asset: first=%+v second=%+v", first, second)
	}

	processor := &AssetProcessor{
		reader:           catalog.ReaderQueries,
		readerDatabase:   catalog.ReaderSQL,
		locationResolver: locations.NewResolver(catalog.ReaderQueries, files),
	}
	source, err := processor.resolveCurrentAssetSource(ctx, first.AssetID, first.ContentID)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	work, err := processor.LoadThumbnailTask(ctx, ThumbnailArgs{
		AssetID: first.AssetID, ExpectedContentID: first.ContentID, PipelineVersion: "lease-boundary-v1",
	})
	if err != nil || work == nil {
		t.Fatalf("load thumbnail task: work=%v err=%v", work, err)
	}
	mutationCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	releaseMutation, err := access.AcquireMutationsContext(mutationCtx, []uuid.UUID{repositoryID})
	if err != nil {
		t.Fatalf("thumbnail load retained repository lease across admission boundary: %v", err)
	}
	releaseMutation()

	processor.locationResolver = unavailableLocationResolver{}
	if _, err := processor.LoadThumbnailTask(ctx, ThumbnailArgs{
		AssetID: first.AssetID, ExpectedContentID: first.ContentID, PipelineVersion: "lease-boundary-v1",
	}); !errors.Is(err, locations.ErrAssetUnavailable) {
		t.Fatalf("temporary location outage error = %v, want ErrAssetUnavailable", err)
	}

	// Simulate repository removal winning between the active-occurrence check
	// and capability resolution. The historical delivery must be acknowledged
	// as a no-op instead of retrying forever on ErrAssetUnavailable.
	processor.locationResolver = deleteLocationResolver{database: catalog.SQL}
	work, err = processor.LoadThumbnailTask(ctx, ThumbnailArgs{
		AssetID: first.AssetID, ExpectedContentID: first.ContentID, PipelineVersion: "lease-boundary-v1",
	})
	if err != nil || work != nil {
		t.Fatalf("stale thumbnail delivery: work=%v err=%v", work, err)
	}
}

type deleteLocationResolver struct {
	database *sql.DB
}

type unavailableLocationResolver struct{}

func (unavailableLocationResolver) LocalAssetPath(context.Context, uuid.UUID) (*locations.OpenedMedia, string, error) {
	return nil, "", locations.ErrAssetUnavailable
}

func (r deleteLocationResolver) LocalAssetPath(ctx context.Context, assetID uuid.UUID) (*locations.OpenedMedia, string, error) {
	if _, err := r.database.ExecContext(ctx, `UPDATE repository_entries SET state = 'missing', missing_since = 1 WHERE asset_id = ?`, assetID); err != nil {
		return nil, "", err
	}
	return nil, "", locations.ErrAssetUnavailable
}
