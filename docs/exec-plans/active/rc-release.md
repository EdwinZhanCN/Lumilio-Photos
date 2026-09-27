# RC release: notes, pipeline, promotion, tag

Status: active, created 2026-09-24. Phase 0 done (release workflow green on `dev` `c397cf38`); Phase 1 (release notes) next, best written once the milestone blockers land. Child of
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
  validation boundaries hold and [rc-smoke-checklist.md](rc-smoke-checklist.md)
  is closed (the compatibility baseline is done: PR #225, [decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md)).
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
- [x] Find why `v26.1.0-beta.2` failed: its Windows job's `desktop:common:generate:icons:windows`
  ran `wails3 generate icons` without `-macfilename`, whose default
  `build/darwin/icon.icns` resolves under the task's `build/` dir to a path
  that does not exist ("open build/darwin/icon.icns"). `36a22ea0` fixed an
  unrelated CI setup problem; PR CI never runs this packaging task.
- [x] Prove it on current `dev` without publishing a release: dispatch run
  36292509609 (2026-09-27, `dev` `5fba7f44`, moved only the `edge` image tag)
  passed metadata, SPA, both Server image arches, the manifest, and the macOS
  DMG; the Windows job failed with the same error. Reproduced locally (exit 1)
  and fixed with `-macfilename ""` (exit 0, `.ico` only).
- [x] The fix landed in `dev` (#227). Dispatch run 36334077943 (2026-09-27,
  `dev` `c397cf38`) passed every job: metadata, SPA, Server image amd64 and
  arm64 plus manifest (`edge`), macOS DMG (arm64), and the Windows installer
  and portable app; the GitHub Release job was skipped as designed for a
  dispatch. Artifacts: `windows-amd64-setup`, `windows-amd64`, `dmg-arm64`.

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
- [ ] Upgrade guidance: rc.1 is the first supported release; data from
  pre-release builds (`v1.0.0-beta.*`, `v26.1.0-beta.*`) is not migrated —
  start fresh. From rc.1 on, updates upgrade in place and take an automatic
  backup first ([decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md)).

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

- [ ] Lock the rc.1 compatibility fixture from the published image and land
  it via a PR into `dev`: with the published rc.1 image, create a small
  catalog with user state (album with a cover, renamed Event and person,
  share link), its config, one backup, and the Storage Location and
  Repository markers; commit them under a testdata path; add a CI test that
  upgrades them with the current build on every run and checks counts, user
  state, and untouched originals. Each later release adds its own fixture.
  From the tag on, the baseline and every released step are frozen
  (`server/migrations/steps/README.md`; [decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md)). This is the last
  item of issue #221's scope.

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
