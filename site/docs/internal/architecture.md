# Architecture

This is the compact system map for contributors. Keep details here stable and
useful; implementation plans belong in `exec-plans/`.

## Runtime Shape

- Docker production on Linux starts with `deploy/compose/compose.yml`, a
  zero-input host-network HTTP deployment at port 6680. Optional
  `caddy.compose.yml` and `acme.compose.yml` add HTTPS. The standalone
  `dev.compose.yml` builds the current checkout in the same host-network runtime
  shape with Docker-managed development volumes. The Go process owns the
  embedded SQLite catalog and serves both the API and built React SPA.
- Linux is the standalone Server/Docker delivery target. macOS and Windows are
  Desktop App delivery targets; the App hosts the same complete `server/app`
  runtime in-process, so both Desktop CI jobs run the full Server and Desktop
  test suites plus a native CGo build.
- Runtime state has three non-overlapping owners: frontend preferences in browser localStorage; runtime-mutable settings in the SQLite catalog through Settings/Setup APIs; and runtime-immutable process configuration in a complete schema-versioned TOML manifest.
- First-run bootstrap (`fresh → catalog_ready → admin_created → ready`) is an orthogonal state machine. It observes owner and primary-repository gates; it is not a fourth configuration source.
- `server/config/examples/` holds one complete manifest per deployment scenario
  (`dev/`, `desktop/`, `docker/`), generated from `server/config/profiles.go`.
  Because TOML comments cannot express conditional legality, a valid manifest
  per scenario is what documents the matrix; `task dev` renders `dev-vite`
  into `.local/dev/config/server.toml`. Container images ship complete
  `docker-http` and `docker-caddy` manifests; ACME and custom operator
  manifests are generated into app-state by `server config init`. The
  Desktop keeps a complete schema-v4 runtime intent and projects it through
  the same strict `server/config` loader before calling `server/app`.
- Standalone requires `--config <path>`. Ordinary environment variables never override `AppConfig`; only CLI diagnostics and the explicit break-glass whitelist are single-run host controls.

## Backend

- `server/cmd/main.go`: thin entrypoint (flags, signals, break-glass env whitelist, strict manifest load) that calls `server/app`.
- `server/app`: the only server runtime — logging, migrations, queue workers, router, repository bootstrap, SPA serving, and graceful shutdown via `Run(ctx, cfg, controls)`. It rejects configuration not produced by the strict loader.
- `server/config`: leaf package exposing the runtime constructor
  `LoadAppConfig(path)` plus a one-shot complete-manifest generator. It strictly
  decodes schema v4, resolves manifest-relative paths and secret files,
  validates the complete graph, and fingerprints the source bytes.
- `server/internal/httporigin`: request-derived target/browser Origin policy
  and trusted-proxy client-IP recovery.
- `server/internal/servertransport`: plaintext and CertMagic ACME listener lifecycle.
- `server/internal/api/router.go`: route map, auth boundaries, CORS.
- `server/internal/api/handler`: HTTP request/response layer.
- `server/internal/service`: business logic, auth, settings, indexing, search, cloud import, and ML/classifier adapters.
- `server/internal/processors`: ingest, metadata, thumbnail, transcode pipeline.
- `server/internal/queue`: River jobs and workers.
- Event topology is owner-wide and derived from logical `media_item` facts.
  `source_revision`/`published_revision` and the shared Event resolver are the
  lifecycle authority; repository Browse Scope is applied only as a read
  projection.
- Owner scope is explicit at topology boundaries: a generic administrator
  asset browse may omit `OwnerID` to view the whole library, but an
  owner-scoped topology such as Event must always carry its resolved owner into
  downstream asset queries. `nil` must never be used to infer an Event owner.
- `server/internal/storage`: repository layout, staging, scanner, repository config.
- `server/internal/sourcing`: unified ingest materialization for upload, scan, and cloud flows.
- `server/internal/db` and `server/migrations`: the single-writer SQLite runtime,
  application/River migrations, Online Backup snapshots, FTS5, and the
  statically linked SQLite Vec1 derived semantic index.
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

- `desktop/`: Wails v3 tray host with a private React Settings window. `main.go`
  only assembles services; `internal/host` owns Wails application/window/tray
  adapters, while `internal/state` owns the immutable revisioned snapshot and
  `internal/operation` owns request-id idempotency and aggregate mutation gates.
- `internal/runtime` calls `server/app.Run` in-process and owns one guarded
  Server generation. `RuntimeReady` and `RepositoryManagerReady` are the typed
  readiness/Storage handoffs; Desktop never proves ownership by probing its own
  HTTP listener. The product SPA remains in the system browser.
- `internal/lumen` is an optional supervised child-process boundary with signed,
  staged artifacts, an owner lock, parent-liveness contract, and platform
  process-tree termination. Its install, desired, and process states remain
  separate in the snapshot.
- `internal/runtime/runtimeconfig`, `internal/resources`, and
  `internal/update` use schema-versioned metadata, atomic files, fingerprints,
  signatures, and explicit journals. `internal/storage` exposes only the typed
  repository handoff and a discardable shortcut cache; no private HTTP bridge
  exists.
- Desktop onboarding materialises a complete candidate from the explicit
  `desktop-local` profile, substitutes OS-owned paths, and runs the same strict
  loader before exposing a small structured projection to React. Draft reads
  and patches do not persist intent; Save/Apply is the only pointer-changing
  boundary. The Settings UI is a sidebar-free Linear-style single column with
  beUI controls and a six-destination Dock; full TOML is an optional Advanced
  recovery surface, not a first-run requirement.

## Contracts

- OpenAPI is the HTTP contract source of truth. Regeneration:
  [lumilio-api-contract-change](../../../.agents/skills/lumilio-api-contract-change/SKILL.md).
- Do not hand-edit generated OpenAPI artifacts.
- `storage.path` is registered at startup as the non-removable default Storage
  Location, identified by `.lumilioroot`; startup does not create repositories.
  Web creation selects a registered `root_id`, while the Desktop Control Panel
  alone can authorize host paths or attach `.lumiliorepo` directories.
- The SQLite catalog, cloud sessions, secrets, logs, and database backups are app-private state and
  must be configured outside `storage.path`. Repository staging remains inside
  its repository under `.lumilio/staging`.
- Registered repository I/O is rooted by `internal/storage.RepositoryFS` and
  serialized against repository relocation/removal. Durable asset and job paths
  are canonical repository-relative values; native codecs receive absolute
  filenames only through the explicit local-path adapter.
- The application SQLite catalog owns the rebuildable `repository_file_index`.
  Scans include `inbox/`, exclude `.lumilio/`, preserve asset identity only for
  unique full-BLAKE3 move matches, and require two authoritative absences before
  soft deletion.
- ML/Lumen paths should degrade when features are disabled; media management should remain usable without external ML.
