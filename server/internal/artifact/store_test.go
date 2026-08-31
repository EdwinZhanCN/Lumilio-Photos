package artifact

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
)

func TestPublishRequiresContext(t *testing.T) {
	store, files, _ := newTestStore(t)
	defer files.Close()
	if _, err := store.Publish(nil, Identity{"f", "s", "v", "n"}, bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("Publish accepted nil context")
	}
	if _, err := store.RemoveOrphans(nil, nil, time.Minute, time.Now()); err == nil || err.Error() != "artifact cleanup context is nil" {
		t.Fatalf("RemoveOrphans error = %v", err)
	}
}

func TestPublishAdoptsFirstCompleteArtifactForIdentity(t *testing.T) {
	store, files, _ := newTestStore(t)
	defer files.Close()
	id := Identity{SourceFence: uuid.NewString(), Stage: "derivatives", PipelineVersion: "asset-v1", Name: "small.webp"}
	first, err := store.Publish(context.Background(), id, bytes.NewReader([]byte("first")))
	if err != nil {
		t.Fatal(err)
	}
	recomputed := bytes.NewReader([]byte("second"))
	second, err := store.Publish(context.Background(), id, recomputed)
	if err != nil {
		t.Fatal(err)
	}
	if recomputed.Len() != 0 {
		t.Fatalf("retry candidate has %d unread bytes", recomputed.Len())
	}
	if first != second {
		t.Fatalf("adopted artifact = %+v, want %+v", second, first)
	}
	privatePath, err := storage.ParsePrivateRepositoryPath(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := files.ReadPrivateFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "first" {
		t.Fatalf("body = %q", body)
	}
}

func TestIdentityIncludesPipelineVersion(t *testing.T) {
	fence := uuid.NewString()
	first, err := (Identity{SourceFence: fence, Stage: "transcode", PipelineVersion: "v1", Name: "web.mp4"}).Path()
	if err != nil {
		t.Fatal(err)
	}
	second, err := (Identity{SourceFence: fence, Stage: "transcode", PipelineVersion: "v2", Name: "web.mp4"}).Path()
	if err != nil {
		t.Fatal(err)
	}
	if first.String() == second.String() {
		t.Fatal("pipeline versions resolved to the same immutable path")
	}
}

func TestRemoveOrphansHonorsReferencesAndGrace(t *testing.T) {
	store, files, repositoryRoot := newTestStore(t)
	defer files.Close()
	oldID := Identity{uuid.NewString(), "derivatives", "v1", "old.webp"}
	keptID := Identity{uuid.NewString(), "derivatives", "v1", "kept.webp"}
	recentID := Identity{uuid.NewString(), "derivatives", "v1", "recent.webp"}
	old, _ := store.Publish(context.Background(), oldID, bytes.NewReader([]byte("old")))
	kept, _ := store.Publish(context.Background(), keptID, bytes.NewReader([]byte("kept")))
	recent, _ := store.Publish(context.Background(), recentID, bytes.NewReader([]byte("recent")))

	oldTime := time.Now().Add(-2 * time.Hour)
	for _, published := range []Published{old, kept} {
		local := filepath.Join(repositoryRoot, filepath.FromSlash(published.Path))
		if err := os.Chtimes(local, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := store.RemoveOrphans(context.Background(), map[string]struct{}{kept.Path: {}}, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	for path, exists := range map[string]bool{old.Path: false, kept.Path: true, recent.Path: true} {
		privatePath, _ := storage.ParsePrivateRepositoryPath(path)
		_, err := files.StatPrivate(privatePath)
		if exists && err != nil {
			t.Errorf("kept path %s: %v", path, err)
		}
		if !exists && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("orphan path %s still exists: %v", path, err)
		}
	}
}

func newTestStore(t *testing.T) (*Store, *storage.RepositoryFS, string) {
	t.Helper()
	repositoryID := uuid.New()
	repositoryRoot := t.TempDir()
	config := repocfg.NewRepositoryConfig("artifact test")
	config.ID = repositoryID.String()
	if err := config.SaveConfigToFile(repositoryRoot); err != nil {
		t.Fatal(err)
	}
	repository := repo.Repository{RepoID: repositoryID, Path: repositoryRoot, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle, Config: *config}
	files, err := storage.NewRepositoryFSFactory(nil, nil).Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(files)
	if err != nil {
		t.Fatal(err)
	}
	return store, files, repositoryRoot
}
