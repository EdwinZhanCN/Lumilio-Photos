package catalog

import (
	"context"
	"database/sql"
)

// DB is the open catalog. SQL is its pool, for the catalog's own packages.
type DB struct {
	SQL    *sql.DB
	Reader *Reader
}

// Reader is the narrow read handle code above the catalog holds.
type Reader struct{ pool *sql.DB }

// Count runs one read.
func (r *Reader) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRowContext(ctx, `SELECT 1`).Scan(&n)
	return n, err
}

// Pool returns the reader's pool.
func (r *Reader) Pool() *sql.DB { return r.pool }
