# Server structure: capability packages and Atlas as the only architecture gate

Status: active, created 2026-10-19; nothing has landed. Owner decisions
(2026-10-19): the cleanest structure wins over a move-only refactor, so code,
Go type names, and OpenAPI schema names may change; Atlas replaces
`server/tools/architecturecheck`; Huma replaces swag, so the HTTP contract is
generated from Go types (B9–B11). Revised 2026-10-28 (owner review): a
database boundary keeps the SQLite engine inside `catalog` and generated rows
out of `api/` and capability APIs (Fixed contracts; B2, B11, B13); gin is
confined to the `api` root package, `api/httpx`, and `server/app`;
`server/platform` moves under `internal`. Phase A
([atlas-rules.md](atlas-rules.md)) touches only tools, tests, docs, and
`atlas.yaml` and may run between rc.1 sessions. Owner decision 2026-10-28:
Phase B also lands before the rc.1 tag, under the fix-before-release rule in
[rc-release.md](rc-release.md); its order relative to the open storage plans
is fixed in [rc1-sequence.md](rc1-sequence.md) (2026-10-28): B1–B2 run after
Phase A and before the storage plans, B3–B14 after them. The session
order is the table under "Execution phases".

Goal: the Server tree reads as eight layers — `foundation`, `media`,
`catalog`, `work`, business capabilities, `ingest`, `api`, `app` — where each
business capability is one package (or a small tree) that owns its rules, its
transactional writes, and its read API, and every architecture rule is a
type-resolved Atlas rule or a registered behaviour test that has been seen to
fail, and the OpenAPI contract is generated from the Go types of the
transport packages. SQLite is an implementation detail of `catalog`: no
package above it imports an engine driver, and no generated row type crosses
a capability's public API or reaches the transport. This is not a feature
change: product behaviour, the catalog schema, and persisted formats stay as
they are.

## Why (measured 2026-10-19 on `dev` `551595db`)

- 72 Server packages, about 109k production lines (plus 31k sqlc-generated
  and 46k test lines). Three packages hold 45% of the production code:
  `internal/service` (20,050 lines in 39 files: auth, MFA, passkeys, users,
  settings, setup, assets, albums, stacks, duplicates, faces, places, species,
  music, share links, embeddings, OCR, classification), `internal/api/handler`
  (17,706 lines, 46 files), and the `internal/storage` root (11,825 lines,
  49 files).
- The eleven Atlas groups form an almost linear chain, so one feature crosses
  most of them: face code appears in 10 of 11 groups.
- `ingest → service` exists because the asynchronous result writers live in
  `service` (`ApplyEmbeddingTx`, `ApplyOCRResultsTx`,
  `ApplySpeciesPredictionsTx`, `PreparedReindex`,
  `PreparedLocationResolution`, …), which the commit coordinator calls;
  `queue` and `api/dto` import `service` types as well.
- Work machinery sits at the top level under seven near-synonymous names
  (`pipeline`, `processing`, `processors`, `queue`, `queue/jobs`,
  `execution`, `workqos`).
- `architecturecheck` mixes real invariants with text snippets of specific
  lines; the audit is in [atlas-rules.md](atlas-rules.md).
- The OpenAPI contract for 257 operations is about 2,200 lines of swag
  comments (`@Failure` 516, `@Param` 396, `@Success` 260, `@Router` 257)
  that the compiler never checks. swag v2 is still a release candidate
  (`v2.0.0-rc5`), and `tools/openapinormalize` rewrites its output afterwards,
  choosing each Problem vocabulary from hard-coded path tables.
- Generated rows reach every layer (measured 2026-10-28 on the working tree
  at `781515c7`). Non-test references to `repo.*` outside `internal/db`:
  `service` 849, `storage` 512, `api/handler` 200, `agent` 160, `cloud` 95.
  Five handlers hold `*repo.Queries` directly (`asset`, `asset_lifecycle`,
  `album`, `duplicate`, `share_link`), and `api/dto` converts generated rows
  (`ToAssetDTO(repo.Asset)`, `ToAssetDetailDTO(repo.GetAssetWithRelationsRow)`,
  `ToShareLinkDTO(repo.ShareLink)`), so a query change is an API change.
  About 250 hand-written SQL call sites through `catalogtx` live outside
  `internal/db` (`event` 65, `storage` 31, `commit` 30, `service` 28,
  `pipeline` 18, `api/handler` 18, `processing` 13, `queue` 11, …), and
  `internal/queue` builds the `riversqlite` River driver itself.

## Non-goals

- Product behaviour, the catalog schema and migrations, persisted formats,
  and the TOML manifest. A session that would need to change one of them has
  found a bug or a design question: stop and take it to the owner.
- Splitting the sqlc package. `catalog/repo` stays one generated package;
  the Atlas table-ownership rule replaces per-capability query packages.
- A database engine abstraction. SQLite stays the only engine: the baseline
  has 91 STRICT tables and FTS5 and vec1 virtual tables; the 69 query files
  use `unixepoch` 156 times, `RETURNING` 111, and `json_extract` or
  `json_each` 103; River runs on `riversqlite`; backups use the SQLite online
  backup API; and the `catalogtx` single-writer model is SQLite's.
  `v1.0.0-beta` was the PostgreSQL server and was left on purpose. The
  database boundary confines the engine; it does not abstract it.
- Repository interfaces in front of sqlc. A capability calls its queries
  directly; an interface with one implementation is a seam with no consumer,
  and tests already run on real SQLite.
- Web and Desktop structure. `web/ARCHITECTURE.md` and
  `web/scripts/check-source-boundaries.ts` keep governing the Web.
- Whether Agent tools should read through capability APIs instead of raw
  queries (owner scope). It is a separate design question for the owner.
- The storage lifecycle work (completed 2026-10-26; decisions
  [command boundary](../../../.agents/decisions/2026-10-26-storage-command-boundary.md), [volume semantics](../../../.agents/decisions/2026-10-26-repository-volume-semantics.md), [recovery registry](../../../.agents/decisions/2026-10-26-executable-recovery-registry.md));
  restructuring the storage root was deferred until it completed and now
  waits only for the rc.1 tag.
- Replacing gin. Huma runs on the existing engine through `humagin`; moving
  to the standard library router (`humago`) is a separate later decision.

## Fixed contracts

- **Timing (owner decision 2026-10-28, replacing "Phase B after rc.1" of
  2026-10-19).** Every session of this plan lands before `v26.1.0-rc.1` is
  tagged, under the fix-before-release rule in [rc-release.md](rc-release.md).
  The order of Phase B relative to the open storage plans
  ([repository-layout.md](repository-layout.md),
  [repository-access-simplification.md](repository-access-simplification.md),
  [portable-asset-sidecars.md](portable-asset-sidecars.md)) is fixed in
  [rc1-sequence.md](rc1-sequence.md), because import-path churn conflicts
  with every open storage branch: sessions run strictly one after another,
  B1–B2 before the storage plans and B3–B14 after them.
- **One session, one PR into `dev`**, branched from the latest `dev`, with
  `task test` and `task verify:generated` green and the narrowest extra checks
  from `lumilio-select-checks` (Desktop compiles the Server in-process, so a
  moved `server/app` dependency needs `task desktop:test`).
- **Behaviour evidence.** Every Phase B PR body carries the
  `atlas movecheck --base origin/dev` report (built in A4): each function
  whose normalized body changed is listed with a one-line reason. A move
  session reports zero body changes beyond the fixes this plan names.
  Transport sessions (B10–B11g) rewrite handlers by design; their behaviour
  evidence is the contract diff, the API tests that drive the router
  (`ServeHTTP`), the route contract tests in `internal/api`, and the E2E
  slice of each migrated capability.
- **OpenAPI.** The exported spec stays at `server/docs/swagger.yaml` and
  `swagger.json`, because Atlas `api:` anchors,
  `web/scripts/generate-openapi-types.ts`, and Redoc read it. Until B9 the
  spec and `web/src/lib/http-commons/schema.d.ts` must not change, and
  packages whose name appears in a schema name (`api`, `dto`, `handler`,
  `processing`, `dbtypes`, `event`, `inject`, `core`) keep their package name
  even when their directory moves. B9 changes schema names and nothing else.
  From B10 on, the semantic contract diff against the PR base
  (`tools/openapi diff`, built in B9) is empty except for entries the PR body
  lists under "Contract changes", each with its reason, and each of one of
  three kinds: a field's required flag corrected to what the handler actually
  sends or accepts, a request body closed with `additionalProperties: false`,
  or a parameter constraint the handler already enforced in code. Paths,
  methods, status codes, media types, Problem types, and schema names never
  change.
- **No shims.** No type aliases, forwarding functions, or re-export packages
  are left behind; every caller moves in the same PR.
- **Database boundary (2026-10-28).** Each rule is a type-resolved Atlas rule
  from the session named:
  1. Engine drivers (`github.com/mattn/go-sqlite3`, `riversqlite`) and
     `*sql.DB` appear only under `catalog/`; `catalog` builds the River
     driver and hands it to `ingest/queue` (B2).
  2. Nothing under `api/` and nothing in `server/app` imports `catalog/repo`
     or calls a `catalogtx` SQL method; transport maps capability types only
     (each B11 session for its capability; all of `api/` at B11h).
  3. No exported function, method, field, or type in a capability's root
     package names a `catalog/repo` type; rows become capability types in
     unexported mappers inside the capability (B13). Until a capability's
     B13 session lands, its count of such exported mentions is a ceiling in
     `atlas.yaml` that may only fall.
  4. Hand-written SQL through `catalogtx` is allowed only in `catalog`,
     `work`, capabilities, and `ingest`, on tables they own; its call-site
     count per package is a ceiling recorded at B2 that may only fall. New
     SQL is a sqlc query unless it is dynamic (for example
     `search/filter_sql.go`).
  5. `catalog/repo` imports only `catalog/dbtypes` and leaf packages that
     import no other `server` package; this one rule replaces the
     per-package `exceptions` in `atlas.yaml` (B13).
- **Directory is group.** From B1 on, a Server package's Atlas group is
  derived from its path; `//atlas:group` directives are removed in A1.
- **Docs travel with code.** A moved package keeps its `doc.go`; Atlas view
  anchors, `docs/architecture.md`, the `CLAUDE.md` repository map, and skills
  that name moved paths are updated in the same PR. Stale views are re-read
  before `task atlas:lock`.

## Target structure (frozen 2026-10-19, revised 2026-10-28; the owner may rename in review)

```
server/
├─ cmd/ app/ config/ migrations/   unchanged (Desktop imports only app and config)
└─ internal/
   ├─ foundation/  logging version secretbox settings memory sysproc fsprivacy testutil
   ├─ media/       exif raw imaging imagesource hash phash geohash file   ← utils/*
   ├─ catalog/     (root: was db; builds the River driver) catalogtx repo dbtypes backup
   │               vectorindex vec1ext sqliteuri conformance   ← db/*, platform/sqliteuri, sqlitespike
   ├─ work/        pipeline jobs execution processing qos   ← pipeline queue/jobs execution processing workqos
   │
   │  business capabilities (declared DAG; never import ingest, api, net/http, or an engine driver)
   ├─ assets/      assets, browse, albums, folder tags, featured, asset search
   ├─ stacks/      ← service/stack_service
   ├─ duplicates/  ← service/duplicate_service
   ├─ search/      retriever fusion, bleveocr/, classify math, indexing/ (embeddings, OCR, classifier, AI tags)
   ├─ people/      ← service/face_*
   ├─ places/      ← service/location_service
   ├─ species/     ← service/species_*
   ├─ music/       ← service/music_service
   ├─ sharing/     ← service/share_link_service
   ├─ auth/        ← service/auth_* user_* totp credential_policy user_roles
   ├─ system/      settings, setup, bootstrap, backup orchestration
   ├─ lumen/       ← service/lumen_service (ML client)
   ├─ storage/     unchanged until B12 (+ artifact/ moves in)
   ├─ event/ lifecycle/ agent/ llm/   unchanged
   │
   ├─ ingest/      queue processors commit sourcing upload cloud
   └─ api/         router, middleware chain, and SPA fallback (root); httpx/ (Huma config,
                   Problem mapping, principal, body limits, stream helpers); problem,
                   ratelimit, origin, transport; and one transport package per capability
                   (Huma operations + their request/response types, importing huma, never
                   gin) after B11
```

Layers, lowest first: `foundation` → `media` → `catalog` → `work` →
capabilities → `ingest` → `api` → `app`. A capability's root package is its
public API: a `Service` type without stutter (`people.Service`, not
`people.FaceService`), its domain types and errors, and the `…Tx` writers
the commit coordinator calls. Private parts go under `<capability>/internal/`
so the Go compiler, not a tool, enforces the boundary. Generated rows stay
inside: the capability maps them to its own types in unexported functions
(B13), so its exported API never names a `catalog/repo` type. Expected capability
edges from today's code: `system → auth`; `places → system`;
`search → lumen, system`; `assets → search, music, lumen, system, storage`;
`stacks → duplicates`; `sharing → assets, agent`;
`agent → search, event, llm`; `storage → lifecycle → event`. Atlas derives
the real graph; an edge outside this list is reviewed, not assumed.

## Decisions (frozen 2026-10-19)

| Decision | Chosen | Alternatives and why they lost |
| --- | --- | --- |
| Package axis | By business capability, with a few infrastructure layers | By kind (handler/service/utils, today): every feature crosses most layers and catch-all packages grow without bound |
| HTTP transport | `api/<capability>` packages over a shared `api/httpx` | Handlers inside each capability: pulls `net/http` and the Problem vocabulary into domain code. One handler package (today): 17.7k lines, boundaries invisible |
| HTTP contract | Huma v2 operations on the existing gin engine (`humagin`); the request and response Go types are the contract | swag annotations (today): 2,200 comment lines the compiler never checks, a release-candidate generator, and a post-processor for RFC 9457. Spec-first generators (ogen, oapi-codegen): a hand-maintained spec of 465 schemas, the opposite of Go types as the source. Huma on `net/http` now: replaces the gin middleware in the same change; it can be done separately later |
| OpenAPI schema names | Package-free names equal to the Go type name. B9 drops the prefix in the spec tool (465 schemas, no collision measured); Huma's default namer yields the same names, so B10–B11 rename nothing; Huma panics on two types with one name, so export fails | Package-derived names (today): every package move is an API change; the Web names them 228 times in 80 files besides the generated `schema.d.ts`. swag `//@name` on every type: 465 annotations that B11 deletes again |
| Read/write separation | Typed handles: constructors take `catalogtx.Reader` or `catalogtx.Writer` (both exist today), never `*sql.DB` | Snippet checks on `app.go` wiring (today): bypassed by a renamed variable; behaviour tests alone catch it only at runtime |
| Query layer | One sqlc package plus Atlas table ownership | sqlc per capability: duplicates shared types and generated helpers for no gain |
| Migration style | Incremental sessions with no compatibility shims | Big-bang move: unreviewable diff; shims: dead seams that outlive the move |
| Database engine (2026-10-28) | SQLite only, confined to `catalog` by the database boundary | A dialect layer for SQLite and PostgreSQL: lowest-common-denominator SQL in place of FTS5, vec1, JSON functions, and `RETURNING`, plus a second River driver and backup path, for an engine the product left in `v1.0.0-beta` |
| Generated rows (2026-10-28) | Mapped to capability types inside the owning capability; never in an exported capability API or under `api/` | Status quo: transport binds to generated rows, so a query change is an API change. Repository interfaces per capability: one implementation each, and tests already run on real SQLite. One domain-type pass over every capability: unreviewable; ceilings that only fall let it land one capability at a time |
| Reach of gin (2026-10-28) | The `api` root package, `api/httpx`, and `server/app` | All of `internal/api`: every transport package would bind to gin, so a later move to `humago` would touch each one |
| `dbtypes` (2026-10-28) | Split at B13: column codecs (`Timestamp`, `JSON`, `UUIDs`, `Strings`, `Vector`) stay in `catalog/dbtypes`; product vocabulary (`AssetType`, per-type metadata, `StackKind`, `RepoRole`, storage enums, ML payloads) moves to leaf packages named by the owning capability, as `storage/repocfg` is today | One package mixing SQL codecs with product vocabulary. Vocabulary in capability root packages: `catalog/repo` would import them, and they import `catalog/repo`, a cycle |

## HTTP layer on Huma (frozen 2026-10-19; checked against the Huma v2.39.1 source)

- **Adapter.** `humagin.New` over the existing gin engine. The gin
  middleware (authentication, origin policy, rate limits, the initialization
  and agent gates) keeps running in front of every operation. B10 adds
  `github.com/danielgtaylor/huma/v2` v2.39.1 and moves gin from v1.10.0 to
  v1.12.0, which `humagin` requires; the Desktop module follows. Only the
  `api` root package (engine, middleware chain, SPA fallback), `api/httpx`,
  and `server/app` import gin or `humagin`; `api/<capability>` packages
  import huma only, so moving to `humago` later changes those three places.
- **Config.** `huma.DefaultConfig` without its create hook, which would add
  a `$schema` field and a `Link` header to every response. `OpenAPIPath`,
  `DocsPath`, and `SchemasPath` stay empty until B11h.
- **Schemas.** The default namer (type name, no package) and the default
  strictness: object schemas get `additionalProperties: false`, and a field
  is required unless it has `omitempty`, `omitzero`, or `required:"false"`.
- **Errors.** `problem.Failure` implements `huma.StatusError` and serialises
  as `problem.Details` with `application/problem+json`. `api/httpx` replaces
  `huma.NewError` and `huma.NewErrorWithContext`, so the errors Huma raises
  itself (validation, body too large, unsupported media type) keep today's
  Problem shapes and statuses: a validation failure is the `about:blank` 400,
  not Huma's 422. Operations declare their error responses from
  `problem.Descriptor` values through `httpx`, as `Responses` rather than
  `Errors`, so Huma adds no automatic 422 or 500. This replaces the path
  tables in `tools/openapinormalize`.
- **Principal.** The authentication middleware stores the principal in the
  request `context.Context`; handlers read it through `httpx` instead of
  `c.Get("user_id")` or `CurrentUser(c)`.
- **Body limits.** Huma defaults every operation to `MaxBodyBytes` 1 MiB and
  `BodyReadTimeout` 5 s. `httpx` sets both explicitly from today's effective
  limits: an operation with no limit today gets none (`-1` disables either),
  and operations that receive files read the body as a stream.
- **Streams.** SSE (agent, uploads), multipart uploads, the backup stream, and
  Range responses through `http.ServeContent` are Huma operations with
  hand-written request and response documentation and a `huma.StreamResponse`
  body, so they stay in the spec. Only `httpx` calls `humagin.Unwrap`; its
  stream helpers hand transport packages the request and writer. The
  SPA fallback (`api/spa.go`) stays a plain gin route.
- **Spec export.** `task server:openapi` runs `go run ./tools/openapi`, which
  builds the router in registration-only mode (a transport package's
  `Register` must not touch its dependencies) and writes the spec. Until B11h
  it also runs swag and merges the two documents; an operation or schema name
  defined by both fails the export.
- **Tests.** 25 of the 34 test files under `internal/api` call handlers
  through `gin.CreateTestContext`; each session rewrites its files to drive
  the router or `humatest`. Tests that already go through `ServeHTTP` keep
  running unchanged.

## Measured split of `service` (2026-10-19)

A type-resolved file analysis (`go/packages` + `go/types`; it becomes
`atlas split-report` in A4) of the 39 files found 60 file-to-file edges:

| Capability | Files | Lines |
| --- | --- | --- |
| assets | `asset_service` `asset_browse_service` `asset_activation` `asset_folder_tag_service` `asset_search_fused` `asset_search_visual` `album_service` `featured_selector` | 4,250 |
| stacks | `stack_service` | 1,103 |
| duplicates | `duplicate_service` | 1,168 |
| search/indexing | `embedding_service` `embedding_canonical` `indexing_service` `indexing_runtime` `ocr_service` `classifier_service` `ai_tag_service` | 2,376 |
| people | `face_service` `face_clustering` `face_carryover` | 2,036 |
| places | `location_service` | 1,271 |
| species | `species_service` `species_reference_service` | 438 |
| music | `music_service` | 2,132 |
| sharing | `share_link_service` | 493 |
| auth | `auth_service` `auth_mfa` `auth_passkeys` `auth_password_change` `credential_policy` `totp` `user_roles` `user_service` | 3,227 |
| system | `settings_service` `setup_service` `bootstrap_service` `backup_service` | 1,177 |
| lumen | `lumen_service` | 336 |

- `asset_search_fused.go` and `asset_search_visual.go` are methods on
  `assetService`; they belong to `assets`, not `search`.
- One cycle (`search ↔ assets`), caused only by the sentinel
  `ErrSemanticSearchUnavailable`, declared in `asset_service.go` and used by
  `embedding_service.go` and `classifier_service.go`. It moves to
  `search/indexing`.
- Unexported symbols crossing capabilities: `canonicalizeSemanticVector`
  (assets → indexing; export it), `normalizeStoredGeocoding`
  (places → system; export it), `optionalStringFromSQLiteUUID`
  (people → assets; move to `catalog/dbtypes`), `derefFloat64`
  (indexing → people; local copy), `detectionOwnerKey`
  (stacks → duplicates; export it).
- Four of the 35 test files span capabilities:
  `asset_service_semantic_test`, `face_carryover_test`,
  `location_service_test`, `setup_service_test`. They move with the dependent
  capability.
- Outside importers to update: `server/app`, `api/dto`, `api/handler`,
  `commit`, `queue`.

## Measured shape of the storage root (2026-10-19, input to B12)

One type, `DefaultRepositoryManager`, spans most of the 49 files (161 file
edges). The operation journal (`lifecycle_operation`, `lifecycle_audit`,
`operation_admission`) is its centre: the rest of the package uses 43 of its
unexported symbols, and it calls back into the manager. The host filesystem
facts (`storage_path_info_*`, `file_identity_*`, `capacity_*`, `dirmode`,
`repository_os_lock*`, `staging_durability_*`) are the cleanest leaf: they
reach back for three unexported helpers (`decodeMountInfoPath`, `queries`,
`uniqueStrings`) and four exported names. B12 re-runs the report on the
post-plan code before choosing the cut.

## Execution phases

| # | Session | Plan |
| --- | --- | --- |
| A1–A4 | Atlas becomes the only architecture gate | [atlas-rules.md](atlas-rules.md) |
| B1 | Infrastructure moves: `utils/*` → `media/` (`memory`, `sysproc` → `foundation/`, `upload` → `ingest/`); `db/*` → `catalog/` (`db` root becomes package `catalog`, `sqlitespike` becomes `conformance`); work packages → `work/` (`workqos` → `work/qos`); `queue` `processors` `commit` `sourcing` `cloud` → `ingest/`; `httporigin` `servertransport` → `api/`; `artifact` → `storage/artifact`; `logging` `version` `secretbox` `settings` `testutil` → `foundation/`; `platform/fsprivacy` → `foundation/`, `platform/sqliteuri` → `catalog/` (only `db` imports it), and `server/platform` is gone. Import paths only, apart from the `catalog`, `conformance`, and `qos` package renames. Groups match paths in `atlas.yaml` | this plan |
| B2 | Typed catalog handles: `catalogtx.Reader` and `catalogtx.Writer` in every constructor that takes `*sql.DB` today; the wiring snippet checks left after A3 become compile errors and are deleted. `catalog` builds the River driver that `queue_setup.go` builds today. Database boundary rule 1 becomes an Atlas rule, and the rule 4 call-site ceilings are recorded | this plan |
| B3 | `music`, `species` out of `service` | this plan |
| B4 | `auth` | this plan |
| B5 | `system`, `lumen` | this plan |
| B6 | `places`, `people` (`optionalStringFromSQLiteUUID` → `catalog/dbtypes`) | this plan |
| B7 | `search/indexing` (moves `ErrSemanticSearchUnavailable`; merges `classify`) | this plan |
| B8 | `duplicates`, `stacks`, `sharing`; the remainder becomes `assets`; `internal/service` is deleted | this plan |
| B9 | OpenAPI schema names and the spec tool: `tools/openapi` replaces `tools/openapinormalize` (same Problem schemas), drops the package prefix from every schema name, fails on a collision, and gains `diff --base <ref>`, a semantic contract diff that proves this PR renames only. One Web codemod over the 228 hand-written `components["schemas"]["pkg.Name"]` references | this plan |
| B10 | Huma foundation ("HTTP layer on Huma"): `api/httpx` (Huma config, Problem mapping, principal, body limits, Problem response declarations, session cookies, pagination parsing); the spec tool merges Huma operations into the swag document; router split by capability; Atlas rules confine Huma imports to `internal/api` and `server/app`, and gin and `humagin` imports to the `api` root package, `api/httpx`, and `server/app`. Pilot, 11 operations: `api/system` (health, setup, settings, capabilities), `api/species`, `api/places` | this plan |
| B11a–g | One transport package per capability on Huma (operations and their request/response types), one PR each: (a) `auth` with users, MFA, passkeys, sessions, 28 operations; (b) `music`, `sharing`, 40; (c) `people`, `stacks`, `duplicates`, `search` (indexing, classifier), 29; (d) `assets` JSON operations (assets, albums, metadata, lifecycle, stats), 40; (e) `assets` streams (query, media Range, upload with SSE), 25; (f) `storage` (repositories, scans, host actions) and backup into `api/system`, 34; (g) `cloud`, `event`, `agent` (SSE, pins), queue into `api/system`, 50 | this plan |
| B11h | swag is deleted: `swaggo/swag`, `gin-swagger`, `swaggo/files`, `server/docs/docs.go`, the general-info annotations in `cmd/main.go`, the `tools:swag` target, and the merge step. `api/handler` and `api/dto` are gone. Huma serves the live spec and docs in place of `/swagger/*any`. Database boundary rule 2 holds for all of `api/` and `server/app`. `CLAUDE.md`, `CONTRIBUTING.md`, `lumilio-api-contract-change`, and the developer OpenAPI page in `site/docs` describe the Huma workflow | this plan |
| B12 | Storage root split, cut chosen from `atlas split-report` (host filesystem facts first) | this plan |
| B13 | Generated rows leave capability APIs (database boundary rule 3), one PR per capability, after B9 and, for `storage`, after B12: domain types and unexported row mappers in the capability, every importer updated, its ceiling set to zero. The first PR splits `dbtypes` (codecs stay in `catalog/dbtypes`, vocabulary moves to capability-named leaf packages) and replaces the `atlas.yaml` exceptions with rule 5 | this plan |
| B14 | Tighten size ceilings to the new reality; extract decisions; complete both plans | this plan |

Recipe for each `service` session (B3–B8):

1. Run `atlas split-report internal/service`; confirm the capability's files
   have no cycle with the remainder. A new cycle is a design question.
2. Move the files to `internal/<capability>/` with `package <capability>`,
   rename stuttering types and constructors (`FaceService` → `Service`,
   `NewFaceService` → `New`), and apply only the fixes this plan lists.
3. Move the tests with their files; a spanning test goes with the dependent
   capability.
4. Update every importer; declare the capability's edges and size ceiling in
   `atlas.yaml`.
5. Attach the movecheck report to the PR.

Recipe for each transport session (B10–B11g):

1. Create `api/<capability>` with `Register(api huma.API, deps)`. Move the
   capability's request and response types out of `api/dto` and
   `api/handler` into it, keeping their Go type names, which are now the
   schema names.
2. Rewrite each handler as `func(ctx, *Input) (*Output, error)`, delete its
   swag annotations, and declare its Problems, body limits, and security
   through `httpx`. The handler calls its capability, never `catalog/repo` or
   a `catalogtx` SQL method; the capability's `api/dto` converters from
   generated rows are deleted, and response types map from capability types
   (database boundary rule 2).
3. Rewrite the capability's `gin.CreateTestContext` tests to drive the
   router or `humatest`.
4. Run `task dto`. Put the `tools/openapi diff --base origin/dev` output in
   the PR body, list every contract change with its reason, and run the
   capability's E2E slice.

## Validation boundaries

- `internal/service`, `internal/utils`, `internal/db`, `internal/api/handler`,
  `internal/api/dto`, `server/platform`, `server/tools/architecturecheck`,
  `server/tools/openapinormalize`, and `server/docs/docs.go` no longer exist;
  no swag annotation or `swaggo` module remains.
- The database boundary holds: no package outside `catalog` imports an
  engine driver or holds `*sql.DB`; nothing under `api/` and nothing in
  `server/app` imports `catalog/repo`; every B13 ceiling is zero;
  `catalog/repo` imports only `catalog/dbtypes` and leaf packages, and
  `atlas.yaml` has no `exceptions`; no package's hand-written SQL call sites
  exceed its B2 ceiling.
- Only the `api` root package, `api/httpx`, and `server/app` import gin or
  `humagin`.
- Every Server package's Atlas group is its path. The layer graph is the
  eight layers above plus a declared, acyclic capability graph, with every
  edge used.
- No capability imports `ingest`, `api`, or `net/http`; no constructor
  outside `catalog` takes `*sql.DB`.
- No package exceeds its recorded size ceiling; a new package's ceiling is
  5,000 production lines.
- OpenAPI schema names equal Go type names with no package prefix. Every
  operation, streams included, is registered through Huma. Apart from the B9
  rename and the contract changes listed in B10–B11g PR bodies, the contract
  is identical to the base of each PR.
- `task test`, `task verify:generated`, `task desktop:test`, and the E2E
  slices are green on the final head; every cross-cutting invariant in
  `docs/architecture.md` names a rule or test that Atlas resolves.
