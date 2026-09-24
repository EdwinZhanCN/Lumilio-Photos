# Decision: rc.1 is the compatibility baseline for every app-owned persisted format

Status: implemented, 2026-09-24 (issue #221, plan
[rc-compat-baseline.md](../../docs/exec-plans/active/rc-compat-baseline.md)
Phases 0–2); the rc.1 fixture upgrade test lands with the tag. Supersedes
[the generation-9 baseline decision](2026-09-19-catalog-generation-9-baseline.md).

## Problem

Until now every persisted format was allowed to break: the catalog baseline is
edited in place and any other `PRAGMA user_version` is rejected with "delete
the SQLite catalog"; the server TOML, backup manifest, and Storage Location
marker are exact-match versions; a build that changed any of them simply
refused older data. The generation-9 decision rejected a migration chain
because "no instance exists to upgrade". That premise ends at
`v26.1.0-rc.1`, the first supported release: from then on a user's catalog,
config, backups, and media-folder markers must survive every later build.

Version numbers are also already crowded. Published and internal builds have
stamped values that the new sequence will reuse:

| Format | Published pre-release (`v26.1.0-beta.1/2`) | `dev` today | rc.1 |
| --- | --- | --- | --- |
| Catalog `PRAGMA user_version` | 8, plus the `lumilio_schema_migrations` ledger | 9, no ledger | 1 |
| Intermediate internal catalog stamps | 3, 5, 6, 7 | — | — |
| Server TOML `schema_version` | 6 (internal history 1–6) | 6 | 1 |
| Backup `manifestFormatVersion` | 2 | 3 | 1 |
| `.lumilioroot` marker `version` | `"1.0"`, same shape as today | `"1.0"` | `"1.0"` |
| `.lumiliorepo` marker `version` | `"1.0"`, same shape as today | `"1.0"` | `"1.0"` |
| Desktop state files | 1 | 1 | 1 |

(`v1.0.0-beta.*` was the PostgreSQL server; it has no SQLite catalog, TOML
manifest, or root marker to meet.) A bare version number therefore cannot tell
an rc.1-lineage file from a pre-release one, now or when later steps reach 2–9.

## Decision

**Two eras.** Before the rc.1 tag the project is internal: baselines are
edited in place, versions are reset, and data from any pre-release build is
disposable. From the rc.1 tag on, shipped baselines and steps are never
edited, user data is never wiped, and every later build (rc.2, 26.1.0, …)
upgrades from rc.1. rc.1 release notes state that pre-release data is not
migrated.

**Versions start at 1.** Every app-owned format's rc.1 version is 1. Zero
stays reserved for "absent/empty" (SQLite's `user_version` default and Go/JSON
zero values) and is never a real version.

**Catalog: identity, frozen baseline, forward steps.**
- A fresh rc.1 catalog is claimed with `PRAGMA application_id = 0x4c554d43`
  ("LUMC") when it is opened, and the baseline
  `server/migrations/000001_storage_baseline.up.sql` stamps
  `PRAGMA user_version = 1`. Every pre-release catalog carries
  `0x4c554d49` ("LUMI"), so the new identity alone marks the rc.1 lineage. At
  the tag the baseline is frozen.
- Later schema changes are embedded steps N→N+1 under
  `server/migrations/steps/`, SQL or Go where SQL cannot express the change,
  each in one transaction that also sets `user_version`. A fresh install runs
  the baseline and then every step, so fresh and upgraded catalogs share one
  code path. `user_version` is the only version discriminator; there is no
  checksum ledger.
- The startup gate, in order:
  1. No user tables and `application_id` 0 → apply baseline and steps.
  2. `application_id` "LUMI" → reject as pre-release; any other value but
     "LUMC" → reject as not a Lumilio catalog. This check runs before any
     version is read, so reused numbers can never be misread.
  3. `user_version` > current → reject: written by a newer Lumilio Photos.
  4. 1 ≤ `user_version` < current → automatic `pre-upgrade-` snapshot
     through the existing backup subsystem (protected from retention like a
     restore point), then apply the remaining steps. A failed step rolls back
     its own transaction, the catalog stays at the last completed version,
     and the server refuses to start naming the step and the snapshot. The
     db package receives the snapshot as a callback from `server/app`
     because the backup package depends on it.
  5. `user_version` = current → start.
- Every rejection says what the file is, that original media and
  Repositories are untouched, and what to do (start a fresh catalog, or use a
  newer build).

**Server TOML config.** The server never rewrites a user's file. rc.1's
`schema_version` is 1. An older supported version fails startup with a message
pointing at an explicit `config upgrade` server command (also runnable through
`docker compose run --rm`), which saves `<file>.bak` and writes the current
version. Config steps transform the parsed TOML tree; the result must pass
the strict load and is re-rendered with the generated per-key comments, so an
operator's own comments survive only in the `.bak`. A version newer than the build is rejected. The complete-manifest,
no-code-defaults rule in `CLAUDE.md` still applies to the upgraded file:
`config upgrade` writes every new field explicitly. Pre-release configs are
identified by the strict decoder (unknown or missing fields), and the message
names pre-release configs as a possible cause. Desktop owns its stored
runtime intents (content-addressed server TOML that carries the user's
network and storage choices), so it upgrades an older intent in memory
through the same config steps and never asks its user to run the command;
the next applied change stores a current intent.

**Backups.** The manifest format restarts at 1. Restore inspects the
snapshot catalog's identity (the gate above) before trusting the manifest's
version fields, so a pre-release backup is rejected as pre-release even where
its manifest number collides. A backup from any supported older version is
installed unchanged and upgraded by the same catalog steps on the next start;
an older config schema version in its manifest is accepted too. A backup
newer than the build is rejected.

**Storage Location and Repository markers (`.lumilioroot`,
`.lumiliorepo`).** They live inside user media folders and are the
longest-lived files Lumilio writes. Both follow the rules below; the
Repository's private `.lumilio/` workspace holds derived artifacts and
staging, which are re-derived rather than migrated. The rc.1 baseline of each stays
the string `"1.0"`, and neither is treated as pre-release: the pre-release
markers are the same shape, and rejecting it would force users to
edit hidden files in their media folders to start fresh. A later marker format
must keep reading `"1.0"`; today an unknown version is refused with a message
naming a newer build as the likely writer (`.lumiliorepo` previously only
required a non-empty version). Lumilio rewrites a marker only when an existing
Storage Location or Repository operation already writes it.

**Desktop state files** (settings, runtime pointer and apply journal,
resource and Lumen install records, update staging pointer, storage shortcut
cache; all version 1 today) keep version 1 at rc.1. Their readers check the
version through `desktop/internal/platform/stateversion`, which names a file
from a newer Desktop as such; a build that bumps a version handles each older
one before that check. The signed update manifest and build-embedded resource
manifest are contracts, not local state, and keep their own checks.

**Out of scope.** Shared contracts owned elsewhere (Lumen protocol,
`assets.lock.json`, `lumen.lock.json` schema versions). Derived artifacts
(`AssetPipelineVersion`, thumbnails, transcodes, vectors, OCR index) may be
dropped and re-derived; a step never has to transform them. QueueDB stays
disposable. No upgrade or restore ever modifies original media.

**Proof.** Right after the rc.1 tag a small catalog, config, backup, and
marker produced by the published rc.1 image are committed as test fixtures,
and CI upgrades them with the current build on every run. Each later release
adds its own fixture.

## Alternatives considered

**Keep editing the baseline in place after rc.1.** Rejected: it is the
generation-9 model, and it means every schema change after the first supported
release tells users to delete their catalog, losing albums, people, Event
names, and share links.

**Wipe per release candidate, start supporting upgrades at 26.1.0.** Rejected:
rc users are real users with real libraries; an rc that cannot be upgraded
from gets no honest testing, and the forward path would ship untested in the
final release.

**Keep the baseline current and prove it equivalent to baseline + steps.**
Rejected: fresh and upgraded catalogs would run different code, and a schema
equivalence test does not catch data transformations a step performs.

**Checksum ledger of applied steps.** Rejected: that is the machinery
generation-9 removed. Frozen embedded steps plus one `user_version` already
tell which steps ran; a ledger adds a second authority that can disagree with
it.

**Detect pre-release catalogs by version number or the legacy ledger alone.**
Rejected: the ledger catches the published betas (stamp 8), but internal
catalogs stamped 3–9 have no ledger, and step versions 2–9 will reuse those
numbers. The catalog already carried an `application_id` ("LUMI", as does
QueueDB with "LUMQ"); giving the rc.1 lineage a new value makes identity
independent of version at no extra cost.

**Start rc.1 versions above every pre-release number (e.g. catalog 10).**
Rejected: it leaves a permanent unexplained gap and still does not
distinguish a pre-release file from a real one; identity is a separate fact
from version.

**Let the server rewrite an older TOML config on startup.** Rejected: the
config is user-owned, often read-only in containers, and a silent rewrite
breaks the explicit-configuration principle; an explicit command with a
`.bak` is inspectable and reversible.

**Change the root marker version to the integer `1`.** Rejected: every
existing marker would need a write inside the user's media folder for no
behavioral gain.
