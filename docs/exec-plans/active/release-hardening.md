# Release hardening week

Status: active, created 2026-09-22 for a release candidate originally
targeted at 2026-09-29. On 2026-09-24 the owner made the rc.1 date **TBD**
until the RC blockers #222 and #223 land.
As of 2026-09-24: Phases 0–2 done; Phase 3 E2E specs done; `dev` at
`f598310a` is fully green in CI on draft promotion PR #210 (every job and all
eight E2E slices, including the new `@people`). Remaining work is split into
four child plans, each written to be picked up by a fresh session:
[repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md) (blockers #222 and #223: scan index, trash, missing, purge),
[rc-compat-baseline.md](rc-compat-baseline.md) (versioned-format baseline and upgrade paths from rc.1),
[rc-smoke-checklist.md](rc-smoke-checklist.md) (Phase 3 manual smoke), and
[rc-release.md](rc-release.md) (notes, release workflow, promotion, tag).
Phase 4 verdicts below await the user. **RC is gated on GitHub issues**: the
milestone `v26.1.0-rc.1` lists every issue that blocks the tag (the user adds
more over the coming days); nothing is promoted or tagged while it has open
issues — see "RC blocker issues" below.

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
- No check is skipped, disabled, or quarantined to reach green.

## RC blocker issues

- Source of truth: `gh issue list --milestone v26.1.0-rc.1 --state open`
  ([milestone](https://github.com/EdwinZhanCN/Lumilio-Photos/milestone/1)).
  The user adds blockers there; do not add or remove issues from it without
  the user.
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
- Run the child plans in this order:
  - repository-index-and-asset-lifecycle first, because it changes the
    catalog baseline, scanning, Storage, and delete. rc-release Phase 0
    (release workflow de-risk) and rc-compat-baseline Phases 0–2 can run in
    parallel with it.
  - rc-compat-baseline Phases 3–4 and rc-smoke-checklist Part A only after
    its last PR merges and an RC image built from that `dev` exists on the
    radxa.
  - rc-release Phases 1–4 last, and only once the RC blocker milestone is
    empty.
  - Other blocker issues can be worked in parallel with all of these; rerun
    the upgrade-restore and smoke rows that a blocker fix touches.
- Remote Docker host `radxa-x4` (Intel N100, 7.5 GiB, Fedora 44, fish shell):
  build images on the Mac for `linux/amd64`, ship with `docker save | gzip -1
  | ssh radxa-x4 'gunzip | docker load'`, run there. For the E2E stack, do not
  use `task web:e2e:up` against the remote (it passes `--build`); use
  `DOCKER_HOST=ssh://radxa-x4 docker compose -f web/e2e/compose.yml -p
  lumilio-photos-e2e up -d --no-build --wait`, tunnel ports 16657–16659, and
  run slices from the Mac with `LUMILIO_E2E_DOCKER_HOST=ssh://radxa-x4`.
- A real Lumen Hub runs on the radxa at `:50051` (face, siglip, ocr).
- CI runs only on PRs and `main` pushes, and a PR into `dev` only runs the
  jobs its paths touch; #210's CI is the full baseline. CI fails on any flaky
  test (`failOnFlakyTests`).

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
- [ ] Manual smoke checklist on a fresh Docker Compose install and on Desktop
  (macOS or Windows): People, Albums/Collections, Share links, Studio,
  Settings/Users, Storage admin, Map. Owned by
  [rc-smoke-checklist.md](rc-smoke-checklist.md).
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
    legacy-claims risk is checked by rc-compat-baseline Phase 1 (Events
    present and stable after upgrading a beta.1 catalog); promote to blocker
    only if that fails.
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
- [ ] RC blockers #222 (scan index) and #223 (Asset lifecycle) —
  [repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md).
- [ ] Release notes (user-facing, bilingual per the terminology registry) —
  [rc-release.md](rc-release.md) Phase 1.
- [ ] Compatibility baseline: every app-owned persisted format reset to
  version 1, pre-release data rejected clearly, forward upgrade paths tested,
  rc.1 fixture locked at tag time — [rc-compat-baseline.md](rc-compat-baseline.md).
  (Upgrading from beta.1 is out of scope: pre-release data is not migrated,
  user decision 2026-09-24.)
- [ ] Release workflow proven on current `dev` (the v26.1.0-beta.2 release run
  failed in the Windows portable build) — [rc-release.md](rc-release.md)
  Phase 0.
- [ ] `dev` → `main` promotion PR, green CI, then the `rc.1` tag; the release
  workflow builds every Desktop and Server artifact.
- [ ] Smoke the published artifacts once (Docker image digest, one Desktop
  build).

## Validation boundaries

- CI green on the promotion commit, with no skipped or quarantined checks.
- zh coverage 100% after a clean extraction.
- The smoke checklist is recorded with a verdict for every flow.
- No open blocker; each deferred item has a tracker entry naming its owner
  path and user impact.
- Milestone `v26.1.0-rc.1` has no open issues.
- An upgraded catalog and a restored backup both open and browse correctly.
