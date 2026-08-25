package queue

import (
	"testing"
	"time"
)

func TestDatabaseBackupWorkerHasExplicitSizeAwareTimeout(t *testing.T) {
	worker := &DatabaseBackupWorker{}
	if got := worker.Timeout(nil); got != DatabaseBackupTimeout || got < 20*time.Minute {
		t.Fatalf("backup timeout = %s, want explicit bounded large-snapshot budget %s", got, DatabaseBackupTimeout)
	}
}
