package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	"server/internal/utils/hash"

	"github.com/google/uuid"
)

func TestRepositoryFSOpenFailsClosedOnStatusAndIdentity(t *testing.T) {
	t.Parallel()

	repository := createRepositoryFSTestRoot(t)
	factory := NewRepositoryFSFactory(nil, nil)

	offline := repository
	offline.Reachability = dbtypes.RepositoryReachabilityOffline
	if _, err := factory.Open(offline); !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("offline error = %v", err)
	}

	mismatch := repository
	mismatch.RepoID = uuid.New()
	if _, err := factory.Open(mismatch); !errors.Is(err, ErrRepositoryIDMismatch) {
		t.Fatalf("identity error = %v", err)
	}

	missing := repository
	missing.Path = filepath.Join(t.TempDir(), "missing")
	if _, err := factory.Open(missing); !errors.Is(err, ErrRepositoryOffline) {
		t.Fatalf("missing error = %v", err)
	}
}

func TestRepositoryFSWalkIncludesInboxAndProtectsPrivateTree(t *testing.T) {
	t.Parallel()

	repository := createRepositoryFSTestRoot(t)
	writeRepositoryFSTestFile(t, repository.Path, "inbox/2026/08/upload.jpg", []byte("upload"))
	writeRepositoryFSTestFile(t, repository.Path, "Trips/photo.jpg", []byte("trip"))
	writeRepositoryFSTestFile(t, repository.Path, ".hidden/secret.png", []byte("hidden"))
	writeRepositoryFSTestFile(t, repository.Path, ".lumilio/assets/private.jpg", []byte("private"))
	writeRepositoryFSTestFile(t, repository.Path, "notes.txt", []byte("unsupported"))

	repositoryFS := openRepositoryFSTestFS(t, repository)
	summary, err := repositoryFS.WalkUserMedia(context.Background(), WalkOptions{ScanID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Authoritative {
		t.Fatalf("walk unexpectedly partial: %s", summary.PartialReason)
	}
	paths := make(map[string]bool)
	for _, observation := range summary.Observations {
		paths[observation.Path.String()] = true
	}
	for _, want := range []string{"inbox/2026/08/upload.jpg", "Trips/photo.jpg", ".hidden/secret.png"} {
		if !paths[want] {
			t.Errorf("missing observation for %s", want)
		}
	}
	for _, unwanted := range []string{".lumilio/assets/private.jpg", ".lumiliorepo", "notes.txt"} {
		if paths[unwanted] {
			t.Errorf("unexpected observation for %s", unwanted)
		}
	}
}

func TestRepositoryFSWalkStopsAtNestedRepositoryBoundary(t *testing.T) {
	t.Parallel()

	repository := createRepositoryFSTestRoot(t)
	writeRepositoryFSTestFile(t, repository.Path, "outside.jpg", []byte("outside"))
	nestedPath := filepath.Join(repository.Path, "nested")
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedConfig := repocfg.NewRepositoryConfig("Nested")
	if err := nestedConfig.SaveConfigToFile(nestedPath); err != nil {
		t.Fatal(err)
	}
	writeRepositoryFSTestFile(t, repository.Path, "nested/must-not-scan.jpg", []byte("nested"))

	repositoryFS := openRepositoryFSTestFS(t, repository)
	summary, err := repositoryFS.WalkUserMedia(context.Background(), WalkOptions{ScanID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Authoritative || !errors.Is(summary.Issues[0].Err, ErrNestedRepository) {
		t.Fatalf("summary = %#v, want nested-repository topology failure", summary)
	}
	for _, observation := range summary.Observations {
		if observation.Path.String() == "nested/must-not-scan.jpg" {
			t.Fatal("walk crossed nested .lumiliorepo boundary")
		}
	}
}

func TestRepositoryFSSymlinkPolicyAndHardLinkIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires optional Windows privileges")
	}
	t.Parallel()

	repository := createRepositoryFSTestRoot(t)
	writeRepositoryFSTestFile(t, repository.Path, "media/original.jpg", []byte("same bytes"))
	if err := os.Link(
		filepath.Join(repository.Path, "media", "original.jpg"),
		filepath.Join(repository.Path, "media", "hardlink.jpg"),
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("media/original.jpg", filepath.Join(repository.Path, "inside.jpg")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.jpg")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repository.Path, "outside.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("media", filepath.Join(repository.Path, "linked-dir")); err != nil {
		t.Fatal(err)
	}

	repositoryFS := openRepositoryFSTestFS(t, repository)
	summary, err := repositoryFS.WalkUserMedia(context.Background(), WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	observations := make(map[string]FileObservation)
	for _, observation := range summary.Observations {
		observations[observation.Path.String()] = observation
	}
	if observations["inside.jpg"].EntryKind != EntryKindSymlink {
		t.Fatalf("inside symlink kind = %q", observations["inside.jpg"].EntryKind)
	}
	if _, ok := observations["outside.jpg"]; ok {
		t.Fatal("external symlink was indexed")
	}
	if _, ok := observations["linked-dir/original.jpg"]; ok {
		t.Fatal("symlink directory was traversed")
	}
	original := observations["media/original.jpg"]
	hardlink := observations["media/hardlink.jpg"]
	if original.FileIdentity == nil || hardlink.FileIdentity == nil || *original.FileIdentity != *hardlink.FileIdentity {
		t.Fatalf("hard-link identities differ: %v and %v", original.FileIdentity, hardlink.FileIdentity)
	}
}

func TestRepositoryFSInspectHashesOpenedStableFile(t *testing.T) {
	t.Parallel()

	repository := createRepositoryFSTestRoot(t)
	contents := []byte("authoritative original bytes")
	writeRepositoryFSTestFile(t, repository.Path, "inbox/photo.jpg", contents)
	repositoryFS := openRepositoryFSTestFS(t, repository)
	mediaPath, err := ParseUserMediaPath("inbox/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	observation, err := repositoryFS.InspectMedia(context.Background(), mediaPath, HashFull)
	if err != nil {
		t.Fatal(err)
	}
	wantHash, err := hash.CalculateReaderHash(bytes.NewReader(contents), hash.AlgorithmBLAKE3)
	if err != nil {
		t.Fatal(err)
	}
	if observation.ContentHash == nil || *observation.ContentHash != wantHash {
		t.Fatalf("content hash = %v, want %s", observation.ContentHash, wantHash)
	}
	if err := repositoryFS.Revalidate(context.Background(), observation); err != nil {
		t.Fatalf("revalidate unchanged: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repository.Path, "inbox", "photo.jpg"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repositoryFS.Revalidate(context.Background(), observation); !errors.Is(err, ErrRepositoryFileUnstable) {
		t.Fatalf("revalidate changed error = %v", err)
	}
}

func TestRepositoryFSSettleAndLifecycleLease(t *testing.T) {
	t.Parallel()

	repository := createRepositoryFSTestRoot(t)
	writeRepositoryFSTestFile(t, repository.Path, "inbox/recent.jpg", []byte("recent"))
	coordinator := NewRepositoryAccessCoordinator()
	factory := NewRepositoryFSFactory(coordinator, nil)
	repositoryFS, err := factory.Open(repository)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := repositoryFS.WalkUserMedia(context.Background(), WalkOptions{Settle: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Observations) != 0 || len(summary.DeferredPaths) != 1 {
		t.Fatalf("settle observations=%d deferred=%d", len(summary.Observations), len(summary.DeferredPaths))
	}

	acquired := make(chan struct{})
	released := make(chan struct{})
	go func() {
		release := coordinator.AcquireMutation(repository.RepoID)
		close(acquired)
		<-released
		release()
	}()
	select {
	case <-acquired:
		t.Fatal("lifecycle mutation acquired while RepositoryFS was open")
	case <-time.After(25 * time.Millisecond):
	}
	if err := repositoryFS.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("lifecycle mutation did not acquire after RepositoryFS close")
	}
	close(released)
	if _, err := repositoryFS.OpenMedia(mustUserMediaPath(t, "inbox/recent.jpg")); !errors.Is(err, ErrRepositoryFSClosed) {
		t.Fatalf("open after close error = %v", err)
	}
}

func TestRepositoryFSRefreshesCatalogPathAfterLifecycleMutation(t *testing.T) {
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

	stale := createRepositoryFSTestRoot(t)
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rootID := uuid.New()
	rootMarker := rootcfg.New("test root")
	rootMarker.ID = rootID.String()
	if err := rootMarker.Save(filepath.Dir(stale.Path)); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "test root", Path: filepath.Dir(stale.Path),
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	stale, err = catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: stale.RepoID, Name: "relocated", Path: stale.Path, Config: stale.Config,
		Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive,
		Activity: dbtypes.RepositoryActivityIdle, CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		t.Fatal(err)
	}
	newPath := t.TempDir()
	if err := stale.Config.SaveConfigToFile(newPath); err != nil {
		t.Fatal(err)
	}
	writeRepositoryFSTestFile(t, newPath, "Trips/current.jpg", []byte("current"))

	coordinator := NewRepositoryAccessCoordinator()
	release := coordinator.AcquireMutation(stale.RepoID)
	if _, err := catalog.Queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
		RepoID: stale.RepoID, Path: newPath, RootID: rootID,
		Reachability: dbtypes.RepositoryReachabilityActive, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		release()
		t.Fatal(err)
	}
	release()

	repositoryFS, err := NewRepositoryFSFactory(coordinator, catalog.Queries).Open(stale)
	if err != nil {
		t.Fatal(err)
	}
	defer repositoryFS.Close()
	opened, err := repositoryFS.OpenMedia(mustUserMediaPath(t, "Trips/current.jpg"))
	if err != nil {
		t.Fatalf("factory used stale repository path: %v", err)
	}
	_ = opened.Close()
}

func createRepositoryFSTestRoot(t *testing.T) repo.Repository {
	t.Helper()
	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	config := repocfg.NewRepositoryConfig("Repository FS test")
	config.ID = repositoryID.String()
	if err := config.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	return repo.Repository{
		RepoID:       repositoryID,
		Path:         repositoryPath,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle,
		Config:       *config,
	}
}

func openRepositoryFSTestFS(t *testing.T, repository repo.Repository) *RepositoryFS {
	t.Helper()
	repositoryFS, err := NewRepositoryFSFactory(nil, nil).Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repositoryFS.Close() })
	return repositoryFS
}

func writeRepositoryFSTestFile(t *testing.T, repositoryPath, relative string, contents []byte) {
	t.Helper()
	fullPath := filepath.Join(repositoryPath, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustUserMediaPath(t *testing.T, value string) RepositoryPath {
	t.Helper()
	parsed, err := ParseUserMediaPath(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
