package queue

import (
	"context"
	"database/sql"
)

// Pending reads the queue's own table.
func Pending(ctx context.Context, db *sql.DB) (*sql.Rows, error) {
	return db.QueryContext(ctx, `SELECT id FROM river_job WHERE state = 'available'`)
}
