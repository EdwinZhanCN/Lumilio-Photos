---
name: lumilio-feature-doc
description: Use when adding a web feature or materially changing one —
  writes or updates the feature's doc.ts, keeps {@link} paired with import
  type, and regenerates the sibling doc.md, which is never hand-edited.
---

# Write A Feature doc.ts

Every documented feature carries a `doc.ts` at its root; the sibling `doc.md`
is generated from it. The mechanism and why the imports exist are owned by
[docts.md](../../../site/docs/internal/docts.md). Canonical example:
`web/src/features/collections/doc.ts`. `task verify:generated` re-renders
every `doc.md` in CI and fails on drift.

## Structure (exact)

One `/** ... @module */` comment containing plain markdown, then the backing
`import type`s, ending in `export {}`. No runtime code, no other exports.
Sections, in order and with no alternates:

1. `# <Feature>` then an unheaded ownership paragraph.
2. `## State` — state-selection rationale.
3. `## Flows` — exactly one Mermaid diagram of ownership-level nodes
   (route/workflow composition for UI features; consumers/hooks/backend
   boundary for API-only features). Not a component inventory.
4. `## Data` — API, DTO, invalidation, and public-entry boundaries.

The comment body is plain markdown — headings, lists, tables, mermaid all
pass through verbatim. Reference real code symbols with `{@link Symbol}`.

## The link contract

- Every `{@link X}` needs a matching `import type { X }` (or default import)
  in the same file. No exceptions.
- tsc catches dead symbols (`TS2305`). The `docts/link-needs-import` oxlint
  rule (runs inside `task web:test` via `vp check`) catches links without
  imports. Together, the prose cannot name a symbol that does not exist.
- Import specifiers are relative with the real extension
  (`./api/useAlbums.ts`), never the `@/` alias — `docts` preserves the
  specifier in `doc.md`, where the alias would not resolve on GitHub. A
  cross-feature reference points at that feature's exact relative `index.ts`.
- `doc.ts` imports are documentation-only: tsc counts a `{@link}` as a use,
  the linter's `no-unused-vars` does not, so that rule is off for `doc.ts`
  in `web/vite.config.ts`. Do not "fix" unused imports by deleting them.

## Regenerate doc.md

After editing one feature, from `web/`:

```sh
task web:docs
```

That renders every `src/features/*/doc.ts`. Commit `doc.ts` and the
regenerated `doc.md` together. Never edit `doc.md` directly; a hand edit is
silently overwritten by the next render.

When you add or materially change a feature, update its `doc.ts` in the same
PR. Search for the old path after a move; do not leave a `README.md` or
`docs/` directory under the feature.

The `@edwinzhancn` scope is mapped to GitHub Packages in `web/.npmrc`;
installing needs a `read:packages` token (local: `~/.npmrc`; CI: a secret).
