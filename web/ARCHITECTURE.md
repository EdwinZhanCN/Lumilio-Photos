# Web Architecture

This document defines where frontend code belongs and which imports are allowed. Keep ownership close to the domain, keep public surfaces small, and preserve an acyclic dependency graph.

## Source ownership

| Path                      | Responsibility                                                                                                                                                                                                                            |
| ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/app/`                | Application composition only: root providers, routing, shell, fatal-error handling, and process-wide effects. Domain behavior does not belong here.                                                                                       |
| `src/features/<feature>/` | One product/domain capability. A feature owns its routes, flows, model rules, API adapters, state lifecycles, tests, and feature-specific styles.                                                                                         |
| `src/components/`         | Reusable presentational UI with no feature/API/store dependency. Components accept plain data and callbacks; semantic families such as `collection` are allowed when multiple domains use them. Do not create a generic `shared/` bucket. |
| `src/hooks/`              | Generic React mechanisms used across domains. Workflow-specific hooks belong in their feature; a small stable query/type contract shared by several features may live under a named `lib/<concept>` namespace.                            |
| `src/lib/`                | Non-visual lower-layer code: HTTP/runtime adapters, algorithms, formatting, preferences, and stable contracts shared across features (for example `albums`, `assets`, or `upload`). It must not import or orchestrate a feature.          |
| `src/contexts/`           | Truly application-wide infrastructure such as global notifications or worker access. Feature-scoped contexts stay inside their feature.                                                                                                   |
| `src/workers/`            | Browser worker entry points and the shared worker client. Feature-owned algorithms stay with their feature; a `.worker.*` entry point may register a worker-safe feature runner.                                                          |
| `src/types/`              | Ambient and third-party declaration files. Domain types stay with their feature; reusable value types stay beside their implementation in `lib`.                                                                                          |
| `src/styles/`             | Global stylesheet entry points and genuinely global rules. Component- or feature-specific CSS is colocated with its owner.                                                                                                                |

## Standard feature shape

Every feature uses the same root vocabulary. Directories are optional—do not create empty placeholders—but a different root directory name is not allowed:

```text
src/features/<feature>/
├── api/          # TanStack Query definitions, mutations and DTO adapters
├── model/        # pure domain types, rules, codecs and transformations
├── flows/        # user workflows; each flow colocates its UI, hooks and local state
├── components/   # UI genuinely reused by multiple flows in this feature
├── docs/         # feature-local supporting notes; doc.ts/doc.md remain at the root
├── hooks/        # rare feature-wide React mechanisms shared across flows
├── modules/      # technically isolated capabilities that are not user workflows
├── routes/       # router entry components
├── state/        # feature-wide state, persistence, migration and reset only
├── utils/        # legacy/general pure helpers with no domain vocabulary
├── types.ts      # feature-wide types
├── index.ts      # narrow cross-feature public API, only when needed
├── doc.ts        # architecture source, when documented
└── doc.md        # generated from doc.ts
```

- Root files other than `index.ts`, `types.ts`, `doc.ts`, and generated `doc.md` are not allowed.
- `api/` owns reusable server-state access and must not contain page composition or modal state.
- `model/` is React-free. Put domain names, normalization, validation, URL codecs, grouping, sorting, and other deterministic rules here rather than growing a catch-all `utils/` directory.
- `flows/<workflow>/` is the default home for workflow UI and orchestration. Colocate its components, hooks, reducers, scoped Zustand stores, tests, and styles; nested names such as `gallery/`, `selection/`, or `header/` describe parts of that workflow.
- `components/` is not a second UI dumping ground. A component used by only one flow stays in that flow; root components must have real consumers in multiple flows.
- `state/` owns only state whose lifecycle spans multiple flows or refreshes: persistence, migration, hydration, reset, or a genuinely feature-wide provider. A reducer or scoped store used by one flow stays with that flow.
- `modules/<capability>/` is for an isolated technical/product capability that is not itself a user journey. Name it after product vocabulary (`editor`, `widgets`, `process`), never `common`, `misc`, or `shared`.
- Tests and feature-specific styles stay beside the implementation they characterize.
- A small feature omits unused directories. Uniformity means identical directory semantics, not identical directory counts.
- The Assets feature expresses its main journeys as `flows/browse`, `flows/viewer`, and `flows/export`; pure filtering and browse-item rules live in `model`. It keeps `map/` and `picker/` as reviewed public sub-entry exceptions whose entry files remain narrow.

## Dependency rules

The normal direction is:

```text
main.tsx -> app -> features -> components / contexts / hooks / lib / workers / types
                         \-> another feature's public API (acyclic only)
productionSmoke.ts -> feature public APIs / lib / workers (browser gate only)
```

- Lower layers (`components`, `config`, `contexts`, `hooks`, `lib`, `types`, and shared worker code) never import features.
- Nothing imports `app` except `main.tsx` and modules already inside `app`. `app` is the composition root and may import route implementations directly; this does not make those paths public to other features. `productionSmoke.ts` is a separate browser-gate entry point and composes only feature public APIs and lower layers.
- Inside one feature, use relative imports. Do not import `@/features/<same-feature>/...` and do not route internal code through the feature's root public barrel. A cohesive submodule may use its own local `index.ts` through a relative path. Use the `@/...` alias when an import leaves the feature.
- Between features, import `@/features/<feature>` unless an approved narrow entry applies. The target feature's `index.ts` is its explicit public contract.
- Keep `index.ts` narrow. Export only symbols with real cross-feature consumers; do not expose internals merely to shorten a path.
- The approved narrow entries are `@/features/assets/map` and `@/features/assets/picker`. `map` exposes the asset-backed geospatial surface; `picker` exposes an isolated single-selection asset flow. These aliases are for consumers outside Assets; Assets internals still use relative paths. A new narrow entry requires a concrete boundary and a corresponding boundary-check update.
- Cross-feature runtime dependencies must remain acyclic. If two features need each other, move the neutral contract downward or invert the orchestration into `app` or a higher owning feature.
- A feature `doc.ts` may use deep, type-only imports so `{@link}` references resolve to real symbols. This documentation-only exception is not a runtime API.

Examples:

```ts
// Inside features/assets: relative to the owning flow.
import { useAssetSelection } from "./selection/useAssetSelection";

// Across features: use a public entry.
import { useMessage } from "@/features/notifications";

// Approved purpose-built entry.
import PhotoPicker from "@/features/assets/picker";
```

## State, data, and utility ownership

Choose state by source and lifecycle:

```mermaid
flowchart TD
    A["One piece of data"] --> B{"Source and lifecycle"}
    B -->|"Server fact"| C["TanStack Query"]
    B -->|"Cross-cutting runtime capability"| D["React Context"]
    B -->|"Shared by components in one feature"| E["Zustand / useReducer"]
    B -->|"Single-component interaction"| F["useState / useReducer"]
    B -->|"Shareable or restorable page state"| G["URL / React Router"]
    B -->|"Temporary reference without rendering"| H["useRef"]
    B -->|"Allowed to survive refresh"| I["localStorage / persisted store"]
```

| Source of truth                      | Owner                               | Placement and rule                                                                                                          |
| ------------------------------------ | ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Server fact                          | TanStack Query                      | Put reusable query/mutation definitions in the feature `api/`. Never copy fetched collections into Context or Zustand.      |
| Cross-cutting runtime capability     | React Context                       | App-wide capabilities live in `src/contexts`; feature-scoped providers and contexts live in that feature's `state/`.        |
| Shared workflow interaction          | Zustand or `useReducer`             | Colocate it under `flows/<flow>/`; use root `state/` only when the lifecycle genuinely spans flows or refreshes.            |
| Single component interaction         | `useState` or local `useReducer`    | Keep it beside the rendering component; do not promote it without multiple consumers.                                       |
| Shareable/restorable page state      | URL / React Router                  | The URL remains authoritative for search, tab, filter, entity, or view state that should survive navigation or be linkable. |
| Non-rendering temporary value        | `useRef`                            | Use for DOM handles, latest callbacks, request identity, and mutable bookkeeping that must not render.                      |
| Refresh-persistent client preference | Versioned storage / persisted store | Persistence code, migration, hydration, and reset live in `state/`; keys and versions use the shared settings registry.     |

- Keep ephemeral state in the component that renders it. Lift it only when multiple siblings need the same interaction state.
- Use TanStack Query for server data, pagination, loading/error state, cache invalidation, and mutations. Do not mirror fetched collections into Context or Zustand.
- Use a scoped Zustand store for transient interaction shared by several components in one flow, such as browse selection. Do not store applied filters, search, sorting, or the current route entity there when those values belong in the URL.
- Keep an unapplied form/filter draft in a local reducer; committing it writes to its authoritative owner (for example the URL or a mutation).
- Use Context for dependency/provider boundaries and truly cross-cutting runtime state. A domain-specific provider belongs under its feature, not `src/contexts`.
- Put workflow orchestration and feature-specific hooks in the owning feature. A small stable query/type contract used by several features may live under a named `lib/<concept>` namespace to avoid coupling those features. Use generated OpenAPI types through `lib/http-commons`; never hand-edit `src/lib/http-commons/schema.d.ts`.
- Keep workflow-specific types, formatters, validators, and adapters in the feature. Move code to `lib` only when it is a stable lower contract and does not depend on feature UI, state, or orchestration.
- A component that performs asset, album, repository, people, upload, or other product behavior is a feature component. A semantic presentational family may live in `components` when it has plain props and no feature dependency.
- Keep worker transport and scheduling in `workers`/`lib/workers`; keep the operation itself with the feature or neutral library that owns its semantics. A worker entry may register only the feature runner imports explicitly allowlisted in the boundary checker.

## Validation

Run focused tests while editing, then the repository gates from the project root:

```bash
cd web
vp test run path/to/changed.test.ts
pnpm run check:boundaries
cd ..
make web-test
```

If no direct test covers the change, run the nearest characterization tests plus typecheck, lint, and `pnpm run check:boundaries`; add coverage when behavior is being changed or moved without it. The full gate remains required.

`make web-test` runs TypeScript checking, linting, the source-boundary checker, and the frontend test suite. The standalone `pnpm run check:boundaries` command gives faster architectural feedback while editing. The checker rejects unresolved internal imports, non-standard feature roots, misplaced shared-state or persistence modules, reusable server queries under `hooks/`, same-feature aliases, cross-feature deep imports, reverse dependencies on `app`, lower-layer imports of features, unapproved worker registrations, runtime import cycles, and feature dependency cycles. Tests/specs participate in ownership and public-entry checks but stay out of the production cycle graph. `doc.ts`, WASM, and the generated schema are intentionally excluded from the runtime graph; `doc.ts` links are checked by the documentation lint rule instead.

Also run `make web-browser-test` after changes to workers, WASM, upload recovery/lifecycle, bundling, or other production-only browser paths. It builds the production app, enforces the bundle budget, and runs the browser smoke suite.

## Placement decision

Before adding a file, ask in order:

1. Does it implement one feature's workflow, UI, state, or API orchestration? Put it in that feature.
2. Is it a small non-visual query/type contract shared by several features with no feature dependency? Put it in a named `lib/<concept>` namespace.
3. Is it presentational UI reusable across unrelated features through plain props? Put it in `components`.
4. Is it a generic React mechanism with no workflow ownership and multiple consumers? Put it in `hooks`.
5. Is it non-visual feature-neutral infrastructure or an algorithm? Put it beside the closest existing concept in `lib`.
6. Is it only application composition or process-wide wiring? Put it in `app` or, for a true global provider, `contexts`.

Do not create a new feature for a single generic helper. Do create one when a capability has its own vocabulary and at least one of: a route, workflow/API boundary, state lifecycle, or reusable domain UI.

## Migration checklist

- Identify one owner before moving code; split orchestration from reusable domain pieces when ownership differs.
- Move tests and feature-specific styles with their implementation.
- Convert same-feature imports to relative paths.
- Route every cross-feature consumer through the target public `index.ts` or an approved narrow entry.
- Remove the old path; do not leave compatibility re-export shims.
- Check that lower layers do not import a feature and that no runtime or feature cycle was introduced.
- Search for the old path and update real `doc.ts` references; never hand-edit generated `doc.md` files.
- Run focused tests, the source-boundary checker, `make web-test`, and `make web-browser-test` when the browser-runtime criteria above apply.
- Review the final diff for accidental behavior, DOM, styling, generated-file, or public-contract changes.
