// Package outbox drains revision-bound ROE effects in bounded at-least-once
// batches. Delivery occurs outside catalog transactions and consumers must use
// the entity plus expected revision as their idempotency/CAS boundary.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

const (
	defaultBatchSize   = 32
	maximumBatchSize   = 64
	defaultLease       = 30 * time.Second
	defaultMaxAttempts = 12
)

type Config struct {
	BatchSize   int64
	Lease       time.Duration
	MaxAttempts int64
}

type Delivery func(context.Context, repo.RepositoryOutbox) error

type Result struct {
	Claimed   int
	Delivered int
	Retrying  int
	Dead      int
	HasMore   bool
}

type Drainer struct {
	database *db.DB
	cfg      Config
	now      func() time.Time
}

func New(database *db.DB, cfg Config) *Drainer {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.BatchSize > maximumBatchSize {
		cfg.BatchSize = maximumBatchSize
	}
	if cfg.Lease <= 0 {
		cfg.Lease = defaultLease
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultMaxAttempts
	}
	return &Drainer{database: database, cfg: cfg, now: func() time.Time { return time.Now().UTC() }}
}

func (d *Drainer) DrainKind(ctx context.Context, effectKind string, deliver Delivery) (Result, error) {
	result := Result{}
	if d == nil || d.database == nil {
		return result, errors.New("repository outbox drainer unavailable")
	}
	if deliver == nil {
		return result, errors.New("repository outbox delivery is required")
	}
	lease := fmt.Sprintf("outbox:%d", d.now().UnixNano())
	nowTime := d.now()
	expiresAt := nowTime.Add(d.cfg.Lease).UnixMicro()
	effects, err := d.database.Queries.ClaimRepositoryOutboxBatch(ctx, repo.ClaimRepositoryOutboxBatchParams{
		LeaseID: &lease, LeaseExpiresAt: &expiresAt,
		UpdatedAt: dbtypes.NewTimestamp(nowTime), EffectKind: effectKind, Limit: d.cfg.BatchSize,
	})
	if err != nil {
		return result, fmt.Errorf("claim repository outbox: %w", err)
	}
	result.Claimed = len(effects)
	for _, effect := range effects {
		deliveryErr := deliver(ctx, effect)
		status := "delivered"
		var failureCode *string
		if deliveryErr != nil {
			code := "delivery_failed"
			failureCode = &code
			status = "pending"
			result.Retrying++
			if effect.AttemptCount >= d.cfg.MaxAttempts {
				status = "dead"
				result.Retrying--
				result.Dead++
			}
		} else {
			result.Delivered++
		}
		rows, completeErr := d.database.Queries.CompleteRepositoryOutboxEffect(ctx, repo.CompleteRepositoryOutboxEffectParams{
			OutboxID: effect.OutboxID, LeaseID: &lease, Status: status,
			LastFailureCode: failureCode, UpdatedAt: dbtypes.NewTimestamp(d.now()),
		})
		if completeErr != nil {
			return result, fmt.Errorf("complete repository outbox effect: %w", completeErr)
		}
		if rows != 1 {
			return result, fmt.Errorf("repository outbox lease lost for %s", effect.OutboxID)
		}
	}
	// A full page may have a follower page. A delivery returned to pending must
	// also remain on the same durable River job even when this page was short.
	// At worst, an exactly-full final page causes one empty bounded turn.
	result.HasMore = outboxHasMore(result.Claimed, d.cfg.BatchSize, result.Retrying)
	return result, nil
}

func outboxHasMore(claimed int, batchSize int64, retrying int) bool {
	return int64(claimed) == batchSize || retrying > 0
}
