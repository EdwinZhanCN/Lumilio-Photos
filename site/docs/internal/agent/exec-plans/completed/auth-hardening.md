# Authentication Throttling And Session Isolation

Status: **completed and verified** (2026-07-24).

Goal: bound online guessing and authentication abuse without adding external
infrastructure, then prove that an exhausted session and a same-browser user
switch cannot retain prior-user credentials, server data, or client state.

## Decisions

- Enforce throttling at the HTTP authentication boundary, before expensive
  password, passkey, MFA, and refresh-token work.
- Apply both network and opaque subject buckets. Subjects are normalized and
  protected with a per-process keyed HMAC before entering the limiter; raw
  usernames and credentials are never retained by it, and digests cannot be
  correlated across restarts.
- Keep the limiter in-process and memory-bounded. Lumilio Photos is a
  single-instance local-first service; Redis or a new persistence table would
  add an unjustified runtime dependency.
- Make limits part of the complete immutable `[auth.rate_limit]` TOML contract.
- Return `429` with `Retry-After` and the same generic response shape regardless
  of account validity.
- Exercise the public API and browser against PostgreSQL through the existing
  isolated Playwright environment. No test may mutate the database directly.

## Implementation

1. Add a concurrency-safe bounded limiter under `server/internal/api/ratelimit`
   with deterministic clock-driven unit tests.
2. Add the strict auth rate-limit config to all production, container, desktop,
   and E2E manifests and wire one limiter into `AuthHandler`.
3. Guard password login, login-options/passkey entry, passkey verification, MFA
   verification, and refresh rotation; document `429` in OpenAPI and regenerate
   DTOs.
4. Extend `auth-session.spec.ts` with public-API throttle recovery, browser
   refresh exhaustion, and user A → user B isolation.
5. Run from `make dev-reset`, then targeted race tests, all server/web/desktop
   gates, the dedicated auth-hardening Playwright target, and remote PR CI.

## Completion

- Limits recover after `Retry-After`, do not reveal whether an account exists,
  and remain bounded under key churn.
- A revoked refresh token plus invalid access token forces the browser to
  `/login` and clears every credential.
- A second user in the same browser never renders the first user's private
  asset while its own server query is pending or after it completes.

## Verification

- `make server-test`
- `CGO_LDFLAGS_ALLOW=-Xpreprocessor CGO_CFLAGS_ALLOW=-Xpreprocessor go test
  -race ./internal/api/ratelimit ./internal/api/handler`
- `make web-test`
- `make desktop-test`
- Clean-stack `make web-browser-test`, followed by
  `make web-auth-hardening-test` and `make web-video-semantic-test`
