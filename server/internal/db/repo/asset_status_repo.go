package repo

import (
	"context"
	"database/sql"
	"fmt"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"

	"github.com/google/uuid"
)

const getAssetStatusForUpdate = `
SELECT status
FROM assets
WHERE asset_id = ?1 AND is_deleted = false
`

type writeTxRunner interface {
	Transact(context.Context, catalogtx.Operation, *sql.TxOptions, func(*sql.Tx) error) error
}

// MutateAssetStatus applies a status mutation under a transaction so concurrent
// workers do not clobber each other's updates.
func (q *Queries) MutateAssetStatus(
	ctx context.Context,
	assetID uuid.UUID,
	mutator func(statusdb.AssetStatus) (statusdb.AssetStatus, error),
) error {
	return q.withAssetStatusMutation(ctx, func(queries *Queries) error {
		return q.mutateAssetStatus(ctx, queries, assetID, mutator)
	})
}

// MutateAssetRating applies a foreground user-state write under the named
// asset mutation operation so writer admission is observable independently
// from background catalog work.
func (q *Queries) MutateAssetRating(ctx context.Context, arg UpdateAssetRatingParams) error {
	return q.withAssetStatusMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetRating(ctx, arg)
	})
}

// MutateAssetLike applies a foreground user-state write under the named asset
// mutation operation.
func (q *Queries) MutateAssetLike(ctx context.Context, arg UpdateAssetLikeParams) error {
	return q.withAssetStatusMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetLike(ctx, arg)
	})
}

// MutateAssetRatingAndLike applies the combined foreground user-state write
// atomically under the named asset mutation operation.
func (q *Queries) MutateAssetRatingAndLike(ctx context.Context, arg UpdateAssetRatingAndLikeParams) error {
	return q.withAssetStatusMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetRatingAndLike(ctx, arg)
	})
}

// MutateAssetDescription applies a foreground description edit under the
// named asset mutation operation.
func (q *Queries) MutateAssetDescription(ctx context.Context, arg UpdateAssetDescriptionParams) error {
	return q.withAssetStatusMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetDescription(ctx, arg)
	})
}

func (q *Queries) withAssetStatusMutation(ctx context.Context, body func(*Queries) error) error {
	runner, ok := q.db.(writeTxRunner)
	if !ok {
		pool, isPool := q.db.(*sql.DB)
		if !isPool {
			return body(q)
		}
		runner = catalogtx.NewWriter(pool, nil)
	}

	return runner.Transact(ctx, catalogtx.OperationAssetStatusMutate, nil, func(tx *sql.Tx) error {
		return body(q.WithTx(tx))
	})
}

func (q *Queries) mutateAssetStatus(
	ctx context.Context,
	queries *Queries,
	assetID uuid.UUID,
	mutator func(statusdb.AssetStatus) (statusdb.AssetStatus, error),
) error {
	var rawStatus []byte
	if err := queries.db.QueryRowContext(ctx, getAssetStatusForUpdate, assetID).Scan(&rawStatus); err != nil {
		return fmt.Errorf("lock asset status: %w", err)
	}

	var current statusdb.AssetStatus
	if len(rawStatus) > 0 {
		var err error
		current, err = statusdb.FromJSON(rawStatus)
		if err != nil {
			return fmt.Errorf("parse asset status: %w", err)
		}
	}

	updated, err := mutator(current)
	if err != nil {
		return err
	}

	statusJSON, err := updated.ToJSON()
	if err != nil {
		return fmt.Errorf("marshal asset status: %w", err)
	}

	if _, err := queries.UpdateAssetStatus(ctx, UpdateAssetStatusParams{
		AssetID: assetID,
		Status:  dbtypes.JSON(statusJSON),
	}); err != nil {
		return fmt.Errorf("persist asset status: %w", err)
	}

	return nil
}
