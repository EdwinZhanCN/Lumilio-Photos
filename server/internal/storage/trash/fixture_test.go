package trash

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/pathsemantics"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	"server/internal/storage/scan"
)

// fixture is a catalog, a Storage Location with repositories on disk, the
// scanner that indexes them, and the trash under test.
type fixture struct {
	t        *testing.T
	ctx      context.Context
	database *db.DB
	files    *storage.RepositoryFSFactory
	scanner  *scan.Scanner
	trash    *Service
	owner    int32
	root     string
	location uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	databaseDirectory := t.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(databaseDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "trash-owner", Password: "unused", DisplayName: "Trash Owner",
		Role: "admin", WebauthnUserHandle: []byte("trash-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	location := uuid.New()
	rootConfig := rootcfg.New("trash root")
	rootConfig.ID = location.String()
	if err := rootConfig.Save(root); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := database.Queries.UpsertStorageLocation(ctx, repo.UpsertStorageLocationParams{
		StorageLocationID: location, Name: "trash root", Path: root,
		Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		t: t, ctx: ctx, database: database, files: storage.NewRepositoryFSFactory(nil, database.Queries),
		owner: owner.UserID, root: root, location: location,
	}
	f.scanner, err = scan.New(database.ReaderQueries, database, f.files, scan.Config{Semantics: pathsemantics.HostDefault()})
	if err != nil {
		t.Fatal(err)
	}
	f.trash, err = New(database.ReaderQueries, database, f.files, f.scanner, nil)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) addRepository(name string) repo.Repository {
	f.t.Helper()
	path := filepath.Join(f.root, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		f.t.Fatal(err)
	}
	id := uuid.New()
	repositoryConfig := repocfg.NewRepositoryConfig(name)
	repositoryConfig.ID = id.String()
	if err := repositoryConfig.SaveConfigToFile(path); err != nil {
		f.t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	repository, err := f.database.Queries.CreateRepository(f.ctx, repo.CreateRepositoryParams{
		RepoID: id, Name: name, Path: path, Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		DefaultOwnerID: &f.owner, CreatedAt: now, UpdatedAt: now, StorageLocationID: f.location,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return repository
}

func (f *fixture) path(repository repo.Repository, relative string) string {
	return filepath.Join(repository.Path, filepath.FromSlash(relative))
}

// write creates a media file with a settled mtime.
func (f *fixture) write(repository repo.Repository, relative, contents string) {
	f.t.Helper()
	target := f.path(repository, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		f.t.Fatal(err)
	}
	settled := time.Now().Add(-time.Hour)
	if err := os.Chtimes(target, settled, settled); err != nil {
		f.t.Fatal(err)
	}
}

// index scans and hashes a repository until its files are bound to Assets.
func (f *fixture) index(repository repo.Repository) {
	f.t.Helper()
	requested, _, err := f.scanner.Request(f.ctx, repository.RepoID, scan.TriggerManual, "", "test", time.Time{})
	if err != nil {
		f.t.Fatal(err)
	}
	for turn := 0; ; turn++ {
		current, err := f.database.ReaderQueries.GetRepositoryScan(f.ctx, requested.ScanID)
		if err != nil {
			f.t.Fatal(err)
		}
		if current.Status != scan.StatusQueued && current.Status != scan.StatusWalking && current.Status != scan.StatusSweeping {
			break
		}
		if turn > 10_000 {
			f.t.Fatal("scan did not finish")
		}
		if _, err := f.scanner.RunTurn(f.ctx, repository.RepoID); err != nil {
			f.t.Fatal(err)
		}
	}
	for pass := 0; pass < 10_000; pass++ {
		result, err := f.scanner.HashTurn(f.ctx, repository.RepoID, scan.MaxBatchRows)
		if err != nil {
			f.t.Fatal(err)
		}
		if result.Bound+result.Deferred+result.Errors == 0 {
			return
		}
	}
	f.t.Fatal("hashing did not drain")
}

// assetAt is the Asset bound to the live entry at a path.
func (f *fixture) assetAt(repository repo.Repository, relative string) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT asset_id FROM repository_entries
		WHERE repository_id = ? AND path = ? AND state = 'present'
	`, repository.RepoID, relative).Scan(&id); err != nil {
		f.t.Fatalf("no present entry at %s: %v", relative, err)
	}
	return id
}

func (f *fixture) lifecycleState(assetID uuid.UUID) string {
	f.t.Helper()
	var state string
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT lifecycle_state FROM assets WHERE asset_id = ?`, assetID).Scan(&state); err != nil {
		f.t.Fatal(err)
	}
	return state
}

func (f *fixture) exists(repository repo.Repository, relative string) bool {
	_, err := os.Lstat(f.path(repository, relative))
	return err == nil
}

// entryStates maps each file path of a repository to its entry states.
func (f *fixture) entryStates(repository repo.Repository) map[string][]string {
	f.t.Helper()
	rows, err := f.database.ReaderSQL.QueryContext(f.ctx, `
		SELECT path, state FROM repository_entries
		WHERE repository_id = ? AND kind = 'file' ORDER BY path, state`, repository.RepoID)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	states := map[string][]string{}
	for rows.Next() {
		var path, state string
		if err := rows.Scan(&path, &state); err != nil {
			f.t.Fatal(err)
		}
		states[path] = append(states[path], state)
	}
	return states
}

func (f *fixture) runningOperations() int {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT count(*) FROM lifecycle_operations WHERE status = 'running'`).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count
}

func (f *fixture) request(assetIDs ...uuid.UUID) Request {
	owner := f.owner
	return Request{AssetIDs: assetIDs, Actor: "test", ActorUserID: &owner}
}
