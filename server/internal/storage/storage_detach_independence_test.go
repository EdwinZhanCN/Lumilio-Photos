package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/dbtypes"
)

func TestRemoveOfflineRepositoryClearsCatalogWithoutDiskPrerequisite(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "offline-detach-create", Actor: "test", Name: "Offline Archive",
		DirectoryName: "offline-archive", Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	originalPath := filepath.Join(created.Repository.Path, DefaultStructure.InboxDir, "kept.jpg")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(originalPath, []byte("offline-original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(created.Repository.Path); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	offline, err := manager.queries.GetRepository(ctx, created.Repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if offline.Reachability != dbtypes.RepositoryReachabilityOffline {
		t.Fatalf("reachability = %q, want offline", offline.Reachability)
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String(), LifecycleRequest{
		RequestID: "offline-detach", Actor: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.GetRepository(ctx, created.Repository.RepoID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("offline detach lookup = %v, want sql.ErrNoRows", err)
	}
}

func TestRemoveOfflineRepositoryClearsStaleActivity(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "offline-stale-activity-create", Actor: "test", Name: "Stale Activity Archive",
		DirectoryName: "stale-activity", Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(created.Repository.Path); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.readerDatabase.ExecContext(ctx, `
		UPDATE repositories SET activity = 'scanning', updated_at = ? WHERE repo_id = ?
	`, now, created.Repository.RepoID); err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.GetRepository(ctx, created.Repository.RepoID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale-activity offline detach lookup = %v, want sql.ErrNoRows", err)
	}
}

func TestRemoveRepositoryIgnoresParentStorageLocationFailure(t *testing.T) {
	for _, failure := range parentStorageLocationFailures {
		t.Run(failure, func(t *testing.T) {
			_, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			rootPath := filepath.Join(t.TempDir(), "default")
			initializeDefaultStorageForTest(t, manager, rootPath)
			defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
			if err != nil {
				t.Fatal(err)
			}
			created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
				RequestID: "detach-parent-" + failure, Actor: "test", Name: "Detach " + failure,
				DirectoryName: "detach-" + failure, Role: dbtypes.RepoRoleRegular,
				StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
			})
			if err != nil {
				t.Fatal(err)
			}
			originalPath := filepath.Join(created.Repository.Path, "parent-fault-original.txt")
			if err := os.WriteFile(originalPath, []byte("preserved"), 0o644); err != nil {
				t.Fatal(err)
			}
			primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			failParentStorageLocationForTest(t, manager, rootPath, primary, failure)

			beforeChecksum, err := sha256FileHex(originalPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String(), LifecycleRequest{
				RequestID: "detach-parent-fault-" + failure, Actor: "test",
			}); err != nil {
				t.Fatalf("detach denied by parent %s: %v", failure, err)
			}
			afterChecksum, err := sha256FileHex(originalPath)
			if err != nil || afterChecksum != beforeChecksum {
				t.Fatalf("original bytes after detach: before %s after %s err=%v",
					beforeChecksum, afterChecksum, err)
			}
		})
	}
}

func TestPreviewRepositoryRemovalSucceedsWhenRepositoryPathIsGone(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "preview-missing-path", Actor: "test", Name: "Missing Path Archive",
		DirectoryName: "missing-path", Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(created.Repository.Path); err != nil {
		t.Fatal(err)
	}
	impact, err := manager.PreviewRepositoryRemoval(ctx, created.Repository.RepoID.String())
	if err != nil {
		t.Fatalf("preview with missing path: %v", err)
	}
	if impact.PrivateStateFound || impact.PrivateStateBytes != 0 {
		t.Fatalf("private state impact = %+v, want absent", impact)
	}
	if impact.RepositoryName != "Missing Path Archive" {
		t.Fatalf("preview impact = %+v", impact)
	}
}

func TestBeginRepositoryWorkIgnoresParentStorageLocationMutationLease(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "begin-work-storageLocation-mutation", Actor: "test", Name: "Sibling Work",
		DirectoryName: "sibling-work", Role: dbtypes.RepoRoleRegular, StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	releaseStorageLocationMutation, err := manager.files.AccessCoordinator().AcquireStorageLocationMutationContext(ctx, defaultStorageLocation.StorageLocationID)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseStorageLocationMutation()
	started, release, err := manager.BeginRepositoryWork(
		ctx, created.Repository.RepoID.String(), dbtypes.RepositoryActivityScanning,
	)
	if err != nil {
		t.Fatalf("BeginRepositoryWork denied by storageLocation mutation lease: %v", err)
	}
	if started.Activity != dbtypes.RepositoryActivityScanning {
		t.Fatalf("started work = %+v", started)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	idle, err := manager.queries.GetRepository(ctx, created.Repository.RepoID)
	if err != nil || idle.Activity != dbtypes.RepositoryActivityIdle {
		t.Fatalf("repository after release = %+v, err = %v", idle, err)
	}
}

func TestDeleteExternalStorageLocationWithMissingPathPreservesFiles(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	externalPath := filepath.Join(base, "vanished-external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.AddStorageLocation(ctx, externalPath, "Vanished Archive")
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(externalPath, "keep-me.txt")
	if err := os.WriteFile(sentinel, []byte("preserved"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(externalPath); err != nil {
		t.Fatal(err)
	}

	impact, err := manager.PreviewStorageLocationRemoval(ctx, storageLocation.StorageLocationID.String())
	if err != nil {
		t.Fatal(err)
	}
	if !impact.CanRemove || !impact.FilesPreserved {
		t.Fatalf("removal impact = %#v", impact)
	}
	if err := manager.DeleteStorageLocation(ctx, storageLocation.StorageLocationID.String(), LifecycleRequest{
		RequestID: "delete-missing-external", Actor: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.GetStorageLocation(ctx, storageLocation.StorageLocationID.String()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("removed storageLocation lookup = %v, want sql.ErrNoRows", err)
	}
	var auditFilesPreserved int
	if err := manager.readerDatabase.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_audit_events
		WHERE request_id = 'delete-missing-external'
		  AND action = 'remove_storage_location'
		  AND json_extract(details, '$.files_preserved') = 1
	`).Scan(&auditFilesPreserved); err != nil {
		t.Fatal(err)
	}
	if auditFilesPreserved != 1 {
		t.Fatalf("files_preserved audit rows = %d, want 1", auditFilesPreserved)
	}
}

func TestDeleteExternalStorageLocationStillBlocksRegisteredChild(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	externalPath := filepath.Join(base, "external-with-child")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.AddStorageLocation(ctx, externalPath, "Child Host")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "external-child", Actor: "test", Name: "Child Repo",
		DirectoryName: "child-repo", Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.DeleteStorageLocation(ctx, storageLocation.StorageLocationID.String()); !errors.Is(err, ErrStorageLocationInUse) {
		t.Fatalf("delete with child = %v, want ErrStorageLocationInUse", err)
	}
}
