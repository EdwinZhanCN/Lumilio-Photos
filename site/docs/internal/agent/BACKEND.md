# Backend

This document describes the current Go backend as implemented in `server/`.

## Runtime Entry

- Main process: `server/cmd/main.go`.
- Config package: `server/config`.
- Tracked config template: `server/config/server.example.toml`.
- Ignored local config file: `server/config/server.local.toml`.
- Docker image: `server/Dockerfile`.

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

Desktop is a host wrapper, not a second server bootstrap: the supervisor prepares
app-data paths, bundled media tools, and the SPA root. Persistent
`config/runtime.toml` is strict-loaded, copied, and projected with host-owned
paths before Desktop atomically writes app-data `config/server.toml` with mode
`0600` and calls `app.Run`. A write or reload error blocks startup.

DesktopSettings v2 contains host/control-plane choices, not Server network
policy. Structured network edits and raw TOML edits both patch one fingerprinted
runtime candidate. Apply is serialized across restart/shutdown, journals
`candidate_staged → candidate_promoted → rolling_back`, proves the previous
generation exited, durably records promotion intent before the atomic file
replacement, and updates last-known-good only after readiness. `Prepare`
reconciles an interrupted journal before starting, including either side of the
promotion boundary. The host-level single-instance lock is acquired before
settings migration or Wails UI, spans all generations, and is released only by
Desktop shutdown. Control-plane reads deliberately expose an invalid active
intent plus structured issues for recovery; candidate acceptance still uses
`LoadAppConfigBytes`.

`server.primary_origin` is the sole canonical browser and WebAuthn Origin.
`internal/httporigin` resolves direct/proxied request context, and
`internal/servertransport` owns off/external/ACME listeners. Proxy-required
deployments reject direct application requests before auth handlers; only
loopback live/ready probes bypass the proxy requirement.

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
and is shut down when the host run exits.

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
  fixes the writer pool at one connection, registers sqlite-vec statically, and
  applies fixed pragmas to every physical connection through driver DSN options
  plus a connection hook. Startup reads the effective values back and fails
  closed if policy differs.
- Application tables and River queues share the same catalog and can commit
  business state plus `InsertTx` jobs in one short `database/sql` transaction.
- A running catalog must never be opened or copied through a host/container
  mount with another SQLite process. Host and container VFS locking is not a
  supported coordination boundary; use the application Online Backup flow, or
  inspect the catalog only after a graceful application stop.
- FTS5, sqlite-vec tables, and the OCR Bleve sidecar are derived query
  structures; authoritative text and embedding data remains in ordinary
  application tables.
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
fails. Desktop's `server.primary_origin` binding remains the WebAuthn RP Origin
for the product Web; it is not a CORS entry.

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
cd server && go test -tags=sqlite_fts5 ./...
```

Run `gofmt` on changed Go files.
