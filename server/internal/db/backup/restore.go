package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"server/internal/db"
)

const (
	pendingStateStaged  = "staged"
	pendingStateApplied = "applied"
)

// PendingRestore is the durable handoff between the API generation that
// validates a restore request and the next generation that opens the restored
// database. Paths are absolute and written only by this package.
type PendingRestore struct {
	State         string           `json:"state"`
	ActivePath    string           `json:"active_path"`
	SnapshotPath  string           `json:"snapshot_path"`
	RestorePoint  string           `json:"restore_point,omitempty"`
	PreviousPath  string           `json:"previous_path,omitempty"`
	StagedAt      time.Time        `json:"staged_at"`
	Metadata      SnapshotMetadata `json:"metadata"`
	Compatibility Compatibility    `json:"compatibility"`
}

// PendingRestorePath returns the fixed marker adjacent to the active database.
func PendingRestorePath(activePath string) string {
	return activePath + ".pending-restore.json"
}

// StageRestore validates a selected snapshot through an independent read-only
// connection, then writes the durable marker that requests a runtime restart.
// The active database is not touched while handlers still hold its *sql.DB.
func StageRestore(
	ctx context.Context,
	activePath string,
	snapshotPath string,
	metadata SnapshotMetadata,
	compatibility Compatibility,
) error {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return fmt.Errorf("resolve active SQLite path: %w", err)
	}
	snapshotPath, err = filepath.Abs(snapshotPath)
	if err != nil {
		return fmt.Errorf("resolve SQLite snapshot path: %w", err)
	}
	if _, _, err := ValidateSnapshot(ctx, snapshotPath, compatibility); err != nil {
		return fmt.Errorf("reject restore snapshot: %w", err)
	}

	markerPath := PendingRestorePath(activePath)
	if _, err := os.Stat(markerPath); err == nil {
		return errors.New("another restore is already in progress")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect pending restore marker: %w", err)
	}
	marker := PendingRestore{
		State:         pendingStateStaged,
		ActivePath:    filepath.Clean(activePath),
		SnapshotPath:  filepath.Clean(snapshotPath),
		StagedAt:      time.Now().UTC(),
		Metadata:      metadata,
		Compatibility: compatibility,
	}
	if err := writePendingRestore(markerPath, marker, true); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(markerPath))
}

// ApplyPendingRestore runs only between runtime generations, after HTTP and
// River have drained and the old database handle has closed. It creates the
// restore point at that boundary, atomically swaps the main file, and retains
// the previous main file until the new generation declares itself healthy.
func ApplyPendingRestore(ctx context.Context, activePath string, logf Logf) (bool, error) {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if marker.ActivePath != filepath.Clean(activePath) {
		return false, fmt.Errorf("pending restore active path %q does not match %q", marker.ActivePath, activePath)
	}
	if marker.State == pendingStateApplied {
		return true, nil
	}
	if marker.State != pendingStateStaged {
		return false, fmt.Errorf("unknown pending restore state %q", marker.State)
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	if _, _, err := ValidateSnapshot(ctx, marker.SnapshotPath, marker.Compatibility); err != nil {
		return false, fmt.Errorf("revalidate staged restore snapshot: %w", err)
	}
	activeInfo, err := db.InspectCatalog(ctx, activePath)
	if err != nil {
		return false, fmt.Errorf("inspect drained active SQLite catalog: %w", err)
	}
	if activeInfo.LibraryID != marker.Compatibility.LibraryID {
		return false, fmt.Errorf("active library identity changed while restore was staged")
	}

	source, err := sql.Open("sqlite3", activePath)
	if err != nil {
		return false, fmt.Errorf("open drained SQLite catalog for restore point: %w", err)
	}
	source.SetMaxOpenConns(1)
	source.SetMaxIdleConns(1)
	restorePoint, snapshotErr := CreateSnapshot(
		ctx,
		source,
		filepath.Dir(marker.SnapshotPath),
		RestorePointPrefix,
		marker.Metadata,
		logf,
	)
	closeErr := source.Close()
	if snapshotErr != nil {
		return false, fmt.Errorf("create pre-restore snapshot: %w", snapshotErr)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close pre-restore snapshot source: %w", closeErr)
	}

	stagedPath := activePath + ".restore-staged"
	previousPath := activePath + ".restore-previous"
	if err := rejectExistingRestoreArtifact(stagedPath); err != nil {
		return false, err
	}
	if err := rejectExistingRestoreArtifact(previousPath); err != nil {
		return false, err
	}
	if err := copyFile(marker.SnapshotPath, stagedPath, 0o600); err != nil {
		return false, err
	}
	cleanupStaged := true
	defer func() {
		if cleanupStaged {
			_ = os.Remove(stagedPath)
		}
	}()
	stagedInfo, err := db.InspectStandaloneCatalog(ctx, stagedPath)
	if err != nil {
		return false, fmt.Errorf("validate staged active SQLite catalog: %w", err)
	}
	if stagedInfo.LibraryID != marker.Compatibility.LibraryID {
		return false, fmt.Errorf("staged active SQLite catalog has wrong library identity")
	}

	if err := removeSQLiteSidecars(activePath); err != nil {
		return false, err
	}
	if err := os.Rename(activePath, previousPath); err != nil {
		return false, fmt.Errorf("preserve previous SQLite catalog: %w", err)
	}
	if err := os.Rename(stagedPath, activePath); err != nil {
		rollbackErr := os.Rename(previousPath, activePath)
		return false, errors.Join(fmt.Errorf("activate restored SQLite catalog: %w", err), rollbackErr)
	}
	cleanupStaged = false
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		return false, err
	}

	marker.State = pendingStateApplied
	marker.RestorePoint = restorePoint.Path
	marker.PreviousPath = previousPath
	if err := writePendingRestore(PendingRestorePath(activePath), marker, false); err != nil {
		return false, err
	}
	logf("restore: activated %s; previous catalog retained until health verification", filepath.Base(marker.SnapshotPath))
	return true, nil
}

// CompletePendingRestore is called only after the new runtime has migrated and
// passed its health checks. It removes the retained previous main file and the
// marker, while keeping the restore-point snapshot in the backups directory.
func CompletePendingRestore(ctx context.Context, activePath string) error {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if marker.State != pendingStateApplied {
		return fmt.Errorf("cannot complete pending restore in state %q", marker.State)
	}
	if _, err := db.InspectCatalog(ctx, activePath); err != nil {
		return fmt.Errorf("verify restored SQLite catalog before completion: %w", err)
	}
	if marker.PreviousPath != "" {
		if err := os.Remove(marker.PreviousPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove previous SQLite catalog: %w", err)
		}
	}
	if err := removeSQLiteSidecars(marker.PreviousPath); err != nil {
		return err
	}
	if err := os.Remove(PendingRestorePath(activePath)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pending restore marker: %w", err)
	}
	return syncDirectory(filepath.Dir(activePath))
}

// RollbackPendingRestore restores the retained previous main file after the
// restored generation fails startup. It is safe to call when no applied marker
// exists.
func RollbackPendingRestore(ctx context.Context, activePath string, logf Logf) error {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if marker.State != pendingStateApplied {
		return fmt.Errorf("cannot roll back pending restore in state %q", marker.State)
	}
	if marker.PreviousPath == "" {
		return errors.New("pending restore has no retained previous catalog")
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	failedPath := activePath + ".failed-restore"
	if err := rejectExistingRestoreArtifact(failedPath); err != nil {
		return err
	}
	if err := removeSQLiteSidecars(activePath); err != nil {
		return err
	}
	if err := os.Rename(activePath, failedPath); err != nil {
		return fmt.Errorf("preserve failed restored SQLite catalog: %w", err)
	}
	if err := os.Rename(marker.PreviousPath, activePath); err != nil {
		_ = os.Rename(failedPath, activePath)
		return fmt.Errorf("roll back previous SQLite catalog: %w", err)
	}
	if _, err := db.InspectCatalog(ctx, activePath); err != nil {
		_ = os.Rename(activePath, marker.PreviousPath)
		_ = os.Rename(failedPath, activePath)
		return fmt.Errorf("verify rolled-back SQLite catalog: %w", err)
	}
	_ = os.Remove(failedPath)
	if err := os.Remove(PendingRestorePath(activePath)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pending restore marker after rollback: %w", err)
	}
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		return err
	}
	logf("restore: rolled back to previous SQLite catalog; restore point kept at %s", marker.RestorePoint)
	return nil
}

// HasAppliedRestore reports whether startup is currently validating a restored
// generation and therefore should roll back on failure.
func HasAppliedRestore(activePath string) bool {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	return err == nil && marker.State == pendingStateApplied
}

func readPendingRestore(path string) (PendingRestore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PendingRestore{}, err
	}
	var marker PendingRestore
	if err := json.Unmarshal(data, &marker); err != nil {
		return PendingRestore{}, fmt.Errorf("decode pending restore marker: %w", err)
	}
	return marker, nil
}

func writePendingRestore(path string, marker PendingRestore, exclusive bool) error {
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("encode pending restore marker: %w", err)
	}
	data = append(data, '\n')
	tmpPath := path + TmpSuffix
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if exclusive {
		if _, err := os.Stat(path); err == nil {
			return errors.New("another restore is already in progress")
		}
	}
	file, err := os.OpenFile(tmpPath, flag, 0o600)
	if err != nil {
		return fmt.Errorf("create pending restore marker: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write pending restore marker: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync pending restore marker: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close pending restore marker: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalize pending restore marker: %w", err)
	}
	return nil
}

func rejectExistingRestoreArtifact(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("restore artifact already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect restore artifact %s: %w", path, err)
	}
	return nil
}

func copyFile(sourcePath, destinationPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open staged restore source: %w", err)
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create staged restore file: %w", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return fmt.Errorf("copy staged restore file: %w", err)
	}
	if err := destination.Sync(); err != nil {
		_ = destination.Close()
		_ = os.Remove(destinationPath)
		return fmt.Errorf("sync staged restore file: %w", err)
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(destinationPath)
		return fmt.Errorf("close staged restore file: %w", err)
	}
	return nil
}

func removeSQLiteSidecars(databasePath string) error {
	if databasePath == "" {
		return nil
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		path := databasePath + suffix
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove SQLite sidecar %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
