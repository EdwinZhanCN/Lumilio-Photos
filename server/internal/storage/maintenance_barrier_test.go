package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

func TestCommittedMaintenanceBarrierBlocksNewRepositoryActivity(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: repository.RepoID, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	visible, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if visible.Reachability != dbtypes.RepositoryReachabilityMaintenance || visible.Activity != dbtypes.RepositoryActivityPaused {
		t.Fatalf("maintenance not externally visible: %+v", visible)
	}
	if _, err := manager.queries.BeginRepositoryActivity(ctx, repo.BeginRepositoryActivityParams{
		RepoID: repository.RepoID, Activity: dbtypes.RepositoryActivityImporting, UpdatedAt: now,
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("new import crossed maintenance barrier: %v", err)
	}
}

func TestRepositoryWorkGateSerializesReprocessAndRemoval(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "work-gate-create", Actor: "test", Name: "Work Gate", DirectoryName: "work-gate",
		Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, release, err := manager.BeginRepositoryWork(ctx, created.Repository.RepoID.String(), dbtypes.RepositoryActivityProcessing)
	if err != nil {
		t.Fatal(err)
	}
	visible, err := manager.queries.GetRepository(ctx, created.Repository.RepoID)
	if err != nil || visible.Activity != dbtypes.RepositoryActivityProcessing {
		t.Fatalf("persistent work gate = %+v, error = %v", visible, err)
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("removal during repository work = %v, want ErrRepositoryBusy", err)
	}
	blockedCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, _, err := manager.BeginRepositoryWork(blockedCtx, created.Repository.RepoID.String(), dbtypes.RepositoryActivityProcessing); !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("concurrent enqueue gate = %v, want ErrRepositoryBusy", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: created.Repository.RepoID, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.BeginRepositoryWork(ctx, created.Repository.RepoID.String(), dbtypes.RepositoryActivityProcessing); !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("enqueue during maintenance = %v, want ErrRepositoryBusy", err)
	}
}

func TestRepositoryEnqueueGateDoesNotWaitForOpenMediaLease(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "enqueue-gate-create", Actor: "test", Name: "Enqueue Gate", DirectoryName: "enqueue-gate",
		Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryFS, err := manager.files.Open(*created.Repository)
	if err != nil {
		t.Fatal(err)
	}
	defer repositoryFS.Close()

	// Reprocess and similar request paths persist only catalog/queue intent.
	// They must not wait for a long-running media reader to close; removal is
	// still fenced by the durable repository activity transition below.
	enqueueCtx, cancelEnqueue := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelEnqueue()
	_, release, err := manager.BeginRepositoryWork(enqueueCtx, created.Repository.RepoID.String(), dbtypes.RepositoryActivityProcessing)
	if err != nil {
		t.Fatalf("enqueue gate waited for an open media lease: %v", err)
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); !errors.Is(err, ErrRepositoryBusy) {
		_ = release()
		t.Fatalf("removal crossed durable enqueue gate: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}

	releaseRootMutation, err := manager.files.AccessCoordinator().AcquireRootMutationContext(ctx, root.RootID)
	if err != nil {
		t.Fatal(err)
	}
	blockedCtx, cancelBlocked := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancelBlocked()
	if _, _, err := manager.BeginRepositoryWork(blockedCtx, created.Repository.RepoID.String(), dbtypes.RepositoryActivityProcessing); !errors.Is(err, ErrRepositoryBusy) {
		releaseRootMutation()
		t.Fatalf("enqueue crossed a Storage Location identity mutation: %v", err)
	}
	releaseRootMutation()
}
