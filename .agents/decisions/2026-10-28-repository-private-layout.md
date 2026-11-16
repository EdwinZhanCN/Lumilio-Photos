# Decision: One class per directory under `.lumilio/`, owned by one package

Status: implemented — owner froze the layout 2026-10-28; landed in RL0–RL4
(#278, #279, #280, #281, and the RL4 PR) of the
[rc.1 sequence](../../docs/exec-plans/active/rc1-sequence.md).

## Problem

Measured on `dev` `ab1c3a45` (2026-10-28), a Repository's private tree had
no single owner:

- Three places declared the layout and disagreed: `repoDirs` in
  `directory_manager.go`, private recovery's own class list, and about 40
  `ParsePrivateRepositoryPath` calls with hard-coded strings in 12 files.
  Scan, the watcher, and path validation each repeated the reserved names.
- Derived data had two homes. Thumbnails went through the artifact store,
  with fences, immutable publishing, and a cleaner. Face crops were written
  directly into `.lumilio/assets/faces/`, outside that contract.
- The catalog stored private paths (`thumbnails.storage_path`,
  `face_items.face_image_path`), so any directory change rewrote rows.
- Upload session manifests leaked. The dev primary held 564 of them, and
  private recovery listed each one as an uncommitted upload.
- Users and backup tools could not tell the irreplaceable data from the
  rebuildable data.

## Decision

```text
<Repository>/.lumilio/
├── user/          studio/<assetID>.lumilio-sidecar, assets/<entry>.json
├── trash/         files/<trashID>, info/<trashID>.json
├── cache/         artifacts/<fence>/<stage>/<version>/<name>; CACHEDIR.TAG
├── work/          0700; uploads/<session>/, tmp/
├── quarantine/    0700; uploads/<session>/, recovery/<receipt>(.json)
└── logs/          operations.log, error.log
```

- **The first segment decides the class**, never a suffix or a deeper rule.
  `layout.Policies` is the only class table. Each class has one creation,
  permission, registration, cleanup, and backup policy. Anything else under
  `.lumilio/` is preserved unchanged and disclosed as unknown.
- **One package owns every private path.** `server/internal/storage/layout`
  builds, parses, classifies, initializes, and validates the tree. An Atlas
  rule fails any other production file that spells `.lumilio` or calls
  `ParsePrivateRepositoryPath` with a literal.
- **Class names describe ownership and lifetime, not mechanism:** `user`,
  `trash`, `cache`, `work`, `quarantine`, `logs`.
- **Face crops are artifacts** under the `faces` stage, so they get fences,
  immutability, and the cleaner.
- **The catalog stores identities.** `thumbnails` stores
  `(artifact_fence, artifact_version, artifact_name)` and `face_items` stores
  `(crop_fence, crop_version, crop_name)`. The table implies the stage, and
  only `layout` turns an `artifact.Identity` into a path.
- **Each upload session is one directory**, `work/uploads/<session>/`, with
  its manifest, chunks, and merged file. Ending a session removes one
  directory, and a failed session moves whole to `quarantine/uploads/`.
- **At startup**, before uploads, workers, and requests resume,
  `recovery.Service.PrepareRepositories` validates each reachable Repository
  and recreates missing init directories. Every file in `work/` that nothing
  can still finish moves under a `startup` receipt in `quarantine/recovery/`,
  and the Storage page lists it in the Recovery inventory. Two kinds of file
  stay: a session that a pending staging commit names, which commit recovery
  finishes, and a session whose manifest lets the browser resume it, which
  `SweepStaging` expires after an hour idle. The receipt is written before
  the moves, and the next start finishes any move a crash interrupted.
- **No upgrade path (owner).** There is no version file, no upgrader, and no
  reader for old paths. A tree in an older shape falls under the "anything
  else" rule. Dev and demo environments are rebuilt.

## Alternatives considered

- **Keep today's tree and only register it.** Rejected. It would leave two
  homes for derived data, keep session manifests mixed with in-flight bytes,
  and leave `sidecars/` meaning two things.
- **One top-level directory per feature** (`studio/`, `faces/`,
  `thumbnails/`). Rejected. Recovery and backup policy would be repeated per
  feature, and every new feature would add a new top-level policy.
- **A separate face directory.** Rejected. A second derived-data contract is
  how the old tree diverged.
- **Private paths in catalog rows.** Rejected. Every layout change would
  then need a catalog rewrite, which is why a moved directory broke
  thumbnails.
- **Fix only `DeleteSession`.** Rejected. Manifests and bytes would still be
  classified apart, and recovery would still split a session.
- **A `layout.json` version file with an idempotent upgrader, or reading
  both old and new paths.** Rejected. Both are compatibility code for
  installs that do not exist. A layout change after the first public release
  brings its own forward step.
- **Quarantine all of `work/` at startup.** This was the frozen plan's
  wording ("an entry left there is a dead operation"). Rejected when it was
  implemented in RL4. A pending staging commit's file is not dead: commit
  recovery finishes it, and moving the file would turn that recovery into a
  failure. A session with a manifest also resumes across restarts. Only
  files nothing can finish are dead.
