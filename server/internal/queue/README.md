# RiverQueue

## Create Worker

## Add New Queue to Setup

## Migration Commands

Export Your PostgreSQL Database Connection String

```shell
export DATABASE_URL="connstring"
```

Migrate Up (Include in internal/db/migration.go)

```shell
river migrate-up --line main --database-url "$DATABASE_URL"
```

Migrate Down

```shell
river migrate-down --line main --database-url "$DATABASE_URL" --max-steps 10
```


# Lumilio Photos — Upload Pipeline and Queue (River) README

This document explains the end-to-end photo upload procedures, the queue topology, and a deep dive into how the CLIP batch dispatcher works in this service.

Sections:
- Overview
- End-to-end upload flow (single and batch)
- Queue topology and workers
- CLIP processing: worker and batch dispatcher
- Configuration knobs
- How to add a new queue/worker
- Migrations (River)
- Troubleshooting

## Overview

Core components involved:
- HTTP handlers: parse incoming uploads and enqueue background work.
- River queues: reliable background processing.
- Asset processor worker: consumes `process_asset` jobs and performs ingestion.
- CLIP worker + dispatcher: consumes `process_clip` jobs and performs batched inference over gRPC.
- Storage service: stores original files and generated assets.
- Database (SQLC queries): metadata, thumbnails, embeddings, and predictions.

Key files:
- `cmd/main.go`: bootstraps services, DB, storage, queues, ML gRPC client, and dispatcher.
- `internal/api/handler/asset_handler.go`: upload endpoints; enqueues `process_asset`.
- `internal/queue/asset_worker.go`: River worker that delegates to the asset processor.
- `internal/queue/clip_worker.go`: River worker that uses the CLIP dispatcher.
- `internal/queue/clip_dispatcher.go`: batching and gRPC streaming logic for CLIP/smart-classify.
- `internal/queue/queue_setup.go`: River queue config and client creation.
- `internal/queue/jobs/types.go`: job payload definitions and `Kind()` mapping.

## End-to-end upload flow

Two public endpoints handle uploads:

- POST `/api/v1/assets` — single upload
- POST `/api/v1/assets/batch` — batch upload

Both stage files to disk, then enqueue a background job to process each file.

### Single upload: `/api/v1/assets`

1) Client sends multipart/form-data with `file`. Optional header `X-Content-Hash` can be provided (e.g., BLAKE3 from an ML service or the client). If absent, the handler logs a warning and generates a UUID placeholder.

2) The handler writes the file to a staging directory:
   - Path derived from `appConfig.StagingPath` (created on startup).
   - A UUID filename plus the original extension is used.

3) The handler enqueues a River job to queue `process_asset` with args:
   - `ClientHash`, `StagedPath`, `UserID`, `Timestamp`, `ContentType`, `FileName`
   - Job kind: `process_asset` (see `internal/queue/jobs/types.go`).

4) The handler responds immediately with:
   - `task_id` (River job ID), `status: "processing"`, original filename, size, and provided (or generated) content hash.

The enqueuing occurs here:
`internal/api/handler/asset_handler.go` — UploadAsset

The job is consumed by:
`internal/queue/asset_worker.go` — ProcessAssetWorker

The worker delegates to your `AssetProcessor`:
`processors.AssetProcessor.ProcessAsset(ctx, payload)`

Note: The actual ingestion steps live in the processor. Typical responsibilities of such a processor include:
- Validating the staged file and content type.
- Persisting the original file to the configured storage backend.
- Creating the asset DB record and metadata.
- Generating and saving thumbnails (if applicable).
- Emitting downstream jobs (e.g., `process_clip`) for ML features.

The service layer provides the primitives used by processors:
- Save original to storage: `AssetService.SaveNewAsset`
- Create thumbnail records: `AssetService.CreateThumbnail` / `SaveNewThumbnail`
- ML persistence: `AssetService.SaveNewEmbedding` / `SaveNewSpeciesPredictions`

### Batch upload: `/api/v1/assets/batch`

- Accepts a multipart with multiple files.
- Each part’s field name must be the file’s content hash (see handler docstring).
- For each file, the same staging + enqueue pattern is used.
- The response includes a result per file, with success/failure, task ID, size, and message.

Implementation:
`internal/api/handler/asset_handler.go` — BatchUploadAssets

## Queue topology and workers

Queues are configured in:
`internal/queue/queue_setup.go`

- `process_asset`
  - MaxWorkers: 5
  - Worker: `ProcessAssetWorker` calls `AssetProcessor.ProcessAsset(...)`.

- `process_clip`
  - MaxWorkers: 1
  - Worker: `ProcessClipWorker` calls the CLIP batch dispatcher.

Workers are registered and the dispatcher is started in:
`cmd/main.go`

River client runs asynchronously and processes jobs until shutdown.

Scaling guidance:
- `process_asset` can scale horizontally by increasing `MaxWorkers` (IO-bound work like storage and thumbnail creation typically benefits).
- `process_clip` is intentionally set to 1 worker to maximize batching efficiency. Increasing it opens more gRPC streams and can reduce batch utilization (see “CLIP dispatcher” below).

## CLIP processing: worker and batch dispatcher

CLIP-related processing is split into two parts:
- A River worker that submits individual inference tasks to the dispatcher.
- A batch dispatcher that aggregates requests and uses a single gRPC stream per batch for efficiency.

### ProcessClipWorker

Location:
`internal/queue/clip_worker.go`

Flow per job:
1) Receive `ProcessClipArgs`:
   - `AssetID` (pgtype.UUID)
   - `ImageData` ([]byte), encoded (e.g. webp)

2) Call dispatcher:
   - `Submit(ctx, assetID, imageData, "image/webp")`
   - Returns embedding + smart classification labels + meta.

3) Persist results:
   - `AssetService.SaveNewEmbedding(ctx, assetID, vector)`
     - Stored as pgvector (see SQLC repo `UpsertEmbedding`).
   - `AssetService.SaveNewSpeciesPredictions(ctx, assetID, predictions)`
     - For each label+score pair.

Worker errors are surfaced to River; River will retry per its configuration/policies.

### ClipBatchDispatcher design

Location:
`internal/queue/clip_dispatcher.go`

Purpose:
- Aggregate multiple concurrent Submit calls into batches.
- For each batch:
  - Open one gRPC stream to the inference service.
  - Send two tasks per item:
    - `clip_image_embed`
    - `smart_classify`
  - Use correlation IDs to relate responses back to items.
  - Return merged result (embedding + labels + meta) to each submitter.

APIs:
- `NewClipBatchDispatcher(client, batchSize, window)`
  - `batchSize`: max jobs per batch (defaults to 8 if <= 0).
  - `window`: max delay before sealing a partial batch (defaults to 1500ms if <= 0).
- `Start(ctx)` — begins the internal goroutine that drains the request channel and forms batches.
- `Submit(ctx, assetID, image, mime)` — synchronous call that enqueues a request and waits for the batch result.

Batching algorithm:
- An internal buffered channel (`in`) receives `Submit` requests.
- The loop:
  - Take the first request, start a timer for `window`.
  - Accumulate until reaching `batchSize` or the timer fires (whichever comes first).
  - Process the assembled batch in `processBatch`.

Per-batch gRPC streaming:
- Creates one `Infer` stream from `proto.InferenceClient`.
- For each request in the batch `i`:
  - Send `clip_image_embed` with `Seq = i*2`.
  - Send `smart_classify` with `Seq = i*2 + 1`, and `Meta = {"topk":"3"}` (configurable in code).
  - Correlation IDs are `${assetID}|emb` and `${assetID}|smart`.
- Close send-side and read responses until EOF or all expected responses are seen.

Response handling and correlation:
- Each response includes `CorrelationId` and optional `Error` and `Meta`.
- The dispatcher:
  - Parses suffix `|emb` or `|smart`.
  - Unmarshals JSON payload into `EmbeddingResult` or `LabelsResult`, respectively.
  - Captures `Meta` for `smart_classify` responses (includes a `source` key that indicates the classification branch the server used).
- Errors:
  - Per-response server errors are attached to the corresponding item.
  - If the stream errors mid-flight, remaining items are marked as errors.

Delivery:
- When both `EmbeddingResult` and `LabelsResult` are collected for an item (and no error recorded), the merged `ClipResult` is sent back via the item’s `resultCh`, unblocking the worker.

Why batch?
- A single stream amortizes overhead:
  - Fewer TCP/HTTP2 handshakes and TLS costs.
  - Larger, more efficient messages for the inference runtime.
- Better GPU utilization:
  - Model batching improves throughput with minimal latency impacts under load.

Concurrency notes:
- With `process_clip` MaxWorkers = 1, the dispatcher tends to form full or fuller batches.
- Increasing CLIP workers opens multiple streams in parallel:
  - Higher throughput when the ML backend and network support it.
  - Potentially lower batch fill ratios (fewer items per batch) which can reduce per-request efficiency.
- Tune `batchSize` and `window` based on real traffic and latency budget.

## Configuration knobs

- Staging directory:
  - Set in app config; created at startup: `appConfig.StagingPath`
- Storage:
  - Strategy and base path are loaded from environment variables.
  - `AssetHandler.StorageBasePath` pulls from `STORAGE_PATH` for serving thumbnails and original files.
- Queue concurrency:
  - `internal/queue/queue_setup.go` => `MaxWorkers` per queue.
- CLIP dispatcher:
  - Created in `cmd/main.go`: `NewClipBatchDispatcher(clipClient, 8, 1500ms)`
  - Tweak `batchSize` and `window` based on workload.
- ML gRPC address:
  - `appConfig.MLServiceAddr` and insecure credentials in dev by default.

## How to add a new queue/worker

1) Define job args and `Kind()` in `internal/queue/jobs/types.go`.
2) Implement a worker in `internal/queue/<your_worker>.go`:
   - `type YourArgs = jobs.YourArgs`
   - `type YourWorker struct { river.WorkerDefaults[YourArgs]; ... }`
   - `func (w *YourWorker) Work(ctx context.Context, job *river.Job[YourArgs]) error { ... }`
3) Register queue and concurrency in `internal/queue/queue_setup.go`:
   - Add to `Queues: map[string]river.QueueConfig{ "your_kind": { MaxWorkers: N }, ... }`.
4) Register the worker in `cmd/main.go`:
   - `river.AddWorker[jobs.YourArgs](workers, &queue.YourWorker{...})`
5) Enqueue jobs where needed:
   - `queueClient.Insert(ctx, jobs.YourArgs(...), &river.InsertOpts{Queue: "your_kind"})`

## Migrations (River)

Export your PostgreSQL connection string:

```/dev/null/commands.sh#L1-1
export DATABASE_URL="connstring"
```

Migrate Up (include in your setup or run manually):

```/dev/null/commands.sh#L1-1
river migrate-up --line main --database-url "$DATABASE_URL"
```

Migrate Down:

```/dev/null/commands.sh#L1-1
river migrate-down --line main --database-url "$DATABASE_URL" --max-steps 10
```

Notes:
- The application also runs its own data migrations (`db.AutoMigrate(...)`) for domain tables, which is separate from River’s migrations.
- Make sure the DB user has privileges to create/update schemas and tables.

## Troubleshooting

- Upload works but no background processing:
  - Verify River client started (logs: “Queues initialized successfully”).
  - Confirm `process_asset` jobs are inserted (check DB river tables).
  - Check that `process_asset` worker is registered in `main.go` and the `Queues` map.

- CLIP results missing or errors:
  - Ensure ML gRPC address is reachable (`appConfig.MLServiceAddr`) and server is healthy.
  - Dispatcher logs: look for stream errors or JSON unmarshal errors (embedding/labels).
  - Consider increasing `window` if batches are consistently too small, or increasing `process_clip` concurrency if GPU is underutilized.

- Thumbnails/original file not found:
  - Confirm `STORAGE_PATH` is consistent between writer (storage service) and reader (handlers).
  - Check storage paths in DB records.

- Hashes and duplicates:
  - `X-Content-Hash` is optional in single upload; batch upload expects the field name to be the hash. Use your ML or client tooling to compute hashes consistently.
