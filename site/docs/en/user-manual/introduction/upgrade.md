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
  Steps: [Coming from a beta build](#coming-from-a-beta-build).

## Coming from a beta build

`v26.1.0-rc.1` is the first supported release. It does not open a catalog,
configuration, or backup written by a beta build, and the albums, People,
ratings, Event names, and share links from the beta are not carried over.
Your media files and the Repository folders are untouched, and rc.1 reads
the `.lumilioroot` and `.lumiliorepo` markers the beta wrote.

1. If you want the beta catalog for reference, create a database backup in
   the beta build first; back up your media independently.
2. Stop the beta build and move its application state aside. Docker: the
   `LUMILIO_STATE` directory (the catalog, secrets, and backups). Desktop: the
   `Lumilio Photos` folder under your user's application-data directory
   (see [Installation](./installation)). Do not move or rename the media
   folder.
3. Install rc.1 and run the first-run setup with the same Default Storage
   Location. At the storage step, setup finds the beta's Primary Repository
   in `primary/` and offers **Adopt Primary Repository**: adopt it to keep its
   identity and every file; the scan registers them again. To start over
   instead, point the Default Storage Location at another folder first
   (Desktop: **Settings → Storage**; Docker: `storage.path`); the beta's
   folder is left untouched.
4. On the **Storage** page, add every other Repository the beta used as an
   existing Repository and let the scan register its files. Trash contents
   the beta kept on disk are listed and held in the Trash until you decide;
   a file the beta showed in its Trash without moving it is a normal file
   again.

## Desktop (macOS / Windows)

`v26.1.0-rc.1` has no in-app update: **Settings → Updates** reports that no
signed update provider is configured in this build. Updates are installed
from the GitHub Releases page; the in-app updater is planned for rc.2.

1. Quit Lumilio Photos from the menu bar or tray;
2. Download the new `.dmg` or installer from the
   [GitHub Releases page](https://github.com/EdwinZhanCN/Lumilio-Photos/releases)
   and verify its SHA-256 (see [Installation](./installation));
3. Run the installer or replace the app in Applications;
4. Start the app again.

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
