# Architecture

This is the compact system map for agents. Keep details here stable and useful; implementation plans belong in `exec-plans/`.

## Runtime Shape

- `docker-compose.yml` runs PostgreSQL, the Go server, and the Caddy-served web app.
- Linux is the standalone Server/Docker delivery target. macOS and Windows are
  Desktop App delivery targets; the App hosts the same complete `server/app`
  runtime in-process, so both Desktop CI jobs run the full Server and Desktop
  test suites plus a native CGo build.
- Runtime state has three non-overlapping owners: frontend preferences in browser localStorage; runtime-mutable settings in PostgreSQL through Settings/Setup APIs; and runtime-immutable process configuration in a complete schema-versioned TOML manifest.
- First-run bootstrap (`fresh → db_rotated → admin_created → ready`) is an orthogonal state machine. It observes setup gates; it is not a fourth configuration source.
- `server/config/server.example.toml` is the complete local template; `server/config/server.container.toml` is the image manifest; `desktop/supervisor/server.template.toml` is the versioned desktop compiler input.
- Standalone requires `--config <path>`. Ordinary environment variables never override `AppConfig`; only CLI diagnostics and the explicit break-glass whitelist are single-run host controls.

## Backend

- `server/cmd/main.go`: thin entrypoint (flags, signals, break-glass env whitelist, strict manifest load) that calls `server/app`.
- `server/app`: the only server runtime — logging, migrations, queue workers, router, repository bootstrap, SPA serving, and graceful shutdown via `Run(ctx, cfg, controls)`. It rejects configuration not produced by the strict loader.
- `server/config`: leaf package exposing the sole production constructor `LoadAppConfig(path)`. It strictly decodes schema v1, resolves manifest-relative paths and secret files, validates the complete graph, and fingerprints the source bytes.
- `server/internal/api/router.go`: route map, auth boundaries, CORS.
- `server/internal/api/handler`: HTTP request/response layer.
- `server/internal/service`: business logic, auth, settings, indexing, search, cloud import, and ML/classifier adapters.
- `server/internal/processors`: ingest, metadata, thumbnail, transcode pipeline.
- `server/internal/queue`: River jobs and workers.
- `server/internal/storage`: repository layout, staging, scanner, repository config.
- `server/internal/sourcing`: unified ingest materialization for upload, scan, and cloud flows.
- `server/internal/db` and `server/migrations`: database runtime and schema changes.

## Frontend

- `web/ARCHITECTURE.md`: authoritative and boundary-enforced frontend ownership, feature vocabulary, dependency direction, and state-placement rules.
- `web/src/features/*`: domain features. User journeys live in named `flows/`; reusable server access in `api/`; React-free rules and codecs in `model/`; cross-flow state or persistence in `state/`; isolated technical capabilities in `modules/`.
- Feature route files are thin entries. Runtime imports between features go through the target feature's narrow `index.ts`, except the reviewed `assets/map` and `assets/picker` entries.
- `web/src/lib/http-commons`: generated OpenAPI types and typed API client.
- `web/src/contexts`: cross-cutting runtime capabilities and provider boundaries.
- `web/src/components`: reusable UI components.
- `web/src/wasm` and `web/src/workers`: checked-in `blake3`/`studio` browser bundles and worker entry points for compute-heavy paths.
- `wasm/*`: Rust source crates for `blake3-wasm`, `studio-wasm`, `thumbnail-wasm`, and `export-wasm`.

## Desktop

- `desktop/`: Wails v3 tray host; private PostgreSQL; runs `server/app` in-process
  and serves the React SPA at `localhost:6680`. Its supervisor compiles the
  versioned template to app-data `config/server.toml`, atomically writes it with
  mode `0600`, and reloads it through `LoadAppConfig`. See `desktop/README.md`.

## Contracts

- OpenAPI is the HTTP contract source of truth. Run `make dto` after backend API changes.
- Do not hand-edit generated OpenAPI artifacts.
- `storage.path` seeds repository defaults and suggests the first primary path
  (`<storage.path>/primary`); startup does not create repositories. Primary
  identity is explicit via `repositories.role`.
- ML/Lumen paths should degrade when features are disabled; media management should remain usable without external ML.
