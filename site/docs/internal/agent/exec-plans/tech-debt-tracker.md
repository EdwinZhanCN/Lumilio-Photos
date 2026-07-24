# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-07-24.

## Security and test coverage

- **Refresh tokens are stored in `localStorage`.** Owner:
  `web/src/lib/http-commons/auth.ts`. An XSS can exfiltrate both access and
  refresh tokens. Moving the refresh token to an `HttpOnly` cookie requires an
  explicit CSRF design and must preserve Desktop localhost and authenticated
  media behavior; track that cross-cutting change in its own hardening plan.

## Product paths

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
