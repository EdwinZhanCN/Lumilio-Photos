package repo

import (
	"context"
	"fmt"

	statusdb "server/internal/db/dbtypes/status"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const getAssetStatusForUpdate = `
SELECT status
FROM assets
WHERE asset_id = $1 AND is_deleted = false
FOR UPDATE
`

type txStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

// MutateAssetStatus applies a status mutation under a transaction so concurrent
// workers do not clobber each other's updates.
func (q *Queries) MutateAssetStatus(
	ctx context.Context,
	assetID pgtype.UUID,
	mutator func(statusdb.AssetStatus) (statusdb.AssetStatus, error),
) error {
	starter, ok := q.db.(txStarter)
	if !ok {
		return q.mutateAssetStatus(ctx, q, assetID, mutator)
	}

	tx, err := starter.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin asset status tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := q.mutateAssetStatus(ctx, q.WithTx(tx), assetID, mutator); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit asset status tx: %w", err)
	}
	return nil
}

func (q *Queries) mutateAssetStatus(
	ctx context.Context,
	queries *Queries,
	assetID pgtype.UUID,
	mutator func(statusdb.AssetStatus) (statusdb.AssetStatus, error),
) error {
	var rawStatus []byte
	if err := queries.db.QueryRow(ctx, getAssetStatusForUpdate, assetID).Scan(&rawStatus); err != nil {
		return fmt.Errorf("lock asset status: %w", err)
	}

	var current statusdb.AssetStatus
	if len(rawStatus) > 0 {
		var err error
		current, err = statusdb.FromJSONB(rawStatus)
		if err != nil {
			return fmt.Errorf("parse asset status: %w", err)
		}
	}

	updated, err := mutator(current)
	if err != nil {
		return err
	}

	statusJSON, err := updated.ToJSONB()
	if err != nil {
		return fmt.Errorf("marshal asset status: %w", err)
	}

	if _, err := queries.UpdateAssetStatus(ctx, UpdateAssetStatusParams{
		AssetID: assetID,
		Status:  statusJSON,
	}); err != nil {
		return fmt.Errorf("persist asset status: %w", err)
	}

	return nil
}
