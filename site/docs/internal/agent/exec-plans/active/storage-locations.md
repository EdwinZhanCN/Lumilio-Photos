# Storage Locations

Status: completed 2026-07-21. This plan supersedes the unrestricted Desktop `FreePolicy`
direction in `repository-relocate.md`: native Desktop authorization produces a
bounded Storage Location, and the shared Web API only creates repositories by
location id.

## Goal

Make repository placement explicit and symmetric across Desktop and Server:

- App-private state stays machine-local and is never inferred from a media
  directory.
- A Storage Location is an authorized directory that may contain repositories.
  Its portable identity is stored in `.lumilioroot`.
- A repository remains a portable `.lumiliorepo` directory whose on-disk config
  is authoritative.
- Web clients choose a registered Storage Location; they never submit an
  arbitrary filesystem path.
- The Desktop Control Panel is the only surface that can grant an external host
  path or attach an existing external repository.

This is deliberately not called a workspace: database, credentials, cloud
sessions, logs, model state, and backups do not move with a Storage Location.

## Filesystem contract

```text
app-private state (machine local)
├── postgres/
├── config/
├── secrets/
├── cloud/
├── logs/
├── lumen/
└── backups/              # explicit/configurable, Desktop defaults local

storage location
├── .lumilioroot          # id, name, version, created_at
├── primary/              # optional; the one primary repository
│   ├── .lumiliorepo
│   └── .lumilio/
└── another-repository/
    ├── .lumiliorepo
    └── .lumilio/
```

`.cloud` and `.secrets` are not Storage Location contents. Cloud-import staging
remains repository-owned under `.lumilio/staging`, because it is recoverable
work for that repository rather than a credential/session artifact.

## Contracts

- `repository_roots` stores location id, name, canonical path, kind
  (`default`/`external`), reachability status, and timestamps.
- `repositories.root_id` records which registered root authorized creation;
  directly attached external repositories may have no root.
- The configured `storage.path` is registered as the non-removable default
  location during startup. Existing repositories are associated with it when
  their canonical paths are contained by it.
- `POST /api/v1/repositories` accepts `root_id` and applies explicit
  `storage_strategy` and `duplicate_handling`. Missing `root_id` remains a
  compatibility path to the configured default location.
- `GET /api/v1/repository-roots` is admin-only and returns current reachability.
- Desktop calls the in-process repository manager after a native picker grant;
  the shared authenticated API never gains an arbitrary-path attach endpoint.
- Same-id repository conflicts return structured registered/requested paths and
  require an explicit relocate-or-copy choice.

## TODO

- [x] Add explicit repository policy controls to local and cloud creation.
- [x] Add `.lumilioroot`, the repository-root migration/query layer, startup
      registration/reconcile, and admin list DTO.
- [x] Change Web creation from implicit root/path semantics to `root_id`.
- [x] Split cloud credential artifacts and database backups from
      `storage.path`; stop creating `.cloud`/`.secrets` below media roots.
- [x] Add Desktop Control Panel actions for external Storage Locations and
      existing `.lumiliorepo` repositories via the native directory picker.
- [x] Add explicit relocate/copy conflict resolution in the Control Panel.
- [x] Surface offline roots/repositories and prevent writes while preserving
      browse identity.
- [x] Regenerate sqlc/OpenAPI/i18n/docs and run Server, Web, Desktop, Docker E2E,
      and native Windows validation.

## Validation boundary

- Root containment and identity tests must cover POSIX paths, Windows drive
  letters, case normalization, missing/remounted roots, nested repositories,
  and a same-id marker at another path.
- Local and cloud create tests must assert the same root/storage/duplicate body
  fields; only `cloud_credential_id` differs.
- App-state tests must prove cloud/session and backup paths do not follow an
  external media root.
- Desktop tests must prove cancelled pickers do not mutate grants and an
  unreachable grant is reported rather than silently replaced by another root.
- Native Windows verification uses the Parallels VM in addition to mirrored
  macOS/Windows Desktop CI.

## Validation record

- `make server-test`: pass, including Storage Location identity, containment,
  reconcile, relocation mapping, private-state configuration, and migrations.
- `make web-test`: 51 files / 189 tests pass. Local and cloud creation submit
  the same root, storage-strategy, and duplicate-policy fields.
- `make desktop-test`: Svelte check/build and Desktop Go tests pass, including
  cancelled native pickers returning before repository control is accessed.
- `make web-browser-test`: 4 Docker-backed Playwright smoke tests pass; the E2E
  containers, networks, and volumes were removed afterwards.
- `sqlc generate`, `make dto`, doc generation, and i18n status all pass; Chinese
  repository strings report 100% coverage.
- Windows cross-vet passes. On Windows 11 under Parallels, the storage,
  `.lumilioroot`, filesystem-DACL, and portable config suites pass; NTFS casing
  and `D:\\Lumilio` → `E:\\Lumilio` relocation mapping were exercised natively.
  The VM was returned to its original suspended state and test artifacts were
  removed.

### Critical Files for Implementation

- `server/internal/storage/repo_manager.go`
- `server/internal/api/handler/repository_scan_handler.go`
- `server/config/config.go`
- `desktop/control_panel.go`
- `web/src/features/repositories/flows/manage/AddRepositoryModal.tsx`
