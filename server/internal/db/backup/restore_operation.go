package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RestoreOperationStatus is the durable, user-visible lifecycle of one
// database restore request. The pending restore marker remains the file-swap
// journal; this receipt is the observation surface used across runtime restarts.
type RestoreOperationStatus string

const (
	RestoreOperationStaged           RestoreOperationStatus = "staged"
	RestoreOperationRestartRequested RestoreOperationStatus = "restart_requested"
	RestoreOperationInstalling       RestoreOperationStatus = "installing"
	RestoreOperationVerifying        RestoreOperationStatus = "verifying"
	RestoreOperationCompleted        RestoreOperationStatus = "completed"
	RestoreOperationRollingBack      RestoreOperationStatus = "rolling_back"
	RestoreOperationRolledBack       RestoreOperationStatus = "rolled_back"
	RestoreOperationFailed           RestoreOperationStatus = "failed"
)

// RestoreOperation is safe to expose through the admin API. It contains no
// filesystem paths or raw internal errors. Only the latest accepted operation
// is retained because at most one restore may be active.
type RestoreOperation struct {
	ID           string                 `json:"id"`
	BackupName   string                 `json:"backup_name"`
	Status       RestoreOperationStatus `json:"status"`
	Message      string                 `json:"message"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	RestorePoint string                 `json:"restore_point,omitempty"`
	RequestedAt  time.Time              `json:"requested_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}

// Terminal reports whether polling can stop for this operation.
func (o RestoreOperation) Terminal() bool {
	switch o.Status {
	case RestoreOperationCompleted, RestoreOperationRolledBack, RestoreOperationFailed:
		return true
	default:
		return false
	}
}

// RestoreOperationPath is the single latest-operation receipt adjacent to the
// active catalog. A new accepted restore replaces only a terminal predecessor.
func RestoreOperationPath(activePath string) string {
	return activePath + ".restore-operation.json"
}

func newRestoreOperation(backupName string, now time.Time) RestoreOperation {
	return RestoreOperation{
		ID:          uuid.NewString(),
		BackupName:  filepath.Base(strings.TrimSpace(backupName)),
		Status:      RestoreOperationStaged,
		Message:     "Restore validated and staged.",
		RequestedAt: now,
		UpdatedAt:   now,
	}
}

// ReadLatestRestoreOperation returns the latest durable receipt.
func ReadLatestRestoreOperation(activePath string) (RestoreOperation, error) {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return RestoreOperation{}, fmt.Errorf("resolve active SQLite path: %w", err)
	}
	data, err := os.ReadFile(RestoreOperationPath(filepath.Clean(activePath)))
	if err != nil {
		return RestoreOperation{}, err
	}
	var operation RestoreOperation
	if err := json.Unmarshal(data, &operation); err != nil {
		return RestoreOperation{}, fmt.Errorf("decode restore operation: %w", err)
	}
	if operation.ID == "" {
		return RestoreOperation{}, errors.New("restore operation is missing its identity")
	}
	return operation, nil
}

// ReadRestoreOperation returns the latest receipt only when its immutable ID
// matches the caller's operation ID. A later restore therefore cannot be
// mistaken for an operation a browser was already tracking.
func ReadRestoreOperation(activePath, operationID string) (RestoreOperation, error) {
	if _, err := uuid.Parse(strings.TrimSpace(operationID)); err != nil {
		return RestoreOperation{}, fmt.Errorf("invalid restore operation id: %w", err)
	}
	operation, err := ReadLatestRestoreOperation(activePath)
	if err != nil {
		return RestoreOperation{}, err
	}
	if operation.ID != strings.TrimSpace(operationID) {
		return RestoreOperation{}, os.ErrNotExist
	}
	return operation, nil
}

func writeRestoreOperation(activePath string, operation RestoreOperation) error {
	activePath, err := filepath.Abs(activePath)
	if err != nil {
		return fmt.Errorf("resolve active SQLite path: %w", err)
	}
	activePath = filepath.Clean(activePath)
	operation.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(operation, "", "  ")
	if err != nil {
		return fmt.Errorf("encode restore operation: %w", err)
	}
	data = append(data, '\n')
	path := RestoreOperationPath(activePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create restore operation directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".restore-operation-*.tmp")
	if err != nil {
		return fmt.Errorf("create restore operation temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure restore operation temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write restore operation: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync restore operation: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close restore operation: %w", err)
	}
	if err := renameFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalize restore operation: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync restore operation directory: %w", err)
	}
	return nil
}

func updateRestoreOperation(marker PendingRestore, status RestoreOperationStatus, code, message string) (RestoreOperation, error) {
	if marker.OperationID == "" {
		return RestoreOperation{}, nil
	}
	operation, err := ReadRestoreOperation(marker.ActivePath, marker.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		operation = RestoreOperation{
			ID:          marker.OperationID,
			BackupName:  marker.BackupName,
			RequestedAt: marker.StagedAt,
		}
	} else if err != nil {
		return RestoreOperation{}, err
	}
	now := time.Now().UTC()
	operation.Status = status
	operation.ErrorCode = strings.TrimSpace(code)
	operation.Message = strings.TrimSpace(message)
	operation.RestorePoint = ""
	if marker.RestorePoint != "" {
		operation.RestorePoint = filepath.Base(marker.RestorePoint)
	}
	operation.UpdatedAt = now
	if operation.Terminal() {
		operation.CompletedAt = &now
	} else {
		operation.CompletedAt = nil
	}
	if err := writeRestoreOperation(marker.ActivePath, operation); err != nil {
		return RestoreOperation{}, err
	}
	return operation, nil
}

// MarkPendingRestoreRestartRequested commits the observable hand-off before
// the API requests that the current runtime generation drain and restart.
func MarkPendingRestoreRestartRequested(activePath string) (RestoreOperation, error) {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if err != nil {
		return RestoreOperation{}, err
	}
	return updateRestoreOperation(
		marker,
		RestoreOperationRestartRequested,
		"",
		"Runtime restart requested.",
	)
}

// ValidatePendingRestoreOperation prevents an old browser response from
// restarting a later restore operation.
func ValidatePendingRestoreOperation(activePath, operationID string) error {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if err != nil {
		return err
	}
	if marker.OperationID == "" || marker.OperationID != strings.TrimSpace(operationID) {
		return errors.New("restore operation no longer matches the staged restore")
	}
	return nil
}

// FailRestoreOperationIfCurrent marks operationID failed only when it is still
// the latest receipt. A stale browser response must never overwrite a newer
// restore operation.
func FailRestoreOperationIfCurrent(activePath, operationID, code, message string) error {
	operation, err := ReadRestoreOperation(activePath, operationID)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if operation.Terminal() {
		return nil
	}
	now := time.Now().UTC()
	operation.Status = RestoreOperationFailed
	operation.ErrorCode = strings.TrimSpace(code)
	operation.Message = strings.TrimSpace(message)
	operation.CompletedAt = &now
	return writeRestoreOperation(activePath, operation)
}

// FailPendingRestoreOperation records a stable public failure without exposing
// the underlying error. It is best-effort for callers already on a fatal path.
func FailPendingRestoreOperation(activePath, code, message string) error {
	marker, err := readPendingRestore(PendingRestorePath(activePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = updateRestoreOperation(marker, RestoreOperationFailed, code, message)
	return err
}

// CancelStagedRestore removes a marker before any database file was touched.
// It is used if the API cannot durably commit the restart-requested status.
func CancelStagedRestore(activePath, code, message string) error {
	markerPath := PendingRestorePath(activePath)
	marker, err := readPendingRestore(markerPath)
	if err != nil {
		return err
	}
	if marker.State != pendingStateStaged {
		return fmt.Errorf("cannot cancel pending restore in state %q", marker.State)
	}
	var errs []error
	if _, err := updateRestoreOperation(marker, RestoreOperationFailed, code, message); err != nil {
		errs = append(errs, err)
	}
	if err := removeFileIfPresent(markerPath); err != nil {
		errs = append(errs, fmt.Errorf("remove cancelled restore marker: %w", err))
	}
	if err := syncDirectory(filepath.Dir(markerPath)); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
