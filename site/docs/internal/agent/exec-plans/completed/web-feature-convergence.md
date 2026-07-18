# Web Feature Structure Convergence

## Goal

Converge the remaining active frontend features on the vocabulary and ownership
rules established by `web/ARCHITECTURE.md` and the Assets refactor. The work
must improve internal ownership without changing routes, public behavior,
server contracts, persisted keys, or the intentional state lifecycles.

## Scope

The first implementation group contains four medium features:

1. `auth`
2. `settings`
3. `upload`
4. `collections`

Smaller features are handled before or beside their first dependent medium
feature: `notifications`, `repositories`, `users`, `cloud`, `monitor`,
`people`, `share`, `home`, and finally the composition-only `manage` feature.

`lumilio` and `studio` remain a separate high-risk follow-up group because they
own persisted chat/editor state and worker-backed capabilities.

## Structural Contract

- Use only the optional standard roots documented in `web/ARCHITECTURE.md`.
- Put server state in `api/` and deterministic domain rules in `model/`.
- Make `flows/<workflow>/` the default owner for workflow UI, orchestration,
  and flow-local React state.
- Keep `components/` only for UI reused by multiple flows in the same feature.
- Keep root `state/` only for state spanning flows, routes, or refreshes.
- Keep route entries thin and preserve URL-owned restorable state.
- Keep every cross-feature import behind the target feature's narrow public
  entry and do not leave compatibility re-export shims at old internal paths.
- Do not create empty directories merely to make small features look larger.

## Implementation Order

1. Stabilize the high-fan-in `notifications` and `repositories` boundaries.
2. Refactor `users` and `auth`; pause at a reviewable validation checkpoint.
3. Refactor `settings` with `cloud` and `monitor`.
4. Refactor `upload`, preserving the cross-route queue/provider and isolated
   process module.
5. Refactor `collections` with `people`, `share`, and `home`.
6. Refactor `manage` after all of the capabilities it composes are stable.
7. Refactor `lumilio` and `studio`, preserving persisted state and worker
   boundaries.
8. Audit public exports, old paths, feature cycles, generated architecture
   docs, and the final working tree.

Each feature or tightly coupled small-feature slice should remain a separate,
reviewable commit rather than one group-wide change.

## Validation

- Run focused characterization tests after moving a covered module.
- Run `pnpm run check:boundaries` during each structural slice.
- Run `make web-test` at every medium-feature checkpoint.
- Run `make web-browser-test` after Upload or any worker/bundle-sensitive
  change.
- Regenerate a feature's `doc.md` from `doc.ts`; never edit generated markdown
  by hand.
- Review the diff for unintended behavior, DOM, styling, public-contract,
  persisted-key, or generated-file changes.

## Completion Evidence

Completion requires all in-scope feature roots to use the standard vocabulary,
all workflow-specific code to have one clear owner, no obsolete paths or
compatibility shims, an acyclic cross-feature graph, current generated feature
docs, and passing full Web and browser gates.

## Result

Completed on 2026-07-18. All 16 frontend features now use the shared structural
vocabulary. Workflow UI and orchestration live under named `flows/`; public
route entries are thin; server state remains in `api/`; deterministic rules and
wire/view models live in `model/`; cross-flow state and technical modules retain
their explicit boundaries. Obsolete and empty feature directories were removed.

Final evidence:

- Type checking and linting passed for 532 source files.
- Source boundaries passed for 459 runtime modules and 1,156 runtime edges with
  zero cycles.
- All 48 test files and 152 tests passed.
- Production build, entry bundle budget, BLAKE3/upload lifecycle browser smoke,
  and Studio worker/WASM packaging passed.
