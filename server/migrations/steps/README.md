# Catalog forward steps

`000001_storage_baseline.up.sql` is catalog schema version 1, the
`v26.1.0-rc.1` compatibility baseline. After the rc.1 tag the baseline and
every step in this directory are frozen: a released file is never edited,
renamed, or deleted, because users' catalogs have already run it.

A schema change is a new step:

1. Add `NNNN_<name>.sql` here, where `NNNN` is the version the step produces
   (`0002` is the first). Versions run 2..`SchemaVersion` with no gaps.
2. Bump `SchemaVersion` in `server/internal/db/migration.go` to `NNNN`.
3. Do not set `PRAGMA user_version`: the runner stamps it in the same
   transaction as the step. `PRAGMA foreign_keys` cannot change inside a
   transaction, so a table rebuild must keep foreign keys valid on its own.
4. Derived data (thumbnails, transcodes, vectors, the OCR index) is never
   transformed; drop it and let the pipeline re-derive it.
5. Run `task server:sqlc` if queries changed, `task dto` if an API changed,
   then `task server:test`.

Fresh installs run the baseline and then every step, so a step runs on both
empty and populated catalogs. Before upgrading an existing catalog, the server
takes a protected `pre-upgrade-` backup. Decision:
[.agents/decisions/2026-09-24-rc-compatibility-baseline.md](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md).
