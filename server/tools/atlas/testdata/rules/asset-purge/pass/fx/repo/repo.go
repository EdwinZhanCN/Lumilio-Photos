package repo

import (
	"context"
	"database/sql"
)

// Queries is the generated query set.
type Queries struct{ db *sql.DB }

const deleteAsset = `DELETE FROM assets WHERE id = ?`

// DeleteAsset is a generated query.
func (q *Queries) DeleteAsset(ctx context.Context, id int) error {
	_, err := q.db.ExecContext(ctx, deleteAsset, id)
	return err
}

const countAssets = `SELECT count(*) FROM assets`

// CountAssets is a generated query.
func (q *Queries) CountAssets(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, countAssets).Scan(&n)
	return n, err
}
