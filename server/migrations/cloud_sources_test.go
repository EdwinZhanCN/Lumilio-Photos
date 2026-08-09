//go:build sqlite_fts5

package migrations_test

import (
	"context"
	"testing"

	"server/internal/db/sqlitespike"
	"server/migrations"
)

func TestRepositoryCloudBindingsAllowMultipleScopedSources(t *testing.T) {
	ctx := context.Background()
	database, err := sqlitespike.Open(ctx, t.TempDir()+"/cloud-sources.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	baseline, err := migrations.FS.ReadFile("000003_vec1_baseline.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(baseline)); err != nil {
		t.Fatal(err)
	}

	const rootID = "00000000-0000-0000-0000-000000000001"
	const repositoryID = "00000000-0000-0000-0000-000000000002"
	const firstCredential = "00000000-0000-0000-0000-000000000003"
	const secondCredential = "00000000-0000-0000-0000-000000000004"
	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (user_id, username, password, created_at, updated_at, role, webauthn_user_handle)
		VALUES (1, 'owner', 'hash', 1, 1, 'admin', x'01');
		INSERT INTO repository_roots (root_id, name, path, kind, created_at, updated_at)
		VALUES (?, 'Default', '/storage', 'default', 1, 1);
		INSERT INTO repositories (repo_id, name, path, created_at, updated_at, root_id)
		VALUES (?, 'Photos', '/storage/photos', 1, 1, ?);
		INSERT INTO cloud_credentials (
			credential_id, provider, display_name, identity_hash, masked_identity,
			owner_id, created_at, updated_at
		) VALUES
			(?, 'icloud', 'First', 'first', 'f***', 1, 1, 1),
			(?, 'icloud', 'Second', 'second', 's***', 1, 1, 1);
	`, rootID, repositoryID, rootID, firstCredential, secondCredential); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO repository_cloud_bindings (
			repository_id, credential_id, owner_id, provider, remote_scope, created_at, updated_at
		) VALUES
			(?, ?, 1, 'icloud', '{"album":"Favorites"}', 1, 1),
			(?, ?, 1, 'icloud', '{"album":"Videos"}', 2, 2)
	`, repositoryID, firstCredential, repositoryID, secondCredential); err != nil {
		t.Fatalf("bind two iCloud accounts to one repository: %v", err)
	}
	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM repository_cloud_bindings WHERE repository_id = ?
	`, repositoryID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("cloud source count = %d, want 2", count)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE repository_cloud_bindings SET remote_scope = '{bad json'
		WHERE repository_id = ? AND credential_id = ?
	`, repositoryID, firstCredential); err == nil {
		t.Fatal("invalid remote scope JSON was accepted")
	}
}
