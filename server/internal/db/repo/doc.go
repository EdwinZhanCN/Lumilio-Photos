// Package repo is the sqlc-generated query layer over the catalog baseline
// (server/migrations/000001_storage_baseline.up.sql), plus a few hand-written
// helpers for query plans and model extensions.
//
// [Queries] is the only typed access to catalog tables; [Querier] is its
// interface. Generated files are produced by `task server:sqlc` from
// internal/db/repo/queries and must not be edited by hand. Column types that
// belong to other owners are mapped by sqlc overrides (for example the
// repository configuration column uses
// [server/internal/storage/repocfg.RepositoryConfig]).
//
//atlas:group catalog
package repo
