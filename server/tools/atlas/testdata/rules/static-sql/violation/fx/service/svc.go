package service

import (
	"context"
	"database/sql"
)

// Dynamic runs SQL chosen by its caller.
func Dynamic(ctx context.Context, db *sql.DB, query string) error {
	_, err := db.ExecContext(ctx, query)
	return err
}
