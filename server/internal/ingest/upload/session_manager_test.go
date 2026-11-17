package upload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/catalog"
	"server/internal/catalog/dbtypes"
	"server/internal/catalog/repo"
	"server/internal/storage"
	"server/internal/storage/layout"
	"server/internal/storage/pathsemantics"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
)

func newPersistentSessionFixture(t *testing.T) (*catalog.DB, repo.Repository, *storage.RepositoryFSFactory, *storage.DefaultStagingManager) {
	t.Helper()
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := catalog.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	repositoryConfig := repocfg.NewRepositoryConfig("upload session")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	storageLocationID := uuid.New()
	rootConfig := rootcfg.New("upload root")
	rootConfig.ID = storageLocationID.String()
	if err := rootConfig.Save(filepath.Dir(repositoryPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertStorageLocation(ctx, repo.UpsertStorageLocationParams{
		StorageLocationID: storageLocationID, Name: "upload root", Path: filepath.Dir(repositoryPath),
		Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	rule, err := pathsemantics.Probe(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "upload session", Path: repositoryPath, Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		CreatedAt: now, UpdatedAt: now, StorageLocationID: storageLocationID,
		PathCase: rule.Case, PathNormalization: rule.Normalization,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, catalog.Queries)
	return catalog, repository, files, storage.NewStagingManager(files)
}

func TestSessionManagerTracksChunks(t *testing.T) {
	manager := NewSessionManager(time.Hour, nil, nil)
	sessionID := uuid.NewString()
	manager.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", uuid.NewString(), "user")
	if !manager.UpdateSessionChunk(sessionID, 0, 5) || !manager.UpdateSessionChunk(sessionID, 0, 5) {
		t.Fatal("failed to record chunk")
	}
	session, ok := manager.GetSession(sessionID)
	if !ok || len(session.ReceivedChunks) != 1 || session.BytesReceived != 5 {
		t.Fatalf("unexpected session: %#v", session)
	}
	if chunks := SessionChunks(session); len(chunks) != 1 || chunks[0].ChunkIndex != 0 || chunks[0].Size != 5 {
		t.Fatalf("chunks = %#v", chunks)
	}
}

func TestSessionManagerProgressAndExpiry(t *testing.T) {
	manager := NewSessionManager(time.Nanosecond, nil, nil)
	session := manager.CreateSession("", "photo.jpg", 5, 1, "image/jpeg", uuid.NewString(), "user")
	if progress, ok := manager.GetSessionProgress(session.SessionID); !ok || progress != 0 {
		t.Fatalf("progress = %v/%v", progress, ok)
	}
	time.Sleep(time.Millisecond)
	if removed := manager.CleanupExpiredSessions(); removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
}

func TestSessionManagerRestoresOnlyExistingChunks(t *testing.T) {
	catalog, repository, files, staging := newPersistentSessionFixture(t)
	sessionID := uuid.New()
	chunk, opened, err := staging.CreateSessionChunk(repository, sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.WriteString("chunk"); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}

	first := NewSessionManager(time.Hour, catalog.Queries, files)
	first.CreateSession(sessionID.String(), "photo.jpg", 10, 2, "image/jpeg", repository.RepoID.String(), "user")
	if !first.UpdateSessionChunk(sessionID.String(), 0, 5) {
		t.Fatal("persist chunk")
	}

	restarted := NewSessionManager(time.Hour, catalog.Queries, files)
	restored := restarted.CreateSession(sessionID.String(), "photo.jpg", 10, 2, "image/jpeg", repository.RepoID.String(), "user")
	if len(restored.ReceivedChunks) != 1 || restored.ReceivedChunks[0] != 0 || restored.BytesReceived != 5 {
		t.Fatalf("restored session = %#v", restored)
	}

	if err := staging.RemoveStagingFile(repository, chunk); err != nil {
		t.Fatal(err)
	}
	restarted = NewSessionManager(time.Hour, catalog.Queries, files)
	restored = restarted.CreateSession(sessionID.String(), "photo.jpg", 10, 2, "image/jpeg", repository.RepoID.String(), "user")
	if len(restored.ReceivedChunks) != 0 || restored.BytesReceived != 0 {
		t.Fatalf("missing chunk was restored: %#v", restored)
	}
}

// A session's directory lives exactly as long as the session: ending it
// removes the manifest and the emptied directory, and expiry removes the
// directory with whatever chunks it still holds.
func TestSessionDirectoryEndsWithTheSession(t *testing.T) {
	catalog, repository, files, staging := newPersistentSessionFixture(t)
	directoryOf := func(id uuid.UUID) string {
		directory, err := layout.UploadSession(id)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(repository.Path, filepath.FromSlash(directory.String()))
	}
	stageChunk := func(manager *SessionManager, id uuid.UUID) *storage.StagingFile {
		chunk, opened, err := staging.CreateSessionChunk(repository, id, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := opened.WriteString("chunk"); err != nil {
			t.Fatal(err)
		}
		if err := opened.Close(); err != nil {
			t.Fatal(err)
		}
		manager.CreateSession(id.String(), "photo.jpg", 5, 1, "image/jpeg", repository.RepoID.String(), "user")
		if !manager.UpdateSessionChunk(id.String(), 0, 5) {
			t.Fatal("persist chunk")
		}
		return chunk
	}

	ended := uuid.New()
	manager := NewSessionManager(time.Hour, catalog.Queries, files)
	chunk := stageChunk(manager, ended)
	if err := staging.RemoveStagingFile(repository, chunk); err != nil {
		t.Fatal(err)
	}
	if !manager.EndSession(ended.String()) {
		t.Fatal("end session")
	}
	if _, err := os.Lstat(directoryOf(ended)); !os.IsNotExist(err) {
		t.Fatalf("ended session directory remains: %v", err)
	}

	expired := uuid.New()
	expiring := NewSessionManager(time.Nanosecond, catalog.Queries, files)
	stageChunk(expiring, expired)
	time.Sleep(time.Millisecond)
	if removed := expiring.CleanupExpiredSessions(); removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	if _, err := os.Lstat(directoryOf(expired)); !os.IsNotExist(err) {
		t.Fatalf("expired session directory remains: %v", err)
	}
}
