package storage

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/catalog/dbtypes"
	"server/internal/catalog/repo"
	"server/internal/storage/layout"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
)

func newStagingTestRepository(t *testing.T, strategy, duplicateMode string) (*DefaultStagingManager, repo.Repository) {
	t.Helper()
	root := t.TempDir()
	if err := NewDirectoryManager().CreateStructure(root); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	config := repocfg.NewRepositoryConfig("Test", repocfg.WithStorageStrategy(strategy), repocfg.WithLocalSettings(duplicateMode))
	config.ID = id.String()
	if err := config.SaveConfigToFile(root); err != nil {
		t.Fatal(err)
	}
	repository := repo.Repository{RepoID: id, Path: root, Name: "Test", Reachability: dbtypes.RepositoryReachabilityActive}
	return NewStagingManager(NewRepositoryFSFactory(nil, nil)), repository
}

func createStagedContent(t *testing.T, manager *DefaultStagingManager, repository repo.Repository, filename, content string) *StagingFile {
	t.Helper()
	staged, opened, err := manager.CreateStagingFile(repository, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	return staged
}

func waitForQueuedMutation(t *testing.T, coordinator *RepositoryAccessCoordinator, id uuid.UUID) {
	t.Helper()
	lock := coordinator.lockFor(id)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		lock.mu.Lock()
		queued := len(lock.waiters) > 0
		lock.mu.Unlock()
		if queued {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("lifecycle mutation never queued")
}

// Merging chunks holds the destination's lease while it reads every chunk. A
// second lease per chunk waited behind a lifecycle mutation queued in between,
// and that mutation waited for the destination: the upload deadlock.
func TestOpenStagingSiblingDoesNotWaitBehindQueuedMutation(t *testing.T) {
	_, repository := newStagingTestRepository(t, "flat", "rename")
	coordinator := NewRepositoryAccessCoordinator()
	manager := NewStagingManager(NewRepositoryFSFactory(coordinator, nil))
	chunk := createStagedContent(t, manager, repository, "chunk-0", "chunk")
	_, destination, err := manager.CreateStagingFile(repository, "merged.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()

	mutated := make(chan struct{})
	go func() {
		release := coordinator.AcquireMutation(repository.RepoID)
		close(mutated)
		release()
	}()
	waitForQueuedMutation(t, coordinator, repository.RepoID)

	opened := make(chan error, 1)
	go func() {
		source, err := destination.OpenStagingSibling(repository, chunk)
		if err == nil {
			err = source.Close()
		}
		opened <- err
	}()
	select {
	case err := <-opened:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("staging sibling waited behind the mutation its own handle blocks")
	}
	if err := destination.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-mutated:
	case <-time.After(time.Second):
		t.Fatal("lifecycle mutation did not acquire after the merge handle closed")
	}
}

func TestStagingCommitUsesRelativeHandleAndNeverReplaces(t *testing.T) {
	manager, repository := newStagingTestRepository(t, "flat", "rename")
	staged := createStagedContent(t, manager, repository, "photo.jpg", "original")
	if staged.RepositoryID != repository.RepoID || !strings.HasPrefix(staged.PrivatePath, layout.UploadsDir().String()+"/") {
		t.Fatalf("unexpected staging handle: %+v", staged)
	}
	if err := manager.CommitStagingFile(repository, staged, "Trips/photo.jpg"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repository.Path, "Trips", "photo.jpg"))
	if err != nil || string(content) != "original" {
		t.Fatalf("committed content = %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(repository.Path, filepath.FromSlash(staged.PrivatePath))); !os.IsNotExist(err) {
		t.Fatalf("staging source remains: %v", err)
	}

	conflict := createStagedContent(t, manager, repository, "photo.jpg", "second")
	if err := manager.CommitStagingFile(repository, conflict, "Trips/photo.jpg"); err == nil {
		t.Fatal("existing original was replaced")
	}
	content, _ = os.ReadFile(filepath.Join(repository.Path, "Trips", "photo.jpg"))
	if string(content) != "original" {
		t.Fatalf("existing original changed: %q", content)
	}
	if _, err := os.Stat(filepath.Join(repository.Path, filepath.FromSlash(conflict.PrivatePath))); err != nil {
		t.Fatalf("conflicting staging evidence removed: %v", err)
	}
}

func TestStagingInboxStrategies(t *testing.T) {
	for _, test := range []struct {
		strategy string
		hash     string
		prefix   string
	}{
		{strategy: "flat", prefix: "inbox/photo.jpg"},
		{strategy: "date", prefix: "inbox/" + time.Now().UTC().Format("2006/01") + "/photo.jpg"},
		{strategy: "cas", hash: "abcdef123456", prefix: "inbox/ab/cd/ef/abcdef123456.jpg"},
	} {
		t.Run(test.strategy, func(t *testing.T) {
			manager, repository := newStagingTestRepository(t, test.strategy, "rename")
			staged := createStagedContent(t, manager, repository, "photo.jpg", test.strategy)
			target, err := manager.CommitStagingFileToInbox(repository, staged, test.hash)
			if err != nil {
				t.Fatal(err)
			}
			if target != test.prefix {
				t.Fatalf("target = %q, want %q", target, test.prefix)
			}
		})
	}
}

func TestStagingDuplicateNameAndQuarantine(t *testing.T) {
	manager, repository := newStagingTestRepository(t, "flat", "rename")
	first := createStagedContent(t, manager, repository, "duplicate.jpg", "first")
	if _, err := manager.CommitStagingFileToInbox(repository, first, ""); err != nil {
		t.Fatal(err)
	}
	second := createStagedContent(t, manager, repository, "duplicate.jpg", "second")
	target, err := manager.CommitStagingFileToInbox(repository, second, "")
	if err != nil {
		t.Fatal(err)
	}
	if target != "inbox/duplicate (1).jpg" {
		t.Fatalf("duplicate target = %q", target)
	}

	failed := createStagedContent(t, manager, repository, "failed.jpg", "evidence")
	if err := manager.MoveStagingToFailed(repository, failed); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(failed.PrivatePath, layout.QuarantineUploadsDir().String()+"/") {
		t.Fatalf("failed path = %q", failed.PrivatePath)
	}
	if content, err := os.ReadFile(onDisk(t, repository.Path, mustPath(layout.Parse(failed.PrivatePath)))); err != nil || string(content) != "evidence" {
		t.Fatalf("quarantined content = %q, %v", content, err)
	}
}

func TestStagingHandleRejectsWrongRepository(t *testing.T) {
	manager, repository := newStagingTestRepository(t, "flat", "rename")
	staged := createStagedContent(t, manager, repository, "photo.jpg", "bytes")
	other := repository
	other.RepoID = uuid.New()
	if _, err := manager.OpenStagingFile(other, staged); err == nil {
		t.Fatal("cross-repository staging handle accepted")
	}
}

// writeSessionFile stages content as one file of an upload session.
func writeSessionFile(t *testing.T, manager *DefaultStagingManager, repository repo.Repository, sessionID uuid.UUID, index int, content string) *StagingFile {
	t.Helper()
	staged, opened, err := manager.CreateSessionChunk(repository, sessionID, index)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	return staged
}

func TestQuarantineSessionKeepsTheWholeSession(t *testing.T) {
	manager, repository := newStagingTestRepository(t, "flat", "rename")
	sessionID := uuid.New()
	writeSessionFile(t, manager, repository, sessionID, 0, "first-bytes")
	writeSessionFile(t, manager, repository, sessionID, 1, "second-bytes")

	if err := manager.QuarantineSession(repository, sessionID); err != nil {
		t.Fatal(err)
	}
	for index, want := range []string{"first-bytes", "second-bytes"} {
		chunk := mustPath(layout.UploadChunk(sessionID, index))
		quarantined := mustPath(layout.QuarantineUploadFile(sessionID, path.Base(chunk.String())))
		if content, err := os.ReadFile(onDisk(t, repository.Path, quarantined)); err != nil || string(content) != want {
			t.Fatalf("quarantined chunk %d = %q, %v", index, content, err)
		}
	}
	if _, err := os.Lstat(onDisk(t, repository.Path, mustPath(layout.UploadSession(sessionID)))); !os.IsNotExist(err) {
		t.Fatalf("work session remains after quarantine: %v", err)
	}
	// Quarantining a session that holds nothing is a no-op.
	if err := manager.QuarantineSession(repository, uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupStagingRemovesOnlyIdleUnclaimedWorkSessions(t *testing.T) {
	manager, repository := newStagingTestRepository(t, "flat", "rename")
	old := time.Now().Add(-2 * time.Hour)
	age := func(staged *StagingFile) {
		t.Helper()
		if err := os.Chtimes(onDisk(t, repository.Path, mustPath(layout.Parse(staged.PrivatePath))), old, old); err != nil {
			t.Fatal(err)
		}
	}
	idle, claimed, fresh, failed := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	age(writeSessionFile(t, manager, repository, idle, 0, "idle"))
	age(writeSessionFile(t, manager, repository, claimed, 0, "claimed"))
	writeSessionFile(t, manager, repository, fresh, 0, "fresh")
	failedFile := writeSessionFile(t, manager, repository, failed, 0, "failed")
	if err := manager.MoveStagingToFailed(repository, failedFile); err != nil {
		t.Fatal(err)
	}
	age(failedFile)

	if err := manager.CleanupStaging(repository, time.Hour, func(id uuid.UUID) bool { return id == claimed }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(onDisk(t, repository.Path, mustPath(layout.UploadSession(idle)))); !os.IsNotExist(err) {
		t.Fatalf("idle unclaimed session remains: %v", err)
	}
	for _, kept := range []layout.Path{
		mustPath(layout.UploadSession(claimed)),
		mustPath(layout.UploadSession(fresh)),
		mustPath(layout.QuarantineUpload(failed)),
	} {
		if _, err := os.Lstat(onDisk(t, repository.Path, kept)); err != nil {
			t.Fatalf("%s was cleaned: %v", kept, err)
		}
	}
}
