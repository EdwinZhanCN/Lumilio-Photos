package catalogtx

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// StatementOutcome is deliberately smaller than transaction Outcome: an
// individual statement either completed or failed, while commit/rollback are
// properties of the enclosing transaction.
type StatementOutcome uint8

const (
	StatementOutcomeUnknown StatementOutcome = iota
	StatementOutcomeSucceeded
	StatementOutcomeFailed
)

func (o StatementOutcome) String() string {
	switch o {
	case StatementOutcomeSucceeded:
		return "succeeded"
	case StatementOutcomeFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func (o StatementOutcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// StatementSample separates database/sql admission from driver execution and
// cursor lifetime. A query's Total ends when its driver.Rows closes, so a held
// reader is visible instead of looking like a fast QueryContext call.
type StatementSample struct {
	StartedAt     time.Time `json:"started_at"`
	Operation     Operation `json:"-"`
	OperationName string    `json:"operation"`
	// QueryName and QueryFingerprint are diagnostic-only identifiers. They are
	// never retained by Recorder or used as metric keys. QueryName is accepted
	// only from a bounded sqlc-style leading comment and QueryFingerprint hashes
	// SQL text without arguments, so a failed third-party statement can be
	// located without logging query parameters or arbitrary SQL.
	QueryName        string           `json:"-"`
	QueryFingerprint string           `json:"-"`
	Role             Role             `json:"role"`
	Outcome          StatementOutcome `json:"outcome"`
	Cancellation     Cancellation     `json:"cancellation"`
	Admission        time.Duration    `json:"admission_ns"`
	Execution        time.Duration    `json:"execution_ns"`
	RowsObserved     bool             `json:"rows_observed"`
	RowsLifetime     time.Duration    `json:"rows_lifetime_ns"`
	Total            time.Duration    `json:"total_ns"`
}

// StatementObserver is optional. Transaction-only observers remain valid and
// JoinObservers forwards statement samples only to implementations that opt in.
type StatementObserver interface {
	ObserveStatement(StatementSample)
}

// RowsEvent changes the bounded open-cursor gauge for one static operation.
// It carries no SQL text, parameters, or entity identifiers.
type RowsEvent struct {
	At            time.Time `json:"at"`
	Operation     Operation `json:"-"`
	OperationName string    `json:"operation"`
	Role          Role      `json:"role"`
	Opened        bool      `json:"opened"`
}

type RowsObserver interface {
	ObserveRows(RowsEvent)
}

type statementContextKey struct{}
type namedTransactionContextKey struct{}

// statementSpan is shared between the caller-side capability and the
// driver-side wrapper. Its start precedes database/sql pool admission; the
// driver marks acquisition after database/sql has selected a physical
// connection.
type statementSpan struct {
	ctx       context.Context
	observer  Observer
	operation Operation
	role      Role
	startedAt time.Time

	mu               sync.Mutex
	acquiredAt       time.Time
	rowsOpenedAt     time.Time
	execution        time.Duration
	rowsOpen         bool
	finished         bool
	lastExecutionErr error
	queryName        string
	queryFingerprint string
}

func newStatementContext(ctx context.Context, observer Observer, operation Operation, role Role) (context.Context, *statementSpan, error) {
	descriptor, ok := operation.Descriptor()
	if !ok || descriptor.Kind != OperationKindStatement || descriptor.Role != role {
		return ctx, nil, ErrInvalidOperation
	}
	span := &statementSpan{
		ctx:       ctx,
		observer:  observer,
		operation: operation,
		role:      role,
		startedAt: time.Now(),
	}
	return context.WithValue(ctx, statementContextKey{}, span), span, nil
}

func defaultStatementSpan(ctx context.Context, observer Observer, operation Operation, role Role) *statementSpan {
	now := time.Now()
	span := &statementSpan{
		ctx:        ctx,
		observer:   observer,
		operation:  operation,
		role:       role,
		startedAt:  now,
		acquiredAt: now,
	}
	return span
}

func statementSpanFrom(ctx context.Context) *statementSpan {
	if ctx == nil {
		return nil
	}
	span, _ := ctx.Value(statementContextKey{}).(*statementSpan)
	return span
}

func withNamedTransaction(ctx context.Context) context.Context {
	return context.WithValue(ctx, namedTransactionContextKey{}, true)
}

func isNamedTransaction(ctx context.Context) bool {
	value, _ := ctx.Value(namedTransactionContextKey{}).(bool)
	return value
}

func (s *statementSpan) acquire(at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || !s.acquiredAt.IsZero() {
		return
	}
	s.acquiredAt = at
}

func (s *statementSpan) describeQuery(query string) {
	if s == nil || query == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queryFingerprint != "" {
		return
	}
	digest := sha256.Sum256([]byte(query))
	s.queryFingerprint = hex.EncodeToString(digest[:8])
	s.queryName = sqlcQueryName(query)
}

func sqlcQueryName(query string) string {
	const prefix = "-- name:"
	trimmed := strings.TrimLeft(query, " \t\r\n")
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)))
	if len(fields) < 2 || fields[0] == "" || !strings.HasPrefix(fields[1], ":") || len(fields[0]) > 96 {
		return ""
	}
	for _, char := range fields[0] {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' {
			continue
		}
		return ""
	}
	return fields[0]
}

func (s *statementSpan) rowsOpened(at time.Time, execution time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.finished || s.rowsOpen {
		s.mu.Unlock()
		return
	}
	if s.acquiredAt.IsZero() {
		s.acquiredAt = at
	}
	s.execution = execution
	s.rowsOpenedAt = at
	s.rowsOpen = true
	event := s.rowsEvent(at, true)
	s.mu.Unlock()
	safeObserveRows(s.observer, event)
}

func (s *statementSpan) noteExecutionError(err error) {
	if s == nil || err == nil || errors.Is(err, sql.ErrNoRows) {
		return
	}
	s.mu.Lock()
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) || s.lastExecutionErr == nil {
		s.lastExecutionErr = err
	}
	s.mu.Unlock()
}

func (s *statementSpan) finish(err error, at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	if err == nil {
		err = s.lastExecutionErr
	}
	if s.acquiredAt.IsZero() {
		// database/sql rejected or canceled admission before the driver saw
		// the context. The complete elapsed time is therefore admission.
		s.acquiredAt = at
	}
	if s.execution == 0 && !s.rowsOpen {
		s.execution = at.Sub(s.acquiredAt)
	}
	rowsLifetime := time.Duration(0)
	rowsObserved := false
	var closeEvent *RowsEvent
	if s.rowsOpen {
		rowsObserved = true
		rowsLifetime = at.Sub(s.rowsOpenedAt)
		event := s.rowsEvent(at, false)
		closeEvent = &event
		s.rowsOpen = false
	}
	outcome := StatementOutcomeSucceeded
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		outcome = StatementOutcomeFailed
	}
	sample := StatementSample{
		StartedAt:        s.startedAt,
		Operation:        s.operation,
		OperationName:    s.operation.Name(),
		QueryName:        s.queryName,
		QueryFingerprint: s.queryFingerprint,
		Role:             s.role,
		Outcome:          outcome,
		Cancellation:     cancellationFrom(s.ctx, err),
		Admission:        s.acquiredAt.Sub(s.startedAt),
		Execution:        s.execution,
		RowsObserved:     rowsObserved,
		RowsLifetime:     rowsLifetime,
		Total:            at.Sub(s.startedAt),
	}
	s.finished = true
	s.mu.Unlock()
	if closeEvent != nil {
		safeObserveRows(s.observer, *closeEvent)
	}
	safeObserveStatement(s.observer, sample)
}

func (s *statementSpan) finishIfUnacquired(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	unacquired := s.acquiredAt.IsZero() && !s.finished
	s.mu.Unlock()
	if unacquired {
		s.finish(err, time.Now())
	}
}

func (s *statementSpan) rowsEvent(at time.Time, opened bool) RowsEvent {
	return RowsEvent{
		At:            at,
		Operation:     s.operation,
		OperationName: s.operation.Name(),
		Role:          s.role,
		Opened:        opened,
	}
}

func safeObserveStatement(observer Observer, sample StatementSample) {
	statementObserver, ok := observer.(StatementObserver)
	if !ok {
		return
	}
	defer func() { _ = recover() }()
	statementObserver.ObserveStatement(sample)
}

func safeObserveRows(observer Observer, event RowsEvent) {
	rowsObserver, ok := observer.(RowsObserver)
	if !ok {
		return
	}
	defer func() { _ = recover() }()
	rowsObserver.ObserveRows(event)
}

// ExecContext observes a standalone writer statement with exact pool
// admission when the pool uses NewConnector.
func (w *Writer) ExecContext(ctx context.Context, operation Operation, query string, args ...any) (sql.Result, error) {
	if w == nil || w.pool == nil {
		return nil, ErrNilPool
	}
	observed, span, err := newStatementContext(ctx, w.observer, operation, RoleWriter)
	if err != nil {
		return nil, err
	}
	result, execErr := w.pool.ExecContext(observed, query, args...)
	span.finishIfUnacquired(execErr)
	return result, execErr
}

func (w *Writer) QueryContext(ctx context.Context, operation Operation, query string, args ...any) (*sql.Rows, error) {
	if w == nil || w.pool == nil {
		return nil, ErrNilPool
	}
	observed, span, err := newStatementContext(ctx, w.observer, operation, RoleWriter)
	if err != nil {
		return nil, err
	}
	rows, queryErr := w.pool.QueryContext(observed, query, args...)
	span.finishIfUnacquired(queryErr)
	return rows, queryErr
}

func (w *Writer) QueryRowContext(ctx context.Context, operation Operation, query string, args ...any) *sql.Row {
	if w == nil || w.pool == nil {
		panic(ErrNilPool)
	}
	observed, span, err := newStatementContext(ctx, w.observer, operation, RoleWriter)
	if err != nil {
		panic(err)
	}
	row := w.pool.QueryRowContext(observed, query, args...)
	span.finishIfUnacquired(row.Err())
	return row
}

func (r *Reader) QueryContext(ctx context.Context, operation Operation, query string, args ...any) (*sql.Rows, error) {
	if r == nil || r.pool == nil {
		return nil, ErrNilPool
	}
	observed, span, err := newStatementContext(ctx, r.observer, operation, RoleReader)
	if err != nil {
		return nil, err
	}
	rows, queryErr := r.pool.QueryContext(observed, query, args...)
	span.finishIfUnacquired(queryErr)
	return rows, queryErr
}

func (r *Reader) QueryRowContext(ctx context.Context, operation Operation, query string, args ...any) *sql.Row {
	if r == nil || r.pool == nil {
		panic(ErrNilPool)
	}
	observed, span, err := newStatementContext(ctx, r.observer, operation, RoleReader)
	if err != nil {
		panic(err)
	}
	row := r.pool.QueryRowContext(observed, query, args...)
	span.finishIfUnacquired(row.Err())
	return row
}
