# Upgrade Lumilio Photos

**Back up the database before upgrading** (see [Backup and Data Integrity](./integrity)) and make sure no import or restore task is running. Startup may migrate the catalog forward even though Lumilio preserves original media.

## What an upgrade does to your data

- **Database**: when a new version changes the database schema, startup first
  writes a `pre-upgrade-…` snapshot to the backups directory, then upgrades
  the database. Retention never deletes this snapshot, and the backup list
  shows it as a restore point. If an upgrade step fails, the server does not
  start, and the log names the failed step and the snapshot.
- **Restoring an older backup**: a backup made by an earlier supported version
  can be restored; the next start upgrades it in the same way.
- **Server configuration file**: the server never rewrites your
  `server.toml`. If a new version changes its format, startup stops and asks
  you to upgrade it explicitly:

  ```bash
  docker compose run --rm lumilio server config upgrade --config /data/app-state/server.toml
  ```

  The previous file is saved as `server.toml.bak`, and any comments you added
  are kept only there. The default Docker profile's configuration is built
  into the image, and Desktop upgrades its own configuration automatically;
  neither needs this step.
- **Downgrades**: a version refuses a database, backup, or configuration
  written by a newer version. Roll back with the pre-upgrade snapshot.
- **Pre-release data**: data from pre-release builds (`v1.0.0-beta.*` and
  `v26.1.0-beta.*`) is not migrated to `v26.1.0-rc.1` or later. The server
  names such a database as pre-release and does not open it; move it aside and
  start with a new one. Original media and Repositories are not touched.

## Desktop (macOS / Windows)

1. Click **Update available** in the menu bar or tray;
2. Download the new installer and verify its SHA-256 (see [Installation](./installation));
3. Run the installer or replace the app in Applications;
4. Restart.

**Success**: the app opens normally and the version on the About page is updated.
**Rollback**: remove the new version and reinstall the previous one. If the new version upgraded the database, the previous one refuses it; restore the `pre-upgrade-…` snapshot. Original media are unaffected.

## Docker (Linux / NAS)

Download and verify the target release's Server bundle. Keep the previous
bundle and its OCI digest, then replace the deployment files while preserving
your `LUMILIO_STORAGE` and `LUMILIO_STATE` values:

```bash
docker compose pull
docker compose up -d --wait
docker compose ps
```

**Success**: the `lumilio` service is healthy on the new version.
**Rollback**: check `docker compose logs lumilio`. Restore the previous
digest-pinned bundle together with the pre-upgrade database snapshot; an older
binary may not read a catalog already migrated by the target release. The media
directory remains separate and must not be replaced as part of catalog
rollback.

> If something looks wrong after upgrading, save the logs and error samples before rolling back (see [Diagnostics & Logs](../features/monitor)).
