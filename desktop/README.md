# Lumilio Photos — Desktop (Wails v3)

A macOS desktop build that bundles a private PostgreSQL 17 runtime and runs the
existing Go API server **in-process**, reusing the same bootstrap (`server/app`)
and the same React UI over HTTP. See the full design in
`site/docs/internal/agent/exec-plans/active/desktop-wails-v3.md`.

## Architecture

```
Wails v3 system tray (menubar app, no webview)
  → "Open Lumilio Photos" opens the default browser at http://localhost:6680
desktop/supervisor
  → manages a private PostgreSQL 17 (initdb / pg_ctl / pg_isready / createdb)
  → generates secrets under the app-data dir
  → builds typed server config through server/config.NewDesktopConfig(...)
  → writes config/server.local.toml as a debug copy only
  → runs server/app.Run(ctx, cfg) in-process (migrations + API + queue + ML)
  → the Go API server also serves the React SPA at localhost:6680 (server.web_root)
```

There is **no embedded webview**: the UI runs in the user's real browser. This is
deliberate — a real browser surfaces platform passkeys (Touch ID / iCloud
Keychain) at the `localhost` RP, whereas an embedded WKWebView would require an
Apple Associated-Domains entitlement (a paid Developer account) to do so. The
React bundle is served by the Go server (`server.web_root`), not Wails' assets;
the tray auto-opens the browser on launch and on demand.

## Module wiring

`desktop` is a separate Go module that depends on the sibling `server` module via
a `replace server => ../server` directive (committed). `go.work` is gitignored in
this repo, so the replace directive is the load-bearing wiring for local builds
and CI.

## Develop

```sh
# Run the app against a locally installed PostgreSQL (no bundling required):
make desktop-dev PG_BIN_DIR=/opt/homebrew/opt/postgresql@17/bin
# (or any local PostgreSQL, e.g. .../postgresql@14/bin — version-agnostic for dev)

# Run the Go tests (the PostgreSQL lifecycle test auto-skips when no PG is found):
make desktop-test
```

App data (always local, never on the user's relocatable media drive):
`~/Library/Application Support/Lumilio Photos/` — `postgres/`, `secrets/`,
`config/`, `backups/`, `lumilio.lock`. Override the root with `LUMILIO_APP_DATA`.
`config/server.local.toml` is a generated debug copy of the typed runtime config;
desktop does not boot by reloading it.

Useful env overrides (development):

| Env | Purpose |
|---|---|
| `LUMILIO_APP_DATA` | App-data root (isolate instances / tests) |
| `LUMILIO_PG_BIN_DIR` | PostgreSQL bin dir (dev, no bundle) |
| `LUMILIO_WEB_ROOT` | Web SPA dir to serve at `/` (dev points at `web/dist`) |
| `LUMILIO_RESOURCES_DIR` | Bundled-resources root (dev, no `.app`) |

`make desktop-dev` sets `LUMILIO_WEB_ROOT` to `web/dist`; run `cd web && vp build`
first so the browser shows the UI (otherwise the server runs API-only).

The full stack (PG → migrations → in-process API → SPA at `localhost:6680`) has an
opt-in end-to-end test:

```sh
LUMILIO_E2E=1 LUMILIO_PG_BIN_DIR=/opt/homebrew/opt/postgresql@14/bin \
  go test ./supervisor/ -run TestDesktopRuntimeE2E -v
```

## Build (.app + DMG)

```sh
brew install vips dylibbundler create-dmg      # build-time deps
desktop/scripts/fetch-resources.sh             # ffmpeg/ffprobe/exiftool (pinned + sha256)
# also stage PostgreSQL 17 + pgvector into resources/postgres/17/<platform>/ (from source), then:
make desktop-build                             # → desktop/build/Lumilio Photos.app
desktop/scripts/build-macos.sh arm64 --dmg     # also produce a DMG
```

The `--dmg` step builds the classic "drag the app onto Applications" window via
`create-dmg` (Applications symlink + positioned icons; optional background art at
`packaging/dmg/background.png` — see that dir's README). It needs a GUI session
for the window styling and falls back to a plain DMG (still with an Applications
symlink) when run headless.

Distribution is a single **ad-hoc-signed DMG** from GitHub Releases (no Apple
Developer account). Ad-hoc signing is still required: Apple Silicon won't run
unsigned binaries, and `dylibbundler`'s install-name rewrites invalidate the
bundled dylib signatures, so they are re-signed. The DMG container is unsigned.

First launch on the user's machine: drag the app to Applications, then (because
the download is quarantined and the app isn't notarized) approve it once via
**System Settings → Privacy & Security → Open Anyway**. This persists afterward.
Removing that prompt entirely requires Developer-ID signing + notarization, which
needs a paid Apple Developer account — a clean future upgrade to the same DMG.

> Homebrew Cask was intentionally not used: Homebrew quarantines casks by default,
> so a cask install of an ad-hoc app hits the same Gatekeeper prompt as the DMG —
> all maintenance, no UX benefit.

## Build (Windows: portable + installer)

The Windows build is native — CGo via MSYS2/MINGW64, not a cross-compile from
macOS (libvips + libraw would need the whole imaging stack cross-built). Run
inside an MSYS2 MINGW64 shell with the toolchain from the `windows` job in
`.github/workflows/release-desktop.yml` (`mingw-w64-x86_64-{go,gcc,pkgconf,libvips,libraw,ntldd}`):

```sh
# stage resources first: desktop/resources/postgres/17/windows-amd64,
# ffmpeg/exiftool via desktop/scripts/fetch-resources.ps1, and web/dist (vp build)
LUMILIO_VERSION=1.2.3 desktop/scripts/build-windows.sh   # → desktop/build/windows/Lumilio Photos/
```

Two distribution forms, both from GitHub Releases:

- **Installer (recommended)** — `Lumilio-Photos-<ver>-windows-amd64-setup.exe`,
  built from `packaging/windows/lumilio.iss` with Inno Setup 6.1+
  (`ISCC.exe /DAppVersion=1.2.3 desktop\packaging\windows\lumilio.iss`). It
  installs per-user to `%LocalAppData%\Programs\Lumilio Photos` (no UAC), ensures
  the Edge **WebView2 Runtime** the first-run onboarding window needs, adds Start
  Menu shortcuts, and registers an uninstaller (stops the app + bundled Postgres,
  optional data removal with a photo-library safety prompt). See
  [packaging/windows/README.md](packaging/windows/README.md).
- **Portable** — zip the `Lumilio Photos` directory; the user extracts and runs
  `lumilio-photos.exe`. Requires the WebView2 Runtime to already be present for
  the setup window.

Both are unsigned, so SmartScreen shows **More info → Run anyway** on first run —
the same posture as the unsigned macOS DMG. Authenticode (ideally EV) signing
removes it, tracked in `release-cicd.md`.

There is **no Windows uninstaller script to maintain by hand**: the installer
generates it. macOS deliberately has no uninstaller — drag the app to the Trash
(the app data under `~/Library/Application Support/Lumilio Photos` can be removed
manually).

## Updates

The tray checks GitHub Releases once per launch (async, best-effort, silent on
failure — `update.go`). If a newer semver release exists it adds an "Update
available: vX.Y.Z" tray item that opens the release page; the user installs it
manually (Windows: run the new setup.exe, which upgrades in place; macOS: drag
the new app over the old). App data under the per-user data dir is untouched, and
the server runs its embedded migrations on next launch.

> Because installed builds carry real user data, **released schema changes must
> be additive forward migrations** — the "edit the original migration in place"
> habit only holds pre-release; once users have data, editing a shipped migration
> desyncs their database on update.

Full silent auto-update is deferred: it needs code-signing (Sparkle/Authenticode)
we don't have yet, so today an "update" is a signed-out manual reinstall that
preserves data.

## Status / remaining work

Implemented: supervisor (PG lifecycle, typed config/secrets generation,
single-instance lock, storage-path persistence, quarantine cleanup), embedded
migrations, in-process server boot, the Go server serving the SPA, the Wails
system-tray controller with auto-open browser, and dev/build tooling. Verified end to end
against a real PostgreSQL (`TestDesktopRuntimeE2E`): PG → migrations → API → SPA
at `localhost:6680`.

Remaining (ops / requires a build host with the staged binaries):
bundling PostgreSQL+pgvector / ffmpeg / exiftool / libvips, building+staging the
web SPA, ad-hoc signing, and the DMG release. (The WKWebView passkey risk is moot
now that the UI runs in a real browser.) Tracked in the exec plan's Validation
checklist.
