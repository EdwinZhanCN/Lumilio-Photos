package queue

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"server/internal/queue/jobs"
	"server/internal/service"
)

type ReindexAssetsArgs = jobs.ReindexAssetsArgs

type ReindexAssetsWorker struct {
	river.WorkerDefaults[ReindexAssetsArgs]
	IndexingService service.AssetIndexingService
}

func (w *ReindexAssetsWorker) Work(ctx context.Context, job *river.Job[ReindexAssetsArgs]) error {
	if w.IndexingService == nil {
		return fmt.Errorf("reindex assets worker not configured")
	}

	args := job.Args
	tasks := make([]service.AssetIndexingTask, 0, len(args.Tasks))
	for _, task := range args.Tasks {
		tasks = append(tasks, service.AssetIndexingTask(task))
	}

	return w.IndexingService.ProcessReindexAssets(ctx, service.ReindexAssetsInput{
		RepositoryID: args.RepositoryID,
		Tasks:        tasks,
		Limit:        args.Limit,
		MissingOnly:  args.MissingOnly,
	})
}
