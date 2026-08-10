# Upgrade Lumilio Photos

**Back up the database before upgrading** (see [Backup and Data Integrity](./integrity)) and make sure no import or edit tasks are running. Upgrading never modifies your media or database.

## Desktop (macOS / Windows)

1. Click **Update available** in the menu bar or tray;
2. Download the new installer and verify its SHA-256 (see [Installation](./installation));
3. Run the installer or replace the app in Applications;
4. Restart.

**Success**: the app opens normally and the version on the About page is updated.
**Rollback**: remove the new version and reinstall the previous one; media and database are unaffected.

## Docker (Linux / NAS)

In the deployment directory:

```bash
docker compose pull
docker compose up -d
docker compose ps
```

**Success**: the `lumilio` service is healthy on the new version.
**Rollback**: check `docker compose logs lumilio`; if needed, restore the previous image tag and start again. The media directory (`./lumilio/media`) and app state (`./lumilio/app-state`) survive container recreation as long as you did not delete them.

> If something looks wrong after upgrading, save the logs and error samples before rolling back (see [Diagnostics & Logs](../features/monitor)).
