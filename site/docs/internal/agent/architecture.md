# Architecture

This is the compact system map for agents. Keep details here stable and useful; implementation plans belong in `exec-plans/`.

## Runtime Shape

- `docker-compose.yml` runs PostgreSQL, the Go server, and the Caddy-served web app.
- `server/config/server.example.toml` is the tracked template for runtime configuration.
- `server/config/server.local.toml` is ignored local runtime configuration; `make setup` creates it from the example if missing.
- `SERVER_ENV` selects normal server runtime mode; env variables are for bootstrap, secrets, deployment wiring, and machine-specific overrides. Desktop builds construct a typed config through `server/config` instead of booting from `SERVER_CONFIG_FILE`.

## Backend

- `server/cmd/main.go`: thin entrypoint (signal handling + normal TOML/env config load) that calls `server/app`.
- `server/app`: the only server runtime — logging, migrations, queue workers, router, repository bootstrap, SPA serving, and graceful shutdown via `Run(ctx, cfg)`. Imported by the CLI and, in-process, by the desktop supervisor.
- `server/config`: single config schema/defaults boundary, including normal TOML/env loading, external tool paths, and `NewDesktopConfig` for the desktop host. Desktop may write a generated TOML debug copy, but runtime uses typed config.
- `server/internal/api/router.go`: route map, auth boundaries, CORS.
- `server/internal/api/handler`: HTTP request/response layer.
- `server/internal/service`: business logic, auth, settings, indexing, search, cloud import, and ML/classifier adapters.
- `server/internal/processors`: ingest, metadata, thumbnail, transcode pipeline.
- `server/internal/queue`: River jobs and workers.
- `server/internal/storage`: repository layout, staging, scanner, repository config.
- `server/internal/sourcing`: unified ingest materialization for upload, scan, and cloud flows.
- `server/internal/db` and `server/migrations`: database runtime and schema changes.

## Frontend

- `web/src/features/*`: domain features.
- `web/src/lib/http-commons`: generated OpenAPI types and typed API client.
- `web/src/contexts`: cross-cutting app state.
- `web/src/components`: reusable UI components.
- `web/src/wasm` and `web/src/workers`: checked-in `blake3`/`studio` browser bundles and worker entry points for compute-heavy paths.
- `wasm/*`: Rust source crates for `blake3-wasm`, `studio-wasm`, `thumbnail-wasm`, and `export-wasm`.

## Contracts

- OpenAPI is the HTTP contract source of truth. Run `make dto` after backend API changes.
- Do not hand-edit generated OpenAPI artifacts.
- `storage.path` seeds repository defaults and suggests the first primary path
  (`<storage.path>/primary`); startup does not create repositories. Primary
  identity is explicit via `repositories.role`.
- ML/Lumen paths should degrade when features are disabled; media management should remain usable without external ML.
