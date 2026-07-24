# Video Semantic Search

Status: active, not started. **Sequenced after
[embedding-architecture](../completed/embedding-architecture.md)** — it builds directly on the
unified `search_embeddings` table and must not start until that refactor lands.
Ships production text→video semantic search by generating SigLIP2 frame
embeddings for video assets. Video reuses the same canonical (768-dim, cosine)
space and the same retriever as photos, so one text query ranks photos and
videos together.

## Dependency on the embedding refactor

The architecture plan already delivers everything storage- and search-side:

- `search_embeddings` is **multi-row per asset** (`asset_id`, nullable
  `frame_ts_ms`, `vector(768)`), so a video is just N frame rows — no separate
  video table.
- The **max-pool retriever** already returns `best_ts` (nearest frame timestamp),
  which is exactly the video deep-link signal.
- `canonicalizeEmbedding(emb, 768)` (truncate + renorm) is already the shared
  write/query path; video frames call the same helper.

So this plan is only: **extract frames → embed them → write rows → wire the
pipeline → backfill → surface in the UI.** The earlier draft's separate
`video_frame_embeddings` table and bespoke retriever are dropped; they are
subsumed by the unified table.

## Where the PHOTO-only restriction lives

Semantic embedding runs only for `PHOTO` today, gated in two places:

- `internal/processors/thumbnail_task.go:63` — `enqueueMLJobs` is photo-only.
- `internal/queue/ml_image_loader.go:55` — `LoadMLImage` rejects non-photos.

Video takes a **dedicated path** (below) and does not reuse the photo loader, so
neither of these is relaxed.

## Frame sampling (conservative, consumer hardware)

CPU-only SigLIP2 inference is the binding cost. Every strategy is bounded by a
single hard cap `N_max` (default **8** frames/video). Strategy is chosen by
duration `D` (already probed into `VideoInfo.Duration`):

- `D < 4s`: 1 frame at the midpoint. Scene detection is meaningless.
- `4s ≤ D < 300s` (short): **scene-change detection**, `select='gt(scene,0.4)'`.
  If detected cuts exceed `N_max`, uniformly subsample the cut list to `N_max`.
- `D ≥ 300s` (long): **uniform interval sampling**, interval `= D / N_max`,
  decoding I-frames only (`-skip_frame nokey`) to cut decode cost. Capped at
  `N_max`.

`N_max` (8), the short/long boundary (300s), and the scene threshold (0.4) live
in ML config, not as literals. Each extracted frame carries its `frame_ts_ms`,
written straight into `search_embeddings.frame_ts_ms`.

## Extract from the transcoded web.mp4, not the original

Frame extraction runs as a downstream job that depends on transcode completion
and reads the normalized 1080p H.264 `web` version:

- Scene detection on a downscaled 1080p file is far cheaper than on a 4K original
  — that decode, not the pixel copy, is the real cost.
- The web.mp4 is small and fast-seeking; I-frame-only decode on the long path is
  cheap.
- Independent job → independent retry/observability; a frame-extract failure
  never blocks or corrupts transcode.

We deliberately do **not** fuse extraction into the transcode ffmpeg command
(`-filter_complex split`): the one-decode saving is real only on the pure
software path; with videotoolbox/vaapi/nvenc it forces `hwdownload` and couples
two concerns into one fragile invocation. The existing fast single-frame poster
(`generateVideoThumbnail`, one cheap seek on the original) stays as-is so the
poster still appears without waiting on transcode.

## Implementation

### Phase 1 — Frame extraction

`internal/processors/video_helpers.go`: `extractSemanticFrames` — resolve the
`web` version path, pick the duration-based strategy, run ffmpeg (scene-select or
I-frame interval) to a temp dir, return frames each tagged with `frame_ts_ms`.
Bounded by `N_max`.

### Phase 2 — Embedding worker

1. `internal/queue/jobs/types.go`: `ProcessVideoFramesArgs{AssetID,
   PreprocessVersion}`, queue `process_video_frames`.
2. `internal/queue/ml_video_frames_worker.go`: gated by `SemanticEnabled` (+ new
   `VideoSemanticEnabled` sub-toggle). For each frame:
   `imagesource.ProcessMLImageTensorBytes(..., PurposeSemantic)` →
   `LumenService.SemanticImageEmbed` → `canonicalizeEmbedding(emb, 768)` → collect
   rows → replace the asset's frame rows in one transaction
   (`DeleteSearchEmbeddingsByAsset` + `UpsertSearchEmbeddings`).
3. Register the worker in `app/app.go` (next to the ML workers ~line 288) and add
   `process_video_frames` to `internal/queue/queue_setup.go:48` with low
   `MaxWorkers` (start at 1, matching the transcode budget) to bound CPU.

### Phase 3 — Pipeline wiring

Chain the frames job off transcode success: in
`internal/processors/transcode_task.go`, in the `AssetTypeVideo` branch after
`transcodeVideoSmart`, enqueue `ProcessVideoFramesArgs` via
`river.ClientFromContextSafely` (the same chaining pattern `ProcessSemanticWorker`
uses for zero-shot). Depending on transcode guarantees the `web` version exists.

### Phase 4 — Backfill / reindex

The reindex path is photo-scoped (`CountPhotoAssetsForIndexing`,
`ListPhotoAssetsForIndexingBatch`, `ListPhotoAssetsMissingEmbeddingType` in
`internal/service/indexing_service.go`). Add a video-semantic task:

1. New `AssetIndexingTaskVideoSemantic` + sqlc queries listing video assets with
   no `search_embeddings` rows, plus count queries for stats.
2. `enqueueVideoFramesTask` inserting `ProcessVideoFramesArgs`; include it in
   `enqueueAssetIndexingTasks` and the enabled-task gating.
3. Extend `AssetIndexingStats` with a video-semantic bucket for Settings progress.

Note: the architecture plan's model-swap reindex (`DeleteAllSearchEmbeddings` +
reprocess) must reprocess videos too — reuse this task on that path.

### Phase 5 — Frontend

1. Search results: render video hits (badge, poster) and, when `best_ts` is
   present, deep-link the player to that timestamp.
2. Settings → indexing: expose `VideoSemanticEnabled` and reindex progress. i18n
   keys via extract-then-fill.

## Config additions

`internal/settings/settings.go` `MLConfig`:

- `VideoSemanticEnabled bool` (default true; disables the heavier video path
  independently of photo semantic).
- `VideoMaxFrames int` (default 8).
- `VideoLongThresholdSeconds int` (default 300).
- `VideoSceneThreshold float64` (default 0.4).

## Verification

- `make server-test`: `ProcessVideoFramesWorker` test (fake image loader + fake
  Lumen, asserts ≤ `N_max` rows with sane `frame_ts_ms`), plus a search test
  confirming a video with frames is returned and carries `best_ts`.
- Manual: ingest a short clip (scene cuts) and a long clip (interval); confirm
  frame count ≤ `N_max`, text search returns both photos and videos, and the hit
  jumps to a sensible timestamp.
- `make web-test` + i18n extract/fill for search and settings.

## Non-goals

- Audio semantic search (Whisper / CLAP) — separate plan.
- OCR / face on video frames — the frame pipeline makes these possible later,
  out of scope here.
- Storage/retriever/ANN work — owned by
  [embedding-architecture](../completed/embedding-architecture.md).

## Critical files

- `server/internal/processors/video_helpers.go` (frame extraction)
- `server/internal/queue/ml_video_frames_worker.go` (new worker) + `app/app.go`,
  `queue_setup.go` (registration/queue)
- `server/internal/processors/transcode_task.go` (chain after transcode)
- `server/internal/service/indexing_service.go` (video backfill task)
- `server/internal/settings/settings.go` (config knobs)
