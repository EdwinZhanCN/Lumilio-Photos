package service

import (
	"context"
	"database/sql"

	"fx/repo"
)

// Remove deletes an asset directly.
func Remove(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "DELETE FROM assets WHERE id = ?", 1)
	return err
}

// ViaQuery deletes through the generated query.
func ViaQuery(ctx context.Context, q *repo.Queries) error { return q.DeleteAsset(ctx, 1) }
