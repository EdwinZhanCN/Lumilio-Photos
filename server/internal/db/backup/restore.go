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
	"strings"
	"time"

	"server/internal/db"
)

const (
	pendingStateStaged            = "staged"
	pendingStatePreviousPreserved = "previous_preserved"
	pendingStateActiveInstalled   = "active_installed"
	pendingStateVerified          = "verified"
	pendingStateCompleted         = "completed"
	pendingStateRollbackStarted   = "rollback_started"
	pendingStateFailedPreserved   = "failed_preserved"
	pendingStatePreviousRestored  = "previous_restored"

	faultAfterStagedCopy     = "after_staged_copy"
	faultAfterPreviousRename = "after_active_to_previous"
	faultAfterPreviousMarker = "after_previous_preserved_marker"
	faultAfterActiveRename   = "after_staged_to_active"
	faultBeforeActiveMarker  = "before_active_installed_marker"
	faultAfterActiveMarker   = "after_active_installed_marker"
	faultAfterVerifiedMarker = "after_verified_marker"
	faultAfterFailedRename   = "after_active_to_failed"
	faultAfterFailedMarker   = "after_failed_preserved_marker"
	faultAfterRollbackRename = "after_previous_to_active"
	faultAfterRollbackMarker = "after_previous_restored_marker"
)

type restoreFault func(point string) error

// PendingRestore is the durable journal shared by the API generation that
// stages a restore and every later generation that reconciles its file swap.
// Paths are absolute and written only by this package.
type PendingRestore struct {
	State          string           `json:"state"`
	OperationID    string           `json:"operation_id,omitempty"`
	BackupName     string           `json:"backup_name,omitempty"`
	FailureCode    string           `json:"failure_code,omitempty"`
	FailureMessage string           `json:"failure_message,omitempty"`
	ActivePath     string           `json:"active_path"`
	SnapshotPath   string           `json:"snapshot_path"`
	StagedPath     string           `json:"staged_path"`
	PreviousPath   string           `json:"previous_path"`
	FailedPath     string           `json:"failed_path"`
	RestorePoint   string           `json:"restore_point,omitempty"`
	StagedAt       time.Time        `json:"staged_at"`
	Metadata       SnapshotMetadata `json:"metadata"`
	Compatibility  Compatibility    `json:"compatibility"`
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
	_, err := StageRestoreTracked(
		ctx,
		activePath,
		snapshotPath,
		filepath.Base(snapshotPath),
		metadata,
		compatibility,
	)
	return err
}

// StageRestoreTracked performs the same validation as StageRestore and creates
// a durable observation record before writing the cross-generation marker.
func StageRestoreTracked(
	ctx context.Context,
	activePath string,
	snapshotPath string,
	backupName string,
	metadata SnapshotMetadata,
	compatibility Compatibility,
) (RestoreOperation, error) {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return RestoreOperation{}, fmt.Errorf("resolve active SQLite path: %w", err)
	}
	activePath = filepath.Clean(activePath)
	snapshotPath, err = filepath.Abs(snapshotPath)
	if err != nil {
		return RestoreOperation{}, fmt.Errorf("resolve SQLite snapshot path: %w", err)
	}
	snapshotPath = filepath.Clean(snapshotPath)
	if _, _, err := ValidateSnapshot(ctx, snapshotPath, compatibility); err != nil {
		return RestoreOperation{}, fmt.Errorf("reject restore snapshot: %w", err)
	}

	markerPath := PendingRestorePath(activePath)
	if _, err := os.Stat(markerPath); err == nil {
		return RestoreOperation{}, errors.New("another restore is already in progress")
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreOperation{}, fmt.Errorf("inspect pending restore marker: %w", err)
	}
	for _, artifact := range restoreArtifactPaths(activePath) {
		if err := rejectExistingRestoreArtifact(artifact); err != nil {
			return RestoreOperation{}, err
		}
	}

	now := time.Now().UTC()
	operation := newRestoreOperation(backupName, now)
	if err := writeRestoreOperation(activePath, operation); err != nil {
		return RestoreOperation{}, err
	}
	marker := PendingRestore{
		State:         pendingStateStaged,
		OperationID:   operation.ID,
		BackupName:    operation.BackupName,
		ActivePath:    activePath,
		SnapshotPath:  snapshotPath,
		StagedPath:    activePath + ".restore-staged",
		PreviousPath:  activePath + ".restore-previous",
		FailedPath:    activePath + ".failed-restore",
		StagedAt:      now,
		Metadata:      metadata,
		Compatibility: compatibility,
	}
	if err := writePendingRestore(markerPath, marker, true); err != nil {
		_, updateErr := updateRestoreOperation(
			marker,
			RestoreOperationFailed,
			"restore_stage_failed",
			"Restore could not be staged.",
		)
		return RestoreOperation{}, errors.Join(err, updateErr)
	}
	return operation, nil
}

// ApplyPendingRestore reconciles a pending file swap between runtime
// generations. Each state is idempotent: a crash after a rename but before its
// marker update is inferred from the durable file layout on the next call.
func ApplyPendingRestore(ctx context.Context, activePath string, logf Logf) (bool, error) {
	return applyPendingRestore(ctx, activePath, logf, nil)
}

func applyPendingRestore(ctx context.Context, activePath string, logf Logf, fault restoreFault) (bool, error) {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return false, fmt.Errorf("resolve active SQLite path: %w", err)
	}
	activePath = filepath.Clean(activePath)
	markerPath := PendingRestorePath(activePath)
	marker, err := readPendingRestore(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	normalizeRestorePaths(&marker)
	if marker.ActivePath != activePath {
		return false, fmt.Errorf("pending restore active path %q does not match %q", marker.ActivePath, activePath)
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	if marker.OperationID != "" {
		status := RestoreOperationInstalling
		message := "Installing the selected database snapshot."
		switch {
		case isRollbackState(marker.State):
			status = RestoreOperationRollingBack
			message = "Restore verification failed; rolling back."
		case marker.State == pendingStateActiveInstalled || marker.State == pendingStateVerified:
			status = RestoreOperationVerifying
			message = "Verifying the restored runtime."
		case marker.State == pendingStateCompleted:
			status = RestoreOperationCompleted
			message = "Restore completed."
		}
		if _, err := updateRestoreOperation(marker, status, marker.FailureCode, message); err != nil {
			logf("restore: update operation %s to %s: %v", marker.OperationID, status, err)
		}
	}

	if isRollbackState(marker.State) {
		if err := rollbackPendingRestore(ctx, activePath, logf, fault); err != nil {
			return false, err
		}
		return false, nil
	}
	if marker.State == pendingStateCompleted {
		if err := completePendingRestore(ctx, activePath, &marker); err != nil {
			return false, err
		}
		return false, nil
	}

	for {
		switch marker.State {
		case pendingStateStaged:
			if _, _, err := ValidateSnapshot(ctx, marker.SnapshotPath, marker.Compatibility); err != nil {
				return false, fmt.Errorf("revalidate staged restore snapshot: %w", err)
			}
			layout, err := inspectRestoreLayout(ctx, marker)
			if err != nil {
				return false, err
			}
			switch {
			case layout.previous && !layout.active && layout.staged:
				if err := setRestoreState(markerPath, &marker, pendingStatePreviousPreserved); err != nil {
					return false, err
				}
				continue
			case layout.previous && layout.active && !layout.staged:
				if err := setRestoreState(markerPath, &marker, pendingStateActiveInstalled); err != nil {
					return false, err
				}
				continue
			case layout.previous:
				return false, fmt.Errorf("ambiguous staged restore layout: active=%t staged=%t previous=%t", layout.active, layout.staged, layout.previous)
			case !layout.active:
				return false, errors.New("staged restore has no active or retained previous catalog")
			}

			if err := requireCatalogIdentity(ctx, marker.ActivePath, marker.Compatibility.LibraryID, false); err != nil {
				return false, fmt.Errorf("inspect drained active SQLite catalog: %w", err)
			}
			if marker.RestorePoint == "" {
				restorePoint, err := createRestorePoint(ctx, marker, logf)
				if err != nil {
					return false, err
				}
				marker.RestorePoint = restorePoint
				if err := writePendingRestore(markerPath, marker, false); err != nil {
					return false, err
				}
			}
			if layout.staged {
				if err := requireCatalogIdentity(ctx, marker.StagedPath, marker.Compatibility.LibraryID, true); err != nil {
					// A crash during the exclusive copy can leave a partial
					// staged artifact. The validated snapshot and untouched
					// active catalog are still authoritative, so discard only
					// that derived copy and recreate it.
					if removeErr := removeFileIfPresent(marker.StagedPath); removeErr != nil {
						return false, errors.Join(
							fmt.Errorf("validate staged active SQLite catalog: %w", err),
							fmt.Errorf("remove incomplete staged catalog: %w", removeErr),
						)
					}
					if syncErr := syncDirectory(filepath.Dir(marker.ActivePath)); syncErr != nil {
						return false, syncErr
					}
					layout.staged = false
				}
			}
			if !layout.staged {
				if err := copyFile(marker.SnapshotPath, marker.StagedPath, 0o600); err != nil {
					return false, err
				}
				if err := syncDirectory(filepath.Dir(marker.ActivePath)); err != nil {
					return false, err
				}
				if err := runRestoreFault(fault, faultAfterStagedCopy); err != nil {
					return false, err
				}
			}
			if err := requireCatalogIdentity(ctx, marker.StagedPath, marker.Compatibility.LibraryID, true); err != nil {
				return false, fmt.Errorf("validate staged active SQLite catalog: %w", err)
			}
			if err := removeSQLiteSidecars(marker.ActivePath); err != nil {
				return false, err
			}
			if err := renameAndSync(marker.ActivePath, marker.PreviousPath); err != nil {
				return false, fmt.Errorf("preserve previous SQLite catalog: %w", err)
			}
			if err := runRestoreFault(fault, faultAfterPreviousRename); err != nil {
				return false, err
			}
			if err := setRestoreState(markerPath, &marker, pendingStatePreviousPreserved); err != nil {
				return false, err
			}
			if err := runRestoreFault(fault, faultAfterPreviousMarker); err != nil {
				return false, err
			}

		case pendingStatePreviousPreserved:
			layout, err := inspectRestoreLayout(ctx, marker)
			if err != nil {
				return false, err
			}
			switch {
			case layout.active && layout.previous && !layout.staged:
				if err := setRestoreState(markerPath, &marker, pendingStateActiveInstalled); err != nil {
					return false, err
				}
				continue
			case !layout.active && layout.previous && layout.staged:
				if err := renameAndSync(marker.StagedPath, marker.ActivePath); err != nil {
					return false, fmt.Errorf("activate restored SQLite catalog: %w", err)
				}
				if err := runRestoreFault(fault, faultAfterActiveRename); err != nil {
					return false, err
				}
				if err := runRestoreFault(fault, faultBeforeActiveMarker); err != nil {
					return false, err
				}
				if err := setRestoreState(markerPath, &marker, pendingStateActiveInstalled); err != nil {
					return false, err
				}
				if err := runRestoreFault(fault, faultAfterActiveMarker); err != nil {
					return false, err
				}
			default:
				return false, fmt.Errorf("invalid previous_preserved restore layout: active=%t staged=%t previous=%t", layout.active, layout.staged, layout.previous)
			}

		case pendingStateActiveInstalled:
			layout, err := inspectRestoreLayout(ctx, marker)
			if err != nil {
				return false, err
			}
			if !layout.active || !layout.previous || layout.staged {
				return false, fmt.Errorf("invalid active_installed restore layout: active=%t staged=%t previous=%t", layout.active, layout.staged, layout.previous)
			}
			if err := requireCatalogIdentity(ctx, marker.ActivePath, marker.Compatibility.LibraryID, true); err != nil {
				return false, fmt.Errorf("verify installed SQLite catalog: %w", err)
			}
			if err := setRestoreState(markerPath, &marker, pendingStateVerified); err != nil {
				return false, err
			}
			if _, err := updateRestoreOperation(
				marker,
				RestoreOperationVerifying,
				"",
				"Verifying the restored runtime.",
			); err != nil {
				logf("restore: update operation %s to verifying: %v", marker.OperationID, err)
			}
			if err := runRestoreFault(fault, faultAfterVerifiedMarker); err != nil {
				return false, err
			}
			logf("restore: activated %s; previous catalog retained until health verification", filepath.Base(marker.SnapshotPath))
			return true, nil

		case pendingStateVerified:
			return true, nil

		default:
			return false, fmt.Errorf("unknown pending restore state %q", marker.State)
		}
	}
}

// CompletePendingRestore is called only after the new runtime has migrated and
// passed its health checks. Completion itself is journaled before cleanup so a
// crash cannot turn a healthy restore into an attempted rollback.
func CompletePendingRestore(ctx context.Context, activePath string) error {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return fmt.Errorf("resolve active SQLite path: %w", err)
	}
	activePath = filepath.Clean(activePath)
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	normalizeRestorePaths(&marker)
	return completePendingRestore(ctx, activePath, &marker)
}

func completePendingRestore(ctx context.Context, activePath string, marker *PendingRestore) error {
	markerPath := PendingRestorePath(activePath)
	if marker.State != pendingStateVerified && marker.State != pendingStateCompleted {
		return fmt.Errorf("cannot complete pending restore in state %q", marker.State)
	}
	if marker.State == pendingStateVerified {
		if err := requireCatalogIdentity(ctx, activePath, marker.Compatibility.LibraryID, false); err != nil {
			return fmt.Errorf("verify restored SQLite catalog before completion: %w", err)
		}
		if err := setRestoreState(markerPath, marker, pendingStateCompleted); err != nil {
			return err
		}
	}
	if _, err := updateRestoreOperation(
		*marker,
		RestoreOperationCompleted,
		"",
		"Restore completed.",
	); err != nil {
		return fmt.Errorf("record completed restore operation: %w", err)
	}
	if err := removeFileIfPresent(marker.PreviousPath); err != nil {
		return fmt.Errorf("remove previous SQLite catalog: %w", err)
	}
	if err := removeSQLiteSidecars(marker.PreviousPath); err != nil {
		return err
	}
	if err := removeFileIfPresent(marker.StagedPath); err != nil {
		return fmt.Errorf("remove staged SQLite catalog: %w", err)
	}
	if err := syncDirectory(filepath.Dir(activePath)); err != nil {
		return err
	}
	if err := removeFileIfPresent(markerPath); err != nil {
		return fmt.Errorf("remove pending restore marker: %w", err)
	}
	return syncDirectory(filepath.Dir(activePath))
}

// RollbackPendingRestore restores the retained previous main file after the
// restored generation fails startup. Its intermediate renames use the same
// durable reconciliation rules as the forward swap.
func RollbackPendingRestore(ctx context.Context, activePath string, logf Logf) error {
	return rollbackPendingRestoreWithCause(
		ctx,
		activePath,
		logf,
		nil,
		"restore_runtime_failed",
		"The restored database did not pass runtime verification. The previous database was restored.",
	)
}

// RollbackPendingRestoreWithCause records a stable public reason while raw
// startup errors remain in structured server logs.
func RollbackPendingRestoreWithCause(
	ctx context.Context,
	activePath string,
	logf Logf,
	code string,
	message string,
) error {
	return rollbackPendingRestoreWithCause(ctx, activePath, logf, nil, code, message)
}

func rollbackPendingRestore(ctx context.Context, activePath string, logf Logf, fault restoreFault) error {
	return rollbackPendingRestoreWithCause(
		ctx,
		activePath,
		logf,
		fault,
		"restore_runtime_failed",
		"The restored database did not pass runtime verification. The previous database was restored.",
	)
}

func rollbackPendingRestoreWithCause(
	ctx context.Context,
	activePath string,
	logf Logf,
	fault restoreFault,
	code string,
	message string,
) error {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return fmt.Errorf("resolve active SQLite path: %w", err)
	}
	activePath = filepath.Clean(activePath)
	markerPath := PendingRestorePath(activePath)
	marker, err := readPendingRestore(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	normalizeRestorePaths(&marker)
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if strings.TrimSpace(code) != "" {
		marker.FailureCode = strings.TrimSpace(code)
	}
	if strings.TrimSpace(message) != "" {
		marker.FailureMessage = strings.TrimSpace(message)
	}
	if err := writePendingRestore(markerPath, marker, false); err != nil {
		return err
	}
	if _, err := updateRestoreOperation(
		marker,
		RestoreOperationRollingBack,
		marker.FailureCode,
		"Restore verification failed; rolling back.",
	); err != nil {
		logf("restore: update operation %s to rolling_back: %v", marker.OperationID, err)
	}

	for {
		switch marker.State {
		case pendingStateActiveInstalled, pendingStateVerified:
			if err := setRestoreState(markerPath, &marker, pendingStateRollbackStarted); err != nil {
				return err
			}

		case pendingStateRollbackStarted:
			layout, err := inspectRestoreLayout(ctx, marker)
			if err != nil {
				return err
			}
			switch {
			case !layout.active && layout.previous && layout.failed:
				if err := setRestoreState(markerPath, &marker, pendingStateFailedPreserved); err != nil {
					return err
				}
				continue
			case layout.active && layout.previous && !layout.failed:
				if err := removeSQLiteSidecars(marker.ActivePath); err != nil {
					return err
				}
				if err := renameAndSync(marker.ActivePath, marker.FailedPath); err != nil {
					return fmt.Errorf("preserve failed restored SQLite catalog: %w", err)
				}
				if err := runRestoreFault(fault, faultAfterFailedRename); err != nil {
					return err
				}
				if err := setRestoreState(markerPath, &marker, pendingStateFailedPreserved); err != nil {
					return err
				}
				if err := runRestoreFault(fault, faultAfterFailedMarker); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid rollback_started layout: active=%t previous=%t failed=%t", layout.active, layout.previous, layout.failed)
			}

		case pendingStateFailedPreserved:
			layout, err := inspectRestoreLayout(ctx, marker)
			if err != nil {
				return err
			}
			switch {
			case layout.active && !layout.previous && layout.failed:
				if err := setRestoreState(markerPath, &marker, pendingStatePreviousRestored); err != nil {
					return err
				}
				continue
			case !layout.active && layout.previous && layout.failed:
				if err := renameAndSync(marker.PreviousPath, marker.ActivePath); err != nil {
					return fmt.Errorf("roll back previous SQLite catalog: %w", err)
				}
				if err := runRestoreFault(fault, faultAfterRollbackRename); err != nil {
					return err
				}
				if err := setRestoreState(markerPath, &marker, pendingStatePreviousRestored); err != nil {
					return err
				}
				if err := runRestoreFault(fault, faultAfterRollbackMarker); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid failed_preserved layout: active=%t previous=%t failed=%t", layout.active, layout.previous, layout.failed)
			}

		case pendingStatePreviousRestored:
			if err := requireCatalogIdentity(ctx, marker.ActivePath, marker.Compatibility.LibraryID, false); err != nil {
				return fmt.Errorf("verify rolled-back SQLite catalog: %w", err)
			}
			publicMessage := marker.FailureMessage
			if publicMessage == "" {
				publicMessage = "Restore failed and the previous database was restored."
			}
			if _, err := updateRestoreOperation(
				marker,
				RestoreOperationRolledBack,
				marker.FailureCode,
				publicMessage,
			); err != nil {
				return fmt.Errorf("record rolled-back restore operation: %w", err)
			}
			if err := removeFileIfPresent(marker.FailedPath); err != nil {
				return fmt.Errorf("remove failed restored SQLite catalog: %w", err)
			}
			if err := removeSQLiteSidecars(marker.FailedPath); err != nil {
				return err
			}
			if err := syncDirectory(filepath.Dir(activePath)); err != nil {
				return err
			}
			if err := removeFileIfPresent(markerPath); err != nil {
				return fmt.Errorf("remove pending restore marker after rollback: %w", err)
			}
			if err := syncDirectory(filepath.Dir(activePath)); err != nil {
				return err
			}
			logf("restore: rolled back to previous SQLite catalog; restore point kept at %s", marker.RestorePoint)
			return nil

		default:
			return fmt.Errorf("cannot roll back pending restore in state %q", marker.State)
		}
	}
}

// HasAppliedRestore reports whether startup is validating the newly installed
// catalog and should roll it back if runtime health checks fail.
func HasAppliedRestore(activePath string) bool {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if err != nil {
		return false
	}
	return marker.State == pendingStateActiveInstalled || marker.State == pendingStateVerified
}

type restoreLayout struct {
	active   bool
	staged   bool
	previous bool
	failed   bool
}

func inspectRestoreLayout(ctx context.Context, marker PendingRestore) (restoreLayout, error) {
	active, err := pathExists(marker.ActivePath)
	if err != nil {
		return restoreLayout{}, err
	}
	staged, err := pathExists(marker.StagedPath)
	if err != nil {
		return restoreLayout{}, err
	}
	previous, err := pathExists(marker.PreviousPath)
	if err != nil {
		return restoreLayout{}, err
	}
	failed, err := pathExists(marker.FailedPath)
	if err != nil {
		return restoreLayout{}, err
	}
	_ = ctx
	return restoreLayout{active: active, staged: staged, previous: previous, failed: failed}, nil
}

func createRestorePoint(ctx context.Context, marker PendingRestore, logf Logf) (string, error) {
	source, err := sql.Open("sqlite3", marker.ActivePath)
	if err != nil {
		return "", fmt.Errorf("open drained SQLite catalog for restore point: %w", err)
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
		return "", fmt.Errorf("create pre-restore snapshot: %w", snapshotErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close pre-restore snapshot source: %w", closeErr)
	}
	return restorePoint.Path, nil
}

func requireCatalogIdentity(ctx context.Context, path, expectedLibraryID string, standalone bool) error {
	var (
		info db.CatalogInfo
		err  error
	)
	if standalone {
		info, err = db.InspectStandaloneCatalog(ctx, path)
	} else {
		info, err = db.InspectCatalog(ctx, path)
	}
	if err != nil {
		return err
	}
	if expectedLibraryID != "" && info.LibraryID != expectedLibraryID {
		return fmt.Errorf("catalog library identity %q does not match %q", info.LibraryID, expectedLibraryID)
	}
	return nil
}

func setRestoreState(markerPath string, marker *PendingRestore, state string) error {
	marker.State = state
	return writePendingRestore(markerPath, *marker, false)
}

func runRestoreFault(fault restoreFault, point string) error {
	if fault == nil {
		return nil
	}
	if err := fault(point); err != nil {
		return fmt.Errorf("injected restore fault at %s: %w", point, err)
	}
	return nil
}

func renameAndSync(sourcePath, destinationPath string) error {
	if err := renameFile(sourcePath, destinationPath); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(destinationPath))
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect restore artifact %s: %w", path, err)
}

func removeFileIfPresent(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func isRollbackState(state string) bool {
	switch state {
	case pendingStateRollbackStarted, pendingStateFailedPreserved, pendingStatePreviousRestored:
		return true
	default:
		return false
	}
}

func restoreArtifactPaths(activePath string) []string {
	return []string{
		activePath + ".restore-staged",
		activePath + ".restore-previous",
		activePath + ".failed-restore",
	}
}

func normalizeRestorePaths(marker *PendingRestore) {
	if marker.StagedPath == "" {
		marker.StagedPath = marker.ActivePath + ".restore-staged"
	}
	if marker.PreviousPath == "" {
		marker.PreviousPath = marker.ActivePath + ".restore-previous"
	}
	if marker.FailedPath == "" {
		marker.FailedPath = marker.ActivePath + ".failed-restore"
	}
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
	if exclusive {
		if _, err := os.Stat(path); err == nil {
			return errors.New("another restore is already in progress")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect pending restore marker: %w", err)
		}
	}
	file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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
	if err := renameFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalize pending restore marker: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync pending restore marker directory: %w", err)
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
