# Frontend

This document describes the current React frontend as implemented in `web/`.

## Runtime Entry

- App entry: `web/src/main.tsx`.
- App shell and providers: `web/src/App.tsx`.
- Routes: `web/src/routes/routes.tsx`.
- Vite+ config: `web/vite.config.ts`.
- Container runtime: `web/Dockerfile`, `web/Caddyfile`, `web/scripts/docker-entrypoint.sh`.

The app mounts `I18nProvider`, then `SettingsProvider`, `GlobalProvider`, `QueryClientProvider`, `AuthProvider`, router/bootstrap gates, worker/upload providers, and the shell layout.

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

Equivalent direct commands:

```bash
cd web
vp dev --host --port 6657
vp check --no-fmt --no-lint
vp lint
vp test
```

## Source Layout

- `src/features/*`: domain features and routes.
- `src/components`: reusable app components and UI pieces.
- `src/contexts`: cross-cutting providers.
- `src/lib`: API client, i18n, utilities, feature support libraries.
- `src/lib/http-commons`: generated OpenAPI schema, typed client, React Query integration.
- `src/routes`: route registration.
- `src/styles`: global styles.
- `src/locales`: translation resources.
- `src/wasm`: checked-in generated/bundled WASM support code.
- `src/workers`: browser worker entry points and worker tests.

Current feature areas include assets, auth, collections, home, Lumilio chat, manage/upload, monitor, people, settings, studio, updates, and users.

## API Contract

OpenAPI is the source of truth for HTTP contracts.

- Use `$api` from `src/lib/http-commons/queryClient.ts`.
- Prefer `$api.useQuery`, `$api.useInfiniteQuery`, and `$api.useMutation`.
- Do not hand-edit `src/lib/http-commons/schema.d.ts`.
- Do not create ad-hoc request/response types when an endpoint exists in OpenAPI.

For API changes:

1. Update backend annotations and handler behavior.
2. Run `make dto`.
3. Update frontend hooks/components against generated types.

## State Boundaries

Use TanStack Query for server state:

- fetched backend data
- cache lifecycle
- loading/error state
- pagination and refetch behavior

Use Context for cross-cutting app state:

- settings
- auth session
- global online/notification behavior
- worker runtime dependencies

Use Zustand for feature-local interactive UI state:

- selections
- tabs and panels
- local feature preferences
- URL-synced controls

Do not mirror the same data across Query, Context, and Zustand without a clear ownership reason.

## Routing And Shell

Public routes include login and register. Bootstrap routes handle first-user setup. Protected standalone routes handle MFA and password changes.

Main app routes are rendered inside the shell with `NavBar`, `SideBar`, a scroll container, and footer. Notable route groups include:

- `/`
- `/assets`
- `/collections`
- `/collections/albums`
- `/collections/map`
- `/collections/people`
- `/collections/utilities/duplicates`
- `/people/:personId`
- `/manage`
- `/studio`
- `/server-monitor`
- `/lumilio`

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
- falls back to `index.html` for SPA routes

## Quality Gate

Frontend gate:

```bash
make web-test
```

Equivalent:

```bash
cd web && vp check --no-fmt --no-lint && vp lint && vp test
```

Use `vp build` when changing bundling, Caddy/runtime behavior, WASM loading, workers, or production-only code paths.
