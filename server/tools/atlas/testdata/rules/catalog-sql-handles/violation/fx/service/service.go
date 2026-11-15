package service

import (
	"context"
	"database/sql"

	"fx/catalog"
)

// Holder declares the pool it should not hold.
type Holder struct{ pool *sql.DB }

// Stats reaches the pool through a field without declaring it.
func Stats(db *catalog.DB) sql.DBStats { return db.SQL.Stats() }

// Unwrap reaches the pool through the reader.
func Unwrap(ctx context.Context, reader *catalog.Reader) error {
	return reader.Pool().PingContext(ctx)
}
