package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
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

func TestStagingCommitUsesRelativeHandleAndNeverReplaces(t *testing.T) {
	manager, repository := newStagingTestRepository(t, "flat", "rename")
	staged := createStagedContent(t, manager, repository, "photo.jpg", "original")
	if staged.RepositoryID != repository.RepoID || !strings.HasPrefix(staged.PrivatePath, DefaultStructure.IncomingDir+"/") {
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
	if !strings.HasPrefix(failed.PrivatePath, DefaultStructure.FailedDir+"/") {
		t.Fatalf("failed path = %q", failed.PrivatePath)
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
