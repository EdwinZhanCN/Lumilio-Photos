package queuesetup

import (
	"context"
	"runtime"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/service"

	pb "server/proto"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupAssetQueue 返回已经注册好 handler、但还没 Start 的 AssetPayload 队列
func SetupAssetQueue(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	assetProcessor *processors.AssetProcessor,
) queue.Queue[processors.AssetPayload] {
	q := queue.NewRiverQueue[processors.AssetPayload](dbPool)

	// 注册 ProcessAsset handler
	q.RegisterWorker(
		string(queue.JobTypeProcessAsset),
		queue.WorkerOptions{Concurrency: runtime.NumCPU()},
		func(ctx context.Context, job queue.Job[processors.AssetPayload]) error {
			_, err := assetProcessor.ProcessAsset(ctx, job.Payload())
			return err
		},
	)

	return q
}

// SetupCLIPQueue 返回已经注册好 handler、但还没 Start 的 CLIPPayload 队列
func SetupCLIPQueue(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	mlService service.MLService,
	assetService service.AssetService,
) queue.Queue[processors.CLIPPayload] {
	q := queue.NewRiverQueue[processors.CLIPPayload](dbPool)

	q.RegisterWorker(
		string(queue.JobCLIPProcess),
		queue.WorkerOptions{Concurrency: 1},
		func(ctx context.Context, job queue.Job[processors.CLIPPayload]) error {
			payload := job.Payload()
			resp, err := mlService.ProcessImageForCLIP(&pb.ImageProcessRequest{
				ImageId:   payload.AssetID.String(),
				ImageData: payload.ImageData,
			})
			if err != nil {
				return err
			}
			guuid, err := uuid.FromBytes(payload.AssetID.Bytes[:])
			if err != nil {
				return err
			}
			err = assetService.SaveNewEmbedding(ctx, guuid, resp.ImageFeatureVector)
			return err
		},
	)

	return q
}
