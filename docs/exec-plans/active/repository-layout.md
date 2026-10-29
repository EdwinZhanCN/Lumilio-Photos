# Repository layout: one class per directory, owned by one package

Status: active. Created 2026-10-28; nothing has landed yet. The layout below
was frozen by the owner on 2026-10-28. It lands before the rc.1 tag, per the
"fix what is found" contract in [rc-release.md](rc-release.md).

This is the first layout, not a second version. There is no production
install, so no code upgrades, migrates, or reads the current tree. Dev and
demo environments are rebuilt (owner, 2026-10-28).

The order across plans is recorded in [rc1-sequence.md](rc1-sequence.md)
(steps RL0–RL4, after server-structure B2). The
[repository access](repository-access-simplification.md) plan's Phase 3 and
the [portable asset sidecars](portable-asset-sidecars.md) plan both build on
this plan's `cache/artifacts/` and `user/`.

Goal: every path under a Repository's `.lumilio/` belongs to exactly one
class. The first path segment decides the class. Each class has one policy
for creation, permission, recovery, cleanup, and backup. One package
(`server/internal/storage/layout`) builds every private path, classifies
every private path, and initializes and validates the tree. No other code
spells a `.lumilio` path. The catalog stores identities, not private paths,
so the private tree can be rearranged without touching catalog rows.

## Why (measured 2026-10-28 on `dev` `ab1c3a45`)

- **The layout is declared in three places that disagree.**
  - `repoDirs` in `directory_manager.go` calls itself the "single source of
    truth" but omits `artifacts/`, `trash/`, and `recovery/`.
  - `private_recovery.go` classifies with its own list.
  - The artifact store, trash, recovery, staging, the face service, and the
    upload session manager each hard-code their own strings. There are about
    40 `ParsePrivateRepositoryPath` call sites in 12 files.
  - Scan (`repository_scan_fs.go`), the watcher, and path validation
    (`repository_path.go`) each repeat the reserved names.
- **Derived data has two homes.** Thumbnails and transcodes go through the
  artifact store, under `.lumilio/artifacts/`, which publishes immutably,
  checks fences, and has a cleaner. Face crops are written directly by
  `face_service` into `.lumilio/assets/faces/`, outside that contract.
  `assets/` is a legacy name.
- **The catalog stores private paths.**
  `thumbnails.storage_path` holds values like
  `.lumilio/artifacts/<fence>/derivatives/asset-v1/large.webp`, and
  `face_items.face_image_path` holds face crop paths. Any directory change
  therefore rewrites rows.
- **Upload session manifests leak.**
  - `CleanupExpiredSessions` skips `completed` sessions, and `DeleteSession`
    has no production caller. Every chunked upload therefore leaves
    `staging/incoming/upload_sessions/<id>.json` behind for good, and its
    in-memory session until restart.
  - The dev primary held 564 of them: 561 completed, 2 stuck in `merging`
    from the 2026-10-28 wedge, and 1 pending.
  - Because they sit under `staging/`, private recovery lists each one as an
    uncommitted upload and moves it under a receipt on open or adopt.
- **Temporary names differ.** The root holds `.lumilio_permission_test-*`,
  `.lumilio_case_probe-a*`, `.lumilio-volume-probe-*`,
  `.lumilio-write-test-*`, and `.lumilio-marker-*`. Inside the tree there is
  also `<path>.tmp-<uuid>`.
- **`sidecars/` names a mechanism, not a class.** User-authored Studio
  records would share it with the planned asset sidecars, and private
  recovery marks any nested file there as unknown.
- **Nothing marks what is safe to drop.** Users and backup tools cannot tell
  the irreplaceable data in `.lumilio/` from the rebuildable data. Repository
  audit logs are never rotated.

## Layout (frozen 2026-10-28)

```text
<Repository>/
├── .lumiliorepo                 identity and config (unchanged contract)
├── .lumiliorepo.lock            runtime ownership lock (unchanged)
├── inbox/                       managed uploads land here (unchanged)
├── <user folders>/              originals
└── .lumilio/
    ├── user/                    authored by people; irreplaceable; never rebuilt
    │   ├── studio/<assetID>.lumilio-sidecar
    │   └── assets/<entry path>.json            (portable asset sidecars)
    ├── trash/                   deleted originals under retention
    │   ├── files/<trashID>
    │   └── info/<trashID>.json
    ├── cache/                   rebuildable; safe to delete; CACHEDIR.TAG
    │   └── artifacts/<source fence>/<stage>/<pipeline version>/<name>
    │                            thumbnails, transcodes, waveforms, face crops
    ├── work/                    0700; owned by one running operation
    │   ├── uploads/<session>/   session.json + chunks + merged file
    │   └── tmp/                 atomic-write temporaries
    ├── quarantine/              0700; bytes kept for a human decision
    │   ├── uploads/<id>/        failed uploads (was staging/failed)
    │   └── recovery/<receipt>/  registration receipts and the bytes they moved
    └── logs/                    operations.log, error.log; size-rotated
```

Every row in the policy table below is a fixed contract. The paths are
relative to `.lumilio/`.

| Class | Created | Perm | Open, adopt, register-copy | Cleanup | Backup |
| --- | --- | --- | --- | --- | --- |
| `user` | init | 0755 | Kept in place; reassociated only on proven content | Never automatic | Include |
| `trash` | init | 0755 | Adopted into the catalog under a hold | Retention policy | Include |
| `cache` | init | 0755 | Kept and reused when fences match, else rederived | Artifact cleaner; whole class deletable | Exclude (`CACHEDIR.TAG`) |
| `work` | init | 0700 | Moved to `quarantine/recovery/<receipt>/`; never resumed | Removed when its operation ends | Exclude |
| `quarantine` | lazily | 0700 | Kept | Only by an explicit user decision | Include |
| `logs` | init | 0755 | Kept | Rotation | Optional |
| anything else | — | — | Preserved unchanged and disclosed as unknown | Never | Include |

Root files outside `.lumilio/` remain the public contract described in the
user manual's repository layout page: the marker, its lock, and `inbox/`.
The only other root names Lumilio may create are short-lived probes and
marker temporaries. They use one prefix, `.lumilio-tmp-`, which scan,
watcher, and recovery ignore with one rule.

## Non-goals

- The Storage Location root (`.lumilioroot` and its lock) and Repository
  storage strategies (`date`, `flat`, `cas`) for originals.
- What sidecars contain. That belongs to the portable asset sidecars plan;
  this plan only gives them `user/assets/`.
- Moving the marker or the lock into `.lumilio/`. Both must be claimable and
  readable before the private tree exists, and the marker is a public,
  documented contract.

## Decisions (frozen 2026-10-28; extract at completion)

- **The first segment decides the class.** A path's class comes from its
  first segment under `.lumilio/`, never from a suffix or a deeper rule. That
  gives one classifier for recovery, scan, watcher, and path validation.
- **Class names describe ownership and lifetime, not mechanism.** The names
  are `user`, `trash`, `cache`, `work`, `quarantine`, and `logs`. Rejected
  alternatives:
  - *Keep today's tree and only register it.* That would leave two homes for derived
    data, session manifests mixed with in-flight bytes, and `sidecars/`
    meaning two things.
  - *One directory per feature* (`studio/`, `faces/`, `thumbnails/` at top
    level). Recovery and backup policy would then have to be repeated per
    feature, and each new feature would add a new top-level policy.
- **Face crops become artifacts.** They are published through
  `artifact.Store` under the `faces` stage, so they get fences, immutability,
  and the cleaner. A separate face directory was rejected, because a second
  derived-data contract is how the current tree diverged.
- **The catalog stores identities.**
  - `thumbnails` and `face_items` store the artifact identity: source fence,
    stage, pipeline version, and name. They do not store a private path; the
    layout package resolves the path.
  - Rejected: keeping private paths in rows. That couples every layout
    change to a catalog rewrite, and it is the reason
    a moved directory breaks thumbnails.
- **Each upload session is one directory.**
  `work/uploads/<session>/` holds `session.json`, the chunks, and the merged
  file. Ending a session, whether completed, failed, or expired, removes one
  directory. A failed session moves whole to `quarantine/uploads/`. Rejected:
  fixing only `DeleteSession`. Manifests and bytes would still be classified
  apart, and recovery would still split a session.
- **No upgrade path (owner).** The tree has no version file, no upgrader,
  and no reader for old paths. A tree in the current shape gets no code of
  its own. Its old directories fall under the ordinary "anything else" rule:
  they are preserved unchanged and disclosed as unknown. Dev and demo
  environments are rebuilt and re-seeded. Rejected alternatives:
  - *A `layout.json` version file with an idempotent upgrader.*
  - *Reading both the old and the new paths.*

  Both are compatibility code for installs that do not exist. When the first
  public release ships and the layout later has to change, that change brings
  its own forward step.

## Execution phases

### Phase 0 — Lock the failures

Each test must fail on today's code.

- **Session leak.** A completed chunked upload leaves no manifest or
  in-memory session behind. Open or adopt does not list a completed session
  as an uncommitted upload.
- **Single owner.** A registered architecture behavior test fails when a
  non-test file outside `storage/layout` contains the `.lumilio` string
  literal or calls `ParsePrivateRepositoryPath` with a string literal. Start
  it in report mode with today's list of violations, and make it fail once
  Phase 2 empties that list.
- **Path inventory.** Inventory every catalog column and JSON field that
  stores a private path. Known so far: `thumbnails.storage_path` and
  `face_items.face_image_path`. Also search the specific metadata, the
  receipts, and the trash info JSON.

### Phase 1 — The layout package

`server/internal/storage/layout` holds the following, with `doc.go` in the
storage Atlas group:

- the class table above as data;
- typed constructors for every path kind, such as `Studio(assetID)`,
  `AssetSidecar(entryPath)`, `Artifact(identity)`, `TrashFile(id)`,
  `UploadSession(id)`, `QuarantineUpload(id)`, `RecoveryReceipt(id)`,
  `Log(name)`, and `Temp()`;
- `Classify(privatePath)` and `IsReserved(rootRelative)`;
- `Init` and `Validate`;
- `CACHEDIR.TAG` handling;
- the `.lumilio-tmp-` prefix.

`RepositoryPath` for private paths becomes constructible only through this
package.

### Phase 2 — Move every caller

Move these callers onto the constructors:

- the directory manager;
- private recovery and `storage/recovery`;
- the artifact store;
- trash;
- staging and upload sessions;
- the face service;
- `sourcing/materializer.go`;
- the media path handler;
- the queue thumbnail loaders;
- scan;
- the watcher;
- `repository_path.go`;
- `storage_observation.go`;
- the storage probes;
- marker temporaries;
- `testfixture/layout.go`.

Delete `DefaultStructure`, `repoDirs`, and the per-package root constants.
Then turn the Phase 0 architecture test to failing mode.

### Phase 3 — Catalog identities

- Edit the catalog baseline in place before rc.1, following the
  [rc compatibility baseline](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md):
  `thumbnails` and `face_items` store the artifact identity.
- Face crops are published as `faces` artifacts at the point where they are
  cut today.
- Rebuild `.local/dev`, then re-seed the demo with `task web:demo:seed`.
  Nothing converts an existing tree.

### Phase 4 — Policies

- Size-based rotation for `logs/`.
- The cleaner covers `cache/artifacts/`, and the `faces` stage replaces the
  face directory.
- Clear `work/` at startup: an entry left there is a dead operation, and it
  moves to quarantine under a receipt.
- Validate the class table at startup, and repair missing init-created
  directories (the existing `layout_repaired` setup field).

### Phase 5 — Docs

- The user manual's repository layout page, in `en/` and `zh-cn/`: the
  classes, what is safe to delete, and what to exclude from backups.
- Update the Atlas views that anchor the touched symbols, then run
  `task atlas:lock` after re-reading them.
- Point `portable-asset-sidecars.md` at `user/assets/`, which is already
  done in this PR.

## Validation boundaries

- No production code outside `storage/layout` spells a `.lumilio` path. The
  architecture test enforces this.
- A fresh Repository has exactly the tree above, and `Validate` accepts it.
- No upgrade, migration, version file, or old-path reader exists for the
  private tree.
- A chunked upload leaves nothing in `work/` after it completes, fails, or
  expires.
- Deleting `cache/` loses nothing that cannot be rederived. Deleting
  `work/` while stopped loses nothing that recovery would have resumed,
  because work is never resumed.
- These are green: `task server:test`, `task atlas:check`, the `trash` and
  `storage-recovery` E2E slices, and `task desktop:test` (the Desktop storage
  migration tests reference Repository trees).
