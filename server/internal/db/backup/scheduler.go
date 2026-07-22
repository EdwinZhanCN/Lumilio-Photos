package backup

import (
	"context"
	"fmt"
	"os"
	"time"

	"server/internal/settings"
)

// Scheduler decides on each periodic tick whether a routine dump is due and,
// if so, runs it and prunes retention. All policy comes from runtime settings
// read at tick time, so schedule changes take effect without re-registering
// the River periodic job.
type Scheduler struct {
	Conn        Conn
	Pool        RowQuerier
	ToolsBinDir string // optional override (desktop bundle); empty = autodetect
	Dir         string // explicit private dump target
	AppVersion  string
	Settings    func(ctx context.Context) (settings.Backup, error)
	Logf        Logf

	// now is a test seam; nil means time.Now.
	now func() time.Time
}

// Run performs one scheduler pass. On a periodic tick (force=false), skips
// (disabled, not yet due, storage unreachable) return nil — only actual
// dump/prune failures are errors, so River retries real failures but never
// "retries" a skip. A forced run (admin "back up now") bypasses the enabled
// and due checks, and an unreachable backup destination becomes an error the
// API can surface instead of a silent skip.
func (s *Scheduler) Run(ctx context.Context, force bool) error {
	logf := s.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	nowFn := s.now
	if nowFn == nil {
		nowFn = time.Now
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

	// The backup destination is explicit and independent from repository mounts.
	// Never redirect it when unavailable: periodic work skips and a forced admin
	// request gets a useful error.
	if _, err := os.Stat(s.Dir); err != nil {
		if force {
			return fmt.Errorf("backup destination %s unreachable: %w", s.Dir, err)
		}
		logf("backup: destination %s unreachable, skipping this run: %v", s.Dir, err)
		return nil
	}

	pgVersion, major, err := ServerVersion(ctx, s.Pool)
	if err != nil {
		return err
	}
	toolsDir, err := LocateTools(s.ToolsBinDir, major)
	if err != nil {
		return err
	}

	if _, err := Dump(ctx, s.Conn, toolsDir, s.Dir, s.AppVersion, pgVersion, logf); err != nil {
		return err
	}
	if _, err := Prune(s.Dir, cfg.KeepLast, logf); err != nil {
		return err
	}
	return nil
}
