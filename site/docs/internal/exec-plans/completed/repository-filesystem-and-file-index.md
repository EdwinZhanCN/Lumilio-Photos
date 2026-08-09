# Repository Filesystem And File Index

Status: completed and verified on 2026-08-08.

## Goal

Give users control over completed originals, including files uploaded into
`inbox/`, while preserving Lumilio asset identity and preventing repository
scans, ingest, and asynchronous jobs from racing one another.

## Shipped contract

- `storage.RepositoryFS` is the single repository-rooted filesystem boundary.
  It uses Go `os.Root`, repository-relative typed paths, marker validation, and
  repository lifecycle leases. The factory refreshes the repository record
  from SQLite after acquiring a lease so an open cannot retain a pre-relocate
  absolute path.
- The user-media namespace includes `inbox/**` and every other supported media
  path. `.lumilio/**` is private application state and is never scanned as
  user media. Symlinked directories and out-of-repository file targets are
  rejected; contained symlinks to regular files are supported.
- Migration 5 adds a rebuildable SQLite file index and durable scan state.
  Historical migrations remain unchanged. The index records repository-relative
  paths, observations, platform file identity hints, hashes, asset bindings,
  absence counts, ambiguity, and scan authority.
- Scans walk supported media without a watcher. Stable files are revalidated
  around hashing. Same-repository moves preserve the Asset ID only after an
  unambiguous full BLAKE3 match. Duplicate candidate groups are deferred rather
  than guessed, and missing files require two consecutive authoritative scans
  before soft deletion.
- Partial scans never confirm absence. Settling files, interrupted runs, and
  traversal errors retain enough state for a later authoritative scan. A reset
  rebuilds the index without changing catalog identity.
- Existing recoverable ingest state is the authoritative claim for a reserved
  destination. Upload materialization returns only after the staged file is
  durably committed and its file-index row is bound to the Asset in the same
  database transaction. Recovery is idempotent across process restarts.
- Cloud import advances its provider cursor only after materialization and
  provider acknowledgement succeed. Failure leaves the cursor unchanged and
  keeps recoverable staging evidence.
- Metadata, thumbnail, transcode, perceptual-hash, and ML work carries an
  Asset ID plus observation token/hash, not an absolute source path. Workers
  resolve the current indexed source at execution time, allowing a verified
  move to satisfy work queued before the move.
- Original downloads, shares, ZIP export, analysis reads, derived-file writes,
  staging, upload chunks, and native-tool path handoff now pass through the
  repository boundary. An architecture check rejects new direct repository-root
  joins outside the owning package.
- Repository scan responses and the maintenance UI distinguish authoritative,
  partial, ambiguous, moved, indexed, and deletion-pending outcomes. English
  and Simplified Chinese documentation explain that Inbox is a landing area,
  not an immutable ownership boundary.

Primary owners: `server/internal/storage`, `server/internal/storage/scanner`,
`server/internal/sourcing`, `server/internal/cloud`,
`server/internal/processors`, and `web/src/features/manage`.

## Useful decisions

- Use a domain-specific concrete `RepositoryFS` over `os.Root`; do not add a
  general VFS dependency or an interface hierarchy around every operation.
- Store the rebuildable index in application SQLite, not a repository sidecar.
- Treat identical physical paths as separate Assets; duplicate grouping remains
  a separate catalog concern.
- Preserve the Asset ID for same-path content replacement, then invalidate and
  enqueue derived processing for the new content facts.
- Use platform identity, size, and quick fingerprints only to find candidates;
  full BLAKE3 equality is the move proof.
- Do not bypass settling for forced scans, guess ambiguous matches, or confirm
  deletion from partial observations.
- Use existing recoverable ingest state instead of adding a claims table.
- Keep watcher support, file-manager features, and cross-repository move
  identity outside this implementation.

The separate active
[`storage-location-repository-lifecycle.md`](storage-location-repository-lifecycle.md)
continues to own Storage Location authorization, `.lumilioroot`, repository
attach/relocate/remove semantics, and their management workflows.

## Verification evidence

- `task server:test` passes, including RepositoryFS policy, scan reconciliation,
  ingest recovery, cloud cursor, and stale-job tests.
- `task web:test` passes: 70 test files and 303 tests passed; type, lint, and
  source-boundary checks also pass.
- `task desktop:test` passes with the race detector. The macOS linker emitted
  existing `LC_DYSYMTAB` warnings but no test failure.
- `task ci:architecture` passes the SQLite, browse, Desktop, repository-path,
  Compose, Lumen-lock, and asset-lock checks.
- `task server:sqlc` and `task dto` regenerated the query and OpenAPI clients.
  `vp exec i18next-cli extract` regenerated locale keys before Chinese values
  were filled.
- `task site:checks` and `task site:build` pass.
- Darwin tests exercise the native implementation; Linux and Windows file
  identity sources were compile-checked. Native runtime behavior remains owned
  by the platform CI matrix.
- `git diff --check` passes.

## Completion boundary

No compatibility layer was retained because the project has no production
deployment to migrate. Filesystem watching, repository browsing/organizing UI,
cross-repository identity transfer, and generalized VFS backends remain out of
scope; none is required for the contracts above.
