package catalogtx

import (
	"context"
	"database/sql"
)

// Statements is the database/sql-shaped read surface of the query-only
// reader pool, for code that runs hand-written SQL without a named operation
// and for helpers that accept either a pool or a *sql.Tx. Each statement is
// observed under catalog.statement.unknown_read, the label the observed
// connector gives a raw reader statement, with its pool admission measured.
//
// It has no ExecContext and no writer form: a write is a named
// Writer.ExecContext statement or part of a named transaction.
type Statements struct {
	reader *Reader
}

// Statements returns the reader's unnamed statement surface.
func (r *Reader) Statements() Statements {
	return Statements{reader: r}
}

func (s Statements) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.reader.QueryContext(ctx, OperationCatalogUnknownReaderStatement, query, args...)
}

func (s Statements) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.reader.QueryRowContext(ctx, OperationCatalogUnknownReaderStatement, query, args...)
}
