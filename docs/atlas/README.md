# Lumilio Atlas

The Atlas is the map of this codebase: architecture, sequence, data-flow, and
lifecycle diagrams that **cannot silently drift** from the code. You read it
instead of reading hundreds of thousands of lines; agents read it to know where
a change belongs and which boundaries it must not cross.

```sh
task atlas            # open the Atlas in the internal docs site: http://localhost:6690/atlas/
task atlas:data       # refresh its data; an open Atlas page reloads by itself
task atlas:check      # fail if anything is out of sync (also in task test and CI)
task atlas:generate   # rewrite docs/atlas/generated after editing views or docs
task atlas:lock       # mark views verified — only after re-reading stale ones
```

### Exploring

The Atlas is an internal page of the VitePress site (`site/docs/atlas/`, UI in
`site/docs/.vitepress/atlas/`). It is built only when `LUMILIO_ATLAS=1`, which
`task atlas` sets, and is never part of the public docs build. Diagrams are
laid out with ELK.

- **Click** a node, step, or state to inspect it: its anchor (open it in VS Code
  or on GitHub), its connections, and the other views it appears in. The rest
  of the map dims so the neighbourhood stands out.
- **Double-click** or **Enter** goes inside: a group opens its package map, a
  module opens its neighbourhood.
- **J / K** walk the elements in order, **F** fits, **⌫** goes back, **⌘K** or
  **/** jumps to any view, group, module, or symbol, **I** toggles the
  inspector, **[** the navigator, **1–5** switch lens.
- Drag or two-finger scroll to pan; pinch or ⌘/Ctrl + scroll to zoom. The trail
  at the top records where you have been; the briefing remembers it.

Without a browser, read [generated/README.md](generated/README.md) — every
view is also checked-in markdown with a Mermaid diagram (GitHub renders it)
and a table that links each element to `file:line`.

## The three layers and what each guarantees

| Layer | Source | Written by | Guarantee |
| --- | --- | --- | --- |
| **Derived maps** — group maps, per-group package maps, the [module catalog](generated/modules.md), each module's imports and importers | Go imports, Web imports, `atlas.yaml` | nobody (generated) | Always equal to the code. `atlas:check` fails if the checked-in copy is stale. |
| **Package docs** — `doc.go` per Go package, `doc.ts` per Web feature | the package itself | people and agents | Every package has one; every `[Name]` doc link (Go) and `{@link X}` (TS) resolves; every code-like Mermaid node in a `doc.ts` is imported. A renamed or deleted symbol fails the build. |
| **Anchored views** — `views/<kind>/<id>.yaml` | YAML next to this file | people (agents draft) | Every element is anchored to a symbol, API operation, table, or declared external. A broken anchor fails. A **changed** anchor makes the view *stale* and fails until someone re-reads the view and runs `task atlas:lock`. A lifecycle with `enum:` must draw exactly the enum's members. |

The dependency rules are the fourth piece: [`atlas.yaml`](atlas.yaml) lists
every group and the groups it may use. The checker compares that declaration
with the real import graph **in both directions** — an undeclared import fails,
and so does a declaration nothing needs — and requires the declared graph to be
acyclic. Reviewed package-level exceptions carry a written reason.

### What it does not guarantee

Anchors make **reference drift impossible** and **semantic drift visible**, not
impossible: a stale view says "this code changed after the view was verified",
and a human or agent must re-read and decide. Prose that names no anchor is not
checked. Runtime behaviour (performance, races) is not a diagram's job — tests
and telemetry own it.

## Layout

```
docs/atlas/
  atlas.yaml              groups, uses, exceptions          (authored)
  views/
    architecture/*.yaml   system-level views                (authored)
    sequence/*.yaml       request and job sequences         (authored)
    dataflow/*.yaml       how data moves and where it lands (authored)
    lifecycle/*.yaml      state machines                    (authored)
  atlas.lock.json         verified hash of every code anchor (task atlas:lock)
  generated/              markdown for every view + module catalog (never edit)
server/tools/atlas/       the Go extractor, checker, and generator (writes .local/atlas/*.json)
web/scripts/atlas-facts.ts  the Web extractor (modules, imports, doc.ts, symbols)
site/docs/.vitepress/atlas/  the explorer UI (Vue, Mermaid + ELK), local-only
```

## Package docs (`doc.go`)

Every Go package in `server/` and `desktop/` (except `atlas.yaml`'s `exclude`
list: dev tools and generated code) has a `doc.go` that holds the only package
comment, nothing but the package clause, and a group directive at the end:

```go
// Package scan is the repository scan index. The catalog mirrors each
// repository's tree in repository_entries ...
//
// [Scanner.RunTurn] walks one bounded turn; [Scanner.HashTurn] hashes ...
//
//atlas:group storage
package scan
```

Write what the package **owns**, its **invariants**, and the **entry points**
as doc links (`[Name]`, `[Type.Method]`, `[server/internal/db.DB]`). Do not
write who imports it or what it imports — that is derived and shown next to the
doc. Web features follow [docts.md](../docts.md).

## Anchors

| Anchor | Resolves to | Example |
| --- | --- | --- |
| `go:<pkg>.<Name>[.<Member>]` | a Go declaration; `<pkg>` is a unique package name or a full import path | `go:scan.Scanner.RunTurn`, `go:desktop/internal/control/dto.RuntimePhase` |
| `ts:[<module>#]<Name>` | an exported Web declaration; scope it when the name is not unique | `ts:useAssetsSearch`, `ts:features/upload#runUploadProcess` |
| `api:<METHOD> <path>` | an operation in `server/docs/swagger.yaml` | `api:POST /api/v1/assets/batch` |
| `sql:<table>` | a `CREATE TABLE` in the catalog baseline | `sql:repository_entries` |
| `mod:<module id>` | a whole package or Web module | `mod:server/internal/service` |
| `group:<id>` | an `atlas.yaml` group | `group:storage` |
| `file:<path>` | a checked-in file | `file:deploy/compose/compose.yml` |
| `ext:<name>` | something outside the repository (drawn dashed) | `ext:Lumen Hub` |

`go:` and `ts:` anchors are hashed (the declaration's source text); the others
only have to exist.

## Authoring a view

Every view has `id` (= file name), `kind` (= directory), `title`, `summary`,
`nodes` (each with `id`, `label`, `anchor`, optional `note`, `shape`, `group`),
optional `notes` (titled markdown paragraphs), and `related` view ids. Then,
per kind:

- **architecture / dataflow** — `edges: [{from, to, label?, anchor?, style?}]`.
  `shape`: `store`, `actor`, `queue`, `decision`, `external`. `group` draws a
  labelled box. `style`: `dashed`, `thick`.
- **sequence** — nodes are participants (`shape: actor` for people);
  `steps` are messages `{from, to, label, anchor?, reply?, async?}`, notes
  `{note, over: "a,b"}`, or blocks `{block: loop|alt|opt|par|critical|break,
  label, steps, else: [{label, steps}]}`. Messages are numbered in the diagram
  and in the step table.
- **lifecycle** — nodes are states; `transitions: [{from, to, label?,
  anchor?}]` with `"[*]"` for start and end. Set `enum:` to a typed Go
  constant set or a TS string-literal union, and anchor each state to its
  constant (Go) or set `member:` (TS); the checker then requires the states to
  be exactly the enum.

Optional `layout: dagre` switches a view from the default ELK layout to
dagre, which suits some cyclic state machines (see `desktop-runtime`).

Good views answer one question ("how does an upload become an Asset?"), keep
to roughly 6–14 nodes, and put the *why* in `notes`. Look at
[upload-to-catalog](views/sequence/upload-to-catalog.yaml) and
[desktop-runtime](views/lifecycle/desktop-runtime.yaml) as references.

## When the check fails

| Message | Do this |
| --- | --- |
| `imports X, but group G does not declare uses: [H]` | Decide whether the dependency is right. If yes, add `H` to `G.uses` in `atlas.yaml` (reviewers see an architecture change). If not, remove the import. |
| `declares uses H, but no package import needs it` | Delete the stale declaration. |
| `missing doc.go` / `missing //atlas:group` / `package comment belongs in doc.go` | Write or move the package doc as described above. |
| `doc link [X] does not resolve` | Fix the link; the symbol was renamed or removed. |
| `anchor …: … has no declaration` | The anchored code moved or was deleted. Re-anchor the element, or redraw the view if the flow changed. |
| `N anchor(s) changed since their view was verified` | Open each listed view, compare it with the code at the listed `file:line`, fix the diagram or notes if behaviour changed, then `task atlas:lock`. |
| `enum … member X is not drawn as a state` | Draw the new state and its transitions. |
| `generated files are out of date` | `task atlas:generate`. |

## Prompting agents with the Atlas

A task that names Atlas vocabulary is precise and checkable:

```text
Scope: group `storage`, package server/internal/storage/trash.
Flow: lifecycle view `asset-lifecycle`, transition trashed → [*].
Rules: no new group dependencies; storage must not import service.
Expected Atlas diff: asset-lifecycle gains an "expiry" transition anchored to
the new function; atlas.yaml unchanged.
```

The pull request's diff of `docs/atlas/` then shows whether the agent stayed in
its lane: new group edges appear in `atlas.yaml`, changed flows in the views,
and re-verified anchors in `atlas.lock.json`.
