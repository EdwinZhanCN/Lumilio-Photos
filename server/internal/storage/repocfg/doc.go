// Package repocfg defines a single repository's own configuration: the
// .lumiliorepo file and the catalog column that mirrors it. [RepositoryConfig]
// carries identity and version; [LocalSettings] holds the per-repository
// behaviour (storage strategy, filename preservation, duplicate handling).
//
// The catalog stores it as a typed column; that sqlc mapping is the reviewed
// exception that lets the catalog import a storage type. Shared marker
// primitives remain within the Storage group.
//
// [ReadMarker] distinguishes absent, denied, unknown, corrupt and unsupported
// markers from compatible format 1.0. It validates complete portable identity
// independently from layout. [RepositoryConfig.SaveGuarded] provides atomic
// identity-guarded mutation; legacy saves retain their admission until command
// boundaries refresh identity under leases.
//
//atlas:group storage
package repocfg
