package catalogtx

import (
	"context"
	"database/sql"
)

// Begin is the sanctioned transaction boundary.
func Begin(ctx context.Context, db *sql.DB) (*sql.Tx, error) { return db.BeginTx(ctx, nil) }
