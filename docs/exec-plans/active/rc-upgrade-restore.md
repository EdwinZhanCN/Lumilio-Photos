# RC upgrade and restore qualification

Status: active, created 2026-09-24. Not started. Child of
[release-hardening.md](release-hardening.md) (Phase 6, "Upgrade test"); must
finish before the rc.1 tag (target 2026-09-29).

Goal: prove, on the Intel N100 qualification host, that a catalog created by
the last published build (**v26.1.0-beta.1**) starts on the release candidate
build, migrates, and still browses correctly; and that a backup taken from it
restores. Record every result in this file. This plan is a qualification run,
not a feature: any failure becomes a blocker fix PR into `dev`.

## Non-goals

- Upgrades from `v1.0.0-beta.*` (July pre-releases on a different catalog
  generation). Note the result if cheap, but do not block on it.
- Desktop in-place upgrade (covered by the Desktop row of
  [rc-smoke-checklist.md](rc-smoke-checklist.md)).
- Performance or ingest throughput.

## Fixed contracts

- Already-released migrations are forward-only: never edit a migration that
  shipped in v26.1.0-beta.1. A migration bug is fixed by a new migration.
- Original media is never modified by an upgrade or restore (local-first
  principle). Verify by checksum, not by eye.
- Blocker definition and "no check skipped" rule: see
  [release-hardening.md](release-hardening.md#fixed-contracts).

## Environment (read before starting)

- Remote Docker host: `ssh radxa-x4` (Intel N100, 7.5 GiB, Fedora 44, **fish**
  login shell — wrap bash syntax as `ssh radxa-x4 'bash -c "…"'`). Docker
  29, Compose v5. User `edwin`.
- The user's workflow: **build images on this Mac** (arm64 → always
  `--platform linux/amd64`, OrbStack must be running: `orb start`), ship with
  `docker save <img> | gzip -1 | ssh radxa-x4 'gunzip | docker load'`
  (~3.5 min for the server image), and run on radxa-x4. Do not build on the
  radxa.
- Published images: `ghcr.io/edwinzhancn/lumilio-server:<tag>`. Pull the
  beta.1 image **on the radxa** (`docker pull
  ghcr.io/edwinzhancn/lumilio-server:26.1.0-beta.1` — confirm the exact tag
  from `.github/workflows/release.yml` `tags:` and the GitHub release page; if
  the registry needs auth, ask the user to run `! docker login ghcr.io` on the
  radxa).
- Deployment files: `deploy/compose/compose.yml` (server; `LUMILIO_IMAGE`,
  `LUMILIO_STORAGE`, `LUMILIO_STATE` select image and bind-mounted data
  dirs). The server requires a complete schema-versioned TOML manifest — read
  `site/docs/en/user-manual/introduction/upgrade.md` and
  `site/docs/en/user-manual/introduction/first-use.md` first and follow the
  documented user path exactly; the point is to test what users do.
- Use a dedicated directory on the radxa, e.g. `~/rc-upgrade/`, and a
  dedicated compose project name (`-p rc-upgrade`) and host port that do not
  collide with the E2E project `lumilio-photos-e2e` (ports 16657–16659).
- Backup API (admin): `POST /api/v1/settings/backups`, `GET
  /api/v1/settings/backups`, `GET …/backups/:name/download`, `POST
  …/backups/:name/restore`, `GET /api/v1/settings/backup-restores/latest`.
  Prefer the UI for the restore step (it is what users do), and use the API
  to assert facts.
- Test media: the pinned assets cache
  (`/Volumes/CodeBase/Projects/Lumilio-Photos/.cache/lumilio-assets/<rev>/`,
  profiles `smoke`/`e2e`/`demo`; `web/scripts/assets-sync.ts`). Use a mix:
  JPEG with EXIF/GPS, HEIC, a RAW (NEF), a video, audio (m4a + flac), plus a
  duplicate and a burst if available.

## Execution phases

### Phase 0 — Seed a beta.1 catalog
- [ ] Fresh beta.1 install on the radxa via the documented Compose path;
  complete the setup wizard (admin + TOTP if offered + primary Repository).
- [ ] Import the media mix (upload via UI or API, and at least one file placed
  in the Repository and picked up by a scan). Wait until processing is idle
  (Monitor/Processing page or the stats API).
- [ ] Create user state that an upgrade must preserve: an album with a cover,
  a rename of an Event, a person rename (only if face recognition is
  available — the radxa runs a Lumen Hub on :50051; enable it only if beta.1
  supports it), likes/ratings, a tag, a share link (note its URL), a second
  non-admin user.
- [ ] Record a **baseline manifest**: counts from the API (assets by type,
  albums, events, people, share links, users), 10 sampled asset ids with
  their `original_filename`, `taken_time`, dimensions; and `sha256sum` of every
  original file under `LUMILIO_STORAGE`.
- [ ] Take a backup through the UI; download it; note its name and size.
  Stop the stack. Copy the whole data dir (`cp -a`) to `~/rc-upgrade/pristine`
  so every later phase can restart from the same beta.1 state.

### Phase 1 — Upgrade in place to the RC build
- [ ] Build the RC server image on the Mac from `origin/dev` (the promotion
  head) for linux/amd64; ship it; tag it clearly (e.g.
  `lumilio-server:rc-candidate-<shortsha>`).
- [ ] Point `LUMILIO_IMAGE` at it and `up` against the beta.1 data dir,
  following upgrade.md. Capture startup logs: every migration applied, no
  errors/panics, health `/api/v1/health/ready` OK.
- [ ] Re-run the baseline manifest and diff it: counts equal (or explained),
  sampled metadata equal, **original-file checksums identical**, user state
  from Phase 0 present (album cover, Event rename, person rename, share link
  still opens logged out, second user can log in).
- [ ] Browse in a real browser (through an SSH tunnel to the chosen port):
  Photos grid, a photo detail, video playback with a seek, music playback,
  search (filename + semantic if ML on), Storage page shows non-zero Assets
  (regression check for the fix in #209).
- [ ] Let processing settle; confirm no stuck jobs on the Processing page.

### Phase 2 — Restore the beta.1 backup on the RC build
- [ ] From the pristine copy, start the **RC** build fresh (new data dir) and
  restore the beta.1 backup through the UI (or the documented restore path);
  poll `backup-restores/latest` until terminal.
- [ ] Diff against the baseline manifest as in Phase 1.
- [ ] Take a new backup on the RC build and restore it onto another fresh RC
  instance (round trip on the new version).

### Phase 3 — Downgrade expectation (documented behaviour only)
- [ ] Check what upgrade.md promises about going back to beta.1 after an
  upgrade. Do not invent a guarantee; if it's undocumented, add one sentence
  to the docs stating the actual behaviour you observe (starting beta.1 on an
  upgraded catalog), in both `en` and `zh-cn`.

### Phase 4 — Close out
- [ ] Record results per phase in a "Results" section below (commands,
  counts, diffs, verdicts). Every failure: blocker fix PR into `dev` (with a
  failing-first test) or an explicit deferred verdict agreed with the user.
- [ ] Tick "Upgrade test" in release-hardening.md Phase 6; tear down
  `rc-upgrade` on the radxa (`down -v`, remove `~/rc-upgrade`) unless the
  user wants it kept; delete this plan per the exec-plan skill (extract any
  durable decision first).

## Validation boundaries

- beta.1 catalog → RC build: migrations apply cleanly, server healthy, all
  baseline counts and sampled metadata match, original checksums identical,
  Phase 0 user state intact.
- beta.1 backup restores on the RC build with the same result; an RC backup
  round-trips on the RC build.
- No new blocker left open.
