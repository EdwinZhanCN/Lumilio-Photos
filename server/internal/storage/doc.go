// Package storage owns Lumilio's on-disk media layout and the lifecycle of
// repositories. It is the single authority over what exists under the immutable
// storage root and how each repository is structured on disk; other packages
// reach storage only through its interfaces and never touch repository paths
// directly.
//
// # Responsibilities
//
//   - RepositoryManager (repo_manager.go): repository lifecycle — create,
//     register existing, look up, list, update, remove — keeping the database
//     records and the on-disk repository in sync. It is the consumer-facing
//     contract; constructors return the concrete *DefaultRepositoryManager and
//     callers depend on the narrow slice they need.
//   - DirectoryManager (directory_manager.go): the structure *inside* a single
//     repository — inbox, staging, trash, sidecars, system directories — plus
//     the file operations over them (commit, trash, recover, sidecar I/O).
//   - StagingManager (staging_manager.go): transient staging files used while an
//     asset is being ingested, before it is committed into a repository.
//   - scanner (subpackage): periodic filesystem scans that reconcile a
//     repository's on-disk contents with the database.
//   - repocfg (subpackage): a single repository's own configuration — the
//     .lumiliorepo file and the DB config column (storage strategy, filename
//     preservation, duplicate handling). This is per-repository mutable
//     behaviour and is owned here, decoupled from the global settings service.
//
// # Storage layout
//
// The storage root <path> is immutable boot configuration (see server/config,
// StorageConfig). Every well-known location under it is derived by convention,
// not configured:
//
//	<path>/primary       the mandatory primary repository
//	<path>/<name>        additional user-created repositories
//	<path>/.secrets      db_password and the app secret key
//	<path>/.cloud        cloud sync working area
//
// # Direction (in progress)
//
// Provisioning of the root layout and the mandatory primary repository, plus
// repository default behaviour, is being consolidated into this package so the
// bootstrap "dirs_ready" gate has a single owner. Until that lands, root
// directory creation and repository-creation policy are still partly performed
// by the setup flow and the HTTP handlers. See
// site/docs/internal/agent/exec-plans/active/config-settings-bootstrap-refactor.md.
package storage
