package service

import (
	"context"
	"database/sql"
)

// Peek consults the queue for product correctness.
func Peek(ctx context.Context, db *sql.DB) error {
	var n int
	return db.QueryRowContext(ctx, `SELECT count(*) FROM assets a, river_job j WHERE j.id = a.id`).Scan(&n)
}

// Requeue writes the queue's table.
func Requeue(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `UPDATE river_job SET state = 'available'`)
	return err
}
