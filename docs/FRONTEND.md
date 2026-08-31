# Frontend

This document describes the current React frontend as implemented in `web/`.
Procedures live in `.agents/skills/`; this file keeps contracts, maps, and
design boundaries.

## Runtime Entry

- App entry: `web/src/main.tsx`.
- Root application composition and providers: `web/src/app/App.tsx`.
- Router gates and route table: `web/src/app/router/AppRouter.tsx`, `web/src/app/router/routes.tsx`.
- Authenticated navigation shell: `web/src/app/shell/AppShellLayout.tsx`.
- Runtime health polling: `web/src/app/status/HealthPoller.tsx`.
- Vite+ config: `web/vite.config.ts`.
- Container runtime: `server/Dockerfile` builds the SPA and embeds it in the
  single Lumilio image.

The app mounts `I18nProvider`, then `PreferencesEffects`, `GlobalProvider`, `QueryClientProvider`, `AuthProvider`, router/bootstrap gates, worker/upload providers, and the shell layout.

## Toolchain

The frontend uses Vite+ as the command surface.

Core stack: React 19, TypeScript, React Router 7, TanStack Query 5, Zustand 5
with immer, Tailwind CSS 4 and DaisyUI 5, Vitest 4 through Vite+, Web Workers
and WASM modules for compute-heavy paths.

Daily commands: `task web:dev`, `task web:test`. Direct `vp` commands from
`web/` are acceptable when intentionally scoped to that workspace. Tooling
notes: [vite-plus.md](vite-plus.md).

Every user-facing string goes through the i18n layer. Translation JSON is
never hand-edited beyond filling values the extractor created. Procedure:
[lumilio-frontend-i18n](../../../.agents/skills/lumilio-frontend-i18n/SKILL.md).

## Source Layout

- `src/app`: root providers, router composition, application shell, and runtime status effects.
- `src/features/*`: domain features. The enforced shape and dependency rules
  live in `web/ARCHITECTURE.md`.
- `src/components`: reusable app components and UI pieces.
- `src/contexts`: cross-cutting providers.
- `src/lib`: API client, i18n, utilities, feature support libraries.
- `src/lib/http-commons`: generated OpenAPI schema, typed client, React Query integration.
- `src/styles`: global styles.
- `src/locales`: translation resources.
- `src/wasm`: checked-in generated/bundled WASM support code.
- `src/workers`: browser worker entry points and worker tests.

Current feature areas are assets, auth, cloud, collections, events, home,
Lumilio, manage, monitor, notifications, people, repositories, settings, share,
studio, upload, and users.

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
- `doc.ts`: the feature architecture source; generated `doc.md` stays beside it
  at the feature root. Authoring:
  [lumilio-feature-doc](../../../.agents/skills/lumilio-feature-doc/SKILL.md).

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
- An `as` cast on an API response is a red flag, never a fix.

The checked-in fetch/query runtime comes from the official `openapi-fetch`,
`openapi-react-query`, and `openapi-typescript-helpers` packages. Regeneration,
cast triage, and the swag empty-object quirk:
[lumilio-api-contract-change](../../../.agents/skills/lumilio-api-contract-change/SKILL.md).

All request failures remain structured until presentation.
`src/lib/http-commons/problem.ts` consumes the generated RFC 9457 Problem and
Problem Reference unions, validates untrusted runtime payloads, and distinguishes
registered Problems from unknown/malformed responses, network failures, and
browser aborts. Manual fetch, XHR, binary-download, and SSE adapters throw or
return that shared structure; they never build an `Error.message` from response
text or parse a private `{message,error}` shape.

The rendering boundary calls `localizeProblem`/`localizeProblemReference` (or
their combined adapter) with an already-localized operation fallback and the
current `t` function. Known actionable types may override the fallback through
literal keys below `apiErrors`; `about:blank`, future types, malformed bodies,
and network states cannot display or stringify wire content. Business behavior
branches only on HTTP status or the exact Problem `type`, never translated copy.
Because localization happens at render/recovery time, repeating a failure after
a runtime language change uses the new language without changing the Server
response or persisted operation state.

The type mapping is exhaustive against the generated union. Architecture checks
also require every registered URI to have a literal switch case and non-empty
English and Simplified Chinese catalog values, and reject dynamic URI-derived
translation keys or feature-private Problem parsers.

## State Boundaries

Use TanStack Query for server state: fetched backend data, cache lifecycle,
loading/error state, pagination and refetch behavior.

Events follow this boundary directly: the index uses opaque server cursors,
detail and mutation state remain in TanStack Query, and the detail gallery
composes the public Assets entry with an immutable `event_id` constraint.
Assets exposes feature-neutral logical selection values to Event correction
actions; Events never imports Assets selection internals.

Use Context for cross-cutting runtime capabilities: auth session, global
runtime/notification coordination, worker dependencies.

Use flow-local Zustand or `useReducer` for interaction shared by several
components in one workflow. Use component-local state for one component, URL
state for linkable/restorable page state, `useRef` for non-rendering temporary
values, and versioned storage only for explicitly refresh-safe preferences.

Session teardown is centralized in `features/auth/state/resetSession.ts`. Logout and
refresh exhaustion must use that boundary so in-flight Query/Lumilio work and
all user-scoped caches, notifications, repository choices, searches, and
filters are cleared before another user authenticates.

The browser stores only the short-lived access token and a browser-readable,
session-bound CSRF proof that is not independently authenticating but must not
leak cross-origin. The refresh credential is a host-only `HttpOnly` cookie and
must never be added to response DTOs or browser storage.
`http-commons/client.ts` includes credentials, recovers CSRF through the safe
session endpoint, and serializes refresh/logout with the
`lumilio-auth-refresh` Web Lock so multiple tabs cannot replay a rotated
credential. Unsafe typed-client requests carry `X-CSRF-Token`; refresh and
logout must use the dedicated session helpers.

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
- Entity detail: album, Event, folder, tag, person, and asset routes with optional asset-viewer segments.
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
  queries `/assets/map-points` with its current WGS-84 viewport. The Collections
  Places rail drains location-cluster pages to produce complete city summaries,
  but it never drains map points.
- `web/scripts/check-bundle-budget.ts` enforces a 420 KiB gzip budget for the
  production entry chunk as part of `vp run test:bundle`.

## Z-Index

Decorative overlays use DOM order; component-internal overlap uses
`isolation: isolate`; cross-component floating layers use the token scale in
`App.css`. Application procedure:
[lumilio-z-index](../../../.agents/skills/lumilio-z-index/SKILL.md).

## Test layers

Pick the layer by what the test must exercise; the file name and directory pick
the runner (`web/vite.config.ts` `test.projects`). Do not invent other
conventions. Placement, GPU self-skip, and proving a guard can fail:
[lumilio-write-a-test](../../../.agents/skills/lumilio-write-a-test/SKILL.md).

| Layer | File | Runner / Vitest project | Answers |
| --- | --- | --- | --- |
| Unit | `*.test.ts` | `unit` — Node, no DOM | React-free rules, transforms, codecs, validators, reducers, state migrations, algorithms |
| Component | `*.test.tsx` | `integration` — Browser Mode (real Chromium) | one component or small tree: accessible semantics, state, interaction |
| Flow Integration | `*.spec.tsx` | `integration` — Browser Mode + MSW | flows, routes, multi-component, Router, Query, HTTP workflows |
| Browser Capability | `*.browser.test.ts` | `browser` — Chromium | Worker, WASM, SSE, Blob, Canvas/WebGL — real browser capabilities |
| Full E2E | `web/e2e/specs/*.spec.ts` | Playwright + real services | key user paths on real API, DB, storage, queues |

The `unit` project excludes `*.browser.test.ts` and `src/workers/**` so an
accidental browser dependency fails instead of hiding; `integration` and
`browser` run real Chromium via the Playwright provider. Core-browsing UI is
assigned to Playwright by the
[test-layer assignment decision](../../../.agents/decisions/2026-08-14-frontend-test-layer-assignment.md).

Flow specs: [lumilio-integration-spec](../../../.agents/skills/lumilio-integration-spec/SKILL.md).
Playwright specs: [lumilio-e2e-spec](../../../.agents/skills/lumilio-e2e-spec/SKILL.md).
E2E stack: [lumilio-e2e-environment](../../../.agents/skills/lumilio-e2e-environment/SKILL.md).

## Quality Gate

```bash
task web:test
```

That is typecheck, lint, source-boundary check, and the Vitest
unit/integration/browser projects. Playwright slices are separate Task
targets and need Docker; CI selects them through path filters. Map a diff to
the narrowest evidence with
[lumilio-select-checks](../../../.agents/skills/lumilio-select-checks/SKILL.md).
