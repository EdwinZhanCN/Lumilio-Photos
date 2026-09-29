package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

func TestRemoveExternalStorageLocationPreservesEveryDiskFile(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	externalPath := filepath.Join(base, "external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.AddStorageLocation(ctx, externalPath, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(externalPath, "keep-me.txt")
	if err := os.WriteFile(sentinel, []byte("preserved"), 0o644); err != nil {
		t.Fatal(err)
	}

	impact, err := manager.PreviewStorageLocationRemoval(ctx, storageLocation.StorageLocationID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !impact.CanRemove || !impact.FilesPreserved {
		t.Fatalf("removal impact = %#v", impact)
	}
	if err := manager.DeleteStorageLocation(ctx, storageLocation.StorageLocationID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.GetStorageLocation(ctx, storageLocation.StorageLocationID.String()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("removed storageLocation lookup error = %v, want sql.ErrNoRows", err)
	}
	for _, path := range []string{sentinel, filepath.Join(externalPath, rootcfg.FileName)} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved file %s: %v", path, err)
		}
	}
}

func TestStorageLocationRemovalBlocksDefaultAndRegisteredRepositories(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defaultImpact, err := manager.PreviewStorageLocationRemoval(ctx, defaultStorageLocation.StorageLocationID.String())
	if err != nil {
		t.Fatal(err)
	}
	if defaultImpact.CanRemove || defaultImpact.BlockingReason != "default_storage_location" {
		t.Fatalf("default removal impact = %#v", defaultImpact)
	}
	if err := manager.DeleteStorageLocation(ctx, defaultStorageLocation.StorageLocationID.String()); !errors.Is(err, ErrStorageLocationNotRemovable) {
		t.Fatalf("default removal error = %v", err)
	}

	externalPath := filepath.Join(base, "external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	external, err := manager.AddStorageLocation(ctx, externalPath, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "storageLocation-removal-child", Actor: "test", Name: "Archive",
		DirectoryName: "archive", Role: dbtypes.RepoRoleRegular, StorageLocationID: external.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	impact, err := manager.PreviewStorageLocationRemoval(ctx, external.StorageLocationID.String())
	if err != nil {
		t.Fatal(err)
	}
	if impact.CanRemove || impact.RepositoryCount != 1 || impact.BlockingReason != "registered_repositories" {
		t.Fatalf("in-use removal impact = %#v", impact)
	}
	if err := manager.DeleteStorageLocation(ctx, external.StorageLocationID.String()); !errors.Is(err, ErrStorageLocationInUse) {
		t.Fatalf("in-use removal error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(created.Repository.Path, ".lumiliorepo")); err != nil {
		t.Fatalf("registered repository marker was changed: %v", err)
	}
}

func TestRelocateRepositoryRefusesWhileOriginalIdentityIsOnline(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	makeExternal := func(name string) (string, string) {
		path := filepath.Join(base, name)
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		storageLocation, err := manager.AddStorageLocation(ctx, path, name)
		if err != nil {
			t.Fatal(err)
		}
		return storageLocation.Path, storageLocation.StorageLocationID.String()
	}
	_, originalStorageLocationID := makeExternal("original-storageLocation")
	copyRoot, _ := makeExternal("copy-storageLocation")
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "original-online", Actor: "test", Name: "Original",
		DirectoryName: "original", Role: dbtypes.RepoRoleRegular, StorageLocationID: originalStorageLocationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(copyRoot, "copied")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(created.Repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(target); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(target); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.RelocateRepository(ctx, created.Repository.RepoID.String(), target); !errors.Is(err, ErrRepositoryOriginalOnline) {
		t.Fatalf("relocate error = %v, want ErrRepositoryOriginalOnline", err)
	}
	current, err := manager.GetRepository(created.Repository.RepoID.String())
	if err != nil {
		t.Fatal(err)
	}
	if current.Path != created.Repository.Path {
		t.Fatalf("refused relocation changed path to %s", current.Path)
	}
}

func TestRelocateRepositorySucceedsAcrossRegisteredStorageLocations(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	firstPath := filepath.Join(base, "first-storageLocation")
	secondPath := filepath.Join(base, "second-storageLocation")
	if err := os.Mkdir(firstPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(secondPath, 0o755); err != nil {
		t.Fatal(err)
	}
	firstRoot, err := manager.AddStorageLocation(ctx, firstPath, "First")
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := manager.AddStorageLocation(ctx, secondPath, "Second")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "cross-storageLocation-source", Actor: "test", Name: "Cross Root",
		DirectoryName: "cross-storageLocation", Role: dbtypes.RepoRoleRegular, StorageLocationID: firstRoot.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(secondRoot.Path, "cross-storageLocation")
	if err := os.Rename(created.Repository.Path, newPath); err != nil {
		t.Fatal(err)
	}
	relocated, err := manager.RelocateRepository(ctx, created.Repository.RepoID.String(), newPath, LifecycleRequest{
		RequestID: "cross-storageLocation-relocate", Actor: "test", ConfirmationType: "update_location",
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if relocated.StorageLocationID != secondRoot.StorageLocationID || relocated.Path != canonicalNewPath {
		t.Fatalf("cross-storageLocation relocate = storageLocation %s path %q", relocated.StorageLocationID, relocated.Path)
	}
	marker, err := repocfg.LoadConfigFromFile(relocated.Path)
	if err != nil || marker.ID != created.Repository.RepoID.String() {
		t.Fatalf("relocated marker = %#v, err=%v", marker, err)
	}
}

func TestPrivateStateRollbackHandlesCrashDuringPartialIsolation(t *testing.T) {
	repositoryPath := t.TempDir()
	privateRoot := filepath.Join(repositoryPath, DefaultStructure.SystemDir)
	if err := os.MkdirAll(privateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"derived", "logs"} {
		if err := os.Mkdir(filepath.Join(privateRoot, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(privateRoot, name, "sentinel"), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rollback, err := planRepositoryPrivateStateIsolation(repositoryPath, "partial-crash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rollback.MovedEntries) != 2 {
		t.Fatalf("rollback plan = %#v", rollback)
	}
	if err := os.MkdirAll(rollback.RecoveryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	first := rollback.MovedEntries[0]
	if err := os.Rename(filepath.Join(privateRoot, first), filepath.Join(rollback.RecoveryPath, first)); err != nil {
		t.Fatal(err)
	}

	if err := rollbackRepositoryPrivateStateIsolation(repositoryPath, rollback); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"derived", "logs"} {
		content, err := os.ReadFile(filepath.Join(privateRoot, name, "sentinel"))
		if err != nil || string(content) != name {
			t.Fatalf("restored %s = %q, err = %v", name, content, err)
		}
	}
}
