# uploadbench — upload-to-photo-ready benchmark

A host-side benchmark for the Lumilio Photos upload pipeline with ML/AI
excluded. It drives a running server over HTTP and can optionally open its
SQLite library read-only for exact River timing, so it works against native
development or the production Compose stacks under `deploy/compose/`.

A photo is **photo-ready** only when its `metadata_asset` *and* `thumbnail_asset`
task states are both `complete`. HTTP upload acceptance is never treated as
completion.

## What it does

1. Builds an immutable `manifest.json` of the dataset (walk, filter by
   extension, size, and — in client-hash mode — BLAKE3 computed with the same
   routine the server uses so the trusted `X-Content-Hash` matches the stored
   asset hash).
2. Logs in, resolves the primary repository.
3. Preflight: reads settings, **disables ML** via the settings API, and asserts
   the River queues are empty (clean-room check).
4. Uploads all files at a fixed concurrency (`POST /api/v1/assets`), recording
   per-request timing and HTTP status.
5. Polls `POST /api/v1/assets/list` until every accepted upload is photo-ready
   (matched by original filename), recording first-observed completion time.
6. Snapshots queue summaries + ML settings again as ML-exclusion evidence.
7. Emits `manifest.json`, `events.jsonl`, `summary.json`, `report.md`, and (if a
   sampler is attached) `resource_samples.csv`.

## Profiles

- `-profile core` (default, **the headline number**): does **not** send
  `repository_id`, so the server does not enqueue `detect_stacks`. ML disabled.
  Completion predicate is metadata + thumbnail only. Natural side jobs from
  metadata (`rebuild_location_clusters`, `match_live_photo`) can still enqueue
  but do not block the predicate — they are reported separately in the queue
  table.
- `-profile default-nonml`: sends `repository_id` (triggers `detect_stacks`) to
  approximate a real library settling with AI off. ML still disabled.

## Run it

Prereqs: a running server, a **non-MFA** benchmark admin account, a **clean DB +
repository** (no prior assets/thumbnails/River jobs), and — for meaningful
numbers — machine controls (AC power, no sleep/backup/Spotlight on the repo).

```bash
cd server

# Smoke test (20 files) against local dev or the public production origin.
go run ./tools/uploadbench \
  -base https://photos.example.com \
  -dataset "/Volumes/CodeBase/Photography/Sep 28 2025" \
  -user admin -pass 'YOUR_PASSWORD' \
  -concurrency 8 -profile core -limit 20

# Full run with resource sampling of the production stack.
go run ./tools/uploadbench \
  -base https://photos.example.com \
  -dataset "/Volumes/CodeBase/Photography/Sep 28 2025" \
  -user admin -pass 'YOUR_PASSWORD' \
  -concurrency 8 -profile core \
  -sampler ./tools/uploadbench/sample.sh
```

Key flags: `-concurrency` (sweep 1/2/4/8/16/32 to find saturation),
`-profile`, `-disable-ml` (default true), `-client-hash` (default true),
`-timeout` (default 60m), `-limit` (cap files), `-run-id`, `-out`,
`-instant-pass` (below), `-sqlite` (below).

### Instant upload via `-instant-pass` (optional)

With `-instant-pass`, the harness re-uploads the whole dataset once the first
pass has drained. Every file should come back with status `duplicate`: the server
recognizes the content hash, skips staging, and enqueues no ingest job. The report
gains an **Instant upload** section with the duplicate count, wall time, and the
bytes that never crossed the wire.

`ingested` above zero in that section is a real failure signal, not noise. It
means the fingerprint the client sent for a file does not match the one the server
stored for the very same bytes — the two hash implementations have drifted apart.

### Exact timing via `-sqlite` (optional)

Without `-sqlite`, completion is detected by polling `/assets/list`, so per-asset
photo-ready latency is bounded by `-poll-interval` (default 1 s) — and the status
`updated_at` value is itself only second-precision. For **sub-second** per-asset
and makespan numbers, point `-sqlite` at the live library catalog; the tool opens
it read-only and reads `river_job.finalized_at` (millisecond-precision) for the
two core jobs per asset.

For native development, use the `database.path` value from the active manifest:

```bash
go run ./tools/uploadbench ... \
  -sqlite "/absolute/path/to/library.sqlite3"
```

When the server runs in a container, the catalog path must also be visible on
the host. Pass the host-side path from the app-state volume. Do not copy the
live database file; SQLite's WAL may contain committed pages that have not yet
been checkpointed.

`report.md`/`summary.json` record whether exact timing was used.

### Clean-room reset (production stack)

The tool aborts preflight if any River job is pending. Reset between runs:

```bash
# Stop the selected stack. App-state and media are bind mounts, so down -v is
# intentionally not treated as a data reset.
export COMPOSE_FILE=deploy/compose/compose.caddy.yml
docker compose down
# Point LUMILIO_STATE and LUMILIO_STORAGE at fresh, empty directories, generate
# a new complete manifest, then start the selected stack.
docker compose up -d
# Re-run first-time setup to recreate the admin user + primary repository.
```

## Concurrency sweep + 5 repetitions

Publishable numbers need the saturation point and >=5 repeats (report the
median, never the best). Example:

```bash
for c in 1 2 4 8 16 32; do
  # reset clean-room here (see above) between every run
  go run ./tools/uploadbench ... -concurrency $c -run-id "sweep-c$c"
done
```

Aggregate `summary.json` files across runs for median / p25-p75 / bootstrap CI.

---

## Testing the *latest code* via a production Compose stack

The benchmark tool itself runs on the host and does **not** ship in any image —
you only need to rebuild images when you want to benchmark **server** changes.
The release stack pulls the published unified GHCR image
(`ghcr.io/edwinzhancn/lumilio-server`).

`.github/workflows/release-docker.yml` publishes images on:

- **Git tag `v*`** → `:{version}`, `:{major}.{minor}`, and `:latest` (stable).
- **`workflow_dispatch`** (manual) → `:edge`.

### Push code and pull the corresponding images

```bash
# 1. Commit + push your branch (adds this tool; server images unaffected).
git add server/tools/uploadbench
git commit -m "feat(bench): upload-to-photo-ready benchmark harness"
git push origin main            # or your branch

# 2a. Build images from the current commit WITHOUT a release tag:
#     trigger the workflow manually -> images tagged :edge.
gh workflow run "Release Docker" --ref main
gh run watch "$(gh run list --workflow 'Release Docker' -L1 --json databaseId -q '.[0].databaseId')"

# 2b. OR cut a real release tag -> images tagged :latest + version.
# git tag v1.2.3 && git push origin v1.2.3

# 3. Pull + start the stack on the benchmark host.
export LUMILIO_STORAGE=/srv/lumilio-bench       # fresh media root
export LUMILIO_STATE=/srv/lumilio-bench-state   # fresh app-state root
export LUMILIO_IMAGE=ghcr.io/edwinzhancn/lumilio-server:edge
export COMPOSE_FILE=deploy/compose/compose.caddy.yml
docker compose pull
docker compose up -d

# 4. Wait for health, complete first-time setup (creates admin + primary repo),
#    then run the benchmark against its public HTTPS origin.
docker compose ps
```

Notes:
- The production image serves API and Web from one public HTTPS origin.
- `sample.sh` records container resource usage with `docker stats`. Exact
  per-job timing (`-sqlite`) reads the host-visible app-state catalog directly.
- The release image installs `libraw23`, so NEF thumbnails resolve through the
  libraw embedded-preview path — RAW photos reach photo-ready in Docker.
