# RC smoke checkpoint (CI evidence map)

Status: active, created 2026-09-24 as a manual checklist; rewritten
2026-10-13 as a CI evidence map because no physical machine is available
and every session runs in Claude Code cloud
([decision](../../../.agents/decisions/2026-10-13-rc1-evidence-without-hardware.md)).
Child of [release-hardening.md](release-hardening.md) (Phase 3); session S11
in its order builds the missing evidence and fills Results. Closes before the
rc.1 tag. Rows marked **rc.2** need a person at a machine and are recorded
as the first rc.2 checkpoints, never ticked for rc.1.

Goal: every core flow has a named, CI-run proof on the promotion head of
`dev` (the head of draft PR #210), in English and with the zh catalog at its
reported coverage; the published artifact is re-proven by the
`release-smoke` workflow after the tag.

## Non-goals

- Re-proving what a PR into `dev` already runs for the paths it touches; the
  promotion PR's full run is the proof.
- Fixing cosmetic issues found while mapping evidence. File them; fix only
  blockers.
- Any row that needs Docker on the sandbox host, a display, or a physical
  disk: those are rc.2 rows or GitHub Actions jobs, never local runs.

## Fixed contracts

- Build under test: the promotion head of `dev`. Record its short SHA and
  the CI run link at the top of Results.
- Blocker definition: see
  [release-hardening.md](release-hardening.md#fixed-contracts). A cosmetic or
  copy issue is never a blocker; a data-loss, blank page, or broken core flow
  always is.
- Evidence is a spec or test name plus the CI job that ran it. "Covered by
  E2E" without a name is not evidence.
- Bilingual: the language-switch spec runs the core journey after switching
  to 中文 and asserts canonical terms (Storage Location / 存储位置,
  Repository / 资源库) from the `lumilio-frontend-i18n` registry; zh catalog
  coverage is reported from `i18next-cli status` at tag time (target 100%,
  not a gate).

## Part A — Promotion-head CI (replaces "any Docker host")

The `@smoke` slice and the Server gate on #210 are the tag-time smoke. S11
adds the two missing specs below and names P2's removable-Location evidence.
The Compose stack used by CI is the E2E image with its baked configuration,
not the release bundle; the release bundle is proven in Part C.

## Part B — Desktop (rc.2)

Installing a Desktop build, onboarding, importing a folder, tray menu,
"open in browser" at `localhost:6680`, and restart persistence need a Mac
or Windows machine. For rc.1 the evidence is the macOS and Windows CI jobs
(`ci:desktop:native`: Server and Desktop tests plus the CGo build) and the
embedded Desktop picker-controller and runtime-config harness from the
storage refactor. The native pass is the first rc.2 checkpoint, next to the
updater proof (#240).

## Part C — Published artifact (post-tag, `release-smoke` workflow)

Built in S11 and dispatched in S15 with the published image digest: fresh
install from the release bundle's Compose files, setup wizard through the
browser, upload, browse, open a detail view, backup → restore round trip,
and the compatibility-fixture artifact. Its run link is the Docker verdict
in [rc-release.md](rc-release.md) Phase 4.

## Checklist

| Flow | Core? | Evidence on the promotion head | Status |
| --- | --- | --- | --- |
| First run / setup | core | `@auth-totp` first-admin bootstrap (`auth-totp-bootstrap.spec.ts`); the fresh-bootstrap setup journey from storage P5 (R3: Adopt Primary / fresh elsewhere); the setup wizard itself in Part C | P5 + S11 |
| Upload / Photos / browse | core | `@smoke` upload, album, viewer, Trash, restore journey (`upload.spec.ts`); `@smoke` scan-and-see (`scan.spec.ts`) | exists |
| Video & music | spot | `@video-regression` (`video-semantic-regression.spec.ts`); `@smoke` music import and playback (`music.spec.ts`) | exists |
| Search | spot | `@video-regression` semantic coverage; filename and filter search have no E2E — Server handler tests only | gap, non-blocking |
| People | spot | `@people` merge with recorded faces (`people-merge.spec.ts`) | exists |
| Albums / Collections | spot | album create and membership inside `upload.spec.ts` and `trash.spec.ts` | exists |
| Events | spot | `@smoke` Events rebuild, correct, redirect, freeze (`events.spec.ts`) | exists |
| Share links | spot | `@smoke` create, open logged out, revoke (`share-links.spec.ts`) | exists |
| Studio | spot | Vitest browser project (`web/src/features/studio/**` worker and export tests); no E2E | gap, non-blocking |
| Map | spot | none | gap, non-blocking |
| Language switch | core | `@smoke` `language-switch.spec.ts` (S11): switches to 中文 from Settings, keeps it across a reload, opens Storage and Photos, asserts the registry terms 存储位置 and 资源库 from the `zh` bundle and no raw translation key on either page, switches back to English | exists (S11) |
| Settings / Users | spot | `@auth-hardening` logout, lockout, session boundaries (`auth-session.spec.ts`) | exists |
| Storage admin | core | `@smoke` add Repository, scan, Storage view counts (`storage-admin.spec.ts`); removable non-default Storage Location create (P2, #250) = the Web flow spec `StoragePanel.spec.tsx` "Desktop create on a removable external Storage Location reviews, acknowledges, and creates" (Vitest browser project, `task web:test`) and the Server tests `TestAssessCreateTargetReviewsTheActualTargetWithInjectedFacts`, `TestCreateRepositoryBindsAcknowledgementToTheAssessedTarget`, and `TestCreateRepositoryRefusesUnsupportedStorageBeforeAnyConsent` in `server/internal/storage/storage_consent_test.go` plus the handler test `TestCreateRepositoryReturnsTheConsentProblemWithRisksAndToken` (`task server:test`) | exists (P2 + S11) |
| Scanning (#222) | spot | `scan.spec.ts`; watcher and settle follow-up Server tests in `storage/scan` | exists |
| Trash / restore (#223) | core | `@smoke` Trash restore with album intact and permanent delete gone from disk (`trash.spec.ts`); offline-refusal and duplicate-resolution Server tests (S10) | exists + S10 |
| Missing and edits (#223) | spot | `server/app` lifecycle regressions: missing file leaves browse, in-place overwrite keeps album; the S10 XMP/EXIF write test | exists + S10 |
| Processing / Monitor | spot | Server stage catalog tests; no E2E | gap, non-blocking |
| Agent (Lumilio) | spot | `@agent-runtime` keyless runtime (`agent-runtime.spec.ts`), `@agent-trust` | exists |
| Backup / restore | core | `@backup-recovery` admin UI backup, download, restore, rollback (`backup-recovery.spec.ts`); Part C round trip on the published image | exists + S15 |
| Desktop onboarding, import, tray, restart | rc.2 | macOS/Windows CI jobs and the embedded picker harness only | rc.2 |

## Execution phases

- [x] S11 (2026-10-18): `language-switch.spec.ts` landed in the `@smoke`
  slice (and in the `browser_smoke` path filter); P2's removable-Location
  evidence is named in the table; `release-smoke.yml` exists, takes an
  image reference or digest on dispatch, and also runs on pull requests
  that change it (default image `edge`), which is how it was first proven:
  run link in Results.
- [ ] Tag time (S13): the promotion head's CI run link and every core row's
  verdict are recorded below; zh coverage reported.
- [ ] Post-tag (S15): Part C run link and digest recorded; Part B written up
  as rc.2 checkpoints; tick release-hardening Phase 3; delete this plan.

## Results

(Fill in at tag time: promotion-head SHA, CI run link, date; one line per
core row: verdict, spec or test, notes, PR or tracker link. Part C: digest,
run link.)

- 2026-10-18, S11: `release-smoke.yml` proof run against
  `ghcr.io/edwinzhancn/lumilio-server:edge` on the S11 pull request — run
  link: RELEASE_SMOKE_RUN_LINK.

## Validation boundaries

- Every core row names a spec or test that ran green in the promotion
  head's CI, including language switch.
- Rows that need a physical machine are recorded as rc.2 checkpoints, not
  passes.
- No open blocker.
