# SQLite Merge-Blocker Hardening

Status: completed and verified on 2026-07-26.

## Shipped contract

- Staging materialization never reports success after a commit error. Failed
  quarantine preserves the source, and an existing inbox target is accepted
  only after exact size and BLAKE3 verification. Mismatches retain both files
  in a structured recoverable conflict state.
- Ingest recovery uses `status.ingest.phase/code`, not localized or
  user-readable message text. Repository-relative recovery paths are the only
  paths persisted in asset status.
- In-place materialization rejects absolute, volume-qualified, traversal, and
  symlink-escaping source paths at the final filesystem boundary.
- Restore uses a durable, idempotent journal:
  `staged → previous_preserved → active_installed → verified → completed`.
  Rollback likewise journals `rollback_started → failed_preserved →
  previous_restored`. Every file rename is directory-synced before the next
  marker state is written and synced.
- Runtime generations give HTTP drain, River drain, and River forced
  cancellation independent budgets. SQLite is not closed or swapped unless
  River is confirmed stopped; close failures also abort the generation
  restart. The optional pprof server belongs to the outer host lifecycle.
- Fixed SQLite policy is applied to every physical connection through
  go-sqlite3 DSN options plus a connection hook, then verified at startup.
- The fresh baseline accepts the shared `bio` album type used by indexing
  queries. Applied Lumilio migrations record SHA-256 checksums and historical
  SQL changes fail closed.

Primary owners: `server/internal/sourcing`, `server/internal/storage`,
`server/internal/db`, `server/internal/db/backup`, and `server/app`.

## Verification evidence

- `make server-test` passes.
- Restore fault injection covers crashes after staged copy, both forward
  renames, forward marker boundaries, both rollback renames, and rollback
  marker boundaries; the next startup converges without manual file repair.
- Staging regressions prove commit plus quarantine failure returns an error,
  preserves the unique source, and records a recoverable structured phase.
- Connection replacement uses `driver.ErrBadConn` and re-verifies
  `foreign_keys`, `busy_timeout`, `temp_store`, and `wal_autocheckpoint`.
- Path tests cover Unix/Windows rooted input, both separator styles for
  traversal, contained symlinks, and symlink escape.
- Schema/query literal audit and a real insert prove `bio` is a legal album
  domain value. Migration tampering is rejected on the next migrate call.
- `cd server && sqlc generate` was run after the baseline/query changes; the
  duplicate-content query now carries `storage_path` for physical verification.

## Remaining hardening

Long-running concurrent import/backup/restore stress, disk-full injection, and
the complete kill-at-every-instruction state space remain release-hardening
work. They are not substitutes for the deterministic crash boundaries covered
here.
