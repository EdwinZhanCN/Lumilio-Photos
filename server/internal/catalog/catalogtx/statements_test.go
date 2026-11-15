package catalogtx

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/mattn/go-sqlite3"
)

func TestStatementsReadOnTheReaderUnderTheUnknownReadLabel(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
	capture := &capturingObserver{}
	driver := &sqlite3.SQLiteDriver{}

	writerPool := sql.OpenDB(NewConnector(driver, dsn, RoleWriter, capture))
	writerPool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = writerPool.Close() })
	if _, err := NewWriter(writerPool, capture).ExecContext(context.Background(), OperationCatalogGeneratedWriterExec, `CREATE TABLE statement_probes(value INTEGER NOT NULL)`); err != nil {
		t.Fatalf("create statement probe: %v", err)
	}

	readerPool := sql.OpenDB(NewConnector(driver, dsn, RoleReader, capture))
	readerPool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = readerPool.Close() })
	statements := NewReader(readerPool, capture).Statements()

	var count int
	if err := statements.QueryRowContext(context.Background(), `SELECT count(*) FROM statement_probes`).Scan(&count); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if sample := capture.statement(t, OperationCatalogUnknownReaderStatement); sample.Role != RoleReader || sample.Outcome != StatementOutcomeSucceeded {
		t.Fatalf("statement role/outcome = %v/%v, want reader/succeeded", sample.Role, sample.Outcome)
	}

	rows, err := statements.QueryContext(context.Background(), `SELECT value FROM statement_probes`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close rows: %v", err)
	}
}

func TestStatementsOfANilReaderFailWithoutAPool(t *testing.T) {
	var reader *Reader
	if _, err := reader.Statements().QueryContext(context.Background(), `SELECT 1`); !errors.Is(err, ErrNilPool) {
		t.Fatalf("QueryContext error = %v, want ErrNilPool", err)
	}
}
