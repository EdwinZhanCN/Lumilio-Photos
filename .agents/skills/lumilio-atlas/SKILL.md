---
name: lumilio-atlas
description: Use when adding or moving a Go package, adding an import that may
  cross an Atlas group, editing code that an Atlas view anchors, writing or
  updating a sequence/data-flow/lifecycle/architecture view, or when
  task atlas:check fails in Lumilio Photos — package doc.go, group rules,
  anchored views, and re-verifying stale views.
---

# Keep The Atlas In Sync

The Atlas is the code map: derived group and package maps, every package's
`doc.go`/`doc.ts`, and anchored views under `docs/atlas/views/`. Contract,
anchor grammar, and view schema: [docs/atlas/README.md](../../../docs/atlas/README.md).

## Before changing code

1. Find the area: `docs/atlas/generated/modules.md` (package → group →
   purpose) and the views that mention it. In the site (`task atlas`) a module
   page lists "Appears in views".
2. Read the owning `doc.go` / `doc.ts`. Its invariants are the contract you
   must keep.
3. If the change will add an import into a group that is not in your group's
   `uses` in `docs/atlas/atlas.yaml`, stop and decide: either the dependency is
   right (it becomes an `atlas.yaml` change the reviewer sees) or the code
   belongs elsewhere. Never add an `exceptions:` entry to silence the check
   without a reason a reviewer would accept.

## Adding a Go package

1. Create `doc.go` with the only package comment: what the package owns, its
   invariants, and its entry points as doc links (`[Name]`,
   `[Type.Method]`, `[server/internal/x.Name]`). Do not list importers or
   imports; they are derived.
2. End the comment with `//` and `//atlas:group <id>` using a group from
   `atlas.yaml` on the same side (server/desktop).
3. `task atlas:check`.

## Editing code that a view anchors

`task atlas:check` reports `anchor(s) changed since their view was verified`
with the view file and `file:line`.

1. Open each listed view YAML and the code at each listed location.
2. If behaviour changed, fix the view: labels, steps, transitions, notes. If
   the symbol was renamed or moved, re-anchor it.
3. Only then run `task atlas:lock`. Locking without reading defeats the gate;
   the lock diff is reviewed in the PR as "these views were re-verified".

## Writing or updating a view

1. Pick the kind by the question: *what talks to what over time* → sequence;
   *where data goes and lands* → dataflow; *which states and why it moves* →
   lifecycle (use `enum:` when the states are a typed Go const set or a TS
   string-literal union); *system boundaries* → architecture.
2. Copy the closest existing view. Anchor every node; prefer the narrowest
   anchor (`go:scan.Scanner.RunTurn` over `mod:...`). Use `ext:` only for
   things outside the repository.
3. Keep 6–14 nodes. Put the *why* in `notes`. Link `related` views.
4. `task atlas:generate`, open the view with `task atlas`
   (http://localhost:6690/atlas/; `task atlas:data` refreshes an open page),
   check the drawing — if ELK draws a cyclic state machine badly, try
   `layout: dagre` — then `task atlas:lock`.

## Verification

```sh
task atlas:check
cd server && go test ./tools/atlas/
```

`task atlas:check` must print "all views resolve, all anchors are verified,
and generated files are current". Commit `docs/atlas/**` (views, lock,
generated) with the code change.

## Known failure modes

- **Bare `vp fmt` or editor reflow of `doc.go`** is fine — `gofmt` keeps the
  directive last. Never put code in `doc.go`.
- **A Go doc comment with prose in brackets** like `[optional]` is read as a
  doc link and fails; rephrase it.
- **Ambiguous names**: `go:dto.X` fails because server and desktop both have a
  `dto` package; use the full import path. Same for `ts:` names exported by
  several modules — scope them as `ts:features/<name>#X`.
- **Stale Web facts**: the Go tool reads `.local/atlas/web-facts.json`; the
  Task targets regenerate it first. Run the Task targets, not the Go command.
- **Doc reference failures** come from handwritten docs (`docs:` in
  `atlas.yaml`) naming a path, symbol, Task target, or route that no longer
  exists. Fix the reference; never add the file to an ignore list. Do not
  write a new `docs/*.md` that describes how a package works — put that in
  its `doc.go` / `doc.ts` or an Atlas view.
- **Explorer UI changes** live in `site/docs/.vitepress/atlas/`; verify with
  `task site:build:atlas` (the public `task site:build` tree-shakes the Atlas
  away and would not catch a broken component).
