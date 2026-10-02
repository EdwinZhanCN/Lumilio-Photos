// Package scan is the repository scan index (#222). The catalog mirrors each
// repository's tree in repository_entries, one row per file or directory;
// the disk is the truth and a full scan is the authority.
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
//atlas:group storage
package scan
