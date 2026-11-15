package catalog

import (
	"context"
	"database/sql"
)

// Statements is the unnamed statement surface over one pool.
type Statements struct{ pool *sql.DB }

// QueryRowContext forwards its caller's SQL, which is not counted here.
func (s Statements) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.pool.QueryRowContext(ctx, query, args...)
}

// Version is the catalog's own SQL, outside the rule's scope.
func Version(ctx context.Context, pool *sql.DB) (int, error) {
	var v int
	err := pool.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v)
	return v, err
}
