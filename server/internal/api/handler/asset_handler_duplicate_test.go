package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/utils/hash"

	"github.com/google/uuid"
)

func duplicateTestRepository(t *testing.T) (*AssetHandler, repo.Repository) {
	t.Helper()
	repositoryPath := t.TempDir()
	repositoryID := uuid.New()
	config := repocfg.NewRepositoryConfig("Test")
	config.ID = repositoryID.String()
	if err := config.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	factory := storage.NewRepositoryFSFactory(nil, nil)
	return &AssetHandler{files: factory}, repo.Repository{RepoID: repositoryID, Path: repositoryPath, Status: dbtypes.RepoStatusActive}
}

func TestVerifyDuplicateAssetFileRequiresMatchingBytes(t *testing.T) {
	handler, repository := duplicateTestRepository(t)
	repositoryPath := repository.Path
	targetPath := filepath.Join(repositoryPath, "inbox", "asset.jpg")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	content := []byte("authoritative media")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	contentHash, err := hash.CalculateBLAKE3(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	storagePath := "inbox/asset.jpg"
	duplicate := &duplicateAsset{storagePath: &storagePath}

	verified, err := handler.verifyDuplicateAssetFile(context.Background(), repository, duplicate, contentHash, int64(len(content)))
	if err != nil || !verified {
		t.Fatalf("matching duplicate = %t/%v", verified, err)
	}
	verified, err = handler.verifyDuplicateAssetFile(context.Background(), repository, duplicate, contentHash, int64(len(content)+1))
	if err != nil || verified {
		t.Fatalf("size-conflicting duplicate = %t/%v", verified, err)
	}

	conflictingPath := filepath.Join(repositoryPath, "conflicting.jpg")
	conflictingContent := append([]byte(nil), content...)
	conflictingContent[0] ^= 0xff
	if err := os.WriteFile(conflictingPath, conflictingContent, 0o600); err != nil {
		t.Fatal(err)
	}
	conflictingHash, err := hash.CalculateBLAKE3(conflictingPath)
	if err != nil {
		t.Fatal(err)
	}
	verified, err = handler.verifyDuplicateAssetFile(context.Background(), repository, duplicate, conflictingHash, int64(len(content)))
	if err != nil || verified {
		t.Fatalf("hash-conflicting duplicate = %t/%v", verified, err)
	}
}

func TestVerifyDuplicateAssetFileRejectsSymlinkEscape(t *testing.T) {
	handler, repository := duplicateTestRepository(t)
	repositoryPath := repository.Path
	outsidePath := filepath.Join(t.TempDir(), "outside.jpg")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(repositoryPath, "outside-link.jpg")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	contentHash, err := hash.CalculateBLAKE3(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	storagePath := "outside-link.jpg"
	verified, err := handler.verifyDuplicateAssetFile(
		context.Background(),
		repository,
		&duplicateAsset{storagePath: &storagePath},
		contentHash,
		int64(len("outside")),
	)
	if err == nil || verified {
		t.Fatalf("escaping duplicate symlink = %t/%v", verified, err)
	}
}
