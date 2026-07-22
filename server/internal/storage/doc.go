// Package storage owns Lumilio's on-disk media layout and the lifecycle of
// repositories. It is the single authority over registered Storage Locations
// and how each repository is structured on disk; other packages
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
//   - Repository ownership is deliberately not per-repository. The first
//     account is the Host Owner and is used as every repository's fallback
//     owner for filesystem discovery; explicit upload owners and stable cloud
//     binding owners still win.
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
package storage
