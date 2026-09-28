package scan

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/pathsemantics"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	hashutil "server/internal/utils/hash"
)

// recordingWriter measures every catalog write transaction the scanner opens,
// so tests can assert the writer-hold budget from outside the scanner.
type recordingWriter struct {
	database  *db.DB
	mu        sync.Mutex
	durations []time.Duration
}

func (w *recordingWriter) WithTx(ctx context.Context, operation catalogtx.Operation, fn func(*sql.Tx, *repo.Queries) error) error {
	return w.database.WithTx(ctx, operation, func(tx *sql.Tx, queries *repo.Queries) error {
		started := time.Now()
		err := fn(tx, queries)
		w.mu.Lock()
		w.durations = append(w.durations, time.Since(started))
		w.mu.Unlock()
		return err
	})
}

func (w *recordingWriter) reset() {
	w.mu.Lock()
	w.durations = nil
	w.mu.Unlock()
}

func (w *recordingWriter) p99() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.durations) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), w.durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[(len(sorted)*99)/100]
}

// clock is a settable time source; tests that do not set it follow the wall.
type clock struct {
	mu     sync.Mutex
	offset time.Duration
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().UTC().Add(c.offset)
}

func (c *clock) advance(by time.Duration) {
	c.mu.Lock()
	c.offset += by
	c.mu.Unlock()
}

type fixture struct {
	t        testing.TB
	ctx      context.Context
	database *db.DB
	writer   *recordingWriter
	files    *storage.RepositoryFSFactory
	clock    *clock
	scanner  *Scanner
	owner    int32
	rootPath string
	location uuid.UUID
	repos    map[uuid.UUID]string
	primary  repo.Repository
}

func hostSemantics() pathsemantics.Semantics { return pathsemantics.HostDefault() }

func newFixture(tb testing.TB, settle time.Duration) *fixture {
	tb.Helper()
	ctx := context.Background()
	databaseDirectory := tb.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		tb.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(databaseDirectory, "catalog.sqlite3")})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		tb.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "scan-owner", Password: "unused", DisplayName: "Scan Owner",
		Role: "admin", WebauthnUserHandle: []byte("scan-owner-handle"),
	})
	if err != nil {
		tb.Fatal(err)
	}
	rootPath := tb.TempDir()
	location := uuid.New()
	rootConfig := rootcfg.New("scan root")
	rootConfig.ID = location.String()
	if err := rootConfig.Save(rootPath); err != nil {
		tb.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := database.Queries.UpsertStorageLocation(ctx, repo.UpsertStorageLocationParams{
		StorageLocationID: location, Name: "scan root", Path: rootPath,
		Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		tb.Fatal(err)
	}
	f := &fixture{
		t: tb, ctx: ctx, database: database, writer: &recordingWriter{database: database},
		files: storage.NewRepositoryFSFactory(nil, database.Queries), clock: &clock{},
		owner: owner.UserID, rootPath: rootPath, location: location, repos: map[uuid.UUID]string{},
	}
	f.scanner, err = New(database.ReaderQueries, f.writer, f.files, Config{
		Settle: settle, Semantics: hostSemantics(), Now: f.clock.now,
	})
	if err != nil {
		tb.Fatal(err)
	}
	f.primary = f.addRepository("primary")
	return f
}

func (f *fixture) addRepository(name string) repo.Repository {
	f.t.Helper()
	repositoryPath := filepath.Join(f.rootPath, name)
	if err := os.Mkdir(repositoryPath, 0o755); err != nil {
		f.t.Fatal(err)
	}
	repositoryID := uuid.New()
	repositoryConfig := repocfg.NewRepositoryConfig(name)
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		f.t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	repository, err := f.database.Queries.CreateRepository(f.ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: name, Path: repositoryPath,
		Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle, DefaultOwnerID: &f.owner,
		CreatedAt: now, UpdatedAt: now, StorageLocationID: f.location,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	f.repos[repositoryID] = repositoryPath
	return repository
}

func (f *fixture) path(repository repo.Repository, relative string) string {
	return filepath.Join(f.repos[repository.RepoID], filepath.FromSlash(relative))
}

// write creates or replaces a media file with a settled mtime.
func (f *fixture) write(repository repo.Repository, relative, contents string) {
	f.t.Helper()
	target := f.path(repository, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		f.t.Fatal(err)
	}
	settled := f.clock.now().Add(-time.Hour)
	if err := os.Chtimes(target, settled, settled); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) remove(repository repo.Repository, relative string) {
	f.t.Helper()
	if err := os.RemoveAll(f.path(repository, relative)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) rename(from repo.Repository, fromRelative string, to repo.Repository, toRelative string) {
	f.t.Helper()
	target := f.path(to, toRelative)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Rename(f.path(from, fromRelative), target); err != nil {
		f.t.Fatal(err)
	}
}

// scan requests a full scan and turns until it is terminal, returning the
// finished scan row.
func (f *fixture) scan(repository repo.Repository) repo.RepositoryScan {
	f.t.Helper()
	requested, _, err := f.scanner.Request(f.ctx, repository.RepoID, TriggerManual, "", "test", time.Time{})
	if err != nil {
		f.t.Fatal(err)
	}
	f.turnUntilDone(repository, requested.ScanID)
	finished, err := f.database.ReaderQueries.GetRepositoryScan(f.ctx, requested.ScanID)
	if err != nil {
		f.t.Fatal(err)
	}
	return finished
}

// turnUntilDone runs turns until the given scan is terminal; a scan that
// was running before it finishes first.
func (f *fixture) turnUntilDone(repository repo.Repository, scanID uuid.UUID) {
	f.t.Helper()
	for turn := 0; turn < 1_000_000; turn++ {
		current, err := f.database.ReaderQueries.GetRepositoryScan(f.ctx, scanID)
		if err != nil {
			f.t.Fatal(err)
		}
		if current.Status != StatusQueued && current.Status != StatusWalking && current.Status != StatusSweeping {
			return
		}
		result, err := f.scanner.RunTurn(f.ctx, repository.RepoID)
		if err != nil {
			f.t.Fatal(err)
		}
		if result.Status == StatusQueued && !result.NotBefore.IsZero() {
			f.t.Fatalf("scan %s is waiting until %s", result.ScanID, result.NotBefore)
		}
	}
	f.t.Fatal("scan did not finish")
}

// hash drains every pending row and returns how many were bound.
func (f *fixture) hash(repository repo.Repository) int {
	f.t.Helper()
	bound := 0
	for pass := 0; pass < 100_000; pass++ {
		result, err := f.scanner.HashTurn(f.ctx, repository.RepoID, MaxBatchRows)
		if err != nil {
			f.t.Fatal(err)
		}
		bound += result.Bound
		if result.Bound+result.Deferred+result.Errors == 0 {
			return bound
		}
	}
	result, err := f.scanner.HashTurn(f.ctx, repository.RepoID, MaxBatchRows)
	var stuck []string
	for _, row := range f.entries(repository) {
		if row.State == StatePendingHash {
			_, statErr := os.Lstat(f.path(repository, row.Path))
			stuck = append(stuck, row.Path+" stat="+errString(statErr))
		}
	}
	f.t.Fatalf("hashing did not drain: last pass %+v (%v); pending %v", result, err, stuck)
	return bound
}

func (f *fixture) entries(repository repo.Repository) []repo.RepositoryEntry {
	f.t.Helper()
	rows, err := f.database.ReaderSQL.QueryContext(f.ctx, `
		SELECT entry_id, path, path_key, kind, state, revision, asset_id, content_id
		FROM repository_entries WHERE repository_id = ? ORDER BY path, entry_id`, repository.RepoID)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var out []repo.RepositoryEntry
	for rows.Next() {
		var row repo.RepositoryEntry
		if err := rows.Scan(&row.EntryID, &row.Path, &row.PathKey, &row.Kind, &row.State, &row.Revision, &row.AssetID, &row.ContentID); err != nil {
			f.t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		f.t.Fatal(err)
	}
	return out
}

// entry returns the single live (or, failing that, missing) row at a path.
func (f *fixture) entry(repository repo.Repository, relative string) (repo.RepositoryEntry, bool) {
	f.t.Helper()
	var found *repo.RepositoryEntry
	for _, row := range f.entries(repository) {
		if row.Path != relative {
			continue
		}
		if row.State == StatePresent || row.State == StatePendingHash {
			rowCopy := row
			return rowCopy, true
		}
		rowCopy := row
		found = &rowCopy
	}
	if found != nil {
		return *found, true
	}
	return repo.RepositoryEntry{}, false
}

func (f *fixture) mustEntry(repository repo.Repository, relative, state string) repo.RepositoryEntry {
	f.t.Helper()
	row, ok := f.entry(repository, relative)
	if !ok || row.State != state {
		f.t.Fatalf("entry %s = %+v (found %t), want state %s; all: %+v", relative, row, ok, state, f.entries(repository))
	}
	return row
}

func (f *fixture) assetCount() int {
	f.t.Helper()
	var count int
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `SELECT count(*) FROM assets`).Scan(&count); err != nil {
		f.t.Fatal(err)
	}
	return count
}

func (f *fixture) assetContentHash(assetID uuid.UUID) string {
	f.t.Helper()
	var hash string
	if err := f.database.ReaderSQL.QueryRowContext(f.ctx, `
		SELECT content.full_hash FROM assets asset
		JOIN content_objects content ON content.content_id = asset.content_id
		WHERE asset.asset_id = ?`, assetID).Scan(&hash); err != nil {
		f.t.Fatal(err)
	}
	return hash
}

func fileHash(tb testing.TB, filename string) string {
	tb.Helper()
	opened, err := os.Open(filename)
	if err != nil {
		tb.Fatal(err)
	}
	defer opened.Close()
	hash, err := hashutil.CalculateReaderHash(opened, hashutil.AlgorithmBLAKE3)
	if err != nil {
		tb.Fatal(err)
	}
	return hash
}

func errString(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
