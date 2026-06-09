# Tech Debt Tracker

Keep this list short. Each item should have a concrete owner path and a reason it matters.

- Docker image build is not currently verified in this workspace when the local Docker/Orbstack socket is unavailable.
- **Desktop bundle: native binaries + signing not yet wired into a release.** The
  desktop module (`desktop/`) is code-complete and compiles, but packaging is
  unfinished and needs a macOS build host with staged binaries:
  - Stage PG16+pgvector / ffmpeg / exiftool into `desktop/resources/` (see its
    README; `.github/workflows/build-postgres.yml` builds the PG artifact).
  - Build + stage the web SPA: `cd web && vp build`, then the build script copies
    `web/dist` into `Resources/web` (the supervisor sets `server.web_root` to it).
  - `desktop/scripts/build-macos.sh` runs `dylibbundler` + ad-hoc `codesign`;
    verify `@rpath` resolves and RAW decode works (libraw via libvips).
  - Publish the ad-hoc-signed DMG(s) (arm64 + amd64) to GitHub Releases. No
    Homebrew cask (it quarantines by default, so it would gain nothing over a DMG
    for an ad-hoc app). Notarization (needs a paid Apple Developer account) is the
    only way to remove the one-time "Open Anyway" prompt — a future upgrade.
  - Passkeys now run in the real browser (the app is a tray + browser, no
    webview), so the WKWebView entitlement spike is moot — just confirm Touch ID
    registration works in Safari/Chrome against `localhost` once the SPA ships.
  - **Phase 2 UI**: native storage-location picker + "reconnect external drive"
    dialog (supervisor currently persists/falls back but has no picker UI).
