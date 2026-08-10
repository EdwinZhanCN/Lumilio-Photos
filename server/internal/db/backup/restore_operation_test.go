package backup

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestRestoreOperationTracksCompletionAcrossGeneration(t *testing.T) {
	activePath := preparePendingRestore(t)
	operation, err := ReadLatestRestoreOperation(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != RestoreOperationStaged || operation.Terminal() {
		t.Fatalf("staged operation = %+v", operation)
	}
	operationID := operation.ID

	operation, err = MarkPendingRestoreRestartRequested(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != RestoreOperationRestartRequested {
		t.Fatalf("accepted operation status = %q", operation.Status)
	}

	applied, err := ApplyPendingRestore(context.Background(), activePath, t.Logf)
	if err != nil || !applied {
		t.Fatalf("ApplyPendingRestore = %t/%v", applied, err)
	}
	operation, err = ReadRestoreOperation(activePath, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != RestoreOperationVerifying || operation.Terminal() {
		t.Fatalf("verifying operation = %+v", operation)
	}

	if err := CompletePendingRestore(context.Background(), activePath); err != nil {
		t.Fatal(err)
	}
	operation, err = ReadRestoreOperation(activePath, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != RestoreOperationCompleted || !operation.Terminal() || operation.CompletedAt == nil {
		t.Fatalf("completed operation = %+v", operation)
	}
	if operation.RestorePoint == "" || operation.RestorePoint == "." {
		t.Fatalf("completed operation restore point = %q", operation.RestorePoint)
	}
}

func TestRestoreOperationTracksSuccessfulRollback(t *testing.T) {
	activePath := preparePendingRestore(t)
	operation, err := MarkPendingRestoreRestartRequested(activePath)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := ApplyPendingRestore(context.Background(), activePath, t.Logf)
	if err != nil || !applied {
		t.Fatalf("ApplyPendingRestore = %t/%v", applied, err)
	}
	if err := RollbackPendingRestoreWithCause(
		context.Background(),
		activePath,
		t.Logf,
		"restore_health_check_failed",
		"The restored database did not pass runtime verification. The previous database was restored.",
	); err != nil {
		t.Fatal(err)
	}

	receipt, err := ReadRestoreOperation(activePath, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != RestoreOperationRolledBack || !receipt.Terminal() {
		t.Fatalf("rolled-back operation = %+v", receipt)
	}
	if receipt.ErrorCode != "restore_health_check_failed" {
		t.Fatalf("error code = %q", receipt.ErrorCode)
	}
	if receipt.Message == "" || receipt.CompletedAt == nil {
		t.Fatalf("rolled-back receipt is incomplete: %+v", receipt)
	}
}

func TestRestoreOperationIDCannotObserveLaterReceipt(t *testing.T) {
	activePath := preparePendingRestore(t)
	operation, err := ReadLatestRestoreOperation(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRestoreOperation(activePath, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mismatched operation lookup = %v, want os.ErrNotExist", err)
	}
	if _, err := ReadRestoreOperation(activePath, operation.ID); err != nil {
		t.Fatalf("matching operation lookup: %v", err)
	}
}

func TestFailRestoreOperationIfCurrentDoesNotOverwriteLaterReceipt(t *testing.T) {
	activePath := preparePendingRestore(t)
	current, err := ReadLatestRestoreOperation(activePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := FailRestoreOperationIfCurrent(
		activePath,
		"00000000-0000-0000-0000-000000000000",
		"stale_failure",
		"A stale request failed.",
	); err != nil {
		t.Fatal(err)
	}
	unchanged, err := ReadLatestRestoreOperation(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.ID != current.ID || unchanged.Status != current.Status {
		t.Fatalf("stale failure changed current receipt: before=%+v after=%+v", current, unchanged)
	}

	if err := FailRestoreOperationIfCurrent(
		activePath,
		current.ID,
		"restore_restart_rejected",
		"Runtime restart could not be requested; the active database was not changed.",
	); err != nil {
		t.Fatal(err)
	}
	failed, err := ReadRestoreOperation(activePath, current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != RestoreOperationFailed || !failed.Terminal() || failed.CompletedAt == nil {
		t.Fatalf("failed operation = %+v", failed)
	}
	if failed.ErrorCode != "restore_restart_rejected" {
		t.Fatalf("error code = %q", failed.ErrorCode)
	}
}
