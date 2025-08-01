package queue

import (
	"context"
	"runtime"
	"server/internal/processors"
	"server/internal/service"

	pb "server/proto"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupAssetQueue 返回已经注册好 handler、但还没 Start 的 AssetPayload 队列
func SetupAssetQueue(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	assetProcessor *processors.AssetProcessor,
) Queue[processors.AssetPayload] {
	q := NewRiverQueue[processors.AssetPayload](dbPool)

	// 注册 ProcessAsset handler
	q.RegisterWorker(
		JobTypeProcessAsset,
		WorkerOptions{Concurrency: runtime.NumCPU()},
		func(ctx context.Context, job Job[processors.AssetPayload]) error {
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
) Queue[processors.CLIPPayload] {
	q := NewRiverQueue[processors.CLIPPayload](dbPool)

	q.RegisterWorker(
		JobTypeCalcPHash,
		WorkerOptions{Concurrency: 1},
		func(ctx context.Context, job Job[processors.CLIPPayload]) error {
			payload := job.Payload()
			resp, err := mlService.ProcessImageForCLIP(&pb.ImageProcessRequest{
				ImageId:   payload.AssetID.String(),
				ImageData: payload.ImageData,
			})
			if err != nil {
				return err
			}
			return assetService.SaveNewEmbedding(ctx, payload.AssetID, resp.ImageFeatureVector)
		},
	)

	return q
}
