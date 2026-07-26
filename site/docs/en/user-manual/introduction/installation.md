# Installation

If every administrator is locked out after installation, follow [Recover administrator access](./break-glass.md) instead of editing the database.

Lumilio Photos is local-first: your photos, videos, and database live on your
own machine. Pick the install path that matches where you want it to run:

| Where it runs | Method |
|---|---|
| A Mac (Apple Silicon) | [macOS app](#macos-apple-silicon) — menu-bar app, everything bundled |
| A Windows 10/11 PC (x64) | [Windows installer](#windows) — per-user setup, everything bundled |
| A Linux server or NAS | [Docker Compose](#docker-linux-server-nas) |

The desktop apps bundle their own private PostgreSQL and media tools — there is
nothing else to install. All downloads are on the
[GitHub Releases page](https://github.com/EdwinZhanCN/Lumilio-Photos/releases).

## macOS (Apple Silicon)

1. Download the `.dmg` from the latest release, open it, and drag
   **Lumilio Photos** into **Applications**.
2. Launch it. The app is not notarized yet, so macOS shows a Gatekeeper prompt
   the first time: open **System Settings → Privacy & Security** and click
   **Open Anyway** (once).
3. Lumilio Photos lives in the **menu bar** (no Dock icon). On first run a
   setup window shows the machine-local default media location, download
   region, and open-source terms. External Storage Locations can be authorized
   later in the Desktop Control Panel; the database and credentials remain on
   the local disk.
4. The app initializes its private database, then opens your default browser at
   `http://localhost:6680`.
5. In the browser, the first-run wizard creates your **admin account**
   (password now; adding an authenticator app, passkey, and recovery codes is
   offered right after and can be skipped).

::: tip Updates
When a newer release exists, the menu-bar menu shows **Update available** —
click it to download the new `.dmg`, then replace the app in Applications.
Your library and database are untouched. During setup (or later in the
control panel) you can set **Download region** to Mainland China so the
installer is fetched via a GitHub mirror; this is separate from in-app map
region settings.
:::

App data (database, secrets, logs) lives under
`~/Library/Application Support/Lumilio Photos/`. To uninstall, quit from the
menu bar, delete the app, and delete that folder if you also want the data gone
(your photo library location is separate and is never deleted).

## Windows

1. Download `Lumilio-Photos-<version>-windows-amd64-setup.exe` from the latest
   release and run it. SmartScreen may warn about an unknown publisher —
   choose **More info → Run anyway**.
2. The installer is **per-user** (no administrator prompt), creates Start Menu
   shortcuts, and installs the Microsoft Edge WebView2 runtime automatically if
   it is missing (needed by the first-run setup window).
3. Launch **Lumilio Photos** from the Start Menu. It runs in the **system
   tray** and shows the same first-run setup window as macOS: review the local
   default location, accept the terms, and the browser opens at
   `http://localhost:6680` for the admin-account wizard.

Uninstall from **Settings → Apps & features**; the uninstaller stops the app
and its database, and can optionally remove the app data.

::: tip Updates
When a newer release exists, the tray menu shows **Update available** — click
it to download the new `setup.exe` and run it over the existing install. Your
library and database are untouched. **Download region** (Mainland China vs
other) controls whether the installer URL uses a GitHub mirror; it is separate
from in-app map region settings.
:::

## Docker (Linux server / NAS)

Requires Docker with the Compose plugin.

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/docker-compose.release.yml
mkdir -p /srv/lumilio/media /srv/lumilio/state
LUMILIO_STORAGE=/srv/lumilio/media \
LUMILIO_STATE=/srv/lumilio/state \
docker compose -f docker-compose.release.yml up -d
```

The single `lumilio-server` image serves both the web interface and API. Set
`LUMILIO_STORAGE` to the media-library directory and `LUMILIO_STATE` to a
separate machine-local directory for `library.sqlite3`, application snapshots,
credentials, and logs. Both mounts are required and must be writable by
container UID 10001. Then open `http://<host>:6657` and complete the first-run
wizard.

- Pin a version with `LUMILIO_VERSION=v1.0.0` (default `latest`).
- Change the published HTTP port with `WEB_HTTP_PORT` (default `6657`).
- Put a trusted HTTPS reverse proxy in front of Lumilio before exposing it to
  the internet; TLS is not a required service in the default stack.

::: warning Back up through Lumilio
Do not copy `library.sqlite3`, `-wal`, or `-shm` while the container is running.
Use the snapshot and download actions under **Settings → Server**, and back up
the media directory separately. Removing and recreating the application
container is safe when both host mounts are retained.
:::

## Optional: AI features

Semantic search, face recognition, and OCR are optional and provided by a
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference server. Nothing
is downloaded until you enable it.

- **Desktop (same machine):** menu-bar/tray → **Enable AI on This Machine**.
  The app downloads the right hub build for your hardware and manages it for
  you; the first start also downloads model weights (~1.3 GB).
- **Another machine or Docker:** run a Lumen Hub on your LAN (Docker tags
  `cpu` / `vulkan` / `cuda`) and provide a complete schema-v2 server manifest
  with the desired discovery policy. Runtime environment variables do not
  override immutable manifest fields. See the Lumen Hub README for details.
