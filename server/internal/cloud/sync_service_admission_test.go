package cloud

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestStartRepositoryImportRejectsOfflineDestination(t *testing.T) {
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(storage.NewRepositoryAccessCoordinator(), catalog.Queries)
	manager, err := storage.NewRepositoryManager(catalog.SQL, catalog.Queries, zap.NewNop(), nil, files)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.EnsureDefaultStorageLocation(ctx, filepath.Join(t.TempDir(), "storage"))
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, storage.CreateRepositorySpec{
		RequestID: "cloud-admission-create", Actor: "test", Name: "Cloud target",
		DirectoryName: "cloud-target", Role: dbtypes.RepoRolePrimary, StorageLocationID: root.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpdateRepositoryReachability(ctx, repo.UpdateRepositoryReachabilityParams{
		RepoID: created.Repository.RepoID, Reachability: dbtypes.RepositoryReachabilityOffline,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}

	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "cloud-admission-owner", Password: "test", DisplayName: "Owner", Role: "admin",
		WebauthnUserHandle: []byte("cloud-admission-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialID := uuid.New()
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO cloud_credentials (
			credential_id, provider, display_name, identity_hash, masked_identity,
			owner_id, created_at, updated_at, status
		) VALUES (?, 'icloud', 'Test account', 'admission-identity', 't***@example.com', ?, 1, 1, ?)
	`, credentialID, owner.UserID, CredentialStatusConnected); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertRepositoryCloudBinding(ctx, repo.UpsertRepositoryCloudBindingParams{
		RepositoryID: created.Repository.RepoID, CredentialID: credentialID,
		Provider: "icloud", OwnerID: owner.UserID, RemoteScope: dbtypes.JSON(`{"album":"Favorites"}`),
	}); err != nil {
		t.Fatal(err)
	}

	service := NewCloudSyncService(catalog.Queries, nil, nil, nil, "", t.TempDir(), zap.NewNop())
	_, err = service.StartRepositoryImport(ctx, StartRepositoryImportInput{
		RepositoryID: created.Repository.RepoID,
		CredentialID: credentialID,
		Access:       CredentialAccess{UserID: owner.UserID},
	})
	if !errors.Is(err, storage.ErrRepositoryOffline) {
		t.Fatalf("import error = %v, want ErrRepositoryOffline", err)
	}
}
