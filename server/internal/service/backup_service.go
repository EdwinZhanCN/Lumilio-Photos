package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"server/internal/db/backup"
	"server/internal/queue/jobs"

	"github.com/riverqueue/river"
)

// BackupEntry is one finalized SQLite snapshot with manifest provenance.
type BackupEntry struct {
	Name          string
	SizeBytes     int64
	CreatedAt     time.Time
	AppVersion    string
	SQLiteVersion string
	RestorePoint  bool
}

// BackupService is the admin-facing surface over SQLite snapshots.
type BackupService interface {
	List(ctx context.Context) ([]BackupEntry, error)
	TriggerNow(ctx context.Context) error
	ResolvePath(name string) (string, error)
	Delete(ctx context.Context, name string) error
	// Restore validates and stages a snapshot, then requests a full runtime
	// generation restart. Replacement happens only after all old DB handles close.
	Restore(ctx context.Context, name string) error
}

// BackupRuntime carries restore staging inputs. Routine snapshots are created
// by the River scheduler from the same live database.
type BackupRuntime struct {
	ActivePath     string
	Dir            string
	Metadata       backup.SnapshotMetadata
	Compatibility  backup.Compatibility
	RequestRestart func()
	Logf           backup.Logf
}

type backupService struct {
	rt          BackupRuntime
	queueClient *river.Client[*sql.Tx]
	restore     sync.Mutex
}

func NewBackupService(rt BackupRuntime, queueClient *river.Client[*sql.Tx]) BackupService {
	if rt.Logf == nil {
		rt.Logf = func(string, ...any) {}
	}
	return &backupService{rt: rt, queueClient: queueClient}
}

func (s *backupService) List(_ context.Context) ([]BackupEntry, error) {
	entries, err := os.ReadDir(s.rt.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []BackupEntry{}, nil
		}
		return nil, fmt.Errorf("read backups directory: %w", err)
	}

	out := make([]BackupEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		base, restorePoint := trimRestorePoint(name)
		info, ok := backup.ParseName(base)
		if !ok {
			continue
		}
		manifest, err := backup.ReadManifest(filepath.Join(s.rt.Dir, backup.ManifestName(name)))
		if err != nil {
			continue
		}
		stat, err := entry.Info()
		if err != nil || stat.Size() != manifest.DatabaseSize {
			continue
		}
		out = append(out, BackupEntry{
			Name:          name,
			SizeBytes:     stat.Size(),
			CreatedAt:     info.CreatedAt,
			AppVersion:    manifest.AppVersion,
			SQLiteVersion: manifest.SQLiteVersion,
			RestorePoint:  restorePoint,
		})
	}
	sortBackupEntries(out)
	return out, nil
}

func (s *backupService) TriggerNow(ctx context.Context) error {
	if s.queueClient == nil {
		return errors.New("backup queue is unavailable")
	}
	_, err := s.queueClient.Insert(ctx, jobs.DatabaseBackupArgs{Force: true}, nil)
	return err
}

func (s *backupService) ResolvePath(name string) (string, error) {
	base, _ := trimRestorePoint(name)
	if _, ok := backup.ParseName(base); !ok || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid backup name %q", name)
	}
	return filepath.Join(s.rt.Dir, name), nil
}

func (s *backupService) Delete(_ context.Context, name string) error {
	path, err := s.ResolvePath(name)
	if err != nil {
		return err
	}
	for _, artifact := range []string{path, backup.ManifestPath(path)} {
		if err := os.Remove(artifact); err != nil {
			return fmt.Errorf("delete backup artifact %s: %w", filepath.Base(artifact), err)
		}
	}
	s.rt.Logf("backup: deleted %s and its manifest", name)
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
		return errors.New("another restore is already in progress")
	}
	defer s.restore.Unlock()

	if s.rt.RequestRestart == nil {
		return errors.New("runtime restart hook is unavailable")
	}
	if err := backup.StageRestore(ctx, s.rt.ActivePath, path, s.rt.Metadata, s.rt.Compatibility); err != nil {
		return err
	}
	s.rt.Logf("restore: staged %s; requesting runtime restart", name)
	s.rt.RequestRestart()
	return nil
}

func trimRestorePoint(name string) (string, bool) {
	return strings.CutPrefix(name, backup.RestorePointPrefix)
}

func sortBackupEntries(entries []BackupEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
}
