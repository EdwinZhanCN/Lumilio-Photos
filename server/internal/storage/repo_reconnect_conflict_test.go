package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
)

// A reconnect whose destination path another repository row already claims
// fails inside its catalog transaction with ErrRepositoryExistsAtPath: the
// row keeps its path and the journal records the operation as failed. The
// failure is audited after the rollback, because the audit log is a file;
// the Atlas tx-purity rule is what keeps that write out of the transaction.
func TestReconnectOntoAClaimedPathFailsAndKeepsTheRow(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	create := func(requestID, directory string) string {
		t.Helper()
		created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
			RequestID: requestID, Actor: "test", Name: directory, DirectoryName: directory,
			Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return created.Repository.RepoID.String()
	}
	movingID := create("reconnect-conflict-moving", "moving")
	claimingID := create("reconnect-conflict-claiming", "claiming")
	moving, err := manager.GetRepository(movingID)
	if err != nil {
		t.Fatal(err)
	}
	originalPath := moving.Path
	movedPath := filepath.Join(storageLocation.Path, "moved")
	if err := os.Rename(originalPath, movedPath); err != nil {
		t.Fatal(err)
	}
	// The other repository's row claims the destination.
	if _, err := catalog.SQL.ExecContext(ctx, `UPDATE repositories SET path = ? WHERE repo_id = ?`, movedPath, claimingID); err != nil {
		t.Fatal(err)
	}

	_, err = manager.ReconnectRepository(ctx, movingID, movedPath, LifecycleRequest{
		RequestID: "reconnect-conflict", Actor: "web:admin", HostInstanceID: "web-server-1",
		ConfirmationType: "update_location",
	})
	if !errors.Is(err, ErrRepositoryExistsAtPath) {
		t.Fatalf("reconnect onto a claimed path: err = %v, want ErrRepositoryExistsAtPath", err)
	}
	stored, err := manager.GetRepository(movingID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Path != originalPath {
		t.Fatalf("repository path = %q after the conflict, want unchanged %q", stored.Path, originalPath)
	}
	operation, err := manager.queries.GetLifecycleOperationByRequestID(ctx, "reconnect-conflict")
	if err != nil || operation.Status == lifecycleStatusRunning || operation.Status == lifecycleStatusCompleted {
		t.Fatalf("reconnect journal after the conflict = %#v err=%v", operation, err)
	}
}
