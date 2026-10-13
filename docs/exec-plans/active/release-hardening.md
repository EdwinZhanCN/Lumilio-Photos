# Release hardening

Status: active, created 2026-09-22 for a release candidate originally
targeted at 2026-09-29. On 2026-09-24 the owner made the rc.1 date **TBD**
until the RC blockers #222 and #223 land.
rc.1 is a quality milestone, not a date deadline: the storage refactor completes fully,
storage-repository bugs are eradicated, and core paths hold up through a stretch of real
use by Edwin and a few friends. It can be postponed.
As of 2026-09-24: Phases 0–2 done; Phase 3 E2E specs done; `dev` at
`f598310a` is fully green in CI on draft promotion PR #210 (every job and all
eight E2E slices, including the new `@people`). Remaining work is split into
the following child plans, each written to be picked up by a fresh session:
[repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md) (blockers #222 and #223: scan index, trash, missing, purge),
[storage-repository-lifecycle.md](storage-repository-lifecycle.md) (P0–P8: storage assessment, creation, private recovery, setup adoption, and reconnect),
the compatibility baseline (done 2026-09-24, PR #225; [decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md)),
[rc-smoke-checklist.md](rc-smoke-checklist.md) (Phase 3 manual smoke), and
[rc-release.md](rc-release.md) (notes, release workflow, promotion, tag).
Phase 4 verdicts below await the user. **RC is gated on GitHub issues**: the
milestone `v26.1.0-rc.1` lists every issue that blocks the tag (the user adds
more over the coming days); nothing is promoted or tagged while it has open
issues — see "RC blocker issues" below.

**2026-10-13 re-plan (owner constraint: no physical machine for the
foreseeable future; every remaining session runs in Claude Code cloud).**
Every pre-tag gate is now CI- or checkout-reproducible; the Mac pass and the
published Desktop install are the first rc.2 checkpoints
([decision](../../../.agents/decisions/2026-10-13-rc1-evidence-without-hardware.md)).
Where things stand: #222/#223 are implemented and wait only on the
deterministic acceptance tests listed in their 2026-10-07 audit comments;
the storage refactor is at P1 of P8 (#246, #247) and owns #242; `dev` is
126 commits ahead of `main`, and #210's CI has been red since #241 because
`main` carries five commits `dev` never received (the export dialog from
#201), so `verify:generated` sees Atlas drift on the merge ref. The
sequenced session list under "Handoff" is the single order of work.

Landed before this plan (`bdd1375..74526ae`): Processing two-pattern pass,
Agent per-tab transcript and retry, Person Recognition relation decoding, UI
language normalization (Storage page crash), `asset_handler.go` and desktop
`App.tsx` splits, ML-tab cleanup, and the Processing stage grid
([decision](../../../.agents/decisions/2026-09-22-processing-stage-grid.md)).
Landed during it (PRs into `dev`): #205 i18n extractor + low-power E2E
timing, #206 commit-metric race, #207 scan settle race, #208 share-link
startup error + E2E, #209 Storage asset count (blocker) + E2E, #211 tracker,
#212 paged reindex abandoned pages (blocker) + video flake, #213 Event edits
reverted by in-flight rebuilds (blocker) + events flake, #214 sign-out double
login mount + TOTP flake, #215 People merge E2E with recorded faces, #216 web
media URL switched representation mid-playback (blocker).

Goal: a `YY.TRAIN.PATCH-rc.1` tag on `main`, promoted from a green `dev`, with
every release-blocking item below closed or explicitly deferred with a reason.
This plan is not a feature roadmap: anything not needed to ship safely is a
follow-up in the tech-debt tracker.

## Non-goals

- New product features beyond the Processing stage grid, and beyond the
  repository trash and Missing view required by #223.
- Durable Agent chat history (needs a retention and privacy decision).
- Changing the pipeline, River, or the Catalog model. **Exception (owner
  decision, 2026-09-24):** the repository scan index (#222) and the Asset
  lifecycle (#223) replace the Repository Observation Engine tables and
  `assets.is_deleted` before rc.1. A quadratic scan cost holds the single
  writer, and ghost, swallowed, and orphaned Assets are data-integrity
  defects. The work is owned by
  [repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md).

## Fixed contracts

- `dev` is the integration branch; `main` changes only through one
  `dev` → `main` promotion PR (see `CLAUDE.md`).
- A blocker is a bug that loses or corrupts user data, blanks or blocks a core
  flow (sign-in, upload, browse, view, search, albums, people, share, storage,
  backup/restore), or breaks install/upgrade. Everything else is deferrable.
- Correctness tests are never skipped, disabled, or quarantined to reach green.
  Performance and hardware checks in declared non-required lanes are not quarantine.

## RC blocker issues

- Source of truth: `gh issue list --milestone v26.1.0-rc.1 --state open`
  ([milestone](https://github.com/EdwinZhanCN/Lumilio-Photos/milestone/1)).
  Only issues meeting the blocker definition belong in rc.1. #242 remains;
  #217 is outside rc.1, and #240 moves to rc.2: real-build updater proof needs
  a prior released build with an updater. The user adds blockers there; do not
  add or remove issues from it without the user.
- Work each issue in its own fresh session: branch from the latest `dev`,
  fix with a failing-first test where applicable, open a PR into `dev` whose
  body says `Closes #<n>`, and keep the Status of this plan current. The
  issue closes when the PR merges into `dev`.
- An issue that turns out to need a design decision (not just a fix) goes
  back to the user before code is written.
- The gate is enforced in [rc-release.md](rc-release.md) (Fixed contracts):
  no ready-for-review on #210, no merge, no tag while the milestone has open
  issues.

## Handoff: how to resume

- Start a fresh session in this repo, read this file, then the child plan you
  are working on. Each child plan lists its own environment, phases, and
  validation boundaries; keep its `Status:` and checkboxes current in the
  same PR that changes reality.
- Every session: branch from the latest `dev`, one PR into `dev` per
  session (body says `Closes #<n>` when it closes an issue), the narrowest
  checks per `lumilio-select-checks`, and never a skipped or quarantined
  correctness test. A session that ends without a mergeable PR leaves its
  branch pushed and a one-paragraph note in the child plan's `Status:`.
- The cloud sandbox has Go, Node, Task, Chromium, and the GitHub MCP tools,
  but no Docker daemon and no display. Server and Web unit/integration
  tests run locally; the Compose E2E slices, Desktop native jobs, and
  anything that needs a published image run only on GitHub Actions, so a
  session proves those by pushing and reading the PR's checks.
- Required CI runs only on PRs and `main` pushes, and a PR into `dev` only
  runs the jobs its paths touch; #210's CI is the full baseline. CI fails
  on any flaky test (`failOnFlakyTests`).
- Optional reference hardware (`radxa-x4`, Intel N100) is unavailable; the
  `lumilio-remote-qualification` skill stays for when it returns (#243).

### Session order (each row is one session and normally one PR into `dev`)

| # | Session | Closes / unblocks | Plan |
| --- | --- | --- | --- |
| S1 | Sync `main` into `dev`: merge `origin/main` (five commits, export dialog), run `task atlas:generate` and `task verify:generated`, re-read any Atlas view the new modules make stale before `task atlas:lock`. #210 turns green again. Close the stale draft #229 (its tests landed through #232–#235). | #210 CI | [rc-release.md](rc-release.md) Phase 0b |
| S2 | Storage P2 — assessment DTOs/endpoints, consent token, capability-backed Create, shared en/zh risk review. | #242 | [storage-repository-lifecycle.md](storage-repository-lifecycle.md) |
| S3 | Storage P3 — safe command boundary (reassess under barriers, rename refresh, role-aware actions, idempotent receipts). May split into two PRs: commands, then native selection/review. | — | same |
| S4 | Storage P4 — private preservation and recovery (held inherited Trash, Studio reassociation, journaled staging). Highest data risk; fault injection at every boundary. | — | same |
| S5 | Storage P5 — setup and startup recovery (Adopt Primary / fresh elsewhere, authenticated setup recovery, startup facade). | leftover-primary dead end | same |
| S6 | Storage P6 — reconnect journeys and volume semantics. Volume evidence comes from CI filesystem images (Linux loop-mounted vfat/exFAT/casefold-ext4, macOS `hdiutil` APFS/ExFAT); native picker paths from the embedded harness. May split: regular/Location reconnect, then default Apply. | silent Reconnect no-op | same |
| S7 | Storage P7 — remove competing classifiers and Desktop marker parsing; architecture checks that catch a deliberate violation. | — | same |
| S8 | Storage P8 — `web:test:storage-recovery` E2E slice wired into `ci.yml` path filters; evidence tables; extract decisions; delete the plan. The Mac pass is recorded as an rc.2 checkpoint, not run. | storage plan complete | same |
| S9 | #222 acceptance tests: no-change rescan counts hash calls and entry writes; album membership and cross-repository directory move; upload-then-scan zero rehash; watcher-off convergence; structural writer budget enforced by test or `architecturecheck`. | #222 | [repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md) |
| S10 | #223 acceptance tests: two-copy delete/restore with albums and faces; external deletion hidden from search, albums, counts; duplicate resolution into Trash and back; in-place edit with a real XMP/EXIF write, tags, rating/like, regenerated artifacts; fork and bind-without-merge snapshots; backup-before-delete restore rebuilding Trash from sidecars. Then complete that plan (decisions already recorded) and close #223. | #223 | same |
| S11 | Smoke-to-CI: a `@smoke` language-switch spec; the removable-Location create journey as the P2 flow spec plus Server injected-fact test (name them in the checklist); `release-smoke.yml` (dispatch: image reference or digest → fresh Compose install from the release bundle → setup → upload → browse → backup/restore; uploads the catalog, config, backup, and markers as the compatibility-fixture artifact). Rewrite the checklist's Results from CI run links. | smoke checkpoint | [rc-smoke-checklist.md](rc-smoke-checklist.md) |
| S12 | Release notes (en + zh-cn, registry terms), upgrade page, Known issues, the Desktop CI-only verification statement. | notes | [rc-release.md](rc-release.md) Phase 1 |
| S13 | Promote: blocker gate (milestone empty), update #210's body, mark ready, CI green on the final head; **Edwin merges**. | #210 | [rc-release.md](rc-release.md) Phase 2 |
| S14 | Tag: blocker gate again; **Edwin confirms the tag in writing**; the session pushes `v26.1.0-rc.1` on the merged `main` commit and watches the release workflow; verifies the GitHub pre-release, image tags and digest, DMG, Windows installer and portable app. | rc.1 published | [rc-release.md](rc-release.md) Phase 3 |
| S15 | Post-tag: dispatch `release-smoke` against the published digest; commit the rc.1 compatibility fixture with its CI upgrade test; record digests and results here; complete this plan and rc-release per the exec-plan skill; open the rc.2 checkpoints (Mac pass, published Desktop install, #240). | plans complete | [rc-release.md](rc-release.md) Phases 3–4 |

S9 and S10 touch `storage/scan`, `lifecycle`, and `server/app` tests only,
so they can run between any two storage sessions; S11 and S12 can run any
time after S2 (S11 needs the P2 evidence names). S2–S8 are strictly ordered.
S13–S15 wait for everything before them and for an empty milestone.

## Execution phases

### Phase 0 — Green baseline
- [x] Full CI green on `dev`, including Desktop native and the E2E slices
  CI runs: draft promotion PR #210 (2026-09-23, `dev` at `169ddc9b`) passed
  every job — server, web, Desktop macOS and Windows, site, producer pins —
  and all seven E2E slices. Getting there fixed two `dev` failures its first
  full run exposed (#206 commit-metric race, #207 scan settle race) and the
  Storage asset-count blocker (#209).
- [x] Confirm `internal/llm` `ark` conformance passes in CI (server job green
  on #210). It fails in the
  agent sandbox identically on untouched `dev`, so it is believed to be
  environment-specific; if CI also fails, it is a blocker. (2026-09-22: passes
  on a macOS host, supporting the sandbox theory.)
- Local baseline, 2026-09-22 at `b0d5b932`: `task test` green. `ci.yml` runs
  only on `main` pushes and PRs, so the 25 commits since `88400fc8` have no CI
  run; the promotion PR is the first. The E2E slices, run on the Intel N100
  qualification host, found three timing assumptions, all fixed:
  - `@smoke` music playback: default 5s poll for the first audio bytes while
    the host drains the seed backlog.
  - `@auth-hardening` lockout: four hashed logins did not fit the 1s
    rate-limit window.
  - `@agent-runtime` music audition: the same 5s audio-start poll. Both music
    specs now share `PLAYBACK_START_TIMEOUT` from `e2e/support/assets.ts`.
  - Not fixed: `agent-runtime.spec.ts` plain chat and the music-agent
    selection each missed a 5s reply-visibility wait once and passed on retry
    and on rerun. Watch them in CI before widening any timeout.
  - A clean rerun on a fresh stack passed every slice without retries except
    one `@video-regression` retry on `fetch failed: other side closed`, most
    likely the SSH port forward the remote run needs, not the spec.

### Phase 1 — Processing stage grid
- [x] Stage catalog read model, items, retry, diagnostics, and the stage-grid
  UI; its plan is complete and deleted, and its decision is recorded in
  `.agents/decisions/2026-09-22-processing-stage-grid.md`.

### Phase 2 — i18n integrity
- [x] Make table-driven copy extractor-visible: the Storage state,
  verification, and upload admission-reason tables now call
  `t("literal", "default")` per entry. Extraction was deleting 20 live keys
  (the tracker's ~80 had shrunk as other tables were rewritten).
- [x] One clean extraction: the only removals are 10 unreferenced keys and 4
  plural base keys superseded by `_one`/`_other`; no values changed; zh 100%
  (2215/2215).
- [x] Tracker item removed; `lumilio-frontend-i18n` now requires literal keys.

### Phase 3 — Flow coverage for untested journeys
- [ ] Tag-time core smoke checkpoint, run by CI on the promotion head
  (2026-10-13: no hardware): setup, upload/browse, Storage create including a
  removable location, Trash/restore, and language switch map to named
  specs and Server tests; rows without automatable evidence are listed as
  deferred to rc.2. Owned by [rc-smoke-checklist.md](rc-smoke-checklist.md).
- [x] Playwright specs for the three highest-risk untested flows: Share link
  create/open/revoke, Storage admin add/verify Repository, People merge.
  - [x] Share link create/open/revoke: `web/e2e/specs/share-links.spec.ts`
    (`@smoke`, `task web:test:browser`; the `browser_smoke` CI filter follows
    the share handler, service, and `web/src/features/share/**`).
  - [x] Storage admin: `web/e2e/specs/storage-admin.spec.ts` (`@smoke`) adds a
    Repository through the wizard, scans it from the row menu, and asserts
    the scan run, the ingested asset, and the Storage view. It found a
    blocker-candidate: `GET /api/v1/storage/view` never returns
    `asset_count` (the Storage page shows 0 Assets for every Repository)
    because `storage_handler.go` binds `dbtypes.JSON("ready")`, which is
    invalid JSON, and swallows the error; `ready` is also not a state the
    pipeline writes (`completed` is). Fixed on the same branch: the status
    queries bind the state as TEXT, the view counts every non-deleted Asset
    with an active occurrence whatever its processing state (user decision,
    matching the removal-impact dialog), and a count failure is a Problem
    instead of a silent omission. Verified end to end on the N100 with all
    release-hardening branches combined (every slice green, no retries).
  - [x] People merge: `web/e2e/specs/people-merge.spec.ts` (`@people`, new
    slice `task web:test:people`, CI filter `people_e2e`). It uploads five
    `demo`-profile portraits (plus one duplicate: clustering needs three
    faces per person), enables face recognition for the test, waits for two
    people, merges them in the edit dialog, and asserts one survivor owning
    all six faces/assets and a 404 for the merged id. Face results are real
    Hub recordings (antelopev2, five payloads) replayed by fakelumen;
    fakelumen now overlays recorded capabilities per service so recording
    face does not change SigLIP/BioCLIP/OCR for other slices. The slice
    fetches only its five portraits (`assets:sync --profile demo --asset …`,
    about 1.9 MB of LFS objects) instead of the whole demo profile. Later
    cleanup: move the portraits into the `e2e` profile in the next assets
    release and drop the selection.
- [ ] Every smoke failure is fixed or filed with a blocker/deferred verdict.

### Phase 4 — Debt triage
- [ ] Decide blocker or deferred for each tracker item (recommendations
  below; the user decides, then record the verdict here):
  - Music embedded covers (placeholder; audio as album cover 404s) —
    **defer**, list under Known issues in the release notes.
  - Event late-EXIF fixture and legacy recovery — **defer the fixture**; the
    user-visible half (edits reverted by rebuilds) was fixed in #213. The
    legacy-claims risk is moot for rc.1: pre-release catalogs are not
    migrated (compatibility decision, 2026-09-24).
  - Video semantic reprocess completion proof — **defer**; narrowed by #212
    (rebuild receipts are now observable), remaining gap is test-only.
  - Linux bind-mount capacity test — **defer**; privileged-test gap, no known
    user impact.
  - Manual scan within the settle window — **deferred** (user decision
    2026-09-23, tracker entry from #211). Superseded 2026-09-24: folded into
    #222 as a requirement (a scan that defers settling files schedules a
    delayed follow-up for that subtree). The tracker entry is deleted when
    that lands, and it is no longer a release-notes Known issue.
  - Follow-ups found this week (tracker entries added 2026-09-24):
    `LoginPage.signIn` 5s wait, share UI accessible names, pre-commit hook on
    `doc.md`-only commits, `useMergePeople` cast, unexplained idle
    `music-agent` pause — **defer** all; none is user-data or core-flow.
- [x] `NewShareLinkService` panics on secret-key failure at construction;
  return an error so startup reports a diagnosable failure instead.

### Phase 5 — Low-risk cleanups (only if Phases 0–4 are done)
- [ ] Pure-move splits verified line-for-line: `music_service.go`,
  `asset_service.go`, `app/app.go`, `StudioEditor.tsx`, `AccountTab.tsx`.
- [ ] Agent: per-tool progress and duration in tool chips; copy-answer action.

### Phase 6 — Release candidate
- [ ] Sync `main` into `dev` so #210's merge ref passes `verify:generated`
  again (S1; [rc-release.md](rc-release.md) Phase 0b).
- [ ] RC blockers #222 (scan index) and #223 (Asset lifecycle): implemented;
  the deterministic acceptance tests from the 2026-10-07 audit remain (S9,
  S10) —
  [repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md).
- [ ] RC blocker #242 and the storage refactor P2–P8 (S2–S8) —
  [storage-repository-lifecycle.md](storage-repository-lifecycle.md).
- [ ] Release notes (user-facing, bilingual per the terminology registry) —
  [rc-release.md](rc-release.md) Phase 1.
- [x] Compatibility baseline: every app-owned persisted format reset to
  version 1, pre-release data rejected clearly, forward upgrade paths tested
  and proven on an RC build — PR #225, [decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md). The rc.1 fixture is
  locked at tag time in [rc-release.md](rc-release.md) Phase 3.
  (Upgrading from beta.1 is out of scope: pre-release data is not migrated,
  user decision 2026-09-24.)
- [x] Release workflow proven on current `dev` (the v26.1.0-beta.2 release run
  failed in the Windows portable build; fixed in #227, dispatch run
  36334077943 green on `dev` `c397cf38`) — [rc-release.md](rc-release.md)
  Phase 0.
- [ ] `dev` → `main` promotion PR, green CI, then the `rc.1` tag; the release
  workflow builds every Desktop and Server artifact.
- [ ] Post-tag checkpoint: the `release-smoke` workflow installs the
  published Docker image by digest and runs setup, upload, browse, and a
  backup/restore round trip (S15). Installing a published Desktop build is
  an rc.2 checkpoint
  ([decision](../../../.agents/decisions/2026-10-13-rc1-evidence-without-hardware.md)).

## Validation boundaries

- CI green on the promotion commit. Correctness tests are never skipped, disabled, or
  quarantined to reach green. Performance and hardware checks in declared non-required
  lanes are not quarantine.
- Report zh catalog coverage after a clean extraction at tag time; 100% is
  a target, not a blocking gate.
- The smoke checklist core rows have tag-time verdicts from CI runs on the
  promotion head; rows that need a physical machine are recorded as rc.2
  checkpoints, not as passes.
- No open blocker; tracker entries with owner paths and user impact are
  encouraged for deferred items, but missing materials do not block merges.
- Milestone `v26.1.0-rc.1` has no open issues.
- An upgraded catalog and a restored backup both open and browse correctly.
