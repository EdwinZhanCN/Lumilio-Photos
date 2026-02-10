# AGENTS.md (web)

This file defines the frontend engineering contract for `web/`.
All contributors and coding agents should follow these rules.

## 1. Source Of Truth: API Contract First

- Backend OpenAPI spec is the single source of truth for HTTP contracts.
- Do not hand-edit `web/src/lib/http-commons/schema.d.ts`.
- Do not introduce ad-hoc HTTP request shapes when an OpenAPI endpoint exists.

### Required workflow for API changes

1. Add/update backend handler swagger annotations.
2. From repo root, run:
   - `make dto`
3. Verify changed files include:
   - `server/docs/swagger.yaml`
   - `server/docs/swagger.json`
   - `server/docs/docs.go`
   - `web/src/lib/http-commons/schema.d.ts`
4. Update frontend calls to use generated types.

## 2. HTTP Client Rules (Type Safety)

- Prefer `web/src/lib/http-commons/queryClient.ts` (`$api`) for all API calls.
- Prefer `$api.useQuery`, `$api.useInfiniteQuery`, `$api.useMutation` over raw `fetch`.
- Use route/path types generated from OpenAPI; avoid stringly typed payloads.

### Allowed exception

- Temporary `fetch` is allowed only if endpoint is not yet in OpenAPI.
- In that case, add a TODO to remove after `make dto` and migrate to `$api`.

## 3. State Management Boundaries (React Query + Context + Zustand)

This project intentionally uses mixed state management. Keep boundaries strict.

### React Query (`@tanstack/react-query`)

Use for **server state**:
- Data fetched from backend
- Caching / refetch / retry / pagination
- Request lifecycle status (`isLoading`, `isError`, etc.)

Do not duplicate query payloads into Zustand or Context unless there is a clear reason.

### React Context

Use for **cross-cutting app state**:
- App-level settings (`SettingsProvider`)
- Auth session context (`AuthProvider`)
- Global notifications/online status (`GlobalContext`)
- Worker runtime dependencies (`WorkerProvider`)

Context should not become a general entity store.

### Zustand

Use for **feature-local interactive UI state**:
- Selection state
- Panel/tab UI state
- URL-synced feature controls
- Local persistent UI preferences for a feature

Current canonical example: assets domain store at `web/src/features/assets/assets.store.ts`.

### Anti-patterns

- Same piece of state mirrored in Query + Context + Zustand without strict ownership.
- Persisting server entities in localStorage unless explicitly required.
- Writing business API cache logic manually when React Query already provides it.

## 4. Frontend Stack Baseline

- React 19 + TypeScript 5
- Vite 7
- React Router 7
- TanStack Query 5
- Zustand 5 + immer
- Tailwind CSS 4 + DaisyUI 5
- Vitest 4
- WASM + Web Workers for heavy compute paths

## 5. Important Dependencies and Usage Guidance

- `@tanstack/react-query`: server state and API lifecycle
- `zustand` + `immer`: feature UI store slices
- `react-router-dom`: route state, params, URL-driven UX
- `i18next` + `react-i18next`: all user-facing copy should be i18n-ready
- `lucide-react` / `@heroicons/react`: icons (avoid mixing many icon systems in one component)
- `@immich/justified-layout-wasm`, local wasm modules: compute-intensive image workflows

## 6. File and Module Conventions

- Use path alias imports via `@/...`.
- Keep code feature-oriented under `web/src/features/*`.
- Keep reusable infrastructure under:
  - `web/src/lib/*` (HTTP, utilities, i18n)
  - `web/src/contexts/*`
  - `web/src/components/*`
- Do not place domain logic inside generic UI components.

## 7. Quality Gate Before Merge

From `web/` run:

1. `pnpm type-check`
2. `pnpm lint`
3. `pnpm test` (when behavior changes or bug fix touches logic)

If backend contract changed, also ensure `make dto` has been run from repo root.

## 8. Practical Patterns

- New endpoint consumption pattern:
  1. Ensure endpoint exists in OpenAPI
  2. Run `make dto`
  3. Add hook using `$api.useQuery`
  4. Map response to UI-friendly shape in hook layer
- Keep pages thin; move data fetching and mapping into feature hooks.
- Prefer deterministic behavior for home/recommendation output when UX needs reproducibility (e.g. seed-based featured sets).

## 9. Do Not

- Do not manually modify generated OpenAPI artifacts.
- Do not bypass `$api` with handcrafted request/response types for existing endpoints.
- Do not introduce new global state containers when existing ones satisfy ownership.
