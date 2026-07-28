# Database Backup and Recovery

Status: completed and verified on 2026-07-24.

## Shipped contract

- The server owns scheduled logical backups for Docker and Desktop.
- Automatic backups wait until first-run setup is ready; a pre-setup database
  cannot produce a misleading snapshot with no administrator.
- Routine dumps live at the explicit `storage.backups_path` (Desktop defaults
  it to local app data) and use version-matched PostgreSQL client tools. They do
  not follow a removable media Storage Location.
- Dumps are gzip-compressed SQL, written through a temporary file and renamed
  atomically. Retention does not remove restore points.
- Backup and restore tools resolve the current rotated database secret at each
  invocation rather than retaining the bootstrap credential.
- Restore is admin-only, takes a fresh restore point, runs transactionally, and
  rolls back automatically if restore, migration, or verification fails.
- Restore refreshes the application connection pool after PostgreSQL terminates
  stale sessions, then runs migrations and health checks on fresh connections.
- Settings → Server exposes schedule, retention, create, download, restore, and
  delete operations.

Primary owners: `server/internal/db/backup`, `server/internal/service/backup_service.go`,
`server/internal/queue/db_backup_worker.go`, `desktop/supervisor`, and
`server/db.Dockerfile`.

## Verification evidence

- `make server-test` passes the backup engine, scheduler, credential-resolution,
  dump, and restore rollback tests.
- `make web-test` passes type, lint, boundary, unit, and browser-integration
  checks.
- `make web-backup-recovery-test` passes against an empty PostgreSQL 18 E2E
  environment and proves on-demand creation, UI download, successful restore,
  restore-point creation, corrupt-dump failure, automatic rollback, and
  preservation of state created before the failed restore.

## Deferred release decision

PostgreSQL major-version upgrade orchestration is not an active plan for the
pre-release product. Desktop continues to fail closed on a major mismatch.
Create a fresh active exec plan only when an RC or stable compatibility baseline
has been declared and a concrete subsequent major-version bump is scheduled.

## Non-goals

Filesystem/media backup, WAL archiving, pre-release PostgreSQL 17-to-18 upgrade
support, and permanent Trash deletion are outside this completed plan. Original
media remains the user's backup responsibility.
