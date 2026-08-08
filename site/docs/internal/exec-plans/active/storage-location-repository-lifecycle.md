# Storage Location And Repository Lifecycle

> Status: draft. This plan records the current direction and intentionally
> leaves product and implementation details open for further discussion.

## Goal

Make Storage Location and repository lifecycle behavior coherent across the
Desktop host, Server control plane, and Web UI without renaming the existing
Repository / `资源库` product concept.

Keep `.lumilioroot` as the portable identity and native authorization boundary
for a storage container, while treating Storage Location as low-frequency
infrastructure in the product experience. Every repository must belong to one
registered Storage Location. Users should primarily work through repository
tasks: create a repository, open an existing repository, recover a moved or
offline repository, and remove a repository without accidentally deleting
original media.

## Agreed Direction

- Keep Repository / `资源库` as the existing user-facing concept. This plan
  does not rename it to Library / `图库` or introduce another synonym.
- Keep `.lumilioroot`. It provides stable identity for a host-authorized
  storage container independently of its absolute mount path.
- Keep `.lumiliorepo`. It provides stable identity for a portable repository
  independently of the catalog row and current path.
- A repository is always a child of exactly one registered Storage Location;
  no newly created, attached, relocated, or copied repository may remain with
  a null or unresolved `root_id`.
- Storage Location remains visible for storage administration and recovery,
  but normal repository workflows should not require users to understand the
  underlying marker or registration protocol.
- Native filesystem authorization remains owned by Desktop. The shared Web API
  must not accept arbitrary host paths.
- Attach, relocate, and register-copy remain distinct backend operations, but
  the UI presents them as task-oriented repository recovery choices rather
  than protocol verbs.

## Current Model

```text
Lumilio instance / SQLite catalog
└── Storage Location (.lumilioroot)
    ├── Repository A (.lumiliorepo)
    └── Repository B (.lumiliorepo)
```

The configured `storage.path` is the non-removable default Storage Location.
Desktop can authorize additional external Storage Locations. Web repository
creation selects a registered `root_id` and creates a child directory under
that root.

Server's in-process `app.RepositoryControl` currently supports listing,
adding, relocating, and removing Storage Locations, plus attaching a repository
and resolving a repository identity conflict as either `relocate` or `copy`.
Desktop's `runtime.StorageControl` exposes only list, add, and attach, and the
Desktop UI currently exposes Storage Location addition but not the complete
repository recovery workflow.

## Problems To Resolve

### Invariant gap

`AttachRepository(path)` accepts a repository directory directly. Association
with a Storage Location is currently inferred from its path and can be absent.
This weakens the intended parent-child model and makes authorization,
reachability, relocation, and removal behavior harder to explain.

### Split task ownership

Storage Location authorization is performed in Desktop, repository creation is
performed in Web, and several conflict-resolution operations exist only in the
Server control plane. A user cannot complete every normal or recovery journey
through one coherent workflow.

### Infrastructure-first UI

The current UI exposes Storage Location selection and repository policy details
before establishing the user's task. Low-frequency storage topology competes
with the primary jobs of creating or reopening a repository.

### Incomplete conflict recovery

The backend distinguishes a moved repository from a copied repository, but the
Desktop projection and UI do not expose structured conflicts or either
resolution. Failures are reduced to a generic recovery error.

### Ambiguous destructive semantics

Removing a catalog registration and deleting repository files are materially
different operations. The product needs explicit contracts and language so a
user never interprets one as the other.

## Target Product Workflows

### Create repository

The primary flow asks for a repository name and destination Storage Location.
It uses safe repository-policy defaults. File-layout and duplicate-filename
policies may remain available as advanced settings rather than dominating the
initial task.

If no suitable active Storage Location exists, the flow offers a native
Desktop authorization handoff and resumes repository creation after the
location is registered. The user should not have to discover a separate
Control Panel page and restart the task manually.

### Open existing repository

"Open existing repository" is the user-facing entry for attaching a directory
that already contains a valid `.lumiliorepo`.

The selected repository must be inside an active registered Storage Location.
If it is not, the workflow must obtain an explicit native grant and register an
appropriate Storage Location before attaching the repository. Attachment must
not silently create a repository with no `root_id`.

### Recover moved repository

When the selected `.lumiliorepo` identity is already registered at another
path, the UI explains that the repository appears to have moved and offers to
update its location. The Server verifies repository identity before changing
the registered path.

If an entire Storage Location moved, recovery should occur at the Storage
Location level so all child repository paths are validated and relocated
together.

### Register repository copy

When the selected directory is a deliberate copy of an already registered
repository, the UI offers to add it as a separate repository. The Server mints
a fresh repository UUID in the copied `.lumiliorepo`, associates it with the
selected Storage Location, and preserves the existing repository registration.

The UI must explain that this establishes a separate repository identity; it
is not a backup-link or live synchronization relationship.

### Offline storage and repositories

Offline Storage Locations and repositories remain visible with their last
known names and paths. Recovery actions are contextual:

- reconnect or relocate the Storage Location when its `.lumilioroot` identity
  is found at a new path;
- locate an individual repository when only that repository moved within an
  otherwise active Storage Location;
- do not silently switch write destinations when an explicitly selected
  repository becomes unavailable.

### Remove repository or Storage Location

Removing a repository unregisters its catalog record and leaves
`.lumiliorepo`, original media, and repository-owned files on disk so it can be
opened again later.

Removing an external Storage Location is allowed only when no registered
repositories belong to it. The configured default Storage Location remains
non-removable.

Physical deletion of a repository and its media, if supported, is a separate
explicitly destructive workflow and is not implied by either removal action.

## Server And Data Contracts

- Enforce non-null valid `repositories.root_id` for every active repository.
- Validate that repository paths are strict descendants of their associated
  Storage Location and that Storage Locations do not overlap.
- Change attach to resolve or require a registered Storage Location before
  inserting the repository record.
- Preserve structured Storage Location identity conflicts with the supported
  `relocate` resolution.
- Preserve structured repository identity conflicts with the supported
  `relocate` and `copy` resolutions.
- Relocating a Storage Location validates the `.lumilioroot` identity and every
  child `.lumiliorepo` identity before committing the new paths.
- Relocating an individual repository must keep it inside its associated
  active Storage Location, unless the same operation explicitly selects and
  validates a different registered parent.
- Registering a repository copy mints a new repository identity and assigns the
  current Host Owner as its default ingest owner, consistent with repository
  creation and attachment.
- Removal operations do not delete original media or repository-owned files.
- API contracts remain OpenAPI-first; host path grants remain outside the
  shared HTTP API.

The migration policy for existing repository rows with a null `root_id` is not
yet decided. It must be explicit and fail safely rather than guessing across
overlapping or unavailable paths.

## Desktop Control Plane

- Extend the Desktop-safe `runtime.StorageControl` projection to carry the
  structured operations required by the approved workflows rather than
  flattening them into generic recovery failures.
- Preserve request-ID idempotency, expected-version checks, operation receipts,
  generation ownership, and clearing of stale controls during restart.
- Use native directory selection for Storage Location authorization and opening
  an existing `.lumiliorepo` directory.
- Return structured conflict facts and allowed resolutions to the UI, including
  registered path, requested path, identity, and supported actions.
- Refresh the shared storage/repository state after successful mutations so
  Desktop and Web do not present contradictory status.

The exact Web-to-Desktop handoff mechanism for a flow initiated in the browser
is still open and must respect the existing boundary that the Desktop Settings
WebView does not call the Server HTTP API.

## UI/UX Contracts

- Repository / `资源库` remains the primary product noun.
- Storage Location is shown when choosing a destination, inspecting capacity or
  reachability, authorizing external storage, and recovering a moved/offline
  container. Marker filenames and internal IDs are diagnostic details, not
  primary labels.
- Primary actions are task-oriented: create repository, open existing
  repository, add storage location, locate/reconnect, and remove from Lumilio.
- Do not expose raw `attach`, `relocate`, or `copy` action tokens as unexplained
  button labels.
- Create and recovery flows represent loading, cancellation, offline paths,
  identity conflicts, partial failure, and successful refresh explicitly.
- Removal copy states that disk files are preserved. Any physical-delete flow
  uses separate language, confirmation, and authorization.
- Repository policy defaults remain deterministic. Advanced policy controls do
  not block a simple create flow.
- The UI must not duplicate mutation ownership between independent Desktop and
  Web controls. A workflow may hand off to the native host, but has one visible
  task state and a clear resume point.

## Proposed Implementation Slices

### 1. Specify and enforce ownership invariants

- Inventory repository rows and code paths that permit a missing `root_id`.
- Decide the migration and recovery policy for legacy/unassociated rows.
- Enforce repository-to-Storage-Location association in creation, attachment,
  relocation, copy registration, reconciliation, and database constraints where
  appropriate.
- Add focused Server tests for path containment, offline roots, identity
  mismatch, overlap, and rollback.

### 2. Complete the in-process lifecycle contract

- Refine `RepositoryControl` inputs/results around explicit parent association
  and structured conflicts.
- Project the required methods and typed conflict data through Desktop runtime,
  storage controller, operation registry, DTOs, and generated Wails bindings.
- Preserve the native authorization and runtime generation boundaries.

### 3. Build a coherent recovery surface

- Add the approved entry points for opening an existing repository and
  recovering offline/moved storage.
- Present relocate-versus-copy decisions using user-facing explanations.
- Ensure state refresh and retry behavior survive Desktop/Server restart
  boundaries.

### 4. Simplify repository creation

- Keep repository name and Storage Location selection in the primary path.
- Move optional storage strategy, duplicate handling, and cloud-specific
  details according to the final product decision.
- Provide an in-context path to authorize a new Storage Location and resume the
  original creation intent.

### 5. Complete removal and diagnostics

- Expose safe repository unregister and eligible external Storage Location
  removal with explicit file-preservation copy.
- Show child repository counts and blocking reasons before Storage Location
  removal.
- Keep physical deletion out of scope unless its separate contract is approved.
- Update user documentation and durable architecture documentation.

## Open Questions

1. Which surface owns the unified repository lifecycle: the Web Manage page,
   the Desktop Control Panel, or a coordinated handoff between both?
2. When opening a `.lumiliorepo` outside any registered Storage Location, which
   directory becomes the new Storage Location: the repository's parent, a
   user-selected ancestor, or an explicitly selected container?
3. Should an individual repository be allowed to move between two already
   registered Storage Locations, and if so, is that relocation or a distinct
   move operation that also moves files?
4. How should existing database rows with null `root_id` be migrated when their
   paths are offline or outside all registered locations?
5. Should advanced repository policies remain in the create flow behind a
   disclosure, move to repository settings, or be removed from user control?
6. How should Cloud source selection relate to repository creation: creation
   option, post-create import action, or separate guided flow?
7. What should the final user-facing names be for Storage Location actions in
   English and Simplified Chinese while keeping Repository / `资源库` unchanged?
8. Is physical repository deletion in scope, or should Lumilio only support
   unregistering repositories in this lifecycle project?
9. What capacity/free-space and filesystem facts must be visible for a Storage
   Location, and which are diagnostics only?
10. Which repository and Storage Location operations require audit records or
    administrator-only authorization?

## Out Of Scope For This Draft

- Renaming Repository / `资源库`.
- Replacing the application-wide SQLite catalog with per-repository catalogs.
- Making `.lumilioroot` or `.lumiliorepo` user-editable configuration.
- Allowing arbitrary host paths through the shared HTTP API.
- Treating repository copies as synchronized replicas or backups.
- Defining physical media deletion before its product and safety contract is
  approved.

## Validation Boundaries

The exact commands may expand as implementation scope is finalized. At minimum:

- `task ci:architecture`
- `task server:test`
- `task web:test`
- `task desktop:test`
- `task dto` after HTTP DTO or annotation changes
- Wails binding regeneration/checks after Desktop DTO changes
- `vp exec i18next-cli extract` followed by filling generated Chinese values
  for Web copy changes
- Desktop frontend localization generation/checks for Desktop copy changes
- focused tests covering create, attach, Storage Location registration,
  relocate, copy registration, offline recovery, removal guards, idempotency,
  restart generation safety, and non-destructive failure rollback
- `git diff --check`

Before implementation, update affected workflow path filters in the same change
whenever CI-relevant Taskfile targets or orchestration change.
