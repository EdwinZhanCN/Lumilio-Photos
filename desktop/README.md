# Lumilio Photos — Desktop (Wails v3)

The desktop app links the SQLite catalog runtime into the Go executable and
runs the existing API service in-process through `server/app`. The React product
UI opens at the configured canonical `server.primary_origin`.

## Architecture

```text
Wails v3 system tray + private native control-panel webview
  → "Open Lumilio Photos" opens the persisted primary origin
desktop/supervisor
  → owns the host-lifetime single-instance lock and one server generation
  → validates persistent config/runtime.toml through the strict Server loader
  → projects machine-owned paths into generated config/server.toml (mode 0600)
  → journals candidate apply/readiness/last-known-good rollback
  → runs server/app.Run(ctx, cfg, controls) in-process
server/app
  → opens/migrates <app-data>/library.sqlite3
  → serves the API and React SPA on the selected network profile
```

The default local profile uses `http://localhost:6680`, so platform passkeys
work without TLS. The optional LAN HTTP profile changes only the listener and
shows an unencrypted-transport warning; remote passkeys remain unavailable.
External HTTPS trusts only configured proxy CIDRs and never makes Desktop own
certificates.

`desktop` is a separate Go module with `replace server => ../server`. That
committed replace is the load-bearing wiring for local builds and CI.

## Develop and test

```sh
cd web && vp build
cd ..
make desktop-dev
make desktop-test
```

`make desktop-test` includes a first-launch/relaunch integration test. It creates
and migrates a real SQLite catalog, exercises the health endpoint and SPA
fallback, stops the runtime, reopens the same catalog, and verifies its durable
library identity.

### Control panel UI

`desktop/panel` is a Svelte 5 + Tailwind v4 + Bits UI app embedded into the Go
binary via `//go:embed all:panel/dist`. The desktop Make targets build it first.
To iterate independently:

```sh
cd desktop/panel && vp dev
LUMILIO_PANEL_API=http://host:port vp dev
```

The first command uses the built-in `/__onb` mock API; the second proxies to a
live app.

The mock accepts deterministic scenario parameters, for example:

```text
?mode=dashboard&runtime=failed&network=external&lumen=failed
?mode=dashboard&network=lan&lumen=starting&apply=rollback
```

`runtime` supports `stopped|starting|running|restarting|failed`; `network`
supports `local|lan|external`; `lumen` supports
`disabled|starting|running|failed`; and `apply` supports
`success|rollback|failure`.

## App data

On macOS, machine-local state lives under:

```text
~/Library/Application Support/Lumilio Photos/
├── library.sqlite3
├── backups/
├── cloud/
├── config/
│   ├── runtime.toml
│   ├── runtime.last-known-good.toml
│   └── server.toml
├── logs/
├── lumen/
├── secrets/
├── storage/
└── lumilio.lock
```

Windows uses `%LocalAppData%\Lumilio Photos`. The SQLite catalog, credentials,
configuration, and optional AI runtime always stay in app data; only explicitly
authorized media repositories may live on external Storage Locations.

`config/runtime.toml` is the complete persistent schema-v3 runtime intent.
`config/server.toml` is regenerated for each server generation after Desktop
projects machine-owned paths and is the immutable input for that run.
`desktop-settings.json` v2 stores only host/control-plane choices such as
onboarding, download region, LAN-risk acknowledgement, and Lumen settings; it
does not duplicate network policy.

Runtime and structured network edits share one candidate. Apply writes
`runtime.candidate.toml` and `runtime-apply.json`, proves the previous generation
stopped, promotes the candidate, waits for internal readiness, then updates
`runtime.last-known-good.toml`. Failed readiness restores LKG. An interrupted
journal is reconciled on the next Desktop launch.

The Wails Control Panel consumes a typed stopped/starting/running/restarting/
failed snapshot and remains available after ordinary Server startup failures.
The host lock is released only when Desktop quits; a shutdown timeout retains
generation ownership and blocks a second listener/SQLite runtime.

Development and test overrides:

| Environment variable | Purpose |
|---|---|
| `LUMILIO_APP_DATA` | Isolated app-data root |
| `LUMILIO_WEB_ROOT` | React SPA directory served at `/` |
| `LUMILIO_RESOURCES_DIR` | Unpackaged resource root |

## Build: macOS

```sh
brew install vips libraw dylibbundler create-dmg
desktop/scripts/fetch-resources.sh
make desktop-build
desktop/scripts/build-macos.sh arm64 --dmg
```

SQLite, FTS5, and sqlite-vec are linked into the application. The bundle stages
only the media tools, web assets, licenses, and libvips dependency tree.

The macOS bundle declares purpose strings for user-selected folders and network
or removable Storage Locations. Distribution is an ad-hoc-signed DMG. Because
it is not Developer-ID signed or notarized, the first launch requires approval
through **System Settings → Privacy & Security → Open Anyway**.

## Build: Windows

The Windows build runs natively in an MSYS2 MINGW64 shell with the toolchain
listed in `.github/workflows/release-desktop.yml`.

```sh
desktop/scripts/fetch-resources.ps1
LUMILIO_VERSION=1.2.3 desktop/scripts/build-windows.sh
```

The output is `desktop/build/windows/Lumilio Photos/`. The release workflow
produces both a portable zip and an Inno Setup per-user installer. The installer
ensures WebView2 is available, adds shortcuts, and registers an uninstaller with
an optional, separately confirmed app-data removal step. See
[packaging/windows/README.md](packaging/windows/README.md).

Windows artifacts are currently unsigned, so SmartScreen may require
**More info → Run anyway** on first launch.

## Updates and compatibility

The tray checks GitHub Releases once per launch and offers the matching DMG or
Windows installer. Installation remains manual; app data is preserved and the
embedded SQLite migrations run on the next launch.

Released schema changes must be additive forward migrations. Editing a migration
already shipped to users would desynchronize existing catalogs.
