# Backup and Data Integrity

A recoverable Lumilio backup covers at least **the original media** and **the catalog data in the SQLite database**. Thumbnails, search indexes, and model caches can be rebuilt, but they are not substitutes for those two. In-app database backups contain only catalog data (albums, people, edits); **they do not contain media files**.

## Backing up the database in the app

<DocPath :items="['Settings', 'Server', 'Database backups']" />

Administrators can enable automatic backups, set the interval and retention, and create, download, delete, or restore a snapshot immediately. New repositories enable automatic database backups by default — 24-hour interval, keeping the last 14 — but these values can change after migrations, so trust what the page currently shows.

The Desktop local backup directory is shown under **Desktop Control Panel → Paths**. The Server backup directory is set by `storage.backups_path` in the full TOML. Download a database snapshot to a separate device so the backup survives if the application data disk fails at the same time.

## Recommended practices

- Keep at least two copies of the original media, one of them not permanently connected to the main device;
- Create consistent snapshots with **Settings → Server → Database backups**; copy the database directory directly only while the service is fully stopped;
- Record which repositories map to which Storage Locations;
- Actually perform restore drills, instead of only checking that the backup task reports “success”;
- Create extra recovery points before upgrades, large deletions, and disk moves.

::: danger Do not copy a running SQLite file
Copying a live database and its sidecar files can produce an inconsistent snapshot. Prefer the online backup mechanism; if you must copy offline, fully quit Desktop or stop the Server first.
:::

The Trash is for undoing mistakes, not a backup. RAID, sync folders, and cloud drives each cover only part of the risk; they do not replace independent, restore-verified backups.

## What happens after you click “Restore”

Restore is **asynchronous**: clicking **Restore** only submits the request. The server returns `202 Accepted` and a persistent **operation ID**; the restore has not started yet, let alone finished.

After submission the browser tracks these stages:

```text
staged → restart → install → verify → complete
                                       └→ rollback / fail
```

- **staged**: the backup has been copied to the install location; the current database has not been touched;
- **restart**: the service performs a controlled restart and swaps in the staged database; if the restart hook does not exist, the staged restore is cancelled and **the current database stays unchanged**;
- **install**: the new database becomes the current one;
- **verify**: the server validates that the new database is usable;
- **complete**: the restore is finished;
- **rollback / fail**: if any step fails, the previous database is restored instead of leaving a half-broken state.

**The page briefly disconnecting is expected** during the restart. Do not keep refreshing or resubmitting the restore; note the operation ID, and once the page comes back you can continue observing the same operation with it.

::: danger Do not interfere during a restore
Do not close containers, delete media directories, or click Restore again while a restore is running. If verification fails, the service rolls back to the previous database by itself; external interference can break the recovery site so even rollback fails.
:::

Before restoring on a production instance, create a fresh recovery point and confirm that no import or edit tasks are running.

## A backup is not “fully recoverable” by itself

- **The in-app database backup contains only SQLite catalog data** (albums, people, edit records, accounts) — not original media, thumbnails, or model caches; after a restore the media must still be readable at their original locations;
- **A backup on the same disk cannot survive a disk failure**: when the backup directory lives on the same disk as the application data, a disk failure takes both. Keep at least one database snapshot and one media copy on a separate device;
- The Trash is for undoing mistakes, not a backup; RAID, sync folders, and cloud drives each cover only part of the risk.

## Restore drill

1. Prepare one database snapshot and the matching original-media backup;
2. Record the snapshot creation time, app version, and repository locations;
3. Restore the database in a non-production instance;
4. Confirm you can sign in, and spot-check albums, people, and media paths;
5. Open several original media formats to confirm the files themselves restore.
