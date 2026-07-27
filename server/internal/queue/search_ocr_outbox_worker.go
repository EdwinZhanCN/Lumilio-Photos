package queue

import (
	"context"
	"fmt"

	"server/internal/queue/jobs"
	"server/internal/search/bleveocr"

	"github.com/riverqueue/river"
)

type ProcessOCROutboxArgs = jobs.ProcessOCROutboxArgs

type ProcessOCROutboxWorker struct {
	river.WorkerDefaults[ProcessOCROutboxArgs]
	Writer *bleveocr.Writer
}

func (w *ProcessOCROutboxWorker) Work(ctx context.Context, _ *river.Job[ProcessOCROutboxArgs]) error {
	if w.Writer == nil {
		return fmt.Errorf("OCR outbox worker is not configured")
	}
	_, err := w.Writer.Drain(ctx, bleveocr.DefaultOutboxBatchSize, bleveocr.DefaultMaxDrainBatches)
	return err
}
