package queue

import "testing"

func TestScanRepositoryWorkerHasNoFixedTimeout(t *testing.T) {
	worker := &ScanRepositoryWorker{}
	if timeout := worker.Timeout(nil); timeout != -1 {
		t.Fatalf("scan worker timeout = %s, want no fixed timeout", timeout)
	}
}
