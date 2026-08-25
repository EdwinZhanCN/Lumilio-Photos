package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteWriterMonitorPublishesOnlyOnTelemetryCadence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	telemetry := make(chan time.Time)
	checkpoints := make(chan time.Time)
	published := make(chan struct{}, 3)
	checkpointed := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runSQLiteWriterMonitor(
			ctx,
			telemetry,
			checkpoints,
			func() { published <- struct{}{} },
			func() { checkpointed <- struct{}{} },
		)
	}()

	select {
	case <-published: // Initial publication.
	case <-time.After(time.Second):
		t.Fatal("monitor did not publish its initial observation")
	}
	checkpoints <- time.Now()
	select {
	case <-checkpointed:
	case <-time.After(time.Second):
		t.Fatal("monitor did not perform checkpoint work")
	}
	select {
	case <-published:
		t.Fatal("checkpoint turn published a second latest-only telemetry interval")
	case <-time.After(25 * time.Millisecond):
	}

	telemetry <- time.Now()
	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("telemetry turn did not publish")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("monitor did not stop after cancellation")
	}
}

func TestWriteSQLiteRuntimeObservationPublishesLatestOnly(t *testing.T) {
	directory := t.TempDir()
	first := sqliteRuntimeObservation{
		ObservedAt:     time.Unix(10, 0).UTC(),
		RuntimeElapsed: time.Second,
	}
	if err := writeSQLiteRuntimeObservation(directory, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ObservedAt = time.Unix(20, 0).UTC()
	second.RuntimeElapsed = 2 * time.Second
	if err := writeSQLiteRuntimeObservation(directory, second); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, sqliteRuntimeObservationFilename)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded sqliteRuntimeObservation
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if !decoded.ObservedAt.Equal(second.ObservedAt) || decoded.RuntimeElapsed != second.RuntimeElapsed {
		t.Fatalf("decoded observation = %#v, want latest %#v", decoded, second)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("diagnostic permissions = %o, want 600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != sqliteRuntimeObservationFilename {
		t.Fatalf("diagnostic directory contains stale files: %#v", entries)
	}
}

func TestBoundedDiagnosticError(t *testing.T) {
	if got := boundedDiagnosticError(nil); got != "" {
		t.Fatalf("nil error = %q", got)
	}
	got := boundedDiagnosticError(errors.New(strings.Repeat("x", 600)))
	if len(got) != 512 {
		t.Fatalf("bounded error length = %d, want 512", len(got))
	}
}
