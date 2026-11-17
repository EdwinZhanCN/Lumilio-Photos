package catalogtx

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"sync"
	"time"
)

// NewConnector wraps a concrete SQLite driver with per-catalog observation.
// Using sql.OpenDB with a connector avoids process-global observer state and
// lets writer and reader physical connections retain their immutable roles.
func NewConnector(base driver.Driver, dsn string, role Role, observer Observer) driver.Connector {
	return &observedConnector{base: base, dsn: dsn, role: role, observer: observer}
}

type observedConnector struct {
	base     driver.Driver
	dsn      string
	role     Role
	observer Observer
}

func (c *observedConnector) Connect(context.Context) (driver.Conn, error) {
	connection, err := c.base.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return newObservedConn(connection, c.role, c.observer), nil
}

func (c *observedConnector) Driver() driver.Driver {
	return &observedDriver{base: c.base, role: c.role, observer: c.observer}
}

type observedDriver struct {
	base     driver.Driver
	role     Role
	observer Observer
}

func (d *observedDriver) Open(dsn string) (driver.Conn, error) {
	connection, err := d.base.Open(dsn)
	if err != nil {
		return nil, err
	}
	return newObservedConn(connection, d.role, d.observer), nil
}

type observedConn struct {
	driver.Conn
	role     Role
	observer Observer

	mu            sync.Mutex
	inTransaction bool
}

func newObservedConn(connection driver.Conn, role Role, observer Observer) *observedConn {
	return &observedConn{Conn: connection, role: role, observer: observer}
}

// UnwrapDriverConnection removes any catalog observation wrappers. It is
// intended only for native SQLite boundaries such as Online Backup and the
// generated-statement readonly audit.
func UnwrapDriverConnection(connection any) any {
	for {
		unwrapper, ok := connection.(interface{ UnwrapDriverConnection() driver.Conn })
		if !ok {
			return connection
		}
		connection = unwrapper.UnwrapDriverConnection()
	}
}

func (c *observedConn) UnwrapDriverConnection() driver.Conn { return c.Conn }

func (c *observedConn) Prepare(query string) (driver.Stmt, error) {
	statement, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &observedStmt{Stmt: statement, connection: c, query: query}, nil
}

func (c *observedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var (
		statement driver.Stmt
		err       error
	)
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		statement, err = preparer.PrepareContext(ctx, query)
	} else {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		statement, err = c.Conn.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return &observedStmt{Stmt: statement, connection: c, query: query}, nil
}

func (c *observedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	span := c.startStatement(ctx, query)
	result, err := execer.ExecContext(ctx, query, args)
	span.finish(err, time.Now())
	return result, err
}

func (c *observedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	span := c.startStatement(ctx, query)
	executionStarted := time.Now()
	rows, err := queryer.QueryContext(ctx, query, args)
	returnedAt := time.Now()
	if err != nil {
		span.finish(err, returnedAt)
		return nil, err
	}
	if span == nil {
		return rows, nil
	}
	span.rowsOpened(returnedAt, returnedAt.Sub(executionStarted))
	return &observedRows{Rows: rows, span: span}, nil
}

func (c *observedConn) Begin() (driver.Tx, error) {
	return c.begin(context.Background(), driver.TxOptions{})
}

func (c *observedConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.begin(ctx, options)
}

func (c *observedConn) begin(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	startedAt := time.Now()
	var (
		transaction driver.Tx
		err         error
	)
	if beginner, ok := c.Conn.(driver.ConnBeginTx); ok {
		transaction, err = beginner.BeginTx(ctx, options)
	} else {
		if options.Isolation != driver.IsolationLevel(0) || options.ReadOnly {
			return nil, errors.New("catalog driver does not support requested transaction options")
		}
		transaction, err = c.Conn.Begin()
	}
	named := isNamedTransaction(ctx)
	if err != nil {
		if !named && c.role == RoleWriter {
			safeObserve(c.observer, TransactionSample{
				StartedAt:     startedAt,
				Operation:     OperationCatalogRawWriterTransaction,
				OperationName: OperationCatalogRawWriterTransaction.Name(),
				Role:          RoleWriter,
				Outcome:       OutcomeBeginFailed,
				Cancellation:  cancellationFrom(ctx, err),
				Total:         time.Since(startedAt),
			})
		}
		return nil, err
	}
	c.mu.Lock()
	c.inTransaction = true
	c.mu.Unlock()
	var sample *driverTransactionSample
	if !named && c.role == RoleWriter {
		sample = &driverTransactionSample{
			ctx:        ctx,
			observer:   c.observer,
			startedAt:  time.Now(),
			acquiredAt: time.Now(),
		}
	}
	return &observedTx{Tx: transaction, connection: c, sample: sample}, nil
}

func (c *observedConn) Ping(ctx context.Context) error {
	if pinger, ok := c.Conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *observedConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *observedConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *observedConn) startStatement(ctx context.Context, query string) *statementSpan {
	if explicit := statementSpanFrom(ctx); explicit != nil {
		explicit.acquire(time.Now())
		explicit.describeQuery(query)
		return explicit
	}
	c.mu.Lock()
	inTransaction := c.inTransaction
	c.mu.Unlock()
	if inTransaction {
		return nil
	}
	operation := OperationCatalogUnknownWriterStatement
	if c.role == RoleReader {
		operation = OperationCatalogUnknownReaderStatement
	}
	span := defaultStatementSpan(ctx, c.observer, operation, c.role)
	span.describeQuery(query)
	return span
}

type observedTx struct {
	driver.Tx
	connection *observedConn
	sample     *driverTransactionSample
	once       sync.Once
}

func (t *observedTx) Commit() error {
	err := t.Tx.Commit()
	t.finish(true, err)
	return err
}

func (t *observedTx) Rollback() error {
	err := t.Tx.Rollback()
	t.finish(false, err)
	return err
}

func (t *observedTx) finish(commit bool, err error) {
	t.once.Do(func() {
		t.connection.mu.Lock()
		t.connection.inTransaction = false
		t.connection.mu.Unlock()
		if t.sample != nil {
			t.sample.finish(commit, err)
		}
	})
}

type driverTransactionSample struct {
	ctx        context.Context
	observer   Observer
	startedAt  time.Time
	acquiredAt time.Time
}

func (s *driverTransactionSample) finish(commit bool, err error) {
	finishedAt := time.Now()
	outcome := OutcomeRolledBack
	if commit {
		if err == nil {
			outcome = OutcomeCommitted
		} else {
			outcome = OutcomeCommitFailed
		}
	} else if err != nil && !errors.Is(err, driver.ErrBadConn) {
		outcome = OutcomeRollbackFailed
	}
	safeObserve(s.observer, TransactionSample{
		StartedAt:     s.startedAt,
		Operation:     OperationCatalogRawWriterTransaction,
		OperationName: OperationCatalogRawWriterTransaction.Name(),
		Role:          RoleWriter,
		Outcome:       outcome,
		Cancellation:  cancellationFrom(s.ctx, err),
		Body:          finishedAt.Sub(s.acquiredAt),
		Total:         finishedAt.Sub(s.startedAt),
	})
}

type observedStmt struct {
	driver.Stmt
	connection *observedConn
	query      string
}

func (s *observedStmt) Exec(args []driver.Value) (driver.Result, error) {
	span := s.connection.startStatement(context.Background(), s.query)
	result, err := s.Stmt.Exec(args)
	span.finish(err, time.Now())
	return result, err
}

func (s *observedStmt) Query(args []driver.Value) (driver.Rows, error) {
	span := s.connection.startStatement(context.Background(), s.query)
	executionStarted := time.Now()
	rows, err := s.Stmt.Query(args)
	returnedAt := time.Now()
	if err != nil {
		span.finish(err, returnedAt)
		return nil, err
	}
	if span == nil {
		return rows, nil
	}
	span.rowsOpened(returnedAt, returnedAt.Sub(executionStarted))
	return &observedRows{Rows: rows, span: span}, nil
}

func (s *observedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := s.Stmt.(driver.StmtExecContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	span := s.connection.startStatement(ctx, s.query)
	result, err := execer.ExecContext(ctx, args)
	span.finish(err, time.Now())
	return result, err
}

func (s *observedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := s.Stmt.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	span := s.connection.startStatement(ctx, s.query)
	executionStarted := time.Now()
	rows, err := queryer.QueryContext(ctx, args)
	returnedAt := time.Now()
	if err != nil {
		span.finish(err, returnedAt)
		return nil, err
	}
	if span == nil {
		return rows, nil
	}
	span.rowsOpened(returnedAt, returnedAt.Sub(executionStarted))
	return &observedRows{Rows: rows, span: span}, nil
}

type observedRows struct {
	driver.Rows
	span *statementSpan

	mu       sync.Mutex
	closed   bool
	closeErr error
}

func (r *observedRows) Close() error {
	r.mu.Lock()
	if r.closed {
		err := r.closeErr
		r.mu.Unlock()
		return err
	}
	r.closed = true
	r.mu.Unlock()
	err := r.Rows.Close()
	r.mu.Lock()
	r.closeErr = err
	r.mu.Unlock()
	r.span.finish(err, time.Now())
	return err
}

func (r *observedRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err != nil && !errors.Is(err, io.EOF) {
		r.span.noteExecutionError(err)
	}
	return err
}

func (r *observedRows) HasNextResultSet() bool {
	if rows, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return rows.HasNextResultSet()
	}
	return false
}

func (r *observedRows) NextResultSet() error {
	if rows, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return rows.NextResultSet()
	}
	return io.EOF
}

func (r *observedRows) ColumnTypeScanType(index int) reflect.Type {
	if rows, ok := r.Rows.(driver.RowsColumnTypeScanType); ok {
		return rows.ColumnTypeScanType(index)
	}
	return reflect.TypeOf((*any)(nil)).Elem()
}

func (r *observedRows) ColumnTypeDatabaseTypeName(index int) string {
	if rows, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return rows.ColumnTypeDatabaseTypeName(index)
	}
	return ""
}

func (r *observedRows) ColumnTypeLength(index int) (int64, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypeLength); ok {
		return rows.ColumnTypeLength(index)
	}
	return 0, false
}

func (r *observedRows) ColumnTypeNullable(index int) (bool, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return rows.ColumnTypeNullable(index)
	}
	return false, false
}

func (r *observedRows) ColumnTypePrecisionScale(index int) (int64, int64, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypePrecisionScale); ok {
		return rows.ColumnTypePrecisionScale(index)
	}
	return 0, 0, false
}
