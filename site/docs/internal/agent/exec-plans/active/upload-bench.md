# Upload throughput benchmark

## Goal

Design and implement a reproducible benchmark for Lumilio Photos' upload-to-photo-ready pipeline, with ML/AI processing excluded. The output should support two audiences:

1. **Engineering**: identify bottlenecks in upload staging, ingest, metadata extraction, thumbnail generation, queueing, PostgreSQL, and disk IO.
2. **Public/product**: produce defensible throughput, latency, CPU, and memory numbers that can be published on a landing page without over-claiming.

The benchmark must not count HTTP upload success as processing completion. A photo is **photo-ready** only after the non-ML core pipeline for that asset has completed successfully.

## Current pipeline facts

From the current codebase:

- Upload endpoints:
  - `POST /api/v1/assets` (`AssetHandler.UploadAsset`) accepts one multipart file, writes it into repository staging, computes or trusts BLAKE3, and enqueues `ingest_asset`.
  - `POST /api/v1/assets/batch` (`AssetHandler.BatchUploadAssets`) supports multi-file and chunked upload sessions.
- Photo core pipeline:
  - `ingest_asset` materializes staged upload into repository inbox through `SourceMaterializer`.
  - `SourceMaterializer.enqueuePipeline` enqueues `metadata_asset` and `thumbnail_asset` for photos.
  - Photo completion is tracked in `assets.status.tasks` with `metadata_asset` and `thumbnail_asset` task states.
- Default worker sizing:
  - `ingest_asset`: `clamp((NumCPU+1)/2, 2, 8)`.
  - `metadata_asset`: 20 workers.
  - `thumbnail_asset`: `clamp(NumCPU, 4, 12)`.
- Optional/non-core work to handle explicitly:
  - ML queues: `process_semantic`, `process_bioclip`, `process_ocr`, `process_face`, `classify_zeroshot` — disabled/excluded for this benchmark.
  - Non-ML side jobs may include `rebuild_location_clusters`, `detect_stacks`, `match_live_photo`, and `process_phash` fallback. Report them separately or run a separate “default non-ML” profile.
- Existing queue observability:
  - `GET /api/v1/admin/river/queue-summary` exposes per-queue job totals, remaining jobs, average latency, and average runtime.
  - `GET /api/v1/admin/river/stats` exposes job counts by River state.

## Benchmark profiles

### Profile A — Core photo-ready throughput

Use this for the main marketing number.

Included:

- HTTP upload acceptance.
- Staging write.
- Hash resolution.
- `ingest_asset`.
- `metadata_asset`.
- `thumbnail_asset`.
- PostgreSQL writes and thumbnail file writes required by those tasks.

Excluded or isolated:

- Semantic indexing.
- BioCLIP.
- OCR.
- Face processing.
- Zero-shot classification.
- Optional non-core side jobs, unless they are proven inactive or measured separately.

Completion condition for one photo:

- Asset row exists with a benchmark run marker or deterministic uploaded filename prefix.
- `status.state = "complete"`, or more explicitly:
  - `status.tasks.metadata_asset.state = "complete"`.
  - `status.tasks.thumbnail_asset.state = "complete"`.
- No `metadata_asset` or `thumbnail_asset` failure remains for that asset.

### Profile B — Default non-ML throughput

Use this as a secondary real-world number.

Included:

- Everything in Profile A.
- Non-ML side jobs that are naturally triggered by upload, such as location cluster rebuild, stack detection, live-photo matching, and pHash fallback.

Still excluded:

- ML/AI queues listed above.

Report this separately because it answers a different user question: “how fast does my library settle with AI off?” rather than “how fast are photos browse-ready?”

## Metrics

### Primary public metrics

- **Photo-ready throughput**: completed photos / second and photos / minute.
- **Data throughput**: completed GB / minute, using original uploaded bytes.
- **End-to-end makespan**: first upload request start → last photo-ready completion.
- **Drain time**: final HTTP upload accepted → last photo-ready completion.
- **Completion latency distribution** per asset: p50, p90, p95, p99.

### Engineering metrics

- Upload acceptance:
  - request latency p50/p95/p99;
  - accepted files/sec;
  - accepted MB/sec;
  - HTTP 4xx/5xx/timeout count.
- Queue behavior by queue:
  - enqueue count;
  - completed count;
  - failed/retryable/discarded count;
  - queue wait time = `attempted_at - created_at`;
  - runtime = `finalized_at - attempted_at`;
  - max/average queue depth;
  - worker saturation.
- Resource usage at 1-second cadence:
  - Lumilio server CPU%, RSS, peak RSS;
  - PostgreSQL CPU%, RSS;
  - media child-process CPU% when applicable (`exiftool`, libvips-backed work, `ffmpeg` if videos are included in another profile);
  - total system CPU%;
  - disk read/write MB/s, IOPS, and latency;
  - network throughput if client is remote;
  - Go heap/GC stats if pprof/expvar is added.
- Database behavior:
  - connection count;
  - transaction rate;
  - WAL volume;
  - lock waits;
  - slow queries;
  - checkpoint/autovacuum events.

## Dataset plan

Use fixed datasets with published characteristics.

### Dataset J — standard JPEG claim set

Purpose: clean public headline number.

- 1,000–10,000 JPEG photos.
- At least 20 GB or at least 10 minutes of steady-state runtime on target hardware.
- Disclose:
  - file count;
  - total GB;
  - median/p90/p99 file size;
  - megapixel distribution;
  - EXIF/GPS/orientation distribution;
  - camera/device mix.

### Dataset R — real-world mixed photo set

Purpose: expectation-setting number.

- JPEG + HEIC + PNG + RAW if supported in the current environment.
- Same disclosure fields as Dataset J, plus format breakdown.
- RAW/HEIC should not be mixed into the headline JPEG claim unless the landing page text says so.

### Dataset hygiene

- No duplicate files unless duplicate handling is part of the claim.
- No preexisting asset rows, thumbnails, or stale River jobs.
- Keep videos/audio out of the main photo claim. They need a separate media benchmark because transcode changes the bottleneck.

## Environment controls

For every run, record:

- Lumilio Photos commit/version.
- Native/server/docker/desktop mode.
- OS version.
- CPU model, core count, power mode, thermal state.
- RAM.
- Disk model, filesystem, and whether DB/repository/staging/temp share the same disk.
- PostgreSQL version.
- Go version.
- libvips, exiftool, ffmpeg versions.
- River worker configuration.
- Runtime config and ML settings snapshot.

Controls:

- Run on AC power.
- Disable sleep, cloud sync, Time Machine/backups, Spotlight indexing on the benchmark repository, antivirus scans, and heavy background jobs.
- Start each run from a clean DB + clean benchmark repository.
- Warm-cache and cold-cache runs must be labeled separately.
- Run at least 5 repetitions per configuration. Use median, p25/p75, and bootstrap 95% CI; never publish the best run as typical.

## ML exclusion protocol

Before each run:

1. Disable ML-related runtime settings through the settings API/UI or a clean DB seed.
2. Disable or disconnect Lumen discovery if possible for the benchmark environment.
3. Ensure no preexisting jobs in:
   - `process_semantic`
   - `process_bioclip`
   - `process_ocr`
   - `process_face`
   - `classify_zeroshot`
4. Capture queue counts before upload starts.
5. Capture queue counts after benchmark completion.
6. The report must include evidence that ML queues were empty or excluded from the measurement window.

## Harness design

### Components

1. **Dataset manifest generator**
   - Walk benchmark dataset.
   - Record relative path, file size, extension, MIME guess, dimensions if cheaply available, and BLAKE3 hash if precomputed.
   - Emit immutable `manifest.json` for the run.

2. **Run orchestrator**
   - Creates a clean benchmark repository / DB state.
   - Logs in or uses an existing benchmark user.
   - Resolves target repository ID.
   - Verifies ML-off settings and empty queues.
   - Starts samplers.
   - Uploads files with a run-specific filename prefix or metadata marker.
   - Polls completion until all expected assets are photo-ready or timeout.
   - Emits raw event logs and a final summary.

3. **Uploader**
   - Uses `POST /api/v1/assets` first for a simple, controlled baseline.
   - Later adds `/assets/batch` / chunk mode to benchmark the production UI path.
   - Supports concurrency sweep: 1, 2, 4, 8, 16, 32 concurrent uploads, stopping when throughput plateaus or errors/backlog grow.
   - Records request start/end, status code, response `task_id`, filename, size, and hash.
   - Prefer client-side BLAKE3 hash headers to avoid measuring server-side hash calculation unless the claim explicitly includes it.

4. **Completion poller**
   - Polls DB directly for benchmark assets, not just queue totals.
   - Computes per-asset photo-ready time from `status.updated_at` when `metadata_asset` and `thumbnail_asset` are complete.
   - Also samples `/api/v1/admin/river/queue-summary` for queue-level reporting.

5. **Resource sampler**
   - 1-second cadence.
   - macOS: `ps`, `vm_stat`, `iostat`, and optionally Activity Monitor export / powermetrics for thermal notes.
   - Linux: `pidstat`, `iostat`, `/proc`, cgroup stats when Docker is used.
   - PostgreSQL: `pg_stat_activity`, `pg_stat_database`, `pg_stat_bgwriter` / checkpoint views, WAL bytes where available.
   - Future enhancement: add guarded `net/http/pprof` or expvar in dev/benchmark mode for Go heap/GC.

6. **Analyzer**
   - Converts raw event logs to per-run summaries.
   - Computes throughput, latency percentiles, queue stats, CPU/RSS peaks, disk IO peaks, and error counts.
   - Produces machine-readable JSON/CSV plus a Markdown report.

## Experiment matrix

Minimum initial matrix:

| Axis | Values | Purpose |
| --- | --- | --- |
| Dataset | JPEG claim set | Public baseline |
| Upload API | single-file `/assets` | Simple controlled baseline |
| Concurrency | 1, 2, 4, 8, 16, 32 | Find saturation point |
| Cache | warm-cache first; cold-cache if feasible | Separate repeatability from cold-start cost |
| Profile | Core photo-ready | Main claim |

Secondary matrix:

| Axis | Values | Purpose |
| --- | --- | --- |
| Dataset | mixed real-world set | User expectation |
| Upload API | `/assets/batch` chunked | UI-path realism |
| Profile | default non-ML | Include natural side jobs |
| Runtime | native vs Docker/desktop | Packaging comparison, if needed |

## Reporting format

Public claim template:

> On [hardware/OS/storage], Lumilio Photos processed [N] JPEG photos totaling [GB] at a median [X] photos/min ([Y] GB/min) from upload acceptance to metadata + thumbnail completion, with ML queues disabled. Median CPU usage was [A]%, peak memory was [B] GB, and p95 photo-ready latency was [C] s. Results are median of [R] runs on commit [sha].

Required caveats:

- Results depend on hardware, disk, dataset, and worker configuration.
- HTTP upload acceptance is asynchronous and is not the same as photo-ready completion.
- ML/AI work was disabled/excluded: semantic indexing, BioCLIP, OCR, face processing, and zero-shot classification.
- RAW/HEIC/video workloads have different performance characteristics unless explicitly included.
- State whether non-ML side jobs were enabled or excluded.

## Workstreams

### W0 — measurement definitions

- Finalize the exact completion predicate for photo-ready assets.
- Decide whether Profile A should suppress non-core side jobs or only measure them separately.
- Decide whether server-side hash computation is included in the headline claim.

### W1 — benchmark harness scaffold

- Add a benchmark runner under a non-production path (candidate: `tools/bench/upload/` or `server/tools/uploadbench/`).
- Implement manifest generation, authenticated upload, completion polling, and JSON event output.
- Support run IDs and deterministic filename prefixing.

### W2 — resource sampling

- Implement macOS and Linux samplers.
- Record process tree metrics for server, PostgreSQL, and media helper processes.
- Emit time-series CSV/JSON for later plotting.

### W3 — clean-room run setup

- Script creation/reset of benchmark DB + repository.
- Script ML-off verification and queue-empty preflight.
- Script post-run artifact collection.

### W4 — analysis and report generation

- Compute per-run and aggregate statistics.
- Generate Markdown report with tables and claim-ready summary.
- Keep raw logs for auditability.

### W5 — calibration and publication run

- Run local calibration to find saturation concurrency.
- Run 5+ independent repeats for the chosen public hardware.
- Review outliers and rerun only with documented external interruptions.
- Publish numbers with environment and dataset disclosure.

## Acceptance criteria

A benchmark result is publishable only if:

- It uses a clean repository and clean queue state.
- The dataset manifest is committed or archived with the run artifacts.
- ML queues are disabled/empty or explicitly excluded with evidence.
- 100% of expected photos reach the core completion predicate, or failures are disclosed and excluded from throughput with a reason.
- HTTP errors, queue retries, and failed core jobs are zero or explicitly reported.
- At least 5 repetitions exist for the headline configuration.
- The reported number is the median, not the best run.
- Environment, worker config, dataset composition, and cache posture are disclosed.

## Risks and open questions

- **Side-job contention**: GPS metadata can enqueue location rebuilds; photo metadata can enqueue stack/live-photo work; thumbnails may enqueue pHash fallback. Decide whether to disable, include, or separately report each.
- **Hash semantics**: If client hashes are provided, benchmark excludes server hash computation. If not, server BLAKE3 becomes part of upload acceptance cost. The claim must say which mode was used.
- **Status timestamp granularity**: `assets.status.updated_at` is updated at task completion; use task states for completion and River timestamps for queue timing.
- **Client bottleneck**: Upload generator can become CPU/disk bound. Prefer a separate client host or prove local client overhead is below server saturation.
- **Thermal throttling**: Laptop marketing numbers must disclose power/thermal controls or use a stable desktop/server host.
- **Full `vp fmt --check` currently has unrelated formatting drift**: do not mix benchmark work with unrelated formatting cleanup unless explicitly scoped.

## Critical files for implementation

- `server/internal/api/handler/asset_handler.go`
- `server/internal/sourcing/materializer.go`
- `server/internal/processors/asset_status_tracker.go`
- `server/internal/processors/thumbnail_task.go`
- `server/internal/queue/queue_setup.go`
- `server/internal/api/handler/queue_handler.go`

## Implementation status (2026-07-07)

Harness landed at `server/tools/uploadbench/` (host-side Go tool, HTTP-only,
no direct DB access → works against the Docker Compose **release** stack). Run
with `cd server && go run ./tools/uploadbench ...`; see its `README.md` for the
runbook and the GitHub push → GHCR image pull flow.

### W0 measurement definitions — resolved

- **Completion predicate**: per asset, `status.tasks.metadata_asset.state ==
  "complete"` AND `status.tasks.thumbnail_asset.state == "complete"` (parsed
  from the base64 JSONB `status` field on `POST /assets/list`). A `failed` core
  task marks the asset failed. Assets are matched to uploads by
  `original_filename` (dataset hygiene requires unique names; the tool aborts on
  a collision). Per-asset completion latency = first-observed complete time −
  HTTP accept time, at the poll cadence (default 1 s) — disclose this
  granularity; it is API-polling, not DB-exact.
- **Profile A side jobs**: suppressed where cheap — the core profile does **not**
  send `repository_id`, so `detect_stacks` is never enqueued (it only fires when
  `repository_id` is passed). `rebuild_location_clusters` / `match_live_photo`
  still enqueue from `metadata_asset` but do not block the predicate; reported
  separately in the queue table. Profile B (`-profile default-nonml`) sends
  `repository_id` to include them.
- **Server-side hash**: excluded from the headline by default (`-client-hash`,
  sends `X-Content-Hash` computed with the server's BLAKE3 routine). Set
  `-client-hash=false` to fold server hashing into acceptance cost.
- **ML exclusion**: ML defaults **on** in `production` (`SERVER_ENV=production`
  seeds Semantic/BioCLIP/OCR/Face = true), so the tool explicitly disables all
  four via `PATCH /settings/system` in preflight and re-checks the ML queues are
  idle at the end (evidence in `summary.json` / `report.md`). It does not rely on
  Lumen simply being absent.

### Components built

- W1 harness scaffold: manifest gen, auth, repository resolve, concurrent
  uploader, completion poller, JSON/JSONL/Markdown output. ✅
- W2 resource sampling: `sample.sh` (docker stats + `pg_stat_*` via
  `docker exec`), spawned via `-sampler`. ✅ (Docker-stack focused; native
  `pidstat`/`iostat` sampler intentionally **not** built — operator decision.)
- W3 clean-room: preflight aborts if any River job is pending; README documents
  `down -v` reset. ✅
- W4 analyzer: single-run `summary.json` + `report.md` with publishability
  checklist. ✅ Multi-run aggregator (median / CI across ≥5 repeats)
  intentionally **not** built — operator decision; aggregate the per-run
  `summary.json` files by hand for now.
- **Exact timing (`-db`)**: optional Postgres DSN reads
  `river_job.finalized_at` (µs precision) for the two core jobs per asset,
  removing the poll-cadence bound (API polling and status JSONB `updated_at` are
  only 1 s). The release stack does not expose the DB port, so a committed
  benchmark overlay `docker-compose.release.dbport.yml` publishes `5432:5432`
  without touching the release images. ✅
- W5 calibration/publication: not started (needs the images + clean host).

### Operator decisions (2026-07-07)

- **This mixed set is now the standard dataset.** The provided `Sep 28 2025`
  set (374 JPG + 375 NEF, ~11 GB) is a personal travel record and is the only
  dataset; no separate JPEG-only headline set will be produced. The public claim
  is therefore a **mixed RAW+JPEG** number, not a pure-JPEG headline — the
  landing-page copy must say "mixed RAW+JPEG library" rather than imply a JPEG
  claim. Dataset J / Dataset R split above is superseded by this single set.
- **Do not over-tighten acceptance.** Keep 100%-complete + zero-error + ML-idle
  as gates, but the "separate JPEG-only headline set" and "multi-run aggregator"
  requirements are dropped for this effort.
- No per-format sub-number. RAW dominates latency by design; report the blended
  number and disclose the format mix.
- DB port exposed via the benchmark overlay only (release images unchanged); the
  point of "release" — running GHCR-built images — is preserved.
