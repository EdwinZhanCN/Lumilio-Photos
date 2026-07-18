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

## Quality Gate

Frontend gate:

```bash
make web-test
make web-browser-test
```

Direct equivalent when intentionally scoped to `web/`:

```bash
cd web && vp check --no-fmt --no-lint && vp lint && vp test
```

`web-browser-test` is the real-browser worker/release smoke job for cross-origin
isolation, BLAKE3, upload recovery, and background lifecycle transitions. Use
`vp build` when changing bundling, Caddy/runtime behavior, WASM loading,
workers, or production-only code paths.
