# Decision: the Atlas is the code map, and every diagram is derived or anchored

Status: implemented, 2026-10-01 (branch `feat/atlas`). `task atlas:check` runs
in `task test` and in `task verify:generated` (CI `generated` job).

## Problem

The codebase passed 330k lines (Server ~210k Go, Web ~106k TS, plus Desktop)
and its owner could no longer hold a mental model of it. Agent tasks were
written in vaguer terms as a result, which pushed coding agents toward local
optima. The existing maps had already drifted: `docs/architecture.md` and
`docs/BACKEND.md` still described the Repository Observation Engine, its
`repository_nodes`/`asset_locations` tables, and `storage/roe` after #222
deleted all three; `architecture.md` said the manifest was schema v5 while
`config.SchemaVersion` is 1; `storage/doc.go` listed the removed `roe`
subpackage. Only 1 of ~75 server packages had a `doc.go`, and Mermaid nodes in
feature `doc.ts` files were unchecked strings, so a renamed hook survived in
the diagram.

## Decision

One map with three layers, each with a mechanical guarantee
([docs/atlas/README.md](../../docs/atlas/README.md)):

1. **Derived maps** (group maps, per-group package maps, module catalog,
   imports/importers) are generated from the Go and Web import graphs plus
   `docs/atlas/atlas.yaml`. Large graphs are drawn as a transitive reduction of
   the declared group DAG; the full edge list sits beside each map.
2. **Package docs**: every Go package in `server/` and `desktop/` (minus
   tooling and generated code) has exactly one package comment, in `doc.go`,
   ending with `//atlas:group <id>`; Go doc links must resolve (our analyzer,
   since the toolchain silently renders unresolved links as text). Web
   features keep `doc.ts` via docts, and code-like Mermaid node labels must be
   imported.
3. **Anchored views** for architecture, sequence, data flow, and lifecycle are
   YAML whose every element anchors to `go:`, `ts:`, `api:`, `sql:`, `mod:`,
   `group:`, `file:`, or `ext:`. `go:`/`ts:` anchors are hashed into
   `atlas.lock.json`; a changed hash marks the view stale and fails CI until
   someone re-reads it and runs `task atlas:lock`. Lifecycle views may declare
   `enum:` and must then draw exactly that enum's members.

`atlas.yaml` declares group dependencies, compared with real imports in both
directions and required to be acyclic. Generated markdown is checked in so
agents and GitHub can read every view without running anything; the
interactive site is built locally by `task atlas` and not checked in. The tool
is a Go program (`server/tools/atlas`) plus a Web extractor
(`web/scripts/atlas-facts.ts`); Mermaid renders in the browser from the pinned
`mermaid` devDependency.

The owner authors the views and `atlas.yaml`; agents draft package docs and
views, and must re-verify stale views rather than lock blindly. Dead utility
packages found while documenting (`internal/utils` root, `utils/errgroup`) were
deleted rather than documented.

## Alternatives considered

- **Fully generated diagrams only** (call graphs, dependency-cruiser, go-callvis
  dumps): zero drift, but no intent. A 130-module call graph is unreadable and
  cannot say *why* a flow is shaped the way it is. Kept only for the
  structural layer, where it is exact.
- **Hand-written diagrams in Markdown/Mermaid** (the previous state): carries
  intent but drifted within weeks, as the ROE descriptions proved. Rejected as
  the source of truth; the authored layer must be anchored.
- **LLM-generated docs and diagrams on demand**: cheap to produce, but output
  is plausible rather than verified and changes run to run, so nobody can
  review a diff of it. LLMs may draft views; the anchors and lock are what make
  them trustworthy.
- **Hard-fail on every anchored-code change vs. warn**: warning would let
  stale views accumulate silently, which is the original problem. The gate
  fails; the cost is one re-read and `task atlas:lock` per touched view, and
  the lock diff tells reviewers which views were re-verified.
- **C4/Structurizr DSL or D2 as the authoring format**: richer layout, but a
  second toolchain and no symbol anchoring. YAML with our own anchors renders
  to Mermaid, which GitHub, VitePress, and the site all display.
- **Declaring each package's dependencies in its own `doc.go`**: duplicates
  derivable facts and drifts. Packages declare only their group; rules live
  once, per group, in `atlas.yaml`.
