# Tech Debt Tracker

Keep this list short. Each item should have a concrete owner path and a reason it matters.

- Docker image build is not currently verified in this workspace when the local Docker/Orbstack socket is unavailable.
- ~~Lumilio Agent RichInput temporarily offline~~ **(resolved 2026-06-12)** — mention/slash were rebuilt natively in `web/src/features/lumilio/components/Chat/MentionInput.tsx` (textarea + popover, no contentEditable) and the legacy `components/RichInput/` directory was deleted. See `exec-plans/active/agent-context-mention-slash.md` v2.
- ~~`/assets/filter-options` response under-typed / cast in `MentionInput`~~ **(resolved 2026-06-13)** — root cause was a stale `make dto`, not a missing DTO: `dto.OptionsResponseDTO` (`camera_models`, `lenses`) and the handler `@Success` annotation were already correct, but `schema.d.ts` was stale so it surfaced as `Record<string, never>`. Regenerated with `make dto` and removed all `as` casts from `MentionInput.tsx` (now fully type-safe). Rule materialized in [FRONTEND.md](FRONTEND.md)/[BACKEND.md](BACKEND.md): an `as`-cast on an API response means check DTO/annotation and re-run `make dto`, never cast.
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
