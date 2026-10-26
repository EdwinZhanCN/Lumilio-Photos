package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"server/internal/settings"
)

// Scheduler decides on each River tick whether a routine SQLite snapshot is
// due, then applies count-based retention.
type Scheduler struct {
	Source   *sql.DB
	Dir      string
	Metadata SnapshotMetadata
	Ready    func(ctx context.Context) (bool, error)
	Settings func(ctx context.Context) (settings.Backup, error)
	Logf     Logf

	now func() time.Time
}

// Run performs one scheduler pass. Periodic skips are silent; a forced admin
// request bypasses enabled/due checks and surfaces an unreachable destination.
func (s *Scheduler) Run(ctx context.Context, force bool) error {
	logf := s.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	nowFn := s.now
	if nowFn == nil {
		nowFn = time.Now
	}

	if s.Ready != nil {
		ready, err := s.Ready(ctx)
		if err != nil {
			return fmt.Errorf("check backup readiness: %w", err)
		}
		if !ready {
			logf("backup: first-run setup is incomplete, skipping this run")
			return nil
		}
	}

	cfg, err := s.Settings(ctx)
	if err != nil {
		return fmt.Errorf("load backup settings: %w", err)
	}
	if !force {
		if !cfg.Enabled {
			return nil
		}
		interval := time.Duration(cfg.IntervalHours) * time.Hour
		if interval < time.Hour {
			interval = time.Hour
		}
		if latest, ok := LatestRoutine(s.Dir); ok && nowFn().Sub(latest) < interval {
			return nil
		}
	}

	if err := ensureDestination(s.Dir); err != nil {
		if force {
			return fmt.Errorf("backup destination %s unreachable: %w", s.Dir, err)
		}
		logf("backup: destination %s unreachable, skipping this run: %v", s.Dir, err)
		return nil
	}
	if _, err := CreateSnapshot(ctx, s.Source, s.Dir, "", s.Metadata, logf); err != nil {
		return err
	}
	if _, err := Prune(s.Dir, cfg.KeepLast, logf); err != nil {
		return err
	}
	return nil
}

// ensureDestination accepts an existing destination directory and creates a
// missing one whose parent exists. A fresh install (Desktop app-data or a
// container's app-state volume) has the parent but no backups folder yet, and
// routine snapshots must not wait for the first manual backup to create it;
// this is the same rule CreateSnapshot applies to a forced run. A destination
// whose parent is also missing is an unmounted volume and stays unreachable,
// so no snapshot lands on the wrong disk.
func ensureDestination(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent, parentErr := os.Stat(filepath.Dir(dir))
	if parentErr != nil || !parent.IsDir() {
		return err
	}
	return os.Mkdir(dir, 0o700)
}
