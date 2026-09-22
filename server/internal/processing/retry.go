package processing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/pipeline"
	"server/internal/workqos"

	"github.com/google/uuid"
)

// MaxRetryBatch bounds how many failed subjects one retry call re-requests.
const MaxRetryBatch = 500

// ErrStageNotRetryable reports a stage without a whole-stage retry.
var ErrStageNotRetryable = errors.New("processing stage is not retryable")

// RetryResult reports what one retry call re-requested and what is left.
type RetryResult struct {
	Accepted  int64   `json:"accepted"`
	Remaining int64   `json:"remaining"`
	ReceiptID *string `json:"receipt_id,omitempty"`
}

// Retrier re-requests a stage's failed subjects through the Catalog request
// path the scheduler already derives work from. It never inserts River jobs.
type Retrier struct {
	writer *catalogtx.Writer
}

// NewRetrier binds the Catalog writer.
func NewRetrier(writer *catalogtx.Writer) *Retrier {
	return &Retrier{writer: writer}
}

// Retry re-requests at most MaxRetryBatch failed subjects of one stage.
func (r *Retrier) Retry(ctx context.Context, id StageID) (RetryResult, error) {
	spec, ok := LookupStage(id)
	if !ok {
		return RetryResult{}, ErrUnknownStage
	}
	if !spec.Retryable {
		return RetryResult{}, ErrStageNotRetryable
	}
	if r == nil || r.writer == nil {
		return RetryResult{}, errors.New("processing retrier is not configured")
	}
	var result RetryResult
	err := r.writer.Transact(ctx, catalogtx.OperationProcessingStageRetry, nil, func(tx *sql.Tx) error {
		var err error
		switch {
		case spec.AssetStage != "":
			result, err = retryAssetStage(ctx, tx, spec)
		case id == StageEvents:
			result, err = retryEvents(ctx, tx)
		case id == StagePlaces:
			result, err = retryPlaces(ctx, tx)
		case id == StageTextSearch:
			result, err = retryTextSearch(ctx, tx)
		default:
			err = ErrStageNotRetryable
		}
		return err
	})
	return result, err
}

func retryAssetStage(ctx context.Context, tx *sql.Tx, spec StageSpec) (RetryResult, error) {
	rows, err := tx.QueryContext(ctx, `
 SELECT s.asset_id, a.content_id FROM asset_pipeline_state s
 JOIN assets a ON a.asset_id=s.asset_id AND a.is_deleted=0
 WHERE s.stage=? AND s.terminal_error IS NOT NULL
 ORDER BY s.updated_at, s.asset_id LIMIT ?`, string(spec.AssetStage), MaxRetryBatch)
	if err != nil {
		return RetryResult{}, fmt.Errorf("select failed %s rows: %w", spec.ID, err)
	}
	type target struct{ asset, content uuid.UUID }
	var targets []target
	for rows.Next() {
		var assetRaw, contentRaw string
		if err := rows.Scan(&assetRaw, &contentRaw); err != nil {
			rows.Close()
			return RetryResult{}, err
		}
		assetID, err := uuid.Parse(assetRaw)
		if err != nil {
			rows.Close()
			return RetryResult{}, err
		}
		contentID, err := uuid.Parse(contentRaw)
		if err != nil {
			rows.Close()
			return RetryResult{}, err
		}
		targets = append(targets, target{assetID, contentID})
	}
	if err := rows.Close(); err != nil {
		return RetryResult{}, err
	}
	if len(targets) == 0 {
		return RetryResult{}, nil
	}

	// One receipt names the operation; each re-requested stage binds to it, so
	// the Analysis card can attribute the work to a retry.
	receiptID := uuid.New()
	now := time.Now().UTC().UnixMicro()
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts (receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES (?,'retry',?,1,'pending',?,?)`,
		receiptID.String(), "stage:"+string(spec.ID), now, now); err != nil {
		return RetryResult{}, fmt.Errorf("create retry receipt: %w", err)
	}
	for _, target := range targets {
		if err := pipeline.RequestAssetStagesTx(ctx, tx, target.asset, target.content, []pipeline.Stage{spec.AssetStage}, pipeline.AssetPipelineVersion, workqos.Interactive, receiptID); err != nil {
			return RetryResult{}, err
		}
	}
	var remaining int64
	if err := tx.QueryRowContext(ctx, `
 SELECT count(*) FROM asset_pipeline_state s JOIN assets a ON a.asset_id=s.asset_id AND a.is_deleted=0
 WHERE s.stage=? AND s.terminal_error IS NOT NULL`, string(spec.AssetStage)).Scan(&remaining); err != nil {
		return RetryResult{}, err
	}
	id := receiptID.String()
	return RetryResult{Accepted: int64(len(targets)), Remaining: remaining, ReceiptID: &id}, nil
}

func retryEvents(ctx context.Context, tx *sql.Tx) (RetryResult, error) {
	rows, err := tx.QueryContext(ctx, `SELECT owner_id, source_revision FROM event_projection_pipeline_state WHERE terminal_error IS NOT NULL LIMIT ?`, MaxRetryBatch)
	if err != nil {
		return RetryResult{}, err
	}
	type target struct {
		owner    int32
		revision uint64
	}
	var targets []target
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.owner, &item.revision); err != nil {
			rows.Close()
			return RetryResult{}, err
		}
		targets = append(targets, item)
	}
	if err := rows.Close(); err != nil {
		return RetryResult{}, err
	}
	for _, item := range targets {
		if err := pipeline.RequestEventProjectionTx(ctx, tx, item.owner, item.revision, false); err != nil {
			return RetryResult{}, err
		}
	}
	return RetryResult{Accepted: int64(len(targets))}, nil
}

func retryPlaces(ctx context.Context, tx *sql.Tx) (RetryResult, error) {
	rows, err := tx.QueryContext(ctx, `SELECT repository_id, owner_id FROM location_projection_state WHERE terminal_error IS NOT NULL LIMIT ?`, MaxRetryBatch)
	if err != nil {
		return RetryResult{}, err
	}
	type target struct {
		repository uuid.UUID
		owner      int32
	}
	var targets []target
	for rows.Next() {
		var repositoryRaw string
		var item target
		if err := rows.Scan(&repositoryRaw, &item.owner); err != nil {
			rows.Close()
			return RetryResult{}, err
		}
		if item.repository, err = uuid.Parse(repositoryRaw); err != nil {
			rows.Close()
			return RetryResult{}, err
		}
		targets = append(targets, item)
	}
	if err := rows.Close(); err != nil {
		return RetryResult{}, err
	}
	for _, item := range targets {
		if err := pipeline.RequestLocationProjectionTx(ctx, tx, item.repository, item.owner); err != nil {
			return RetryResult{}, err
		}
	}
	accepted := int64(len(targets))
	var resolutionRevision sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT source_revision FROM location_resolution_pipeline_state WHERE terminal_error IS NOT NULL`).Scan(&resolutionRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return RetryResult{}, err
	}
	if resolutionRevision.Valid {
		if err := pipeline.RequestLocationResolutionTx(ctx, tx, uint64(resolutionRevision.Int64)); err != nil {
			return RetryResult{}, err
		}
		accepted++
	}
	return RetryResult{Accepted: accepted}, nil
}

// retryTextSearch clears the terminal outcome; the projection is still behind
// its version, so the scheduler derives the work again.
func retryTextSearch(ctx context.Context, tx *sql.Tx) (RetryResult, error) {
	result, err := tx.ExecContext(ctx, `UPDATE ocr_projection_pipeline_state SET terminal_error=NULL, updated_at=? WHERE terminal_error IS NOT NULL`, time.Now().UTC().UnixMicro())
	if err != nil {
		return RetryResult{}, err
	}
	accepted, err := result.RowsAffected()
	return RetryResult{Accepted: accepted}, err
}
