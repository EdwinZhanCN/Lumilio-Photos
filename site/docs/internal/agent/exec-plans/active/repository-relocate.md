# Repository Relocate & Path Policy

Status: active. Steps 1, 3, 4 and 5 are implemented; step 2 (macOS signing/TCC
spike) is unstarted and needs a human on real hardware, and step 6 (desktop UI)
is deliberately deferred behind it. Implements ADR-004 (repository identity
separated from location; deployment-differentiated path policy). This plan
records what the code already provides, the corrections the investigation forced
on the ADR, and the ordered remaining work.

## Current contract

Identity and location are already separate in the data model, which makes
relocate far cheaper than the ADR assumed:

- `.lumiliorepo` carries the repository UUID and is written at init
  (`server/internal/storage/repocfg`); the DB `repositories` row stores that
  same UUID as `repo_id` plus the last-known `path`.
- `assets.storage_path` is **repository-relative** — see
  `UNIQUE (repository_id, storage_path)` in
  `server/migrations/000003_assets_repositories.up.sql:80` and every consumer
  joining it onto the repository path
  (`server/internal/service/indexing_service.go:515`,
  `server/internal/queue/ml_image_loader.go:76`). Relocate is a single `UPDATE`
  on `repositories.path`; no asset rows move.
- The scanner already reads `ListActiveRepositories`
  (`server/internal/storage/scanner/scanner.go:122`), so an `offline` repository
  is excluded from scanning for free. `RepoStatusOffline` exists as a value
  (`server/internal/db/dbtypes/repo_types.go:9`) but nothing writes it today.
- Path resolution is currently rooted-only and derived from the repository
  *name*: `resolveRepositoryCreatePath(root, name, role)` slugifies the name
  under the storage root and enforces containment with `filepath.Rel`
  (`server/internal/storage/repo_provisioning.go:98-128`). This function *is*
  the unwritten RootedPolicy.
- `CreateRepositoryRequestDTO` has no `path` field
  (`server/internal/api/dto/repository_scan_dto.go:5-11`); the caller cannot
  express a location at all.
- The desktop already owns a native directory picker returning a plain path
  (`desktop/onboarding.go:103-120`, `desktop/onboarding.go:298`).

Primary owners: `server/internal/storage` (`repo_manager.go`,
`repo_provisioning.go`), `server/internal/db/repo/queries/repositories.sql`,
`server/internal/api/handler/repository_scan_handler.go`,
`web/src/features/repositories`, `desktop/onboarding.go`.

## Investigation findings that amend ADR-004

### The macOS blocker is code signing, not security-scoped bookmarks

ADR-004 assumes desktop needs security-scoped bookmarks and that Wails may not
expose them. Verified against the pinned toolchain, that framing is wrong:

- Wails v3 `v3.0.0-alpha.96` has **no bookmark support of any kind** —
  `grep -ril bookmark` over `pkg/` returns nothing. Its darwin open dialog is a
  plain `NSOpenPanel` returning `[url path]` strings, with no
  `startAccessingSecurityScopedResource` call
  (`pkg/application/dialogs_darwin.go:122-211`).
- That absence does not matter here: security-scoped bookmarks are an App
  Sandbox mechanism. **Lumilio Desktop is not sandboxed.** There is no
  `.entitlements` file anywhere in the repo, and `scripts/build-macos.sh:392-401`
  ad-hoc signs the bundle (`codesign --force --deep -s -`) with no entitlements
  argument.
- The real durability risk is the ad-hoc signature. TCC grants are keyed on the
  application's code-signing identity; an ad-hoc signature has no stable
  designated requirement, so its identity is effectively the cdhash. Every
  rebuild or in-place update produces a new cdhash, which is expected to
  invalidate previously granted access to protected locations.
- `desktop/build/Lumilio Photos.app/Contents/Info.plist` declares no
  `NSDesktopFolderUsageDescription`, `NSDocumentsFolderUsageDescription`,
  `NSRemovableVolumesUsageDescription`, or `NSPhotoLibraryUsageDescription`.

Consequence: the FreePolicy spike is no longer about Wails APIs. It is a
signing-and-TCC question that **already applies to the shipped onboarding
storage picker and to `desktop/update.go`**, independent of this plan. Treat the
last two bullets as expected macOS behaviour that the spike must confirm
empirically before FreePolicy is promised to users.

### Storage root is not only a repository container

ADR-004's comparison table says desktop "cancels the root concept". It cannot:
`config.StorageConfig` also hosts `SecretsDir()`, `CloudDir()`, `PrimaryDir()`
and `BackupsDir()` (`server/config/config.go:80-83`), the backup scheduler stats
it (`server/internal/db/backup/scheduler.go:66`), and `storage.path` is a
required TOML field (`server/config/config.go:303`). Desktop drops only the
constraint that *repositories* live under root. Root stays as the application
data directory.

### PathPolicy is an API contract change, not a pure refactor

Because the create DTO has no `path`, FreePolicy requires a new request field,
`make dto`, and a frontend change alongside the policy abstraction. These ship
as one unit.

### Move vs. copy cannot be inferred reliably

ADR-004's add/relocate flow distinguishes relocate from a duplicate by checking
whether the old path still holds a valid same-ID repository. That check fails in
the ordinary external-drive sequence: drive unplugged → `offline` → user
registers a copy → relocate succeeds → drive returns, and reconcile only
consults the current DB path. Replace the automatic verdict with an explicit
user choice: "this appears to be repository X, last seen at `<old path>` —
**relocate**, or **register as a new copy**". Registering as a copy mints a new
UUID into the copy's `.lumiliorepo`, which is the `git clone` answer and turns a
dead-end error into an actionable path.

### `.lumiliorepo` is authoritative for repository config

Repository config lives twice: the DB `config` jsonb column and the on-disk
`.lumiliorepo`; `UpdateRepository` writes both
(`server/internal/storage/repo_manager.go:525-542`). ADR-004 declares identity
to live on disk but leaves config unassigned. Decision for this plan: **disk is
authoritative, the DB column is a cache, and reconcile overwrites the DB from
disk.** Otherwise a relocated repository can silently revert a rename.

## Remaining work

### 1. Prerequisites (must land before relocate) — DONE

- **Stop `UpdateRepository` from forcing status.** DONE. `status` is gone from
  the `UpdateRepository` query (`repositories.sql`), so status writes funnel
  through `UpdateRepositoryStatus` alone. This required regenerating sqlc
  (`cd server && sqlc generate`) and updating the one call site; there is no
  make target for sqlc.
- **Normalize paths before comparing them.** DONE, as
  `CanonicalizeRepositoryPath` in `server/internal/storage/repo_paths.go`.

Two corrections the implementation forced:

- **Canonicalization must be best-effort, not strict.** `EvalSymlinks` errors
  when the path does not exist, and an offline repository is *defined* by its
  path not existing. A strict helper would turn "this repository is offline"
  into "this path is invalid" for every lookup, including reconcile's own. The
  helper resolves only the deepest existing ancestor and re-appends the missing
  tail verbatim, so an unplugged drive still canonicalizes to a stable,
  comparable path.
- **`EvalSymlinks` does not fix casing.** APFS/HFS+ answer `Stat` for any
  casing, so `/Volumes/photos` and `/Volumes/Photos` both resolve and stay
  distinct strings — exactly the duplicate-row case this step exists to prevent.
  The helper walks the existing prefix and rewrites each component to the
  casing the filesystem actually stores (exact match wins, so case-sensitive
  filesystems are unaffected).

Scope note: the helper is applied at the boundary that reads and writes
`repositories.path` (`repo_manager.go`, `repo_provisioning.go`), not to every
`filepath.Abs(filepath.Clean(...))` in the package. `directory_manager.go` and
`staging_manager.go` receive an already-canonical repository path from those
callers; re-resolving per file operation would only add `ReadDir` calls.

Additional prerequisite found during implementation:

- **Offline repositories must refuse settings edits.** `UpdateRepository` ends
  with `config.SaveConfigToFile(dbRepo.Path)`. With disk authoritative and the
  DB column a cache, accepting the DB half of an edit that cannot reach disk
  forks the two until reconcile silently reverts it on remount. It now returns
  `ErrRepositoryOffline` instead.

### 2. macOS signing/TCC spike (gates step 5, run in parallel with 1)

Empirically answer, on a real ad-hoc-signed `.app`:

- Does selecting a directory outside the app container via the existing picker
  grant durable read/write access across an app restart?
- Does that access survive a rebuild with a changed cdhash, and an in-place
  `desktop/update.go` update?
- What happens on an external volume and on an iCloud Drive folder, with and
  without the missing `NS*UsageDescription` keys?

Outcome decides whether FreePolicy ships as-is, ships with added usage-
description keys, or is blocked on a Developer ID signature. Record the result
here before starting step 5.

### 3. Relocate — DONE

`server/internal/storage/repo_relocate.go`. `AddRepository`'s "ID already
registered" hard error now returns `*RepositoryConflictError` carrying both the
registered and requested paths, which the handler renders as a 409 so the client
can offer the choice. `RelocateRepository` and `RegisterRepositoryCopy` are the
two resolutions, exposed as `POST /repositories/{id}/relocate` and
`POST /repositories/copies`. `RegisterRepositoryCopy` restores the original
`.lumiliorepo` identity if registration fails, so a failed attempt cannot leave
a directory holding a UUID that belongs to no row. Assets are untouched by
construction.

Deviation: the path update is **not** wrapped in a transaction. It is a single
`UPDATE` and therefore already atomic; the only second statement would be the
config-cache refresh, and `DefaultRepositoryManager` holds `*repo.Queries` with
no pool to open a transaction from. The refresh is instead a separate,
best-effort step — if it fails the repository is still correctly located and the
next boot reconcile brings the cache forward. Introducing a pool dependency for
this alone was not worth it.

### 4. Boot reconcile — DONE

`server/internal/storage/repo_reconcile.go`, called from `app.Run` immediately
after the repository manager is constructed. Both constraints hold: `scanning`
is skipped outright, and one unreachable repository neither aborts the loop nor
blocks boot. `.lumiliorepo` missing → `offline`; present but unparseable, or
carrying a different ID → `error`; matching → `active` plus a config-cache
refresh from disk. Covered by six state-machine tests against real temp dirs.

Offline enforcement landed at the two resolvers that every path funnels through,
which is narrower and more reliable than the three call sites this plan
originally listed:

- `getRepositoryForAsset` (`internal/api/handler/media_paths.go`) — every
  thumbnail, original, download and share read — returns
  `storage.ErrRepositoryOffline`.
- `resolveUploadRepository` (`asset_handler.go`) — every ingest path — refuses
  offline targets, mapped to HTTP 409 "Repository is offline".

Corrections to this plan's original list: `asset_service.go:834,1096,1157` do
not resolve a repository at all, they receive `repoPath` as an argument from
those callers, so there was nothing to gate there. `asset_folder_tag_service.go:219`
is deliberately left unfiltered — it builds a `repo_id → name` map for display,
and filtering it would blank out the names of exactly the offline repositories
the user needs to recognize. `asset_handler.go:2480` (`ListIndexingRepositories`)
also still lists everything, but now returns `status` so a selector can keep an
offline repository as a browse filter while refusing it as an upload target;
staging cleanup (`asset_handler.go:3564`) skips offline repositories.

### 5. PathPolicy + API contract (one unit) — DONE

`server/internal/storage/repo_path_policy.go`. `RootedPolicy` wraps
`resolveRepositoryCreatePath` and **rejects** a caller-supplied path rather than
ignoring it — silently relocating the request is worse than refusing it.
`FreePolicy` requires an absolute path, rejects `.photoslibrary`/`.aplibrary`
bundles, and warns (does not reject) on cloud-sync directories. The policy is
injected with `storage.WithPathPolicy`; the default is `RootedPolicy`, so a
deployment cannot accidentally accept free paths. `pathPolicyForRole` keeps an
unpositioned primary repository rooted even under `FreePolicy`.

Deviation: the policy does **not** re-check "already a repository" or "nested
inside a repository". `InitializeRepository` and `AddRepository` already enforce
both on every create path; a second copy in the policy would be a parallel seam
with no distinct behaviour.

`CreateRepository` now returns `*CreateRepositoryResult` (repository plus
location warnings) so warnings have a channel to the user;
`CreateRepositoryResponseDTO` grew `warnings`, and `CreateRepositoryRequestDTO`
grew `path`. Regenerated with `make dto`.

Frontend: `RepositoryOption` grew `status`, with an unrecognized value
normalizing to `active` (wrongly reading a repository as offline would block
writes). `useWorkingRepository` auto-selects only reachable repositories but
leaves an explicit user choice alone. `useCreateRepository` plumbs `path` and
`AddRepositoryModal` surfaces `warnings`. No new i18n keys: the warnings are
server-authored strings shown verbatim.

Not done, deliberately: no free-path input in the web UI. A server runs
`RootedPolicy` and would reject it, so that field belongs with the desktop
picker in step 6, behind the step 2 spike.

### 6. Desktop UI

Reuse the existing picker (`desktop/onboarding.go:298`) for "initialize this
folder as a Lumilio repository" and for the relocate flow, including the
relocate-vs-copy choice.

## Verification

Done:

- `make server-test` and `make web-test` both pass (web: 47 files, 147 tests).
- Path canonicalization: symlinked path, case-differing path, missing path, and
  partially-missing path are covered in `repo_paths_test.go`. The case test
  self-skips on a case-sensitive filesystem.
- Reconcile state machine: six tests in `repo_reconcile_test.go` against real
  temp dirs, including offline → remount → active and the scanning skip.
- Path policy: `repo_path_policy_test.go` covers rooted placement, rejection of
  a caller-supplied path under `RootedPolicy`, absolute-path enforcement,
  `.photoslibrary` rejection, cloud-sync warnings, and the primary-stays-rooted
  rule.

### Windows

Windows desktop is a release-v1 target. Before this work its only CI coverage
was `desktop-windows`, which did trigger on `server/**` (that path is in the
`desktop` paths-filter) but ran `go build ./...` alone, with no tests and
`continue-on-error: true` — so a Windows-only defect could not have failed the
build. Three were found and fixed by review rather than by a failing test:

- `cloudSyncProvider` matched forward-slash markers only, so the OneDrive
  warning — the most valuable one on Windows, where OneDrive is on by default
  and Files On-Demand evicts originals — could never fire. Backslashes are now
  folded **unconditionally** rather than with `filepath.ToSlash`, which is a
  no-op off Windows; that keeps the behaviour testable on the Linux/macOS CI
  that is the only CI running these tests.
- The casing walk in `realCasePath` cannot reach the volume, because there is no
  parent directory to list it from. `c:\photos` and `C:\photos` therefore stayed
  two distinct strings and would produce two `repositories.path` rows for one
  directory. `canonicalVolumeName` now uppercases a drive letter; UNC volumes
  are left alone.
- `TestFreePolicyWarnsAboutCloudSyncedLocations` used hardcoded `/Users/...`
  paths, which `filepath.IsAbs` rejects on Windows (no drive letter), so the
  test would have failed there. It now builds paths from the platform's own
  volume.

Both fixes are covered by platform-independent tests
(`TestCloudSyncProviderMatchesBothSeparators`,
`TestCanonicalVolumeNameUppercasesDriveLetter`) that run on the existing CI.

CI now covers the automated half. A new `server-windows` job runs
`go test ./internal/storage/...` on `windows-2025` as a blocking gate; the
package is CGo-free, so it needs no MSYS2/libvips toolchain and stays fast.
`desktop-windows` lost its `continue-on-error`, making the mingw64 CGo compile
blocking as well — if that job starts failing for toolchain reasons rather than
code reasons, pin or fix the toolchain instead of quietly restoring the flag.

The storage tests have never actually been executed on Windows, only
cross-compiled (`GOOS=windows go vet` and `go test -c` both pass). The first run
of `server-windows` is therefore itself an experiment; a failure there is
information we do not currently have, not necessarily a regression. Two known
risks: `os.Symlink` needs Developer Mode or elevation on Windows (the two
symlink tests self-skip), and 8.3 short paths in the runner's temp directory
would pass through the casing walk unchanged (harmless, since every comparison
is canonical-against-canonical).

Two Windows behaviours remain unverified and need a real machine:

- **Drive-letter reassignment is the Windows relocate case**, and it is far more
  common than the macOS `/Volumes/Name 1` equivalent: a USB drive that was `D:`
  can come back as `E:`. Expected flow is offline → user relocates. Untested.
- Case-insensitive NTFS behaviour of the casing walk, and `EvalSymlinks` against
  junctions/reparse points, are assumed to behave like the APFS path but have
  never been run.

Making `desktop-windows` non-experimental and adding `go test ./internal/storage/...`
to it would cover the automated half cheaply; that is not done here.

Manual integration run against a live server and database (macOS, 2026-07-21),
all passing:

- Move a repository, then relocate: thumbnail and original serve byte-identical
  to the pre-move baseline, `assets.storage_path` unchanged, only
  `repositories.path` updated. This is the assertion that validates the whole
  design — assets are untouched because storage paths are repository-relative.
- Same-ID duplicate: `POST /repositories` returns 409 naming both the
  registered and requested paths; `POST /repositories/copies` then registers it
  with a fresh UUID written to disk, both repositories coexisting, and a repeat
  registration of the same path is refused.
- Case normalization: relocating with an all-caps path stores the real on-disk
  casing and adds no duplicate row. Relocating through a symlink resolves to the
  target. Both would have produced a duplicate row or a unique-constraint
  violation without `CanonicalizeRepositoryPath`.
- Full reachability cycle across restarts: `active` → directory removed →
  restart marks exactly that repository `offline` (the other two untouched) →
  reads return 409 → relocate to the new location returns it to `active` with
  assets intact. A repository whose directory is present is likewise restored
  from `offline` to `active` by reconcile with no user action.

**One defect was found only by this run.** The read path returned **500** for an
offline repository: `getRepositoryForAsset` produced `ErrRepositoryOffline`
correctly, but all six call sites mapped every error to `GinInternalError`, so
the semantic error never reached the client. A 500 reads as a server fault and
leaves the UI unable to distinguish "the drive is unplugged" from "the photo is
gone" — the entire point of the change. Fixed with a shared
`respondRepositoryResolveError` mapping offline to **409**, matching what the
ingest path already returned. Unit tests could not have caught this: they assert
which error `getRepositoryForAsset` returns, not how the HTTP layer translates
it.

Still outstanding:

- The integration scenarios above are manual. `RelocateRepository` and
  `ReconcileAll` have no automated coverage because `DefaultRepositoryManager`
  takes a concrete `*repo.Queries` and cannot be faked; making that seam
  testable is a prerequisite for turning this run into a regression test.
- Deleting a repository that still holds assets returns 500 on a raw
  `assets_repository_id_fkey` violation. Pre-existing and untouched here, but it
  should be a 409 naming the reason.
- `/Volumes/Name 1` remount: the helper canonicalizes such a path fine, but
  macOS assigns the suffixed name at mount time, so it needs the real
  unplug/remount test, not a unit test.
- `make desktop-test` and a manual TCC durability run per step 2.
- Step 6 desktop UI, and the free-path input it carries.
