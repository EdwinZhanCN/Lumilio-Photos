package commit

import (
	"context"
	"database/sql"

	"server/internal/db/catalogtx"
	"server/internal/db/repo"
)

// ScanWriter submits the scan index's write transactions through the
// coordinator, each in its own catalog transaction. The scan sizes its batches
// to the writer budget, so coordinator batching would only lengthen holds.
type ScanWriter struct {
	coordinator *Coordinator
}

func (c *Coordinator) ScanWriter() ScanWriter { return ScanWriter{coordinator: c} }

// WithTx runs fn in a coordinator transaction. The operation name is the
// caller's label; the coordinator records its own batch operation.
func (w ScanWriter) WithTx(ctx context.Context, _ catalogtx.Operation, fn func(*sql.Tx, *repo.Queries) error) error {
	_, err := w.coordinator.SubmitOperation(ctx, Operation{
		Kind: OperationKindRepositoryScan, BatchLimit: 1,
		Apply: func(ctx context.Context, tx *sql.Tx) (Result, error) {
			if err := fn(tx, repo.New(tx)); err != nil {
				return Result{}, err
			}
			return Result{Outcome: OutcomeApplied}, nil
		},
	})
	return err
}
