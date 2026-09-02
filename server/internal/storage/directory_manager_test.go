package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryManagerCreatesAndValidatesOnlyRepositoryStructure(t *testing.T) {
	directoryManager := NewDirectoryManager()
	repositoryPath := t.TempDir()
	if err := directoryManager.CreateStructure(repositoryPath); err != nil {
		t.Fatal(err)
	}
	validation, err := directoryManager.ValidateStructure(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || len(validation.MissingDirectories) != 0 {
		t.Fatalf("created repository structure = %+v", validation)
	}
	for _, directory := range repoDirs {
		info, err := os.Stat(filepath.Join(repositoryPath, directory.path))
		if err != nil || !info.IsDir() {
			t.Fatalf("repository directory %s: info=%v error=%v", directory.path, info, err)
		}
	}
	for _, obsolete := range []string{
		".lumilio/assets/thumbnails",
		".lumilio/assets/videos",
		".lumilio/assets/audios",
		".lumilio/temp",
		".lumilio/trash",
		".lumilio/logs/app.log",
	} {
		if _, err := os.Stat(filepath.Join(repositoryPath, obsolete)); !os.IsNotExist(err) {
			t.Fatalf("obsolete repository path %s exists or returned unexpected error: %v", obsolete, err)
		}
	}
}

func TestDirectoryManagerValidationReportsMissingWithoutRepairing(t *testing.T) {
	directoryManager := NewDirectoryManager()
	repositoryPath := t.TempDir()
	validation, err := directoryManager.ValidateStructure(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || len(validation.MissingDirectories) == 0 {
		t.Fatalf("empty repository validation = %+v", validation)
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, DefaultStructure.SystemDir)); !os.IsNotExist(err) {
		t.Fatalf("validation mutated repository structure: %v", err)
	}
}
