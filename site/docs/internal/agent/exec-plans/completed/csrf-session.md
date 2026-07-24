# Cookie Session And CSRF Hardening

Status: **completed and verified** (2026-07-24).

Goal: remove the long-lived refresh credential from browser-readable storage
without making LAN, public-domain, reverse-proxy, or Desktop access require an
origin allowlist.

## Decisions

- Keep short-lived access tokens in the existing Bearer flow.
- Store refresh tokens only in a host-only `HttpOnly` cookie scoped to
  `/api/v1/auth`.
- Issue a CSRF token cryptographically bound to the current refresh token.
  Browser clients send it through `X-CSRF-Token`; refresh rotation rotates both
  values.
- Recover the CSRF token through a safe authenticated-cookie GET so clearing
  browser storage does not strand a valid session.
- Treat `server.cors_allowed_origins` as the explicit list of browser origins
  trusted to use credentialed cross-origin sessions.
- Allow credentialless cross-origin API requests by default. Never combine a
  wildcard origin with cookies or `Access-Control-Allow-Credentials`.
- Accept the dynamically derived request origin without configuration. This is
  what keeps direct LAN IPs, public domains, reverse proxies, and the Desktop
  product Web at localhost zero-config. The private Wails Control Panel remains
  outside the product HTTP authentication boundary.
- Require a trusted `Origin`/`Referer` for browser requests that create, rotate,
  or destroy a cookie session. Requests with no browser-origin metadata remain
  available to non-browser clients that maintain a cookie jar.

## Implementation

1. Add session-bound CSRF generation and constant-time validation to
   `AuthService`.
2. Centralize auth response/cookie writing, CSRF recovery, refresh validation,
   logout cleanup, and cookie attributes in the HTTP handler layer.
3. Replace the CORS middleware with credential-aware behavior and add exact
   origin validation to setup and session endpoints.
4. Remove refresh-token response/body contracts and browser `localStorage`
   usage; enable credentials and CSRF headers in the typed client.
5. Regenerate OpenAPI types and update unit, integration, and full E2E session
   tests, including negative cross-origin cases.

## Completion

- Browser-readable storage contains the short-lived access token and the
  session-bound CSRF proof, never the refresh credential.
- Refresh and logout validate both the cookie-bound proof and the browser
  origin; rejected requests do not consume a valid session.
- Same-origin LAN/public/Desktop-browser access remains zero-config. Untrusted
  cross-origin cookie sessions receive no credentialed CORS grant, while
  credentialless Bearer API calls remain available.
- Rotation and logout share a cross-tab Web Lock, and refresh-token replay still
  revokes the user's refresh-token family.

## Verification

- `make dev-reset`
- `make dto`
- `make server-test`
- `make web-test`
- `make desktop-test`
- `make web-browser-test` — 7 passed
- `make web-auth-hardening-test` — 3 passed, including negative origin/CSRF
  checks and real-asset cross-user isolation
