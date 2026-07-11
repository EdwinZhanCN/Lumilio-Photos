# Database Backup & Major-Version Upgrade

Status: active

## Goal

One shared, app-driven database backup/restore engine for both deployment
shapes (Docker server and desktop), and on top of it a PostgreSQL
major-version upgrade path for each shape. Modeled on Immich's
`database-backup.service.ts` design (version-matched client binaries, atomic
gzip dumps with provenance filenames, restore-point + rollback restores), but
adapted to what we control that Immich does not: the db image, the desktop
supervisor, and both ends of every binary.

## Non-goals

- Filesystem/media backup (originals stay the user's responsibility; docs only).
- Point-in-time recovery / WAL archiving. Logical dumps are the right size and
  complexity for metadata-scale databases.
- Automatic *minor*-version handling (binary-compatible; nothing to do).
- Full web onboarding "restore from backup" wizard (phase 4 stub only).

## Current state (anchors)

- Docker: `docker-compose.release.yml` runs `lumilio-db`
  (`server/db.Dockerfile`, base `pgvector/pgvector:pg17`) with a `db_data`
  volume; `lumilio-server` (`server/Dockerfile`, debian trixie runtime, no
  postgres client tools today) mounts media at `/data/storage`
  (`STORAGE_PATH`).
- Desktop: `desktop/supervisor` owns a private cluster
  (`postgres/<major>/data` versioned layout), refuses a major mismatch via
  `DataDirStatus` (`postgres.go`), bundles full PG bin dirs per platform
  (`resources/postgres/<major>/<platform>/bin`), and reserves an app-data
  `backups/` dir (`paths.go`) that nothing writes to yet.
- Queue: River periodic-job pattern exists (`server/app/app.go:270`,
  `repository_scan`); queues/workers are registered in
  `server/internal/queue/queue_setup.go`.
- Runtime-mutable admin settings live in `server/internal/settings`
  (changed through the API, not TOML).
- Migrations are golang-migrate, embedded (`server/migrations/embed.go`),
  applied at server boot.
- Tech-debt tracker holds the placeholder entry this plan replaces
  ("Desktop: PostgreSQL major-version upgrade migrator not implemented").

## Design decisions

1. **The app runs the dumps, not the database container.** A River periodic
   job in the server (both shapes run the same `server/app`) shells out to a
   version-matched `pg_dump`. This gives one code path, admin-API control, and
   works against the desktop cluster and the Docker db service identically.
2. **Client binaries are version-matched by asking the server first**
   (Immich's trick). `SELECT version()` → major → resolve a bin dir:
   - Docker: `/usr/lib/postgresql/<major>/bin` — install PGDG
     `postgresql-client-<major>` packages in `server/Dockerfile` for every
     supported major (17 now; 17+18 during a transition release).
   - Desktop: the bundled `resources/postgres/<major>/<platform>/bin`
     (config plumbs the dir through `serverconfig.DesktopParams`, like
     ExifTool/FFmpeg already do).
   Unsupported major → job fails loudly, never dumps with a mismatched client.
3. **Routine dumps live with the media, restore-points live with the app.**
   Routine backups go to `<storage>/backups/` on both shapes so one folder
   backup captures media + metadata together (Immich's rationale). Upgrade
   restore-points go to the desktop app-data `backups/` dir / a Docker-local
   path, because they protect against upgrade failure, not disk loss.
4. **Dump format**: `pg_dump --clean --if-exists | gzip` (plain SQL, gzip
   `--rsyncable`), written as `<name>.tmp` then atomically renamed. Filename
   `lumilio-db-backup-<yyyyMMddTHHmmss>-v<appVersion>-pg<pgVersion>.sql.gz`
   (app version from `server/internal/version.Version`); the restore UI and
   the upgrade orchestrator parse provenance from the filename only.
5. **Restore is transactional with an automatic rollback point.** Before any
   restore: take a fresh `restore-point-` dump. Then terminate other backends,
   `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`, stream the dump through
   `psql --single-transaction --set ON_ERROR_STOP=on`, run migrations, health
   check; on any failure restore the restore-point. We skip Immich's streaming
   `OWNER TO` rewrite: both shapes have a single fixed database user.
6. **Major upgrades are dump/restore on desktop, pg_upgrade in the db image on
   Docker.** Desktop already versions data dirs and bundles binaries, so the
   supervisor can orchestrate old-start → dump → new-initdb → restore
   end-to-end. Docker's `db_data` volume boots a bare postgres entrypoint
   before our app runs, so the transition `lumilio-db` image carries the old
   major's server binaries + pgvector and runs `pg_upgrade --link` from its
   entrypoint (pgautoupgrade-style), keeping the old dir as rollback.

## Phase 1 — Backup engine + periodic job (both shapes)

- New package `server/internal/db/backup`:
  - `Locate(ctx, pool, cfg)` → resolves server version, client bin dir, dump
    target dir; explicit error types for unsupported major / missing tools.
  - `Dump(ctx, ...)` → pg_dump pipeline, `.tmp` + rename, provenance filename.
  - `Prune(dir, keep)` → retention by sorted routine-backup names; also
    deletes stale `.tmp` and failed leftovers.
  - Filename parse/validate helpers (shared with restore + upgrade).
- Config: `Database.ToolsBinDir` (optional override; desktop sets it from the
  bundle, Docker autodetects `/usr/lib/postgresql/<major>/bin`).
- Settings (runtime-mutable, `server/internal/settings`): backup `enabled`
  (default on), `cron`/interval (default daily 02:00 local), `keepLast`
  (default 14).
- Queue: `db_backup` queue (MaxWorkers 1) + `DatabaseBackupWorker`; periodic
  registration next to `repository_scan` in `server/app/app.go` (unique job so
  restarts don't double-enqueue).
- `server/Dockerfile`: add PGDG repo + `postgresql-client-17`.
- Desktop: verify `fetch-resources` stages `pg_dump`/`psql`/`pg_restore` into
  the bundled bin dir; plumb the dir through `DesktopParams`.
- Tests: unit tests for filenames/prune with fixtures; lifecycle test against
  a real PG (gated like `desktop/supervisor/postgres_smoke_test.go`) covering
  dump → gunzip → sanity-psql.

## Phase 2 — Restore engine + admin API

- `backup.Restore(ctx, ...)` implementing decision 5, with a progress callback
  (job log lines are enough; no UI streaming yet).
- Maintenance gate: restore requires quiescence — stop River client, close app
  pools except the restore connection, terminate remaining backends, restore,
  re-run migrations, health check, restart queues. On desktop the supervisor
  already owns this ordering; on Docker the server process orchestrates itself.
- Admin API (OpenAPI-first, `make dto`): list backups (name/size/parsed
  provenance), trigger backup now, download, delete, restore-by-name.
  Admin-only; restore returns a job handle.
- Web: minimal Settings > Backup tab (schedule/retention, list, create,
  download, delete, restore with confirm). No onboarding flow yet.

## Phase 3 — Major-version upgrade orchestration

### 3a. Desktop (automated)

Hook where `DataDirStatus == DataDirVersionMismatch` errors today
(`desktop/supervisor/supervisor.go`):

1. Transition release stages both `resources/postgres/<old>` and `<new>`.
2. Supervisor upgrade path (new `upgrade.go`): start old cluster with old
   binaries → fresh dump (backup engine, to app-data `backups/`) → stop old →
   initdb `postgres/<new>/data` (scram, existing path) → start new → restore →
   server migrations run on boot → health check.
3. Success: rename `postgres/<old>` → `postgres/<old>.retired` (deleted one
   release later). Failure: stop new, clear `postgres/<new>`, next launch
   retries; old dir untouched until success.
4. Tray progress via the existing `OnStage` stages
   (`upgrading_database` stage key).
- Tests: unit-test the state machine with a fake runner; full cycle in the
  smoke-test harness using two local PG majors when available.

### 3b. Docker (db image)

1. Transition `server/db.Dockerfile`: base `pgvector/pgvector:pg<new>` + old
   major's `postgresql-<old>` + `postgresql-<old>-pgvector` from PGDG.
2. Entrypoint wrapper: if `PGDATA/PG_VERSION` == old → run `pg_upgrade
   --link` into a fresh new-major dir, swap, keep old as
   `data.pg<old>.retired`, then exec the stock entrypoint. Idempotent; refuses
   unknown versions.
3. Document the manual fallback (app-level backup → fresh volume → restore via
   Phase 2 API) for users who prefer dump/restore or hit pg_upgrade edge cases.
4. Post-upgrade: server boot re-runs `CREATE EXTENSION`-safe migrations;
   `REINDEX`/`ANALYZE` guidance in release notes (hnsw indexes survive
   pg_upgrade but stats don't).

## Phase 4 — Recovery UX stub

- Fresh-install restore: setup flow gains a "restore from backup" entry that
  lists dumps found in `<storage>/backups/` (shape-agnostic, since the storage
  mount/dir is the thing users re-attach). Full onboarding wizard deferred.

## Validation gates

- `make server-test` (backup package unit tests always; real-PG lifecycle
  tests gated on available binaries).
- `make desktop-test` + supervisor smoke test extended with dump/restore.
- Manual: Docker compose up → trigger backup → wipe volume → restore → assets
  and albums intact. Desktop: seed 17 data dir → run transition build → data
  present under 18.
- `make dto` after the admin API lands; no hand-edited schema.

## Sequencing & ownership notes

- Phase 1 is independently shippable and immediately valuable (it is also the
  prerequisite for everything else — restore-points and desktop upgrade dumps
  reuse it).
- Phase 3a should land in the same release that first bumps `pgMajorVersion`
  past 17; until then the loud `DataDirVersionMismatch` error is the guard.
- When this plan completes, remove the tech-debt tracker entry it replaces.

## Open questions

- Windows desktop dump target: app-data vs storage dir when the library sits
  on a removable drive that may be absent at 02:00 — current lean: write to
  storage dir, skip with a logged warning when unreachable.
- Whether Docker `lumilio-db` should also ship a cron-less `pg_dumpall`
  convenience script for operators who bypass the app (lean: no; one path).
