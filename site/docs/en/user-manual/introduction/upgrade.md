# Upgrade Lumilio Photos

**Back up the database before upgrading** (see [Backup and Data Integrity](./integrity)) and make sure no import or restore task is running. Startup may migrate the catalog forward even though Lumilio preserves original media.

## Desktop (macOS / Windows)

1. Click **Update available** in the menu bar or tray;
2. Download the new installer and verify its SHA-256 (see [Installation](./installation));
3. Run the installer or replace the app in Applications;
4. Restart.

**Success**: the app opens normally and the version on the About page is updated.
**Rollback**: remove the new version and reinstall the previous one; media and database are unaffected.

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
