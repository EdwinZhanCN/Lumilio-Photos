# RC release: notes, pipeline, promotion, tag

Status: active, created 2026-09-24. Not started. Child of
[release-hardening.md](release-hardening.md) (Phase 6). Target: `rc.1` tag on
`main` by 2026-09-29. Draft promotion PR **#210** (`dev` → `main`) is open and
its CI was fully green at `dev` `f598310a` (all jobs, all 8 E2E slices).

Goal: a published `rc.1` whose Server image and Desktop artifacts built from
the promotion commit, with bilingual release notes, smoked once.

## Non-goals

- Changing the versioning scheme or the release workflow's design. Fix only
  what blocks this release.
- Store/notarized distribution (Desktop ships ad-hoc DMG and Windows
  installer/portable as today).

## Fixed contracts

- Version: `YY.TRAIN.PATCH-rc.N` → expect **`v26.1.0-rc.1`**; confirm against
  the tag pattern in `.github/workflows/release.yml` and the existing tags
  (`v26.1.0-beta.1` published; `v26.1.0-beta.2` tag exists but its release
  run failed).
- `main` changes only via the single promotion PR #210; never push to `main`
  directly. Merge only when [release-hardening.md](release-hardening.md)
  validation boundaries hold and [rc-upgrade-restore.md](rc-upgrade-restore.md)
  and [rc-smoke-checklist.md](rc-smoke-checklist.md) are closed.
- Tags and releases are outward-facing: the user confirms before any tag is
  pushed.
- **RC blocker gate (hard stop).** The GitHub milestone
  [`v26.1.0-rc.1`](https://github.com/EdwinZhanCN/Lumilio-Photos/milestone/1)
  holds every issue that blocks the RC; the user keeps adding to it. Before
  marking #210 ready, before merging it, and again immediately before
  tagging, run:
  `gh issue list --milestone v26.1.0-rc.1 --state open`
  If it lists **anything**, stop: do not mark ready, merge, or tag. Report the
  open issues to the user and end the session. An empty list is necessary
  but not sufficient; the user still confirms the tag explicitly, because
  new blockers may arrive after the check.

## Execution phases

### Phase 0 — De-risk the release workflow (do first)
- [ ] Find why `v26.1.0-beta.2` failed: release run on 2026-09-05, job
  "Windows installer and portable app", step "Build portable app"
  (`gh run list --workflow release.yml`, `gh run view <id> --log-failed`).
  Check whether a later commit fixed it (`git log v26.1.0-beta.2..origin/dev
  -- .github/workflows/release.yml desktop/`; e.g. `36a22ea0 ci: stop
  setup-vp running an implicit root vp install`).
- [ ] Prove the release workflow on the current `dev` without publishing:
  use its `workflow_dispatch` path if it builds without publishing, or read
  the workflow and reproduce the failing Windows portable build step locally
  / in a throwaway branch. Do not create a real `v*` tag for this.
- [ ] Any fix lands as a PR into `dev` (and therefore #210).

### Phase 1 — Release notes (bilingual)
- [ ] Collect user-facing changes since `v26.1.0-beta.1`:
  `git log --merges --oneline v26.1.0-beta.1..origin/dev` plus PR bodies
  (`gh pr view <n>`). This week's user-visible fixes include: Storage page
  asset counts (#209), search vectors after a full reindex (#212), Event
  edits no longer reverted (#213), sign-out no longer resets the login form
  (#214), playback no longer hangs when a transcode finishes (#216), share
  link startup errors are diagnosable (#208), Processing stage grid.
- [ ] Write notes for users, not developers: grouped (New, Fixed, Known
  issues), no internal names. Known issues come from
  `docs/exec-plans/tech-debt-tracker.md` items the user deferred (e.g. music
  embedded covers placeholder; manual scan right after a file lands).
- [ ] English and Simplified Chinese, using the canonical terms in the
  `lumilio-frontend-i18n` skill's terminology registry. Put them where the
  release workflow reads them (check `release.yml`; otherwise the GitHub
  release body) and, if the docs site has a changelog/upgrade page, update
  `site/docs/en` and `site/docs/zh-cn` together.
- [ ] Include upgrade guidance from rc-upgrade-restore.md results (back up
  first; what the migration does).

### Phase 2 — Promote
- [ ] Run the RC blocker gate (Fixed contracts). Stop if anything is open.
- [ ] Update #210's body with the final list; mark it ready for review; CI
  must be green on its final head with no skipped/quarantined checks.
- [ ] User reviews and merges #210.

### Phase 3 — Tag and publish (user confirms first)
- [ ] Run the RC blocker gate again right before tagging; stop if anything is
  open, and ask the user to confirm there is nothing new to add.
- [ ] Tag `v26.1.0-rc.1` on the merged `main` commit and push the tag; watch
  the release workflow to completion (`gh run watch`).
- [ ] Verify outputs: GitHub pre-release created with notes; Server image
  `ghcr.io/edwinzhancn/lumilio-server` with the expected tags and a recorded
  digest; Desktop macOS DMG and Windows installer/portable attached.

### Phase 4 — Smoke the published artifacts
- [ ] Docker: on the radxa, pull the published image **by digest**, fresh
  install with `deploy/compose/compose.yml`, setup wizard, upload a photo,
  view it. (Pull on the radxa directly — this checks the registry artifact,
  not a local build.)
- [ ] Desktop: the user installs the published build on macOS or Windows,
  onboarding, import, view.
- [ ] Record digests and results in release-hardening.md; tick Phase 6;
  complete the parent plan per the exec-plan skill.

## Validation boundaries

- Milestone `v26.1.0-rc.1` has zero open issues at tag time, and the user
  confirmed the tag.
- Release workflow green for `v26.1.0-rc.1`; every Desktop and Server
  artifact present.
- Published image digest recorded and smoke-tested from the registry.
- Bilingual notes published with the pre-release.
