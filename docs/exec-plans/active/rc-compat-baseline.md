# RC compatibility baseline and upgrade paths

Status: active, created 2026-09-24. Phases 0–2 done; Phase 3 (radxa proof) next. Child of
[release-hardening.md](release-hardening.md); tracked by issue #221 in the
`v26.1.0-rc.1` milestone, so it blocks the tag. Supersedes the beta.1 upgrade
test that was planned here earlier (pre-release data is not migrated).

Goal: rc.1 is the first supported release. Every persisted format the app
owns starts a fresh versioned baseline at rc.1, and every later build has a
working, tested forward path from it. Before rc.1 the project is internal;
from rc.1 on, nothing a user has on disk may be wiped or broken by an update.

## Fixed contracts (user decision, 2026-09-24)

- **Two eras.** Until the rc.1 tag: internal — baselines may be edited in
  place and versions reset; data from any pre-release build (`v1.0.0-beta.*`,
  `v26.1.0-beta.*`) is disposable. From rc.1 on: public — shipped migrations
  are never edited, data is never wiped, every later build (rc.2, 26.1.0,
  onward) upgrades from rc.1.
- **Pre-release data is rejected, clearly.** A catalog/config/backup from a
  pre-release build fails closed with a message naming it as pre-release and
  telling the user to start fresh; never a silent misread. rc.1 release notes
  say pre-release data is not migrated.
- **Versions start at 1, never 0.** 0 means "absent/empty" (SQLite
  `user_version` default, Go/JSON zero values) and stays reserved for that.
- **Shared contracts are out of scope**: the Lumen protocol version,
  `assets.lock.json` / `lumen.lock` schema versions belong to other repos.
- **Catalog: frozen baseline + forward steps.** At rc.1 the baseline
  `server/migrations/000001_storage_baseline.up.sql` is frozen as version 1.
  Later changes are numbered steps N→N+1, each in one transaction that also
  sets `PRAGMA user_version`. Fresh installs run baseline + all steps (one
  code path). No checksum ledger; `user_version` stays the only
  discriminator.
- **Automatic backup before any catalog upgrade**, through the existing backup
  subsystem; a failed step rolls back and the server refuses to start naming
  the step, leaving the catalog untouched.
- **Startup gate:** empty (0, no tables) → baseline + steps; 1..current−1 →
  backup then steps; current → start; > current → reject ("newer Lumilio");
  pre-release markers (e.g. the legacy `lumilio_schema_migrations` table, or
  a stamp not produced by any supported release) → reject as pre-release.
- **Server TOML config:** the server never rewrites a user's file. An older
  `schema_version` fails with a message pointing at an explicit
  `config upgrade` command that rewrites the file after saving a `.bak`.
  Desktop upgrades its own stored intents in memory (it does not regenerate
  them: they carry user choices). The "no code defaults / complete
  manifest" rule in `CLAUDE.md` still holds for the upgraded file.
- **Backups:** a build restores backups from any supported older version,
  then applies catalog steps; a backup newer than the build is rejected.
- **Repository root config inside user media folders** (`.lumilio`,
  `server/internal/storage/rootcfg`, today exact-match `"1.0"`) must accept
  older supported versions and upgrade them; it is the longest-lived data.
- **Derived artifacts** (`AssetPipelineVersion = "asset-v1"`, thumbnails,
  transcodes, vectors, OCR index) may be dropped and re-derived; a step never
  has to transform them. **QueueDB stays disposable.**
- **Desktop state files** (settings, pointer, install/staging/cache
  manifests, all version 1) read older versions forward.
- Original media is never modified by any upgrade or restore.

## Execution phases

### Phase 0 — Decision record
- [x] Write `.agents/decisions/2026-09-24-rc-compatibility-baseline.md`
  recording the contracts above, with alternatives and why they lost
  (edit-in-place forever; wipe per RC; baseline-kept-current + equivalence
  test; checksum ledger). Mark
  `2026-09-19-catalog-generation-9-baseline.md` as superseded (its rejection
  of a chain rested on "no instance exists to upgrade", which ends at rc.1).
  Update `docs/BACKEND.md` and the `migration.go` doc comments to match.
  Get the user's approval of the record before Phase 2.
  (Approved 2026-09-24 with three refinements: catalog identity via
  `PRAGMA application_id` "LUMC" checked before any version; `.lumilioroot`
  stays `"1.0"` and pre-release markers are accepted; pre-release server
  configs are caught by the strict decoder. `docs/BACKEND.md` and the
  `migration.go` comments change with the Phase 1 code they describe.)

### Phase 1 — Reset to version 1 (internal era, last edit-in-place)
- [x] Catalog `SchemaVersion` 9 → 1 (`server/internal/db/migration.go`) and
  the baseline's `PRAGMA user_version` stamp together; backup inspection
  (`InspectStandaloneCatalog`) follows. Catalog `application_id` "LUMI" →
  "LUMC"; "LUMI" is rejected as `ErrPreReleaseCatalog` before any version.
- [x] Server TOML `schema_version` 6 → 1 (`server/config/config.go`); regenerate
  `server/config/examples/**` with the repo's config-examples task; update any
  docs showing it (`site/docs/en` and `site/docs/zh-cn`; none do). The
  schema id is now `lumilio-server-v1.schema.json`.
- [x] Backup `manifestFormatVersion` 3 → 1 (`server/internal/db/backup`).
- [x] Repository root config: `"1.0"` stays (decision record); pre-release
  markers are the same shape and are accepted, so no code change.
- [x] Pre-release rejection with clear messages for catalog, config, backup
  (root config is accepted, above); unit tests: `TestOpenRejectsPreReleaseCatalog`
  (beta stamp 8 + ledger, stamp 9, colliding stamp 1),
  `TestValidateSnapshotAttributesManifestFormatMismatch` (v2/v3 pre-release,
  newer, missing), `TestLoadAppConfigRejectsNewerOrPreReleaseSchemaVersion`.
- [x] `task server:test`, `task verify:generated`, `task desktop:test` green
  on 2026-09-24 (commit `8b1482fb`).
- [ ] Wipe local/radxa E2E and Desktop state before any manual testing: every
  existing catalog is now rejected as pre-release.

### Phase 2 — Forward paths
- [x] Catalog step runner (`server/internal/db/migration.go`): embedded
  `server/migrations/steps/NNNN_<name>.sql`, contiguous 2..`SchemaVersion`
  (checked at load and by `TestEmbeddedCatalogStepsMatchSchemaVersion`); a
  step is a `func(ctx, tx)`, so a Go step is one more slice entry when SQL
  cannot express a change. Each step runs in one transaction with its
  `user_version` stamp. `MigrateCatalog(ctx, backup)` takes the pre-upgrade
  backup as a callback from `server/app`, which writes a protected
  `pre-upgrade-` snapshot (retention never prunes it; the backup list shows it
  as a restore point). Restore accepts older schema/config versions and
  installs them unchanged; the next start upgrades them. Tests
  (`migration_steps_test.go`, test-only registry): fresh install without
  backup, backup before the first step then in-order steps, failed step rolls
  back and names step + snapshot then resumes, no backup refuses, newer
  rejected, loader naming/gap/stamp rules; backup package: older snapshot
  staged and installed, newer schema and config snapshots rejected,
  `pre-upgrade-` protected from pruning.
- [x] `server config upgrade --config <file>` (`server/cmd/config_cli.go`,
  `server/config/upgrade.go`): config steps transform the parsed tree, the
  result must pass the strict load, the original goes to `<file>.bak`
  (refuses to overwrite an existing `.bak`), and the new file is re-rendered
  with generated comments. Startup points an older version at the command;
  pre-release/newer versions are refused. Documented in en
  `introduction/upgrade.md` and zh-cn `admin/upgrade.md`. Tests: test-only
  1→2 step, refusals, current file untouched, CLI current/pre-release paths.
- [x] Root marker: stays `"1.0"`, pre-release markers read as-is (test), an
  unknown version names a newer build. Desktop runtime intents are upgraded
  in memory through `config.UpgradeManifest` in `Store.Validate` and
  `LoadCurrentConfig`. Desktop: persisted readers go through
  `desktop/internal/platform/stateversion` (newer → `ErrNewer`); settings test.
- [x] Contributor rule in `server/migrations/steps/README.md`,
  `docs/BACKEND.md`, and the `lumilio-api-contract-change` skill.

### Phase 3 — Prove it on the RC build
- [ ] On the radxa (build on the Mac for linux/amd64, ship, run — see the
  parent plan's resume section): fresh install stamps version 1 everywhere;
  a pre-release catalog and config are rejected with the intended messages;
  backup → restore round-trip on the RC build keeps counts, user edits
  (album cover, Event rename, person rename, share link), and original-file
  checksums.

### Phase 4 — Lock the rc.1 fixture (at tag time, with rc-release)
- [ ] Right after tagging, generate a small rc.1 catalog (plus its config,
  a backup, and a repository root config) with the published rc.1 image,
  commit it under a testdata path, and add a CI test that upgrades it with
  the current build on every run (counts, user state, originals untouched).
  Each later release adds its own fixture. Coordinate the exact step with
  [rc-release.md](rc-release.md) Phase 3.

## Validation boundaries

- All app-owned persisted formats are at version 1 on a fresh rc.1 install;
  pre-release data is rejected with explicit messages.
- The step runner, config upgrade, and restore-then-upgrade paths are tested
  in CI; no step exists in production yet.
- The decision record is approved and supersedes generation-9.
- After the tag: the rc.1 fixture upgrade test runs in CI.
