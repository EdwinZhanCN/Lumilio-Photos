// Package scan is the repository scan index (#222). The catalog mirrors each
// repository's tree in repository_entries, one row per file or directory;
// the disk is the truth and a full scan is the authority. Entries record
// repository-relative path, comparison path_key, parent_key, stat tuple,
// state, and revision; files bind content and Asset IDs.
//
// A scan is a depth-first walk in sorted name order. Each directory is one
// unit: list it completely, diff the listing against the catalog rows whose
// parent_key is that directory, write only what changed, and mark a
// catalog-only row missing only after a positive absence probe with the
// repository marker re-checked. Hashing is a separate bounded pass over
// pending_hash rows that commits by compare-and-swap on the row's revision.
//
// Nothing here performs filesystem I/O, hashing, or sleeping inside a catalog
// transaction; every write batch is at most MaxBatchRows rows and is sized
// to stay well inside the writer-hold budget.
//
// [Scanner.RunTurn] walks inbox/ but excludes .lumilio/, markers, and nested
// repositories. Unreadable directories preserve indexed children. Unchanged
// stat tuples skip hashing except within [RacyGranularity] (two seconds of
// mtime); settling files schedule a delayed follow-up. [Scanner.RequestScan]
// coalesces queued scopes; repository_scans persists resume paths, cancellation,
// and counters. Statuses are queued, walking, sweeping, completed, offline,
// failed, and cancelled; sweeping is not a repository-wide delete transaction.
// Cancellation takes effect at turn boundaries and keeps committed entries.
//
// [Scanner.HashTurn] uses stable-handle BLAKE3 with before/after observation
// checks and revision fencing. Walk completion does not imply hashing or Asset
// processing completion. [Scanner.BindKnownContent] binds upload/cloud content
// without rehashing and follows the same in-place rules as Scanner.commitHashTx:
// keep identity and metadata if no other present copy or target content Asset
// exists; fork and copy user metadata if another present copy remains; otherwise
// bind to the existing owner/content Asset without merging metadata, retaining
// the old Asset as missing when necessary. Forked descriptions copy only user
// edits. No branch leaves an Asset without an entry.
//
// [Watcher] uses syncthing/notify hints in 10-second batches; more than 512
// events, buffer overflow, or watcher startup failure requests a full scan,
// with backoff retries for startup failures. Full scans avoid unchanged writes
// and hashes, rather than relying on native journal cursors. Writer batches
// adapt toward 10 ms within the 25 ms hold budget through
// [server/internal/commit.ScanWriter], capped at [MaxBatchRows] (256). Scans
// never unlink, trash, or purge Assets; returning files can rebind their Asset.
//
//atlas:group storage
package scan
