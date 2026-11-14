# Atlas rules: one architecture gate

Status: active, created 2026-10-19; A1 landed 2026-11-14 (#270); A2 landed 2026-11-14 (#271); A3 landed 2026-11-14 (#272); A4 landed 2026-11-14 (#273). Every phase is done; the plan completes with parent B14. Child of
[server-structure.md](server-structure.md) (Phase A, sessions A1–A4). It
touches only `server/tools`, tests, docs, `docs/atlas`, the Taskfile, and CI
wiring, so it may run between rc.1 sessions. A4 supersedes part of the
[Atlas decision](../../../.agents/decisions/2026-10-01-atlas-anchored-code-map.md)
(hash drift stops failing CI); recorded in
[the arrows decision](../../../.agents/decisions/2026-11-14-atlas-views-verify-arrows.md).

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
| `checkRecoveryReachability` (R10, added after the audit): every public Problem's recovery entry names a Web route and Desktop host action kind that exist | D | Test in `internal/api/problem` that reads the Web route table and the storage host action kinds; A3 |
| `checkStorageClassifierBoundary` (P7): storage risk code literals only in the classifier files; retired classifier symbols absent | D, then C | Literal half: test in `internal/storage` over its non-classifier files; retired symbols: delete; A3 |
| `checkDesktopMarkerParsing` (P7): no marker file names or YAML decoding in Desktop | A for YAML, D for names | `forbid_imports` rule `desktop-marker-parsing` (A2); the marker file names become a test in `desktop/internal/runtime`; A3 |
| `checkHandlerActionLists` (P7): no storage action or resolution code built into a slice inside `api/handler` | D | Test in `internal/api/handler`; A3 |
| `checkCandidateCreateRisks` (P7): repository candidates carry no risks, and candidate commands take no boolean confirmation | D | Go half: test in `internal/storage` and `internal/api/dto`; Web half: `check-source-boundaries.ts`; A3 |
| `checkContractEnumCoverage` (P7): every closed vocabulary the contract exposes has literal copy | D | Moves with the terminology check to `server/tools/terminology`; A3 |

## Target `atlas.yaml` additions (the sketch; A2 landed the rules in `docs/atlas/atlas.yaml`)

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
- [x] Load Server and Desktop facts with `golang.org/x/tools/go/packages`
  and `go/types` (already an indirect dependency of `server`).
- [x] Server and Desktop groups declare `match` paths like the Web groups;
  delete every `//atlas:group` directive (each `doc.go` stays). Today's
  membership is expressed with explicit paths; parent B1 simplifies them to
  directory globs.
- [x] Rule engine skeleton (`rules:` list, `id`/`why`, findings, baselines in
  `atlas.lock.json`) and the fixture harness. The group-dependency check is
  the first rule; its findings and the generated maps are unchanged.

Found in A1 (2026-11-14), binding on A2–A4:

- **Two fact layers.** Typed facts (`typed.go`) are the host build:
  cgo-processed syntax and `go/types` objects for the files the platform
  compiles (Linux in CI). The map facts (symbols, anchor hashes, doc links,
  imports, line counts) stay a syntactic parse of every non-test file,
  because the Server has `_windows`/`_darwin` files and cgo rewrites file
  text; only that keeps generated files and lock hashes identical on every
  host. Rules use typed facts; a rule about platform-specific files must say
  so.
- **The loader builds like the compiler.** `task atlas:run` sets
  `GOTOOLCHAIN` to Desktop's `go` line plus `+auto` (go/types refuses a
  module newer than the tool's own Go, and Desktop is on 1.27) and
  `GOFLAGS=-tags=$GO_BUILD_TAGS`; the root CGo allowlist applies. The CI
  `generated` job therefore sets up Go from `desktop/go.mod` and installs the
  Server's CGo headers. Any type error is an Atlas problem, never ignored.
- **Lock format.** `atlas.lock.json` is `{views, baselines}`. Finding keys
  are symbols or package paths, never lines. `atlas baseline <rule>`
  (`task atlas:baseline -- <rule>`) records findings only for a rule with no
  baseline entry yet; `atlas check` fails on a stale entry and `atlas lock`
  drops it.
- **Fixture shape.** `testdata/rules/<id>/{violation,pass}/` is a small
  repository: `atlas.yaml`, Go module(s), and, for `violation`, `want.txt`
  with the exact finding keys. `TestEveryDeclaredRuleHasFixtures` fails for
  a declared rule without both.

### A2 — Static rules
- [x] Rule kinds `calls` (only/forbid), `constructs`, `forbid-types`,
  `forbid-imports`, `tables`, `tx-purity`, and `budgets` (ceilings recorded
  per package, default 5,000 production lines).
- [x] Port every A-class check and the type-resolvable B-class checks from
  the audit, each with a violation and a passing fixture; record baselines.
- [x] Run `architecturecheck` and Atlas side by side: every finding the old
  tool reports on a deliberately broken branch is reported by Atlas or by a
  named A3 test.

Found in A2 (2026-11-14), binding on A3–A4:

- **Side by side.** Fourteen deliberate breakages, one per ported check
  (raw `BeginTx`, `DELETE FROM assets`, I/O and sleep in a scan `Tx`
  function, a producer holding `*sql.DB` or importing `db`, the scan index
  building `repo.New`, `river_job` in a service, a worker inserting child
  work, a second governor, a `Resources{}` literal, a scheduling call in
  `app.go`, a handler calling `ReconcileAll`, YAML in Desktop): each fails
  both tools, and Atlas names the rule. The old text check for
  `switch args.ProjectionKind` in `app.go` became a `scheduling-algorithms`
  target, `(*pipelineRuntime).prepareProjection`. Marker file names in
  Desktop are left to the A3 Desktop test.
- **Executors forward SQL.** `sql.executors` lists `*sql.DB`, `*sql.Tx`,
  `*sql.Conn`, the `catalogtx` `Tx`, `Writer`, and `Reader`, and the
  `db.queryRouter`; an executor's own methods pass their caller's SQL on, so
  the statement is resolved at the caller. A function that picks SQL at run
  time is a `static-sql` finding, and every SQL constant in it is attributed
  to it, so `lifecycle.collectAssetsTx`, which runs a table of constants,
  still owns its `DELETE FROM assets`. A table is known when a migration
  creates it or any statement uses it (River creates its own).
- **tx-purity precision.** Interface calls resolve to module implementations
  only for interfaces declared in a mapped package; a library interface
  method (`io.Writer.Write` on a SHA-256 hash) is never judged by its
  package, which had resolved one hash write to every module `io.Writer`. A
  non-deferred `Rollback` ends a Begin region on its path. Findings carry the
  call chain from the root. `CountStackedMediaItems` is allowed: it is
  bounded by the media items of one Live Photo pair.
- **Fixed rather than baselined:** `reconnectRepository` wrote the
  repository audit log file inside its writer transaction on the
  duplicate-path failure; it now rolls back first
  (`TestReconnectOntoAClaimedPathFailsAndKeepsTheRow`). `architecturecheck`
  walks skip `testdata`, so the rule fixtures do not trip it.
- **Baselines recorded (owner approval in the A2 PR):**
  - `tx-purity`, 16 entries, all one root:
    `(*faceService).ApplyFaceResultsTx` opens the repository FS, encodes,
    writes, and fsyncs face crops, removes old crops, and runs the
    owner-wide `CountIncrementalFaceNeighbors` DBSCAN core check per face,
    all inside the writer transaction. Owned by sequence step RA3
    ([repository access](repository-access-simplification.md) Phase 3):
    crops become `faces` artifacts published before the transaction (RL3),
    the `RepositoryFS` open leaves it, and the DBSCAN COUNT moves out or is
    bounded. Its gate, "`tx-purity` clean for face apply", empties this
    baseline.
  - `static-sql`, 16 entries: migration and pragma application in `db`,
    `sqlitespike`, and run-time-built statements in `event`, `lifecycle`,
    `processing`, `search`, and `service` classifier definitions.
  - `package-size` ceilings: `api/handler` 18,129, `db/repo` 31,464,
    `service` 20,116, `storage` 14,605. Parent B14 tightens them.
- **`atlas baseline <rule>` records only that rule.** Recording one rule had
  also written empty baselines for every other unbaselined rule, which
  would have blocked baselining them later.

### A3 — Behaviour tests, relocated lints, and the switch
- [x] Connection-policy and foreground-read tests, the River kind-catalog
  test, and the `MaxBatchRows` assertion; each shown failing on a temporary
  violation before it lands.
- [x] `server/tools/terminology` and `task terminology:check`; the Problem,
  sqlc-hazard, and ffmpeg lints in their package tests and in
  `check-source-boundaries.ts`.
- [x] `invariants:` registry: every bullet under "Cross-cutting invariants"
  in `docs/architecture.md` has an entry whose `enforced_by` resolves; an
  invariant without enforcement fails `atlas check`.
- [x] Delete `server/tools/architecturecheck` and `task architecture:check`.
  `task test` keeps `atlas:check`; `ci:architecture` runs
  `terminology:check` (Atlas already runs in CI through
  `verify:generated` → `atlas:generate`; confirm the `generated` job covers
  it); update workflow path filters per `lumilio-add-task-target`.
- [x] Update every live reference: `CLAUDE.md`, `docs/architecture.md`,
  `lumilio-select-checks`, `lumilio-frontend-i18n`, `lumilio-atlas`,
  `lumilio-add-task-target`. Decision records keep their history.

Found in A3 (2026-11-14), binding on A4:

- **A foreground read wrote.** The writer-held test found that opening an
  agent ref (`GET /api/v1/agent/refs/:id` and its assets, music, and pins)
  ran `GetAgentRef`, an `UPDATE` that slid the ref's expiry, under the ref
  store's lock. Owner decision: the read is a `SELECT`; a ref lives while its
  thread is active, then for the TTL `ReleaseAgentThreadRefs` starts, and
  viewing it renews nothing.
- **"Foreground reads never write" is tested by holding the writer.** The
  `server/app` tests take the one writer connection, then require setup
  status (before and after setup), storage targets and view, native tasks,
  processing status, and an agent ref to answer within 3 s. That catches a
  read bound to the writer pool and a read that persists on the way; the
  audit's observer design could not tell a request's writes from background
  work. Two reconciles write nothing on an idle system, so they are covered
  elsewhere: the bootstrap phase by the before-setup test, runtime status by
  `storage-reconciliation`, whose `calls` rule now takes `only` and `forbid`
  together (`forbid` carves `runtime_status.go` out of `storage`).
- **Wiring checks wait for B2.** The agent library's reader only feeds the
  event resolver of agent tool runs, which no HTTP read reaches. The
  `app.go` wiring snippets therefore moved verbatim to
  `TestProductionCatalogWiring`, which B2 deletes when constructors take
  `catalogtx.Reader` and `catalogtx.Writer`.
- **`MaxBatchRows` was already asserted** by
  `TestWriterTransactionsStayWithinTheRowBudget` (#222).
- **Problem artifacts were not covered** by `problem_test.go`, as the audit
  assumed; `TestEveryProblemTypeHasItsArtifacts` ports that check.
- **Web halves scan source files only**, not the `doc.md`, JSON, and HTML
  under `features/` or the `wasm/` bundles the old walk also read.
- **`invariants:`** is one section (`doc:`) with an entry per bold bullet
  title; references are `rule:`, `go:<import path>.<Test>`, and `task:`.
- **Linux gate target.** `task server:test:linux` builds
  `server/test.Dockerfile` on the Docker host named by `DOCKER_CONTEXT` and
  runs CI's three Linux Server jobs (`server:test:ci`,
  `server:test:concurrency:ci`, `server:test:volumes` on loop-mounted vfat,
  exfat, ext4, and casefold images) in a detached, privileged container.
  The vfat image is mounted with `utf8` explicitly: Fedora kernels (the
  Radxa) default to `iocharset=ascii` and refuse `é`, which Ubuntu CI
  runners accept by default.

### A4 — Views that verify arrows, and refactor tools
- [x] Sequence and data-flow steps whose `from` and `to` are `go:` anchors
  are verified: the `from` body references `to` directly, or through an
  interface method that `to` implements, or through a declared `via:` hop.
  A missing arrow is an error. A changed anchor hash becomes a notice that
  does not fail CI.
- [x] `atlas split-report <package> [--clusters file]`: type-resolved
  file-to-file dependencies, inter-cluster edges, crossing unexported
  symbols, and cycles (the analysis behind the parent plan's tables).
- [x] `atlas movecheck --base <ref>`: indexes every function of the base and
  head trees by receiver type and name, normalizes package qualifiers and
  export capitalization, and lists moved, changed, added, and removed
  functions; parent Phase B attaches it to every PR.
- [x] Update `lumilio-atlas` (lock procedure, notices versus errors, the two
  tools).

Found in A4 (2026-11-14), binding on Phase B:

- **The views drew calls the code does not make.** Checked against the typed
  facts, seven views were redrawn: the search handler calls
  `SearchBrowseItems` and the service embeds the query and hydrates fused
  members (`resolveSemanticQueryEmbedding`, `browseItemsForMediaRefs`);
  workers reach the scanner, the materializer, and the processors through
  `server/app`'s `pipelineRuntime`; the original is read through
  `RepositoryFS`; thumbnails, not the derivatives stage, feed enrichment;
  River, the LLM provider, and the agent runtime are `ext:` participants.
- **Hop semantics beyond the plan's three.** Direct reference, interface
  implementation, and `via:` were not enough for real wiring. A hop also
  holds through a constructor returning the target, through func values (a
  func-typed field reaches what is stored in it by literal, assignment,
  constructor, or setter; a func-typed parameter reaches its call sites'
  arguments; a func-returning wrapper counts), and for a `mod:` participant,
  through anything the package declares. A func value counts only where it
  is called. Step anchors on neither end are a message type both ends name
  or a wiring site that names both. Data-flow edges are undirected. Fixture:
  `testdata/arrows/{pass,violation}`; eight mutations of the pass fixture
  each fail it.
- **Generated pages link files without lines.** Otherwise any edit above an
  anchor rewrote checked-in markdown and failed `generated`, so a refactor
  that keeps every call could not produce "a notice only".
- **`split-report` reproduces the parent plan's numbers**: `service` is 39
  files, 20,116 lines, 60 file-to-file edges. With the plan's capability
  clusters it finds one blocking cycle, assets ↔ people ↔ search, and five
  unexported symbols crossing the cut (`canonicalizeSemanticVector`,
  `optionalStringFromSQLiteUUID`, `normalizeStoredGeocoding`,
  `derefFloat64`, `detectionOwnerKey`).
- **`movecheck` renames need a unique, non-empty body.** Empty or shared
  bodies (`func gone() {}`) would otherwise pair unrelated additions and
  removals.

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
