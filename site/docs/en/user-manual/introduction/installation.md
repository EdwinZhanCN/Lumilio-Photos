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

## Desktop network access

The Desktop Control Panel offers three explicit network modes:

- **Local only** is the default. Lumilio listens on loopback and opens
  `http://localhost:6680`. Use this unless another device needs access.
- **LAN HTTP** listens on the local network but keeps
  `http://localhost:6680` as the canonical origin. It is intended only for a
  trusted home LAN and requires acknowledging that traffic from other devices
  is unencrypted. Passkeys remain available only from the Desktop machine's
  localhost URL. Your firewall may also require an inbound rule.
- **External HTTPS** is for an existing HTTPS reverse proxy. Enter the exact
  public HTTPS origin and the narrow CIDR of each trusted proxy; Desktop never
  obtains ACME certificates itself. The proxy must forward the original
  scheme, host, and client address. Saving performs a restart and readiness
  check, and restores the previous working settings if the candidate fails.

The configured primary origin is also the WebAuthn Origin, and its hostname is
the RP ID. Changing the hostname therefore requires registering new passkeys;
keep password and TOTP recovery available during the move.

## Docker (Linux server / NAS)

Requires Docker with the Compose plugin.

Production has no implicit plaintext mode. Choose built-in ACME when this host
owns public TCP 80/443, or the external-proxy profile when another service owns
TLS.

### Built-in ACME HTTPS

Point the DNS A/AAAA record at the host and allow inbound TCP 80 and 443:

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/docker-compose.acme.yml
mkdir -p /srv/lumilio/media /srv/lumilio/state
export LUMILIO_STORAGE=/srv/lumilio/media
export LUMILIO_STATE=/srv/lumilio/state
export LUMILIO_IMAGE=ghcr.io/edwinzhancn/lumilio-server:latest
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-acme \
  --origin https://photos.example.com --email admin@example.com \
  --output /data/app-state/server.toml
docker compose -f docker-compose.acme.yml up -d
```

Open `https://photos.example.com`. CertMagic stores its account and certificate
under `/data/app-state/tls`, so retaining the state mount also retains renewals.
Certificate acquisition failure stops startup; it never falls back to HTTP.

### Existing reverse proxy

Generate the external profile with the dedicated Docker network CIDR:

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/docker-compose.proxy.yml
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-external-proxy \
  --origin https://photos.example.com \
  --trusted-proxy 172.30.0.0/24 \
  --output /data/app-state/server.toml
export LUMILIO_DOMAIN=photos.example.com
docker compose -f docker-compose.proxy.yml up -d
```

The included Caddy example publishes 80/443. Lumilio itself has only `expose:
6680`, so direct host access cannot bypass the proxy. Nginx and Traefik examples
are under `deploy/reverse-proxy/`. Trust the narrowest proxy address or
dedicated subnet; never trust a network shared with unrelated containers.

The single image serves both web and API. `LUMILIO_STORAGE` holds media;
`LUMILIO_STATE` separately holds the schema-v3 manifest, SQLite catalog,
snapshots, credentials, logs, and ACME state. Both mounts must be writable by
container UID 10001. Validate edits before restart:

```bash
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config validate --config /data/app-state/server.toml
```

Passkey Origin is exactly `server.primary_origin`, and RP ID is exactly its
hostname. Changing that hostname means existing passkeys cannot sign in at the
new RP ID; keep password + TOTP recovery available and register new passkeys.

::: warning Back up through Lumilio
While Lumilio is running, do not copy or open `library.sqlite3`, `-wal`, or
`-shm` with a host SQLite tool. Crossing the container mount boundary can
violate WAL locking. Use the snapshot and download actions under
**Settings → Server**, and back up the media directory separately. Removing
and recreating the application container is safe when both host mounts are
retained.
:::

## Optional: AI features

Semantic search, face recognition, and OCR are optional and provided by a
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference server. Nothing
is downloaded until you enable it.

- **Desktop (same machine):** menu-bar/tray → **Enable AI on This Machine**.
  The app downloads the right hub build for your hardware and manages it for
  you; the first start also downloads model weights (~1.3 GB).
- **Another machine or Docker:** run a Lumen Hub on your LAN (Docker tags
  `cpu` / `vulkan` / `cuda`) and provide a complete schema-v3 server manifest
  with the desired discovery policy. Runtime environment variables do not
  override immutable manifest fields. See the Lumen Hub README for details.
