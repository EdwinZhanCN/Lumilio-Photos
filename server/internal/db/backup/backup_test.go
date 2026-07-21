package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"server/internal/settings"
)

func touch(t *testing.T, dir, name string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
}

func names(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

func TestPruneKeepsNewestAndRestorePoints(t *testing.T) {
	dir := t.TempDir()
	oldest := FileName(time.Date(2026, 7, 8, 2, 0, 0, 0, time.Local), "v1", "17.5")
	middle := FileName(time.Date(2026, 7, 9, 2, 0, 0, 0, time.Local), "v1", "17.5")
	newest := FileName(time.Date(2026, 7, 10, 2, 0, 0, 0, time.Local), "v1", "17.5")
	restorePoint := RestorePointPrefix + newest

	for _, n := range []string{oldest, middle, newest, restorePoint} {
		touch(t, dir, n, time.Time{})
	}

	removed, err := Prune(dir, 2, t.Logf)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != oldest {
		t.Fatalf("removed = %v, want only %q", removed, oldest)
	}
	left := names(t, dir)
	for _, want := range []string{middle, newest, restorePoint} {
		if !left[want] {
			t.Errorf("%q should have been kept", want)
		}
	}
}

func TestPruneRemovesStaleTmpOnly(t *testing.T) {
	dir := t.TempDir()
	staleTmp := FileName(time.Date(2026, 7, 9, 2, 0, 0, 0, time.Local), "v1", "17.5") + TmpSuffix
	freshTmp := FileName(time.Date(2026, 7, 11, 2, 0, 0, 0, time.Local), "v1", "17.5") + TmpSuffix
	touch(t, dir, staleTmp, time.Now().Add(-2*time.Hour))
	touch(t, dir, freshTmp, time.Now())

	if _, err := Prune(dir, 5, t.Logf); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	left := names(t, dir)
	if left[staleTmp] {
		t.Error("stale .tmp from a failed run should be removed")
	}
	if !left[freshTmp] {
		t.Error("fresh .tmp (possibly in-progress) must be kept")
	}
}

func TestPruneMissingDirIsNoop(t *testing.T) {
	if _, err := Prune(filepath.Join(t.TempDir(), "absent"), 3, t.Logf); err != nil {
		t.Fatalf("Prune on missing dir: %v", err)
	}
}

func TestLatestRoutine(t *testing.T) {
	dir := t.TempDir()
	if _, ok := LatestRoutine(dir); ok {
		t.Fatal("empty dir should report no latest backup")
	}
	newest := time.Date(2026, 7, 10, 2, 0, 0, 0, time.Local)
	touch(t, dir, FileName(newest.Add(-24*time.Hour), "v1", "17.5"), time.Time{})
	touch(t, dir, FileName(newest, "v1", "17.5"), time.Time{})
	touch(t, dir, RestorePointPrefix+FileName(newest.Add(24*time.Hour), "v1", "17.5"), time.Time{})

	got, ok := LatestRoutine(dir)
	if !ok || !got.Equal(newest) {
		t.Fatalf("LatestRoutine = %v/%v, want %v/true (restore points must not count)", got, ok, newest)
	}
}

func TestLocateToolsPrefersOverride(t *testing.T) {
	dir := t.TempDir()
	tool := clientToolName("pg_dump", runtime.GOOS)
	if err := os.WriteFile(filepath.Join(dir, tool), []byte("test fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := LocateTools(dir, 17)
	if err != nil || got != dir {
		t.Fatalf("LocateTools = %q/%v, want %q", got, err, dir)
	}
}

func TestClientToolName(t *testing.T) {
	if got := clientToolName("pg_dump", "windows"); got != "pg_dump.exe" {
		t.Fatalf("windows pg_dump = %q", got)
	}
	if got := clientToolName("psql", "windows"); got != "psql.exe" {
		t.Fatalf("windows psql = %q", got)
	}
	if got := clientToolName("pg_dump", "darwin"); got != "pg_dump" {
		t.Fatalf("darwin pg_dump = %q", got)
	}
}

func TestLocateToolsFailsLoudlyWhenNothingMatches(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // hide any real pg_dump
	_, err := LocateTools("", 99)
	var unsupported *ErrUnsupportedTools
	if !errors.As(err, &unsupported) {
		t.Fatalf("want ErrUnsupportedTools, got %v", err)
	}
}

// schedulerFixture wires a Scheduler whose settings and clock are test-local.
func schedulerFixture(t *testing.T, cfg settings.Backup, now time.Time) (*Scheduler, string) {
	t.Helper()
	storage := t.TempDir()
	s := &Scheduler{
		Dir:        filepath.Join(storage, "backups"),
		AppVersion: "test",
		Settings:   func(context.Context) (settings.Backup, error) { return cfg, nil },
		Logf:       t.Logf,
		now:        func() time.Time { return now },
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return s, storage
}

func TestSchedulerSkipsWhenDisabled(t *testing.T) {
	s, _ := schedulerFixture(t, settings.Backup{Enabled: false}, time.Now())
	// Pool is nil: reaching the version probe would panic, proving the skip.
	if err := s.Run(context.Background(), false); err != nil {
		t.Fatalf("disabled scheduler should be a silent no-op, got %v", err)
	}
}

func TestSchedulerSkipsWhenNotDue(t *testing.T) {
	now := time.Date(2026, 7, 11, 3, 0, 0, 0, time.Local)
	s, _ := schedulerFixture(t, settings.Backup{Enabled: true, IntervalHours: 24, KeepLast: 3}, now)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, s.Dir, FileName(now.Add(-2*time.Hour), "v1", "17.5"), time.Time{})

	if err := s.Run(context.Background(), false); err != nil {
		t.Fatalf("not-due scheduler should be a silent no-op, got %v", err)
	}
}

func TestSchedulerSkipsWhenBackupDestinationUnreachable(t *testing.T) {
	now := time.Now()
	s, storage := schedulerFixture(t, settings.Backup{Enabled: true, IntervalHours: 24, KeepLast: 3}, now)
	if err := os.RemoveAll(storage); err != nil {
		t.Fatal(err)
	}
	// Unreachable storage skips before the version probe (nil Pool would panic).
	if err := s.Run(context.Background(), false); err != nil {
		t.Fatalf("unreachable storage must skip with a warning, got %v", err)
	}
}

func TestSchedulerForcedRunErrorsOnUnreachableBackupDestination(t *testing.T) {
	s, storage := schedulerFixture(t, settings.Backup{Enabled: true, IntervalHours: 24, KeepLast: 3}, time.Now())
	if err := os.RemoveAll(storage); err != nil {
		t.Fatal(err)
	}
	// A forced run is an explicit admin action: surface the problem instead of
	// silently skipping.
	if err := s.Run(context.Background(), true); err == nil {
		t.Fatal("forced run with unreachable storage must return an error")
	}
}

func TestSchedulerForcedRunBypassesDisabledAndDue(t *testing.T) {
	now := time.Date(2026, 7, 11, 3, 0, 0, 0, time.Local)
	s, _ := schedulerFixture(t, settings.Backup{Enabled: false, IntervalHours: 24, KeepLast: 3}, now)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, s.Dir, FileName(now.Add(-time.Minute), "v1", "17.5"), time.Time{})

	// Force proceeds past enabled/due into the version probe; the nil Pool
	// panic proves the checks were bypassed (recovered into a pass).
	defer func() {
		if recover() == nil {
			t.Fatal("forced run should have reached the version probe (nil Pool panic)")
		}
	}()
	_ = s.Run(context.Background(), true)
}
