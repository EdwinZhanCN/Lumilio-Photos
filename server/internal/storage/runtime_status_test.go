package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/storage/repocfg"
)

func TestStorageRuntimeStatusDegradesOnlyForBootstrapAnchors(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	defaultPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, defaultPath)
	defaultRoot, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	regular, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-ordinary", Actor: "test", Name: "Ordinary",
		DirectoryName: "ordinary", Role: dbtypes.RepoRoleRegular,
		RootID: defaultRoot.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(regular.Repository.Path, ".lumiliorepo")); err != nil {
		t.Fatal(err)
	}
	status, err := manager.StorageRuntimeStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StorageRuntimeStateActive {
		t.Fatalf("ordinary repository failure degraded instance: %+v", status)
	}

	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(primary.Path, ".lumiliorepo")); err != nil {
		t.Fatal(err)
	}
	status, err = manager.StorageRuntimeStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StorageRuntimeStateDegraded || status.Reason != StorageRuntimeReasonRecoveryRequired {
		t.Fatalf("primary failure runtime status = %+v", status)
	}
}

func TestRelocateStorageLocationIsAllOrNothing(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	externalPath := filepath.Join(base, "external-old")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	external, err := manager.AddRepositoryRoot(ctx, externalPath, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	externalPath = external.Path
	first, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "external-first", Actor: "test", Name: "First", DirectoryName: "first",
		Role: dbtypes.RepoRoleRegular, RootID: external.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "external-second", Actor: "test", Name: "Second", DirectoryName: "second",
		Role: dbtypes.RepoRoleRegular, RootID: external.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(base, "external-new")
	if err := os.Rename(externalPath, newPath); err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	secondNewPath := filepath.Join(newPath, "second")
	secondConfig, err := repocfg.LoadConfigFromFile(secondNewPath)
	if err != nil {
		t.Fatal(err)
	}
	secondConfig.ID = "00000000-0000-0000-0000-000000000099"
	if err := secondConfig.SaveConfigToFile(secondNewPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RelocateRepositoryRoot(ctx, external.RootID.String(), newPath); err == nil {
		t.Fatal("relocate with invalid child identity unexpectedly succeeded")
	}
	rootAfterFailure, _ := manager.queries.GetRepositoryRoot(ctx, external.RootID)
	firstAfterFailure, _ := manager.queries.GetRepository(ctx, first.Repository.RepoID)
	secondAfterFailure, _ := manager.queries.GetRepository(ctx, second.Repository.RepoID)
	if rootAfterFailure.Path != externalPath || firstAfterFailure.Path != first.Repository.Path || secondAfterFailure.Path != second.Repository.Path {
		t.Fatalf("failed relocate partially committed: root=%s first=%s second=%s",
			rootAfterFailure.Path, firstAfterFailure.Path, secondAfterFailure.Path)
	}

	secondConfig.ID = second.Repository.RepoID.String()
	if err := secondConfig.SaveConfigToFile(secondNewPath); err != nil {
		t.Fatal(err)
	}
	relocateRequest := LifecycleRequest{RequestID: "relocate-external-stable", Actor: "test"}
	if _, err := manager.RelocateRepositoryRoot(ctx, external.RootID.String(), newPath, relocateRequest); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RelocateRepositoryRoot(ctx, external.RootID.String(), newPath, relocateRequest); err != nil {
		t.Fatalf("stable relocate retry: %v", err)
	}
	var stableOperations int
	if err := manager.database.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations WHERE request_id = ?
	`, relocateRequest.RequestID).Scan(&stableOperations); err != nil {
		t.Fatal(err)
	}
	if stableOperations != 1 {
		t.Fatalf("stable relocate operations = %d, want 1", stableOperations)
	}
	rootAfterSuccess, _ := manager.queries.GetRepositoryRoot(ctx, external.RootID)
	firstAfterSuccess, _ := manager.queries.GetRepository(ctx, first.Repository.RepoID)
	secondAfterSuccess, _ := manager.queries.GetRepository(ctx, second.Repository.RepoID)
	if rootAfterSuccess.Path != canonicalNewPath || firstAfterSuccess.Path != filepath.Join(canonicalNewPath, "first") || secondAfterSuccess.Path != filepath.Join(canonicalNewPath, "second") {
		t.Fatalf("successful relocate paths: root=%s first=%s second=%s",
			rootAfterSuccess.Path, firstAfterSuccess.Path, secondAfterSuccess.Path)
	}
}
