package db

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"server/internal/db/catalogtx"
	"server/internal/db/repo"
)

// queryRouter is the default non-transactional sqlc surface. sqlc emits reads
// through QueryContext/QueryRowContext and mutations through ExecContext or a
// DML ... RETURNING query. Routing here keeps existing domain services honest:
// ordinary reads cannot consume the catalog's sole writer connection, while
// every mutation and unknown statement conservatively remains on the writer.
// Queries rebound with repo.Queries.WithTx bypass this router and stay on the
// explicit transaction connection.
type queryRouter struct {
	writerPool       *sql.DB
	writer           *catalogtx.Writer
	reader           *sql.DB
	readerCapability *catalogtx.Reader
}

var _ repo.DBTX = (*queryRouter)(nil)

func newQueryRouter(writerPool, reader *sql.DB, writer *catalogtx.Writer, readerCapability *catalogtx.Reader) *queryRouter {
	return &queryRouter{writerPool: writerPool, writer: writer, reader: reader, readerCapability: readerCapability}
}

func (r *queryRouter) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return r.writer.ExecContext(ctx, catalogtx.OperationCatalogGeneratedWriterExec, query, args...)
}

func (r *queryRouter) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return r.pool(query).PrepareContext(ctx, query)
}

func (r *queryRouter) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if sqliteStatementReadOnly(query) {
		return r.readerCapability.QueryContext(ctx, catalogtx.OperationCatalogGeneratedReaderRows, query, args...)
	}
	return r.writer.QueryContext(ctx, catalogtx.OperationCatalogGeneratedWriterReturning, query, args...)
}

func (r *queryRouter) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if sqliteStatementReadOnly(query) {
		return r.readerCapability.QueryRowContext(ctx, catalogtx.OperationCatalogGeneratedReaderRows, query, args...)
	}
	return r.writer.QueryRowContext(ctx, catalogtx.OperationCatalogGeneratedWriterReturning, query, args...)
}

// Transact is intentionally outside repo.DBTX but is discovered by the
// repository's hand-written atomic helpers (for example MutateAssetRating).
func (r *queryRouter) Transact(ctx context.Context, operation catalogtx.Operation, options *sql.TxOptions, body func(*sql.Tx) error) error {
	return r.writer.Transact(ctx, operation, options, body)
}

func (r *queryRouter) pool(query string) *sql.DB {
	if sqliteStatementReadOnly(query) {
		return r.reader
	}
	return r.writerPool
}

func sqliteStatementReadOnly(query string) bool {
	keyword := sqliteStatementKeyword(query)
	switch keyword {
	case "SELECT", "VALUES", "EXPLAIN":
		return true
	default:
		// PRAGMA is deliberately not treated as a read verb: SQLite has both
		// observational and mutating PRAGMAs, and the router cannot classify
		// that second grammar safely without preparing the statement. Generated
		// sqlc reads do not depend on PRAGMA, so fail closed to the writer.
		return false
	}
}

// sqliteStatementKeyword returns the top-level statement verb while ignoring
// sqlc's leading comments and any CTE bodies. SQLite CTE bodies are SELECT-only;
// the verb following their balanced parentheses determines whether the whole
// statement reads or mutates. Unknown or malformed input returns an empty verb
// and therefore routes to the writer.
func sqliteStatementKeyword(query string) string {
	scanner := sqlTokenScanner{source: query}
	first := scanner.next()
	if first == "" {
		return ""
	}
	if first == "EXPLAIN" {
		return first
	}
	if first != "WITH" {
		return first
	}

	depth := 0
	for {
		token := scanner.next()
		if token == "" {
			return ""
		}
		switch token {
		case "(":
			depth++
		case ")":
			if depth == 0 {
				return ""
			}
			depth--
		default:
			if depth == 0 {
				switch token {
				case "SELECT", "VALUES", "INSERT", "UPDATE", "DELETE", "REPLACE":
					return token
				}
			}
		}
	}
}

type sqlTokenScanner struct {
	source string
	offset int
}

func (s *sqlTokenScanner) next() string {
	for s.offset < len(s.source) {
		current := s.source[s.offset]
		switch {
		case unicode.IsSpace(rune(current)):
			s.offset++
		case current == '-' && s.offset+1 < len(s.source) && s.source[s.offset+1] == '-':
			s.offset += 2
			for s.offset < len(s.source) && s.source[s.offset] != '\n' {
				s.offset++
			}
		case current == '/' && s.offset+1 < len(s.source) && s.source[s.offset+1] == '*':
			s.offset += 2
			for s.offset+1 < len(s.source) && !(s.source[s.offset] == '*' && s.source[s.offset+1] == '/') {
				s.offset++
			}
			if s.offset+1 >= len(s.source) {
				s.offset = len(s.source)
				return ""
			}
			s.offset += 2
		case current == '\'' || current == '"' || current == '`' || current == '[':
			s.skipQuoted(current)
		case current == '(' || current == ')' || current == ',':
			s.offset++
			return string(current)
		case isSQLIdentifierByte(current):
			start := s.offset
			for s.offset < len(s.source) && isSQLIdentifierByte(s.source[s.offset]) {
				s.offset++
			}
			return strings.ToUpper(s.source[start:s.offset])
		default:
			s.offset++
		}
	}
	return ""
}

func (s *sqlTokenScanner) skipQuoted(open byte) {
	close := open
	if open == '[' {
		close = ']'
	}
	s.offset++
	for s.offset < len(s.source) {
		if s.source[s.offset] != close {
			s.offset++
			continue
		}
		if close != ']' && s.offset+1 < len(s.source) && s.source[s.offset+1] == close {
			s.offset += 2
			continue
		}
		s.offset++
		return
	}
}

func isSQLIdentifierByte(value byte) bool {
	return value == '_' || value == '$' || value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
