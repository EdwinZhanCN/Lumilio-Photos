package upload

import (
	"context"
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

	"github.com/google/uuid"
)

func newPersistentSessionFixture(t *testing.T) (*db.DB, repo.Repository, *storage.RepositoryFSFactory, *storage.DefaultStagingManager) {
	t.Helper()
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
	repositoryConfig := repocfg.NewRepositoryConfig("upload session")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	repository, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "upload session", Path: repositoryPath, Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Status: dbtypes.RepoStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, catalog.Queries)
	return catalog, repository, files, storage.NewStagingManager(files)
}

func TestSessionManagerTracksPrivateChunks(t *testing.T) {
	manager := NewSessionManager(time.Hour, nil, nil)
	sessionID := uuid.NewString()
	manager.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", uuid.NewString(), "user")
	if !manager.UpdateSessionChunk(sessionID, 0, 5, ".lumilio/staging/incoming/chunk-0") {
		t.Fatal("failed to record private chunk")
	}
	if manager.UpdateSessionChunk(sessionID, 1, 5, "/tmp/chunk-1") {
		t.Fatal("accepted absolute chunk path")
	}
	session, ok := manager.GetSession(sessionID)
	if !ok || len(session.ReceivedChunks) != 1 || session.BytesReceived != 5 {
		t.Fatalf("unexpected session: %#v", session)
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

func TestSessionManagerRestoresOnlyExistingPrivateChunks(t *testing.T) {
	catalog, repository, files, staging := newPersistentSessionFixture(t)
	chunk, opened, err := staging.CreateStagingFile(repository, "chunk-0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.WriteString("chunk"); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}

	sessionID := uuid.NewString()
	first := NewSessionManager(time.Hour, catalog.Queries, files)
	first.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", repository.RepoID.String(), "user")
	if !first.UpdateSessionChunk(sessionID, 0, 5, chunk.PrivatePath) {
		t.Fatal("persist chunk")
	}

	restarted := NewSessionManager(time.Hour, catalog.Queries, files)
	restored := restarted.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", repository.RepoID.String(), "user")
	if len(restored.ReceivedChunks) != 1 || restored.ReceivedChunks[0] != 0 || restored.BytesReceived != 5 || restored.ChunkFiles[0] != chunk.PrivatePath {
		t.Fatalf("restored session = %#v", restored)
	}

	if err := staging.RemoveStagingFile(repository, chunk); err != nil {
		t.Fatal(err)
	}
	restarted = NewSessionManager(time.Hour, catalog.Queries, files)
	restored = restarted.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", repository.RepoID.String(), "user")
	if len(restored.ReceivedChunks) != 0 || restored.BytesReceived != 0 {
		t.Fatalf("missing chunk was restored: %#v", restored)
	}
}
