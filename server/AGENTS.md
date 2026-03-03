# Repository Guidelines

## Runtime Contract (Read First)
- Use repo-root scripts for integration flows; do not boot services ad hoc.
- Backend-only dev loop:
  - `make db-wait`
  - `make server-dev`
- Full integration (DB + server + web): `make dev`.
- Compatibility matrix:
  - Go `1.24.x`
  - PostgreSQL `16` with `pgvector` (Docker service `db`)
  - River CLI `0.24.x` (`river --version` must work)
  - Media tools on PATH: `exiftool`, `ffmpeg`, `ffprobe`, `dcraw`

## Important Modules
- `server/cmd/main.go`: startup order, auto-migration, queue workers, watchman, ML bootstrap.
- `server/internal/api/router.go`: route map, auth boundaries, CORS policy.
- `server/internal/api/handler/*.go`: request validation + HTTP contracts.
- `server/internal/service/*.go`: business logic and ML adapters.
- `server/internal/processors/*.go`: async ingest/metadata/thumbnail/transcode pipeline.
- `server/internal/storage/*`: repository layout, staging, watchman integration.
- `server/internal/db/*` and `server/migrations/*`: schema and migration runtime.

## Common Tasks -> Entry Files
- Add/modify API endpoint: `internal/api/router.go` -> `internal/api/handler/*` -> `internal/service/*`.
- Add background job: `internal/queue/*worker.go` + `internal/processors/*` + `internal/queue/queue_setup.go`.
- Change repository storage behavior: `internal/storage/staging_manager.go` + `internal/storage/repocfg/*`.
- Update auth/session behavior: `internal/api/handler/auth_handler.go` + `internal/service/auth_service.go`.
- Change OpenAPI contract: annotate handlers, then run `make dto` from repo root.

## Frequent Bugs -> Fast Triage Path
- DB connection fails locally: check `server/.env.development` port (`5433` in local Docker) -> `internal/db/db.go`.
- Server starts but queue errors later: River migration not applied -> `internal/db/migration.go` and `river --version`.
- Boot fails with watchman enabled: invalid or empty `WATCHMAN_SOCK` -> `internal/storage/monitor/watchman_monitor.go`.
- Storage init failure on startup: `STORAGE_PATH` must be a storage root; primary repo is `<STORAGE_PATH>/primary` -> `cmd/main.go:initPrimaryStorage`.
- Browser API blocked in dev: origin must match current CORS config (`http://localhost:6657`) -> `internal/api/router.go`.

## Lumen SDK: How We Use It (`go doc` First)
- SDK pin: `github.com/edwinzhancn/lumen-sdk v1.1.3` (`server/go.mod`).
- Inspect docs quickly:
  - `cd server && go doc github.com/edwinzhancn/lumen-sdk/pkg/client LumenClient`
  - `cd server && go doc github.com/edwinzhancn/lumen-sdk/pkg/config`
  - `cd server && go doc github.com/edwinzhancn/lumen-sdk/pkg/types InferRequestBuilder`
- Standard usage pattern (see `internal/service/lumen_service.go`):
  1. Build config (`config.DefaultConfig()`) and client (`client.NewLumenClient`).
  2. Start client (`Start(ctx)`), close on shutdown.
  3. Build typed request (`types.NewInferRequest(...).ForEmbedding/ForOCR/...`).
  4. Execute with retry (`InferWithRetry`, `client.WithMaxRetries`, `client.WithMaxWaitTime`).
  5. Parse response via `types.ParseInferResponse(resp).As...()`.
