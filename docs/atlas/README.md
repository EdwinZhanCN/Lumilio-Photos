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
task atlas:baseline -- <rule-id>   # record a NEW rule's existing violations (owner approves)
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

Handwritten documents are the fifth piece. Every file listed under `docs:` in
[`atlas.yaml`](atlas.yaml) — AGENTS.md, CONTRIBUTING.md, `docs/*.md`,
`web/ARCHITECTURE.md`, and every skill — must only name things that exist:
repository paths in backticks (including the `server/app.Run` symbol form),
relative links, `task` targets, and `/api/v1` routes. Package doc comments get
the same check for routes and Task targets. A document that explains *how code
works* does not belong in `docs/` at all: that is the owning `doc.go` or
`doc.ts`, or an Atlas view. `docs/` keeps intent, cross-cutting rules,
external contracts, and procedures.

The dependency rules are the fourth piece: [`atlas.yaml`](atlas.yaml) lists
every group, the paths it `match`es, and the groups it may use. Every Go
package and Web module belongs to exactly one group. The `group-dependencies`
rule compares the declaration with the real import graph **in both
directions** — an undeclared import fails, and so does a declaration nothing
needs — and the declared graph must be acyclic. Reviewed package-level
exceptions carry a written reason. The other architecture rules live beside it
under `rules:` (see [Rules](#rules)).

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
  atlas.lock.json         verified hash of every code anchor, rule baselines (task atlas:lock)
  generated/              markdown for every view + module catalog (never edit)
server/tools/atlas/       the Go extractor, rule engine, checker, and generator (writes .local/atlas/*.json)
server/tools/atlas/testdata/rules/<id>/  each rule's violation and passing fixture
web/scripts/atlas-facts.ts  the Web extractor (modules, imports, doc.ts, symbols)
site/docs/.vitepress/atlas/  the explorer UI (Vue, Mermaid + ELK), local-only
```

## Package docs (`doc.go`)

Every Go package in `server/` and `desktop/` (except `atlas.yaml`'s `exclude`
list: dev tools and generated code) has a `doc.go` that holds the only package
comment and nothing but the package clause:

```go
// Package scan is the repository scan index. The catalog mirrors each
// repository's tree in repository_entries ...
//
// [Scanner.RunTurn] walks one bounded turn; [Scanner.HashTurn] hashes ...
package scan
```

The package joins its group through a `match` path in `atlas.yaml`, never a
directive in the file. Server and Desktop groups list explicit package paths
today; the server-structure plan turns them into directory globs once each
group owns one directory.

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

## Rules

Every architecture rule is declared under `rules:` in
[`atlas.yaml`](atlas.yaml) with an `id`, a one-line `why` naming the invariant
it protects, and exactly one kind. Rules match **type-resolved facts**: the
Server and Desktop modules are loaded with `golang.org/x/tools/go/packages`
and type-checked by `go/types`, exactly as the compiler builds them (CGo,
`GO_BUILD_TAGS`, and the newest Go toolchain any module declares, which the
Task targets select). A rule never matches the text of a line, a variable
name, or a name suffix; a check like that is a lint and belongs in the owning
package's tests.

Typed facts cover the files the host platform builds, which in CI is Linux.
The map itself (symbols, anchor hashes, doc links, imports) still comes from
parsing every non-test file regardless of build constraints, so generated
files are identical on every host.

Targets name Go objects the way `go doc` would print them with full import
paths: `pkg.Func`, `(*pkg.T).Method`, `(pkg.Iface).Method`, and types as
`pkg.T` or `*pkg.T`. A target that does not resolve is a problem, so a rename
cannot silently disable a rule. Scopes (`only`, `forbid`, `in`, `from`, table
owners) are package directories, `dir/**` globs, or single files.

| Kind | Checks |
| --- | --- |
| `group_uses: true` | Every import between groups follows `groups[].uses` and the reviewed `exceptions` (rule `group-dependencies`). |
| `calls: {targets, only, forbid}` | Who may refer to a function or method: calls, method values, and generic instantiations, through interfaces whose implementations include a target. A reference outside `only`, or inside `forbid`, is a finding; give either or both (`forbid` then carves files out of `only`). Key `caller -> target`. |
| `constructs: {types, only}` | Who may build a value of a type with a composite literal or `new`. Key `func constructs T`. |
| `forbid_types: {in, types}` | No field, parameter, result, variable, or function literal in scope holds one of the types (directly, behind pointers, or in slices, maps, and channels). Key `name holds T`. |
| `forbid_imports: {from, imports}` | No file in scope imports a listed path or `path/**`. Key `file imports X`, or `package imports X` when a package is in scope whole. |
| `static_sql: true` | Every SQL statement handed to an executor (`sql.executors`) is a constant or a sqlc query, so the table rules see it. A function that picks a statement at run time is a finding, and its SQL constants are still attributed to it. Key `func executes non-constant SQL`. |
| `tables: {table: {read, write, insert, update, delete}}` | Who may read or write a table. Statements come from sqlc query files (`sql.queries`, attributed to the callers of the generated method), constants passed to executors, and migration triggers (owner `trigger:<name>`); `write` covers the operations not listed on their own. Key `owner op table`. |
| `tx_purity: {tx_types, begin, forbid_packages, forbid_calls, forbid_count_queries, allow}` | Nothing slow runs inside a transaction. Roots are every function or literal with a `tx_types` parameter and the statements between a `begin` call and its `Commit` (a non-deferred `Rollback` ends that path). The scope is everything statically reachable from the roots; calls through interfaces declared in the module resolve to their module implementations, while library interfaces such as `io.Writer` are judged at the implementation the walk reaches, never by their package. Key `func calls callee`. |
| `budgets: {default}` | No package exceeds its production-line ceiling: the recorded one in `atlas.lock.json` `ceilings`, else the default. Ceilings only fall: `task atlas:lock` lowers one to the package's size and drops it once the package is under the default. |

`sql:` in `atlas.yaml` says where statements come from; it is shared by
`static_sql` and every `tables` rule.

**Every rule is proven.** `server/tools/atlas/testdata/rules/<rule-id>/` holds
a `violation/` fixture, whose findings must equal its `want.txt` exactly, and
a `pass/` fixture that must load without any problem. Each fixture is a small
repository: an `atlas.yaml` and one or more Go modules, type-checked like the
real one. `go test ./tools/atlas` fails for a declared rule without both.

**Baselines only shrink.** A finding has a key made of symbols and package
paths, never line numbers. A new rule that finds existing violations lands
with them recorded by `task atlas:baseline -- <rule-id>`, which works only
for a rule that has no baseline yet; the owner approves those entries in the
PR. After that, `atlas check` fails on any finding not in the baseline and on
any baseline entry that no longer occurs, and `task atlas:lock` drops fixed
entries but never adds one.

**Runtime facts are tests, not rules.** What only running code shows (one
writer connection, foreground reads that never wait for it, the River worker
set) is a behaviour test in the package that owns it, and a content lint
(Problem responses, sqlc hazards, risk-code literals) is a test of the
package it guards. Product vocabulary and contract copy are
`task terminology:check`.

**Every cross-cutting invariant names its enforcement.** `invariants:` ties
each bold-titled bullet under `doc:` (`docs/architecture.md#cross-cutting-invariants`)
to the `rule:<id>`, `go:<import path>.<TestName>`, and `task:<name>`
references that enforce it. `atlas check` fails on a bullet without an entry,
an entry without a bullet or without enforcement, and a reference that does
not resolve.

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
| `missing doc.go` / `package comment belongs in doc.go` | Write or move the package doc as described above. |
| `no group in docs/atlas/atlas.yaml matches this module` / `match … matches no module` | Add the new package's path to its group's `match`, or remove the path of a package that moved or was deleted. |
| `[rule-id] …` | A rule found a new violation. Fix the code. Read the rule's `why` first; the rule is the decision, the code is what changes. |
| `baselined violation … no longer occurs` | You fixed a baselined violation. Run `task atlas:lock` to drop the entry. |
| `ceiling N for P is above its M lines` | A package over the default shrank. Run `task atlas:lock` to lower its ceiling. |
| `has N production lines, over its ceiling of M` | Split the package or move code out; a ceiling is never raised by hand. |
| `… does not resolve` (a rule target, type, or `sql:` entry) | The named object was renamed or removed. Point the rule at its new name; if the code it guarded is gone, ask whether the rule still protects anything. |
| `type-check failed: …` | The Atlas could not type-check a package, so rule facts would be unsound. Run through the Task target (it sets the CGo flags, build tags, and toolchain) with the Server's CGo headers installed, or fix the compile error. |
| `invariant "…" has no invariants entry` / `enforcement … does not resolve` | A new or reworded invariant needs an `invariants:` entry naming the rule, test, or Task target that enforces it; a renamed test or rule needs its reference updated. Never add an entry with nothing real behind it. |
| `doc link [X] does not resolve` | Fix the link; the symbol was renamed or removed. |
| `anchor …: … has no declaration` | The anchored code moved or was deleted. Re-anchor the element, or redraw the view if the flow changed. |
| `N anchor(s) changed since their view was verified` | Open each listed view, compare it with the code at the listed `file:line`, fix the diagram or notes if behaviour changed, then `task atlas:lock`. |
| `enum … member X is not drawn as a state` | Draw the new state and its transitions. |
| `generated files are out of date` | `task atlas:generate`. |
| `` `x` names a repository path or symbol that does not exist`` / `link … points at a file that does not exist` / `names a Task target` / `API route … is not in the OpenAPI document` | A handwritten document drifted. Point it at the new name, or delete the sentence if the thing is gone. A real route deliberately missing from OpenAPI goes in `undocumentedRoutes`. |

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
