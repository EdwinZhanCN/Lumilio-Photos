// Package repocfg defines a single repository's own configuration: the
// .lumiliorepo file and the catalog column that mirrors it. [RepositoryConfig]
// carries identity and version; [LocalSettings] holds the per-repository
// behaviour (storage strategy, filename preservation, duplicate handling).
//
// It is dependency-free because the catalog stores it as a typed column; that
// sqlc mapping is the reviewed exception that lets the catalog import a
// storage type.
//
//atlas:group storage
package repocfg
