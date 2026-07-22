# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-07-18.

## Release and operations

- **Desktop distribution is not fully signed or available on macOS Intel.**
  Owners: `.github/workflows/release-desktop.yml`,
  `desktop/scripts/fetch-resources.sh`. macOS arm64 and Windows amd64 packaging
  are wired. Remaining distribution work is Intel ffmpeg/ffprobe pins plus a
  `macos-15-intel` build, Apple notarization and Windows Authenticode/installer
  when signing is available, and the corresponding real-machine smoke. Updates
  remain release-page/manual-install based until signed platform updaters are
  viable.
## Security and test coverage

- **Refresh tokens are stored in `localStorage`.** Owner:
  `web/src/lib/http-commons/auth.ts`. An XSS can exfiltrate both access and
  refresh tokens. Moving the refresh token to an `HttpOnly` cookie requires an
  explicit CSRF design and must preserve Desktop localhost and authenticated
  media behavior; track that cross-cutting change in its own hardening plan.
- **Authentication endpoints have no brute-force rate limit.** Owner:
  `server/internal/api/router.go` plus a future shared limiter. Password login,
  passkey login verification, and MFA verification have no per-IP or
  per-account throttle/lockout policy, so online password or TOTP guessing is
  unbounded. This remains a pre-public-deployment hardening item.
- **Refresh-token rotation/reuse lacks a DB-backed regression test.** Owner:
  `server/internal/service/auth_service.go` (`RefreshToken`). The fail-closed
  rotation and token-family reuse revocation exist, but the service owns a
  concrete `*repo.Queries` and the suite has no PostgreSQL auth harness. Add an
  integration test, or introduce the smallest query seam needed to test the
  transaction behavior without weakening the current contract.

## Product paths

- **Assets have no permanent-delete or automatic Trash retention path.**
  Owners: `server/internal/service/asset_service.go`,
  `server/internal/db/repo/queries/assets.sql`, and
  `web/src/features/assets/routes/AssetsTrash.tsx`. The current app-level path
  is database soft-delete plus restore; the lower-level repository trash purge
  helper is not exposed as the product operation. Any future permanent delete
  must be Trash-only, owner-scoped, strongly confirmed, and explicit that it
  destroys original media.
- **The S3/R2 cloud provider is a runtime placeholder.** Owner:
  `server/internal/cloud/provider_s3.go`. `List` and `Download` always return
  `s3 provider not implemented`; it is not currently wired into a usable import
  path. Either implement and wire the existing `CloudProvider` contract or
  remove the placeholder when the provider is formally descoped.
- **AgentBoard has no mobile column reflow.** Owner:
  `web/src/features/lumilio/flows/board/AgentBoard.tsx`. It renders one
  persisted 12-column layout at every width, so phone columns compress into
  narrow slivers. Add a client-only narrow-screen remap or a separately
  persisted breakpoint layout, then verify it against a live backend without
  corrupting the canonical desktop layout.
