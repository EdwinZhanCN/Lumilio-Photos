# RC manual smoke checklist

Status: active, created 2026-09-24. Not started. Child of
[release-hardening.md](release-hardening.md) (Phase 3, "Manual smoke
checklist"); must finish before the rc.1 tag (date TBD, owner decision
2026-09-24). Part A starts only after the last PR of
[repository-index-and-asset-lifecycle.md](repository-index-and-asset-lifecycle.md)
(#222, #223) merges; those PRs replace scanning, Storage counts, and delete.

Goal: a tag-time checkpoint of core flows on a fresh Docker Compose install
on any Docker host (including this cloud box or Mac Docker) and Desktop on
Edwin's Mac. Record first-run setup, upload/browse, Storage create including a
removable Storage Location, Trash/restore, and language switch. Other rows
are spot checks; Windows is checked if available. This complements E2E.

## Non-goals

- Re-testing what E2E already proves on every PR: sign-in/TOTP, upload,
  Storage admin add, share link create/open/revoke, People merge,
  Agent runtime, video semantic search, backup/restore (restore is covered by
  the compatibility baseline's RC-build proof, PR #225). Spot-check them only.
- Fixing cosmetic issues during the run. File them; fix only blockers.

## Fixed contracts

- Build under test: the promotion head of `dev` (the head of draft PR #210).
  Record its short SHA at the top of the Results section.
- Blocker definition: see
  [release-hardening.md](release-hardening.md#fixed-contracts). A cosmetic or
  copy issue is never a blocker; a data-loss, blank page, or broken core flow
  always is.
- Bilingual checkpoint: run core flows in English and switch to 中文 to catch untranslated
  or broken strings (canonical terms: Storage Location / 存储位置, Repository / 资源库 — see the
  `lumilio-frontend-i18n` skill's registry).

## Part A — Docker Compose (any Docker host)

Install the promotion-head image with `deploy/compose/compose.yml` as the
first-use guide describes. Use an isolated `rc-smoke` Compose project, data
directory, and port, then drive a real browser. ML flows are optional spot
checks with a configured Hub. Intel N100 / radxa-x4 is optional reference
hardware, never a release prerequisite.

Seed with a realistic library: the pinned `demo` profile from the assets cache
(`/Volumes/CodeBase/Projects/Lumilio-Photos/.cache/lumilio-assets/<rev>/demo`,
see `web/scripts/assets-sync.ts`), uploaded through the UI.

## Part B — Desktop (Edwin's Mac; Windows if available)

Install the Desktop build from the promotion head (or the latest CI artifact
of #210), complete onboarding, import a folder of mixed media, and walk the
same table. Also check: tray menu open/quit, "open in browser" at
`localhost:6680`, restart the app and confirm the library persists.

## Checklist (core checkpoint rows; other rows are spot checks)

| Flow | What to check |
|---|---|
| First run / setup (core) | Wizard completes; no "Unable to verify system status"; primary Repository created; zh copy renders |
| Upload / Photos / browse (core) | Upload mixed media, browse the grid, and open a detail view. Spot-check timeline scrubbing, EXIF, zoom, keyboard navigation, and empty states |
| Video & music | Video play + seek right after import (regression area of #216), music album play/next/queue, lyrics if present |
| Search | Filename, semantic ("ocean", "portrait"), filters (date, camera, location), no-results state |
| People | Clusters appear with Hub on, rename (modal), merge, hide; cover choice |
| Albums / Collections | Create, rename, cover, add/remove from photo view and bulk select, delete |
| Events | Events list, rename/cover/hide persists after new imports finish (regression area of #213) |
| Share links | Create from gallery and viewer, open logged out, revoke → "no longer available" |
| Studio | Open a photo, frame/text tools, export; RAW (NEF) open |
| Map | Photos with GPS appear; empty state without GPS |
| Language switch (core) | Switch English / 中文 and confirm the page remains usable with translated copy |
| Settings / Users | Appearance/theme, create a second user, change password → sign out → sign in (regression area of #214), MFA page |
| Storage admin (core) | Create a Repository and a Storage Location, including a removable location; asset counts non-zero (#209), scan now, verification badge |
| Scanning (#222) | Spot-check: copy a folder in and see it appear without a manual scan; move and rename a folder and check that albums survive; unplug an external disk (or stop the share) and check that nothing goes Missing and the Repository shows offline |
| Trash / restore (#223, core) | Delete a photo and see the file under `<repo>/.lumilio/trash`; restore it with album, rating, and people intact. Spot-check: delete permanently and see it gone from disk; delete on an offline Repository is refused with a reason; resolve a duplicate group and see the duplicates in Trash |
| Missing and edits (#223) | Delete a file outside Lumilio: it leaves the library and appears in the Missing view on the Storage page, and Remove missing items clears it; edit a photo in another app (save over the file): album, rating, and people are kept |
| Processing / Monitor | Stage grid settles to idle; retry on a failed item |
| Agent (Lumilio) | Plain chat with a configured provider if available; otherwise note "not configured" |

## Execution phases

- [ ] Part A core rows run on any Docker host; results recorded below.
- [ ] Part B core rows run on Edwin's Mac; Windows if available; results recorded below.
- [ ] Every failure: fix blockers in `dev`; recording deferred findings in the tracker
  is encouraged; tick release-hardening Phase 3 items; tear down
  `rc-smoke` on the chosen host; delete this plan.

## Results

(Fill in: build SHA, date, part, one line per flow: verdict, notes, PR or
tracker link.)

## Validation boundaries

- Core rows have tag-time verdicts for Docker and Edwin's Mac, including language switch.
- Other rows are spot checks; missing spot checks do not block the tag.
- No open blocker.
