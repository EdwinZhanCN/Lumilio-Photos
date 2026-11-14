package service

import (
	"context"
	"database/sql"
)

const touch = `UPDATE assets SET entry_count = entry_count`

// Static runs a constant statement.
func Static(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, touch)
	return err
}
