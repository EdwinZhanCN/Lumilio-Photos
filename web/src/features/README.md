# Feature Placement Guide

`web/ARCHITECTURE.md` is the authoritative, boundary-enforced frontend
architecture. This file is the short checklist for work inside
`src/features/`.

## Standard shape

Feature directories use this optional vocabulary only:

```text
src/features/<feature>/
├── api/          # TanStack Query reads, mutations, DTO adapters
├── model/        # React-free domain types, rules, codecs, transformations
├── flows/        # named user journeys with their UI, hooks, local state, tests
├── components/   # UI reused by multiple flows in this feature
├── hooks/        # rare React mechanisms reused across multiple flows
├── modules/      # isolated technical capabilities, not user journeys
├── routes/       # thin router entries delegating to flows
├── state/        # cross-flow/persisted state, migration, hydration, reset
├── utils/        # legacy/general pure helpers; prefer model/ for domain rules
├── docs/         # feature-local supporting notes
├── index.ts      # narrow cross-feature public API, only when needed
├── types.ts      # genuinely feature-wide types, when needed
├── doc.ts        # architecture-document source
└── doc.md        # generated from doc.ts; never edit directly
```

Omit unused directories. Uniformity means identical semantics, not identical
directory counts.

## Choose state by source and lifecycle

| Data | Owner |
| --- | --- |
| Server fact, loading, pagination, mutation | TanStack Query in `api/` |
| Cross-cutting runtime capability | React Context |
| Interaction shared inside one flow | Flow-local Zustand or `useReducer` |
| One component's interaction | Local `useState` / `useReducer` |
| Shareable or restorable page state | URL / React Router |
| Temporary value that must not render | `useRef` |
| Explicitly refresh-safe preference | Versioned persisted store in `state/` |

Never mirror a fetched collection into Context or Zustand. Do not store URL
state in a feature store. Root `state/` is not the default location for local
reducers.

## Ownership rules

- Put workflow UI and orchestration in `flows/<workflow>/` by default.
- Keep a component inside its flow until another flow actually consumes it.
- Put deterministic domain logic in `model/`, not a catch-all `utils/` folder.
- Keep route files thin and move route behavior into an owning flow.
- Use relative imports inside one feature.
- Import another feature through `@/features/<feature>`; only reviewed narrow
  entries such as `@/features/assets/map` and `@/features/assets/picker` may be
  imported directly.
- Keep public `index.ts` files narrow and backed by real external consumers.
- Remove old paths after moves; do not leave compatibility shims.
- Keep tests and feature-specific styles beside the implementation they cover.

## Validation

From the repository root:

```bash
make web-test
make web-browser-test  # workers, WASM, upload lifecycle, bundling/runtime work
```

`make web-test` performs type checking, linting, the source-boundary audit, and
the frontend test suite. When `doc.ts` changes, regenerate its sibling `doc.md`
as described in `site/docs/internal/agent/docts.md`.
