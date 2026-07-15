# Backend

This document describes the current Go backend as implemented in `server/`.

## Runtime Entry

- Main process: `server/cmd/main.go`.
- Config package: `server/config`.
- Tracked config template: `server/config/server.example.toml`.
- Ignored local config file: `server/config/server.local.toml`.
- Docker image: `server/Dockerfile`.
- Database image: `server/db.Dockerfile`.

Startup ownership is split between the thin CLI host and the shared app runtime:

1. `server/cmd/main.go` requires `--config <path>`, strictly loads that complete
   TOML manifest, collects explicit single-run operator controls, and hands the
   resolved `config.AppConfig` to `app.Run(ctx, cfg, controls)`.
2. `server/app.Run` owns the actual runtime bootstrap:
   - initialize logging
   - start libvips runtime
   - run database migrations
   - open PostgreSQL pool and generated query layer
   - initialize settings, repository storage, River queues, ML services, processors, handlers, and router
   - start the HTTP server on `server.port`

## Configuration Boundary

Runtime-immutable configuration is a complete schema v1 manifest, not a defaults
or override layer. Missing, unknown, legacy, contradictory, or invalid fields
fail startup. Relative paths use the manifest directory. Startup logs the
absolute path, schema version, and source SHA-256 without logging secret content.

Desktop is a host wrapper, not a second server bootstrap: the supervisor prepares
private PostgreSQL, app-data paths, secrets, bundled media tools, and the SPA
root, compiles `desktop/supervisor/server.template.toml`, atomically writes
app-data `config/server.toml` with mode `0600`, reloads it through the same
strict loader, then calls `app.Run`. A write or reload error blocks startup.

TOML contains all immutable database/server/logging/storage/scanner/geocoding/
auth/transcode/Lumen/tool decisions. Database bootstrap, rotated database, and
app root secrets are file references only. Bootstrap must be readable and
non-empty; rotated may be absent until setup; the app key may be created at its
explicit path. No secret value appears in TOML, generated desktop manifests, or
logs.

Standalone accepts diagnostics through `--pprof-addr` and
`--agent-audit-log`. Only `LUMILIO_BREAK_GLASS` and
`LUMILIO_BREAK_GLASS_USERNAME` remain as product runtime env controls, read by
the CLI host and passed separately from `AppConfig`. Desktop resource-location
env, test/conformance opt-ins, and third-party container env are host/harness
contracts, not server configuration.

## Important Packages

- `internal/api/router.go`: Gin route tree, CORS, auth boundaries.
- `internal/api/handler`: HTTP handlers and request/response wiring.
- `internal/api/dto`: API DTO types.
- `internal/service`: business services for auth, assets, settings, search, locations, faces, species, indexing, duplicate detection, cloud import, and Lumen/LLM/classifier integration.
- `internal/processors`: ingest, metadata, thumbnail, transcode, retry, and asset processing tasks.
- `internal/queue`: River queue setup and worker implementations.
- `internal/db`: database connection, migrations, generated sqlc repo layer.
- `internal/storage`: repository manager, staging manager, repository config, scanner.
- `internal/cloud`: cloud ingest and sync providers.
- `internal/sourcing`: unified ingest materialization for upload, repository scans, and cloud sync.
- `internal/classify`: classifier support code shared by API/service paths.
- `internal/logging`: zap logger setup, stdlib bridge, and repository audit helpers.
- `internal/agent`: agent service and tools.
- `internal/utils`: media, hashing, raw, exif, upload, imaging, and support utilities.

## Storage Model

`storage.path` is the default repository root used to seed runtime repository
defaults. Startup does not create a repository there. During authenticated
first-run setup, the primary repository form defaults to:

```text
<storage.path>/primary
```

Repository identity is explicit in the database through `repositories.role`
(`primary` or `regular`). The app is fully initialized only when database
credential setup is complete, an admin exists, and exactly one active primary
repository exists. Repository config lives in `.lumiliorepo` files and is handled
by `internal/storage/repocfg`.

Repositories are unowned shared storage; per-user visibility and mutation
authorization run entirely on `assets.owner_id`.

## Database And API Contracts

- Migrations live in `server/migrations`.
- Generated sqlc code lives under `server/internal/db/repo`.
- Generated OpenAPI output lives in `server/docs`.
- Frontend generated types live in `web/src/lib/http-commons/schema.d.ts`.

After API changes, run:

```bash
make dto
```

After SQL schema or SQL queries change, run:

```bash
cd server && sqlc generate
```

Do not hand-edit generated OpenAPI or frontend schema artifacts.

Swag v2 currently emits an extra empty-object `oneOf` branch for body
parameters in OpenAPI 3.1. The frontend DTO generator removes that branch only
for required JSON request bodies in memory, then generates `schema.d.ts`.
Optional empty payloads remain optional; backend annotations and DTO validation
tags still define the contract.

> **If the frontend is casting (`as { ... }`) around a response, the contract is
> the bug — not the frontend.** Either the handler's `@Success ... {data=dto.X}`
> annotation is missing/points at the wrong DTO, or the DTO is correct and the
> generated artifacts are stale (`make dto` was not re-run). Fix the annotation /
> DTO and regenerate; never let the frontend cast around a typed endpoint. If
> generated `schema.d.ts` exposes `data?: Record<string, never>` or
> `data?: unknown` for an endpoint that returns payload data, that is a contract
> failure: fix backend DTO/annotation/codegen before frontend work proceeds. Do
> not add frontend compatibility shims for stale DTOs. A stale `make dto` once
> let `dto.OptionsResponseDTO.camera_models` surface to the SPA as an untyped
> `Record<string, never>`, so a frontend cast guessed `cameras` and silently
> broke a feature.

## Queues And Processing

River worker counts and queue config live in `internal/queue/queue_setup.go`. Worker registration happens in `server/app/app.go`, and the implementations live in `internal/queue`. The processing pipeline uses services and processors for:

- asset ingest and discovery
- cloud import materialization
- metadata extraction
- thumbnail generation
- video/audio transcoding
- location clustering
- repository scans
- stack and live photo analysis
- perceptual hashing
- ML tasks through Lumen, including BioCLIP, OCR, face, semantic indexing, and zero-shot classifier tagging

Accepted uploads expose their user-scoped ingest lifecycle at
`GET /api/v1/assets/batch/jobs?task_ids=…`. Frontend upload completion means the
River ingest job reached a terminal state, not merely that multipart transport
returned 2xx. Repository scans expose run lifecycle through the existing
`/api/v1/repositories/{id}/scans/latest` endpoint.

`GET /api/v1/assets/map-points` accepts an optional complete
`south,north,west,east` WGS-84 viewport. All four values must be supplied
together; longitude bounds support antimeridian crossing. This keeps map
rendering proportional to the visible region instead of the full GPS library.

## ML, Lumen, And LLM

Photos maps every Lumen SDK field it consumes directly from `[lumen]`; it never
calls SDK defaults or env loading. ML and LLM feature settings remain
runtime-mutable PostgreSQL settings and do not belong in `AppConfig`. Zero-shot
classifier preview is exposed through `/api/v1/classifiers/preview`.

The app should remain useful when ML/LLM features are disabled.

## Quality Gate

Backend gate:

```bash
make server-test
```

Use the Makefile target by default. It exports the local cgo flag allowlist
needed by media dependencies on macOS. Only run the direct command when you have
a concrete reason and preserve the same environment:

```bash
cd server && go test ./...
```

Run `gofmt` on changed Go files.
