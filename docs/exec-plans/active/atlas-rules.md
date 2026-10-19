# Atlas rules: one architecture gate

Status: active, created 2026-10-19; nothing has landed. Child of
[server-structure.md](server-structure.md) (Phase A, sessions A1–A4). It
touches only `server/tools`, tests, docs, `docs/atlas`, the Taskfile, and CI
wiring, so it may run between rc.1 sessions. A4 supersedes part of the
[Atlas decision](../../../.agents/decisions/2026-10-01-atlas-anchored-code-map.md)
(hash drift stops failing CI); record that in a new decision at completion.

Goal: `task atlas:check` is the only architecture gate. Every rule is
declared in `docs/atlas/atlas.yaml`, matches type-resolved facts (`go/types`
objects or parsed SQL, never the text of a line or the name of a variable),
and has a fixture that proves it fails. Runtime invariants are behaviour
tests registered in Atlas, content lints live with the module that owns the
content, and `server/tools/architecturecheck` is deleted.

## Non-goals

- Moving packages (Phase B of the parent plan).
- Folding `web/scripts/check-source-boundaries.ts` into Atlas. It resolves
  imports with the TypeScript compiler and is sound; it only gains the Web
  half of the Problem lint (A3).
- Changing the explorer UI beyond what A4's edge results need.

## Fixed contracts

- **Type-resolved or parsed, never textual.** A rule names Go objects
  (`(*database/sql.DB).BeginTx`, `execution.Resources`) resolved through
  `go/types`, or SQL parsed from sqlc query files, string constants, and the
  baseline migration. Matching a source line, a variable name, or a
  function-name suffix is a lint, and lints belong to the owning package's
  tests.
- **Every rule is proven.** Each rule ships a violation fixture that must
  report exactly the expected finding and a passing fixture, both under
  `server/tools/atlas/testdata/rules/<rule-id>/`, run by
  `go test ./tools/atlas`.
- **Baselines only shrink.** A new rule that finds existing violations
  records them by symbol in `atlas.lock.json`; `atlas check` fails on any
  finding not in the baseline, and `atlas lock` drops entries that
  disappeared but never adds them. Adding a baseline entry by hand needs the
  owner's approval in the PR.
- **Every rule states why.** A rule carries an `id` and a one-line `why` that
  names the invariant it protects, and the invariant registry references it.
- **No coverage gap during the switch.** `architecturecheck` is deleted in
  the same PR (A3) that lands the last replacement, never earlier.

## Audit of `architecturecheck` (2026-10-19, 2,103 lines)

Class A is sound and type-aware or close to it, B matches snippets of code
text, C guards names that no longer exist, and D is a content lint rather
than architecture.

| Check today | Class | Destination |
| --- | --- | --- |
| `checkSQLiteTransactionInventory`: `BeginTx` allowed when the receiver is *named* `writer`, `Writer`, or `snapshot` | A, name-based | `calls` rule on `(*sql.DB).Begin`, `(*sql.DB).BeginTx`, `(*sql.Conn).BeginTx`, `(*sql.DB).Exec`, `(*sql.DB).ExecContext`, only in `catalogtx` and `db` (migration, pragmas); A2 |
| `scanAssetHardDeletes`: regex `DELETE FROM assets` outside `lifecycle/purge.go` | A | `tables` rule `assets: delete → [lifecycle]`; A2 |
| `checkScanIndexWriterBudget`: no I/O, hashing, sleep, or repository-wide `COUNT` inside `…Tx`-named functions and `WithTx` closures of `storage/scan`; `MaxBatchRows ≤ 256` | A, scope by name | `tx-purity` rule over the whole Server with a type-defined scope (below) and the same forbidden set resolved by type, existing violations baselined; the constant becomes an assertion in `storage/scan/budget_test.go`; A2, A3 |
| `scanAsyncProducerCapabilities`, `scanScanIndexWriteCapabilities`: listed files must not hold `*sql.DB`, `*repo.Queries`, the `db` import, or listed service types | A, partly by name | `forbid-types` rule (fields, parameters, variables) on the same packages; A2 |
| River: workers inserting child jobs; `river_job` named outside `queue`; `Governor` construction; `execution.Resources{}` literals | B | `calls` rules on `(*river.Client).Insert*` (scheduler only) and `execution.NewGovernor`; `constructs` rule on `execution.Resources`; `tables` read and write rule for River tables; A2 |
| `checkSQLiteConnectionArchitecture`: 8 required snippets in `db.go`, 13 required and 4 forbidden snippets in `app.go`, required or forbidden lines in `bootstrap_service.go`, `runtime_status.go`, `storage_locations.go`, `host_action.go`; `Reconcile` calls in handlers | B | Behaviour tests (A3): connection policy in `internal/db` (writer `MaxOpenConnections == 1`, a reader write fails as query-only, reader pool bound, `wal_autocheckpoint = 0`) and "foreground reads never write" in `server/app` (a `catalogtx` observer installed with `db.WithTransactionObserver` sees zero writer admissions across setup status, the storage view, host-action lists, queue status, and the Agent library read); `calls` rule limiting `ReconcileAll` and `ReconcileStorageLocations` to `server/app` and `storage`. Parent B2 later turns reader wiring into compile errors |
| Closed River macro catalog (8 kinds registered once, parsed from `river.AddWorker[` text) | B | Test in `server/app` that builds the production worker set and compares its kinds with `queue/jobs`; A3 |
| `app.go` must not contain scheduling algorithm calls | B | `calls` rule: those functions are called only from ingest packages; A2 |
| Legacy worker and job names (17 + 17), `MigrateWithPreflight`, `DirectoryManager` methods, `IsRaw`, `total_assets` | C | Delete. The OpenAPI contract and generated types already pin the API fields |
| `StackMode` in fused search, `stack_mode` outside `useAssetsList.ts` | C | Delete. `asset_handler_browse_contract_test.go` rejects `stack_mode` on search, and the generated search request type has no such field |
| Exported raw-path destructive methods, `filepath.Join` with `repoPath`-named variables | B, name-based | Delete. `RepositoryFS` is the guard; B12's `storage/internal/` boundary makes it compile-checked |
| `checkDesktopArchitecture` (Server imports outside `desktop/internal/runtime`) | duplicate | Delete; the Atlas Desktop groups already enforce it |
| `checkUserFacingTerminology` (Web, Desktop, docs, READMEs, API annotations) | D | Move verbatim to `server/tools/terminology` with `task terminology:check`, owned by `lumilio-frontend-i18n`; A3 |
| `checkAPIProblemArchitecture` (success writes, private error use, catalog artifacts, Web Problem helpers) | D | Go half to a test in `internal/api`; Web half to `check-source-boundaries.ts`; catalog completeness already in `api/problem/problem_test.go`; A3 |
| `checkSQLiteSQL` (unresolved sqlc macros, `randomblob`, slice plus fixed parameters) | D | Test in `internal/db` over the generated and query files; A3 |
| Naked ffmpeg `-threads`/`-preset` flags | D | Test in `internal/processors`; A3 |

## Target `atlas.yaml` additions (shape, finalized in A2)

```yaml
rules:
  - id: catalog-transactions
    why: one writer; every transaction is named, admitted, and measured
    calls: ["(*database/sql.DB).BeginTx", "(*database/sql.DB).ExecContext", "…"]
    only: [server/internal/db/catalogtx, server/internal/db]
  - id: asset-purge
    why: an Asset disappears only through the lifecycle purge
    tables: { assets: { delete: [server/internal/lifecycle] } }
  - id: tx-purity
    why: no filesystem, network, hashing, or sleeping inside a writer transaction
    tx_purity: { forbid_packages: [os, io, io/fs, net, …], forbid_calls: ["time.Sleep", …] }
  - id: capabilities-no-http
    why: transport stays in api
    forbid_imports: { from: [<capability paths>], imports: [net/http] }   # added in B1
budgets: { default: 5000 }        # recorded ceilings live in atlas.lock.json
invariants:
  - id: one-writer
    doc: docs/architecture.md#cross-cutting-invariants
    enforced_by: [rule:catalog-transactions, go:server/internal/db.TestConnectionPolicy]
```

The `tables` rule reads DML targets from three sources: sqlc query files
(`-- name: X` maps to `(*repo.Queries).X`, whose call sites give the writing
package), string constants passed to `Exec`/`Query` methods (constant-folded
by `go/types`), and triggers in the baseline migration (owner
`trigger:<name>`). Dynamic SQL that cannot be folded is reported as
unresolved and baselined. The SQL scanner handles `WITH`, `INSERT OR …`,
`REPLACE INTO`, `UPDATE OR …`, and `DELETE FROM`, and has its own tests.

The `tx-purity` scope is defined by types, not names: its roots are every
function or function literal with a `*sql.Tx` or `*catalogtx.Tx` parameter
(which includes every `WithTx` closure) and the statements between a
`(*catalogtx.Writer).BeginTx` call and its `Commit`; the scope is the roots
plus every module function statically reachable from them, with interface
calls resolved to their module implementations. A function reached from a
transaction body runs inside that transaction, so `PurgeEntriesTx` and its
peers are covered through the call graph whatever their names.

## Execution phases

### A1 — Typed facts and path-based groups
- [ ] Load Server and Desktop facts with `golang.org/x/tools/go/packages`
  and `go/types` (already an indirect dependency of `server`).
- [ ] Server and Desktop groups declare `match` paths like the Web groups;
  delete every `//atlas:group` directive (each `doc.go` stays). Today's
  membership is expressed with explicit paths; parent B1 simplifies them to
  directory globs.
- [ ] Rule engine skeleton (`rules:` list, `id`/`why`, findings, baselines in
  `atlas.lock.json`) and the fixture harness. The group-dependency check is
  the first rule; its findings and the generated maps are unchanged.

### A2 — Static rules
- [ ] Rule kinds `calls` (only/forbid), `constructs`, `forbid-types`,
  `forbid-imports`, `tables`, `tx-purity`, and `budgets` (ceilings recorded
  per package, default 5,000 production lines).
- [ ] Port every A-class check and the type-resolvable B-class checks from
  the audit, each with a violation and a passing fixture; record baselines.
- [ ] Run `architecturecheck` and Atlas side by side: every finding the old
  tool reports on a deliberately broken branch is reported by Atlas or by a
  named A3 test.

### A3 — Behaviour tests, relocated lints, and the switch
- [ ] Connection-policy and foreground-read tests, the River kind-catalog
  test, and the `MaxBatchRows` assertion; each shown failing on a temporary
  violation before it lands.
- [ ] `server/tools/terminology` and `task terminology:check`; the Problem,
  sqlc-hazard, and ffmpeg lints in their package tests and in
  `check-source-boundaries.ts`.
- [ ] `invariants:` registry: every bullet under "Cross-cutting invariants"
  in `docs/architecture.md` has an entry whose `enforced_by` resolves; an
  invariant without enforcement fails `atlas check`.
- [ ] Delete `server/tools/architecturecheck` and `task architecture:check`.
  `task test` keeps `atlas:check`; `ci:architecture` runs
  `terminology:check` (Atlas already runs in CI through
  `verify:generated` → `atlas:generate`; confirm the `generated` job covers
  it); update workflow path filters per `lumilio-add-task-target`.
- [ ] Update every live reference: `CLAUDE.md`, `docs/architecture.md`,
  `lumilio-select-checks`, `lumilio-frontend-i18n`, `lumilio-atlas`,
  `lumilio-add-task-target`. Decision records keep their history.

### A4 — Views that verify arrows, and refactor tools
- [ ] Sequence and data-flow steps whose `from` and `to` are `go:` anchors
  are verified: the `from` body references `to` directly, or through an
  interface method that `to` implements, or through a declared `via:` hop.
  A missing arrow is an error. A changed anchor hash becomes a notice that
  does not fail CI.
- [ ] `atlas split-report <package> [--clusters file]`: type-resolved
  file-to-file dependencies, inter-cluster edges, crossing unexported
  symbols, and cycles (the analysis behind the parent plan's tables).
- [ ] `atlas movecheck --base <ref>`: indexes every function of the base and
  head trees by receiver type and name, normalizes package qualifiers and
  export capitalization, and lists moved, changed, added, and removed
  functions; parent Phase B attaches it to every PR.
- [ ] Update `lumilio-atlas` (lock procedure, notices versus errors, the two
  tools).

## Validation boundaries

- `server/tools/architecturecheck` and `task architecture:check` are gone,
  and every row of the audit is a typed Atlas rule, a named test,
  `terminology:check`, or listed as deleted.
- `go test ./tools/atlas` runs a violation and a passing fixture for every
  rule, and each violation fixture fails.
- No Atlas rule matches source text, variable names, or name suffixes.
- Baselines contain only violations that existed when their rule landed,
  and none has grown.
- Every cross-cutting invariant in `docs/architecture.md` names enforcement
  that Atlas resolves.
- A sequence view with a removed call fails `atlas check`; a refactor that
  keeps every drawn call produces a notice only.
