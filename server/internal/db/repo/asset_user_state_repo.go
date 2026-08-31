package repo

import (
	"context"
	"database/sql"

	"server/internal/db/catalogtx"
)

type writeTxRunner interface {
	Transact(context.Context, catalogtx.Operation, *sql.TxOptions, func(*sql.Tx) error) error
}

// MutateAssetRating applies a foreground user-state write under the named
// asset mutation operation so writer admission is observable independently
// from background catalog work.
func (q *Queries) MutateAssetRating(ctx context.Context, arg UpdateAssetRatingParams) error {
	return q.withAssetUserMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetRating(ctx, arg)
	})
}

// MutateAssetLike applies a foreground user-state write under the named asset
// mutation operation.
func (q *Queries) MutateAssetLike(ctx context.Context, arg UpdateAssetLikeParams) error {
	return q.withAssetUserMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetLike(ctx, arg)
	})
}

// MutateAssetRatingAndLike applies the combined foreground user-state write
// atomically under the named asset mutation operation.
func (q *Queries) MutateAssetRatingAndLike(ctx context.Context, arg UpdateAssetRatingAndLikeParams) error {
	return q.withAssetUserMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetRatingAndLike(ctx, arg)
	})
}

// MutateAssetDescription applies a foreground description edit under the
// named asset mutation operation.
func (q *Queries) MutateAssetDescription(ctx context.Context, arg UpdateAssetDescriptionParams) error {
	return q.withAssetUserMutation(ctx, func(queries *Queries) error {
		return queries.UpdateAssetDescription(ctx, arg)
	})
}

func (q *Queries) withAssetUserMutation(ctx context.Context, body func(*Queries) error) error {
	runner, ok := q.db.(writeTxRunner)
	if !ok {
		pool, isPool := q.db.(*sql.DB)
		if !isPool {
			return body(q)
		}
		runner = catalogtx.NewWriter(pool, nil)
	}

	return runner.Transact(ctx, catalogtx.OperationAssetUserStateMutate, nil, func(tx *sql.Tx) error {
		return body(q.WithTx(tx))
	})
}
