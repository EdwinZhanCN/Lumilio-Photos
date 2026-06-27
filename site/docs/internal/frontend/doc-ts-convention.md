---
title: doc.ts convention
search: false
---

# `doc.ts` convention

One `doc.ts` per feature, holding that feature's architecture prose — the
TypeScript take on Go's `doc.go`. Kept honest by [`docts`](https://github.com/EdwinZhanCN/docts)
(`3rd-party/docts`, wired into `vp check` and render).

```
web/src/features/<feature>/doc.ts    # source (the comment is the doc)
web/src/features/<feature>/doc.md    # rendered, committed, reads on GitHub
```

One file per feature — do not split. A signature page's component tree goes
inline as a ` ```mermaid ` diagram in the same `doc.ts`; the prose stays next to
the code it describes.

## Rules

1. **Marker** — end the comment with the TSDoc `@module` tag.
2. **Links** — every load-bearing symbol is `{@link X}` in the prose **and**
   `import type { X }` at the bottom. `tsc` guards that `X` exists;
   `docts/link-needs-import` guards that every `{@link X}` has an import. Drift
   fails `make web-test`.
3. **Verbatim prose, mermaid allowed** — render passes the body through as-is
   (lists, tables, fenced code), rewriting only real `{@link}` tags. Write
   ` ```mermaid ` blocks directly.

## Skeleton

Five sections, fixed order. The first two are present in almost every feature;
the rest are added when they apply.

```ts
/**
 * # <Feature>
 *
 * One-line responsibility: what this feature owns, which routes, where the
 * boundary is.
 *
 * ## State
 *
 * {@link XProvider} holds local UI state; components read it via {@link useX}.
 * Server state lives in TanStack Query hooks.
 *
 * ## Data
 *
 * Consumes `/api/v1/...` ({@link SomeDTO}) via {@link useY}. Note here any range
 * that the standard DTO can't express and uses its own endpoint.
 *
 * ## Composition
 *
 * Signature page's component tree. One line if trivial; an inline
 * ```mermaid ``` diagram if not.
 *
 * ## Decisions
 *
 * Non-obvious choices and why, stated in one line each.
 *
 * @module
 */
import type { XProvider, useX } from "./XProvider.tsx";
import type { useY } from "./hooks/useY.ts";
import type { SomeDTO } from "@/lib/...";
export {};
```

| Section | Required | Holds | Excludes |
| --- | --- | --- | --- |
| Title + responsibility | yes | boundary, owned routes, split vs. neighbours | implementation detail |
| `## State` | if stateful | Provider/reducer/store, key hooks (`{@link}`) | every field — only load-bearing ones |
| `## Data` | if it fetches | endpoints/DTOs (API seam), Query hooks | field-level schema (that's OpenAPI) |
| `## Composition` | if non-trivial | signature page tree (inline mermaid) | call sequence (it churns; document structure, not order) |
| `## Decisions` | if any | non-obvious choices + why, one line each | full rationale essays |

## Maintenance

- Rename/delete a symbol → `tsc` red. Add a `{@link}` without the import →
  `docts/link-needs-import` red. Both ride `make web-test`; no new CI.
- `doc.md` is a render artifact — regenerated from `doc.ts`, committed with the
  source (same as `make dto`, diff guards drift). Never hand-edit it.
- `**/doc.ts` imports are doc-only: `tsc` counts a `{@link}` as a use, the
  linter's `no-unused-vars` does not — already disabled for `**/doc.ts` in
  `web/vite.config.ts`.

## Scope

Document slow-moving structure — boundary, state ownership, data seams,
signature composition. Skip call sequences and churny detail: they move fastest
and no checker guards them, so they rot. One sentence beats a diagram; anchor
every load-bearing name with `{@link}` so drift fails the build.

Reference implementation: `web/src/features/collections/doc.ts`.
