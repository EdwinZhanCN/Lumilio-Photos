# Architecture

This is the compact system map for agents. Keep details here stable and useful; implementation plans belong in `exec-plans/`.

## Runtime Shape

- `docker-compose.yml` runs PostgreSQL, the Go server, and the Caddy-served web app.
- `server/config/server.example.toml` is the tracked template for runtime configuration.
- `server/config/server.local.toml` is ignored local runtime configuration; `make setup` creates it from the example if missing.
- `SERVER_ENV` selects runtime mode; env variables are for bootstrap, secrets, deployment wiring, and machine-specific overrides.

## Backend

- `server/cmd/main.go`: process startup, config load, logging, migrations, queue workers, router, repository bootstrap.
- `server/config`: TOML/env config boundary.
- `server/internal/api/router.go`: route map, auth boundaries, CORS.
- `server/internal/api/handler`: HTTP request/response layer.
- `server/internal/service`: business logic, auth, settings, indexing, search, ML adapters.
- `server/internal/processors`: ingest, metadata, thumbnail, transcode pipeline.
- `server/internal/queue`: River jobs and workers.
- `server/internal/storage`: repository layout, staging, scanner, repository config.
- `server/internal/db` and `server/migrations`: database runtime and schema changes.

## Frontend

- `web/src/features/*`: domain features.
- `web/src/lib/http-commons`: generated OpenAPI types and typed API client.
- `web/src/contexts`: cross-cutting app state.
- `web/src/components`: reusable UI components.
- `web/src/wasm` and workers: compute-heavy browser paths.

## Contracts

- OpenAPI is the HTTP contract source of truth. Run `make dto` after backend API changes.
- Do not hand-edit generated OpenAPI artifacts.
- Storage root means a root directory containing `primary`; the primary repository is `<storage.path>/primary`.
- ML/Lumen paths should degrade when features are disabled; media management should remain usable without external ML.
