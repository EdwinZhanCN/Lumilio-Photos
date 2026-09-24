# RC manual smoke checklist

Status: active, created 2026-09-24. Not started. Child of
[release-hardening.md](release-hardening.md) (Phase 3, "Manual smoke
checklist"); must finish before the rc.1 tag (target 2026-09-29).

Goal: every flow below has a recorded verdict (pass / fail + PR / deferred +
tracker entry) on **a fresh Docker Compose install** and on **Desktop**. This
is a human-eye pass over what E2E does not cover (layout, empty states,
copy, i18n, keyboard and error paths), not a replacement for the E2E slices.

## Non-goals

- Re-testing what E2E already proves on every PR: sign-in/TOTP, upload,
  scan, Storage admin add/scan, share link create/open/revoke, People merge,
  Agent runtime, video semantic search, backup/restore (restore is covered by
  [rc-compat-baseline.md](rc-compat-baseline.md)). Spot-check them only.
- Fixing cosmetic issues during the run. File them; fix only blockers.

## Fixed contracts

- Build under test: the promotion head of `dev` (the head of draft PR #210).
  Record its short SHA at the top of the Results section.
- Blocker definition: see
  [release-hardening.md](release-hardening.md#fixed-contracts). A cosmetic or
  copy issue is never a blocker; a data-loss, blank page, or broken core flow
  always is.
- Bilingual: run each flow in English, and switch to 中文 at least once per
  page to catch untranslated or broken strings (canonical terms: Storage
  Location / 存储位置, Repository / 资源库 — see the
  `lumilio-frontend-i18n` skill's registry).

## Part A — Docker Compose (agent-runnable on the radxa)

Environment: `ssh radxa-x4` (Intel N100, Fedora 44, fish shell — wrap bash in
`bash -c`). Build the server image **on this Mac** for `linux/amd64` from the
promotion head, ship it with `docker save | gzip -1 | ssh radxa-x4 'gunzip |
docker load'`, then install with `deploy/compose/compose.yml` exactly as
`site/docs/en/user-manual/introduction/first-use.md` tells a user (set
`LUMILIO_IMAGE` to the shipped tag). Use its own compose project (`-p
rc-smoke`), its own data dir (`~/rc-smoke/`), and a port that doesn't collide
with `lumilio-photos-e2e` (16657–16659) or `rc-compat`. Reach it from the Mac
with an SSH tunnel and drive a real browser (Chrome via the claude-in-chrome
tools, or Playwright headed). For ML flows, the radxa runs a real Lumen Hub
on `:50051` (face, siglip, ocr); connect it through the documented settings
path.

Seed with a realistic library: the pinned `demo` profile from the assets cache
(`/Volumes/CodeBase/Projects/Lumilio-Photos/.cache/lumilio-assets/<rev>/demo`,
see `web/scripts/assets-sync.ts`), uploaded through the UI.

## Part B — Desktop (the user runs this; macOS or Windows)

Install the Desktop build from the promotion head (or the latest CI artifact
of #210), complete onboarding, import a folder of mixed media, and walk the
same table. Also check: tray menu open/quit, "open in browser" at
`localhost:6680`, restart the app and confirm the library persists, and an
in-place upgrade from the installed v26.1.0-beta.1 Desktop if available.

## Checklist (record one row per flow, per part)

| Flow | What to check |
|---|---|
| First run / setup | Wizard completes; no "Unable to verify system status"; primary Repository created; zh copy renders |
| Photos / browse | Grid, timeline scrubbing, detail view, EXIF panel, zoom, keyboard nav, empty state before import |
| Video & music | Video play + seek right after import (regression area of #216), music album play/next/queue, lyrics if present |
| Search | Filename, semantic ("ocean", "portrait"), filters (date, camera, location), no-results state |
| People | Clusters appear with Hub on, rename (modal), merge, hide; cover choice |
| Albums / Collections | Create, rename, cover, add/remove from photo view and bulk select, delete |
| Events | Events list, rename/cover/hide persists after new imports finish (regression area of #213) |
| Share links | Create from gallery and viewer, open logged out, revoke → "no longer available" |
| Studio | Open a photo, frame/text tools, export; RAW (NEF) open |
| Map | Photos with GPS appear; empty state without GPS |
| Settings / Users | Appearance/theme, language switch, create a second user, change password → sign out → sign in (regression area of #214), MFA page |
| Storage admin | Storage Locations and Repositories, asset counts non-zero (#209), scan now, verification badge |
| Processing / Monitor | Stage grid settles to idle; retry on a failed item |
| Agent (Lumilio) | Plain chat with a configured provider if available; otherwise note "not configured" |

## Execution phases

- [ ] Part A run on the radxa; results recorded below.
- [ ] Part B run by the user; results recorded below.
- [ ] Every failure: blocker fix PR into `dev` or a tracker entry with owner
  path and user impact; tick release-hardening Phase 3 items; tear down
  `rc-smoke` on the radxa; delete this plan.

## Results

(Fill in: build SHA, date, part, one line per flow: verdict, notes, PR or
tracker link.)

## Validation boundaries

- Every row has a verdict in both parts.
- No open blocker.
