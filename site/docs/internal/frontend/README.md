---
title: Frontend Architecture Docs
search: false
---

# Frontend architecture docs (internal)

Engineering docs for humans and agents — not public product docs (those live in
`site/docs/{en,zh-cn}`), not agent-harness ops (those live in `internal/agent/`).

The architecture itself is documented **next to the code**, one `doc.ts` per
feature (`web/src/features/<feature>/doc.ts`), rendered to a sibling `doc.md`.
That keeps the prose honest: [`docts`](https://github.com/EdwinZhanCN/docts)
fails the build when a `{@link}` drifts from the code it names.

- [`doc-ts-convention.md`](./doc-ts-convention.md) — how to write and maintain a
  feature's `doc.ts` (sections, rules, gates).

> These pages are built by VitePress but kept out of nav/sidebar and out of
> search (`search: false` + path-level exclusion), so they don't surface in the
> public site. They remain deploy-reachable (unlinked).
