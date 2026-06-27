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
