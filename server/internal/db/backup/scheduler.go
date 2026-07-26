package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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

	if _, err := os.Stat(s.Dir); err != nil {
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
