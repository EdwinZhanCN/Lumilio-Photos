package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
)

func TestRepositoryRelocationRollsBackCatalogWhenAuditPersistenceFails(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "relocate-audit-original", Actor: "test", Name: "Archive",
		DirectoryName: "archive", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	originalPath := created.Repository.Path
	movedPath := filepath.Join(root.Path, "archive-moved")
	if err := os.Rename(originalPath, movedPath); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `
		CREATE TRIGGER reject_relocation_audit
		BEFORE INSERT ON lifecycle_audit_events
		WHEN NEW.action = 'update_repository_location'
		BEGIN
			SELECT RAISE(ABORT, 'injected audit failure');
		END
	`); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.RelocateRepository(ctx, created.Repository.RepoID.String(), movedPath, LifecycleRequest{
		RequestID: "relocate-audit-failure", Actor: "web:admin", HostInstanceID: "web-server-1",
		ConfirmationType: "update_location",
	}); err == nil {
		t.Fatal("repository relocation succeeded despite injected audit failure")
	}
	stored, err := manager.GetRepository(created.Repository.RepoID.String())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Path != originalPath {
		t.Fatalf("repository path = %q after audit failure, want unchanged %q", stored.Path, originalPath)
	}
	var audits int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_audit_events
		WHERE request_id = 'relocate-audit-failure'
	`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 0 {
		t.Fatalf("relocation audit rows = %d after rolled-back transaction", audits)
	}
}
