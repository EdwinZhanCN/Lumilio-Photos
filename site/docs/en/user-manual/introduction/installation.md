# Installation

If every administrator is locked out after installation, follow [Recover administrator access](./break-glass.md) instead of editing the database.

Lumilio Photos is local-first: your photos, videos, and database live on your
own machine. Pick the install path that matches where you want it to run:

| Where it runs | Method |
|---|---|
| A Mac (Apple Silicon) | [macOS app](#macos-apple-silicon) — menu-bar app, everything bundled |
| A Windows 10/11 PC (x64) | [Windows installer](#windows) — per-user setup, everything bundled |
| A Linux server or NAS | [Docker Compose](#docker-linux-server-nas) |

The desktop apps use an embedded machine-local SQLite catalog and bundle their
media tools, so there is no database service to install. All downloads are on the
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
(your media library location is separate and is never deleted).

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

## Desktop network access

The Desktop Control Panel offers two network modes:

- **Local only** is the default. Lumilio listens on loopback and opens
  `http://localhost:6680`. Use this unless another device needs access.
- **LAN HTTP** listens on the local network. It is intended only for a trusted
  home LAN and requires acknowledging that traffic from other devices is
  unencrypted. Passkeys remain available from localhost but not a remote HTTP
  address. Your firewall may also require an inbound rule.

Domain, certificate, and reverse-proxy configuration remains outside the
Desktop Control Panel. Lumilio derives the browser and passkey identity from
the address currently used.

## Docker (Linux server / NAS)

Requires Docker Engine with Compose 2.23.1 or newer on Linux. Download and
start the default host-network deployment:

```bash
mkdir lumilio-server && cd lumilio-server
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.yml
docker compose up -d
```

Open `http://<Linux-host-IP>:6680`. No domain, HTTPS URL, or hand-written TOML
is required. Media and application state default to `./lumilio/media` and
`./lumilio/app-state`; set `LUMILIO_STORAGE` and `LUMILIO_STATE` before startup
to use other paths.

The default Compose uses host networking so Lumen mDNS discovery works on the
LAN. It does not need a `ports` mapping.

Remote HTTP is unencrypted and cannot use passkeys. Existing reverse proxies
may forward to port 6680 without configuring a public URL in Lumilio. Optional
same-host Caddy and built-in ACME files remain available as
`caddy.compose.yml` and `acme.compose.yml`.

::: warning Back up through Lumilio
While Lumilio is running, do not copy or open `library.sqlite3`, `-wal`, or
`-shm` with a host SQLite tool. Crossing the container mount boundary can
violate WAL locking. Use the snapshot and download actions under
**Settings → Server**, and back up the media directory separately. Removing
and recreating the application container is safe when both host mounts are
retained.
:::

## Optional: AI features

Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP
Species Recognition are optional and provided by a
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference server. Nothing
is downloaded until you enable it.

- **Desktop (same machine):** menu-bar/tray → **Enable AI on This Machine**.
  The app downloads the right hub build for your hardware and manages it for
  you; the first start also downloads model weights (~1.3 GB).
- **Another machine:** use `lumen-cli` to run a native Hub that advertises over
  LAN mDNS.
- **Linux server or NAS:** choose and download a complete CPU, Vulkan, or CUDA
  host-network Compose from the [Lumen AI guide](../features/lumen-ai). Once the
  Hub is healthy, Lumilio Photos discovers it automatically; no server manifest
  edit or static address is required.
