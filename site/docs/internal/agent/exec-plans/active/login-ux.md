# Identifier-first login UX

Status: planned (2026-07-09). Product review of the current Passkey / password /
TOTP login surface. Auth mechanics are stable; the gap is UX branching.

Scope: `web/src/features/auth` login flow + a small public auth capability probe
on the server. Enrollment invariants, MFA settings pages, bootstrap wizard, and
registration are out of scope unless a login-path bug forces a touch.

## Why

Today `/login` is a single screen that always shows:

1. Username
2. Passkey primary button (when the browser supports WebAuthn)
3. Password field + “Continue with password”

That fights the account model we already enforce:

| Account state | Exists? | Intended login |
|---------------|---------|----------------|
| Password only | Yes | Username → password → session |
| Password + TOTP | Yes | Username → password → TOTP / recovery |
| Password + TOTP + Passkey | Yes | Username → Passkey (primary), or password → TOTP |
| Passkey without TOTP | **No** | Enrollment returns `ErrTOTPRequiredForPasskey` |

Passkey is a full primary authenticator (user verification required) and
correctly skips TOTP. Password remains the fallback and still gates TOTP after
a successful password check. The bug is presentation: users without passkeys
still see a Passkey CTA that fails after click; users with passkeys still get a
password field competing for attention.

## Product decisions

1. **Identifier-first.** Step 1 is username only. Step 2 branches on capability
   + browser support.
2. **Passkey preferred when available.** If the account has ≥1 passkey and the
   browser supports WebAuthn in a secure context, show Passkey as the primary
   CTA. Keep a secondary “Use password instead” path (password → TOTP).
3. **Never hide the password fallback** for passkey accounts. Passkeys can be
   unavailable on a new device/browser; TOTP exists so that fallback stays
   usable.
4. **Do not reveal TOTP before password success.** The capability probe must
   not return `totp_enabled`. TOTP still appears only after
   `POST /auth/login` returns `requires_mfa`.
5. **Unknown / inactive usernames look like password-only.** The probe returns
   the same shape as a real password-only account (`password: true`,
   `passkey: false`) so existence is not trivially distinguishable from the
   probe alone. Real failure stays on password submit (existing generic 401).
6. **Accept limited passkey enumeration.** Returning `passkey: true` for real
   passkey accounts leaks “this username has a passkey.” That is the same class
   of signal `POST /passkeys/login/options` already gives on 200 vs 401. For a
   local-first / self-hosted app this is acceptable; document it. Rate limiting
   on auth endpoints remains tracked separately in `tech-debt-tracker.md`.
7. **Passkey stays a primary path, not a second factor on the MFA screen.** Do
   not add “verify with passkey” on the post-password MFA challenge in this
   plan. If the user wants passkey, they use the Step 2 passkey CTA (or go
   back). Backend may still list `"passkey"` in `mfa_methods`; login MFA UI
   continues to handle `totp` | `recovery_code` only.

## Target UX

```text
Step 1 — Identifier
  Username
  [Continue]

Step 2a — No passkey (or browser unsupported)
  Password
  [Sign in]
  → if TOTP enabled: existing MFA challenge (TOTP / recovery)

Step 2b — Has passkey + browser supported
  [Sign in with a passkey]          ← primary
  Use password instead              ← secondary link
    → password → TOTP / recovery (same as today)
```

Copy notes:

- Step 1 subtitle: identify the account, not “passkey or password”.
- Step 2b secondary: “Use password instead” (implies authenticator when TOTP
  is on; do not promise “password only”).
- Keep recovery-code toggle on the MFA challenge as today.

Optional follow-up (not required for this plan): WebAuthn conditional mediation
on the password field (`autocomplete` already includes `username webauthn`) so
browsers can offer a stored passkey without a separate button. Ship only if it
does not complicate Step 2 branching.

## Non-goals

- Changing enrollment rules (passkey still requires TOTP).
- Passkey-as-2FA after password on the MFA screen.
- Discoverable / usernameless passkey login (resident key without username).
- Auth rate limiting (already tech debt).
- Reworking `/register`, `/mfa`, or `/bootstrap` flows.
- Returning `totp_enabled`, display name, or other identity hints from the
  public probe.

## Backend

### New endpoint

`POST /api/v1/auth/login/options`

Request:

```json
{ "username": "alice" }
```

Response (always 200 for syntactically valid usernames):

```json
{
  "password": true,
  "passkey": false
}
```

Behavior:

| Condition | `password` | `passkey` |
|-----------|------------|-----------|
| Unknown username | `true` | `false` |
| Inactive user | `true` | `false` |
| Active, no passkeys | `true` | `false` |
| Active, ≥1 passkey | `true` | `true` |

Invalid username shape (empty / policy fail) → 400, same as other auth binds.

Implementation sketch:

- Handler on `AuthHandler`, route next to `POST /auth/login`.
- Service method: normalize username → `GetUserByUsername`; on miss/inactive
  return the password-only shape; on hit use existing MFA/passkey count query
  (`GetUserMFAStatus` / passkey list) and set `passkey` from `PasskeyCount > 0`.
- DTO + swag annotations → `make dto`.
- Tests: unknown, inactive, password-only, passkey account; assert response
  shapes and that TOTP is never in the payload.

Do **not** start a WebAuthn ceremony in this endpoint. Passkey login still uses
`/passkeys/login/options` + `/verify` after the user chooses Passkey.

### Unchanged contracts

- `POST /auth/login` — password; MFA challenge when TOTP on.
- `POST /auth/mfa/verify` — TOTP / recovery.
- `POST /passkeys/login/options|verify` — primary passkey session (no TOTP).

## Frontend

Rewrite `LoginPage` as a small step machine (local state is enough; no new
global store):

1. `identify` — username + Continue; call `login/options`.
2. `passkey` — primary Passkey button; secondary “Use password instead”.
3. `password` — password field + submit (entered from 2a or from 2b secondary).
4. `mfa` — existing challenge UI (mostly unchanged).

Rules:

- Entering `passkey` step requires `options.passkey === true` **and**
  `getPasskeySupport().supported`.
- If `options.passkey === true` but browser unsupported → go to `password` and
  show a short non-blocking note (reuse existing support reason strings).
- “Back” from step 2/3 returns to `identify` and clears password / passkey
  errors; keep username.
- Passkey click still runs the existing options → `getPasskeyCredential` →
  verify → `completeAuth` path.
- Password submit still uses `login()` / MFA challenge handling.
- i18n: `t("key", "default")` in code → `vp exec i18next-cli extract` → fill zh.
  Do not hand-edit translation JSON structure.
- Update `auth.type.ts` for the new DTO; keep `MFAMethod = "totp" | "recovery_code"`.

## Workstreams

### W0 — API probe

- DTO, service, handler, router, swag, `make dto`.
- Unit/handler tests for the four account shapes + invalid input.
- Confirm OpenAPI client types land in `web/src/lib/http-commons/schema.d.ts`.

### W1 — Login step UI

- Step machine in `LoginPage.tsx` (or a thin `useLoginFlow` hook if the file
  gets unwieldy).
- Wire `login/options` before showing passkey vs password.
- Preserve MFA challenge + recovery toggle behavior.
- Copy / i18n extract + zh fill.

### W2 — Validation

- Manual matrix:

  | Browser | Account | Expect |
  |---------||---------|--------|
  | WebAuthn OK | password only | identify → password → session |
  | WebAuthn OK | + TOTP | identify → password → MFA |
  | WebAuthn OK | + TOTP + passkey | identify → passkey primary; secondary password → MFA |
  | WebAuthn unsupported | + passkey | identify → password (+ note) → MFA |
  | any | unknown user | identify → password → generic 401 |

- Automated: extend auth frontend tests if present; otherwise backend tests +
  manual checklist above.
- Quality gates: `make server-test` (or targeted auth packages with Makefile
  CGO env), `make web-test` / `vp check` + lint + test for the auth feature.

## Risks

- **Enumeration:** `passkey: true` leaks enrollment. Mitigated by matching
  unknown users to the password-only shape and by existing generic login
  errors. Call out in PR description.
- **Stale probe:** user enrolls/removes a passkey in another session between
  probe and click. Passkey options/verify already fail closed; password path
  still works. Optional: re-probe is unnecessary for v1.
- **Double username entry friction:** one extra Continue click. Acceptable;
  matches the product goal. Do not auto-advance on blur.

## Done when

- Login no longer shows Passkey and password on the same initial screen.
- Passkey accounts get Passkey-primary + password fallback.
- Non-passkey accounts never see a Passkey CTA (unless conditional mediation
  is added later as an optional enhancement).
- TOTP is still only revealed after successful password verification.
- `make dto` regenerated; server + web quality gates pass for the touched
  surface.

## Critical files

- `web/src/features/auth/routes/LoginPage.tsx`
- `server/internal/api/handler/auth_handler.go`
- `server/internal/service/auth_service.go` (or a small sibling for the probe)
- `server/internal/api/dto/auth_dto.go`
- `server/internal/api/router.go`
