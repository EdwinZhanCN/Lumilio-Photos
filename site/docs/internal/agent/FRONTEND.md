# Frontend

This document describes the current React frontend as implemented in `web/`.

## Runtime Entry

- App entry: `web/src/main.tsx`.
- Root application composition and providers: `web/src/app/App.tsx`.
- Router gates and route table: `web/src/app/router/AppRouter.tsx`, `web/src/app/router/routes.tsx`.
- Authenticated navigation shell: `web/src/app/shell/AppShellLayout.tsx`.
- Runtime health polling: `web/src/app/status/HealthPoller.tsx`.
- Vite+ config: `web/vite.config.ts`.
- Container runtime: `web/Dockerfile`, `web/Caddyfile`, `web/scripts/docker-entrypoint.sh`.

The app mounts `I18nProvider`, then `PreferencesEffects`, `GlobalProvider`, `QueryClientProvider`, `AuthProvider`, router/bootstrap gates, worker/upload providers, and the shell layout.

## Toolchain

The frontend uses Vite+ as the command surface.

Core stack:

- React 19
- TypeScript
- React Router 7
- TanStack Query 5
- Zustand 5 with immer
- Tailwind CSS 4 and DaisyUI 5
- Vitest 4 through Vite+
- Web Workers and WASM modules for compute-heavy paths

Daily commands:

```bash
make web-dev
make web-test
```

Use the Makefile targets by default. Direct commands are acceptable when you are
intentionally running only the web workspace:

```bash
cd web
vp dev --host --port 6657
vp check --no-fmt --no-lint
vp lint
vp test
```

For i18n (internationalization), every user-facing string literal must go through the i18n layer. The workflow is **extract-then-fill — never hand-edit translation JSON structure**:

1. **Write code first**: use `t("dotted.key", "English default")` with an inline default. The default doubles as the en value and tells the extractor the key exists.
2. **Extract**: run `vp exec i18next-cli extract` — scans `src/**/*.{ts,tsx}` and creates/updates keys in `src/locales/{en,zh}/translation.json` automatically.
3. **Fill zh**: open `src/locales/zh/translation.json`, translate any new/empty keys. Verify with `vp exec i18next-cli status` (must reach 100%).

```bash
vp exec i18next-cli extract    # step 2: auto-generate keys from code
vp exec i18next-cli status     # step 3: verify zh coverage (must be 100%)
```

**Do NOT** manually add keys, restructure JSON nesting, or delete keys by hand in `translation.json`. The extractor is the single source of truth for key structure; manual edits will be overwritten or cause drift. Only fill values for keys the extractor has created.

## Source Layout

- `src/app`: root providers, router composition, application shell, and runtime status effects.
- `src/features/*`: domain features. The enforced shape and dependency rules live in `web/ARCHITECTURE.md`; the quick placement guide is `web/src/features/README.md`.
- `src/components`: reusable app components and UI pieces.
- `src/contexts`: cross-cutting providers.
- `src/lib`: API client, i18n, utilities, feature support libraries.
- `src/lib/http-commons`: generated OpenAPI schema, typed client, React Query integration.
- `src/styles`: global styles.
- `src/locales`: translation resources.
- `src/wasm`: checked-in generated/bundled WASM support code.
- `src/workers`: browser worker entry points and worker tests.

Current feature areas are assets, auth, cloud, collections, home, Lumilio, manage, monitor, notifications, people, repositories, settings, share, studio, upload, and users.

### Feature Ownership

Feature roots use one optional vocabulary:

- `api/`: reusable TanStack Query reads/mutations and DTO adapters.
- `model/`: React-free domain rules, value types, codecs, validation, and transformations.
- `flows/<workflow>/`: the default owner of user-journey UI, orchestration, flow-local hooks/state, tests, and styles.
- `components/`: UI with real consumers in multiple flows of the same feature.
- `hooks/`: rare React mechanisms reused across multiple flows.
- `state/`: only cross-flow or refresh-spanning state, persistence, migration, hydration, and reset.
- `modules/`: isolated technical capabilities that are not themselves a user journey.
- `routes/`: thin router entries that delegate to a flow.
- `utils/`: legacy/general pure helpers without domain vocabulary; prefer `model/` or a named lower-layer `lib/` owner for new code.
- `docs/`: feature-local supporting notes; `doc.ts` and generated `doc.md` stay at the feature root.

Directories are optional. Do not create placeholders or alternate roots, and do
not leave compatibility re-exports at old internal paths. Inside a feature use
relative imports; between features use the target feature's public `index.ts`
except for reviewed narrow entries documented in `web/ARCHITECTURE.md`.

## API Contract

OpenAPI is the source of truth for HTTP contracts.

- Use `$api` from `src/lib/http-commons/queryClient.ts`.
- Prefer `$api.useQuery`, `$api.useInfiniteQuery`, and `$api.useMutation`.
- Do not hand-edit `src/lib/http-commons/schema.d.ts`.
- Do not create ad-hoc request/response types when an endpoint exists in OpenAPI.

> **An `as` cast on an API response is a red flag — never use it to "fix" a type.**
> If you find yourself writing `someQuery.data?.data as { ... }` (or `as any`) to
> read response fields, the generated type is wrong or missing. Do **not** cast
> around it — casting silently desyncs the frontend from the contract. Instead:
> 1. Check the backend handler's `@Success ... {data=dto.XxxDTO}` annotation —
>    does the referenced DTO actually declare the fields you need?
> 2. If the DTO is correct but `schema.d.ts` shows `Record<string, never>`,
>    `unknown`, or stale/missing fields, the **generated contract is broken** →
>    fix the backend annotation/DTO/codegen and run `make dto`.
> 3. Only then read the now-typed field.
>
> Do not add frontend compatibility shims, endpoint-local response casts, or
> hand-written response types to work around stale DTO output. Runtime guards are
> allowed only as defensive checks after the generated contract is correct; they
> are not a substitute for fixing OpenAPI.
>
> Real bug this caught: `/assets/filter-options` returns `camera_models`, but a
> cast had guessed `cameras`, silently breaking the camera `@`-mention. The DTO
> (`dto.OptionsResponseDTO`) and its `@Success` annotation were both correct —
> `make dto` simply had not been re-run, so the cast masked the stale type.

For API changes:

1. Update backend annotations and handler behavior.
2. Run `make dto`.
3. Update frontend hooks/components against generated types.

The checked-in fetch/query runtime comes from the official `openapi-fetch`,
`openapi-react-query`, and `openapi-typescript-helpers` packages. `make dto`
runs `web/scripts/generate-openapi-types.mjs`, which removes the known empty
object branch emitted by swag v2 for required JSON request bodies before type
generation. Keep this normalization in the generator; never post-edit
`schema.d.ts`.

## State Boundaries

Use TanStack Query for server state:

- fetched backend data
- cache lifecycle
- loading/error state
- pagination and refetch behavior

Use Context for cross-cutting runtime capabilities:

- auth session
- global runtime/notification coordination
- worker dependencies

Use flow-local Zustand or `useReducer` for interaction shared by several
components in one workflow. Use component-local state for one component, URL
state for linkable/restorable page state, `useRef` for non-rendering temporary
values, and versioned storage only for explicitly refresh-safe preferences.

Session teardown is centralized in `features/auth/state/resetSession.ts`. Logout and
refresh exhaustion must use that boundary so in-flight Query/Lumilio work and
all user-scoped caches, notifications, repository choices, searches, and
filters are cleared before another user authenticates.

Do not mirror the same data across Query, Context, Zustand, URL, or storage.
Root feature `state/` is reserved for lifecycles that genuinely span flows or
refreshes; otherwise colocate state with the owning flow.

Repository scoping uses `useBrowseScope` for list pages,
`useWorkingRepository` for upload only, the entity's own `repository_id` for
entity actions, and Manage for maintenance jobs. Do not add repository
parameters to person/album detail pages or mutations.

## Routing And Shell

Public routes include login and register. Bootstrap routes handle first-user setup. Protected standalone routes handle MFA and password changes.

Main app routes are rendered inside the shell with `NavBar`, `SideBar`, a scroll container, and the global ChatDock (except on `/lumilio`). The route table in `web/src/app/router/routes.tsx` is authoritative. Its stable route families are:

- Home and library: `/`, `/assets/*`.
- Collections: `/collections`, albums, places/map, people, folders, tags, liked, trash, shared links, and utility/classifier views.
- Entity detail: album, trip, folder, tag, person, and asset routes with optional asset-viewer segments.
- Operations: `/manage`, `/settings`, `/studio`, `/server-monitor`, and `/lumilio`.
- Public/auth/setup: `/s/:token/*`, login, registration, password/MFA, and bootstrap routes outside or around the authenticated shell as appropriate.

Studio, Map, Lumilio, Monitor, and Settings are route-level lazy chunks. The
global ChatDock also lazy-loads its message renderer and does not mount its
expanded body/input queries while collapsed.

Legacy compatibility routes also redirect `/upload-photos` to `/manage`.

The final top-level `*` route renders a public 404 recovery page outside setup
and authentication gates, so invalid URLs are explained rather than redirected.
`main.tsx` wraps the complete application/provider tree in a root error boundary;
its fallback deliberately uses a document link instead of router state so it
still works when the router itself fails.

## Browser Runtime

The Vite dev server sets:

- `Cross-Origin-Opener-Policy: same-origin`
- `Cross-Origin-Embedder-Policy: credentialless`

The production web image uses Caddy:

- serves static files from `/usr/share/caddy`
- reverse proxies `/api/*` to `LUMILIO_API_UPSTREAM`
- supports HTTP/1, h2c on `:80`, and HTTP/1/2/3 on `:443`
- sets immutable cache headers for static assets
- serves WASM with `application/wasm`
- serves the same COOP/COEP isolation headers as development; the desktop Go
  SPA fallback also sets them on documents and static assets
- falls back to `index.html` for SPA routes

## Large-library boundaries

- Square and justified galleries preserve full scroll geometry but mount only
  an overscanned viewport window. Offscreen thumbnail/media nodes are removed,
  and inactive asset list/search queries use bounded garbage-collection times.
- The Home map waits until visible and requests a bounded preview. The Map route
  queries `/assets/map-points` with its current WGS-84 viewport; only Trips opts
  into draining all map-point and location-cluster pages.
- `web/scripts/check-bundle-budget.mjs` enforces a 420 KiB gzip budget for the
  production entry chunk as part of `make web-browser-test`.

## Z-Index Strategy

Three rules, in priority order:

1. **Decorative overlays → DOM order.** Gradient tints, badges, hover masks, and
   other purely visual layers must not carry a z-index. Place them after the
   content they cover in DOM order.
2. **Component-internal overlap → `isolation: isolate`.** When a component has
   multiple overlapping layers (dropdown inside a card, sticky header inside a
   panel), add `isolate` to the component root and keep internal values small
   (z-10 / z-20 / z-30). Internal z-index must never leak into the global stack.
3. **Cross-component floating layers → z-index tokens.** Use the theme tokens
   defined in `App.css` `@theme inline`:

| Token | Value | Use |
| --- | --- | --- |
| `z-sticky` | 100 | Sticky headers, save bars |
| `z-dropdown` | 200 | Dropdown menus, popovers, autocomplete |
| `z-overlay` | 300 | FABs, floating docks, drag overlays |
| `z-modal` | 400 | Modals, drawers, mobile bottom-sheets |
| `z-lightbox` | 500 | Fullscreen viewers (AssetViewer, PublicShareLightbox) |
| `z-tooltip` | 600 | Portaled tooltips/popovers that escape a lightbox |

Inline styles use `var(--z-<token>)`. Do not introduce new numeric z-index
values for cross-component layers; extend the token scale if a new tier is
genuinely needed.

## Test layers

Pick the layer by what the test must exercise; the file name and directory pick
the runner and dependency boundary for you (`web/vite.config.ts` `test.projects`
maps them). Do not invent other conventions — e.g. there is no
`*.integration.test.tsx`; `flows/<flow>/*.spec.tsx` and `e2e/specs/*.spec.ts`
share the word "spec" but their directory, extension, runner and dependency edge
already disambiguate.

| Layer | File | Runner / Vitest project | Answers |
| --- | --- | --- | --- |
| Unit | `*.test.ts` | `unit` — Node, no DOM | React-free rules, transforms, codecs, validators, reducers, state migrations, algorithms |
| Component | `*.test.tsx` | `integration` — Browser Mode (real Chromium) | one component or small tree: accessible semantics, state, interaction |
| Flow Integration | `*.spec.tsx` | `integration` — Browser Mode + MSW | flows, routes, multi-component, Router, Query, HTTP workflows |
| Browser Capability | `*.browser.test.ts` | `browser` — Chromium | Worker, WASM, SSE, Blob, Canvas/WebGL — real browser capabilities |
| Full E2E | `web/e2e/specs/*.spec.ts` | Playwright + real services | key user paths on real API, DB, storage, queues |

The `unit` project excludes `*.browser.test.ts` and `src/workers/**` so an
accidental browser dependency fails instead of hiding; `integration` and
`browser` run real Chromium via the Playwright provider. Details per layer:
Integration Specs and E2E have their own sections below.

### GPU / WebGL capability tests

A `*.browser.test.ts` gets a real Worker, WASM, Canvas and WebGL context — but
**headless Chromium falls back to SwiftShader, whose WebGL is disabled on Apple
Silicon**, so a WebGL2-dependent test cannot get a context headless on an M-series
Mac (and usually not on a GPU-less CI runner either). Such a test must therefore:

- **Guard the capability** at runtime and skip, never fail: probe with a helper
  like `webgl2Available()` (`new OffscreenCanvas(1,1).getContext("webgl2")`) and
  wrap the suite in `describe.skipIf(!webgl2Available())`. A skipped capability is
  correct; a suite that fails to launch a browser is not.
- **Run headed locally** to actually exercise it. The `browser` project reads
  `STUDIO_GPU=true` (see `vite.config.ts`, same env-gated shape as
  `hash-performance`) and switches to `headless: false`, using the machine's real
  GPU. So `STUDIO_GPU=true vp test <files>` runs them for real; the default stays
  headless so CI can always launch (those suites skip there).

Non-GPU capability tests (Canvas 2D, Worker, WASM) run headless everywhere and
need no guard — keep them assertable in CI.

## Quality Gate

Frontend gate:

```bash
make web-test
make web-browser-test
```

Direct equivalent when intentionally scoped to `web/`:

```bash
cd web && vp check --no-fmt --no-lint && vp lint && vp test
cd web && vp run e2e:up
cd web && vp run e2e:seed
cd web && vp run e2e:test --grep @smoke
cd web && vp run e2e:down
```

`web-browser-test` runs the `@smoke` subset of the Playwright E2E suite against
the isolated Compose environment. The first-party API, PostgreSQL, storage, and
queues are real; only external services may be replaced. Run `e2e:up` first and
`e2e:down` afterwards. Install the project-pinned browser revision with
`vp exec playwright install chromium` locally, or
`vp exec playwright install --with-deps chromium` on Linux CI.

Rebuild the `web` service (`docker compose -f docker-compose.e2e.yml -p
lumilio-photos-e2e up -d --build web`) after changing frontend source; the
container serves a built image, so edits are otherwise invisible to the suite.

### E2E Locators

Specs run under `locale: "en-US"` (set in `playwright.config.ts`), which is what
i18next detects from `navigator`. Pick locators in this order:

1. `getByRole(role, { name })` with the name resolved through `e2e/support/i18n.ts`,
   which reads the same `en` bundle the app renders. Roles are semantic and are
   never translated; rewording a string keeps specs green, renaming a key fails
   them — which is the structural change that should fail.
2. Data anchors from `.cache/e2e/seed.json` via the `seed` export in
   `e2e/fixtures/test.ts`. Filenames and ids are data, not copy.
3. API and URL facts. `waitForResponse` on a real response beats waiting for UI
   wording, and it is the only reliable signal for work that continues after the
   request is accepted.
4. `getByTestId`, only for elements with no stable accessible semantics. Scope
   dynamic rows through a container test id plus a data attribute instead of
   interpolating a runtime id into the test id.

Two things are not allowed:

- **UI copy literals in specs.** They couple tests to translations, so every
  wording change drags a spec change behind it.
- **`aria-label` as a test hook.** It is user-facing text that screen readers
  announce. Translated, it couples to copy exactly like visible text and buys
  nothing; frozen in English to stabilise tests, it breaks non-English assistive
  technology. On an element that already has visible text it also overrides the
  accessible name, violating WCAG 2.5.3 (Label in Name) and breaking voice
  control. Reserve `aria-label` for elements with no visible text, such as
  icon-only buttons.

Form fields get their accessible name from `<label htmlFor>` paired with
`<input id>`. `Field` in `features/auth/components/ui/Fields.tsx` generates the
id with `useId` and passes it down through context, so `TextInput` and
`PasswordField` pair automatically and no call site can forget. Follow that
shape when adding field components rather than re-exposing an optional
`htmlFor`, which is how the pairing was missed before.

Assert on data and API facts, not on wording; copy correctness belongs to i18n,
not to E2E. Note that `getByLabel` matches substrings — pass `{ exact: true }`
where a shorter label would otherwise also match a longer one, as "Password"
does against the "Show password" toggle.

### Integration Specs (Vitest `integration` project)

Component `*.test.tsx` and colocated flow `*.spec.tsx` run in real Chromium via
`vitest-browser-react` — there is no jsdom/happy-dom, so layout, CSS,
`matchMedia`, `ResizeObserver`/`IntersectionObserver`, storage and events are the
real implementations. See ADR-006 for the layering rationale. Shared test
infrastructure lives under `web/test/` (alias `@test`):

- `@test/render` — `renderWithProviders(ui, opts)` wraps real i18n, global
  context and a fresh `QueryClient` (retries off, so a mocked error surfaces at
  once). `opts.auth: true` adds the real `AuthProvider`; `opts.router: false`
  when the spec brings its own `MemoryRouter`/`Routes` (needed for `useParams`,
  a seeded history, or custom `Routes`). Returns the `vitest-browser-react`
  result — `await` it.
- `@test/msw` — the shared `setupWorker` plus re-exported `http`/`HttpResponse`.
  Declare per-test responses with `worker.use(http.get("*/api/v1/…", …))`; the
  `*` origin prefix matches whatever base URL the client builds. Only `/api/`
  requests are guarded (erroring when unhandled) so Vite can still serve modules.
- `@test/i18n` — `t(key, opts?)` resolves a translation key to its current `en`
  copy through the app's own i18next instance.
- `@test/session` — `seedSession(user)` stores tokens and answers the auth
  bootstrap (`/auth/me` + media token) so the real `AuthProvider` settles to
  `user`. Pair with `renderWithProviders(ui, { auth: true })`.

Mock **only** the HTTP boundary through MSW. Do not mock `$api`, Query/Router
hooks, or feature stores — drive them with real data via `worker.use` and, where
a hook needs scope context, wrap in the real provider (e.g. `AssetBrowserScope`
with `initialSelection` to seed selection without a gallery). Type fixtures with
the generated DTOs and `satisfies`, never `as any`. A component `*.test.tsx` may
still stub a genuinely heavy **leaf child** at a clear boundary (a full-screen
`AssetViewer`, a WASM gallery) to keep the subject in focus; a flow `*.spec.tsx`
should not.

**What belongs in E2E instead.** Core-browsing UI — the full `AssetBrowser` with
its WASM justified layout, viewport virtualization, URL/route state and real
selection — is assigned to Playwright by ADR-005/006. Do not force it into the
integration project; a real render there is high-effort and brittle for low
fidelity. Extract any pure logic to a unit test and put the interactive path in
`e2e/specs/`. (This is why `PhotoPicker` and `AlbumDetailsFlow` have E2E
placeholders rather than integration specs.)

**Locators.** Match E2E discipline: **no app-copy literals** — resolve accessible
names by key through `t` from `@test/i18n` (rewording keeps specs green; renaming
a key fails them). Only strings the spec owns (route sentinels, fixtures) are
literals. `getByRole`/`getByLabelText` names match as substrings — pass
`{ exact: true }` when a shorter name also matches a longer one ("Confirm" vs
"Confirm action", "New password" vs "Confirm new password"). There is **no
`getByDisplayValue`** in the browser locator API; target inputs by their label.

Gotchas:

- **Import cross-feature test infrastructure by its narrow module path, never the
  feature barrel.** `@test/render` imports `AuthProvider` from
  `@/features/auth/state/AuthProvider`, not `@/features/auth`. The barrel eagerly
  evaluates the whole feature graph (webauthn, MFA, gates), pulling a second
  React / react-query instance into the pre-bundle; hooks then read a null
  context and throw `Cannot read properties of null (reading 'useContext')`.
- **One unresolved import anywhere under `src/` fails the whole dependency scan**,
  so no integration test runs. While migrating, park a not-yet-rewritten file out
  of the glob rather than leaving a broken import in range.
- **A `let` mutated only inside a closure narrows to `never` under optional
  chaining.** A module-level `let probe = null` assigned inside a `vi.mock`
  factory, then reset with `probe = null` in the test body, makes TS treat
  `probe?.prop` as access on `never`. Use a container object (`const probe =
  { current: null }`) and reset it in `beforeEach`, not inline.
