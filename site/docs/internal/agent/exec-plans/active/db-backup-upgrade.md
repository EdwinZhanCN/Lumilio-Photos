# Database Backup & Major-Version Upgrade

Status: active. Backup and restore are implemented; PostgreSQL major-version
upgrade orchestration is deferred until the first major bump.

## Current contract

- The server owns scheduled logical backups for Docker and Desktop.
- Routine dumps live in `<storage>/backups/` and use version-matched PostgreSQL
  client tools.
- Dumps are gzip-compressed SQL, written through a temporary file and renamed
  atomically. Retention does not remove restore points.
- Restore is admin-only, takes a fresh restore point, runs transactionally, and
  rolls back automatically if restore, migration, or verification fails.
- Settings → Server exposes schedule, retention, create, download, restore, and
  delete operations.
- Desktop refuses a PostgreSQL major mismatch. Do not bump Desktop
  `pgMajorVersion` or the `server/db.Dockerfile` base major until the matching
  upgrade path below ships.

Primary owners: `server/internal/db/backup`, `server/internal/service/backup_service.go`,
`server/internal/queue/database_backup.go`, `desktop/supervisor`, and
`server/db.Dockerfile`.

## Remaining work

### Desktop major upgrade

In the release that bumps PostgreSQL, bundle old and new binaries and add a
supervisor state machine:

1. Start the old cluster and create an app-data restore point.
2. Stop it, initialize the new versioned data directory, and restore the dump.
3. Run server migrations and health checks.
4. Retire the old directory only after success; on failure leave it untouched
   and remove the incomplete new directory.

Unit-test the state machine with a fake runner and run a real two-major smoke
test when both PostgreSQL versions are available.

### Docker major upgrade

For the transition database image, include the old server binaries and
pgvector extension, detect the old `PG_VERSION`, run an idempotent
`pg_upgrade --link`, and retain the old directory for rollback. Unknown source
versions must fail closed. Document app-level dump/fresh-volume/restore as the
manual fallback.

### Optional recovery entry

The fresh-install flow may offer restore from dumps already present in
`<storage>/backups/`. This is useful but is not required before the first
PostgreSQL major bump.

## Outstanding verification

- Run `make web-test` and the i18n extract/fill pass for the backup UI.
- Before a PostgreSQL major bump: run `make server-test`, `make desktop-test`,
  a Desktop two-major upgrade smoke, and a Docker volume upgrade/rollback
  smoke.

## Non-goals

Filesystem/media backup, WAL archiving, and minor-version migration are outside
this plan. Original media remains the user's backup responsibility.
