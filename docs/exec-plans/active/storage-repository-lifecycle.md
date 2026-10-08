# Storage Location and Repository lifecycle

Status: active, created 2026-10-07; principal owner decisions frozen
2026-10-07. P0 merged as PR #246. P1 lands in this PR: observation and
assessment core, stat panic fix, guarded atomic marker primitive, and
swappable lock/identity/watch adapters. P2–P8 remain outstanding. Each fixing
PR lands its regression tests and updates this Status and the evidence tables.
Child of [release-hardening.md](release-hardening.md).

Goal: within the rc.1 storage support policy, creation on a removable
non-default Storage Location completes; every offered Reconnect reaches
success or specific legal recovery; reinstall with a retained primary
offers explicit adoption or a fresh destination. One Server assessment
owns facts, consent, capabilities, and phase-aware recovery across Web and
Desktop. Originals and private user data remain recoverable when the
filesystem changes between review and execution.

## Non-goals

- P0 changes no production Go, Web, Desktop, API, schema, CI, or product
  behavior. It introduces no failing, skipped, or quarantined regressions.
- Managed movement/copying between volumes, arbitrary primary replacement,
  in-place reset, distributed ownership, or network support expansion.
- Pre-rc.1 catalog/config migration. Accepting portable markers does not
  recover albums, ratings, People, shares, or other catalog-only metadata.
- Replacing RepositoryFS, access barriers, the catalog writer, scan/Trash
  engines, or lifecycle journals.

## Fixed contracts

- **Originals and ownership:** disk is authoritative for original media;
  registration never rewrites, relocates, or deletes it. Catalog owns roles,
  registration, authorization, Asset bindings, and durable work; QueueDB is
  disposable delivery state. No filesystem I/O inside catalog transactions.
- **Storage Locations authorize destinations:** a registered child's I/O,
  rename, and detach never depend on parent marker health. Create/open and
  reconnect still require an authorized direct-child destination. Exactly
  one default and one primary; primary is fixed at `<default>/primary`.
- **Configuration:** `storage.path` is runtime-immutable in the complete
  schema-versioned TOML manifest. Default changes are configuration
  transactions with controlled restart, never ad hoc runtime overrides.
- **Compatibility:** portable marker `"1.0"` is accepted regardless of which
  prerelease wrote it. Corrupt/partial/newer markers are preserved unchanged.
  Catalog lineage and configuration compatibility are separate startup
  conditions. Before the rc.1 tag, amend baseline version 1 in place; after
  the tag, use forward steps. See the
  [compatibility decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md).
- **One authority:** read-only observation feeds a pure Server capability
  policy. Orthogonal facts include identity, registration, compatibility,
  layout, availability, ownership, support, backing storage, and volume
  semantics; unknown never means safe. Ordinary views use timestamped
  projections; explicit assessment samples current facts. Existing targets
  use their own filesystem; absent children use the nearest proven ancestor
  and are rechecked after creation. Continuity compares each identity with
  its own prior observation, independently of capacity grouping.
- **Consent and execution:** token-bound per-operation acknowledgement
  covers target identity, intent, risk vocabulary, relevant marker/mount
  facts, and required acknowledgements; capacity fluctuation alone does not
  invalidate it. Authorization is not consent. Commands replay idempotency,
  acquire barriers in stable order, refresh catalog and rooted markers,
  reassess, validate token/acknowledgements, claim ownership and explicitly
  probe writability/materialization, journal, mutate outside transactions,
  commit registration/audit/scan fences, then return a receipt. Rename loads
  row and complete marker after the lease and changes only the raw name;
  guarded atomic writes and rollback never overwrite a replaced identity.
- **Read-only means no probes:** assessment/setup/candidates/status do not
  create permission/case probes, lock files, reconciliation writes, or expiry
  work. Deep materialization checks are bounded, cancellable preflight.
- **Native trust:** Web owns journeys; Desktop supplies selection, trusted
  approval, nonce/version checks, reveal, and Save/Apply/restart; Server owns
  policy and commands. Selected host paths/nonces stay outside HTTP DTOs.
  When the Server cannot start, a small Desktop presenter consumes the same
  server-owned Go facade; standalone uses concrete operator instructions.
- **Recovery is executable:** distinguish beginning a journey from executing
  a transition. Capabilities appear only when their command and presenter
  exist. Closed action descriptors carry kind, actor/phase, typed subject,
  endpoint/host entry, required input, assessment revision, and reason code.
  Transport attaches endpoints; domain policy does not know Problem URIs.
  Generated contracts and en/zh presenters remain exhaustive.

Bootstrap advances catalog-ready → admin-created → explicit primary
create/adopt → ready. Ready stays ready when a disk disappears. Registration
advances unregistered → registered → detached (regular only), with explicit
open to reattach; interrupted mutations enter recovery-required and return
through journal recovery. Availability is independent of both machines.

## State / action / recovery matrix

This is the target contract, not a claim of current behavior. Detailed setup
facts require an authenticated administrator after owner creation; public
setup status stays phase-only. Proposed recovery entries below are semantic
entries to implement in the stated phases. Before HTTP can start, use the
shared native/operator presenter. Evidence means a passing test/PR or native
verdict; **designed** is not passing evidence. The evidence column is a
progress record, not an additional merge gate. R IDs refer to the regression
designs below.

| State / context                                                             | Legal action or transition                                                                                         | Recovery / reachable entry                                                         | Evidence / landing phase                            |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- | --------------------------------------------------- |
| New target or allowed empty target                                          | Review Create, then explicit writable/ownership preflight                                                          | Current create journey; Docker requires an existing empty target to be a mounted volume        | P1 atomic failures pass; R1 command review P2                             |
| Removable/non-default/new child or independent child mount                  | Assess actual target; acknowledge returned risks for that command                                                  | Shared risk review, including authenticated primary creation                       | P1 injected target/child classification pass; R1/P2 review pending                          |
| Compatible unregistered regular / detached regular                          | Explicit Open and registration, retain UUID and private data                                                       | Candidate or native Open journey; disclose lost catalog metadata                   | R4, R9 designed; P4, P6                             |
| New catalog, compatible root + fixed primary, readable originals            | Explicit Adopt Primary with current Host Owner, or fresh elsewhere                                                 | Authenticated setup adoption/selection without completed-setup gate                | R3 designed; P5                                     |
| Catalog absent, markers/originals retained                                  | Compatible backup restore or explicit rebuild via open/adopt                                                       | Setup disclosure; private recovery inventory                                       | R3, R4 designed; P4–P5                              |
| Pre-rc.1 catalog remains (`catalog_lineage_unsupported`)                    | Preserve/archive old app state; start supported fresh catalog                                                      | Startup native/operator guidance; never bypass compatibility                       | R3 variant designed; P5                             |
| Incompatible TOML/runtime intent (`configuration_incompatible`)             | Supply fresh complete manifest/intent, preserve media                                                              | Startup configuration presenter/operator guidance                                  | R3 variant designed; P5                             |
| Prerelease marker `"1.0"`                                                   | Same compatible open/adopt policy                                                                                  | No inferred previous installation or marker-era rejection                          | P1 typed 1.0 readers pass; R3 adoption P5       |
| New catalog; corrupt Location marker                                        | Restricted recovery runtime if catalog usable; no root provisioning                                                | Authenticated setup diagnose/fresh destination; native/operator fallback           | R3 variant designed; P5                             |
| Corrupt/partial or newer Location/Repository marker                         | No marker-write capability; preserve bytes                                                                         | Diagnose, compatible build, known-backup instructions, or fresh destination/import | P1 typed readings/guarded-byte preservation pass; P5 recovery pending |
| Missing marker; nonempty primary (`nonempty_unmarked`)                      | No initialization over tree                                                                                        | Fresh destination and explicit originals import; preserve old folder               | P0 unmarked fixture; R3 designed; P5                |
| Valid marker; missing `.lumilio/` or `inbox/` (`layout_repairable`)         | Explicit bounded repair/adopt, identity unchanged                                                                  | Repair review; file obstruction/denial is a blocker, never replacement permission  | P1 compatible identity + absent layout pass; P5 repair pending    |
| Registered readable identity                                                | Read/verify; upload subject to admission; regular rename/detach with fresh impact                                  | Storage journey and operation receipt                                              | R5, R8, R10 designed; P3                            |
| Registered original positively unavailable                                  | Retain catalog metadata and ready state; begin regular reconnect; regular detach remains possible                  | Select matching tree; primary uses default recovery                                | R2, R3, R8 designed; P5–P6                          |
| Registered marker missing/corrupt/different (`registered_identity_problem`) | No implicit replacement or marker overwrite                                                                        | Matching known tree/marker instructions, diagnose/retry                            | P1 guarded identity replacement pass; R2 reconnect P6                            |
| Same UUID elsewhere, original online (`identity_copy`)                      | Use existing registration; regular independent-copy review; no reconnect commit                                    | Current conflict journey; primary copy prohibited                                  | P0 same-UUID copy; R2, R10 designed; P3, P6         |
| Same UUID elsewhere, original unavailable                                   | Explicit regular reconnect or copy where permitted                                                                 | Primary routes to complete-default configuration recovery                          | R2 designed; P6                                     |
| Original permission denied/unknown                                          | No conclusion of offline; no reconnect commit based on denial                                                      | Access diagnosis and reassessment                                                  | P1 denied/unknown/nil stat tests pass; R2 reconnect P6                             |
| Read-only / permission-denied target                                        | Mutation blocked; retain identities/catalog facts                                                                  | Permissions, retry, writable supported destination, also pre-setup                 | P1 access distinction/read-only core pass; P2 transport pending                              |
| Owned elsewhere / busy                                                      | No conflicting mutation; unrelated healthy trees usable                                                            | Stop owner, wait/inspect operation, retry; cancel only if supported                | R10 designed; P3                                    |
| Unsupported network/remote filesystem                                       | Refuse create/open/reconnect before acknowledgement                                                                | Choose supported destination, with reason; confirmation cannot enable it           | P1 shared injected network classifier pass; P2 refusal wiring; #245 deferred                   |
| Cloud-backed resident tree / unavailable placeholder                        | Review risks; deep preflight checks residency                                                                      | Materialize locally and retry; denial has its own classification                   | P1 access distinction/read-only core pass; P2 transport pending                              |
| Private Trash/Studio/staging/unknown retained                               | Preserve by class; hold inherited Trash; rebuild/reassociate proven matches                                        | Reachable recovery inventory, review/export unresolved records                     | P0 private fixtures; R4 designed; P4                |
| Interrupted mutation (`recovery_required`)                                  | Recover journal before conflicting mutation                                                                        | Recovery receipt, diagnose/retry                                                   | R4, R5 designed; P3–P4                              |
| Unregistered native Location selection                                      | Review authorization; create root marker only when genuinely absent or register compatible marker                  | Native task review; stale/expired selection restarts without losing form           | R9, R10 designed; P3, P6                            |
| Registered external Location                                                | Create/open direct children; reconnect complete identity; detach only with zero children/no pending mutation       | Direct Location reconnect entry, all child markers revalidated atomically          | R2, R9 designed; P6                                 |
| Default, setup incomplete                                                   | Create/adopt fixed primary; choose fresh default only with no primary, no child registrations, no pending mutation | Desktop config selection/Apply or standalone manifest/restart; preserve old tree   | R3 designed; P5                                     |
| Default/primary, setup complete                                             | Identity-preserving whole-default reconnect through configuration Apply                                            | Default recovery handoff; no ordinary primary rename/detach/copy/reconnect         | R2, R10 designed; P3, P6                            |
| Parent marker unavailable/invalid, healthy registered child                 | Child read/rename/detach independently admitted; destination authorization still enforced                          | Parent diagnosis/reconnect separately                                              | P1 parent-independent child observation pass; P3 command boundary pending                                 |

### Error-to-recovery coverage

Every descriptor must be usable in the returned actor/bootstrap phase.
Returning an Open action behind the primary readiness gate is a failure.
Endpoints named here are proposed unless already present; no P0 API change.

| Problem / outcome                                              | Required action and entry                                                                                           | Evidence / phase  |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ----------------- |
| `storage/confirmation-required`                                | Review actual risks; retry current command with token/acknowledgements, also primary setup                          | R1; P2            |
| `repository/conflict` / identity registered elsewhere          | Retain target context; Open regular, Adopt Primary, regular copy/reconnect, or choose elsewhere according to matrix | R2, R3, R9; P2–P6 |
| `storage/assessment-stale`                                     | Refresh ordinary/setup assessment; review changed facts, preserve native task scope                                 | R1, R5; P2–P3     |
| `storage/action-unavailable`                                   | Show current blockers and legal alternatives; no automatic rejected-action retry                                    | R10; P3           |
| `storage/permission-denied`, `storage/read-only`               | In-place instructions, reassess, choose writable destination                                                        | R6; P1–P2         |
| `storage/observation-unknown` | Diagnose access/mount/attribute failure and reassess; no assumption of absence or mutation safety | R6/R7 P1 core; P2 transport |
| `storage/not-a-mounted-volume` | Mount a host directory in compose; preserve the existing Docker empty-folder rejection | `TestDockerEmptyTargetConstraint`; P1 code/copy, P2 transport |
| `storage/unsupported-filesystem`                               | Supported destination or deployment instructions, no confirmation bypass                                            | R7; P1–P2         |
| `storage/marker-invalid`, `storage/marker-version-unsupported` | Setup/storage diagnostics, compatible build, known-backup instructions, choose elsewhere; startup fallback          | R3, R12; P1, P5   |
| `storage/identity-mismatch`, `storage/original-online`         | Matching-tree selection or existing registration/regular copy review; no overwrite/primary copy                     | R2, R10; P3, P6   |
| `storage/ownership-unavailable`, `storage/busy`                | Owner/operation diagnostics, wait/retry, supported cancellation                                                     | R10; P3           |
| `storage/materialization-required`                             | Bounded offending-file facts; local residency then retry deep preflight                                             | R6, R7; P1–P2     |
| `storage/recovery-required`                                    | Setup/storage private-recovery inventory and journal receipt; retry recovery                                        | R4; P4–P5         |
| `storage/host-action-expired` / native selection cancelled     | New phase-scoped task / return to selection with form retained; cancellation is a normal outcome                    | R9; P3, P6        |
| Protected primary/default                                      | Default configuration handoff or standalone operator instructions                                                   | R2, R10; P3, P6   |
| Generic failure                                                | Receipt/diagnostics, refresh, explicit safe retry; do not invent a specific remedy                                  | R10; P3, P7       |
| Authentication/authorization/bootstrap failure                 | Sign in, authorized admin entry, or owner/primary setup; no host facts leaked                                       | R3, R10; P5, P7   |

## Decisions (frozen)

The five principal decisions are approved by Edwin on 2026-10-07 and frozen
in the [owner decision record](../../../.agents/decisions/2026-10-07-storage-refactor-decisions.md):
per-operation assessment-bound consent; explicit Adopt Primary or fresh
elsewhere; preserved private state with supported Trash/Studio recovery and
inherited Trash held; network storage unsupported in rc.1; Reconnect only,
never managed movement. NFS, SMB/CIFS, AFP, sshfs, WebDAV, and Windows remote
drives are refused upfront with the reason. P1 places repository
locking, move/identity detection, and change watching behind swappable
interfaces for [network support #245](https://github.com/EdwinZhanCN/Lumilio-Photos/issues/245).

Except for the resolved Docker empty-directory rule, remaining decisions use
the recommended **default, revisit when the blocking phase starts**. Revisiting a default does not reopen the five
approved decisions.

| Smaller decision                     | Default                                                                                                      | Blocking phase |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------ | -------------- |
| Pre-release compatibility            | No catalog/config migration; accept marker `"1.0"`                                                           | P1, P5         |
| Lost catalog metadata                | Compatible backup restore or explicit rebuild disclosure; no portable-data promise for albums/ratings/People | P4–P5          |
| Primary/default movement             | Fixed primary child; whole-default reconnect through Apply; no individual primary reconnect                  | P5–P6          |
| Fresh default during setup           | Only before primary completion with zero children/no pending mutation; preserve old tree                     | P5             |
| Path-selection ownership             | Web journey, Desktop selection/configuration, Server policy; shared native startup fallback                  | P2–P3, P5–P6   |
| Corrupt marker repair/reset          | No automated reset/overwrite; diagnose, known-backup instructions, fresh destination                         | P1, P5         |
| Primary rename                       | Prohibited by command and capability policy; reserved semantic name                                          | P3             |
| Permanently lost established primary | Protect identity; compatible backup or separate fresh app-state/catalog recovery                             | P5             |
| Existing empty directory             | **Resolved 2026-10-07:** retain mounted-volume constraint permanently for Linux/Docker; ordinary empty folders are ephemeral            | P1–P2          |
| Ambiguous Studio match               | Require proven content hash/size and valid source; review/export ambiguity, never filename guessing          | P4             |
| Inherited Trash retention            | Hold until restore/export/delete/explicit retention acceptance; new deletes use configured retention         | P4             |
| Unsupported volume semantics         | Refuse affected transition; extend support only with CI filesystem-image evidence                            | P6             |

## Execution phases

Every implementation PR targets `dev`, remains deployable, and updates this
plan. Tests that fail on current code land with their fixes; record red
output in that fixing PR. No skipped correctness tests. Contracts, commands,
presenters, and recovery entries ship together. The five principal decisions
are already approved; Edwin's phase reviews apply to the concrete changes.

### P0 — Freeze matrix and failures

- Deliver this condensed plan and the
  [filesystem fixture package](../../../server/internal/storage/testfixture/doc.go).
  Passing fixture self-tests prove reader compatibility and deterministic
  relative names, bytes, UUIDs, and mtimes across independent `t.TempDir()`
  trees. No mount/permission behavior is inferred from temporary paths.
- Dependencies: none. Risks: inaccurate defaults, nondeterministic fixtures,
  accidental runtime changes. Exit: matrix, defaults, R1–R12 designs, fixtures
  and required P0 checks pass; no runtime changes. Red tests are deferred to
  fixing branches, with no quarantine or failing tests on P0.
- Edwin: review this PR; provide R2 reproduction inputs. Smaller defaults
  remain revisitable at their blocking phase.

### P1 — Observation and assessment core

- Depends on P0. Typed marker/access observations, pure capabilities, one
  support classifier, read-only inspection, actual-target sampling, stat
  panic fix, guarded atomic marker primitive. Lock/identity/watch boundaries
  must remain swappable as their callers are cut over for #245.
- Risks: incidental transition changes; unknown treated as safe; Windows
  classification gaps. Exit: denied ≠ absent/placeholder, no nil panic,
  write-incapable observation, `"1.0"` readers, parent independence and
  target/continuity tests; Server and architecture gates green. Preserve
  transition outcomes; only the panic fix and Docker explanation change. Later
  phases wire support refusal into complete journeys.
- Edwin: PR review. Empty mount point versus empty folder is resolved: Linux
  ships only as Docker; retain the mounted-volume rule permanently. Windows
  facts are injected in tests; physical Windows validation is optional.

### P1 delivered / next cutover

P1 introduces `StorageObserver` / `OSStorageObserver`, `AccessReading`, typed
`marker.Reading` readers for both portable markers, `ObserveStorageTarget`,
`AssessStorageTarget`, `ClassifyStorage`, and `DeriveStorageCapabilities`.
Existing targets sample their own mount; absent children use the nearest
proven ancestor, never a denied/unknown ancestor. Layout and marker identity
remain separate; mount continuity compares the same subject across samples,
independently of capacity grouping. Registered child read/verify facts do not
consult parent marker health.

`marker.WriteAtomic` and both config types' `SaveGuarded` methods implement
same-directory temp/sync/guard/replace, directory sync where supported, and
refusal of changed complete bytes or unexpected identity. Directory-sync
failure may report the already-installed complete new marker. The caller must
hold its operation/ownership boundary; advisory locks cannot exclude external
applications between the last guard and rename. Legacy marker saves remain
until P3 refreshes identity under leases, so P1 changes no save admission.

`RepositoryLockProvider` / `OSRepositoryLockProvider` now carry runtime
ownership and local lock helpers. `RepositoryIdentityDetector` /
`LocalRepositoryIdentityDetector` carry rooted open/revalidation, Repository
and Location move/original-marker checks, reconciliation readers, and native
file identity; `scan.ChangeWatchBackend` / `scan.LocalChangeWatchBackend`
carry recursive watch start/stop. Alternatives are installed before serving.
The adapters retain current local locking, identity, retry and batching policy.
`TestRuntimeOwnershipUsesInstalledLockProvider`,
`TestRootedOpenUsesInstalledIdentityDetector`, and
`TestWatcherUsesInstalledChangeBackend` exercise replacement at those boundaries.

P1 validation: Go build/vet, touched-package tests, the complete `server:test`,
`server:test:concurrency:ci`, focused watcher race tests, and architecture/Atlas
checks and generated freshness pass. Existing
create/open/reconnect/setup acceptance tests retain their expectations; no
legacy admission branch is changed except handling the nil-stat panic. No new
test is skipped or quarantined.

P2 must expose the assessment through authenticated bounded DTOs/endpoints,
wire the shared support refusal and conditional mutation preflight into Create,
bind consent and provide en/zh presenters. Existing setup/candidate/host
inspection still uses legacy probes/classification where switching would change
admission or projected facts; R7 transport coverage remains P2/P6. P3 applies
atomic saves to refreshed command identities; P5 implements explicit layout
repair. No assessment DTO, endpoint, consent token or UI ships in P1.

### Facts/clarifications 2026-10-07

macOS and Windows ship the Desktop App plus Web App. Linux ships only the
Docker app (Ubuntu-based image); there is no native Linux desktop. The
**Existing empty directory / Linux empty mount point versus empty folder**
question is resolved permanently: an existing empty bind-mounted volume is
allowed; an ordinary existing empty folder is rejected because it belongs to
the container's ephemeral writable layer. New target behavior is unchanged.
The core defines `storage/not-a-mounted-volume`; the existing server rejection
now explains: “This folder isn't a mounted volume; data stored here would be
lost when the container is recreated. Mount a host directory in your compose
file.” P2 wires the typed code into DTO/Problem and en/zh UI contracts.

### P2 — Create contract and shared review

- Depends on P1. Assessment DTOs/endpoints, token acknowledgement binding,
  capability-backed Create including setup, shared en/zh risk review, real
  parent conflict continuation. Backend/Web ship together.
- Risks: contract drift, overstrict capacity fingerprint or weak identity
  binding. Exit: R1 and conflict flow green with native availability and no
  candidates; consent property tests; confirmation Problem contract;
  generated contracts/freshness and localized warning coverage green.
- Edwin: PR review; revisit selection ownership only if needed. #242 closes
  on CI flow + injected-target Server evidence; hardware is not a phase gate.

### P3 — Safe command boundary

- Depends on P1–P2. Reassess mutations under barriers, rename refresh,
  role-aware actions, native selection/review, domain-specific failures,
  idempotency and receipts. Keep barriers, catalog boundaries, scan fencing.
- Risks: races, deadlocks, replay errors. Exit: deterministic R5 race and
  replay tests, concurrency gate, adversarial primary capability tests, no
  stale marker rollback. Native/HTTP paths give equivalent policy decisions.
- Edwin: concurrency review; revisit primary-rename default if needed.

### P4 — Private preservation and recovery

- Depends on P3. Replace blanket isolation with class-specific preservation,
  format-1 portable recovery receipts and catalog inventory/holds. Trash
  source identity mapping handles copied UUIDs; holds precede rebuild/expiry.
  Supported Studio reassociation waits for proven content after scanning;
  ambiguous/newer content stays reviewable/exportable. Staging is journaled
  for recovery, never resumed from a lost catalog automatically; only known
  rebuildable derivatives may be rederived. Preserve diagnostic logs.
- Risks: highest data risk, hidden edits, expiry or misassociation. Exit: R4
  tree/hash preservation through success/failure/restart; fault injection at
  every preservation boundary; held inherited Trash survives rebuild, new
  deletes retain ordinary policy; private-recovery Web flow green.
- Edwin: careful preservation review; revisit metadata/Studio defaults. A
  reinstall of a copied real library is optional.

### P5 — Setup and startup recovery

- Depends on P2–P4. Explicit primary adoption/fresh-elsewhere, authenticated
  setup recovery routes and shared UI, catalog-first fresh/offline startup
  split, Server-owned native/operator startup facade. Restricted recovery
  startup requires a usable catalog; config Apply candidate failure still
  triggers rollback, never bypassing catalog compatibility.
- Risks: authorization widening, config/catalog boundary, fresh-install
  regressions. Exit: R3 adopt/fresh E2E using retained media and fresh app
  state; readiness-independent recovery contract; offline default not
  recreated; corrupt/newer bytes untouched; public setup status stays safe.
- Edwin: review; revisit fresh-default/permanently-lost-primary defaults.

### P6 — Complete reconnect journeys and volume semantics

- Depends on P3–P5. Native Open Existing, regular and whole-external-Location
  reconnect, protected default/primary configuration handoff and Apply.
  Qualify destination volume semantics, preflight collisions and bounded
  revision-fenced key rebuild without false absence or dropped bindings.
- Risks: scan fences, config rollback after catalog cutover, rekey loss.
  Exit: R2/R9 regular/Location/native harness tests; Apply success, crash,
  rollback; case/normalization image tests; “Reconnect” wording/prerequisites.
  Linux GHA loop-mounted vfat/exFAT/casefold-ext4 and macOS `hdiutil`
  case-sensitive APFS/ExFAT images provide reproducible volume evidence.
- Edwin: supply R2 inputs and review; revisit volume-semantics default.
  Real external disks and Windows NTFS are optional reference coverage.

### P7 — Enforce and remove competing paths

- Depends on P2–P6; remove obsolete paths only after callers migrate.
  Delete old classifiers/modal orchestration/Desktop marker parser; menus
  consume capabilities; executable recovery registry; final docs/i18n/codegen.
- Risks: overlooked caller. Exit: core architecture checks forbid competing
  storage classifiers, Desktop marker parsing, handler-built action lists,
  incomplete enum/contract coverage, and authoritative create risks from
  candidates. Each new check catches a deliberate violation. R10 recovery
  reachability and exhaustive en/zh action/risk/Problem presenters pass.
- Edwin: PR review only. No alternate policy retained as a fallback.

### P8 — Release journey wrap-up

- Depends on all earlier phases; wrap-up, not an extra per-phase gate.
  Focused tests already land with each fix. Dedicated fresh-catalog
  `web:test:storage-recovery` E2E slice is green in CI, with isolated media
  and app-state volumes per scenario. Future slice belongs in the Web
  Taskfile; affected workflow path filters change in that same PR.
- Risks: platform findings require further fix PRs. Exit: automatable journey
  and crash/rollback scenarios pass; evidence tables updated; owning docs
  current; durable decisions extracted, surviving debt moved to the tracker,
  then this plan deleted per the exec-plan skill (no completed archive).
- Edwin: one Mac pass before the rc tag, recording build SHA, OS/filesystem,
  and verdict: removable non-default `/Volumes` create in en/zh; externally
  moved regular reconnect plus wrong-marker/online-copy refusal; reinstall
  Adopt and separate fresh-elsewhere; default Apply/restart. Windows pass
  optional. Edwin confirms the tag when P0–P8 and core real-use quality hold.

P1 policy/copy preparation may proceed independently of later transport
work. After P3 freezes the command contract, P4 recovery design, P5 setup
presenters and P6 reconnect presenters can be prepared independently, but
avoid concurrent unowned edits to recovery/schema/shared command boundaries.
All P0–P8 complete before rc.1; this is a quality milestone without a date
deadline. P2 fixing #242 alone does not complete the refactor.

## Black-box regression designs

P1 core evidence is recorded below; other rows remain designs. Use real public
service/API/host entries and observable files/catalog/UI; never replace the
production parent with callbacks it does not supply. Web flow specs live in
`flows/<flow>/*.spec.tsx` (Chromium/MSW); real runtime/browser tests live in
`web/e2e/specs/*.spec.ts`. Go unit/integration tests live beside their owning
package; use the existing `server/app` HTTP harness for wired-runtime proof.
Fixture self-tests below prove fixture construction only.

| ID / failure                                                                  | Test layer and fixture                                                                                                                                                                                                                                   | Precise externally observable assertion                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Fixing phase                                                     |
| ----------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| R1 — #242 removable non-default new-child dead end                            | Go integration with valid default/external trees and P1 injected removable target facts; real StoragePanelFlow Web flow spec with `nativeHostAvailable=true`, candidates absent, new child under non-default Location                                    | Actual selected target returns risks; before acknowledgement command refuses with usable review; checkbox renders/enables submission; acknowledged create registers the selected child UUID/path, emits receipt and scan work. Editing target/intent/marker/mount/risk invalidates consent; capacity-only change does not. Default candidates cannot supply it; en/zh copy is usable                                                                                                                                                                                                                                                                 | P2                                                               |
| R2 — Desktop relocate/reconnect failure                                       | Go Server integration + embedded Desktop picker/runtime-config harness + Web flow; valid regular tree externally renamed away from registered path, same-UUID online copy, wrong-UUID tree, whole-Location copy with translated children/default primary | For positively unavailable original and authorized matching destination, picker → review → command retains UUID, Asset/album bindings, updates paths atomically and fences scan/watch work; bytes are unchanged. Empty/wrong/unauthorized/online-copy targets refuse with specific reachable recovery; denied/unknown original does not authorize reconnect. Whole Location validates every child before any update. Default selection changes only draft until Save/Apply; Apply/restart makes config/current/LKG/catalog coherent; candidate failure/crash rolls back coherently. Primary row routes to default recovery, never ordinary reconnect. Picker returns a path then silent no-op is a regression: selection always reaches visible review, reasoned refusal, or success | P6 (protected-action boundary in P3)                             |
| R3 — leftover primary dead end                                                | Go app/API integration, shared primary-setup Web flow, fresh-bootstrap E2E; `NewLeftoverPrimary`, fresh app-state/catalog, valid root with corrupt/newer/missing primary variants, `NewUnmarkedPrimary`, retained incompatible catalog/config variants   | After admin authentication and before readiness, Adopt and fresh-elsewhere are reachable. Explicit Adopt keeps UUID/original hashes, assigns Host Owner, creates exactly one primary, queues scan and recovery hold, reaches ready. Fresh-elsewhere preserves old tree and registers only new fixed primary. No implicit adoption; nonempty unmarked/corrupt/newer bytes untouched; backup vs rebuild disclosure present. Registered offline default keeps ready, is not recreated or replaced. Public status/unauthorized requests expose no host details                                                                                           | P5; private holds from P4                                        |
| R4 — `.lumilio/` isolation, inherited expiry and edit loss                    | Go integration + private-recovery Web flow; `NewLeftoverPrimary` opened as regular, adopted, and `CopyRepository` registered independently in fresh catalogs; injected crash boundaries and clock                                                        | Hash/tree inventory proves originals, Trash/info, Studio, staging, unknown children remain recoverable, with reachable recovery item actions after success/failure/restart. Trash view rebuilds source identities correctly, inherited entries remain held past ordinary expiry until explicit action, new deletes follow retention. Studio attaches only to proven content hash/size/source, ambiguous/newer records export/review without guessing; staged work never auto-resumes. Retry does not remint UUID or repeat moves                                                                                                                     | P4; adoption variant P5                                          |
| R5 — rename race (unsafe outcome inferred by audit)                           | Deterministic Go integration/concurrency; valid marked Repository, external marker replacement or externally moved matching tree; hold/release operation-boundary hooks introduced with fix, no sleeps                                                   | Park Rename before lease acquisition, complete reconnect or replace marker, then release Rename. Re-query catalog and re-read both old/new markers: current leased row/path is used; only raw name changes; all other config/identity fields survive. A replaced identity produces specific stale/mismatch refusal and no overwrite, including rollback. Replay same payload returns same result, changed payload conflicts                                                                                                                                                                                                                          | P3                                                               |
| R6 — stat panic / denied-as-absent / placeholder masking                      | Go unit observation + Go integration; valid tree, P1 observer returns nil info + `fs.ErrPermission`/other non-NotExist stat/read errors, genuine missing path and cloud placeholder controls                                                             | Public assessment/validation returns without panic; denial is permission-denied, arbitrary failures unknown, neither is absent/offline/placeholder. Re-query catalog/entries: no false Missing, no reconnect capability from unknown original, no marker provisioning. Only positively absent originals enable applicable recovery                                                                                                                                                                                                                                                                                                                   | P1; command/HTTP variants P2–P3                                  |
| R7 — foreground write probes / inconsistent network classification            | Go unit + API integration; P1 write-incapable observer/event recorder, valid root/child tree snapshots; injected NFS/SMB/CIFS/AFP/sshfs/WebDAV/Windows remote-drive facts, Windows removable, cloud resident/evicted controls                            | Setup/status/candidate/assessment requests perform zero create/write/lock/reconcile/expiry calls and leave tree unchanged, including transient-event log. Every network variant returns the same unsupported fact before consent; create/open/reconnect cannot confirm past it. Removable/local-cloud facts produce reviewable risk, unavailable placeholders materialize/retry, permission errors remain distinct. Windows classification is exercised without hardware                                                                                                                                                                             | P1 core; P2 transports/review; P6 reconnect                      |
| R8 — parent/child risk and continuity                                         | Go unit/integration; valid Location with independently mounted-child facts (parent local A, child B, B unchanged across samples), then missing/corrupt parent marker and healthy registered child                                                        | Child risk/support uses B; unchanged B has no mount-changed warning merely because A differs. Actual B replacement changes continuity; capacity grouping is independent. Parent fault does not deny registered child read/rename/detach, while new destination authorization still rejects invalid parent scope                                                                                                                                                                                                                                                                                                                                      | P1; rename/detach boundary P3                                    |
| R9 — Open Existing / conflict handoff / external reconnect UI omissions       | Web flow + embedded Desktop picker-controller integration; non-default compatible marked child, actual StoragePanelFlow composition, new-create conflict, complete external Location                                                                     | Native-backed Open launches native selection for chosen Location instead of default candidates; standalone bounded candidates retain selected context. Conflict handoff reaches Open/reconnect/copy review with location/child retained. External Location menu has direct reconnect entry. Cancel/stale/expired picker retains form and returns legal restart action; task paths/nonces never leak to HTTP                                                                                                                                                                                                                                          | P2 create handoff; P3 task outcomes; P6 native/Location journeys |
| R10 — illegal protected actions / generic unreachable failures / default hint | Go policy/API table integration + Web flow/presentation tests; adversarial capabilities, primary/default and regular/offline trees, owned/busy task controls                                                                                             | Menus never invent primary rename/copy/detach/ordinary reconnect; protected default hint directs to executable config/operator recovery. Every public Problem and async reason has an actor/phase-correct descriptor whose route/host entry can actually be followed from a real failure. Domain reasons survive native transport; auth/phase guards hold, arbitrary browser host paths are rejected; exhausted enum/risk/action cases have literal en/zh copy                                                                                                                                                                                       | P3 role/task policy; P5 setup reachability; P7 exhaustiveness    |
| R11 — host-default scan-key volume debt                                       | Go integration + CI filesystem-image fixtures; sensitive/insensitive and normalization-distinct names, supported cross-volume reconnect and deliberate destination collisions                                                                            | Destination semantics determine keys; reconnect/rekey preserves entry IDs and Asset bindings, suppresses false absence, detects every collision before cutover, refuses without dropping rows or overwriting bytes. Unknown/mixed/directory-specific semantics refuse affected mutation until qualified by a reproducible fixture                                                                                                                                                                                                                                                                                                                    | P6                                                               |
| R12 — marker/layout distinction and unsafe marker writes                      | Go unit/integration; all `Marker` states, `MissingPrivate`, missing inbox and required-directory file obstruction, injected marker replace/write failures                                                                                                | Compatible marker remains compatible when layout missing; explicit repair adds only allowed missing dirs and preserves UUID/originals. File obstruction/denial blocks repair; absent/denied/corrupt/unsupported markers stay distinct. Atomic-write failure leaves complete old/new marker, guarded replacement/rollback never clobbers external identity                                                                                                                                                                                                                                                                                            | P1 marker primitive; P3 command guard; P5 repair/adopt           |

**R2 inputs / Edwin's macOS Desktop repro (2026-10-07):** clicking Relocate
opens Finder; he selects a folder, then nothing happens: no error, no change,
no receipt. Audit pointers: `RepositoryRowActions.tsx:115`, `host_action.go:420`.
R2 must cover **picker returns a path, then silent no-op**: every outcome after
selection must be visible as review, refusal with reason, or success. Exact
destination type/path/marker IDs and whether the original was online are
optional refinements for P6, not blockers. Current inspection establishes
reconnect prerequisites and lower-stack Apply success, not the native cause.

### P1 regression evidence

- R6: `TestValidateRepositoryStatFailureDoesNotPanic` is red against P0 parent
  c3cb6c31: nil-pointer panic at `repo_manager.go:430`. The fixed test and
  `TestValidationInjectedStatErrors` pass; denial, arbitrary error, nil info
  and positive absence are distinct. Assessment error tests never offer
  marker-writing/create/read from unknown facts.
- R7 core: `TestAssessmentReadOnlyAndChildIndependence` snapshots names,
  modes, sizes, mtimes and hashes before/after and uses the write-incapable
  recording observer. `TestProductionAssessmentDoesNotChangeTree` also snapshots
  the production observer over retained original/private fixture bytes.
  `TestSharedStorageClassifierAcrossPlatforms` covers
  Linux/macOS/Windows network variants and Windows remote/removable facts;
  `TestPlaceholderErrorsRemainAccessErrors` covers denied/unknown attributes.
  Setup/status/candidate transport regressions remain P2/P6.
- R8 core: independent A/B mounts, unchanged B continuity, replaced B device,
  corrupt/missing/newer parent marker, healthy child and missing private
  layout pass. `TestChildSupportUsesActualFilesystem` proves a local parent
  cannot admit a CIFS child and capacity changes do not change continuity; `TestAbsentTargetUsesNearestProvenAncestor` also passes.
- R12 primitive: `TestAtomicMarkerFailuresLeaveCompleteOldOrNew`,
  `TestGuardedMarkersRefuseExternalIdentityAndRollback`, and
  `TestGuardedMarkerCreationAndInvalidMarkerPreservation` pass for both
  markers, including injected partial write, short write, sync, close, rename,
  directory-sync and external replacement failures. Command/repair tests remain
  P3/P5. `TestTypedMarkerReadings` covers every typed marker state.


### P0 fixture API and evidence

Import `server/internal/storage/testfixture` only from tests, following the
existing shared-test-helper convention. It belongs to the Storage Atlas
group so it can use rootcfg/repocfg writers without Foundation importing
Storage or creating a storage↔trash test import cycle.

- `NewLocation(t, Marker)` creates a `t.TempDir()` Location; `Marker` is
  Valid, Corrupt, Unsupported (`99.0`), or Missing.
- `Location.Repository(t, child, Marker, Layout)` creates a fresh direct
  child with deterministic name-derived UUID, complete `"1.0"` config, and a
  readable PNG original. Full creates the current private layout;
  MissingPrivate retains inbox/originals without `.lumilio/`.
- `NewLeftoverPrimary(t)` returns default/primary with Trash bytes and
  format-1 info, content-matching Studio v1 sidecar, abandoned incoming/failed
  staging, and opaque unknown child; `NewUnmarkedPrimary(t)` preserves a
  nonempty primary without a marker/private layout.
- `CopyRepository(t, source)` copies bytes/UUID to an independent TempDir;
  it neither registers nor mints an identity. Paths/sidecar IDs and the fixed
  `Timestamp()` are exported for precise later assertions. Builders guarantee
  deterministic names/bytes/mtimes, not mount facts, permissions, ownership,
  or catalog readiness.
- `TestMarkerFixtures`, `TestLeftoverPrimaryFixtures`, and
  `TestMissingLayoutAndUnmarkedPrimaryFixtures` compare independent trees,
  use current marker readers, Trash parser/Studio DTO, verify hashes and PNG
  readability, and check the complete layout with DirectoryManager. They
  pass on current production behavior and are not behavioral regression proof.

No injected-platform fake lands in P0: inspection currently calls concrete
platform functions with no public injection seam. P1 adds the observer and
typed platform facts with its fix and tests; no host-path naming trick or
production hook is introduced here. No red regression branch is prepared
in P0; fixing PRs record deterministic red evidence as they develop R1–R12.

## Validation boundaries

P0 acceptance is documentation completeness plus passing test-only fixture
self-tests, Go build/vet/test for the touched package, architecture and Atlas
checks, and generated freshness. Adding the helper package changes generated
Atlas package maps only; no existing production anchor changes and no view
re-lock is warranted. Run generators rather than editing generated files.

Implementation completion requires observable invariants: capability and
command agreement over generated role/phase/fact combinations; consent
binding; compatible markers unchanged; denied access never false absence;
read-only foreground; child independence; protected roles; preservation and
holds; reconnect identity/metadata/scan coherence; volume collision refusal;
idempotent recovery. Fault injection covers receipt persistence, every
private-entry move, atomic marker replace, filesystem-applied/catalog-commit,
scan follow-up persistence, and Apply quiesce/pointer/startup/rollback.

Use the narrowest module/layer checks per
[select-checks](../../../.agents/skills/lumilio-select-checks/SKILL.md).
Server/Web/Desktop/contract tests and CI filesystem-image fixtures are hard
gates because they reproduce on GHA or an ordinary dev machine. Journey
evidence spans local/new, removable/non-default/child-mounted, network refusal,
cloud residency, Open, same-ID copy/move, regular/Location/default reconnect,
fresh-catalog reinstall, corrupt/newer/missing markers, repair, rename/detach,
and private recovery. Real native picker coverage is the single pre-tag Mac
pass; Windows is optional. No hardware/performance gate per phase.

## Risks and optional reference hardware

| Risk                                             | Required mitigation                                                                                                   |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| Assessment becomes another classifier            | Cut callers over and remove old policy; P7 core architecture checks                                                   |
| Facts change after consent                       | Lease refresh, reassessment, rooted access, guarded writes; advisory locks cannot control external applications       |
| Private bytes survive but user work stays hidden | Reachable recovery inventory/actions, content-proven Studio mapping, staging export, unresolved receipts              |
| Adoption exposes inherited Trash to expiry       | Hold before admission; reconstruct held state even after a second catalog loss; crash tests                           |
| Pre-setup recovery leaks host facts              | Authenticated admin allowlist, unchanged public status, actual router reachability tests                              |
| Apply rolls config back after catalog cutover    | Qualify config/current/LKG/catalog coherence through crash and candidate failure; never overwrite media for rollback  |
| Rekeying loses entries                           | Collision preflight, bounded revision fences, retained entry identity, no false absence; refuse unqualified semantics |
| OS support policy diverges                       | One support decision with injected Windows/macOS/Linux platform facts                                                 |
| Scope creeps into movement/network rewrite       | Frozen non-goals and #245 interface boundaries; retain runtime and lifecycle engines                                  |

N100/radxa measurements or a retained-primary Docker run are optional
reference data for diagnostics/release wording only. They never gate a
phase, this plan's completion, or rc.1. The required real-machine storage
validation is one Mac pass before the rc tag; Windows is optional.
