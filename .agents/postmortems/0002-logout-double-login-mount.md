# Postmortem 0002: Logout mounted the login form twice

## Executive summary

The shell's Logout button and the password-change flow both started
`logout()` without awaiting it and navigated to `/login` right away. `/login`
therefore mounted while the session was still authenticated, and the login
flow redirected straight back to `/`. When logout finished, `ProtectedRoute`
sent the browser to `/login` again, which mounted a second, empty login form.
Anything typed into the first form was lost. On a slow CI runner the
`@auth-totp` E2E slice filled the username into the first form. The second
form then showed `Continue` disabled until the 90-second timeout (workflow run
35816755444). Sign-out now goes through one `useSignOut` hook, which awaits
`logout()` before it navigates.

## What broke

`SideBar` ran `void logout(); void navigate("/login", { replace: true })`.
`logout()` holds the browser-session Web Lock, fetches a CSRF proof, and then
posts `/auth/logout`. After that it runs `resetSession` (Query cache clear) and
dispatches `LOGOUT`. Until then `isAuthenticated` stays true, so the first
`LoginFlow` redirected to `/` in its mount effect. Instrumented E2E runs
recorded `LoginFlow mount → redirect → unmount → logout done →
BootstrapGate loading → LoginFlow mount` on every logout. The login card fades
in on mount, and Playwright treats an opacity-0 input as visible, so `fill`
succeeded on the doomed first form.

## Why every net missed it

- The window is timing-dependent. Locally the transient `/login` lived about
  10 ms, and Playwright's Logout `click()` usually outlasted it. The spec's
  `toHaveURL(/\/login$/)` then matched only the second, stable form.
- No component or flow test covered the sign-out sequence. `LoginFlow` specs
  start from an unauthenticated provider.
- The bounce was not visible to users, who had not started typing yet.

## Guardrails added

- [The sign-out flow spec](../../web/src/features/auth/flows/sign-out/SignOut.spec.tsx)
  holds `/auth/logout` open and asserts that no login form mounts before it
  resolves. It also asserts that exactly one form mounts afterwards and keeps
  its typed username. The spec fails against the old ordering.
- [`useSignOut`](../../web/src/features/auth/flows/sign-out/useSignOut.ts) is
  the single sign-out path, recorded in the auth feature doc. The spec runs
  under `task web:test`.
