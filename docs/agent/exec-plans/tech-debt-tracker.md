# Tech Debt Tracker

Keep this list short. Each item should have a concrete owner path and a reason it matters.

- Docker image build is not currently verified in this workspace when the local Docker/Orbstack socket is unavailable.
- **Assets permanent delete is intentionally deferred.** F4 in
  `exec-plans/active/assets-feature-review.md` resolves ordinary delete as
  Move to Trash + Restore, with no permanent-delete or automatic retention path
  in the current milestone. A future implementation must be Trash-only,
  owner-scoped, strongly confirmed, explicit about deleting original media, and
  distinct from ordinary library delete.
- ~~Lumilio Agent RichInput temporarily offline~~ **(resolved 2026-06-12)** — mention/slash were rebuilt natively in `web/src/features/lumilio/components/Chat/MentionInput.tsx` (textarea + popover, no contentEditable) and the legacy `components/RichInput/` directory was deleted. See `exec-plans/active/agent-context-mention-slash.md` v2.
- ~~`/assets/filter-options` response under-typed / cast in `MentionInput`~~ **(resolved 2026-06-13)** — root cause was a stale `make dto`, not a missing DTO: `dto.OptionsResponseDTO` (`camera_models`, `lenses`) and the handler `@Success` annotation were already correct, but `schema.d.ts` was stale so it surfaced as `Record<string, never>`. Regenerated with `make dto` and removed all `as` casts from `MentionInput.tsx` (now fully type-safe). Rule materialized in [FRONTEND.md](FRONTEND.md)/[BACKEND.md](BACKEND.md): an `as`-cast on an API response means check DTO/annotation and re-run `make dto`, never cast.
- **Auth: refresh token stored in `localStorage` (XSS-exposed).** Owner:
  `web/src/lib/http-commons/auth.ts`. Tokens (access + refresh) live in
  `localStorage`, so any XSS can exfiltrate the refresh token. Moving the refresh
  token to an `HttpOnly` cookie is a cross-cutting change (CSRF strategy, the
  desktop in-process host at `localhost:6680`, and media-element auth which
  relies on a queryable token) and is deliberately deferred. Decided out of
  scope for `exec-plans/active/auth-feature-review.md`; promote to its own plan
  before changing the storage model.
- **Auth: no rate limiting / brute-force protection on auth endpoints.** Owner:
  `server/internal/api/router.go` + a future shared middleware. `login`,
  `passkeys/login`, and `mfa/verify` have no per-IP/per-account throttle, so
  online password/TOTP guessing is unbounded. Needs a dedicated hardening plan
  (shared limiter middleware + lockout policy); intentionally out of scope for
  the auth review fix plan.
- **Auth: refresh-rotation/reuse logic has no DB-backed regression test.** Owner:
  `server/internal/service/auth_service.go` (`RefreshToken`). The fail-closed
  rotation + token-family reuse-revocation added in the auth review is covered by
  build + code review only; `s.queries` is a concrete `*repo.Queries` (not an
  interface), and the service test suite has no Postgres harness, so a real
  regression test needs the integration DB (`make db`) or a queries interface to
  mock. Add when the integration-test harness lands.
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
