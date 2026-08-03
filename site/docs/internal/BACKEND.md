# Backend

This document describes the current Go backend as implemented in `server/`.

## Runtime Entry

- Main process: `server/cmd/main.go`.
- Config package: `server/config`.
- Tracked config templates: `server/config/examples/{dev,desktop,docker}/*.toml`,
  generated from the profile table in `server/config/profiles.go` by
  `task config:examples`. Never hand-edit them; a golden test enforces it.
- Manifest JSON Schema: `server/config/schema/lumilio-server.schema.json`,
  reflected from the manifest struct and referenced by each example's
  `#:schema` directive. It covers presence, types, and closed value sets only —
  conditional legality stays in `resolveManifest`.
- Generated development config:
  `.local/dev/config/server.toml`. `task dev` keeps the catalog, indexes, logs,
  secrets, cloud state, backups, and media under the same `.local/dev/`
  instance root while preserving the state/storage boundary.
- Docker image: `server/Dockerfile`.
- Linux production Compose files:
  `deploy/compose/compose.{caddy,acme,proxy}.yml`; all use host networking.
- Browser E2E Compose files: `web/e2e/compose.yml` plus the CI cache overlay
  `web/e2e/compose.ci.yml`.

Startup ownership is split between the thin CLI host and the shared app runtime:

1. `server/cmd/main.go` requires `--config <path>`, strictly loads that complete
   TOML manifest, collects explicit single-run operator controls, and hands the
   resolved `config.AppConfig` to `app.Run(ctx, cfg, controls)`.
2. `server/app.Run` owns the actual runtime bootstrap:
   - initialize logging
   - start libvips runtime
   - open the single-writer SQLite catalog
   - run application and River migrations
   - construct the generated query layer
   - initialize settings, repository storage, River queues, ML services, processors, handlers, and router
   - start `internal/servertransport` from `server.listen` and `server.tls.mode`

## Configuration Boundary

Runtime-immutable configuration is a complete schema v3 manifest, not a defaults
or override layer. Missing, unknown, legacy, contradictory, or invalid fields
fail startup. Relative paths use the manifest directory. Startup logs the
absolute path, schema version, and source SHA-256 without logging secret content.

Desktop is a host wrapper, not a second server bootstrap: the Wails Host prepares
private app-data paths and calls `server/app.Run` in-process only after the
complete runtime intent has passed `LoadAppConfigBytes`. The RuntimeController
owns one generation and receives typed `RuntimeReady` and
`RepositoryManagerReady` handoffs; it does not use a loopback health probe as an
ownership proof. A write, journal, or strict reload error leaves Tray and
Settings available for recovery.

Desktop settings are host choices only; runtime intent remains a complete,
schema-versioned TOML document. Candidate edits are fingerprinted and guarded
by aggregate/version and base-fingerprint checks. Apply journals prepared,
stopping, candidate-selection, rollback, and committing phases, proves the old
generation has released ownership, and promotes last-known-good only after the
new generation is ready. Shortcut cache corruption is quarantined and does not
block the Host. The Wails single-instance boundary protects the Host, while the
RuntimeController generation guard protects SQLite/listener ownership.

Browser and WebAuthn identity are derived from each request.
`internal/httporigin` normalizes `Forwarded`/`X-Forwarded-*` target metadata
and the browser `Origin`; `server.proxy.trusted_cidrs` is used only to recover
the client IP from `X-Forwarded-For`. `internal/servertransport` owns plaintext
and ACME listeners. Passkey challenges bind the normalized request Origin and
its hostname RP ID, then verification checks the current request against that
same pair.

The standalone Server/Docker distribution is supported on Linux. macOS and
Windows ship the Desktop App rather than a separately operated Server, but that
does not narrow their backend test surface: the App embeds the complete runtime,
so both Desktop platforms run all Server tests in their native CI environment.

TOML contains all immutable database/server/logging/storage/scanner/geocoding/
auth/transcode/Lumen/tool decisions. `[database]` contains only the explicit
persistent catalog path. The application secret is a file reference and may be
created at that exact path on first start. No secret value appears in TOML,
generated desktop manifests, or logs.

Standalone accepts diagnostics through `--pprof-addr` and
`--agent-audit-log`. Agent ref hot memory is bounded by the single-run
`--agent-ref-user-hot-budget-mib` and
`--agent-ref-global-hot-budget-mib` controls (64 MiB and 512 MiB by default);
the global value must be at least the per-user value. Only `LUMILIO_BREAK_GLASS` and
`LUMILIO_BREAK_GLASS_USERNAME` remain as product runtime env controls, read by
the CLI host and passed separately from `AppConfig`. Desktop resource-location
env, test/conformance opt-ins, and third-party container env are host/harness
contracts, not server configuration.

The optional pprof listener belongs to the outer `app.Run` host lifecycle. It
is started once, remains stable across in-process database restore generations,
and is shut down when the host run exits. Desktop lifecycle state is projected
through typed Wails bindings and `desktop:snapshot-changed` revision notices;
the Settings WebView never calls the Server HTTP API.

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
- `internal/event`: owner-scoped Event candidates, deterministic `events-v1`
  segmentation/reconciliation, correction transactions, resolution, and direct
  typed relations. Event membership atoms are always `media_item` rows.
- `internal/utils`: media, hashing, raw, exif, upload, imaging, and support utilities.

## Storage Model

`storage.path` is the non-removable default Storage Location. Startup creates
or validates its portable `.lumilioroot` marker and associates legacy child
repositories, but does not create a repository. During authenticated first-run
setup, the primary repository defaults to:

```text
<storage.path>/primary
```

Repository identity is explicit in the database through `repositories.role`
(`primary` or `regular`). The app is fully initialized only when database
catalog is open, an admin exists, and exactly one active primary
repository exists. Repository config lives in `.lumiliorepo` files and is handled
by `internal/storage/repocfg`.

Additional Storage Locations are rows in `repository_roots`, keyed by the UUID
stored in `.lumilioroot`. The Web API accepts `root_id` for repository creation
and never an arbitrary host path. External root grants, existing repository
attachment, and explicit moved-vs-copied conflict resolution are Desktop-only
operations performed through the native picker and in-process control plane.

The SQLite catalog, `storage.cloud_state_path`, `storage.backups_path`, auth
secrets, and logs are machine-bound app state and must be outside
`storage.path`. A media root can therefore move without carrying credentials,
cloud sessions, or the database backup policy with it.

Repositories are unowned shared storage; per-user visibility and mutation
authorization run entirely on `assets.owner_id`. The first account is the Host
Owner. Every repository uses that same account as `default_owner_id`, solely as
the fallback for filesystem scans or other ingest sources without an explicit
user. Uploads keep their initiating user. Cloud credentials belong to the user
who connected the account; repository cloud bindings retain that identity as
the stable imported-asset owner, and import runs snapshot it. Administrators
may manage every credential while regular users can only access their own.
Owner identity is instance-local database policy rather than portable
`.lumiliorepo` metadata.

## Database And API Contracts

- `github.com/mattn/go-sqlite3` is the only database driver. `internal/db.Open`
  fixes the writer pool at one connection, registers the vendored SQLite Vec1
  0.7 extension statically, and applies fixed pragmas to every physical
  connection through driver DSN options plus a connection hook. Startup reads
  the effective values back and fails closed if policy differs.
- Application tables and River queues share the same catalog and can commit
  business state plus `InsertTx` jobs in one short `database/sql` transaction.
- Migration `000004_media_semantic_events` adds stable Events, membership,
  correction constraints, one-hop redirects, factual dirty ranges, and
  per-owner rebuild state without changing schema generation 4. The
  `rebuild_events` River queue is single-worker; owner jobs deduplicate only in
  non-running states so one follower can collect changes made during compute.
- Event reads, browse filters, shares, relations, and Agent refs resolve through
  the same owner-scoped Event service. Event shares and Agent refs materialize
  immutable displayable-asset snapshots; automatic membership uses no ML/AI
  signal.
- A running catalog must never be opened or copied through a host/container
  mount with another SQLite process. Host and container VFS locking is not a
  supported coordination boundary; use the application Online Backup flow, or
  inspect the catalog only after a graceful application stop.
- FTS5, the Vec1 semantic table, and the OCR Bleve sidecar are derived query
  structures; authoritative text and embedding data remains in ordinary
  application tables. Vec1 is exact-flat below 5,000 semantic rows and
  automatically trains a PQ ANN model at larger sizes; ANN candidates are
  filtered inside Vec1 and exact-reranked from authoritative BLOBs.
- OCR search lives at
  `<sqlite-directory>/indexes/bleve/ocr-v1/`. `ocr_results` and
  `ocr_text_items` remain authoritative; a revisioned SQLite outbox drives
  Bleve batch updates. Missing, corrupt, mapping-mismatched, and post-restore
  indexes are deleted and rebuilt before HTTP starts.
- Migrations live in `server/migrations`. The application migration ledger
  records SHA-256 for every applied SQL file; version, name, and checksum must
  continue to match embedded history, so historical migrations are immutable.
- SQLite restore is a generation boundary, not a live-handle operation. Its
  journal progresses through `staged`, `previous_preserved`,
  `active_installed`, `verified`, and `completed`; rollback has corresponding
  durable phases. Startup reconciles the marker with active/staged/previous/
  failed files after any interrupted rename.
- Generated sqlc code lives under `server/internal/db/repo`.
- Generated OpenAPI output lives in `server/docs`.
- Frontend generated types live in `web/src/lib/http-commons/schema.d.ts`.

After API changes, run:

```bash
task dto
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
> generated artifacts are stale (`task dto` was not re-run). Fix the annotation /
> DTO and regenerate; never let the frontend cast around a typed endpoint. If
> generated `schema.d.ts` exposes `data?: Record<string, never>` or
> `data?: unknown` for an endpoint that returns payload data, that is a contract
> failure: fix backend DTO/annotation/codegen before frontend work proceeds. Do
> not add frontend compatibility shims for stale DTOs. A stale `task dto` once
> let `dto.OptionsResponseDTO.camera_models` surface to the SPA as an untyped
> `Record<string, never>`, so a frontend cast guessed `cameras` and silently
> broke a feature.

## Browser Session And Cross-Origin Boundary

Short-lived access tokens remain explicit Bearer credentials. Long-lived
refresh credentials are host-only `HttpOnly` cookies named `lumilio_refresh`,
scoped to `/api/v1/auth`; they are never returned in JSON. Every successful
session creation or rotation also returns a CSRF proof derived from that
specific refresh credential. Refresh and logout require the proof in
`X-CSRF-Token`, and `GET /api/v1/auth/csrf` lets an existing cookie session
recover it after browser-readable state is cleared. Refresh rotation replaces
the cookie, the CSRF proof, and any previous browser session presented on a new
login.

Same-origin browser access is dynamic and zero-config, including direct LAN
addresses, public domains behind a reverse proxy, and the Desktop product Web
served with the API at `http://localhost:6680`.
`server.cors_allowed_origins` is only the exact allowlist for credentialed
cross-origin browser sessions. Unlisted origins may still call public or Bearer
API endpoints without cookies and receive wildcard CORS, but they never receive
`Access-Control-Allow-Credentials`. Browser requests that create, rotate, or
destroy cookie sessions must match the reconstructed request origin or that
allowlist. Reverse proxies must overwrite, rather than append client-supplied,
`X-Forwarded-Proto` and `X-Forwarded-Host`.

Desktop has two distinct browser surfaces. The private Wails Control Panel is
served by its own asset handler and calls only the host-owned `/__onb/*`
control plane; it does not participate in product accounts, refresh cookies,
server CORS, or this CSRF protocol. The React product Web runs in the user's
default browser and is served by `server/app` on the same localhost origin as
the API. The Control Panel receives the typed Supervisor runtime snapshot and
can validate/apply/restore runtime intent even when ordinary Server startup
fails. Desktop opens the product Web on `http://localhost:6680`; its local and
LAN profiles use the same request-derived Origin behavior as standalone Server.

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

Materialization owns staging files. A commit error is always returned to River
or the caller; failed quarantine never deletes the source. Existing physical
targets and instant-upload duplicates require exact size plus BLAKE3
verification before staging removal, and conflicts retain both files with a
structured recoverable ingest phase. HTTP/cloud callers must not add their own
error cleanup around this boundary.

`GET /api/v1/assets/map-points` accepts an optional complete
`south,north,west,east` WGS-84 viewport. All four values must be supplied
together; longitude bounds support antimeridian crossing. This keeps map
rendering proportional to the visible region instead of the full GPS library.

## ML, Lumen, And LLM

Photos maps every Lumen SDK field it consumes directly from `[lumen]`; it never
calls SDK defaults or env loading. ML and LLM feature settings remain
runtime-mutable catalog settings and do not belong in `AppConfig`. Zero-shot
classifier preview is exposed through `/api/v1/classifiers/preview`.

Desktop installs a platform-specific, release-pinned Lumen Hub into private
app-data and supervises it as a separate process tree. Its Hub configuration
is rendered and validated from the same platform/backend/preset/region/cache
selection model as the upstream CLI/Launcher. Before installation the Desktop
control panel offers `minimal`, `basic`, and `brave`, together with the pinned
backend artifacts supported by the current platform and a native model-cache
directory picker. The selected preset and canonical non-root cache path are
persisted together in host settings; the installed profile remains
authoritative for the backend. Config generation reads that persisted cache
path, falling back to the private Desktop model directory for legacy settings.
It binds loopback port
`50051`; the complete Desktop Server profiles name that
endpoint as a static node. The Hub also advertises only its loopback address so
older Desktop runtime intents can discover it through mDNS without exposing
inference on a LAN interface; separately operated LAN nodes remain discoverable.
Desktop readiness uses Lumen's versioned control gRPC service,
not a TCP-listener probe, and the model download/warmup lifecycle remains
independent from Server readiness. The Desktop host maintains a `WatchStatus`
subscription and projects its phase, inference-ready bit, download progress,
service states, release identity, and failures into the typed UI snapshot.
Structured Control logs are queried with a bounded, non-following `TailLogs`
request. Control telemetry is deliberately separate from the lifecycle version
counter so a status frame cannot make a concurrent start/stop command stale.

The app should remain useful when ML/LLM features are disabled.

## Quality Gate

Backend gate:

```bash
task server:test
```

Use the Taskfile target by default. It exports the local cgo flag allowlist
needed by media dependencies on macOS. Only run the direct command when you have
a concrete reason and preserve the same environment:

```bash
cd server && go test -tags=sqlite_fts5 ./...
```

Run `gofmt` on changed Go files.
