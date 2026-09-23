# Release hardening week

Status: active, created 2026-09-22 for a release candidate on 2026-09-29.
Landed on `dev` so far (`bdd1375..74526ae`): Processing two-pattern pass, Agent
per-tab transcript and retry, Person Recognition relation decoding, UI
language normalization (Storage page crash), `asset_handler.go` and desktop
`App.tsx` splits, ML-tab cleanup, and the Processing stage grid
([decision](../../../.agents/decisions/2026-09-22-processing-stage-grid.md)).

Goal: a `YY.TRAIN.PATCH-rc.1` tag on `main`, promoted from a green `dev`, with
every release-blocking item below closed or explicitly deferred with a reason.
This plan is not a feature roadmap: anything not needed to ship safely is a
follow-up in the tech-debt tracker.

## Non-goals

- New product features beyond the Processing stage grid.
- Durable Agent chat history (needs a retention and privacy decision).
- Changing the pipeline, River, or the Catalog model.

## Fixed contracts

- `dev` is the integration branch; `main` changes only through one
  `dev` → `main` promotion PR (see `CLAUDE.md`).
- A blocker is a bug that loses or corrupts user data, blanks or blocks a core
  flow (sign-in, upload, browse, view, search, albums, people, share, storage,
  backup/restore), or breaks install/upgrade. Everything else is deferrable.
- No check is skipped, disabled, or quarantined to reach green.

## Execution phases

### Phase 0 — Green baseline
- [ ] Full CI green on `dev`, including Desktop native and the E2E slices
  CI runs.
- [ ] Confirm `internal/llm` `ark` conformance passes in CI. It fails in the
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
  Settings/Users, Storage admin, Map. Record results here.
- [ ] Playwright specs for the three highest-risk untested flows: Share link
  create/open/revoke, Storage admin add/verify Repository, People merge.
- [ ] Every smoke failure is fixed or filed with a blocker/deferred verdict.

### Phase 4 — Debt triage
- [ ] Decide blocker or deferred for each tracker item: Music embedded covers
  (placeholder today), Event late-EXIF fixture and legacy recovery, video
  semantic operation-scoped E2E proof, Linux bind-mount capacity test.
- [ ] `NewShareLinkService` panics on secret-key failure at construction;
  return an error so startup reports a diagnosable failure instead.

### Phase 5 — Low-risk cleanups (only if Phases 0–4 are done)
- [ ] Pure-move splits verified line-for-line: `music_service.go`,
  `asset_service.go`, `app/app.go`, `StudioEditor.tsx`, `AccountTab.tsx`.
- [ ] Agent: per-tool progress and duration in tool chips; copy-answer action.

### Phase 6 — Release candidate
- [ ] Release notes (user-facing, bilingual per the terminology registry).
- [ ] Upgrade test: an existing catalog from the last published version starts
  and migrates; a backup from it restores.
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
- An upgraded catalog and a restored backup both open and browse correctly.
