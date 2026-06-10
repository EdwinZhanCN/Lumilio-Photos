# Backend

This document describes the current Go backend as implemented in `server/`.

## Runtime Entry

- Main process: `server/cmd/main.go`.
- Config package: `server/config`.
- Tracked config template: `server/config/server.example.toml`.
- Ignored local config file: `server/config/server.local.toml`.
- Docker image: `server/Dockerfile`.
- Database image: `server/db.Dockerfile`.

Startup order in `cmd/main.go`:

1. Load `.env`, then TOML config, then apply env overrides.
2. Pass the resolved `config.AppConfig` to `server/app.Run(ctx, cfg)`.
3. Initialize logging.
4. Start libvips runtime.
5. Run database migrations.
6. Open PostgreSQL pool and generated query layer.
7. Initialize settings, repository storage, River queues, ML services, processors, handlers, and router.
8. Start the HTTP server on `server.port`.

## Configuration Boundary

Runtime defaults belong in `server/config/server.example.toml`; local runtime choices belong in the ignored `server/config/server.local.toml`. Env is for bootstrap, secrets, deployment wiring, and machine-specific overrides.

Desktop is a host wrapper, not a second server bootstrap: the supervisor prepares
private PostgreSQL, app-data paths, secrets, bundled media tools, and the SPA
root, then calls `config.NewDesktopConfig` and `app.Run(ctx, cfg)`. The generated
desktop `server.local.toml` under app data is a debug copy only and is not used
as the runtime input.

Keep in env:

- `SERVER_ENV`
- `SERVER_CONFIG_FILE`
- `SERVER_PORT`
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL`
- `STORAGE_PATH` when the local or mounted path differs
- `LUMILIO_SECRET_KEY`
- `LLM_API_KEY` and other provider secrets

Keep in TOML:

- `[database].password_file` as the path to the rotated local database password
- logging format and log directory
- CORS defaults
- storage strategy and duplicate handling
- repository scan cadence
- geocoding defaults
- ML task defaults, including zero-shot classification
- auth token TTLs and WebAuthn defaults
- transcoding mode
- Lumen discovery defaults
- external tool paths under `[tools]` for `exiftool`, `ffmpeg`, and `ffprobe`

`config.ApplyRuntimeEnvDefaults` exists to keep older package-level `os.Getenv` reads working after TOML is parsed. Prefer passing typed config into new code.

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

`storage.path` is a storage root. The primary repository is always initialized at:

```text
<storage.path>/primary
```

The startup path rejects a legacy repository directly at the storage root. Repository config lives in `.lumiliorepo` files and is handled by `internal/storage/repocfg`.

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

## Queues And Processing

River workers are registered in `cmd/main.go` and implemented in `internal/queue`. The processing pipeline uses services and processors for:

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

## ML, Lumen, And LLM

Lumen config is loaded by the Lumen SDK during `initMLServices`. ML feature switches are stored in settings and seeded from config on first initialization, including the zero-shot classifier toggle.

LLM settings are represented by `config.LLMConfig`, persisted through the settings service, and validated through `internal/llm`. Zero-shot classifier preview is exposed through `/api/v1/classifiers/preview` and backed by the classifier service.

The app should remain useful when ML/LLM features are disabled.

## Quality Gate

Backend gate:

```bash
make server-test
```

Equivalent:

```bash
cd server && go test ./...
```

Run `gofmt` on changed Go files.
