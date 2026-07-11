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
	StorageRoot string // media root; unreachable → skip the run with a warning
	Dir         string // dump target, <storage>/backups
	AppVersion  string
	Settings    func(ctx context.Context) (settings.Backup, error)
	Logf        Logf

	// now is a test seam; nil means time.Now.
	now func() time.Time
}

// RunDue performs one scheduler tick. Skips (disabled, not yet due, storage
// unreachable) return nil — only actual dump/prune failures are errors, so
// River retries real failures but never "retries" a skip.
func (s *Scheduler) RunDue(ctx context.Context) error {
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

	// Resolved plan decision: when the media library (e.g. an unplugged
	// external drive) is unreachable, skip with a warning rather than dump to
	// a fallback location the user's media backup would not capture.
	if _, err := os.Stat(s.StorageRoot); err != nil {
		logf("backup: storage root %s unreachable, skipping this run: %v", s.StorageRoot, err)
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
