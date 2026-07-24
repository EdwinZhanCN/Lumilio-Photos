# Video Semantic Search

Status: active, **implemented** (pending live verify). Prerequisite
[embedding-architecture](../completed/embedding-architecture.md) is **shipped**
(`search_embeddings` + max-pool + canonicalize). Code for frame extract, embed
worker, `best_ts`, backfill, settings, and UI is in tree; apply migration
`000013` and smoke-test with real clips before moving to `completed/`.

Goal: production text→video semantic search. Sample frames from each video's
transcoded `web.mp4`, embed with SigLIP2 into the same canonical 768-dim L2
unit-vector space as photos, and let the existing retriever rank photos and
videos together. A video hit carries `best_ts_ms` so the player can seek to the
matching frame.

## Current baseline (already shipped)

- `search_embeddings` is multi-row per asset: photo = 1 row (`frame_ts_ms IS
  NULL`); video = N frame rows (`frame_ts_ms` set). Partial uniques and HNSW
  `vector_l2_ops` exist (migration `000012`).
- All semantic paths max-pool per asset (`MIN(dist) + GROUP BY asset_id`):
  aggregate retriever (`search/retrievers.go`), set-retrieve exact
  (`search/setretrieve.go`), and direct vector browse
  (`asset_service.searchAssetsBySemanticSpace`).
- `canonicalizeSemanticVector` (truncate 768 + L2 renorm) is the shared write
  and query path.
- Photo `SaveEmbedding(semantic)` resolves the default space, deletes the
  asset's rows, inserts one primary (`frame_ts_ms IS NULL`) row. Comment already
  reserves frame rows for this plan — do **not** route videos through it.

Still owned here: frame extract → embed → write frame rows → `best_ts` →
pipeline → backfill → UI.

## Where the PHOTO-only restriction lives

Semantic embedding runs only for `PHOTO` today; video keeps a dedicated path and
does **not** relax these:

- `processors/thumbnail_task.go` — `AssetTypePhoto` gate (~L63); `enqueueMLJobs`
  call at ~L70 (body in `photo_helpers.go`). Videos get poster only via
  `generateVideoThumbnail`.
- `queue/ml_image_loader.go` — `LoadMLImage` rejects non-photos (~L55). Video
  frames never use this loader.

## Decisions

- **Same table, same space, same source.** Video frames write
  `search_embeddings` under the default semantic space. Retriever source stays
  `"embedding"` (no separate `video_frame` source / RRF weight). Ranking is
  already per-asset max-pool.
- **Dedicated job after transcode**, reading the normalized 1080p H.264 `web`
  version at `{repo}/.lumilio/assets/videos/web/{contentHash}_web.mp4`. Do not
  fuse extract into the transcode ffmpeg graph (hwaccel + `hwdownload` couples
  two concerns). Poster (`generateVideoThumbnail` on the original) stays
  independent so the UI is not blocked on transcode.
- **Worker shape mirrors transcode, not photo ML.** Thin River worker +
  `AssetProcessor.ProcessVideoFramesTask`. Extract lives next to existing ffmpeg
  helpers; `AssetProcessor` already holds `embeddingService` / `lumenService` /
  `queueClient`. Avoids `queue`↔`processors` extract ownership fights.
- **Replace-all write.** One transaction: `DeleteSearchEmbeddingsByAsset` + N
  `InsertSearchEmbedding` with `frame_ts_ms` set. No NULL-primary row for
  videos. Add `EmbeddingService.SaveVideoFrameEmbeddings` (or equivalent) —
  never call photo `SaveEmbedding` on a video (its delete would wipe frames and
  insert a NULL primary). Optional hardening: narrow photo delete to
  `frame_ts_ms IS NULL` only.
- **No zero-shot / aesthetic / OCR / face on video frames** in this plan.
- **Sampling knobs are runtime settings** (like backup numerics), not TOML
  literals — see Config.

## Frame sampling (conservative, consumer hardware)

CPU-only SigLIP2 is the binding cost. Hard cap `N_max` (default **8**). Strategy
by duration `D` (prefer `asset.Duration` already written by metadata probe; fall
back to ffprobe on `web.mp4`):

| Duration | Strategy |
|----------|----------|
| `D < 4s` | 1 frame at midpoint. Scene detection is meaningless. |
| `4s ≤ D < LongThreshold` (default 300s) | Scene-change: `select='gt(scene,T)'` (default `T=0.4`). If cuts > `N_max`, uniformly subsample the cut list. |
| `D ≥ LongThreshold` | Uniform interval `D / N_max`, I-frames only (`-skip_frame nokey`), capped at `N_max`. |

Each extracted frame is JPEG (or any `imagesource`-decodable bytes) tagged with
`frame_ts_ms`. No scene-select / multi-frame helper exists today — greenfield on
`FFmpegCommand()` beside `generateVideoThumbnail`.

## Implementation

### Phase 1 — Frame extraction

`processors/video_helpers.go`: `extractSemanticFrames(webPath, duration, cfg) →
[]frame{path|bytes, frame_ts_ms}`:

1. Resolve `web` path from repo path + content hash (same layout as
   `SaveVideoVersion` / asset handler).
2. Choose strategy from duration + `settings.ML` sampling fields.
3. Run ffmpeg to a temp dir; bound output to `N_max`; return tagged frames.
4. Caller always cleans the temp dir.

Unit-test strategy selection with fake durations (no ffmpeg required for the
branching helper); one integration-style test can shell out to ffmpeg when
present.

### Phase 2 — Embedding worker + write path

1. `queue/jobs/types.go`: `ProcessVideoFramesArgs{AssetID, PreprocessVersion}`
   (mirror semantic: `MLPreprocessVersionV1`, `UniqueOpts{ByArgs, ByPeriod: 5m}`,
   `MaxAttempts=8`). Kind `process_video_frames`.
2. Thin worker in `queue/` (same pattern as `TranscodeWorker`) delegating to
   `AssetProcessor.ProcessVideoFramesTask`.
3. Task body:
   - Gate: `SemanticEnabled && VideoSemanticEnabled` via
     `isMLTaskEnabled(..., "process_video_frames")` (extend
     `ml_config_provider.go`).
   - Resolve asset + repo; require `web.mp4` on disk (else fail / snooze — do not
     invent frames from the original).
   - `extractSemanticFrames` → for each frame:
     `imagesource.ProcessMLImageTensorBytes(jpegBytes, PurposeSemantic)` →
     `LumenService.SemanticImageEmbed` → collect vectors.
   - `canonicalizeSemanticVector` inside the save helper (same as photo path).
   - `SaveVideoFrameEmbeddings(ctx, assetID, modelID, []{ts, vector})` in one
     transaction (delete all asset rows + insert N frame rows).
4. Register worker in `app/app.go` next to other ML/transcode workers (~L555+,
   **not** the ~L288 transcode-only block). Add queue
   `process_video_frames` in `queue_setup.go` with `MaxWorkers: 1` (match
   transcode CPU budget; `process_semantic` is 2).
5. Timeout: budget for up to `N_max` embeds (start at ~10–15 min; tune after
   smoke).

### Phase 3 — Pipeline wiring

In `processors/transcode_task.go`, after successful `transcodeVideoSmart` (both
`copyVideoAsWebVersion` and `saveTranscodedVideo` leave a `web` file):

```go
ap.queueClient.Insert(ctx, jobs.ProcessVideoFramesArgs{...},
  &river.InsertOpts{Queue: "process_video_frames"})
```

Same enqueue style as `photo_helpers.enqueueMLJobs` / metadata chaining — **not**
`river.ClientFromContextSafely` (that pattern is for ML workers chaining
zero-shot). Gate enqueue on effective ML settings so disabled installs do not
queue no-op jobs.

Also extend `asset_retry.go` so a video ML retry can re-queue
`ProcessVideoFramesArgs` when the `web` version exists (today ML retries are
photo-oriented).

### Phase 4 — Backfill / reindex

Indexing is photo-scoped today (`CountPhotoAssets*`,
`ListPhotoAssets*`, `AssetIndexingTaskSemanticImage`). Add a video-semantic task:

1. `AssetIndexingTaskVideoSemantic = "video_semantic"`.
2. sqlc: count/list video assets (`type = 'VIDEO'`) missing any
   `search_embeddings` row with `frame_ts_ms IS NOT NULL` (or missing the asset
   entirely from the table).
3. `enqueueVideoFramesTask` → `ProcessVideoFramesArgs`; wire into
   `enqueueAssetIndexingTasks`, `filterEnabledIndexingTasks`,
   `normalizeRequestedIndexingTasks` / request parsing.
4. Extend `AssetIndexingStats` + `AssetIndexingTaskSetStatsDTO` with a
   video-semantic bucket; add `VideoTotal` (or fold under the task's
   `TotalCount`). Update Monitor UI (`monitor/flows/overview/MLMonitor.tsx`,
   `monitor/api/useAssetIndexing.ts`) — progress lives in **Monitor**, not
   Settings.
5. **`ResetSemantic` path:** `DeleteAllSearchEmbeddings` already wipes video
   rows; it must also enqueue `video_semantic` (or the frames job) for videos,
   not only photo `ProcessSemanticArgs`.

### Phase 5 — `best_ts` + frontend

**Backend (all three max-pool sites):**

```sql
SELECT
  a.asset_id,
  MIN(dist)::float8 AS raw_score,
  (array_agg(e.frame_ts_ms ORDER BY dist ASC NULLS LAST))[1] AS best_ts
...
GROUP BY a.asset_id
```

Thread through:

1. `search.Candidate` — add `BestTsMs *int32` (NULL for photos).
2. `collectCandidates` — scan the new column.
3. `fusedCandidate` / `fuseWeightedRRF` — when merging sources, keep the
   embedding-channel `BestTsMs` (OCR/place hits have none).
4. Hydration → search response: add optional `best_ts_ms` on `BrowseItemDTO`
   (search-hit metadata; leave `AssetDTO` clean). Run `make dto`.
5. Same column on set-retrieve + `searchAssetsBySemanticSpace` so agent
   set-search and vector browse stay consistent.

**Frontend:**

- Search grids already render video badge/poster via `type === VIDEO`
  (`MediaThumbnail.tsx`). No separate “video hit” chrome required.
- Real work: when opening a search hit with `best_ts_ms`, pass start time into
  `MediaViewer` / Vidstack (`currentTime` / equivalent). No seek-from-search
  path exists today (`useLivePhotoPlayback` only resets to 0).
- Settings → AI (`AiTab` / `useAISettingsDraft`): expose
  `VideoSemanticEnabled` (+ optional advanced sampling knobs if surfaced).
- i18n: extract-then-fill only.

## Config

Type is `settings.ML` (not `MLConfig`). Persistence mirrors backup numerics:

| Field | Default (prod `Default()`) | Dev `Default()` |
|-------|----------------------------|-----------------|
| `VideoSemanticEnabled` | `true` | `false` (zero `ML{}`) |
| `VideoMaxFrames` | `8` | `8` (column default; zero-value must be clamped on read) |
| `VideoLongThresholdSeconds` | `300` | same |
| `VideoSceneThreshold` | `0.4` | same |

Surface:

1. Migration adding `settings.ml_video_semantic_enabled` + numeric columns with
   DEFAULTs.
2. sqlc `settings` queries + `settings_service` map/clamp.
3. `MLSettingsDTO` / `UpdateMLSettingsDTO` + `make dto`.
4. `isMLTaskEnabled` + `HasManualTasksEnabled` / runtime demand include the
   video toggle.
5. Gate formula for the job: `SemanticEnabled && VideoSemanticEnabled` so
   turning off photo semantic also stops video (shared model/space).

## Verification

- `make server-test`:
  - `ProcessVideoFrames` / save helper: fake extract + fake Lumen → ≤ `N_max`
    rows, sane monotonic-ish `frame_ts_ms`, no NULL primary.
  - Search: video with frames returned; `best_ts_ms` set; photo hits still
    `null`.
  - Indexing filter/enqueue for `video_semantic`; `ResetSemantic` re-queues
    videos.
- Manual: short clip (scene cuts) + long clip (interval); confirm frame count
  ≤ `N_max`, mixed photo+video text search, seek lands near the match.
- `make web-test` + i18n extract/fill for AI settings + Monitor + viewer seek.

## Non-goals

- Audio semantic search (Whisper / CLAP).
- OCR / face / zero-shot / aesthetic on video frames.
- Storage schema / HNSW / canonicalize — owned by
  [embedding-architecture](../completed/embedding-architecture.md).
- Separate video retriever or RRF source weight.
- Fusing frame extract into the transcode ffmpeg invocation.

## Critical files

- `server/internal/processors/video_helpers.go` — `extractSemanticFrames`
- `server/internal/processors/transcode_task.go` — enqueue after `web` exists
- `server/internal/queue/jobs/types.go` + thin worker + `queue_setup.go` +
  `app/app.go` — job/queue/registration
- `server/internal/service/embedding_service.go` —
  `SaveVideoFrameEmbeddings`
- `server/internal/search/{retrievers,setretrieve,types,service}.go` +
  `asset_service.go` — `best_ts` through all max-pool paths
- `server/internal/service/indexing_service.go` + indexing SQL — backfill /
  `ResetSemantic`
- `server/internal/settings/settings.go` + settings migration/DTO/UI —
  `VideoSemanticEnabled` + sampling knobs
- `web/.../MediaViewer.tsx` + Monitor indexing + Settings AI — seek + toggles +
  progress
