# Architecture

This is the compact system map: how Lumilio is delivered, which rules hold
across every module, and where to look next. It deliberately does not list
packages or describe their internals — that structure is derived from source
and would drift if it were copied here.

## Start with the Atlas

The [Atlas](atlas/README.md) is the map of the code. Browse it with
`task atlas` (an interactive explorer at <http://localhost:6690/atlas/>), or read the
generated markdown under [`atlas/generated/`](atlas/generated/README.md):

- **Architecture** — [system context](atlas/generated/architecture/system-context.md),
  then the derived [Server](atlas/generated/architecture/server-groups.md),
  [Desktop](atlas/generated/architecture/desktop-groups.md), and
  [Web](atlas/generated/architecture/web-groups.md) group maps and one package
  map per group. The group dependency rules live in
  [`atlas/atlas.yaml`](atlas/atlas.yaml).
- **Sequence, data flow, lifecycle** — authored views whose every element is
  anchored to a real symbol, API operation, or table.
- **Modules** — [module catalog](atlas/generated/modules.md): every Go package
  and Web module with its group and one-line purpose. Each package's own
  `doc.go` (Go) or `doc.ts` (Web features) is the full description.

## Runtime shape

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
- Published versions use `YY.TRAIN.PATCH[-beta.N|-rc.N]`. One tag on `main`
  builds every Desktop and Server artifact before creating the GitHub Release;
  its Server bundle pins the multi-architecture image by OCI digest.
- Runtime state has three non-overlapping owners: frontend preferences in
  browser localStorage; runtime-mutable settings in the SQLite catalog through
  the Settings and Setup APIs; and runtime-immutable process configuration in a
  complete schema-versioned TOML manifest.
- First-run bootstrap (`fresh → catalog_ready → admin_created → ready`) is an
  orthogonal state machine. It observes owner and primary-repository gates; it
  is not a fourth configuration source. Atlas:
  [bootstrap](atlas/generated/lifecycle/bootstrap.md).
- `server/config/examples/` holds one complete manifest per deployment scenario
  (`dev/`, `desktop/`, `docker/`), generated from `server/config/profiles.go`.
  Because TOML comments cannot express conditional legality, a valid manifest
  per scenario is what documents the matrix; `task dev` renders `dev-vite`
  into `.local/dev/config/server.toml`. Container images ship complete
  `docker-http` and `docker-caddy` manifests; ACME and custom operator
  manifests are generated into app-state by `server config init`. The Desktop
  keeps a schema-versioned runtime intent and projects it through the same
  strict `server/config` loader before calling `server/app`.
- Standalone requires `--config <path>`. Ordinary environment variables never
  override `AppConfig`; only CLI diagnostics and the explicit break-glass
  whitelist are single-run host controls.

## Cross-cutting invariants

These hold across packages, so no single `doc.go` owns them. Each is enforced
by a gate or test where one is named.

- **The catalog is product truth.** One physical SQLite writer applies product
  facts and desired/applied work state; bounded query-only WAL readers serve
  foreground reads. River on QueueDB is disposable delivery state; deleting
  QueueDB only delays work. Atlas:
  [background work](atlas/generated/dataflow/background-work.md).
- **No work inside write transactions.** Filesystem, media, network, hashing,
  and unbounded CPU work happen before a transaction begins. Writer
  transactions are named, bounded, and admitted through
  `internal/db/catalogtx` (`task architecture:check` rejects raw ones).
- **Background results have one writer.** Workers compute; the commit
  coordinator alone applies their results and checks each result's fence.
- **Foreground reads never write.** Bootstrap, setup, status, and storage read
  models derive their answer through query-only readers; an HTTP read never
  triggers reconciliation or expiry merely to render current state.
- **Originals are never rewritten, and only users delete.** A scan, watcher, or
  cloud sync never unlinks, trashes, or purges. An Asset exists exactly while
  it has a repository entry; `lifecycle.PurgeEntriesTx` is the only Asset
  delete (`task architecture:check`). Atlas:
  [Asset lifecycle](atlas/generated/lifecycle/asset-lifecycle.md).
- **Owner scope is explicit.** `owner_id` is the only hard partition;
  repositories are unowned shared storage. Owner-scoped topology (Events)
  always carries its resolved owner into downstream queries.
- **ML and LLM are optional.** Media management, browsing, and non-semantic
  search keep working when Lumen or an LLM provider is absent or failing.
- **The Desktop App supervises, it does not reimplement.** It runs
  `server/app.Run` in-process, learns readiness through typed handoffs rather
  than probing its own HTTP listener, and keeps the product UI in the system
  browser. Atlas: [Desktop runtime phase](atlas/generated/lifecycle/desktop-runtime.md).
- **Web dependency direction.** `web/ARCHITECTURE.md` is the authoritative
  frontend ownership and import-direction contract, enforced by
  `web/scripts/check-source-boundaries.ts`; the Atlas Web group map is its
  derived picture.

## Contracts

- OpenAPI is the HTTP contract source of truth. Regeneration:
  [lumilio-api-contract-change](../.agents/skills/lumilio-api-contract-change/SKILL.md).
  Do not hand-edit generated OpenAPI artifacts.
- `storage.path` is registered at startup as the non-removable default Storage
  Location, identified by `.lumilioroot`; startup does not create repositories.
  Authenticated setup creates the primary through
  `POST /api/v1/setup/primary-repository`. Ordinary users choose upload targets
  from `GET /api/v1/storage/targets` on `/manage`; administrators manage storage
  from `/storage` via `GET /api/v1/storage/view` and admin-only
  `/api/v1/storage/*` commands (create/open, verify, detach, native tasks,
  diagnostics). Repository cloud bindings and stack detection remain on
  `/api/v1/repositories/{id}/cloud*` and `/api/v1/repositories/{id}/stacks/detect`.
  Admin creation selects a registered Storage Location by
  `storage_location_id`; the Desktop control plane authorizes host paths, while
  standalone/Docker attach existing `.lumiliorepo` directories through
  `/api/v1/storage/candidates`.
- The SQLite catalog, cloud sessions, secrets, logs, and database backups are
  app-private state and must be configured outside `storage.path`. Repository
  staging remains inside its repository under `.lumilio/staging`.
- Registered repository I/O is rooted by `internal/storage.RepositoryFS` and
  serialized against repository relocation and removal. Assets own logical
  owner/content identity, not paths; `repository_entries` binds files to
  Assets. Durable jobs carry stable IDs and expected revisions. Native codecs
  receive absolute filenames only through the explicit local-path adapter
  after resolving a present entry.
- The scan index walks every repository including `inbox/` and excluding
  `.lumilio/`. Watcher events are hints; a full scan is the authority, and a
  catalog row becomes missing only after a positive absence probe with the
  repository marker re-checked. Atlas:
  [repository scan](atlas/generated/sequence/repository-scan.md).
- Full BLAKE3 plus size is immutable exact content identity. SQL uniqueness
  enforces one Asset per owner/content pair, which may have any number of
  present entries. Revisioned outbox consumers are leased, bounded,
  at-least-once, and idempotent.
