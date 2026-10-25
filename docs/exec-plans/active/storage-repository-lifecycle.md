# Storage Location and Repository lifecycle

Status: active, created 2026-10-07; principal owner decisions frozen
2026-10-07. P0 merged as PR #246. P1 merged as PR #247: observation and
assessment core, stat panic fix, guarded atomic marker primitive, and
swappable lock/identity/watch adapters. P2 merged as PR #250 (session S2,
2026-10-16). P3a (commands) merged as PR #252 (session S3a, 2026-10-18);
P3b (native selection and review) merged as PR #256 (session S3b,
2026-10-19). P4 (private preservation and recovery) merged as PR #257
(session S4, 2026-10-19). P5 (setup and startup recovery) merged as PR #259 (session
S5, 2026-10-21). P6 was split as the schedule allows: P6a (regular and
Storage Location reconnect journeys, the positively-unavailable rule, the
Reconnect wording) merged as PR #260 (session S6a, 2026-10-25); P6b
(per-repository volume semantics, collision preflight and bounded rekey on
reconnect, the Linux and macOS filesystem-image CI jobs, the Desktop Apply
crash evidence) lands in this PR (session S6b). P2 delivered (session S2):
the create contract and
shared risk review, closing #242. P7–P8 remain outstanding. Each fixing PR lands
its regression tests and updates this Status and the evidence tables.
Child of [release-hardening.md](release-hardening.md); its session order
(S2–S8, one phase per session, P3 and P6 may split) is the schedule.
2026-10-13: no physical machine is available for the foreseeable future, so
the P8 Mac pass is an rc.2 checkpoint and every phase exit below is proven
by CI, the embedded Desktop harness, and CI filesystem images only
([decision](../../../.agents/decisions/2026-10-13-rc1-evidence-without-hardware.md)).

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
| New catalog, compatible root + fixed primary, readable originals            | Explicit Adopt Primary with current Host Owner, or fresh elsewhere                                                 | Authenticated setup adoption/selection without completed-setup gate                | P5 adopt, fresh default, setup review pass (R3)     |
| Catalog absent, markers/originals retained                                  | Compatible backup restore or explicit rebuild via open/adopt                                                       | Setup disclosure; private recovery inventory                                       | P4 inventory, P5 setup disclosure and adopt pass    |
| Pre-rc.1 catalog remains (`catalog_lineage_unsupported`)                    | Preserve/archive old app state; start supported fresh catalog                                                      | Startup native/operator guidance; never bypass compatibility                       | P5 startup facade (CLI and Desktop) pass            |
| Incompatible TOML/runtime intent (`configuration_incompatible`)             | Supply fresh complete manifest/intent, preserve media                                                              | Startup configuration presenter/operator guidance                                  | P5 startup facade (CLI and Desktop) pass            |
| Prerelease marker `"1.0"`                                                   | Same compatible open/adopt policy                                                                                  | No inferred previous installation or marker-era rejection                          | P1 typed 1.0 readers pass; P5 adoption pass     |
| New catalog; corrupt Location marker                                        | Restricted recovery runtime if catalog usable; no root provisioning                                                | Authenticated setup diagnose/fresh destination; native/operator fallback           | P5 restricted startup and default-storage review pass |
| Corrupt/partial or newer Location/Repository marker                         | No marker-write capability; preserve bytes                                                                         | Diagnose, compatible build, known-backup instructions, or fresh destination/import | P1 typed readings/guarded-byte preservation pass; P5 setup states pass |
| Missing marker; nonempty primary (`nonempty_unmarked`)                      | No initialization over tree                                                                                        | Fresh destination and explicit originals import; preserve old folder               | P0 unmarked fixture; P5 setup refusal pass (R3)     |
| Valid marker; missing `.lumilio/` or `inbox/` (`layout_repairable`)         | Explicit bounded repair/adopt, identity unchanged                                                                  | Repair review; file obstruction/denial is a blocker, never replacement permission  | P1 compatible identity + absent layout pass; P5 repair and obstruction pass (R12) |
| Registered readable identity                                                | Read/verify; upload subject to admission; regular rename/detach with fresh impact                                  | Storage journey and operation receipt                                              | R5, R8, R10 designed; P3                            |
| Registered original positively unavailable                                  | Retain catalog metadata and ready state; begin regular reconnect; regular detach remains possible                  | Select matching tree; primary uses default recovery                                | P6a regular reconnect over the native task pass (R2); R3, R8 designed |
| Registered marker missing/corrupt/different (`registered_identity_problem`) | No implicit replacement or marker overwrite                                                                        | Matching known tree/marker instructions, diagnose/retry                            | P1 guarded identity replacement pass; P6a reconnect admitted from an identity problem (policy table) |
| Same UUID elsewhere, original online (`identity_copy`)                      | Use existing registration; regular independent-copy review; no reconnect commit                                    | Current conflict journey; primary copy prohibited                                  | P0 same-UUID copy; P6a online-copy refusal on the task pass (R2); R10 designed |
| Same UUID elsewhere, original unavailable                                   | Explicit regular reconnect or copy where permitted                                                                 | Primary routes to complete-default configuration recovery                          | P6a explicit reconnect pass (R2); primary routing P6b |
| Original permission denied/unknown                                          | No conclusion of offline; no reconnect commit based on denial                                                      | Access diagnosis and reassessment (`original_inaccessible` / `check_access`)       | P1 denied/unknown/nil stat tests pass; P6a reconnect and reconcile refuse to conclude from denial (R2, R6) |
| Read-only / permission-denied target                                        | Mutation blocked; retain identities/catalog facts                                                                  | Permissions, retry, writable supported destination, also pre-setup                 | P1 access distinction/read-only core pass; P2 transport pending                              |
| Owned elsewhere / busy                                                      | No conflicting mutation; unrelated healthy trees usable                                                            | Stop owner, wait/inspect operation, retry; cancel only if supported                | R10 designed; P3                                    |
| Unsupported network/remote filesystem                                       | Refuse create/open/reconnect before acknowledgement                                                                | Choose supported destination, with reason; confirmation cannot enable it           | P1 shared injected network classifier pass; P2 refusal wiring; #245 deferred                   |
| Cloud-backed resident tree / unavailable placeholder                        | Review risks; deep preflight checks residency                                                                      | Materialize locally and retry; denial has its own classification                   | P1 access distinction/read-only core pass; P2 transport pending                              |
| Private Trash/Studio/staging/unknown retained                               | Preserve by class; hold inherited Trash; rebuild/reassociate proven matches                                        | Reachable recovery inventory, review/export unresolved records                     | P4 preservation, holds, reassociation, inventory pass |
| Interrupted mutation (`recovery_required`)                                  | Recover journal before conflicting mutation                                                                        | Recovery receipt, diagnose/retry                                                   | P3 journals; P4 `storage/recovery-required` and preservation rollback pass |
| Unregistered native Location selection                                      | Review authorization; create root marker only when genuinely absent or register compatible marker                  | Native task review; stale/expired selection restarts without losing form           | R9, R10 designed; P3, P6                            |
| Registered external Location                                                | Create/open direct children; reconnect complete identity; detach only with zero children/no pending mutation       | Direct Location reconnect entry, all child markers revalidated atomically          | P6a kind-aware Location policy, direct Web entry, all-or-nothing reconnect over the native task pass (R2, R9) |
| Default, setup incomplete                                                   | Create/adopt fixed primary; choose fresh default only with no primary, no child registrations, no pending mutation | Desktop config selection/Apply or standalone manifest/restart; preserve old tree   | P5 fresh default (Server rule, Desktop pre-check) pass |
| Default/primary, setup complete                                             | Identity-preserving whole-default reconnect through configuration Apply                                            | Default recovery handoff; no ordinary primary rename/detach/copy/reconnect         | P3 primary protection, P6a `default_protected` Location verdict pass; Apply crash/rollback evidence P6b |
| Parent marker unavailable/invalid, healthy registered child                 | Child read/rename/detach independently admitted; destination authorization still enforced                          | Parent diagnosis/reconnect separately                                              | P1 parent-independent child observation pass; P3 command boundary pending                                 |

### Error-to-recovery coverage

Every descriptor must be usable in the returned actor/bootstrap phase.
Returning an Open action behind the primary readiness gate is a failure.
Endpoints named here are proposed unless already present; no P0 API change.

| Problem / outcome                                              | Required action and entry                                                                                           | Evidence / phase  |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ----------------- |
| `storage/confirmation-required`                                | Review actual risks; retry current command with token/acknowledgements, also primary setup                          | R1; P2            |
| `repository/conflict` / identity registered elsewhere          | Retain target context; Open regular, Adopt Primary, regular copy/reconnect, or choose elsewhere according to matrix | R2, R3, R9; P2–P6a |
| `storage/assessment-stale`                                     | Refresh ordinary/setup assessment; review changed facts, preserve native task scope                                 | R1, R5; P2–P3     |
| `storage/action-unavailable`                                   | Show current blockers and legal alternatives; no automatic rejected-action retry                                    | R10; P3           |
| `storage/permission-denied`, `storage/read-only`               | In-place instructions, reassess, choose writable destination                                                        | R6; P1–P2         |
| `storage/observation-unknown` | Diagnose access/mount/attribute failure and reassess; no assumption of absence or mutation safety | R6/R7 P1 core; P2 transport |
| `storage/not-a-mounted-volume` | Mount a host directory in compose; preserve the existing Docker empty-folder rejection | `TestDockerEmptyTargetConstraint`; P1 code/copy, P2 transport |
| `storage/unsupported-filesystem`                               | Supported destination or deployment instructions, no confirmation bypass                                            | R7; P1–P2         |
| `storage/marker-invalid`, `storage/marker-version-unsupported` | Setup/storage diagnostics, compatible build, known-backup instructions, choose elsewhere; startup fallback          | P1 readings; P5 setup states and restricted startup |
| `storage/identity-mismatch`, `storage/original-online`         | Matching-tree selection or existing registration/regular copy review; no overwrite/primary copy                     | R2, R10; P3, P6a (both refusals on the native task and the resolve route) |
| `storage/ownership-unavailable`, `storage/busy`                | Owner/operation diagnostics, wait/retry, supported cancellation                                                     | R10; P3           |
| `storage/materialization-required`                             | Bounded offending-file facts; local residency then retry deep preflight                                             | R6, R7; P1–P2     |
| `storage/recovery-required`                                    | Setup/storage private-recovery inventory and journal receipt; retry recovery                                        | P4 Problem and Storage inventory; P5 adopt maps it |
| `storage/host-action-expired` / native selection cancelled     | New phase-scoped task / return to selection with form retained; cancellation is a normal outcome                    | R9; P3, P6        |
| Protected primary/default                                      | Default configuration handoff or standalone operator instructions                                                   | R2, R10; P3, P6a (`default_protected`); Apply P6b |
| Generic failure                                                | Receipt/diagnostics, refresh, explicit safe retry; do not invent a specific remedy                                  | R10; P3, P7       |
| Authentication/authorization/bootstrap failure                 | Sign in, authorized admin entry, or owner/primary setup; no host facts leaked                                       | P5 public status phase-only, admin review; R10 P7 |

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
| Unsupported volume semantics         | Refuse affected transition; extend support only with CI filesystem-image evidence                            | P6b delivered: qualified images are FAT32, exFAT, ext4, case-folding ext4, APFS, case-sensitive APFS, HFS+, macOS ExFAT |

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
Docker app on Debian trixie; there is no native Linux desktop. The
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

### P2 delivered (2026-10-14)

- **Consent is bound to an assessment token.** `AssessCreateTarget` resolves
  the create target from a registered Storage Location and folder, samples it
  through the installed `StorageObserver`, and mints an HMAC token over the
  facts that decide the review: intent, Location identity and kind, folder,
  resolved path, support and problem, the sorted risk vocabulary, access and
  emptiness, filesystem, mount identity (`MountID`, `MountSource`,
  `MountPath`, `Device`, removable, remote, cloud provider), and both marker
  readings. Capacity figures, inode, and capacity-group key are excluded, so
  free-space drift keeps a review valid while a replaced disk, a new marker,
  or a different risk set invalidates it. The key is per process; a restarted
  Server asks for a fresh review. `CreateRepository` re-samples the same
  target and admits only `AssessmentToken` equal to the current token plus an
  `AcknowledgedRisks` set covering every reported risk; it refuses with
  `StorageRiskConfirmationError{Risks, Token}` (no review or incomplete
  acknowledgement), `ErrStorageAssessmentStale` (facts changed), or a typed
  support refusal (`ErrStorageUnsupportedFilesystem`, not-a-mounted-volume,
  `ErrStoragePermissionDenied`, `ErrStorageObservationUnknown`,
  `ErrStorageMaterializationRequired`) before any journal or disk change, and
  audits each refusal (`risk_confirmation`, `assessment_stale`,
  `storage_support`). The legacy `risk_confirmation` boolean and
  `StoragePlacementWarnings` are gone from the create and setup commands;
  candidates, host actions, and open/relocate keep theirs until P3 and P6.
- **Transport.** `POST /api/v1/storage/assessments` (admin) returns
  `StorageAssessmentDTO` (token, intent, Location id and kind, folder,
  support, problem, risks, can_create, existing marker identity, marker
  state; never a path). `CreateRepositoryRequestDTO` and
  `SetupPrimaryRepositoryRequestDTO` carry `assessment_token` and
  `acknowledged_risks`; the create response returns `acknowledged_risks`
  instead of English warning sentences. Setup status carries the
  `setup_primary` assessment under `repository_defaults.assessment` until the
  primary exists. `storage/confirmation-required` is now a typed Problem with
  `risks` and `assessment_token`; `storage/assessment-stale`,
  `storage/unsupported-filesystem`, `storage/not-a-mounted-volume`,
  `storage/observation-unknown`, `storage/permission-denied`, and
  `storage/materialization-required` are registered, documented, and
  localized. The two create commands' 409 is the exact
  `api.RepositoryCreateProblemResponse` union.
- **Web.** The Add Repository Review step runs the assessment for the
  selected Storage Location and folder (`useStorageAssessment`), renders the
  risks through `storageRiskLabel` in `StorageRiskConfirmation`, resets the
  acknowledgement whenever the token changes, submits token plus every risk
  code, re-reviews on `confirmation-required` or `assessment-stale` with an
  inline notice, shows typed refusals inline, and hands `existing_repository_found`
  / `repository_identity` to Open Existing (native task in Desktop, candidate
  modal with the folder highlighted elsewhere) with the Location and folder
  retained. Server candidates only suggest folder names. Setup (gate and
  bootstrap) acknowledges against the setup status assessment. All risk codes
  have en/zh labels; zh coverage 100%.
- **Evidence (R1, R7 transport, R9 create handoff).**
  `TestCreateTargetFingerprintIgnoresCapacityButBindsEveryDecidingFact`,
  `TestConsentTokenIsOpaqueAndStableWithinOneManager`,
  `TestAssessCreateTargetReviewsTheActualTargetWithInjectedFacts`,
  `TestCreateRepositoryBindsAcknowledgementToTheAssessedTarget` (red when
  the token comparison is disabled: a foreign token was accepted),
  `TestCreateRepositoryRefusesUnsupportedStorageBeforeAnyConsent`,
  `TestSetupPrimaryIntentReviewsTheDefaultPrimaryTarget` (Server, injected
  removable/network/denied facts on a real catalog with an external
  Location); handler tests for the consent Problem body, every typed
  refusal on both create commands, and the assessment endpoint; setup
  service tests; Web `AddRepositoryModal.test.tsx` (Desktop shape with no
  candidates, acknowledgement reset on changed facts, unsupported target
  refused inline, conflict handoff payload), `StoragePanel.spec.tsx` (R1
  through the real panel with native host available; create handoff into
  Open Existing with the folder highlighted), `PrimaryRepositoryGate.test.tsx`,
  `useBootstrapFlow.test.ts`, and `problem.test.ts` (bounded extensions,
  en/zh for every new type). #242 closes with this PR.
- **Deviations recorded.** Risk acknowledgement is one checkbox that
  acknowledges every listed risk code at once; the wire contract stays
  per-code so a later UI can split it. The `non_default_location` risk now
  applies to every external Location create, as P1 classified it; before P2
  an external Location without other facts needed no confirmation.

### P3 — Safe command boundary

- Depends on P1–P2. Reassess mutations under barriers, rename refresh,
  role-aware actions, native selection/review, domain-specific failures,
  idempotency and receipts. Keep barriers, catalog boundaries, scan fencing.
- Risks: races, deadlocks, replay errors. Exit: deterministic R5 race and
  replay tests, concurrency gate, adversarial primary capability tests, no
  stale marker rollback. Native/HTTP paths give equivalent policy decisions.
- Edwin: concurrency review; revisit primary-rename default if needed.
- Split 2026-10-16 into P3a (commands, this PR) and P3b (native selection
  and review: host-action task outcomes R9, `storage/host-action-expired`
  as a normal outcome with the form retained, and the native host going
  through the same policy as HTTP).

### P3a delivered (2026-10-16)

- **Role-aware action policy.** `storage.DeriveRepositoryActions` is a
  pure function over catalog facts (role, reachability, activity, a running
  journal entry) returning a verdict per action in the closed vocabulary
  `verify`, `rename`, `detach`, `reconnect`, each withheld verdict carrying
  stable blocker codes (`primary_protected`, `repository_offline`,
  `identity_problem`, `recovery_required`, `maintenance`,
  `repository_busy`, `pending_operation`, `original_available`) and
  alternatives (`default_configuration`, `recover_operation`,
  `wait_for_idle`). The Primary never gets rename, detach, or reconnect
  (smaller decision "Primary rename: prohibited", applied as the default).
  The storage view publishes `actions` and `unavailable_actions` per row and
  the Web menu renders Rename, Remove, and the native reconnect entry only
  from them; the Primary row therefore offers neither Rename nor Remove.
- **Commands admit through the same policy, re-derived under the lease.**
  Rename, detach, and reconnect check the projected row for a typed
  refusal, then re-read the row after acquiring the mutation barrier and
  decide on those facts. Reconnect lets the marker probe, not the stale
  projection, decide whether the original is online. The refusal is
  `RepositoryActionUnavailableError` (Problem `storage/action-unavailable`
  with `action`, `blockers`, `alternatives`); it still matches the legacy
  sentinels (`ErrPrimaryRepositoryNotRemovable`, `ErrPathNotAllowed`,
  `ErrRepositoryBusy`) for existing callers.
- **Rename refresh (R5, R12 command guard).** Under the lease the command
  loads the complete marker at the current path, requires its identity,
  changes only the raw name, and writes it with `SaveGuarded` (expected
  identity), as does the rollback and the startup recovery; a marker another
  writer replaced is refused as `ErrRepositoryIDMismatch` and never
  overwritten, and a rollback that finds a replaced identity leaves it alone
  and marks the operation `rollback_required`.
- **Journaled detach and receipts.** Detach is lifecycle kind
  `detach_repository` (baseline CHECK amended in place, pre-rc.1 rule); its
  journal completes in the removal transaction, so no crash leaves a
  detached Repository with a running operation, and an interrupted detach
  is rolled back at startup with the row intact. Rename and detach return
  `LifecycleReceipt` (`operation_id`, `request_id`, `kind`, `status`,
  `replayed`); a retry with the same Idempotency-Key gets the same receipt
  even after the detached row is gone; a different payload conflicts.
- **Typed failures.** New Problems `storage/action-unavailable`,
  `storage/busy`, `storage/ownership-unavailable`,
  `storage/identity-mismatch`, `storage/original-online` (registry 38),
  the exact `api.RepositoryCommandProblemResponse` 409 union on rename,
  detach, and candidate resolve, en/zh presenters keyed by the first
  blocker, pages and Redoc regenerated. `ErrRepositoryOwnershipUnavailable`
  wraps a failed runtime ownership claim in open and reconnect.
- **Evidence.** `TestDeriveRepositoryActionsIsRoleAware` (adversarial
  primary and every withheld state), `TestCommandsAgreeWithTheActionPolicy`
  (role × reachability × activity × pending journal, real catalog rows:
  each command refuses exactly when the policy withholds, with the same
  blockers, and never on policy grounds when it allows),
  `TestRenameRefreshesTheLeasedRowAfterAReconnect` (rename parked before
  its lease by a test seam while a reconnect moves the tree: the new path
  is renamed, only the raw name changes, the old path stays absent, same
  payload replays the same receipt, changed payload conflicts),
  `TestRenameRefusesAReplacedIdentityWithoutOverwriting`,
  `TestRenameRollbackNeverOverwritesAReplacedIdentity` (both rollback
  branches), `TestDetachIsJournaledAndReplaysItsReceipt`, handler tests for
  every new Problem and the detach receipt replay, the storage view action
  test, Web `RepositoryRowActions.test.tsx`, `problem.test.ts`, and the
  StoragePanel flow spec. Concurrency gate: the storage package under
  `-race -shuffle=on -count=3` passes. The agreement test found two real
  disagreements while it was written (reconnect validated the destination
  before the policy; the Primary's verdict lost its other blockers), both
  fixed before this PR.
- **Deviations.** The Web keeps the "Locate on disk" label for the native
  reconnect entry until P6 renames it with its prerequisites; the entry is
  now shown only for a positively unavailable regular Repository. Blocked
  actions are not yet explained in the Inspector (P7 presenters).

### P3b delivered (2026-10-18)

- **The native host goes through the shared assessment.** `ExecuteHostAction`
  samples the folder Desktop chose with `AssessStorageTarget` through the
  installed P1 observer (`reviewHostActionTarget`): authorizing or
  reconnecting a Storage Location is a non-default destination, opening or
  reconnecting a Repository takes the kind of the Storage Location that
  contains the folder, and a reconnect adds the continuity risk
  `mount_fingerprint_changed` when the selected tree sits on another mount
  than the registration recorded. A classification problem fails the task
  under that typed Problem (`storage/unsupported-filesystem`,
  `storage/not-a-mounted-volume`, `storage/permission-denied`,
  `storage/observation-unknown`, `storage/materialization-required`) before
  anything is written, and no decision can confirm past it; a folder that is
  gone by the time the task runs is unavailable, never created. The legacy
  `inspectHostActionStoragePath` warnings are removed from the native path
  (candidates keep theirs until P6/P7).
- **Risks are acknowledged by code, bound to the facts.** A `storage_risk`
  decision carries the P1 risk codes and a fingerprint digest of the facts
  (task, path, risk set, classification, marker and mount facts; capacity
  excluded). `confirm_risk` requires `acknowledged_risks` covering every
  code, from the Web (`ResolveHostActionRequestDTO.acknowledged_risks`) or
  from Desktop, which acknowledges the codes its ticket displayed. The
  resume re-samples the folder; a different fingerprint stores the current
  review and answers `storage/assessment-stale`, so consent never outlives
  the facts it was given for. The `risk_confirmation` boolean leaves the
  host-action contract (`app.RepositoryControl.ExecuteHostAction` takes the
  acknowledged codes).
- **Policy equivalence.** A `locate_repository` task runs `admitReconnect`
  before the folder review, and every command failure with a typed cause is
  recorded under that cause's Problem path (`storage/action-unavailable`
  with the verdict's action, blockers and alternatives persisted as
  `HostActionResult.Refusal`; `storage/original-online`,
  `storage/identity-mismatch`, `storage/ownership-unavailable`,
  `storage/busy`). `HostActionDTO.problem` names the typed Problem and
  `refusal` carries the verdict, so the Web presents the same en/zh copy as
  the HTTP command; an untyped failure stays `storage/host-action-failed`.
- **Expired and dismissed are normal outcomes (R9).**
  `storage/host-action-expired` is 409 with a recovery text, the Web modal
  presents expired and cancelled as information with the form retained and a
  **Send again** restart under a new idempotency key, and the dismissed
  Desktop picker is `selection_cancelled` (no failure toast; the request
  stays pending). Presenting a request in Desktop rebinds its expected
  storage version (`SetHostActionExpectedVersion` rebinds through the nonce;
  `ListHostActions` rebinds whenever the snapshot moved), so a changed
  snapshot never leaves a request un-approvable.
- **Web review.** `NativeHostActionModal` renders a `storage_risk` decision
  with the shared `StorageRiskConfirmation` (the create review's component
  and copy), resets the acknowledgement when the risk set changes, and
  confirms by code; Desktop's ticket labels gain `non_default_location`.
- **Evidence (R9 task outcomes, R10 domain reasons over native transport).**
  Server: `TestHostActionRequiresAcknowledgedRiskCodesBeforeMutation`
  (injected removable disk → codes `removable_storage`,
  `non_default_location`; empty, partial, wrong-resolution acknowledgements
  refused without mutation; full set succeeds; `storage_risk_confirmed`
  audit), `TestHostActionRefusesNetworkStorageWithoutADecision` (injected
  CIFS → failed under `storage/unsupported-filesystem`, not confirmable,
  not re-runnable), `TestHostActionStaleAcknowledgementPresentsTheCurrentReview`
  (device replaced under the acknowledgement → stale, new fingerprint, no
  mutation, current review acknowledgeable),
  `TestHostActionOpenAndLocateRequireRiskDecisionBeforeCatalogMutation`
  (open, Repository reconnect, Location reconnect with the continuity risk),
  `TestHostActionLocateRefusalCarriesThePolicyBlockers` (Primary reconnect
  through the native host → `storage/action-unavailable` with
  `primary_protected`, primary unmoved), the rebind contract in
  `TestHostActionIsDurableIdempotentAndNativeApproved`; handler tests for the
  consent/stale/original-online Problems on resolve and for the typed
  failure projection without the fingerprint; the storage package under
  `-race -shuffle=on -count=2` passes. Desktop: the embedded host-action
  harness in `desktop/internal/storage/host_action_controller_test.go`
  (stale rebind on listing, dismissed picker as `selection_cancelled` with
  no execution or operation, canonical selection without acknowledgements,
  acknowledged codes without reopening the picker, identity decisions not
  approvable from Desktop). Web: the StoragePanel flow spec drives the real
  panel through a removable-disk review confirmed by code, an expired
  request restarted with the form retained under a new idempotency key, and
  a withheld reconnect rendered with its blocker copy.
- **Deviations.** Docker/standalone candidates (`/storage/candidates`) still
  use the legacy warnings and `risk_confirmation`; they cut over with the
  native/Location journeys (P6) and the classifier removal (P7). The Desktop
  ticket still lists risks as a joined line rather than the shared review
  component (P7 presenters).

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

### P4 delivered (2026-10-19)

- **Preservation by class replaces blanket isolation.** `OpenRepository` and
  `RegisterRepositoryCopy` no longer move `.lumilio/` into a recovery
  folder. `planPrivatePreservation` inventories the private tree read-only
  and classifies every file: the Trash (`trash/info`, `trash/files`), logs,
  and derivatives (`assets`, `artifacts`) stay in place and are summarized;
  Studio records (`sidecars/*.lumilio-sidecar`) stay in place and are listed
  with the Asset they named (a version this build does not read is listed
  `unreadable`); uncommitted staging is moved under
  `.lumilio/recovery/<receipt_id>/staging/…` so ordinary staging cleanup can
  never discard it; anything else, including an earlier build's isolation
  folders, is listed `unknown` and left untouched. The plan is journaled in
  the operation's rollback data, the format-1 receipt
  (`.lumilio/recovery/<receipt_id>.json`, `storage.RecoveryReceipt`) is
  written before any move, every move is no-replace and idempotent (a file
  found under both names keeps the destination), and rollback or startup
  journal recovery puts staging back and removes the receipt.
  `CreateStructure` no longer truncates existing logs.
- **Inherited Trash is held.** `repository_entries.trash_hold`
  (`'inherited'`, baseline amended in place) is set by every file the Trash
  rebuild adopts from a sidecar this catalog did not write; `Expire` skips
  held rows, Restore and Delete permanently end the hold, and
  `trash.Service.AcceptRetention` releases it explicitly for a selection or
  one Repository in the caller's owner scope (audited
  `trash_hold_released`). **Default applied:** acceptance restarts the
  retention clock at the acceptance (`trashed_at` is set to it), because the
  catalog never saw the original deletion and an inherited file deleted
  years ago would otherwise expire on the next pass. A rebuild after a
  second catalog loss re-adopts under a hold, so the hold survives without
  catalog state.
- **Studio reassociation on proven content.**
  `recovery.Service.Reconcile` (new `server/internal/storage/recovery`
  package) runs after a scan drains and on every Trash maintenance pass. A
  preserved record waits until a scan finished after the receipt and
  nothing is left to hash; it is attached only when exactly one Asset in the
  Repository is bound to a present file with the record's hash and size and
  no live file at the record's source path carries different content. The
  record is rewritten under the Asset's name with its `asset_id` and
  `storage_path` corrected (every other field kept), the receipt records the
  reassociation, and only then the old name is removed, so a crash at any
  point is finished by the next pass without a second write. No match,
  several Assets, a replaced source, or an existing different record is
  `ambiguous`; the record stays for review or export. Audited
  `studio_reassociated`.
- **Reachable inventory.** `GET /storage/repositories/{id}/recovery`
  (receipts, items with class, disposition, presence, and the actions
  `export`/`delete`, held Trash counts), `GET …/items/{item}/content`
  (attachment), `DELETE …/items/{item}` (explicit, audited
  `recovery_item_deleted`; a reassociated record is live state and refuses),
  `GET /assets/trash-holds` and `POST /assets/accept-retention` (owner
  scoped like the other Trash commands; Repository-wide is administrator
  only). The Storage view carries `trash_held_count` and
  `recovery_item_count`. `storage/recovery-required` (409) is registered
  and mapped from `ErrLifecycleRecoveryRequired` on the Repository
  commands and candidate open. Web: the row's **Recovery inventory**
  (`RecoveryInventoryModal`: classes, dispositions, export, confirmed
  delete, held link to the Trash), Inspector and Trash-column counts, and
  in the Trash view `TrashHoldNotice` with **Keep with normal retention**
  for the Repository plus the same action for a held selection.
- **Evidence (R4).** `server/internal/storage`:
  `TestPlanPrivatePreservationClassifiesByClass`,
  `TestOpenRepositoryPreservesPrivateStateByClass` (hash inventory of every
  file before and after the open, staging under the receipt, logs not
  truncated, replay repeats nothing),
  `TestPrivatePreservationRollsBackACrashBetweenMoves`,
  `TestPrivatePreservationRetryFinishesAnInterruptedPlanWithoutRepeatingMoves`
  (both-names and conflicting-destination cases),
  `TestLifecycleRecoveryRollsBackAnOpenInterruptedAfterItsFilesystemPhase`
  (journal recovery restores the tree), `TestCreateStructureKeepsExistingLogs`;
  the copy registration test asserts the `register_copy` receipt.
  `server/internal/storage/trash`:
  `TestInheritedTrashIsHeldPastRetentionUntilTheUserDecides` (held past
  retention, this catalog's own Delete expires, hold survives rebuild and a
  second catalog loss, Restore ends it),
  `TestAcceptRetentionReleasesTheHoldFromTheAcceptance` (injected clock).
  `server/internal/storage/recovery`:
  `TestReconcileRecoversInheritedTrashAndProvenStudioRecords` (waits for
  proven content, reassociates the proven record, leaves orphan, replaced,
  and newer records for review, inventory actions, export, explicit and
  idempotent delete), `TestReconcileFinishesAnInterruptedReassociation`
  (crash between the receipt and the removal),
  `TestRecoveryRefusesAnUnreachableRepository`. Handler tests for the
  projection (no internal paths on the wire), refusal mapping, and the
  owner-scope rules of the hold endpoints. `server/app`:
  `TestPrivateStateOfAnOpenedRepositoryIsRecoverable` opens the P0
  leftover-primary tree through `/storage/candidates/open` and proves, over
  HTTP, the held Trash, the reassociated Studio record served by
  `/assets/{id}/sidecar`, export, delete, acceptance, and Restore. Web:
  `StoragePanel.spec.tsx` (recovery inventory exports and deletes
  explicitly), `TrashHoldNotice.test.tsx`, `problem.test.ts`.
- **Deviations.** The private-recovery Web flow is the Storage page's
  inventory modal and the Trash notice; the setup-time inventory (catalog
  absent, adopt) is P5. Studio reassociation rewrites `asset_id` and
  `storage_path` in the record rather than keeping the old name; ambiguous
  records are exported as files, not opened in Studio. Item records are
  capped at 2000 per receipt (the rest is preserved and counted).

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
- Release notes: add the adopt-or-fresh line to
  `docs/releases/v26.1.0-rc.1.md` and `.zh-cn.md` and simplify the
  "Coming from a beta build" steps there and on the en/zh upgrade pages
  (S12 wrote them for today's setup, which stops on a leftover primary).

### P5 delivered (2026-10-19)

- **Setup review and explicit adoption.** `AssessSetupPrimaryTarget` reviews
  `<default>/primary` into a closed `SetupPrimaryState` (`creatable`,
  `adoptable`, `layout_repairable`, `registered`, `nonempty_unmarked`,
  `marker_invalid`, `marker_unsupported`, `unavailable`) with the legal
  actions per state (`create`, `adopt`, always `fresh_elsewhere` before the
  primary exists), the existing identity (id, name, marker version) and a
  read-only recovery preview (the P4 plan folded into class counts). The
  shared create-target assessment and its token are reused, so the review
  binds the adoption like a create. `AdoptPrimaryRepository` admits only
  with the token (mandatory even without risks), every risk acknowledged and
  the `expected_repository_id` the administrator reviewed; it is journaled
  as `adopt_primary_repository` (baseline CHECK amended in place), preserves
  the private tree under an `adopt` receipt (P4), completes a missing
  `.lumilio/` or `inbox/` with empty directories only (a file or denied
  directory in their place is `ErrRepositoryLayoutObstructed`, Problem
  `storage/layout-obstructed`, registry 40), registers the identity as the
  primary with the Host Owner, queues the initial scan, replays its result
  for the same request id, rolls back a crash between moves, and is rolled
  back by startup journal recovery. A registered identity is the ordinary
  `repository/conflict`; a target that is no longer adoptable is
  `storage/assessment-stale`.
- **Fresh default during setup (default applied).** Before the Primary
  Repository exists, with zero Repositories under the default and no running
  operation on it, a different configured `storage.path` replaces the unused
  default registration (`switch_default_storage_location` with
  `confirmation_type = fresh_default` and `new_storage_location_id`): the
  new folder gets a marker or keeps an unregistered compatible one, the old
  tree is not touched, and startup journal recovery finishes or undoes an
  interrupted switch. After the primary exists the same change must prove
  the moved identity, as before. The Desktop pre-check lets a candidate
  without a Primary Repository through (absent, empty, or a compatible root
  without `primary/.lumiliorepo`) and refuses a Repository folder or an
  unreadable root marker; the Server decides at the controlled restart and
  a refusal rolls the intent back.
- **Catalog-first startup and the facade.** `ensureDefaultStorageForRuntime`
  serves a usable catalog in restricted recovery mode when the configured
  default is offline or carries a marker this build cannot adopt (fresh or
  registered at the unchanged path): nothing is provisioned, the reason is
  logged, and `AssessDefaultStorageLocation` names the state for the
  administrator. `EnsureRootLayout` no longer creates `storage.path`, so an
  offline default is not recreated as an empty folder; the instance stays
  ready and its runtime status active (the row-level offline reachability
  is what the Storage page shows), as the matrix requires.
  Fatal causes are classified by `app.ClassifyStartupFailure`
  (`catalog_lineage_unsupported`, `catalog_newer`,
  `configuration_incompatible`, `configuration_upgrade_required`,
  `default_storage_migration_rejected`, `default_storage_unavailable`) with
  operator guidance; `db.ErrCatalogNewer`, `config.ErrManifestIncompatible`
  and `config.ErrManifestUpgradeRequired` are the new sentinels. The CLI
  prints `DescribeStartupFailure`; the Desktop factory maps the codes to
  `catalog_incompatible`, `configuration_incompatible`,
  `storage_migration_rejected`, `storage_unavailable` with the Server's
  guidance as the message, and the runtime presentation labels them.
- **Transport and Web.** `GET /setup/status` is public and phase-only (no
  path, assessment or token); `GET /setup/primary-target` (administrator)
  carries the defaults, the default-storage state, the review and whether a
  native host is present; `POST /setup/primary-repository/adopt`. The shared
  `usePrimarySetup` / `PrimarySetupPanel` flow serves the bootstrap wizard
  and the access gate: create form, Adopt card (identity, recovery preview,
  layout note, backup-vs-rebuild disclosure, risk acknowledgement), blocked
  states with their reason, default-storage diagnosis, and the
  host-specific **Start fresh elsewhere** entry (Desktop Settings → Storage,
  or `storage.path`). Terminology registry: `adopt-primary-repository`
  (`Adopt Primary Repository` / `接管主资源库`), `fresh-elsewhere`
  (`Start fresh elsewhere` / `在其他位置重新开始`); zh 100%.
- **Evidence (R3, R12, startup).** `server/internal/storage`:
  `TestAdoptPrimaryRepositoryKeepsIdentityAndPreservesPrivateState`,
  `TestAdoptPrimaryRepairsAMissingLayoutButNotAnObstructedOne`,
  `TestAdoptPrimaryRollsBackACrashBetweenMovesAndRetries`,
  `TestLifecycleRecoveryRollsBackAnAdoptionInterruptedAfterItsFilesystemPhase`,
  `TestSetupPrimaryReviewClassifiesEveryTargetState` (P0 fixtures: leftover,
  missing layout, unmarked, corrupt, newer, absent, registered; bytes
  untouched), `TestFreshDefaultDuringSetupReplacesAnUnusedRegistration`,
  `TestLifecycleRecoveryRollsBackAnInterruptedFreshDefaultSwitch`,
  `TestEnsureDefaultStorageLocationNeverReplacesAnUnreadableRootMarker`.
  `server/app`: `TestDefaultStorageRecoveryKeepsRuntimeStartableInDegradedMode`
  (restricted startup cases), `TestClassifyStartupFailureNamesEveryCompatibilityCause`,
  and over HTTP `TestLeftoverPrimaryIsAdoptedDuringSetup` (public status
  without host facts, unauthenticated review and adopt refused, create still
  refuses, forged token and other identity refused, adopt keeps identity and
  hashes, assigns the owner, exactly one primary, ready, replay, held Trash
  and reassociated Studio record after the scan),
  `TestUnmarkedPrimaryIsNeverInitializedOverDuringSetup`,
  `TestFreshDefaultElsewhereBeforeThePrimaryExists` (restart with another
  `storage.path` before the primary, old tree untouched, primary created in
  the new default; the established default offline later keeps the instance
  ready and is not recreated). Service and handler tests for the
  phase-only status, the review projection (no Repository path), the adopt
  binding and every refusal mapping. Desktop:
  `TestPatchDraftAllowsAFreshDefaultCandidateWithoutAPrimary`,
  `TestClassifyGenerationFailureCarriesTheServersVerdict`, presentation
  labels. Web: `PrimarySetupPanel.test.tsx` (adopt body, no create over the
  leftover, unmarked with Desktop handoff, unusable default storage,
  creatable), `PrimaryRepositoryGate.test.tsx`, `problem.test.ts`.
- **Deviations.** The R3 "fresh-bootstrap E2E" is the Go HTTP acceptance
  through the same harness the Desktop embeds plus the Web flow specs; the
  Compose slice is P8. Adoption does not run `recovery.Reconcile` inline: the
  hold and the reassociation follow the queued scan exactly as an open does.
  The Desktop binding enum (`models.ts`) was updated by hand to the Go
  codes because the Wails generator is not available in the sandbox; the
  next `wails3 generate bindings` run produces the same entries.

### P6a delivered (2026-10-21)

Session S6a. The reconnect journeys, split from the volume work as the
schedule allows.

- **Positively unavailable, never inferred from denial.** `OriginalIdentityState`
  classifies one read-only look at a registered path (`online`, `absent`,
  `identity_differs`, `inaccessible`). Repository and Location reconnect admit
  only `absent` or `identity_differs`; an online original is
  `storage/original-online` (`ErrStorageLocationOriginalOnline` for Locations,
  same Problem); a denied or unknown read withholds the action with the new
  blocker `original_inaccessible` and alternative `check_access`, on the
  command, on the native task, and in the policy a menu renders from.
  Reconcile no longer flips a row offline on a failed read; it keeps the
  current reachability and logs (R6, no false Missing).
- **Reconnect is journaled.** `ReconnectRepository` (renamed from
  `RelocateRepository`; `relocateStorageLocation` → `ReconnectStorageLocation`,
  kinds `reconnect_repository` and `reconnect_storage_location`, baseline
  CHECK amended in place) replays the same request, conflicts on a different
  destination under it, and is settled by startup recovery from the catalog
  alone (row at the new path completes, row at the old path is recorded as
  failed; the command moves no file). The journal audit carries the
  administrator's confirmation type.
- **Runtime ownership follows a moved tree.** A tree renamed or remounted
  while the Server holds its runtime lock carries the lock file with it;
  claiming the new path used to contend with this process's own advisory lock
  and refused every such reconnect with `storage/ownership-unavailable`
  (found by the R2 acceptance test, the first wired-runtime reconnect).
  `claimRuntimeStoragePath` now recognizes a held lock by file identity and
  rekeys the ownership to the new path, for a Repository and for a whole
  Location with its children.
- **Storage Locations have a policy.** `DeriveStorageLocationActions`
  (`reconnect`, `detach`) with blockers `default_protected`,
  `registered_repositories`, `original_available`, `original_inaccessible`,
  `maintenance`, `pending_operation`; `StorageLocationActionUnavailableError`
  shares the `storage/action-unavailable` Problem and the host-action refusal.
  `ReconnectStorageLocation` admits through it before journaling and
  re-reads the original under its barrier; a child missing or different at the
  destination is a typed `storage/identity-mismatch` and nothing moves. The
  storage view carries `actions` / `unavailable_actions` per Location.
- **Resolve route.** A resumed command's typed refusals (policy verdicts,
  identity mismatch, ownership, busy, recovery) now reach the Web as their
  Problems from `POST /storage/native-tasks/{id}/resolve`; any other failure
  is the host-action Problem, never a bare 400.
- **Reconnect wording and prerequisites.** The row entry is **Reconnect** with
  its prerequisite as the entry hint; the external Location menu has a direct
  **Reconnect Storage Location** entry on a Desktop host, disabled with the
  Server's reason while the policy withholds it; the native modal titles and
  purposes say what Reconnect does and needs; Desktop picker and panel labels
  match; registry row `reconnect` (重新连接) with `Relocate` / `Locate on disk`
  as forbidden synonyms.
- **Evidence.** Storage: `TestReconnectRefusesAnInaccessibleOriginal`,
  `TestReconnectRepositoryIsJournaledReplayedAndRecovered`,
  `TestReconcileKeepsReachabilityWhenTheRegisteredPathCannotBeRead`,
  `TestDeriveStorageLocationActionsIsKindAware`,
  `TestStorageLocationReconnectAgreesWithThePolicy`,
  `TestReconnectFollowsATreeMovedWhileItsRuntimeLockIsHeld`,
  `TestRepositoryReconnectRollsBackCatalogWhenAuditPersistenceFails`, the
  existing agreement and relocate tests under the new names. Handler:
  `TestStorageViewProjectsStorageLocationActions`. App (R2 over the wired
  runtime, native task started over HTTP and approved through the in-process
  control): `TestMovedRegularRepositoryReconnectsThroughTheNativeTask` (moved
  tree reconnects with identity, Asset, album and bytes intact and the scan
  re-queued; online copy, wrong identity and denied original each refuse
  with their reason; the same destination reconnects once the original is
  positively gone), `TestMovedStorageLocationReconnectsEveryChildOrNothing`
  (view verdicts for default/online/moved Locations; partial tree refuses and
  moves nothing; complete tree re-points Location and children atomically).
  Web: `StoragePanel.spec.tsx` (withheld Location entry with the Server's
  reason, Default Storage Location never offers it, unavailable Location
  starts `locate_storage_location`), `RepositoryRowActions.test.tsx`.
- **Deviations.** Docker/standalone candidates still reach
  `ReconnectRepository` through the legacy `risk_confirmation` gate (P7 with
  the classifier removal). `DeleteStorageLocation` keeps its legacy checks;
  the policy's detach verdict agrees with them through `Is`.

### P6b — Volume semantics and default Apply evidence (remaining P6)

- Depends on P6a. Qualify destination volume semantics, preflight collisions
  and bounded revision-fenced key rebuild without false absence or dropped
  bindings; protected default/primary configuration handoff and Apply crash,
  rollback and success evidence.
- Risks: scan fences, config rollback after catalog cutover, rekey loss.
  Exit: Apply success, crash, rollback; case/normalization image tests.
  Linux GHA loop-mounted vfat/exFAT/casefold-ext4 and macOS `hdiutil`
  case-sensitive APFS/ExFAT images provide reproducible volume evidence.
- Edwin: review; revisit volume-semantics default. Real external disks and
  Windows NTFS are optional reference coverage.
- Release notes: the "Relocate is now Reconnect and never moves files" line
  landed with P6a in `docs/releases/v26.1.0-rc.1.md` and `.zh-cn.md`.

### P6b delivered (2026-10-25)

- **The rule lives with the repository, never with the host.** `repositories`
  carries `path_case` and `path_normalization` (no default, CHECK-bounded):
  the name comparison rule measured on the tree when it was created, opened,
  copied, adopted or reconnected. `pathsemantics.Probe` measures it
  behaviorally (a hidden temporary file looked up under a case-changed and an
  NFD/NFC-changed spelling, `os.SameFile` deciding); `pathsemantics.Qualify`
  requires the root and its private `.lumilio/` directory to agree, so a
  per-directory case-folding flag is `ErrMixed`, and a denied or unknown
  lookup is `ErrUnqualified`, never "sensitive". `HostDefault` is gone; the
  scanner, the known-content binder, Trash/Restore/Adopt commits and the
  recovery service all key under the row's rule (`scan.SemanticsOf`,
  `storage.RepositoryVolumeRule`). An unqualified tree is neither registered
  nor reconnected (`storage/volume-unqualified`).
- **Reconnect qualifies the destination, preflights, cuts over, rekeys.**
  `planVolumeRekey` compares the registered rule with the destination's;
  when they differ, `preflightRekey` reads the whole index in bounded pages
  and refuses with `PathKeyCollisionError` (`storage/name-collision`, count
  plus at most ten pairs) if two live entries would share one key, before the
  journal opens and with no row or byte changed. Otherwise the cutover
  transaction moves the path and the rule together
  (`UpdateRepositoryPathSemantics`) and enters maintenance, so no scan,
  upload or Trash command keys against a half-rebuilt index; the journal
  phase advances to `catalog_committed`; `rekeyRepositoryEntries` rewrites
  `path_key`/`parent_key` in revision-fenced writer transactions of at most
  256 rows (the scan-index writer budget), idempotently, and maintenance is
  released. Entry identities, paths, states and Asset bindings are untouched.
  A crash or failure after the cutover leaves the operation running;
  `recoverReconnectRepositoryOperation` and
  `recoverSwitchDefaultStorageOperation` finish the pass
  (`finishVolumeRekey`, `finishStorageLocationRekeys`) and release
  maintenance. The Storage Location reconnect qualifies and preflights every
  child before any row changes; children that need a rekey stay in
  maintenance until theirs completes.
- **Real volumes qualify the support list.** `task server:test:volumes` runs
  the `^TestVolume` rows against the images a workflow mounts and names in
  `LUMILIO_VOLUME_FIXTURES` (`testfixture.VolumeFixtures`, the documented
  rule per image kind in `testfixture/volumes.go`). `ci.yml` adds `server
  volume semantics (Linux filesystem images)` (loop-mounted FAT32, exFAT,
  plain ext4, case-folding ext4 with a `+F` tree) and `server volume
  semantics (macOS filesystem images)` (`hdiutil` APFS, case-sensitive APFS,
  HFS+, ExFAT), both required when `server/**` changes. Adding a filesystem
  means adding its image to a job and its row to the table, never a guess.
- **Desktop Apply crash evidence.** `Store.Reconcile` now also reports
  `SafeToStart`: a journal left where the previous generation is current
  (`prepared`, `stopping_previous`, `previous_stopped` with current=previous,
  `rollback_selected`), or where the journal already recorded the candidate
  ready (`committing`), starts on its own and is closed by the readiness
  promotion; a candidate selected but never proven ready (`previous_stopped`
  with current=candidate, `candidate_selected`, `stopping_candidate`) still
  waits for the user's Restore last known-good. `main.go` blocks the start
  only in the second case (finding: a crash while committing a candidate
  that had reached readiness used to force a manual restore that discarded
  the applied configuration). Every phase keeps the previous generation as
  the rollback target and ends with current, LKG, journal, loaded
  configuration and the catalog's Default Storage Location in agreement.
- **Evidence.** pathsemantics:
  `TestProbeAgreesWithObservedBehaviorAndLeavesNoFile`,
  `TestQualifyRequiresEveryDirectoryToAgree`,
  `TestProbeOfAnUnwritableDirectoryIsUnqualifiedNotSensitive`,
  `TestVolumeImageProbeMatchesTheDocumentedRule` (image jobs). Storage:
  `TestReconnectRefusesADestinationThatCannotTellIndexedNamesApart`,
  `TestReconnectRekeysTheIndexForTheDestinationVolume` (more than one
  batch, identities and revisions, replay, round trip),
  `TestInterruptedRekeyIsFinishedByRecovery`,
  `TestReconnectRefusesAnUnqualifiedDestination` (reconnect and Open),
  `TestStorageLocationReconnectRekeysEveryChildOnTheNewVolume`. App (R11
  over the wired runtime and real images):
  `TestVolumeReconnectAcrossFilesystemImages` (a Repository created on the
  image keys under its rule; one moved onto it reconnects through the native
  task, is rekeyed, and the scan that follows writes no absence while the
  Asset identity and bytes stay; a pair the image cannot tell apart is
  refused with `storage/name-collision` and nothing changes). Desktop:
  `TestEveryApplyCrashPhaseRecoversCoherently` (every phase and pointer
  placement), `TestRestoreLastKnownGoodWaitsForOwnershipRelease`,
  `TestEmbeddedRuntimeCrashAfterCandidateSelectedRestoresThePreviousGeneration`,
  `TestEmbeddedRuntimeCrashWhileCommittingKeepsTheReadyCandidate` beside the
  existing Apply success and rollback tests.
- **What the images found.** The first macOS run measured ExFAT as
  normalization-insensitive, unlike the Linux exFAT driver; the table records
  the measurement, which is why the rule is stored per Repository and never
  derived from a format name. The first Linux run found that no derived
  artifact could be published on FAT32 or exFAT: `WritePrivateFileImmutable`
  gave the complete file its immutable name with a hard link, which those
  filesystems refuse, so every thumbnail failed and enrichment waited on it
  forever. The publish now falls back to an existence check and a rename
  where hard links are refused, the pattern the trash and staging moves
  already used; `TestRepositoryFSImmutablePrivateWritePublishesWhereHardLinksAreRefused`
  stands in for the volume on the runner and the image jobs exercise the real
  refusal. The blackbox processing waiter now names failed and retrying
  per-file stages from the catalog when it times out, so a stage blocked
  behind another is diagnosed from the run.
- **Deviations.** Only the root and `.lumilio/` are probed; a case-folding
  flag set on a deeper subdirectory after the fact is not detected at
  registration (a per-directory check during the walk is the follow-up if a
  fixture ever shows it). Windows NTFS and real removable disks remain
  optional reference coverage; the image table is the support list.

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
- Edwin: review. The Mac pass (removable non-default `/Volumes` create in
  en/zh; externally moved regular reconnect plus wrong-marker/online-copy
  refusal; reinstall Adopt and separate fresh-elsewhere; default
  Apply/restart; Windows optional) is recorded here as the first rc.2
  checkpoint with its exact steps, not run before rc.1 (2026-10-13). Its
  rc.1 stand-ins are the embedded Desktop picker-controller and
  runtime-config harness (R2, R9), the macOS and Windows CI jobs, and the
  `web:test:storage-recovery` slice. Edwin confirms the tag when P0–P8 and
  core quality hold.

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
| R2 — Desktop relocate/reconnect failure                                       | Go Server integration + embedded Desktop picker/runtime-config harness + Web flow; valid regular tree externally renamed away from registered path, same-UUID online copy, wrong-UUID tree, whole-Location copy with translated children/default primary | For positively unavailable original and authorized matching destination, picker → review → command retains UUID, Asset/album bindings, updates paths atomically and fences scan/watch work; bytes are unchanged. Empty/wrong/unauthorized/online-copy targets refuse with specific reachable recovery; denied/unknown original does not authorize reconnect. Whole Location validates every child before any update. Default selection changes only draft until Save/Apply; Apply/restart makes config/current/LKG/catalog coherent; candidate failure/crash rolls back coherently. Primary row routes to default recovery, never ordinary reconnect. Picker returns a path then silent no-op is a regression: selection always reaches visible review, reasoned refusal, or success | P6a delivered for regular and Location reconnect over the wired runtime (`server/app` reconnect acceptance tests); default Apply crash/rollback P6b (protected-action boundary in P3) |
| R3 — leftover primary dead end                                                | Go app/API integration, shared primary-setup Web flow, fresh-bootstrap E2E; `NewLeftoverPrimary`, fresh app-state/catalog, valid root with corrupt/newer/missing primary variants, `NewUnmarkedPrimary`, retained incompatible catalog/config variants   | After admin authentication and before readiness, Adopt and fresh-elsewhere are reachable. Explicit Adopt keeps UUID/original hashes, assigns Host Owner, creates exactly one primary, queues scan and recovery hold, reaches ready. Fresh-elsewhere preserves old tree and registers only new fixed primary. No implicit adoption; nonempty unmarked/corrupt/newer bytes untouched; backup vs rebuild disclosure present. Registered offline default keeps ready, is not recreated or replaced. Public status/unauthorized requests expose no host details                                                                                           | P5 delivered (Go app/API, storage, Web flow); private holds from P4 |
| R4 — `.lumilio/` isolation, inherited expiry and edit loss                    | Go integration + private-recovery Web flow; `NewLeftoverPrimary` opened as regular, adopted, and `CopyRepository` registered independently in fresh catalogs; injected crash boundaries and clock                                                        | Hash/tree inventory proves originals, Trash/info, Studio, staging, unknown children remain recoverable, with reachable recovery item actions after success/failure/restart. Trash view rebuilds source identities correctly, inherited entries remain held past ordinary expiry until explicit action, new deletes follow retention. Studio attaches only to proven content hash/size/source, ambiguous/newer records export/review without guessing; staged work never auto-resumes. Retry does not remint UUID or repeat moves                                                                                                                     | P4; adoption variant P5                                          |
| R5 — rename race (unsafe outcome inferred by audit)                           | Deterministic Go integration/concurrency; valid marked Repository, external marker replacement or externally moved matching tree; hold/release operation-boundary hooks introduced with fix, no sleeps                                                   | Park Rename before lease acquisition, complete reconnect or replace marker, then release Rename. Re-query catalog and re-read both old/new markers: current leased row/path is used; only raw name changes; all other config/identity fields survive. A replaced identity produces specific stale/mismatch refusal and no overwrite, including rollback. Replay same payload returns same result, changed payload conflicts                                                                                                                                                                                                                          | P3                                                               |
| R6 — stat panic / denied-as-absent / placeholder masking                      | Go unit observation + Go integration; valid tree, P1 observer returns nil info + `fs.ErrPermission`/other non-NotExist stat/read errors, genuine missing path and cloud placeholder controls                                                             | Public assessment/validation returns without panic; denial is permission-denied, arbitrary failures unknown, neither is absent/offline/placeholder. Re-query catalog/entries: no false Missing, no reconnect capability from unknown original, no marker provisioning. Only positively absent originals enable applicable recovery                                                                                                                                                                                                                                                                                                                   | P1; command/HTTP variants P2–P3                                  |
| R7 — foreground write probes / inconsistent network classification            | Go unit + API integration; P1 write-incapable observer/event recorder, valid root/child tree snapshots; injected NFS/SMB/CIFS/AFP/sshfs/WebDAV/Windows remote-drive facts, Windows removable, cloud resident/evicted controls                            | Setup/status/candidate/assessment requests perform zero create/write/lock/reconcile/expiry calls and leave tree unchanged, including transient-event log. Every network variant returns the same unsupported fact before consent; create/open/reconnect cannot confirm past it. Removable/local-cloud facts produce reviewable risk, unavailable placeholders materialize/retry, permission errors remain distinct. Windows classification is exercised without hardware                                                                                                                                                                             | P1 core; P2 transports/review; P6 reconnect                      |
| R8 — parent/child risk and continuity                                         | Go unit/integration; valid Location with independently mounted-child facts (parent local A, child B, B unchanged across samples), then missing/corrupt parent marker and healthy registered child                                                        | Child risk/support uses B; unchanged B has no mount-changed warning merely because A differs. Actual B replacement changes continuity; capacity grouping is independent. Parent fault does not deny registered child read/rename/detach, while new destination authorization still rejects invalid parent scope                                                                                                                                                                                                                                                                                                                                      | P1; rename/detach boundary P3                                    |
| R9 — Open Existing / conflict handoff / external reconnect UI omissions       | Web flow + embedded Desktop picker-controller integration; non-default compatible marked child, actual StoragePanelFlow composition, new-create conflict, complete external Location                                                                     | Native-backed Open launches native selection for chosen Location instead of default candidates; standalone bounded candidates retain selected context. Conflict handoff reaches Open/reconnect/copy review with location/child retained. External Location menu has direct reconnect entry. Cancel/stale/expired picker retains form and returns legal restart action; task paths/nonces never leak to HTTP                                                                                                                                                                                                                                          | P2 create handoff; P3 task outcomes; P6a direct Location entry and Location task (Web spec, app acceptance) |
| R10 — illegal protected actions / generic unreachable failures / default hint | Go policy/API table integration + Web flow/presentation tests; adversarial capabilities, primary/default and regular/offline trees, owned/busy task controls                                                                                             | Menus never invent primary rename/copy/detach/ordinary reconnect; protected default hint directs to executable config/operator recovery. Every public Problem and async reason has an actor/phase-correct descriptor whose route/host entry can actually be followed from a real failure. Domain reasons survive native transport; auth/phase guards hold, arbitrary browser host paths are rejected; exhausted enum/risk/action cases have literal en/zh copy                                                                                                                                                                                       | P3 role/task policy; P5 setup reachability delivered; P7 exhaustiveness |
| R11 — host-default scan-key volume debt                                       | Go integration + CI filesystem-image fixtures; sensitive/insensitive and normalization-distinct names, supported cross-volume reconnect and deliberate destination collisions                                                                            | Destination semantics determine keys; reconnect/rekey preserves entry IDs and Asset bindings, suppresses false absence, detects every collision before cutover, refuses without dropping rows or overwriting bytes. Unknown/mixed/directory-specific semantics refuse affected mutation until qualified by a reproducible fixture                                                                                                                                                                                                                                                                                                                    | P6b delivered (storage rekey tests on the runner's filesystem with an injected destination rule; `TestVolumeReconnectAcrossFilesystemImages` and the probe table on the Linux and macOS image jobs) |
| R12 — marker/layout distinction and unsafe marker writes                      | Go unit/integration; all `Marker` states, `MissingPrivate`, missing inbox and required-directory file obstruction, injected marker replace/write failures                                                                                                | Compatible marker remains compatible when layout missing; explicit repair adds only allowed missing dirs and preserves UUID/originals. File obstruction/denial blocks repair; absent/denied/corrupt/unsupported markers stay distinct. Atomic-write failure leaves complete old/new marker, guarded replacement/rollback never clobbers external identity                                                                                                                                                                                                                                                                                            | P1 marker primitive; P3 command guard; P5 repair/adopt delivered |

**R2 inputs / Edwin's macOS Desktop repro (2026-10-07):** clicking Relocate
opens Finder; he selects a folder, then nothing happens: no error, no change,
no receipt. Audit pointers: `RepositoryRowActions.tsx:115`, `host_action.go:420`.
R2 must cover **picker returns a path, then silent no-op**: every outcome after
selection must be visible as review, refusal with reason, or success. Exact
destination type/path/marker IDs and whether the original was online are
optional refinements for P6, not blockers. P6a finding (2026-10-21): the first
wired-runtime reconnect of a tree moved while the Server ran failed with
`storage/ownership-unavailable`, because the runtime lock file travels with
the tree and the new-path claim contended with the process's own lock; the
Web showed only the generic resolve failure. Both are fixed (ownership
follows the moved lock; the resolve route maps typed refusals). The P3b task
outcomes already made the remaining outcomes visible.

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
and private recovery. Real native picker coverage is an rc.2 checkpoint
(2026-10-13); for rc.1 the native picker and Apply paths are proven by the
embedded Desktop harness on the macOS and Windows CI jobs. No
hardware/performance gate per phase.

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
phase, this plan's completion, or rc.1. No real-machine validation gates
rc.1 (2026-10-13); the Mac pass, with Windows optional, is the first rc.2
checkpoint.
