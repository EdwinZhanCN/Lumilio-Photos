package catalogtx

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
	sqlite3 "github.com/mattn/go-sqlite3"
)

func TestWriterRecordsCommittedTransactionAndHistogram(t *testing.T) {
	pool := openTestPool(t)
	capture := &capturingObserver{}
	recorder := NewRecorder()
	writer := NewWriter(pool, JoinObservers(capture, recorder))

	err := writer.Transact(
		context.Background(),
		OperationRepositoryObservationClaim,
		nil,
		func(tx *sql.Tx) error {
			_, err := tx.ExecContext(context.Background(), `INSERT INTO probes(value) VALUES ('committed')`)
			return err
		},
	)
	if err != nil {
		t.Fatalf("transact: %v", err)
	}

	sample := capture.single(t)
	if sample.Operation != OperationRepositoryObservationClaim {
		t.Fatalf("operation = %v, want %v", sample.Operation, OperationRepositoryObservationClaim)
	}
	if sample.Role != RoleWriter || sample.Outcome != OutcomeCommitted {
		t.Fatalf("role/outcome = %v/%v, want writer/committed", sample.Role, sample.Outcome)
	}
	if sample.Cancellation != CancellationNone {
		t.Fatalf("cancellation = %v, want none", sample.Cancellation)
	}
	if sample.Total < sample.Admission+sample.Body+sample.Commit {
		t.Fatalf("total %s is smaller than measured phases %s", sample.Total, sample.Admission+sample.Body+sample.Commit)
	}
	if hold := sample.Total - sample.Admission; hold < sample.Body+sample.Commit {
		t.Fatalf("connection hold %s is smaller than body + commit %s", hold, sample.Body+sample.Commit)
	}

	operation, ok := recorder.Report().Operation(OperationRepositoryObservationClaim)
	if !ok {
		t.Fatal("recorder omitted observed operation")
	}
	if operation.Total.Count != 1 || operation.Hold.Count != 1 || operation.Outcomes.Committed != 1 {
		t.Fatalf("report = %+v, want one committed observation", operation)
	}
}

func TestWriterRecordsAdmissionCancellation(t *testing.T) {
	pool := openTestPool(t)
	holder, err := pool.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer holder.Rollback()
	if _, err := holder.ExecContext(context.Background(), `INSERT INTO probes(value) VALUES ('held')`); err != nil {
		t.Fatalf("prime holder: %v", err)
	}

	capture := &capturingObserver{}
	writer := NewWriter(pool, capture)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = writer.Transact(ctx, OperationRepositoryObservationClaim, nil, func(*sql.Tx) error {
		t.Fatal("transaction body ran despite admission timeout")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}

	sample := capture.single(t)
	if sample.Outcome != OutcomeBeginFailed {
		t.Fatalf("outcome = %v, want begin_failed", sample.Outcome)
	}
	if sample.Cancellation != CancellationDeadlineExceeded {
		t.Fatalf("cancellation = %v, want deadline_exceeded", sample.Cancellation)
	}
	if sample.Admission < 15*time.Millisecond || sample.Total < sample.Admission {
		t.Fatalf("admission/total = %s/%s after %s wall time", sample.Admission, sample.Total, time.Since(started))
	}
	if sample.Body != 0 || sample.Commit != 0 {
		t.Fatalf("body/commit = %s/%s before acquisition, want zero", sample.Body, sample.Commit)
	}
}

func TestReaderRecordsBoundedSnapshotLifetime(t *testing.T) {
	pool := openTestPool(t)
	capture := &capturingObserver{}
	reader := NewReader(pool, capture)

	err := reader.Snapshot(
		context.Background(),
		OperationEventRebuildSnapshot,
		func(tx *sql.Tx) error {
			var count int
			return tx.QueryRowContext(context.Background(), `SELECT count(*) FROM probes`).Scan(&count)
		},
	)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	sample := capture.single(t)
	if sample.Role != RoleReader || sample.Outcome != OutcomeCommitted {
		t.Fatalf("role/outcome = %v/%v, want reader/committed", sample.Role, sample.Outcome)
	}
	if sample.Operation != OperationEventRebuildSnapshot {
		t.Fatalf("operation = %v, want %v", sample.Operation, OperationEventRebuildSnapshot)
	}
}

func TestManualTransactionFinalizationIsObservedOnce(t *testing.T) {
	pool := openTestPool(t)
	capture := &capturingObserver{}
	tx, err := NewWriter(pool, capture).BeginTx(context.Background(), OperationAssetReprocess, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("second finalization = %v, want sql.ErrTxDone", err)
	}
	if got := capture.transactionCount(); got != 1 {
		t.Fatalf("transaction samples = %d, want exactly 1", got)
	}
}

func TestConnectorMeasuresStatementAdmissionAndReaderRowsLifetime(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
	capture := &capturingObserver{}
	recorder := NewRecorder()
	observer := JoinObservers(capture, recorder)
	driver := &sqlite3.SQLiteDriver{}

	writerPool := sql.OpenDB(NewConnector(driver, dsn, RoleWriter, observer))
	writerPool.SetMaxOpenConns(1)
	writerPool.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = writerPool.Close() })
	writer := NewWriter(writerPool, observer)
	if _, err := writer.ExecContext(
		context.Background(),
		OperationCatalogGeneratedWriterExec,
		`CREATE TABLE statement_probes(value TEXT NOT NULL)`,
	); err != nil {
		t.Fatalf("create statement probe: %v", err)
	}

	writeSample := capture.statement(t, OperationCatalogGeneratedWriterExec)
	if writeSample.Outcome != StatementOutcomeSucceeded || writeSample.Role != RoleWriter {
		t.Fatalf("write outcome/role = %v/%v", writeSample.Outcome, writeSample.Role)
	}
	if writeSample.Total < writeSample.Admission+writeSample.Execution {
		t.Fatalf("write total %s smaller than admission + execution %s", writeSample.Total, writeSample.Admission+writeSample.Execution)
	}

	readerPool := sql.OpenDB(NewConnector(driver, dsn, RoleReader, observer))
	readerPool.SetMaxOpenConns(1)
	readerPool.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = readerPool.Close() })
	reader := NewReader(readerPool, observer)
	rows, err := reader.QueryContext(
		context.Background(),
		OperationCatalogGeneratedReaderRows,
		`SELECT value FROM statement_probes`,
	)
	if err != nil {
		t.Fatalf("query statement probe: %v", err)
	}
	if opened, closed := capture.rowCounts(OperationCatalogGeneratedReaderRows); opened != 1 || closed != 0 {
		t.Fatalf("row events before close = %d/%d, want 1/0", opened, closed)
	}
	time.Sleep(time.Millisecond)
	if err := rows.Close(); err != nil {
		t.Fatalf("close rows: %v", err)
	}
	readSample := capture.statement(t, OperationCatalogGeneratedReaderRows)
	if readSample.RowsLifetime < time.Millisecond {
		t.Fatalf("reader rows lifetime = %s, want at least 1ms", readSample.RowsLifetime)
	}
	statement, ok := recorder.Report().Statement(OperationCatalogGeneratedReaderRows)
	if !ok {
		t.Fatal("recorder omitted reader statement")
	}
	if statement.Rows.Current != 0 || statement.Rows.Peak != 1 || statement.Rows.Opened != 1 || statement.Rows.Closed != 1 {
		t.Fatalf("reader rows report = %+v, want balanced peak-one cursor", statement.Rows)
	}
}

func TestStandaloneStatementRecordsAdmissionCancellationBeforeDriver(t *testing.T) {
	capture := &capturingObserver{}
	pool := sql.OpenDB(NewConnector(
		&sqlite3.SQLiteDriver{},
		"file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000",
		RoleWriter,
		capture,
	))
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = pool.Close() })

	held, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatalf("hold writer connection: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err = NewWriter(pool, capture).ExecContext(
		ctx,
		OperationCatalogGeneratedWriterExec,
		`CREATE TABLE never_runs(value TEXT)`,
	)
	_ = held.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("statement error = %v, want deadline exceeded", err)
	}
	sample := capture.statement(t, OperationCatalogGeneratedWriterExec)
	if sample.Cancellation != CancellationDeadlineExceeded || sample.Outcome != StatementOutcomeFailed {
		t.Fatalf("cancellation/outcome = %v/%v", sample.Cancellation, sample.Outcome)
	}
	if sample.Admission < 15*time.Millisecond || sample.Execution != 0 {
		t.Fatalf("admission/execution = %s/%s, want queued admission and no execution", sample.Admission, sample.Execution)
	}
}

func TestConnectorNamesRawWriterTransactionAndSuppressesInnerStatements(t *testing.T) {
	capture := &capturingObserver{}
	pool := sql.OpenDB(NewConnector(
		&sqlite3.SQLiteDriver{},
		"file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000",
		RoleWriter,
		capture,
	))
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	tx, err := pool.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("raw begin: %v", err)
	}
	if _, err := tx.ExecContext(context.Background(), `CREATE TABLE raw_tx_probe(value TEXT)`); err != nil {
		t.Fatalf("raw transaction statement: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("raw commit: %v", err)
	}
	sample := capture.transaction(t, OperationCatalogRawWriterTransaction)
	if sample.Outcome != OutcomeCommitted {
		t.Fatalf("raw transaction outcome = %v, want committed", sample.Outcome)
	}
	if _, exists := capture.findStatement(OperationCatalogUnknownWriterStatement); exists {
		t.Fatal("statement inside raw transaction was double-counted as standalone")
	}
}

func TestFailedUnknownStatementCarriesSafeQueryDiagnostics(t *testing.T) {
	capture := &capturingObserver{}
	pool := sql.OpenDB(NewConnector(
		&sqlite3.SQLiteDriver{},
		"file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000",
		RoleWriter,
		capture,
	))
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })

	_, err := pool.ExecContext(
		context.Background(),
		"-- name: DiagnosticProbe :exec\nINSERT INTO missing_probe(value) VALUES (?)",
		"private argument must never enter diagnostics",
	)
	if err == nil {
		t.Fatal("missing table statement unexpectedly succeeded")
	}
	sample := capture.statement(t, OperationCatalogUnknownWriterStatement)
	if sample.Outcome != StatementOutcomeFailed {
		t.Fatalf("outcome = %v, want failed", sample.Outcome)
	}
	if sample.QueryName != "DiagnosticProbe" {
		t.Fatalf("query name = %q, want DiagnosticProbe", sample.QueryName)
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{16}$`, sample.QueryFingerprint); !matched {
		t.Fatalf("query fingerprint = %q, want 16 lowercase hex characters", sample.QueryFingerprint)
	}
	if strings.Contains(sample.QueryName, "private") || strings.Contains(sample.QueryFingerprint, "private") {
		t.Fatalf("query diagnostics exposed an argument: name=%q fingerprint=%q", sample.QueryName, sample.QueryFingerprint)
	}
}

func TestOperationCatalogIsClosedAndLowCardinality(t *testing.T) {
	validName := regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._][a-z0-9]+)*$`)
	seen := make(map[string]Operation)
	operations := Operations()
	if len(operations) != int(operationCount)-1 {
		t.Fatalf("operation descriptors = %d, want %d", len(operations), int(operationCount)-1)
	}
	for _, descriptor := range operations {
		if !validName.MatchString(descriptor.Name) {
			t.Errorf("operation %v has invalid bounded name %q", descriptor.Operation, descriptor.Name)
		}
		if previous, exists := seen[descriptor.Name]; exists {
			t.Errorf("operations %v and %v share name %q", previous, descriptor.Operation, descriptor.Name)
		}
		seen[descriptor.Name] = descriptor.Operation
		if descriptor.Role != RoleWriter && descriptor.Role != RoleReader {
			t.Errorf("operation %q has invalid role %v", descriptor.Name, descriptor.Role)
		}
		if descriptor.Kind != OperationKindApplicationTransaction && descriptor.Kind != OperationKindStatement && descriptor.Kind != OperationKindDriverTransaction {
			t.Errorf("operation %q has invalid kind %v", descriptor.Name, descriptor.Kind)
		}
	}
	if got := OperationRepositoryObservationClaim.Name(); got != "repository.observe.claim" {
		t.Fatalf("claim operation name = %q", got)
	}

	pool := openTestPool(t)
	err := NewWriter(pool, nil).Transact(context.Background(), Operation(65535), nil, func(*sql.Tx) error {
		t.Fatal("body ran for an operation outside the static catalog")
		return nil
	})
	if !errors.Is(err, ErrInvalidOperation) {
		t.Fatalf("invalid operation error = %v, want ErrInvalidOperation", err)
	}
	err = NewWriter(pool, nil).Transact(context.Background(), OperationEventRebuildSnapshot, nil, func(*sql.Tx) error {
		t.Fatal("reader operation ran on writer capability")
		return nil
	})
	if !errors.Is(err, ErrInvalidOperation) {
		t.Fatalf("wrong-role operation error = %v, want ErrInvalidOperation", err)
	}
	err = NewWriter(pool, nil).Transact(context.Background(), OperationCatalogGeneratedWriterExec, nil, func(*sql.Tx) error {
		t.Fatal("statement operation ran as a transaction")
		return nil
	})
	if !errors.Is(err, ErrInvalidOperation) {
		t.Fatalf("wrong-kind operation error = %v, want ErrInvalidOperation", err)
	}
}

func TestRecorderReportsHDRPercentilesWithoutRetainingSamples(t *testing.T) {
	recorder := NewRecorder()
	for i := 1; i <= 100; i++ {
		duration := time.Duration(i) * time.Millisecond
		recorder.ObserveTransaction(TransactionSample{
			Operation: OperationRepositoryObservationClaim,
			Role:      RoleWriter,
			Outcome:   OutcomeCommitted,
			Admission: duration / 10,
			Body:      duration * 8 / 10,
			Commit:    duration / 10,
			Total:     duration,
		})
	}

	report := recorder.Report()
	operation, ok := report.Operation(OperationRepositoryObservationClaim)
	if !ok {
		t.Fatal("report omitted operation")
	}
	if operation.Total.Count != 100 {
		t.Fatalf("total count = %d, want 100", operation.Total.Count)
	}
	if operation.Total.P50 < 49*time.Millisecond || operation.Total.P50 > 51*time.Millisecond {
		t.Fatalf("p50 = %s, want approximately 50ms", operation.Total.P50)
	}
	if operation.Total.P99 < 98*time.Millisecond || operation.Total.P99 > 100*time.Millisecond {
		t.Fatalf("p99 = %s, want approximately 99ms", operation.Total.P99)
	}
	if operation.Total.Max < 99*time.Millisecond || operation.Total.Max > 101*time.Millisecond {
		t.Fatalf("max = %s, want approximately 100ms", operation.Total.Max)
	}
	if operation.Hold.P99 < 88*time.Millisecond || operation.Hold.P99 > 91*time.Millisecond {
		t.Fatalf("hold p99 = %s, want approximately 90ms", operation.Hold.P99)
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("empty JSON report")
	}
	if !strings.Contains(string(encoded), `"role":"writer"`) {
		t.Fatalf("report uses non-readable enum encoding: %s", encoded)
	}
}

func TestRecorderRotatesExactMergeableIntervalsWithoutResettingCumulativeReport(t *testing.T) {
	recorder := NewRecorder()
	for _, duration := range []time.Duration{3 * time.Millisecond, 9 * time.Millisecond} {
		recorder.ObserveTransaction(TransactionSample{
			Operation: OperationRepositoryObservationClaim,
			Role:      RoleWriter,
			Outcome:   OutcomeCommitted,
			Admission: time.Millisecond,
			Body:      duration - 2*time.Millisecond,
			Commit:    time.Millisecond,
			Total:     duration,
		})
	}

	interval, err := recorder.IntervalReport()
	if err != nil {
		t.Fatal(err)
	}
	operation, ok := interval.Operation(OperationRepositoryObservationClaim)
	if !ok || operation.Total.Count != 2 || operation.Total.HDRV2 == "" || operation.Hold.HDRV2 == "" {
		t.Fatalf("interval operation = %#v", operation)
	}
	decoded, err := hdrhistogram.Decode([]byte(operation.Total.HDRV2))
	if err != nil {
		t.Fatalf("decode interval HDR: %v", err)
	}
	if decoded.TotalCount() != 2 || decoded.ValueAtQuantile(100) < int64((8*time.Millisecond)/time.Microsecond) {
		t.Fatalf("decoded interval count/max = %d/%d", decoded.TotalCount(), decoded.ValueAtQuantile(100))
	}

	empty, err := recorder.IntervalReport()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := empty.Operation(OperationRepositoryObservationClaim); ok {
		t.Fatal("rotated interval retained prior samples")
	}
	cumulative, ok := recorder.Report().Operation(OperationRepositoryObservationClaim)
	if !ok || cumulative.Total.Count != 2 || cumulative.Total.HDRV2 != "" {
		t.Fatalf("cumulative operation = %#v", cumulative)
	}
}

func openTestPool(t *testing.T) *sql.DB {
	t.Helper()
	pool, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	if _, err := pool.ExecContext(context.Background(), `CREATE TABLE probes(value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	return pool
}

type capturingObserver struct {
	mu         sync.Mutex
	samples    []TransactionSample
	statements []StatementSample
	rows       []RowsEvent
}

func (o *capturingObserver) ObserveStatement(sample StatementSample) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.statements = append(o.statements, sample)
}

func (o *capturingObserver) ObserveRows(event RowsEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rows = append(o.rows, event)
}

func (o *capturingObserver) ObserveTransaction(sample TransactionSample) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.samples = append(o.samples, sample)
}

func (o *capturingObserver) transactionCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.samples)
}

func (o *capturingObserver) transaction(t *testing.T, operation Operation) TransactionSample {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, sample := range o.samples {
		if sample.Operation == operation {
			return sample
		}
	}
	t.Fatalf("transaction %s not observed", operation)
	return TransactionSample{}
}

func (o *capturingObserver) statement(t *testing.T, operation Operation) StatementSample {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	var matches []StatementSample
	for _, sample := range o.statements {
		if sample.Operation == operation {
			matches = append(matches, sample)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("statement samples for %s = %d, want 1", operation, len(matches))
	}
	return matches[0]
}

func (o *capturingObserver) findStatement(operation Operation) (StatementSample, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, sample := range o.statements {
		if sample.Operation == operation {
			return sample, true
		}
	}
	return StatementSample{}, false
}

func (o *capturingObserver) rowCounts(operation Operation) (opened, closed int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, event := range o.rows {
		if event.Operation != operation {
			continue
		}
		if event.Opened {
			opened++
		} else {
			closed++
		}
	}
	return opened, closed
}

func (o *capturingObserver) single(t *testing.T) TransactionSample {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.samples) != 1 {
		t.Fatalf("samples = %d, want 1", len(o.samples))
	}
	return o.samples[0]
}
