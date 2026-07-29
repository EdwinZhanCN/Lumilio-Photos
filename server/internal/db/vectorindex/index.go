// Package vectorindex owns the rebuildable Vec1 semantic index policy.
package vectorindex

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"runtime"
	"time"
)

const (
	ModeFlat = "flat"
	ModeANN  = "ann"

	annRowThreshold = int64(5_000)
	maxTrainingRows = int64(100_000)
	minANNBuckets   = 16
	maxANNBuckets   = 4_096
	annCodeSize     = 48
)

const flatConfig = `{"index":"flat","distance":"l2"}`

type state struct {
	mode            string
	rowCount        int64
	trainedRowCount int64
	rebuildPending  bool
}

type trainingConfig struct {
	Distance  string `json:"distance"`
	CodeSize  int    `json:"codesize"`
	Buckets   int    `json:"nbucket"`
	Quantizer string `json:"quantizer"`
	Residual  bool   `json:"residual"`
	Threads   int    `json:"nthread"`
}

// CurrentMode returns the persisted mode of the derived semantic index.
func CurrentMode(ctx context.Context, database *sql.DB) (string, error) {
	var mode string
	if err := database.QueryRowContext(ctx, `
		SELECT mode
		FROM semantic_vector_index_state
		WHERE id = 1
	`).Scan(&mode); err != nil {
		return "", fmt.Errorf("read semantic Vec1 mode: %w", err)
	}
	if mode != ModeFlat && mode != ModeANN {
		return "", fmt.Errorf("invalid semantic Vec1 mode %q", mode)
	}
	return mode, nil
}

// Reconcile verifies authoritative/derived row parity at startup, repairs the
// Vec1 table if needed, then applies the flat/ANN size policy.
func Reconcile(ctx context.Context, database *sql.DB) error {
	var authoritativeRows, derivedRows int64
	if err := database.QueryRowContext(ctx, `
		SELECT
			(SELECT count(*) FROM search_embeddings),
			(SELECT count(*) FROM search_embeddings_vec)
	`).Scan(&authoritativeRows, &derivedRows); err != nil {
		return fmt.Errorf("count semantic Vec1 rows: %w", err)
	}

	current, err := readState(ctx, database)
	if err != nil {
		return err
	}
	if authoritativeRows != derivedRows {
		if err := repairDerivedRows(ctx, database, authoritativeRows); err != nil {
			return err
		}
		current.rebuildPending = true
	} else if current.rowCount != authoritativeRows {
		if _, err := database.ExecContext(ctx, `
			UPDATE semantic_vector_index_state
			SET row_count = ?, rebuild_pending = 1
			WHERE id = 1
		`, authoritativeRows); err != nil {
			return fmt.Errorf("repair semantic Vec1 row count: %w", err)
		}
		current.rebuildPending = true
	}
	return Maintain(ctx, database)
}

// Maintain applies a pending policy transition. It is intentionally cheap
// when no transition is due, so embedding writes can call it after commit.
func Maintain(ctx context.Context, database *sql.DB) error {
	current, err := readState(ctx, database)
	if err != nil {
		return err
	}
	if !current.rebuildPending {
		return nil
	}

	switch {
	case current.rowCount < annRowThreshold:
		if current.mode == ModeFlat {
			return clearPending(ctx, database)
		}
		return rebuildFlat(ctx, database, current.rowCount)

	case current.mode == ModeFlat:
		return trainWithFlatFallback(ctx, database, current.rowCount)

	case current.trainedRowCount <= 0,
		current.rowCount >= current.trainedRowCount*2,
		current.rowCount*2 < current.trainedRowCount:
		return trainWithFlatFallback(ctx, database, current.rowCount)

	default:
		// A transient delete+insert replacement may have set pending while
		// leaving the final population inside the current ANN training band.
		return clearPending(ctx, database)
	}
}

func readState(ctx context.Context, database *sql.DB) (state, error) {
	var current state
	if err := database.QueryRowContext(ctx, `
		SELECT mode, row_count, trained_row_count, rebuild_pending
		FROM semantic_vector_index_state
		WHERE id = 1
	`).Scan(
		&current.mode,
		&current.rowCount,
		&current.trainedRowCount,
		&current.rebuildPending,
	); err != nil {
		return state{}, fmt.Errorf("read semantic Vec1 state: %w", err)
	}
	return current, nil
}

func repairDerivedRows(ctx context.Context, database *sql.DB, rowCount int64) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin semantic Vec1 repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM search_embeddings_vec"); err != nil {
		return fmt.Errorf("clear semantic Vec1 rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO search_embeddings_vec (
			rowid, embedding, space_id, owner_id, is_deleted, asset_type
		)
		SELECT
			e.id, e.vector, e.space_id, a.owner_id, a.is_deleted, a.type
		FROM search_embeddings e
		JOIN assets a ON a.asset_id = e.asset_id
	`); err != nil {
		return fmt.Errorf("backfill semantic Vec1 rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE semantic_vector_index_state
		SET row_count = ?, rebuild_pending = 1
		WHERE id = 1
	`, rowCount); err != nil {
		return fmt.Errorf("record semantic Vec1 repair: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit semantic Vec1 repair: %w", err)
	}
	return nil
}

func clearPending(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		UPDATE semantic_vector_index_state
		SET rebuild_pending = 0
		WHERE id = 1
	`); err != nil {
		return fmt.Errorf("clear semantic Vec1 rebuild request: %w", err)
	}
	return nil
}

func trainWithFlatFallback(ctx context.Context, database *sql.DB, rowCount int64) error {
	if err := trainANN(ctx, database, rowCount); err == nil {
		return nil
	} else {
		log.Printf(
			"Vec1 ANN training failed; falling back to exact flat index: rows=%d error=%v",
			rowCount,
			err,
		)
		if flatErr := rebuildFlat(ctx, database, rowCount); flatErr != nil {
			return errors.Join(err, flatErr)
		}
		return nil
	}
}

func rebuildFlat(ctx context.Context, database *sql.DB, rowCount int64) error {
	started := time.Now()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin semantic Vec1 flat rebuild: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO search_embeddings_vec (cmd, arg)
		VALUES ('rebuild', ?)
	`, flatConfig); err != nil {
		return fmt.Errorf("rebuild semantic Vec1 flat index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE semantic_vector_index_state
		SET mode = 'flat',
		    row_count = ?,
		    trained_row_count = 0,
		    rebuild_pending = 0,
		    config = ?,
		    updated_at = ?
		WHERE id = 1
	`, rowCount, flatConfig, time.Now().UTC().UnixMicro()); err != nil {
		return fmt.Errorf("record semantic Vec1 flat index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit semantic Vec1 flat rebuild: %w", err)
	}
	log.Printf("Vec1 semantic index ready: mode=flat rows=%d duration=%s", rowCount, time.Since(started))
	return nil
}

func trainANN(ctx context.Context, database *sql.DB, rowCount int64) error {
	started := time.Now()
	sampleStep := max(int64(1), (rowCount+maxTrainingRows-1)/maxTrainingRows)
	sampleRows := (rowCount + sampleStep - 1) / sampleStep
	buckets := annBuckets(rowCount, sampleRows)
	threads := min(max(runtime.GOMAXPROCS(0), 1), 8)
	config := trainingConfig{
		Distance:  "l2",
		CodeSize:  annCodeSize,
		Buckets:   buckets,
		Quantizer: "pq",
		Residual:  true,
		Threads:   threads,
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode semantic Vec1 training config: %w", err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin semantic Vec1 ANN training: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var configuredThreads int
	if err := tx.QueryRowContext(
		ctx,
		"SELECT vec1_config('nthread', ?)",
		threads,
	).Scan(&configuredThreads); err != nil {
		return fmt.Errorf("configure semantic Vec1 threads: %w", err)
	}

	var model []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT vec1_train(vector, ?)
		FROM (
			SELECT vector
			FROM (
				SELECT
					vector,
					row_number() OVER (ORDER BY id) AS sample_row
				FROM search_embeddings
			)
			WHERE (sample_row - 1) % ? = 0
			LIMIT ?
		)
	`, string(configJSON), sampleStep, maxTrainingRows).Scan(&model); err != nil {
		return fmt.Errorf("train semantic Vec1 ANN model: %w", err)
	}
	if len(model) == 0 {
		return fmt.Errorf("train semantic Vec1 ANN model: empty model")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO search_embeddings_vec (cmd, arg)
		VALUES ('rebuild', ?)
	`, model); err != nil {
		return fmt.Errorf("install semantic Vec1 ANN model: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE semantic_vector_index_state
		SET mode = 'ann',
		    row_count = ?,
		    trained_row_count = ?,
		    rebuild_pending = 0,
		    config = ?,
		    updated_at = ?
		WHERE id = 1
	`, rowCount, rowCount, string(configJSON), time.Now().UTC().UnixMicro()); err != nil {
		return fmt.Errorf("record semantic Vec1 ANN model: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit semantic Vec1 ANN training: %w", err)
	}

	log.Printf(
		"Vec1 semantic index ready: mode=ann rows=%d samples=%d buckets=%d codesize=%d threads=%d duration=%s",
		rowCount,
		sampleRows,
		buckets,
		annCodeSize,
		configuredThreads,
		time.Since(started),
	)
	return nil
}

func annBuckets(rowCount, sampleRows int64) int {
	buckets := int(math.Sqrt(float64(rowCount)))
	buckets = max(buckets, minANNBuckets)
	buckets = min(buckets, maxANNBuckets)
	if maxForSamples := int(sampleRows / 4); buckets > maxForSamples {
		buckets = maxForSamples
	}
	return max(buckets, 2)
}
