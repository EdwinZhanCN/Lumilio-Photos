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

func TestRemovalMutationsRollbackWhenLifecycleAuditInsertFails(t *testing.T) {
	for _, test := range []struct {
		name   string
		action string
		run    func(context.Context, *DefaultRepositoryManager) (string, error)
		check  func(context.Context, *DefaultRepositoryManager, string) error
	}{
		{
			name: "repository", action: "remove_repository",
			run: func(ctx context.Context, manager *DefaultRepositoryManager) (string, error) {
				root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
				if err != nil {
					return "", err
				}
				created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
					RequestID: "audit-remove-create", Actor: "test", Name: "Audit Remove", DirectoryName: "audit-remove",
					Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
				})
				if err != nil {
					return "", err
				}
				return created.Repository.RepoID.String(), manager.RemoveRepository(ctx, created.Repository.RepoID.String(), LifecycleRequest{RequestID: "audit-remove", Actor: "web:admin"})
			},
			check: func(ctx context.Context, manager *DefaultRepositoryManager, id string) error {
				_, err := manager.GetRepository(id)
				return err
			},
		},
		{
			name: "storage location", action: "remove_storage_location",
			run: func(ctx context.Context, manager *DefaultRepositoryManager) (string, error) {
				path := filepath.Join(t.TempDir(), "external")
				if err := os.Mkdir(path, 0o755); err != nil {
					return "", err
				}
				root, err := manager.AddRepositoryRoot(ctx, path, "Audit External")
				if err != nil {
					return "", err
				}
				return root.RootID.String(), manager.DeleteRepositoryRoot(ctx, root.RootID.String(), LifecycleRequest{RequestID: "audit-root-remove", Actor: "web:admin"})
			},
			check: func(ctx context.Context, manager *DefaultRepositoryManager, id string) error {
				_, err := manager.GetRepositoryRoot(ctx, id)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
			if _, err := catalog.SQL.ExecContext(ctx, `
				CREATE TRIGGER reject_target_lifecycle_audit
				BEFORE INSERT ON lifecycle_audit_events
				WHEN NEW.action = '`+test.action+`'
				BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END
			`); err != nil {
				t.Fatal(err)
			}
			id, err := test.run(ctx, manager)
			if err == nil {
				t.Fatal("mutation succeeded despite injected audit failure")
			}
			if checkErr := test.check(ctx, manager, id); checkErr != nil {
				t.Fatalf("catalog mutation was not rolled back: %v", checkErr)
			}
		})
	}
}

func TestHostTerminalStateRollsBackWhenLifecycleAuditInsertFails(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	if _, err := catalog.SQL.ExecContext(ctx, `
		CREATE TRIGGER reject_host_lifecycle_audit
		BEFORE INSERT ON lifecycle_audit_events
		WHEN NEW.action = 'open_repository'
		BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END
	`); err != nil {
		t.Fatal(err)
	}
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-audit-failure", Kind: HostActionOpenRepository, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-audit", filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("host failure became terminal despite rejected audit")
	}
	stored, err := manager.GetHostAction(ctx, action.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != HostActionRunning || stored.CompletedAt != nil {
		t.Fatalf("host terminal state escaped audit transaction: %#v", stored)
	}
	if err := manager.RecoverHostActions(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.GetHostAction(ctx, action.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != HostActionPending {
		t.Fatalf("host audit failure was not recoverable: %#v", recovered)
	}
	var count int
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT count(*) FROM lifecycle_audit_events WHERE request_id = 'host-audit-failure'`).Scan(&count); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("partial host audit rows = %d", count)
	}
}
