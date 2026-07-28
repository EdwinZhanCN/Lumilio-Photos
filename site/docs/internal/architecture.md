# Architecture

This is the compact system map for contributors. Keep details here stable and
useful; implementation plans belong in `exec-plans/`.

## Runtime Shape

- Docker production on Linux explicitly selects
  `deploy/compose/compose.caddy.yml`, `deploy/compose/compose.acme.yml`, or
  `deploy/compose/compose.proxy.yml`. All three use host networking; there is no
  Docker plaintext-development stack. The Go process owns the embedded SQLite
  catalog and serves both the API and built React SPA.
- Linux is the standalone Server/Docker delivery target. macOS and Windows are
  Desktop App delivery targets; the App hosts the same complete `server/app`
  runtime in-process, so both Desktop CI jobs run the full Server and Desktop
  test suites plus a native CGo build.
- Runtime state has three non-overlapping owners: frontend preferences in browser localStorage; runtime-mutable settings in the SQLite catalog through Settings/Setup APIs; and runtime-immutable process configuration in a complete schema-versioned TOML manifest.
- First-run bootstrap (`fresh → catalog_ready → admin_created → ready`) is an orthogonal state machine. It observes owner and primary-repository gates; it is not a fourth configuration source.
- `server/config/examples/` holds one complete manifest per deployment scenario
  (`dev/`, `desktop/`, `docker/`), generated from `server/config/profiles.go`.
  Because TOML comments cannot express conditional legality, a valid manifest
  per scenario is what documents the matrix; `dev/vite.toml` seeds the local
  file. Container images ship no bootable default manifest. Production
  manifests are generated into app-state by `server config init`, and
  `desktop/supervisor/server.template.toml` is the versioned desktop compiler
  input.
- Standalone requires `--config <path>`. Ordinary environment variables never override `AppConfig`; only CLI diagnostics and the explicit break-glass whitelist are single-run host controls.

## Backend

- `server/cmd/main.go`: thin entrypoint (flags, signals, break-glass env whitelist, strict manifest load) that calls `server/app`.
- `server/app`: the only server runtime — logging, migrations, queue workers, router, repository bootstrap, SPA serving, and graceful shutdown via `Run(ctx, cfg, controls)`. It rejects configuration not produced by the strict loader.
- `server/config`: leaf package exposing the runtime constructor
  `LoadAppConfig(path)` plus a one-shot complete-manifest generator. It strictly
  decodes schema v3, resolves manifest-relative paths and secret files,
  validates the complete graph, and fingerprints the source bytes.
- `server/internal/httporigin`: canonical request/proxy/browser Origin policy.
- `server/internal/servertransport`: off, external, and CertMagic ACME listener lifecycle.
- `server/internal/api/router.go`: route map, auth boundaries, CORS.
- `server/internal/api/handler`: HTTP request/response layer.
- `server/internal/service`: business logic, auth, settings, indexing, search, cloud import, and ML/classifier adapters.
- `server/internal/processors`: ingest, metadata, thumbnail, transcode pipeline.
- `server/internal/queue`: River jobs and workers.
- `server/internal/storage`: repository layout, staging, scanner, repository config.
- `server/internal/sourcing`: unified ingest materialization for upload, scan, and cloud flows.
- `server/internal/db` and `server/migrations`: the single-writer SQLite runtime,
  application/River migrations, Online Backup snapshots, FTS5, and sqlite-vec
  derived indexes.
- `server/internal/search/bleveocr`: rebuildable OCR search sidecar. SQLite OCR
  rows remain authoritative; a revision outbox feeds
  `<sqlite-directory>/indexes/bleve/ocr-v1/`.

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

- `desktop/`: Wails v3 tray host; runs `server/app` and the machine-local
  `library.sqlite3` in-process and serves the React SPA at `localhost:6680`.
  Its supervisor owns the host-lifetime lock and one runtime generation.
  App-data `config/runtime.toml` is the persistent schema-v3 intent; Desktop
  validates it with `LoadAppConfigBytes`, projects host-owned paths, and writes
  the immutable per-generation `config/server.toml` with mode `0600`.
  Candidate apply is journaled and readiness-gated, with
  `runtime.last-known-good.toml` rollback and launch-time reconciliation. The
  host lock is acquired before settings migration or Wails UI construction.
  Recovery reads invalid active intent as raw, fingerprinted control-plane data
  while every replacement candidate still passes the strict Server loader. The
  private Wails Control Panel consumes the supervisor's typed runtime snapshot
  and remains available after recoverable startup failures. See
  `desktop/README.md`.

## Contracts

- OpenAPI is the HTTP contract source of truth. Run `make dto` after backend API changes.
- Do not hand-edit generated OpenAPI artifacts.
- `storage.path` is registered at startup as the non-removable default Storage
  Location, identified by `.lumilioroot`; startup does not create repositories.
  Web creation selects a registered `root_id`, while the Desktop Control Panel
  alone can authorize host paths or attach `.lumiliorepo` directories.
- The SQLite catalog, cloud sessions, secrets, logs, and database backups are app-private state and
  must be configured outside `storage.path`. Repository staging remains inside
  its repository under `.lumilio/staging`.
- ML/Lumen paths should degrade when features are disabled; media management should remain usable without external ML.
