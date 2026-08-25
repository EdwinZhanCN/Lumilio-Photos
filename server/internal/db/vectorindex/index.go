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
	"sync"
	"time"

	"server/internal/db/catalogtx"
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

var (
	errTrainingSnapshotChanged = errors.New("semantic Vec1 training snapshot changed")
	maintenanceMu              sync.Mutex
)

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
func Reconcile(ctx context.Context, writer *catalogtx.Writer, reader *sql.DB) error {
	var authoritativeRows, derivedRows int64
	if err := reader.QueryRowContext(ctx, `
		SELECT
			(SELECT count(*) FROM search_embeddings),
			(SELECT count(*) FROM search_embeddings_vec)
	`).Scan(&authoritativeRows, &derivedRows); err != nil {
		return fmt.Errorf("count semantic Vec1 rows: %w", err)
	}

	current, err := readState(ctx, reader)
	if err != nil {
		return err
	}
	if authoritativeRows != derivedRows {
		if err := repairDerivedRows(ctx, writer, authoritativeRows); err != nil {
			return err
		}
		current.rebuildPending = true
	} else if current.rowCount != authoritativeRows {
		if _, err := writer.ExecContext(ctx, catalogtx.OperationVectorStateRepair, `
			UPDATE semantic_vector_index_state
			SET row_count = ?, rebuild_pending = 1
			WHERE id = 1
		`, authoritativeRows); err != nil {
			return fmt.Errorf("repair semantic Vec1 row count: %w", err)
		}
		current.rebuildPending = true
	}
	return Maintain(ctx, writer, reader)
}

// Maintain applies a pending policy transition. It is intentionally cheap
// when no transition is due, so embedding writes can call it after commit.
func Maintain(ctx context.Context, writer *catalogtx.Writer, reader *sql.DB) error {
	current, err := readState(ctx, reader)
	if err != nil {
		return err
	}
	if !current.rebuildPending {
		return nil
	}

	// ANN training is deliberately outside the SQLite writer transaction, but
	// only one process-local maintainer should spend CPU building a model at a
	// time. Re-read after acquiring the gate because another caller may already
	// have completed the transition.
	maintenanceMu.Lock()
	defer maintenanceMu.Unlock()
	current, err = readState(ctx, reader)
	if err != nil {
		return err
	}
	if !current.rebuildPending {
		return nil
	}

	switch {
	case current.rowCount < annRowThreshold:
		if current.mode == ModeFlat {
			return clearPending(ctx, writer)
		}
		return rebuildFlat(ctx, writer, current.rowCount)

	case current.mode == ModeFlat:
		return trainWithFlatFallback(ctx, writer, reader, current.rowCount)

	case current.trainedRowCount <= 0,
		current.rowCount >= current.trainedRowCount*2,
		current.rowCount*2 < current.trainedRowCount:
		return trainWithFlatFallback(ctx, writer, reader, current.rowCount)

	default:
		// A transient delete+insert replacement may have set pending while
		// leaving the final population inside the current ANN training band.
		return clearPending(ctx, writer)
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

func repairDerivedRows(ctx context.Context, writer *catalogtx.Writer, rowCount int64) error {
	tx, err := writer.BeginTx(ctx, catalogtx.OperationVectorRepairDerived, nil)
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

func clearPending(ctx context.Context, writer *catalogtx.Writer) error {
	if _, err := writer.ExecContext(ctx, catalogtx.OperationVectorClearPending, `
		UPDATE semantic_vector_index_state
		SET rebuild_pending = 0
		WHERE id = 1
	`); err != nil {
		return fmt.Errorf("clear semantic Vec1 rebuild request: %w", err)
	}
	return nil
}

func trainWithFlatFallback(ctx context.Context, writer *catalogtx.Writer, reader *sql.DB, rowCount int64) error {
	if err := trainANN(ctx, writer, reader, rowCount); err == nil {
		return nil
	} else if errors.Is(err, errTrainingSnapshotChanged) {
		// Inserts/deletes raced the reader-side training snapshot. The trigger
		// leaves rebuild_pending set, so a later maintainer can retry without
		// installing a model trained for an obsolete population.
		return nil
	} else {
		log.Printf(
			"Vec1 ANN training failed; falling back to exact flat index: rows=%d error=%v",
			rowCount,
			err,
		)
		if flatErr := rebuildFlat(ctx, writer, rowCount); flatErr != nil {
			return errors.Join(err, flatErr)
		}
		return nil
	}
}

func rebuildFlat(ctx context.Context, writer *catalogtx.Writer, rowCount int64) error {
	started := time.Now()
	tx, err := writer.BeginTx(ctx, catalogtx.OperationVectorRebuildFlat, nil)
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

func trainANN(ctx context.Context, writer *catalogtx.Writer, reader *sql.DB, rowCount int64) error {
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

	// vec1_config is connection-local, so configure and train through one
	// query-only reader connection. This is the expensive part (up to 100k
	// vectors) and must not consume the process's sole writer connection.
	readerConn, err := reader.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire semantic Vec1 training reader: %w", err)
	}
	defer readerConn.Close()
	var configuredThreads int
	if err := readerConn.QueryRowContext(
		ctx,
		"SELECT vec1_config('nthread', ?)",
		threads,
	).Scan(&configuredThreads); err != nil {
		return fmt.Errorf("configure semantic Vec1 threads: %w", err)
	}

	var model []byte
	if err := readerConn.QueryRowContext(ctx, `
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
	if err := readerConn.Close(); err != nil {
		return fmt.Errorf("release semantic Vec1 training reader: %w", err)
	}

	// Installing a trained model mutates the Vec1 virtual table and therefore
	// belongs on the writer. The state check makes the reader-side snapshot a
	// compare-and-swap boundary: never publish a model after the authoritative
	// population changed underneath its training query.
	tx, err := writer.BeginTx(ctx, catalogtx.OperationVectorTrainANN, nil)
	if err != nil {
		return fmt.Errorf("begin semantic Vec1 ANN install: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var currentRows int64
	var rebuildPending bool
	if err := tx.QueryRowContext(ctx, `
		SELECT row_count, rebuild_pending
		FROM semantic_vector_index_state
		WHERE id = 1
	`).Scan(&currentRows, &rebuildPending); err != nil {
		return fmt.Errorf("validate semantic Vec1 training snapshot: %w", err)
	}
	if currentRows != rowCount || !rebuildPending {
		return fmt.Errorf(
			"%w: trained rows=%d current rows=%d pending=%t",
			errTrainingSnapshotChanged,
			rowCount,
			currentRows,
			rebuildPending,
		)
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
		return fmt.Errorf("commit semantic Vec1 ANN install: %w", err)
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
