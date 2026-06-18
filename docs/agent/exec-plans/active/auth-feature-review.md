# Auth Feature Review — Fix Plan

## Context

End-to-end review of the **Auth** feature (web frontend → Go backend) to find
incomplete, faulty, and inconsistent behavior. The auth surface is broad and
largely well-built: password login, JWT access tokens (HS256, 15m) with
DB-backed refresh-token rotation (7d), TOTP + recovery codes, WebAuthn passkeys
(passkeys require TOTP as fallback), media tokens, RBAC (admin/user), encrypted
MFA secrets, and a first-boot setup/bootstrap flow. The review surfaced one real
client/server inconsistency (logout never revokes server-side), some
observability/security hardening gaps, and minor type-safety hygiene.

Every finding below was verified by reading the actual code on both sides
(handler + service + `AuthProvider`/`client` + generated `schema.d.ts`), not
inferred. Three candidate findings from the initial sweep were **disproved** and
are intentionally excluded:

- "Case-variant accounts (alice vs Alice) can coexist because the DB unique
  constraint is case-sensitive" — **false**. `normalizeUsername`
  (`server/internal/service/credential_policy.go:24-25`) lowercases on
  registration (`auth_passkeys.go:105,139`) and on admin update
  (`user_service.go`), and every lookup lowercases (`auth_service.go:170`,
  `auth_passkeys.go`). Usernames are always stored and queried lowercase, so no
  case-variant duplicate is reachable.
- "Force-logout after password change is a UX bug" — **false**. Backend
  `ChangePassword` revokes *all* refresh tokens atomically
  (`user_service.go:328-336`), so the frontend logging the user out
  (`ChangePasswordPage.tsx:93`) is the correct, consistent behavior, not a
  defect.
- "Refresh endpoint camelCase/snake_case mismatch" — **false**. Both the client
  (`client.ts`) and `dto.RefreshTokenRequestDTO`/`dto.AuthResponseDTO` use
  `refreshToken` (camelCase); confirmed in `schema.d.ts`.

Scope: `web/src/features/auth/*`, `web/src/lib/http-commons/{auth,client}.ts`,
`web/src/components/NavBar.tsx`, `server/internal/service/auth_service.go`,
`server/internal/service/auth_mfa.go`,
`server/internal/api/handler/auth_handler.go`. No API-contract change is
required for the priority fix (the logout endpoint and DTO already exist), so
`make dto` is only needed if a step below adds/changes annotations.

## Findings

### F1 — Logout never revokes the refresh token server-side (HIGH)

- The backend ships a working logout endpoint: `POST /api/v1/auth/logout`
  (`auth_handler.go:148-166`) takes `dto.RefreshTokenRequestDTO` and calls
  `RevokeRefreshToken` (`auth_service.go`), and it is registered as a public
  route (`router.go:296`). It is present in the generated client
  (`schema.d.ts:3975`).
- The frontend `logout()` only clears localStorage and dispatches `LOGOUT`
  (`AuthProvider.tsx:201-204` → `removeToken()` in
  `lib/http-commons/auth.ts:75-81`). A repo-wide search shows the logout
  endpoint is **invoked nowhere**; the only callers of `logout()` are
  `NavBar.tsx:193` and `ChangePasswordPage.tsx:93`.
- Consequence: after a user "logs out", their refresh token remains valid for
  the full `refresh_token_ttl` (default 7d / 168h). Anyone holding that token
  (e.g. from a shared/leaked device) can mint fresh access tokens. The
  client/server contract is inconsistent: the server implements revocation, the
  client never triggers it. This is the headline defect.

### F2 — Auth service logs warnings via `fmt.Printf` instead of the structured logger (MEDIUM)

- Failure paths that are deliberately non-fatal print to stdout instead of using
  the project's zap logger:
  - `auth_service.go:200-201` — failed `last_login` update on login.
  - `auth_service.go:250-252` — failed revoke of the old refresh token during
    rotation.
  - `auth_mfa.go:456-457` — failed `last_used` update on TOTP verify.
- These are security-relevant events (token rotation failure especially) and
  belong in the structured/audit log stream, not bare stdout. They are invisible
  to log aggregation and lose request context.

### F3 — Refresh-token rotation is not fail-closed and reuse is not detected (MEDIUM)

- `RefreshToken` (`auth_service.go:211-256`) issues the new token *before*
  revoking the old one, and a revoke failure is only logged
  (`:250-252`) — so a transient DB error can leave **two** valid refresh tokens.
- When an already-revoked token is presented again, the handler returns
  `ErrInvalidToken` (`:219-220`) but takes no further action. A stolen token
  that is later rotated by the legitimate user (or vice versa) is a classic
  refresh-token-reuse signal; the system does not treat reuse as compromise
  (no revoke-all-for-user / token-family invalidation).
- Lower-severity sibling: refresh does not re-check MFA/credential state beyond
  `is_active`, so a token minted before a security change keeps working until
  expiry. Acceptable for now but worth noting.

### F4 — Frontend auth type-safety papering (LOW)

- `AuthProvider.tsx:59` uses `null as any` during init when no tokens exist —
  removable by typing the state branch.
- `webauthn.ts:128,153` use `as unknown as
  PublicKeyCredentialCreationOptions/RequestOptions`. These are *largely
  unavoidable* (the generated DTO cannot express `BufferSource`/`ArrayBuffer`),
  but the casts should be isolated in a single typed adapter rather than spread
  across call sites; passkey response casts also recur at `LoginPage.tsx:130`
  and `useRegistrationFlow.ts:198,204`.
- These weaken type safety but are not currently breaking behavior. WebAuthn
  binary coercion is legitimate; the goal is containment, not elimination.

### F5 — Registration always fires TOTP setup even when MFA is skipped (LOW)

- `useRegistrationFlow.ts:150` starts the TOTP setup mutation unconditionally
  right after account creation, even though the user can skip MFA entirely
  (`:191`). This is a wasted round-trip and issues a setup token that is then
  discarded. Functionally correct, minor efficiency/clarity issue.

## Out-of-scope design tradeoffs (noted, not scheduled here)

These are real but are architectural decisions, not defects to silently flip in
this plan. Track in the tech-debt tracker; pick up only if explicitly chosen.

- **Tokens in `localStorage`** (`lib/http-commons/auth.ts:8-81`) are
  XSS-exposed. Moving the refresh token to an `HttpOnly` cookie is a larger
  cross-cutting change (CSRF strategy, desktop in-process host, media-element
  auth) and should be its own plan.
- **No rate limiting / brute-force protection** on `login`, `passkeys/login`,
  `mfa/verify`. Needs a shared middleware (per-IP + per-account throttle) and is
  better scoped as a dedicated hardening plan than bolted onto this review.

## Fix Plan

Ordered by severity; F1 is the priority and is self-contained (no contract
change).

### Step 1 — Revoke the refresh token on logout (F1)

- Add a logout mutation/helper in the auth feature that calls
  `POST /api/v1/auth/logout` with the stored `refreshToken` (read via
  `getRefreshToken()`), then clears local tokens regardless of the call's
  outcome. Logout must remain best-effort: a network/401 failure still clears
  local state and routes to `/login` (never trap the user in a logged-in UI).
- Wire `AuthProvider.logout()` to await this helper before
  `removeToken()` + `dispatch({type:"LOGOUT"})`. Keep the public `logout()`
  signature usable from `NavBar.tsx` and `ChangePasswordPage.tsx`
  (the latter already revokes server-side via password change, so a missing/sent
  token there is harmless and idempotent).
- Use the typed `$api`/generated client for the call — no raw `as` casts.
- Validation: log out from the navbar, confirm the network tab shows a
  `POST /auth/logout`, then confirm the previously-stored refresh token is
  rejected by `POST /auth/refresh` (401 / `ErrInvalidToken`).

### Step 2 — Route auth warnings through the structured logger (F2)

- Replace the three `fmt.Printf` warnings (`auth_service.go:200-201,250-252`,
  `auth_mfa.go:456-457`) with the service's zap logger at `Warn`, including
  `user_id` and operation fields. If `AuthService`/MFA service does not already
  hold a logger, inject one through its constructor consistent with sibling
  services. Do not change control flow (these stay non-fatal).
- Validation: `make server-test`; grep confirms no `fmt.Printf` remains in the
  auth service/MFA files.

### Step 3 — Make refresh rotation fail-closed + detect reuse (F3)

- Reorder/rework `RefreshToken` so issuing the new token and revoking the old
  one are atomic (single tx) or so a revoke failure fails the refresh
  (fail-closed) rather than logging and returning success.
- On presentation of an already-revoked (non-expired) refresh token, treat it as
  reuse: revoke all of the user's refresh tokens
  (`RevokeUserRefreshTokens`, already in `users.sql`) and return
  `ErrInvalidToken`. Add a structured `Warn` audit log for the reuse event.
- Add regression tests in the auth service test suite: (a) rotation revokes the
  old token, (b) reusing a rotated token triggers family revocation.
- Validation: `make server-test` with the new cases.

### Step 4 — Type-safety hygiene (F4, F5)

- F4: remove `null as any` in `AuthProvider.tsx:59` by typing the no-token init
  branch; consolidate WebAuthn DTO→DOM coercion into one adapter in
  `webauthn.ts` so call sites (`LoginPage.tsx`, `useRegistrationFlow.ts`) consume
  typed helpers instead of repeating `as` casts. Do not attempt to remove the
  unavoidable `BufferSource` cast inside the adapter.
- F5: defer the TOTP setup mutation in `useRegistrationFlow.ts` until the user
  actually enters the TOTP step, so skipping MFA issues no setup token.
- Validation: `cd web && vp check --no-fmt --no-lint && vp lint && vp test`.

## Validation

- Backend gate: `make server-test` (preserves the cgo allowlist). Extend the
  auth service tests for rotation/reuse (Step 3).
- Frontend gate: `cd web && vp check --no-fmt --no-lint && vp lint && vp test`.
- Contracts: no DTO change is required for F1–F5; run `make dto` only if a step
  touches `@Success`/`@Param` annotations. The logout endpoint and
  `RefreshTokenRequestDTO` already exist in `schema.d.ts`.
- Manual (`make dev`): log out → confirm `POST /auth/logout` fires and the old
  refresh token is rejected on the next `POST /auth/refresh`; register a new
  account and skip MFA → confirm no TOTP setup call is made.

## Open Questions

1. **F1**: on logout, revoke only the current device's refresh token (current
   plan) or all of the user's refresh tokens ("log out everywhere")? Default:
   current token only, matching the existing single-token endpoint.
2. **F3**: is fail-closed rotation acceptable given a DB blip would force a
   re-login, or should rotation stay best-effort with reuse-detection only?
   Default: fail-closed + reuse detection.
3. **Out-of-scope**: should `HttpOnly`-cookie refresh tokens and auth rate
   limiting be promoted into their own active plans now, or left in the
   tech-debt tracker?
