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

> **Status:** F1–F4 implemented. F5 and the WebAuthn-cast "consolidation" part of
> F4 are descoped with rationale (see those findings). Out-of-scope tradeoffs
> (localStorage→cookies, rate limiting) and the missing DB-backed rotation test
> are recorded in `tech-debt-tracker.md`. Decisions on the open questions are
> resolved inline below.
>
> Implementation notes:
> - F1: `AuthProvider.logout()` is now async and best-effort — it calls
>   `POST /api/v1/auth/logout` with the current device's refresh token, then
>   clears local tokens regardless of the outcome; the two callers
>   (`NavBar.tsx`, `ChangePasswordPage.tsx`) use `void logout()`.
> - F2: the three `fmt.Printf` warnings now use an injected `*zap.Logger`
>   (`NewAuthService` takes a variadic logger, defaulting to `zap.NewNop()`;
>   wired in `app/app.go` as `appLogger.Named("auth")`).
> - F3: `RefreshToken` rotates fail-closed (revokes the presented token before
>   issuing a new one; a revoke failure aborts) and treats reuse of an
>   already-revoked token as compromise by revoking the whole token family.
> - F4: removed `null as any` (the no-token init path now dispatches `AUTH_IDLE`,
>   which is the correct non-error idle state). The WebAuthn casts are already
>   isolated in `coerceCreationOptions`/`coerceRequestOptions` adapters and are
>   unavoidable (`BufferSource`), so no further change was warranted.
> - Gates: backend `go build ./...` + `go test ./internal/service/...
>   ./internal/api/... ./app/...` pass; `gofmt` clean. The web gate (`vp`) could
>   not run in this sandbox (Vite+ is a licensed CLI; `viteplus.dev` returns 403
>   and `node_modules` is absent) — the frontend changes are small/type-safe and
>   the web gate must be run before merge.

## Findings

### F1 — Logout never revokes the refresh token server-side (HIGH) — ✅ DONE

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

### F2 — Auth service logs warnings via `fmt.Printf` instead of the structured logger (MEDIUM) — ✅ DONE

- Failure paths that are deliberately non-fatal print to stdout instead of using
  the project's zap logger:
  - `auth_service.go:200-201` — failed `last_login` update on login.
  - `auth_service.go:250-252` — failed revoke of the old refresh token during
    rotation.
  - `auth_mfa.go:456-457` — failed `last_used` update on TOTP verify.
- These are security-relevant events (token rotation failure especially) and
  belong in the structured/audit log stream, not bare stdout. They are invisible
  to log aggregation and lose request context.

### F3 — Refresh-token rotation is not fail-closed and reuse is not detected (MEDIUM) — ✅ DONE

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

### F4 — Frontend auth type-safety papering (LOW) — ✅ DONE (partial, by design)

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

### F5 — Registration always fires TOTP setup even when MFA is skipped (LOW) — DESCOPED

- `useRegistrationFlow.ts:150` starts the TOTP setup mutation unconditionally
  right after account creation, even though the user can skip MFA entirely
  (`:191`). This is a wasted round-trip and issues a setup token that is then
  discarded. Functionally correct, minor efficiency/clarity issue.
- **Descoped:** the current UX lands the user on the TOTP step (with its QR)
  immediately after registration, so the setup call is needed to render that
  step. Avoiding the wasted token when the user skips requires inserting a new
  "set up two-factor auth?" gate *before* the QR — a behavioral UX change beyond
  a hygiene fix, and one that would disturb the carefully-handled
  `startedRef`/redirect logic. Not worth the churn for a LOW efficiency nit;
  leaving as-is.

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

### Step 1 — Revoke the refresh token on logout (F1) — ✅ DONE

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

### Step 2 — Route auth warnings through the structured logger (F2) — ✅ DONE

- Replace the three `fmt.Printf` warnings (`auth_service.go:200-201,250-252`,
  `auth_mfa.go:456-457`) with the service's zap logger at `Warn`, including
  `user_id` and operation fields. If `AuthService`/MFA service does not already
  hold a logger, inject one through its constructor consistent with sibling
  services. Do not change control flow (these stay non-fatal).
- Validation: `make server-test`; grep confirms no `fmt.Printf` remains in the
  auth service/MFA files.

### Step 3 — Make refresh rotation fail-closed + detect reuse (F3) — ✅ DONE

- Reorder/rework `RefreshToken` so issuing the new token and revoking the old
  one are atomic (single tx) or so a revoke failure fails the refresh
  (fail-closed) rather than logging and returning success. **Done:** the
  presented token is revoked first and a revoke error aborts the refresh.
- On presentation of an already-revoked (non-expired) refresh token, treat it as
  reuse: revoke all of the user's refresh tokens
  (`RevokeUserRefreshTokens`, already in `users.sql`) and return
  `ErrInvalidToken`. Add a structured `Warn` audit log for the reuse event.
  **Done.**
- Regression tests: **deferred** — the service test suite has no Postgres
  harness and `s.queries` is a concrete type (not mockable). Recorded in
  `tech-debt-tracker.md`; add once the integration DB harness is available.
- Validation: backend `go test ./internal/service/...` passes (existing cases);
  rotation/reuse covered by build + review.

### Step 4 — Type-safety hygiene (F4, F5) — ✅ DONE (F4) / DESCOPED (F5)

- F4: removed `null as any` in `AuthProvider.tsx` — the no-token init branch now
  dispatches `AUTH_IDLE` (the correct non-error idle state). WebAuthn DTO→DOM
  coercion is **already** isolated in `coerceCreationOptions`/
  `coerceRequestOptions`; the `BufferSource` casts there are unavoidable, so no
  further consolidation was warranted.
- F5: **descoped** (see finding F5) — deferring the TOTP setup call would require
  a new pre-QR MFA gate (a UX change), not worth it for a LOW nit.
- Validation: web gate (`vp check --no-fmt --no-lint && vp lint && vp test`)
  pending — not runnable in this sandbox (licensed Vite+ CLI unavailable).

## Validation

- Backend gate: ✅ `go build ./...` and `go test ./internal/service/...
  ./internal/api/... ./app/...` pass; `gofmt` clean. (Ran with the cgo allowlist
  after installing `libvips`/`libraw` system deps in-sandbox.) Rotation/reuse
  regression test deferred — see `tech-debt-tracker.md`.
- Frontend gate: ⏳ `cd web && vp check --no-fmt --no-lint && vp lint && vp test`
  — NOT run in this sandbox (Vite+ is a licensed CLI; `viteplus.dev` returns 403
  and `node_modules` is absent). Must be run before merge.
- Contracts: no DTO change was required for F1–F4; `make dto` not needed. The
  logout endpoint and `RefreshTokenRequestDTO` already exist in `schema.d.ts`.
- Manual (`make dev`): log out → confirm `POST /auth/logout` fires and the old
  refresh token is rejected on the next `POST /auth/refresh`; present a rotated
  (already-revoked) refresh token → confirm all the user's sessions are revoked.

## Open Questions

1. ~~**F1**: on logout, revoke only the current device's refresh token or all of
   the user's refresh tokens ("log out everywhere")?~~ **Resolved:** current
   device's token only (matches the existing single-token endpoint).
2. ~~**F3**: fail-closed rotation vs. best-effort with reuse-detection only?~~
   **Resolved:** fail-closed + reuse detection.
3. ~~**Out-of-scope**: promote `HttpOnly`-cookie refresh tokens and auth rate
   limiting into their own active plans now, or leave them in the tech-debt
   tracker?~~ **Resolved:** left in the tech-debt tracker.
