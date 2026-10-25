// Package storage owns Lumilio's on-disk media layout and the lifecycle of
// repositories. It is the single authority over registered Storage Locations
// and how each repository is structured on disk; other packages
// reach storage only through its interfaces and never touch repository paths
// directly.
//
// # Responsibilities
//
//   - [RepositoryManager]: repository lifecycle — create, register existing,
//     look up, list, update, reconnect, remove — keeping catalog records and
//     the on-disk repository in sync. Constructors return the concrete
//     [DefaultRepositoryManager]; callers depend on the narrow slice they
//     need. Removal purges the repository's entries through
//     [server/internal/lifecycle.PurgeRepositoryEntriesTx].
//   - [DirectoryManager]: the structure inside one repository — inbox,
//     staging, trash, sidecars, system directories — and the file operations
//     over them.
//   - [StagingManager]: transient staging files for uploads and cloud
//     imports, before the sourcing pipeline commits them into the inbox.
//   - [RepositoryFS] and [RepositoryFSFactory]: every repository file
//     operation is rooted here and serialized against reconnect and
//     removal; nothing else opens repository paths.
//   - [server/internal/storage/scan]: the repository scan index. The catalog
//     mirrors each repository tree in repository_entries; walks diff
//     directories against it and a bounded hash pass binds content to Assets.
//   - [server/internal/storage/trash]: journaled Delete and Restore through
//     the repository trash.
//   - [server/internal/storage/locations]: resolves an Asset to a present
//     file just before media I/O.
//   - [server/internal/storage/repocfg] and [server/internal/storage/rootcfg]:
//     the .lumiliorepo and .lumilioroot files.
//   - Repository ownership is deliberately not per-repository. The first
//     account is the Host Owner and is every repository's fallback owner for
//     filesystem discovery; explicit upload owners and stable cloud binding
//     owners still win.
//
// # Storage layout
//
// The configured storage.path is the non-removable default Storage Location.
// External locations are registered by portable .lumilioroot identity. A
// Storage Location contains only its marker and repository directories:
//
//	<path>/.lumilioroot  portable Storage Location identity
//	<path>/primary       the mandatory primary repository (default only)
//	<path>/<name>        additional user-created repositories
//
// Cloud sessions, secrets, logs, and backups are app-private state configured
// outside storage.path. Repository staging remains repository-owned under
// .lumilio because it is recoverable work tied to that repository.
//
// Identity and admission: repositories.role is primary or regular, and the
// instance is set up only when an admin exists and exactly one active primary
// Repository exists. Additional Storage Locations are storage_locations rows
// keyed by the UUID in .lumilioroot; their summaries are catalog projections
// that never authorize or deny child Repository I/O. Admission is decided per
// Repository: [UploadAdmission] for upload and cloud materialization, and
// the role-aware action policy [DeriveRepositoryActions] for verify, rename,
// detach, and reconnect. A command re-derives that policy on the row it reads
// under its mutation lease, refreshes the complete marker there, writes it
// with a guarded atomic replace, journals the operation, and returns a
// [LifecycleReceipt] a retry with the same request receives again; a withheld
// action is a [RepositoryActionUnavailableError] with stable blocker codes.
//
// Setup recovery (storage plan P5): [DefaultRepositoryManager.AssessSetupPrimaryTarget] reviews the
// fixed <default>/primary target into a closed [SetupPrimaryState] with the
// one legal transition per state, and [DefaultRepositoryManager.AdoptPrimaryRepository] registers a
// compatible, unregistered Repository found there as the Primary Repository,
// bound to that review (identity, token, risks), journaled as
// adopt_primary_repository with the P4 private preservation under an adopt
// receipt, and completing only a missing layout. Before the Primary
// Repository exists, a different configured storage.path replaces an unused
// default registration (fresh default) and leaves the previous tree untouched;
// afterwards it must prove the moved identity. [DefaultRepositoryManager.AssessDefaultStorageLocation]
// names the state of storage.path read-only for setup and startup.
//
// Volume semantics (storage plan P6b, R11): repositories.path_case and
// path_normalization hold the name comparison rule measured on the tree by
// [server/internal/storage/pathsemantics.Qualify] when a Repository is
// created, opened, copied, adopted or reconnected; [RepositoryVolumeRule]
// reads it and the scan index keys under it. A reconnect whose destination
// follows another rule proves every live key distinct first
// ([PathKeyCollisionError] otherwise), cuts the path and the rule over
// together into maintenance, rebuilds path_key/parent_key in bounded,
// revision-fenced batches, and releases maintenance; journal recovery
// finishes an interrupted rebuild. A tree whose rule cannot be qualified is
// neither registered nor reconnected.
//
// [StorageObserver] supplies only read/stat/mount facts to
// [ObserveStorageTarget] and [AssessStorageTarget]. [ClassifyStorage] and
// [DeriveStorageCapabilities] are pure; mutation capabilities remain conditional
// until explicit ownership/write preflight. Child observations never depend on
// parent marker health. [RepositoryLockProvider] and
// [RepositoryIdentityDetector] keep local ownership and identity replaceable;
// alternatives are installed before serving, preserving rooted I/O.
//
//atlas:group storage
package storage
