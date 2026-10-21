package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"server/config"
	migrations "server/migrations"
)

// testSteps is a test-only registry that extends the real baseline to
// version 3. No production step exists yet; these prove the runner.
func testSteps(failAt int) []catalogStep {
	create := func(version int, table string) catalogStep {
		return catalogStep{
			Version: version,
			Name:    "create_" + table,
			Apply: func(ctx context.Context, tx *sql.Tx) error {
				if _, err := tx.ExecContext(ctx, "CREATE TABLE "+table+" (id INTEGER PRIMARY KEY) STRICT"); err != nil {
					return err
				}
				if version == failAt {
					return errors.New("injected step failure")
				}
				return nil
			},
		}
	}
	return []catalogStep{create(2, "step_two"), create(3, "step_three")}
}

type recordedBackup struct {
	calls    int
	from, to int
	// tablesAtBackup records whether step tables existed when the backup ran,
	// proving the snapshot precedes the first step.
	stepTwoAtBackup bool
}

func (r *recordedBackup) backup(ctx context.Context, source *sql.DB, from, to int) (string, error) {
	r.calls++
	r.from, r.to = from, to
	r.stepTwoAtBackup = tableExists(ctx, source, "step_two")
	return "/backups/pre-upgrade-test.sqlite3", nil
}

func openCatalogAt(t *testing.T, version int) *DB {
	t.Helper()
	ctx := context.Background()
	database, err := Open(ctx, config.DatabaseConfig{Path: filepath.Join(secureTempDir(t), "steps.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if version == 0 {
		return database
	}
	if err := database.MigrateCatalog(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if version > SchemaVersion {
		recorder := &recordedBackup{}
		if err := database.migrateCatalog(ctx, testSteps(0)[:version-SchemaVersion], version, recorder.backup); err != nil {
			t.Fatal(err)
		}
	}
	return database
}

func tableExists(ctx context.Context, database *sql.DB, name string) bool {
	var count int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?", name).Scan(&count); err != nil {
		panic(err)
	}
	return count == 1
}

func schemaVersionOf(t *testing.T, database *DB) int {
	t.Helper()
	version, err := readSchemaVersion(context.Background(), database.SQL)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func TestMigrateCatalogFreshInstallRunsBaselineThenEveryStepWithoutBackup(t *testing.T) {
	ctx := context.Background()
	database := openCatalogAt(t, 0)
	recorder := &recordedBackup{}
	if err := database.migrateCatalog(ctx, testSteps(0), 3, recorder.backup); err != nil {
		t.Fatal(err)
	}
	if got := schemaVersionOf(t, database); got != 3 {
		t.Fatalf("user_version = %d, want 3", got)
	}
	if !tableExists(ctx, database.SQL, "assets") || !tableExists(ctx, database.SQL, "step_three") {
		t.Fatal("fresh install must run the baseline and every step")
	}
	if recorder.calls != 0 {
		t.Fatalf("fresh install took %d pre-upgrade backups, want 0", recorder.calls)
	}
}

func TestMigrateCatalogBacksUpThenAppliesStepsInOrder(t *testing.T) {
	ctx := context.Background()
	database := openCatalogAt(t, 1)
	recorder := &recordedBackup{}
	if err := database.migrateCatalog(ctx, testSteps(0), 3, recorder.backup); err != nil {
		t.Fatal(err)
	}
	if recorder.calls != 1 || recorder.from != 1 || recorder.to != 3 || recorder.stepTwoAtBackup {
		t.Fatalf("backup = %+v, want one call from 1 to 3 before any step", recorder)
	}
	if got := schemaVersionOf(t, database); got != 3 {
		t.Fatalf("user_version = %d, want 3", got)
	}
	// A current catalog starts without another backup.
	if err := database.migrateCatalog(ctx, testSteps(0), 3, recorder.backup); err != nil {
		t.Fatal(err)
	}
	if recorder.calls != 1 {
		t.Fatalf("current catalog took a backup: %d calls", recorder.calls)
	}
}

func TestMigrateCatalogFailedStepRollsBackAndNamesStepAndBackup(t *testing.T) {
	ctx := context.Background()
	database := openCatalogAt(t, 1)
	recorder := &recordedBackup{}
	err := database.migrateCatalog(ctx, testSteps(3), 3, recorder.backup)
	if err == nil {
		t.Fatal("failed step must block startup")
	}
	for _, want := range []string{"step 3 (create_step_three)", "schema version 2", "/backups/pre-upgrade-test.sqlite3", "injected step failure"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
	if got := schemaVersionOf(t, database); got != 2 {
		t.Fatalf("user_version = %d, want the last completed step 2", got)
	}
	if !tableExists(ctx, database.SQL, "step_two") || tableExists(ctx, database.SQL, "step_three") {
		t.Fatal("the failed step's changes must roll back with its version stamp")
	}
	// A rerun with the fixed step resumes from version 2.
	if err := database.migrateCatalog(ctx, testSteps(0), 3, recorder.backup); err != nil {
		t.Fatal(err)
	}
	if got := schemaVersionOf(t, database); got != 3 {
		t.Fatalf("user_version after resume = %d, want 3", got)
	}
}

func TestMigrateCatalogRefusesUpgradeWithoutBackup(t *testing.T) {
	ctx := context.Background()
	database := openCatalogAt(t, 1)
	err := database.migrateCatalog(ctx, testSteps(0), 3, nil)
	if err == nil || !strings.Contains(err.Error(), "no pre-upgrade backup") {
		t.Fatalf("upgrade without backup error = %v", err)
	}
	if got := schemaVersionOf(t, database); got != 1 {
		t.Fatalf("user_version = %d, want untouched 1", got)
	}
}

func TestMigrateCatalogRejectsNewerThanBuild(t *testing.T) {
	database := openCatalogAt(t, 3)
	err := database.migrateCatalog(context.Background(), testSteps(0)[:1], 2, nil)
	if err == nil || !strings.Contains(err.Error(), "newer than this build supports (2)") {
		t.Fatalf("newer catalog error = %v", err)
	}
}

func TestLoadCatalogStepsRequiresContiguousVersionsMatchingSchemaVersion(t *testing.T) {
	step := func(body string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(body)} }
	for name, tc := range map[string]struct {
		files   fstest.MapFS
		current int
		want    string
	}{
		"gap":            {files: fstest.MapFS{"steps/0003_late.sql": step("SELECT 1;")}, current: 3, want: "without gaps"},
		"stale constant": {files: fstest.MapFS{"steps/0002_add.sql": step("SELECT 1;")}, current: 1, want: "SchemaVersion is 1"},
		"bad name":       {files: fstest.MapFS{"steps/2_add.sql": step("SELECT 1;")}, current: 2, want: "NNNN_<name>.sql"},
		"stamps version": {files: fstest.MapFS{"steps/0002_add.sql": step("PRAGMA user_version = 2;")}, current: 2, want: "must not set user_version"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadCatalogSteps(tc.files, tc.current)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	steps, err := loadCatalogSteps(fstest.MapFS{
		"steps/README.md":       step("rules"),
		"steps/0002_first.sql":  step("SELECT 1;"),
		"steps/0003_second.sql": step("SELECT 1;"),
	}, 3)
	if err != nil || len(steps) != 2 || steps[0].Name != "first" || steps[1].Version != 3 {
		t.Fatalf("steps = %+v, err = %v", steps, err)
	}
}

// TestEmbeddedCatalogStepsMatchSchemaVersion is the production guard: the
// embedded steps must reach exactly SchemaVersion.
func TestEmbeddedCatalogStepsMatchSchemaVersion(t *testing.T) {
	if _, err := loadCatalogSteps(migrations.FS, SchemaVersion); err != nil {
		t.Fatal(err)
	}
}
