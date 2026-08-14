---
name: lumilio-integration-spec
description: Use when writing or debugging a Vitest integration test in
  Lumilio Photos — component *.test.tsx or flow *.spec.tsx in real Chromium
  with MSW, shared test helpers, locators, and the import gotchas that fail
  the whole suite.
---

# Write A Vitest Integration Spec

Component `*.test.tsx` and colocated flow `*.spec.tsx` run in real Chromium
via `vitest-browser-react` — there is no jsdom/happy-dom, so layout, CSS,
`matchMedia`, `ResizeObserver`/`IntersectionObserver`, storage and events are
the real implementations. Layer assignment:
[lumilio-write-a-test](../lumilio-write-a-test/SKILL.md). Rationale:
[test-layer assignment decision](../../decisions/2026-08-14-frontend-test-layer-assignment.md).

## Shared infrastructure

Lives under `web/test/` (alias `@test`):

- `@test/render` — `renderWithProviders(ui, opts)` wraps real i18n, global
  context and a fresh `QueryClient` (retries off, so a mocked error surfaces
  at once). `opts.auth: true` adds the real `AuthProvider`; `opts.router: false`
  when the spec brings its own `MemoryRouter`/`Routes` (needed for `useParams`,
  a seeded history, or custom `Routes`). Returns the `vitest-browser-react`
  result — `await` it.
- `@test/msw` — the shared `setupWorker` plus re-exported `http`/`HttpResponse`.
  Declare per-test responses with `worker.use(http.get("*/api/v1/…", …))`; the
  `*` origin prefix matches whatever base URL the client builds. Only `/api/`
  requests are guarded (erroring when unhandled) so Vite can still serve
  modules.
- `@test/i18n` — `t(key, opts?)` resolves a translation key to its current `en`
  copy through the app's own i18next instance.
- `@test/session` — `seedSession(user)` stores an access token plus CSRF proof
  and answers the cookie-session bootstrap (`/auth/csrf` + `/auth/refresh`),
  `/auth/me`, and media-token requests so the real `AuthProvider` settles to
  `user`. Pair with `renderWithProviders(ui, { auth: true })`.

## Drive real code through the HTTP boundary

Mock **only** the HTTP boundary through MSW. Do not mock `$api`, Query/Router
hooks, or feature stores — drive them with real data via `worker.use` and,
where a hook needs scope context, wrap in the real provider (e.g.
`AssetBrowserScope` with `initialSelection` to seed selection without a
gallery). Type fixtures with the generated DTOs and `satisfies`, never
`as any`.

A component `*.test.tsx` may stub a genuinely heavy **leaf child** at a clear
boundary (a full-screen `AssetViewer`, a WASM gallery) to keep the subject in
focus; a flow `*.spec.tsx` should not.

Core-browsing UI — the full `AssetBrowser` with WASM justified layout,
viewport virtualization, URL/route state and real selection — belongs in
Playwright. Do not force it into the integration project. Extract pure logic
to a unit test and put the interactive path in `e2e/specs/`. This is why
`PhotoPicker` and `AlbumDetailsFlow` have E2E placeholders rather than
integration specs.

## Locators

Match E2E discipline ([lumilio-e2e-spec](../lumilio-e2e-spec/SKILL.md)):

- **No app-copy literals.** Resolve accessible names by key through `t` from
  `@test/i18n`. Rewording a string keeps specs green; renaming a key fails
  them.
- Only strings the spec owns (route sentinels, fixtures) are literals.
- `getByRole`/`getByLabelText` names match as substrings — pass
  `{ exact: true }` when a shorter name also matches a longer one
  ("Confirm" vs "Confirm action", "New password" vs "Confirm new password").
- There is **no `getByDisplayValue`** in the browser locator API; target
  inputs by their label.

## Gotchas

- **Import cross-feature test infrastructure by its narrow module path, never
  the feature barrel.** `@test/render` imports `AuthProvider` from
  `@/features/auth/state/AuthProvider`, not `@/features/auth`. The barrel
  eagerly evaluates the whole feature graph (webauthn, MFA, gates), pulling a
  second React / react-query instance into the pre-bundle; hooks then read a
  null context and throw `Cannot read properties of null (reading
  'useContext')`.
- **One unresolved import anywhere under `src/` fails the whole dependency
  scan**, so no integration test runs. While migrating, park a
  not-yet-rewritten file out of the glob rather than leaving a broken import
  in range.
- **A `let` mutated only inside a closure narrows to `never` under optional
  chaining.** A module-level `let probe = null` assigned inside a `vi.mock`
  factory, then reset with `probe = null` in the test body, makes TS treat
  `probe?.prop` as access on `never`. Use a container object
  (`const probe = { current: null }`) and reset it in `beforeEach`, not
  inline.

## Verify

```sh
cd web && vp test run src/features/<feature>/flows/<flow>/<file>.spec.tsx
task web:test
```
