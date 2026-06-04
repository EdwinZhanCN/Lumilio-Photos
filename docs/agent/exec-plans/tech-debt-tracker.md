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
- **Frontend sync pending: neutral ML naming.** The backend dropped model-specific
  `clip`/`siglip` words from its public semantics in favor of model-agnostic
  `semantic`/`zeroshot` (Lumen SDK model identity such as `SigLIP` is unchanged).
  The frontend was intentionally left untouched and must be synced. Owner paths
  and the exact contract changes:
  - **Regenerate types first:** run `make dto` so
    `web/src/lib/http-commons/schema.d.ts` is regenerated from the updated backend
    OpenAPI spec. (This pass regenerated only the backend `server/docs/*`; the
    `openapi-typescript` step that writes `schema.d.ts` was not run.)
  - **Settings (`web/src/features/settings/hooks/useSystemSettings.ts`,
    `.../components/Tabs/AISettings.tsx`):** request/response key
    `clip_enabled` → `semantic_enabled`, and `siglip_classify_enabled` →
    `zeroshot_classify_enabled`. Rename the local `clipEnabled` field accordingly.
  - **Reprocess queue names (`web/src/config/retryTasks.ts` + `retryTasks.test.ts`):**
    `process_clip` → `process_semantic`.
  - **Indexing/rebuild task ids and ML monitor (`web/src/features/monitor/components/MLMonitor.tsx`,
    `web/src/locales/{en,zh}/translation.json`):** task identifier and stats key
    `"clip"` → `"semantic"` (the rebuild endpoint accepts `semantic`; the indexing
    stats payload now exposes `tasks.semantic` instead of `tasks.clip`).
  - **Asset tag source:** AI tags now carry source `zeroshot` (was `siglip_zeroshot`)
    for smart-album classification hits.
  - **Deployment (non-frontend, but breaking):** env var `ML_CLIP_ENABLED` →
    `ML_SEMANTIC_ENABLED`; TOML key `[ml] clip_enabled` → `semantic_enabled`. Existing
    `.env`/compose overrides must be renamed. The persisted `embedding_type` value
    `clip` is migrated to `semantic` by migration `029_neutral_semantic_naming`.
