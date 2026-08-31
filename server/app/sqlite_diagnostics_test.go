package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeDiagnosticsReadCatalogAndMacroTruth(t *testing.T) {
	startedAt := time.Unix(100, 0).UTC()
	endedAt := startedAt.Add(10 * time.Second)

	catalog, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	catalog.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = catalog.Close() })
	for _, statement := range []string{
		`CREATE TABLE asset_pipeline_state(desired_version INTEGER,applied_version INTEGER,terminal_error TEXT,updated_at INTEGER)`,
		`CREATE TABLE repository_observation_state(desired_epoch INTEGER,applied_epoch INTEGER,full_verification_required INTEGER,terminal_error TEXT,updated_at INTEGER)`,
		`CREATE TABLE event_projection_pipeline_state(source_revision INTEGER,applied_revision INTEGER,terminal_error TEXT,updated_at INTEGER)`,
		`CREATE TABLE location_projection_state(source_revision INTEGER,published_revision INTEGER,terminal_error TEXT,updated_at INTEGER)`,
		`CREATE TABLE location_resolution_pipeline_state(projection_version INTEGER,applied_revision INTEGER,terminal_error TEXT,updated_at INTEGER)`,
		`CREATE TABLE ocr_projection_pipeline_state(projection_version INTEGER,applied_revision INTEGER,terminal_error TEXT,updated_at INTEGER)`,
		`CREATE TABLE asset_reindex_requests(requested_revision INTEGER,applied_revision INTEGER,updated_at INTEGER)`,
		`CREATE TABLE catalog_operation_receipts(desired_version INTEGER,applied_version INTEGER,state TEXT,updated_at INTEGER)`,
		`CREATE TABLE domain_outbox(delivered_at INTEGER,created_at INTEGER)`,
		`INSERT INTO asset_pipeline_state VALUES(3,1,NULL,1)`,
		`INSERT INTO repository_observation_state VALUES(1,1,1,NULL,1)`,
		`INSERT INTO event_projection_pipeline_state VALUES(5,2,'attempts_exhausted',1)`,
		`INSERT INTO location_projection_state VALUES(2,2,NULL,105000000)`,
		`INSERT INTO asset_reindex_requests VALUES(4,4,106000000)`,
		`INSERT INTO catalog_operation_receipts VALUES(1,1,'completed',107000000)`,
		`INSERT INTO domain_outbox VALUES(NULL,106000000)`,
	} {
		if _, err := catalog.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	catalogSnapshot, err := readCatalogPipelineObservation(context.Background(), catalog, startedAt, endedAt)
	if err != nil {
		t.Fatal(err)
	}
	if catalogSnapshot.Backlog != 3 || catalogSnapshot.RunnableBacklog != 2 || catalogSnapshot.TerminalBacklog != 1 || catalogSnapshot.DesiredAppliedLag != 6 {
		t.Fatalf("catalog backlog snapshot = %+v", catalogSnapshot)
	}
	if catalogSnapshot.PendingOutbox != 1 || catalogSnapshot.OldestOutboxAge != 4*time.Second {
		t.Fatalf("catalog outbox snapshot = %+v", catalogSnapshot)
	}
	if catalogSnapshot.AppliedTransitions != 3 || catalogSnapshot.AppliedTransitionsPerSecond != 0.3 {
		t.Fatalf("catalog throughput snapshot = %+v", catalogSnapshot)
	}

	queueDatabase, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	queueDatabase.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = queueDatabase.Close() })
	if _, err := queueDatabase.Exec(`CREATE TABLE river_job(queue TEXT,state TEXT,created_at TIMESTAMP,attempted_at TIMESTAMP,finalized_at TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := queueDatabase.Exec(
		`INSERT INTO river_job VALUES('catalog_macro','completed',?,?,?),('catalog_macro','available',?,NULL,NULL),('other','available',?,NULL,NULL)`,
		startedAt.Add(time.Second), startedAt.Add(3*time.Second), startedAt.Add(4*time.Second),
		endedAt.Add(-5*time.Second), endedAt.Add(-9*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	queueSnapshot, err := readMacroQueueObservation(context.Background(), queueDatabase, startedAt, endedAt)
	if err != nil {
		t.Fatal(err)
	}
	if queueSnapshot.Available != 1 || queueSnapshot.Remaining != 1 || queueSnapshot.Completed != 1 || queueSnapshot.CompletedInterval != 1 || queueSnapshot.CompletedPerSecond != 0.1 {
		t.Fatalf("macro queue counts = %+v", queueSnapshot)
	}
	if durationDelta(queueSnapshot.AverageLatency, 3*time.Second) > 100*time.Microsecond ||
		durationDelta(queueSnapshot.AverageRuntime, time.Second) > 100*time.Microsecond ||
		queueSnapshot.OldestRemainingAge != 5*time.Second {
		t.Fatalf("macro queue latency = %+v", queueSnapshot)
	}
}

func durationDelta(left, right time.Duration) time.Duration {
	if left < right {
		return right - left
	}
	return left - right
}

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
