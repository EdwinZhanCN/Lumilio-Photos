package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"server/internal/db/backup"
	"server/internal/queue/jobs"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// BackupEntry is one dump in the backups directory, described entirely by its
// filename-carried provenance plus filesystem size.
type BackupEntry struct {
	Name         string
	SizeBytes    int64
	CreatedAt    time.Time
	AppVersion   string
	PGVersion    string
	RestorePoint bool
}

// BackupService is the admin-facing surface over the backup engine: list,
// trigger, download, delete, and restore dumps in the app's backups directory.
type BackupService interface {
	List(ctx context.Context) ([]BackupEntry, error)
	// TriggerNow enqueues a forced dump on the db_backup queue and returns
	// immediately; the new dump appears in List when the job finishes.
	TriggerNow(ctx context.Context) error
	// ResolvePath validates name against the backup filename grammar and
	// returns its absolute path — the only sanctioned way to turn API input
	// into a filesystem path (rejects traversal by construction).
	ResolvePath(name string) (string, error)
	Delete(ctx context.Context, name string) error
	// Restore synchronously applies the named dump with a restore point +
	// automatic rollback. Only one restore may run at a time.
	Restore(ctx context.Context, name string) error
}

// BackupRuntime carries the engine inputs the service needs; app.go fills it
// from the boot config and shares Conn/tool resolution with the scheduler.
type BackupRuntime struct {
	Conn        backup.Conn
	Pool        backup.RowQuerier
	ToolsBinDir string
	Dir         string
	AppVersion  string
	Hooks       backup.RestoreHooks
	Logf        backup.Logf
}

type backupService struct {
	rt          BackupRuntime
	queueClient *river.Client[pgx.Tx]
	restore     sync.Mutex
}

// NewBackupService wires the admin backup surface.
func NewBackupService(rt BackupRuntime, queueClient *river.Client[pgx.Tx]) BackupService {
	if rt.Logf == nil {
		rt.Logf = func(string, ...any) {}
	}
	return &backupService{rt: rt, queueClient: queueClient}
}

func (s *backupService) List(ctx context.Context) ([]BackupEntry, error) {
	entries, err := os.ReadDir(s.rt.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupEntry{}, nil
		}
		return nil, fmt.Errorf("read backups dir: %w", err)
	}

	out := make([]BackupEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		base, restorePoint := trimRestorePoint(name)
		info, ok := backup.ParseName(base)
		if !ok {
			continue
		}
		stat, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupEntry{
			Name:         name,
			SizeBytes:    stat.Size(),
			CreatedAt:    info.CreatedAt,
			AppVersion:   info.AppVersion,
			PGVersion:    info.PGVersion,
			RestorePoint: restorePoint,
		})
	}

	// Newest first; filename timestamps sort lexicographically within a prefix
	// class, but sort by parsed time so routine dumps and restore points
	// interleave correctly.
	sortBackupEntries(out)
	return out, nil
}

func (s *backupService) TriggerNow(ctx context.Context) error {
	_, err := s.queueClient.Insert(ctx, jobs.DatabaseBackupArgs{Force: true}, nil)
	return err
}

func (s *backupService) ResolvePath(name string) (string, error) {
	base, _ := trimRestorePoint(name)
	if _, ok := backup.ParseName(base); !ok {
		return "", fmt.Errorf("invalid backup name %q", name)
	}
	return filepath.Join(s.rt.Dir, name), nil
}

func (s *backupService) Delete(ctx context.Context, name string) error {
	path, err := s.ResolvePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete backup %s: %w", name, err)
	}
	s.rt.Logf("backup: deleted %s", name)
	return nil
}

func (s *backupService) Restore(ctx context.Context, name string) error {
	path, err := s.ResolvePath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("backup %s: %w", name, err)
	}

	if !s.restore.TryLock() {
		return fmt.Errorf("another restore is already in progress")
	}
	defer s.restore.Unlock()

	pgVersion, major, err := backup.ServerVersion(ctx, s.rt.Pool)
	if err != nil {
		return err
	}
	toolsDir, err := backup.LocateTools(s.rt.ToolsBinDir, major)
	if err != nil {
		return err
	}

	return backup.RestoreWithRollback(ctx, s.rt.Conn, toolsDir, s.rt.Dir, path, s.rt.AppVersion, pgVersion, s.rt.Hooks, s.rt.Logf)
}

func trimRestorePoint(name string) (string, bool) {
	return strings.CutPrefix(name, backup.RestorePointPrefix)
}

func sortBackupEntries(entries []BackupEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
}
