# Storage Location And Repository Lifecycle

> Status: completed on 2026-08-08 after implementation and independent
> requirement-by-requirement review. Every ADR below has direct implementation
> and test evidence. The decisions are recorded in
> [Lumilio Storage Lifecycle ADRs](Lumilio-Storage-Lifecycle-ADRs.zh-CN.md).

## Goal

Make Storage Location and repository lifecycle behavior coherent across the
Desktop host, Server control plane, Web UI, and Docker deployment while
preserving original media and the existing Repository / `资源库` product name.

Every repository belongs to one registered Storage Location. `.lumilioroot`
is the portable parent identity and native authorization boundary;
`.lumiliorepo` is the portable repository identity. Users work through
task-oriented create, open, reconnect, locate, copy-registration, and safe
removal flows rather than protocol verbs.

## Final Implementation Review

Independent read-only reviews compared every ADR with Server, Web, Desktop,
database, routing, user-path, crash-recovery, concurrency, and test evidence.
Findings were returned to implementation until each contract closed.

| ADR | Implementation status | Final evidence boundary |
| --- | --- | --- |
| ADR-001 | Complete | Durable actor-scoped host actions, restart recovery, global Web discovery, and a Desktop-only capability-broker path. |
| ADR-002 | Complete | Direct-child topology, nested-marker stop, real root/repository OS locks, recovery-target locking, and actual-target capacity checks. |
| ADR-003 | Complete | Safe move/copy identity rules, stable lifecycle requests, cross-root relocation, crash recovery, and durable initial-scan retry. |
| ADR-004 | Complete | Fail-closed default identity, degraded runtime recovery, real Desktop runtime migration/rollback, structured create conflicts, and narrow rename. |
| ADR-005 | Complete | Generation-5 ownership invariants, parent-first write validation, background/explicit reconciliation, and global degraded recovery. |
| ADR-006 | Complete | Durable maintenance barriers, queue/worker coordination, transactional reprocess fan-out, journaled filesystem-plus-catalog mutations, and safe removal accounting. |
| ADR-007 | Complete | Every Web creation entry selects `date` / `flat` / `cas` (default `date`); layout is immutable after create; duplicate and Cloud options remain outside create. |
| ADR-008 | Complete | Post-create multi-source Cloud bindings, remote scopes, isolated cursors, durable cancel/resume receipts, startup recovery, and staging ownership. |
| ADR-009 | Complete | Canonical lifecycle/Repository/capability terminology, fixed removal copy, and repository-wide regression gates with narrow technical-context allowances. |
| ADR-010 | Complete | Capacity preflight and recovery, continuous unknown-stream checks, actual-target/device risks, two-stage confirmation, cross-process locking, and diagnostics. |
| ADR-011 | Complete | Persistent actor/host/request/operation audit coverage, atomic mutation/audit boundaries, admin history, rejected/recovery events, and path-redacted support bundles. |
| ADR-012 | Complete | Lifecycle removal has no physical-delete API or hidden parameter and preserves originals, markers, and private state. |

### Release-blocking safety defects resolved during implementation

- Server startup previously could recreate a missing registered default-root directory and
  `.lumilioroot` marker instead of failing closed when a mount is absent.
- Repository and root coordination previously used only process-local `sync.RWMutex`; a
  second process can bypass the single-writer contract.
- A copy of the Primary Repository previously could be registered as a regular repository
  because the copy path trusts the requested role instead of the marker's
  registered identity.
- Scanning previously crossed a nested `.lumiliorepo` boundary instead of returning a
  topology error.
- Lifecycle tasks previously had no hard capacity preflight or continuous low-space
  pause, and Docker child mounts can show the parent root's capacity.
- Successful repository audit operations were logged at `Debug` while the
  default operations logger starts at `Info`; root audit paths can also resolve
  to a no-op logger. The claimed persistent audit record is therefore absent
  for important operations.

## Accepted Target Contracts

### Topology and schema

- `repositories.root_id` is `NOT NULL` with `ON DELETE RESTRICT` in schema
  version 5. This pre-release change deliberately updates the SQLite baseline
  rather than carrying a compatibility migration for an unreleased schema.
- Every regular repository is a canonical direct child of exactly one
  registered Storage Location. Overlapping roots, nested repositories, path
  aliases that escape the parent, and unassociated active repositories are
  rejected.
- Repository display name and stable `directory_name` are separate. Renaming a
  repository never silently changes its disk path.
- Reachability and activity are separate states. Persistent bootstrap readiness
  is not reset when the default Storage Location or Primary Repository is
  temporarily unavailable; the runtime becomes degraded instead.

### Create and open

- Web creation asks for a name, direct-child directory, active writable
  Storage Location, and immutable `date` / `flat` / `cas` storage strategy.
  Duplicate handling remains a Server-owned deterministic default; cloud
  import is a separate post-create workflow.
- Create never implicitly opens an existing `.lumiliorepo`. Existing markers
  return a structured conflict and direct the user to Open Existing Repository.
- Desktop deployments use persistent `host_actions`. Web owns the visible task;
  the local Desktop user reviews it and chooses a directory with the native
  picker. Raw selected paths and one-time nonces never cross the shared HTTP
  DTO boundary.
- Opening a repository under an already registered root isolates stale active
  `.lumilio/` state into recovery storage, registers the repository, and queues
  an authoritative initial scan without changing original media.
- Opening a repository outside every registered root is intentionally a
  two-step explicit grant: add its direct parent as a Storage Location, then
  open the repository. This avoids silently widening a native grant from one
  repository directory to its parent container.
- Docker and standalone Server accept only a portable direct-child directory
  name below the configured default root. They never accept an arbitrary host
  path. Existing children are classified before create/open.

### Moved identities and independent copies

- A moved Storage Location is reconnected only after its `.lumilioroot`
  identity and every child `.lumiliorepo` are validated. Root and child path
  updates commit atomically.
- A moved repository can be located only at a direct child of an active root.
  Relocation is refused while the registered original identity is still online.
- An independently copied repository requires an explicit confirmation. The
  operation preserves media, isolates copied private state, writes a fresh
  repository UUID, and queues an initial scan. It does not create a sync or
  backup relationship.
- Web exposes the same moved-original and separate-copy choices for bounded
  Docker/Server candidates and Desktop identity conflicts.

### Offline behavior and scanning

- Missing, unreadable, or identity-mismatched roots and repositories fail
  closed. They remain visible with their last known names and paths but reject
  writes.
- Reconciliation can reactivate an identity that returns at the same path. It
  never creates replacement markers or adopts a different UUID automatically.
- Partial directory reads, files still being written, and ambiguous content
  matches do not prove deletion. Authoritative missing-file handling runs only
  after a complete scan.
- A unique full-content match can preserve an asset identity when the user
  moves or renames a file inside the same repository. Scanning never moves the
  original itself.

### Durability, concurrency, and safe removal

- Lifecycle mutations use root/repository locks, maintenance barriers, request
  idempotency, persistent audit records, and `lifecycle_operations` journaling
  whenever filesystem and catalog state change together.
- Journal rollback includes a complete private-state isolation plan before the
  first filesystem move, so a crash between individual moves remains
  recoverable.
- Removing a regular repository requires an impact preview and exact display
  name. The Primary Repository and repositories with active work are blocked.
  Catalog-scoped data is removed, while `.lumiliorepo`, `.lumilio/`, `inbox/`,
  original media, and every other disk file are preserved.
- Removing an external Storage Location is allowed only when it has no
  registered repositories or active lifecycle operation. The default location
  is never removable. `.lumilioroot` and all disk files are preserved.
- Physical deletion of original media is not implemented by any lifecycle API
  or UI action.

### Capacity and deployment

- Target summaries show reachability, writability, available/total capacity,
  repository count, and the admin-visible path. Filesystem type remains a
  diagnostic field rather than a primary placement signal.
- Linux validates an already-existing empty direct-child repository target
  against `/proc/self/mountinfo`; Lumilio-created ordinary child directories do
  not need to be mount points.
- Official Compose files use long bind-mount syntax with
  `create_host_path: false`, preventing a misspelled host path from becoming an
  unintended empty directory.

## Generated Contracts

The implementation keeps generated artifacts generated:

- `task server:sqlc` for query bindings;
- `task dto` for OpenAPI, TypeScript schema, and Redoc output;
- Desktop Wails binding generation for control-plane DTO changes;
- `vp exec i18next-cli extract`, followed by filling generated Simplified
  Chinese values, for Web localization;
- feature `doc.md` generation from the repository feature's canonical
  `doc.ts`.

## Validation Boundaries

The following final gates passed after every ADR had direct implementation and
independent requirement-by-requirement review:

- `task ci:architecture`
- `task server:test`
- `task web:test`
- `task web:build`
- `task desktop:test`
- `task desktop:frontend:build`
- `task desktop:compile`
- `task compose:test`
- `task ci:site`
- `git diff --check`

Focused Server coverage includes create/open, direct-child containment,
Storage Location registration and atomic relocation, moved-versus-copy
resolution, private-state rollback, offline reconciliation, capacity
inspection, scan authority, safe repository/root removal, persistent host
actions, idempotency, and sticky bootstrap readiness.

## Deliberately Out Of Scope

- Per-repository application catalogs.
- Arbitrary host paths in shared HTTP APIs.
- Treating repository copies as synchronized replicas or backups.
- Web runtime relocation of the configured default Storage Location; Desktop
  relocation remains an explicit runtime-intent, validation, and controlled
  restart operation.
- Physical deletion of repository media.
