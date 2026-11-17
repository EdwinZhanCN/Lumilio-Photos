package fresh

import (
	"context"

	"fx/catalog"
)

// One is new hand-written SQL in a package without a ceiling.
func One(ctx context.Context, db catalog.Statements) error {
	return db.QueryRowContext(ctx, `SELECT 1`).Err()
}
