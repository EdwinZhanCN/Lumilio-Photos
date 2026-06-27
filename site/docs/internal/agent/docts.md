# docts — Architecture Docs in `doc.ts`

`docts` keeps a feature's architecture prose honest by making it fail the build
when it drifts — the TypeScript answer to Go's `doc.go`. Package:
[`@edwinzhancn/docts`](https://github.com/EdwinZhanCN/docts).

## The convention

A feature documents itself with a `doc.ts` at its root. Canonical example:
`web/src/features/collections/doc.ts`.

- The whole file is one `/** … @module */` comment plus the `import type`s that
  back it, ending in `export {}`. It exports nothing.
- The comment body is plain **markdown** — headings, lists, tables, even mermaid
  all pass through verbatim.
- Reference real code symbols with `{@link Symbol}`. **Every linked symbol must
  be `import type`-d in the same file**, with the source path including its
  extension (`./hooks/useAlbums.ts`, `@/components/collection`).

```ts
/**
 * # Collections
 *
 * {@link CollectionsProvider} (read via {@link useCollections}) holds the
 * feature's transient UI state, reduced by {@link collectionsReducer}.
 *
 * @module
 */
import type { CollectionsProvider, useCollections } from "./CollectionsProvider.tsx";
import type { collectionsReducer } from "./collections.reducer.ts";

export {};
```

## Why the imports matter — two guarantees

The `import type` is not decoration; it is the anti-drift mechanism:

- **tsc** — rename or delete a linked symbol and its `import type` fails the
  typecheck (`TS2305`). Symbol existence is tsc's job.
- **`docts/link-needs-import` (oxlint rule)** — a `{@link X}` with no backing
  `import` of `X` fails the lint pass, pointing at the link. This is the half
  tsc can't see (tsc ignores `{@link}` entirely).

Together, the prose can never name a symbol that doesn't exist or silently rot.

## Rules (MUST)

- When you add or materially change a feature, update its `doc.ts`.
- Every `{@link X}` needs a matching `import type { X }` (or default import) in
  the same file. No exceptions — that is the contract.
- Keep the import specifier pointing at the symbol's real source file, with the
  extension, so the rendered link resolves on GitHub.
- `doc.md` is **generated** — never hand-edit it. Change `doc.ts`, then
  re-render (below).
- Do not add other exports or runtime code to `doc.ts`; the imports exist only
  to back links, and `export {}` keeps it side-effect-free.

## How it is wired

`web/vite.config.ts` registers the plugin and rule in the `lint` block:

```ts
lint: {
  jsPlugins: ["@edwinzhancn/docts/oxlint"],
  rules: { "docts/link-needs-import": "error" },
  overrides: [
    // doc.ts imports are documentation-only: tsc counts a {@link} as a use, the
    // linter's no-unused-vars does not, so hand that rule back to tsc on doc.ts.
    {
      files: ["**/doc.ts"],
      rules: {
        "no-unused-vars": "off",
        "@typescript-eslint/no-unused-vars": "off",
      },
    },
  ],
  // …existing options…
}
```

The rule runs as part of `make web-test` / `vp check`. The `@edwinzhancn` scope
is mapped to GitHub Packages in `web/.npmrc`; installing needs a `read:packages`
token (local: your `~/.npmrc`; CI: a secret).

## Rendering `doc.md`

Each `doc.ts` renders to a sibling `doc.md` (committed), with the prose verbatim
and every `{@link Symbol}` turned into a link to the symbol's source file. After
editing a `doc.ts`, regenerate from `web/`:

```bash
node --input-type=module -e '
  import { parseDocFile, renderMarkdown } from "@edwinzhancn/docts";
  import { writeFileSync } from "node:fs";
  const f = process.argv[1];
  writeFileSync(f.replace(/\.ts$/, ".md"), renderMarkdown(parseDocFile(f)));
' src/features/collections/doc.ts
```

(The `@edwinzhancn/docts/vite` plugin can do this on build/dev instead, once
wired into the docs pipeline.)
