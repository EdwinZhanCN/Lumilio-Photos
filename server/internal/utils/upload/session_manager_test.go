package upload

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSessionManagerRestoresPersistedChunks(t *testing.T) {
	repoPath := t.TempDir()
	sessionID := uuid.NewString()
	chunkPath := filepath.Join(repoPath, ".lumilio", "staging", "incoming", "chunk-0")
	if err := os.MkdirAll(filepath.Dir(chunkPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chunkPath, []byte("chunk"), 0o600); err != nil {
		t.Fatal(err)
	}

	first := NewSessionManager(time.Hour)
	first.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", repoPath, "user")
	if !first.UpdateSessionChunk(sessionID, 0, 5, chunkPath) {
		t.Fatal("failed to persist chunk")
	}

	restarted := NewSessionManager(time.Hour)
	session := restarted.CreateSession(sessionID, "photo.jpg", 10, 2, "image/jpeg", repoPath, "user")
	if len(session.ReceivedChunks) != 1 || session.ReceivedChunks[0] != 0 {
		t.Fatalf("unexpected restored chunks: %v", session.ReceivedChunks)
	}
	if session.BytesReceived != 5 || session.ChunkFiles[0] != chunkPath {
		t.Fatalf("unexpected restored session: %#v", session)
	}
}

func TestSessionManagerDropsMissingPersistedChunk(t *testing.T) {
	repoPath := t.TempDir()
	sessionID := uuid.NewString()
	chunkPath := filepath.Join(repoPath, ".lumilio", "staging", "incoming", "chunk-0")
	if err := os.MkdirAll(filepath.Dir(chunkPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chunkPath, []byte("chunk"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(time.Hour)
	manager.CreateSession(sessionID, "photo.jpg", 5, 1, "image/jpeg", repoPath, "user")
	manager.UpdateSessionChunk(sessionID, 0, 5, chunkPath)
	if err := os.Remove(chunkPath); err != nil {
		t.Fatal(err)
	}

	restarted := NewSessionManager(time.Hour)
	session := restarted.CreateSession(sessionID, "photo.jpg", 5, 1, "image/jpeg", repoPath, "user")
	if len(session.ReceivedChunks) != 0 || session.BytesReceived != 0 {
		t.Fatalf("missing chunk was restored: %#v", session)
	}
}
