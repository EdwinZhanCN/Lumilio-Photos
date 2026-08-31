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
- Linux production Compose files: `deploy/compose/compose.yml`,
  `deploy/compose/caddy.compose.yml`, and `deploy/compose/acme.compose.yml`;
  all use host networking.
- Current-checkout container deployment: `deploy/compose/dev.compose.yml`; it
  builds the Server and Web sources and uses host networking plus
  Docker-managed named volumes for isolated development state.
- Browser E2E Compose files: `web/e2e/compose.yml` plus the CI cache overlay
  `web/e2e/compose.ci.yml`.
- SQLite/River qualification uses the host-side
  `server/tools/sqlitepressure` controller plus
  `web/e2e/compose.pressure.yml`. Every growing bind mount and artifact stays
  below `.cache/sqlite-concurrency/<run-id>/`; the controller stops producers
  at 15 GiB and fails at 16 GiB so rollback and cleanup keep a 4 GiB reserve.

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

Runtime-immutable configuration is a complete schema v4 manifest, not a defaults
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

TOML contains all immutable database/server/logging/storage/repository-observation/auth/
transcode/Lumen/tool decisions. `[database]` contains only the explicit
persistent catalog path. The application secret is a file reference and may be
created at that exact path on first start. No secret value appears in TOML,
generated desktop manifests, or logs.

Reverse-geocoding provider, endpoint, response language, and User-Agent are
runtime-mutable administrator settings owned by the singleton SQLite `settings`
row. They are read and updated through the system-settings API; they are not
TOML fields or environment overrides.

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
- `internal/storage`: RepositoryFS, repository/staging managers, repository config,
  and the Repository Observation Engine under `internal/storage/roe`.
- `internal/cloud`: cloud ingest and sync providers.
- `internal/sourcing`: unified staged materialization for upload and cloud sync;
  committed files publish the same node/Location facts used by repository observation.
- `internal/classify`: classifier support code shared by API/service paths.
- `internal/logging`: zap logger setup, stdlib bridge, and repository audit helpers.
- `internal/agent`: agent service and tools.
- `internal/event`: owner-scoped Event candidates, deterministic `events-v1`
  segmentation/reconciliation, correction transactions, resolution, and direct
  typed relations. Event membership atoms are always `media_item` rows.
- `internal/utils`: media, hashing, raw, exif, upload, imaging, and support utilities.

## Asset Metadata Projection

EXIFTool output is retained verbatim in `assets.exif_raw`, then projected into
two separate models. Common searchable facts—capture time and offset, GPS,
dimensions, duration, and rating—live in typed `assets` columns. Embedded
keywords are normalized into `asset_tags`. `specific_metadata` contains only
media-type-specific display/filter facts; it never duplicates those common
columns.

Metadata retries replace `specific_metadata` using the current type schema.
They preserve an existing `description` key and never overwrite rating or
embedded keyword relationships after the first successful raw extraction.

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

All runtime access inside a registered repository goes through the shared
`internal/storage.RepositoryFS` factory. It verifies the catalog UUID against
`.lumiliorepo`, holds a lifecycle read lease, and owns canonical user/private
path parsing. Assets do not own paths. `repository_nodes` stores the relative
directory graph, while versioned `asset_locations` binds a physical node to an
owner/content Asset. River payloads carry stable IDs and expected revisions;
workers resolve an active Location immediately before opening media. Native
media tools use the documented RepositoryFS local-path adapter only at that
boundary.

The Repository Observation Engine (ROE) streams the full user tree, including
`inbox/`, in bounded directory pages and excludes only application-private
`.lumilio/`. A run captures change cursor `C0`, crawls progressively, drains to
a fixed `C1`, verifies dirty directories, and only then finalizes absences from
authoritatively covered child sets. Cursor gaps, watcher overflow, volume
replacement, offline repositories, cancellation, and access errors fail closed:
positive observations may publish, but unproven absence never closes a valid
Location. Native USN/ReadDirectoryChangesW, FSEvents, and inotify adapters are
hints backed by periodic authoritative verification.

Changed or unresolved nodes are hashed once with BLAKE3 from a stable open
handle and committed only if their observation revision and before/after token
still match. `content_objects` owns exact byte identity; one owner/content pair
has one Asset and may have multiple active Locations. Catalog changes and
revisioned effects commit together in `repository_outbox`; bounded, leased,
at-least-once drains feed hashing and derived River work without holding a
filesystem operation inside a database transaction.

## Database And API Contracts

- `github.com/mattn/go-sqlite3` is the only database driver. `internal/db.Open`
  fixes the writer pool at one connection, registers the vendored SQLite Vec1
  0.7 extension statically, and applies fixed pragmas to every physical
  connection through driver DSN options plus a connection hook. Startup reads
  the effective values back and fails closed if policy differs.
- The catalog has exactly one physical read/write connection, shared by
  application mutations and River, plus four `mode=ro`, `query_only` WAL
  connections. Do not add another ordinary writer or use `busy_timeout` as a
  writer-admission design. `database.Queries` routes generated read-only
  statements to the reader pool and mutations or unclassified statements to
  the writer; an explicit transaction remains pinned to the connection that
  began it. The router is checked against SQLite's own statement-readonly
  classification for every generated query.
- `internal/db/catalogtx` is the closed application transaction capability.
  Every runtime transaction has a compile-time operation name and role; the
  observed connector also names standalone writer statements and returned-row
  lifetimes. Bounded HDR histograms record admission, body, commit, total,
  cancellation, outcome, and cursor lifetime without retaining SQL text,
  arguments, or entity IDs. The architecture inventory rejects raw production
  `Begin`, `BeginTx`, and standalone writer `Exec` calls outside the documented
  migration/driver boundary.
- Multi-statement planning that needs one snapshot uses a short transaction on
  the reader pool and closes all rows before CPU, filesystem, media, network,
  serialization, sleep, or River work. A write transaction contains only
  bounded SQL and in-memory validation. Large atomic membership changes use
  set-based statements; restartable derived projections may publish in bounded
  turns with revision/CAS protection instead of monopolizing the writer.
- Foreground GET/status paths never reconcile or persist cached projections.
  Before bootstrap reaches its terminal cached value, `Phase` derives the live
  admin/primary gates through query-only readers. Repository and Storage
  Location lists plus setup runtime status consume the latest catalog
  projection; boot and the one-minute storage reconciler own filesystem probes
  and projection writes. Host Action expiry is materialized during reads and
  durably swept only at an explicit recovery boundary.
- Application tables and River queues share the same catalog. ROE commits
  business state plus outbox effects in one short transaction and delivers
  River jobs separately with revision-fenced idempotency. Repeatable River work
  never uses time-window uniqueness. Revisioned, outbox-backed, or immutable
  work is unique across every active state; mutable snapshot projections such
  as Event, Location, and Stack exclude `running` but include every queued state, which
  preserves exactly one follower when facts change during a run. Further
  changes coalesce into that follower. Asynchronous, cancellable reader probes
  arm process-local atomic hints, and
  River periodic constructors only consume those hints because constructors
  must never block. An idle system therefore does not continuously write no-op
  jobs. Bounded ROE outbox pages continue by snoozing the same active job, which
  yields the writer without inserting a follower.
- WAL auto-checkpointing is disabled on every connection so an arbitrary
  foreground commit is not charged an automatic checkpoint. Runtime monitoring
  samples writer pool wait count/duration and WAL size; once the WAL exceeds
  the bounded threshold it requests an explicit passive checkpoint through the
  sole writer. A fully checkpointed WAL file version is remembered because
  passive checkpointing may retain the file for reuse; idle allocated bytes do
  not cause repeated writer maintenance. The latest bounded telemetry,
  writer/reader `DBStats`, WAL state, and checkpoint result are atomically
  published as private mode-0600 `sqlite-runtime.json` under `logging.dir` for
  host-side sampling; there is deliberately no private-data HTTP debug API.
  Slow named transactions remain logged against the write-transaction budget.
  Online Backup and pre-cutover snapshot reads use a reader connection, never
  the writer semaphore.
- Migration `000004_media_semantic_events` adds stable Events, membership,
  correction constraints, one-hop redirects, factual dirty ranges, and
  per-owner rebuild state. Migration `000006_event_convergence` adds the
  owner-wide source/published revision pair, renewable rebuild leases, persisted
  rebuild runs, and the terminal `retired` state. The `event_scheduler` queue
  discovers pending owners while `rebuild_events` runs owner jobs concurrently;
  River uniqueness leaves at most one queued/running follower per owner.
- Event reads, browse filters, shares, relations, and Agent refs resolve through
  the same owner-aware Event resolver. Repository filtering is a read-only
  Browse Scope projection over canonical logical-media membership. Event shares
  and Agent refs materialize immutable displayable-asset snapshots; automatic
  membership uses no ML/AI signal.
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
  Bleve batch updates. Successful OCR result, Trash, and restore commits signal
  a process-local coalescer; recovery and wake ticks insert a River drain only
  when the reader observes durable outbox work. Missing, corrupt,
  mapping-mismatched, and post-restore indexes are deleted and rebuilt before
  HTTP starts.
- Migrations live in `server/migrations`. The application migration ledger
  records SHA-256 for every applied SQL file; version, name, and checksum must
  continue to match embedded history, so historical migrations are immutable.
- SQLite restore is a generation boundary, not a live-handle operation. Its
  journal progresses through `staged`, `previous_preserved`,
  `active_installed`, `verified`, and `completed`; rollback has corresponding
  durable phases. Startup reconciles the marker with active/staged/previous/
  failed files after any interrupted rename.

Generated sqlc code lives under `server/internal/db/repo`. Generated OpenAPI
output lives in `server/docs`. Frontend generated types live in
`web/src/lib/http-commons/schema.d.ts`. None of these are hand-edited.
Regeneration, cast triage, and the swag empty-object quirk:
[lumilio-api-contract-change](../../../.agents/skills/lumilio-api-contract-change/SKILL.md).
`task verify:generated` is the CI freshness gate.

## SQLite Runtime Proof

The root tasks below are cross-module orchestration, not ordinary CI wrappers:

```sh
task sqlite:pressure:smoke RUN_ID=smoke-macos-001
task sqlite:pressure:baseline RUN_ID=baseline-macos-001
task sqlite:pressure:qualify RUN_ID=qualify-macos-001
task sqlite:pressure:soak RUN_ID=soak-macos-001
task sqlite:pressure:startup RUN_ID=startup-macos-001
task sqlite:pressure:recover RUN_ID=recover-macos-001
task sqlite:pressure:compare RUN_ID=qualification-set-macos-001 BASELINE_RUN_ID=baseline-macos-001 MIXED_RUN_IDS=qualify-macos-001,qualify-macos-002,qualify-macos-003
task sqlite:pressure:cleanup RUN_ID=smoke-macos-001
```

The smoke profile validates wiring and all corpus/queue families but its short
histograms are diagnostic only. The baseline uses the same 32 RPS foreground
mix as qualification after initial durable work has remained drained for 65
seconds; it disables media/maintenance producers and controlled failure
injection, and does not require synthetic 23-queue coverage. A steady-state p99
is eligible only with at least 10,000 scheduled terminal samples in each
operation class; warm-up is reported separately and repeats are never pooled.
Startup runs retry-free real Chromium trials and reports p50/p95/max—30
observations do not justify a p99. Normal startup measurement retains only
failure traces and the three slowest successful traces. Direct catalog
inspection occurs only after the runtime is stopped. The compare target refuses
to pool histograms: it checks all three qualification summaries independently,
publishes the worst per-run p99, and requires every mixed/idle class ratio to
stay at or below 4x.
The soak target keeps the complete 1,200-media producer/queue workload active
for two hours at exactly half the qualification arrival rate; it remains a
10,000-sample p99 run rather than a low-volume functional check.

### API Problem boundary

Every non-2xx JSON response below `/api/v1` is an RFC 9457 Problem with
`application/problem+json`. The response contains only required `type`,
`status`, and opaque `instance` members plus fields from an exact registered
subtype. It never contains `title`, `detail`, display copy, or the private Go
cause. Generic status-only failures use `about:blank`; a Lumilio type below
`https://lumilio.org/problems/` exists only when the client needs a distinct
explanation, recovery path, or machine action.

`internal/api/problem` is the closed descriptor registry and wire-structure
owner. Handlers select a typed failure and pass its diagnostic cause to the
single `internal/api.WriteProblem` boundary; they cannot supply display text or
an arbitrary extension map. That boundary generates the occurrence URI and
attaches its instance, type, and cause to the normalized-route request log. The
public instance can therefore identify exactly one structured log event without
revealing the cause. Gin's direct writer is retained as the central handler
boundary because handlers and pre-handler middleware share the same context;
introducing a second error-returning handler signature would add routing
indirection without strengthening the closed writer contract.

Failures after request acceptance use the generated transport-neutral Problem
Reference union: `type`, opaque `instance`, optional retryability, and the same
bounded subtype facts, but no HTTP `status`. Agent/upload streams and durable
scan, restore, cloud, and native-host operation DTOs preserve these references
across polling and restart. Operator-only queue samples and support diagnostics
remain separate and may retain sanitized technical detail; ordinary UI copy
never comes from them.

`tools/openapinormalize` installs the exact HTTP and Reference discriminator
unions and rewrites every documented 4xx/5xx response media type after `swag`.
`tools/problemcatalog` publishes the registry-backed pages under
`site/docs/public/problems/`. Both run through `task dto`; architecture checks
reject legacy responders, direct non-success `c.JSON`, raw public error text,
unregistered type literals, undocumented types, and catalog drift.

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
`/api/v1/repositories/{id}/scans/latest` endpoint. E2E seeding and readiness:
[lumilio-e2e-environment](../../../.agents/skills/lumilio-e2e-environment/SKILL.md).

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

LLM provider identity and configuration requirements are owned by the
deterministic registry in `internal/settings`. The system-settings response
publishes only each provider ID plus whether its API key and base URL are
required; request DTOs defer membership checks to that registry. The supported
classic `ToolCallingChatModel` adapters are Ark, OpenAI, DeepSeek, Ollama,
Claude, Gemini, Qwen, and OpenRouter. `internal/llm` validates the complete
database-backed setting before constructing the named official Eino-ext
adapter, so an empty setting cannot fall through to SDK environment variables,
credential files, or cloud profiles. Ollama is the only keyless provider;
Ollama and Qwen require explicit endpoints because their selected adapters do
not supply one. Ark, OpenAI, DeepSeek, Claude, Gemini, and OpenRouter accept an
empty Base URL and use the pinned adapter's deterministic provider endpoint.
Claude uses only the direct Anthropic path, Gemini uses an explicitly
constructed Developer API client, and DeepSeek uses its native adapter. Qianfan
and hosted cloud variants remain outside this boundary because their credential
shapes or ambient state do not fit the single encrypted-key aggregate.

Provider SDK failures are classified at `internal/llm` before they reach HTTP
or Agent request logging. Ordinary logs never include provider response bodies,
Authorization values, or prompts reflected by a remote service; full prompt
capture remains confined to the explicitly configured `--agent-audit-log`.
Agent confirmation checkpoints are timestamped SQLite rows keyed by user and
thread, and the compatibility fixture locks targeted resume from Eino `v0.9.6`
onto the current ADK path.

The browser E2E stack adds a keyless `agent-model-fixture` at the Ollama HTTP
boundary. Its deterministic Go implementation accepts only the pinned model,
known scenarios, and the real Eino tool-call dialect; it rejects authentication
headers and exposes bounded counters instead of request bodies. The
`@agent-runtime` slice therefore crosses the built Web app, authenticated SSE,
Agent service, migrated SQLite state, checkpoints/effects, and real Lumilio
tools without an external provider or API key. Resume creates a
`prepared_resume` run outside the one-active-run index, then atomically
completes the awaiting run, activates the replacement, and repoints the thread
only after Eino accepts the checkpoint.

Agent OCR reads use a bounded observer rather than the search sidecar or HTTP.
`search_text` uses Bleve only to produce an OCR-matching ref; `read_ocr` accepts
a non-empty ref of at most two assets and reads authoritative ordered
`ocr_results` / `ocr_text_items` rows through the owner-bound
`AuthorizedLibrary`. The structured result restores ref and provider insertion
order, distinguishes unsupported media, missing results, and stored zero-item
results, and exposes only sanitized filenames, status, region count, and text
lines. It never returns UUIDs, repository paths, confidence, geometry, or model
identity; text is capped per region and across the complete tool response.

The SDK owns the continuous Lumen runtime pipeline: bounded DNS-SD scans,
strict service correlation, source-owned snapshot reconciliation, gRPC
transport state, and the in-band capability verdict. Failed scans preserve
prior observations; only consecutive successful omissions expire a node.
mDNS, Broker, and static sources are additive and supervised independently.
Photos consumes one immutable SDK runtime snapshot and does not couple feature
switches, Monitor polling, or status refreshes to discovery progress.

`GET /api/v1/capabilities` exposes only aggregate discovery state/counts and
task enablement/availability. `GET /api/v1/admin/lumen/runtime` is the
administrator-only Monitor projection with bounded backend diagnostics,
endpoint/source details, transport and compatibility states, canonical
service/task identifiers, and descriptive version/runtime. Neither route
returns arbitrary TXT metadata or raw resolver errors.

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

The app should remain useful when ML/LLM features are disabled. Isolated E2E
talks to `fakelumen`, which replays recorded Hub responses (or a deterministic
builtin embedding when no fixture matches). Recording is explicit, never
implicit in CI:
[lumilio-lumen-fixtures](../../../.agents/skills/lumilio-lumen-fixtures/SKILL.md).

## Quality Gate

```bash
task server:test
```

The Taskfile target exports the local cgo flag allowlist needed by media
dependencies on macOS. Reproduce the CI Server gate with `task server:test:ci`.
Run `gofmt` on changed Go files. Map a diff to evidence with
[lumilio-select-checks](../../../.agents/skills/lumilio-select-checks/SKILL.md).
