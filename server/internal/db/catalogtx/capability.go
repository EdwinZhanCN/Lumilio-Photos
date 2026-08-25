package catalogtx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrInvalidOperation means the caller supplied an identifier outside the
	// static catalog or attempted to use it on the wrong connection role.
	ErrInvalidOperation = errors.New("invalid catalog transaction operation")
	ErrNilPool          = errors.New("nil catalog transaction pool")
	ErrNilBody          = errors.New("nil catalog transaction body")
)

// Outcome is a bounded terminal transaction state. Cancellation is recorded
// separately because it can occur during admission, the body, or commit.
type Outcome uint8

const (
	OutcomeUnknown Outcome = iota
	OutcomeCommitted
	OutcomeBeginFailed
	OutcomeRolledBack
	OutcomeRollbackFailed
	OutcomeCommitFailed
	OutcomePanicked
)

func (o Outcome) String() string {
	switch o {
	case OutcomeCommitted:
		return "committed"
	case OutcomeBeginFailed:
		return "begin_failed"
	case OutcomeRolledBack:
		return "rolled_back"
	case OutcomeRollbackFailed:
		return "rollback_failed"
	case OutcomeCommitFailed:
		return "commit_failed"
	case OutcomePanicked:
		return "panicked"
	default:
		return "unknown"
	}
}

func (o Outcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// Cancellation preserves whether a terminal sample was canceled explicitly or
// reached its deadline. It is never derived later from survivor-only data.
type Cancellation uint8

const (
	CancellationNone Cancellation = iota
	CancellationCanceled
	CancellationDeadlineExceeded
)

func (c Cancellation) String() string {
	switch c {
	case CancellationCanceled:
		return "canceled"
	case CancellationDeadlineExceeded:
		return "deadline_exceeded"
	default:
		return "none"
	}
}

func (c Cancellation) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// TransactionSample contains the complete measured lifetime of one named
// transaction. Admission is BeginTx call-to-return and therefore includes
// database/sql pool waiting. Body excludes finalization; Total includes it.
type TransactionSample struct {
	StartedAt     time.Time     `json:"started_at"`
	Operation     Operation     `json:"-"`
	OperationName string        `json:"operation"`
	Role          Role          `json:"role"`
	Outcome       Outcome       `json:"outcome"`
	Cancellation  Cancellation  `json:"cancellation"`
	Admission     time.Duration `json:"admission_ns"`
	Body          time.Duration `json:"body_ns"`
	Commit        time.Duration `json:"commit_ns"`
	Total         time.Duration `json:"total_ns"`
}

// Observer consumes already bounded transaction samples. Implementations must
// return promptly; the capability contains observer panics so telemetry cannot
// change a durable transaction result after commit.
type Observer interface {
	ObserveTransaction(TransactionSample)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(TransactionSample)

func (f ObserverFunc) ObserveTransaction(sample TransactionSample) { f(sample) }

// JoinObservers fans each sample out in declaration order. A faulty observer
// is isolated from both the transaction and the remaining observers.
func JoinObservers(observers ...Observer) Observer {
	filtered := make([]Observer, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return observerGroup(filtered)
}

type observerGroup []Observer

func (group observerGroup) ObserveTransaction(sample TransactionSample) {
	for _, observer := range group {
		safeObserve(observer, sample)
	}
}

func (group observerGroup) ObserveStatement(sample StatementSample) {
	for _, observer := range group {
		safeObserveStatement(observer, sample)
	}
}

func (group observerGroup) ObserveRows(event RowsEvent) {
	for _, observer := range group {
		safeObserveRows(observer, event)
	}
}

// Writer serializes named application transactions through the catalog's sole
// database/sql writer pool. It does not own or close that pool.
type Writer struct {
	pool     *sql.DB
	observer Observer
}

func NewWriter(pool *sql.DB, observer Observer) *Writer {
	return &Writer{pool: pool, observer: observer}
}

// Pool returns the physical writer for boundaries that require *sql.DB, such
// as River. Application transactions should use Transact.
func (w *Writer) Pool() *sql.DB {
	if w == nil {
		return nil
	}
	return w.pool
}

// BeginTx acquires a named writer transaction. The returned wrapper owns the
// only legal Commit/Rollback path so admission and terminal timing cannot be
// omitted by manual transaction scopes.
func (w *Writer) BeginTx(ctx context.Context, operation Operation, options *sql.TxOptions) (*Tx, error) {
	if w == nil || w.pool == nil {
		return nil, ErrNilPool
	}
	if err := validateOperation(operation, RoleWriter); err != nil {
		return nil, err
	}
	if options != nil && options.ReadOnly {
		return nil, fmt.Errorf("%w: writer operation %s requested a read-only transaction", ErrInvalidOperation, operation)
	}
	return begin(ctx, w.pool, w.observer, operation, options)
}

// Transact runs body once and owns commit/rollback. Body must not finalize tx.
func (w *Writer) Transact(
	ctx context.Context,
	operation Operation,
	options *sql.TxOptions,
	body func(*sql.Tx) error,
) error {
	if body == nil {
		return ErrNilBody
	}
	tx, err := w.BeginTx(ctx, operation, options)
	if err != nil {
		return err
	}
	return runBody(tx, body)
}

// Reader owns short, consistent snapshots on the query-only WAL pool. It does
// not own or close that pool.
type Reader struct {
	pool     *sql.DB
	observer Observer
}

func NewReader(pool *sql.DB, observer Observer) *Reader {
	return &Reader{pool: pool, observer: observer}
}

func (r *Reader) Pool() *sql.DB {
	if r == nil {
		return nil
	}
	return r.pool
}

// BeginTx acquires a named read-only snapshot on the query-only pool.
func (r *Reader) BeginTx(ctx context.Context, operation Operation) (*Tx, error) {
	if r == nil || r.pool == nil {
		return nil, ErrNilPool
	}
	if err := validateOperation(operation, RoleReader); err != nil {
		return nil, err
	}
	return begin(ctx, r.pool, r.observer, operation, &sql.TxOptions{ReadOnly: true})
}

// Snapshot executes a bounded read-only transaction and closes it before
// returning. Cursor ownership remains with body and every Rows must be closed.
func (r *Reader) Snapshot(ctx context.Context, operation Operation, body func(*sql.Tx) error) error {
	if body == nil {
		return ErrNilBody
	}
	tx, err := r.BeginTx(ctx, operation)
	if err != nil {
		return err
	}
	return runBody(tx, body)
}

func validateOperation(operation Operation, role Role) error {
	descriptor, ok := operation.Descriptor()
	if !ok || descriptor.Role != role || descriptor.Kind != OperationKindApplicationTransaction {
		return fmt.Errorf("%w: operation=%s role=%s", ErrInvalidOperation, operation, role)
	}
	return nil
}

// Tx is a measured transaction wrapper. Raw exposes the concrete *sql.Tx only
// for sqlc and River APIs; callers must finalize through this wrapper.
type Tx struct {
	raw        *sql.Tx
	ctx        context.Context
	observer   Observer
	sample     TransactionSample
	acquiredAt time.Time

	mu       sync.Mutex
	finished bool
}

func (t *Tx) Raw() *sql.Tx {
	if t == nil {
		return nil
	}
	return t.raw
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.raw.ExecContext(ctx, query, args...)
}

func (t *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return t.raw.PrepareContext(ctx, query)
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.raw.QueryContext(ctx, query, args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.raw.QueryRowContext(ctx, query, args...)
}

func (t *Tx) Commit() error {
	return t.finish(OutcomeCommitted, true)
}

func (t *Tx) Rollback() error {
	return t.finish(OutcomeRolledBack, false)
}

func (t *Tx) rollbackPanicked() error {
	return t.finish(OutcomePanicked, false)
}

func (t *Tx) finish(requested Outcome, commit bool) error {
	if t == nil || t.raw == nil {
		return sql.ErrTxDone
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.finished {
		return sql.ErrTxDone
	}
	finalizeStartedAt := time.Now()
	t.sample.Body = finalizeStartedAt.Sub(t.acquiredAt)
	var err error
	if commit {
		err = t.raw.Commit()
		t.sample.Commit = time.Since(finalizeStartedAt)
		if err != nil {
			t.sample.Outcome = OutcomeCommitFailed
		} else {
			t.sample.Outcome = OutcomeCommitted
		}
	} else {
		err = t.raw.Rollback()
		t.sample.Outcome = requested
		if err != nil && !errors.Is(err, sql.ErrTxDone) && requested != OutcomePanicked {
			t.sample.Outcome = OutcomeRollbackFailed
		}
	}
	t.sample.Cancellation = cancellationFrom(t.ctx, err)
	t.sample.Total = time.Since(t.sample.StartedAt)
	t.finished = true
	safeObserve(t.observer, t.sample)
	return err
}

func begin(
	ctx context.Context,
	pool *sql.DB,
	observer Observer,
	operation Operation,
	options *sql.TxOptions,
) (*Tx, error) {
	startedAt := time.Now()
	sample := TransactionSample{
		StartedAt:     startedAt,
		Operation:     operation,
		OperationName: operation.Name(),
		Role:          operation.Role(),
	}

	raw, err := pool.BeginTx(withNamedTransaction(ctx), options)
	acquiredAt := time.Now()
	sample.Admission = acquiredAt.Sub(startedAt)
	if err != nil {
		sample.Outcome = OutcomeBeginFailed
		sample.Cancellation = cancellationFrom(ctx, err)
		sample.Total = time.Since(startedAt)
		safeObserve(observer, sample)
		return nil, fmt.Errorf("begin SQLite %s transaction: %w", operation, err)
	}
	return &Tx{
		raw:        raw,
		ctx:        ctx,
		observer:   observer,
		sample:     sample,
		acquiredAt: acquiredAt,
	}, nil
}

func runBody(tx *Tx, body func(*sql.Tx) error) (resultErr error) {
	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
		}()
		resultErr = body(tx.Raw())
	}()

	if panicValue != nil {
		_ = tx.rollbackPanicked()
		panic(panicValue)
	}

	if resultErr != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback SQLite %s transaction: %w", tx.sample.Operation, rollbackErr))
		}
		return resultErr
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite %s transaction: %w", tx.sample.Operation, err)
	}
	return nil
}

func cancellationFrom(ctx context.Context, err error) Cancellation {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(context.Cause(ctx), context.DeadlineExceeded):
		return CancellationDeadlineExceeded
	case errors.Is(err, context.Canceled), errors.Is(context.Cause(ctx), context.Canceled):
		return CancellationCanceled
	default:
		return CancellationNone
	}
}

func safeObserve(observer Observer, sample TransactionSample) {
	if observer == nil {
		return
	}
	defer func() { _ = recover() }()
	observer.ObserveTransaction(sample)
}
