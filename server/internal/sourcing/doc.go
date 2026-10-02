// Package sourcing materializes staged uploads and cloud imports into a
// repository without losing them to a crash.
//
// A source is first recorded as a repository staging commit (see
// [StagingJournal]). [SourceMaterializer.MaterializeCommit] then claims the
// commit, re-verifies the staged bytes against their recorded identity, moves
// the file into the repository's inbox without overwriting anything, and binds
// it as known content through the scan index, which creates or reactivates
// the Asset and requests its pipeline stages. Every step is journaled, so a
// retry after a crash before or after the filesystem move converges on the
// same Asset; unrecoverable sources are quarantined, never deleted.
//
// Materialization owns the staging file. A commit error is always returned to
// River or the caller, and a failed quarantine never deletes the source. An
// existing target or an instant-upload duplicate needs exact size plus BLAKE3
// verification before staging is removed; conflicts keep both files with a
// recoverable ingest phase. HTTP and cloud callers must not add their own
// error cleanup around this boundary.
//
//atlas:group ingest
package sourcing
