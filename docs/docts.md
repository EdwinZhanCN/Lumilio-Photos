# docts — Architecture Docs in `doc.ts`

`docts` keeps a feature's architecture prose honest by making it fail the build
when it drifts — the TypeScript answer to Go's `doc.go`. Package:
[`@edwinzhancn/docts`](https://github.com/EdwinZhanCN/docts). Authoring
procedure: [lumilio-feature-doc](../.agents/skills/lumilio-feature-doc/SKILL.md).

## The convention

A feature documents itself with a `doc.ts` at its root. Canonical example:
`web/src/features/collections/doc.ts`.

- Every feature uses the same reader structure: `# Feature`, an unheaded
  ownership paragraph, then exactly `## State`, `## Flows`, and `## Data` in
  that order. Do not add alternate second-level sections.
- `## Flows` contains exactly one Mermaid diagram of ownership-level nodes, not
  a component inventory.
- The whole file is one `/** … @module */` comment plus the `import type`s that
  back it, ending in `export {}`. It exports nothing.
- The comment body is plain markdown. Reference real code with `{@link Symbol}`.
- The sibling `doc.md` is generated. Never hand-edit it.

## Why the imports matter

The `import type` is not decoration; it is the anti-drift mechanism:

- **tsc** — rename or delete a linked symbol and its `import type` fails the
  typecheck (`TS2305`). Symbol existence is tsc's job.
- **`docts/link-needs-import` (oxlint rule)** — a `{@link X}` with no backing
  `import` of `X` fails the lint pass. This is the half tsc can't see (tsc
  ignores `{@link}` entirely).

Together, the prose can never name a symbol that doesn't exist or silently rot.

Diagrams get the same guarantee. Mermaid node labels are plain strings that
neither tsc nor the oxlint rule reads, so `web/scripts/atlas-facts.ts` (run
by `task atlas:check`) treats any label that reads like a code identifier —
camelCase, PascalCase with an inner capital, or a `use*` hook — as a claim to
name real code and fails unless the `doc.ts` imports it. Plain-language
labels stay free prose.
`task verify:generated` re-renders every `doc.md` in CI so a forgotten render
cannot land.

## Wiring

`web/vite.config.ts` registers `@edwinzhancn/docts/oxlint` and
`docts/link-needs-import: error`. `doc.ts` turns `no-unused-vars` off because
tsc already counts `{@link}` as a use and the linter does not. `@edwinzhancn/docts` resolves directly from the public npm registry.
