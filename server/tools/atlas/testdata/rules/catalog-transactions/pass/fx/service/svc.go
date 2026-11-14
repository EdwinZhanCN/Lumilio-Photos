package service

import (
	"context"
	"database/sql"

	"fx/catalogtx"
)

// Use goes through the sanctioned boundary.
func Use(ctx context.Context, db *sql.DB) { _, _ = catalogtx.Begin(ctx, db) }
