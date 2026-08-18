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
type RestoreOperation = backup.RestoreOperation

var (
	ErrInvalidBackupName = errors.New("invalid backup name")
	ErrRestoreInProgress = backup.ErrRestoreInProgress
)

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
	// Restore validates and stages a snapshot. The handler flushes the accepted
	// operation receipt before calling RestartRestore so the response cannot be
	// lost when the current runtime generation begins draining.
	Restore(ctx context.Context, name string) (backup.RestoreOperation, error)
	RestartRestore(operationID string) error
	GetRestoreOperation(ctx context.Context, operationID string) (backup.RestoreOperation, error)
	LatestRestoreOperation(ctx context.Context) (backup.RestoreOperation, error)
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
		return "", fmt.Errorf("%w: %q", ErrInvalidBackupName, name)
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

func (s *backupService) Restore(ctx context.Context, name string) (backup.RestoreOperation, error) {
	path, err := s.ResolvePath(name)
	if err != nil {
		return backup.RestoreOperation{}, err
	}
	if _, err := os.Stat(path); err != nil {
		return backup.RestoreOperation{}, fmt.Errorf("backup %s: %w", name, err)
	}
	if !s.restore.TryLock() {
		return backup.RestoreOperation{}, ErrRestoreInProgress
	}
	defer s.restore.Unlock()

	if s.rt.RequestRestart == nil {
		return backup.RestoreOperation{}, errors.New("runtime restart hook is unavailable")
	}
	operation, err := backup.StageRestoreTracked(
		ctx,
		s.rt.ActivePath,
		path,
		name,
		s.rt.Metadata,
		s.rt.Compatibility,
	)
	if err != nil {
		return backup.RestoreOperation{}, err
	}
	operation, err = backup.MarkPendingRestoreRestartRequested(s.rt.ActivePath)
	if err != nil {
		cancelErr := backup.CancelStagedRestore(
			s.rt.ActivePath,
			"restore_acceptance_failed",
			"Restore could not be accepted.",
		)
		return backup.RestoreOperation{}, errors.Join(err, cancelErr)
	}
	s.rt.Logf("restore: staged %s as operation %s", name, operation.ID)
	return operation, nil
}

func (s *backupService) RestartRestore(operationID string) error {
	if err := backup.ValidatePendingRestoreOperation(s.rt.ActivePath, operationID); err != nil {
		receiptErr := backup.FailRestoreOperationIfCurrent(
			s.rt.ActivePath,
			operationID,
			"restore_restart_rejected",
			"Runtime restart could not be requested; the active database was not changed.",
		)
		return errors.Join(err, receiptErr)
	}
	if s.rt.RequestRestart == nil {
		err := errors.New("runtime restart hook is unavailable")
		cancelErr := backup.CancelStagedRestore(
			s.rt.ActivePath,
			"restore_restart_unavailable",
			"Runtime restart is unavailable; the active database was not changed.",
		)
		return errors.Join(err, cancelErr)
	}
	s.rt.Logf("restore: requesting runtime restart for operation %s", operationID)
	s.rt.RequestRestart()
	return nil
}

func (s *backupService) GetRestoreOperation(_ context.Context, operationID string) (backup.RestoreOperation, error) {
	return backup.ReadRestoreOperation(s.rt.ActivePath, operationID)
}

func (s *backupService) LatestRestoreOperation(_ context.Context) (backup.RestoreOperation, error) {
	return backup.ReadLatestRestoreOperation(s.rt.ActivePath)
}

func trimRestorePoint(name string) (string, bool) {
	return strings.CutPrefix(name, backup.RestorePointPrefix)
}

func sortBackupEntries(entries []BackupEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
}
