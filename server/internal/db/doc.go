// Package db owns the catalog and queue SQLite runtimes.
//
// [Open] opens the catalog as a [DB]: go-sqlite3 is the only driver, the
// writer pool is fixed at one connection for product facts and desired/applied
// work state, and four mode=ro, query_only WAL connections serve foreground
// reads. Every physical connection gets one fixed pragma policy (read back at
// startup, failing closed on mismatch) and the statically linked Vec1
// extension. Generated read-only statements route to the reader pool and
// mutations to the writer; an explicit transaction stays pinned to the
// connection that began it. Never add a second catalog writer or use
// busy_timeout as writer admission.
//
// [OpenQueue] opens [QueueDB], River's separate execution-state file with its
// own one-writer/four-reader pools; it is disposable and
// [OpenQueueWithRecovery] may recreate it, after which the scheduler
// re-derives every lagging desired version from the catalog. There is
// deliberately no catalog-to-QueueDB cutover journal.
//
// WAL auto-checkpointing is disabled on every connection so no foreground
// commit pays for a checkpoint. A runtime monitor watches writer waits and WAL
// size and asks each database's sole writer for an explicit
// [DB.PassiveCheckpoint] past a bounded threshold. Online Backup reads use a
// reader connection, never the writer.
//
// [DB.MigrateCatalog] applies the single baseline and then each numbered step
// on an empty catalog. PRAGMA application_id "LUMC" identifies the catalog
// before any version is read (a pre-release "LUMI" catalog is rejected), and
// PRAGMA user_version is the only version discriminator; a newer version is
// rejected as written by a newer build. On an older catalog it first takes a
// protected pre-upgrade snapshot (see [server/internal/db/backup]), then
// applies each step in its own transaction; a failed step rolls back, leaves
// the last completed version, and stops startup naming the step and the
// snapshot.
//
// A running catalog must never be opened or copied through a host/container
// mount by another SQLite process: VFS locking across that boundary is not
// supported. Use the application's Online Backup, or inspect the catalog only
// after a graceful stop ([InspectCatalog]). Named transactions and admission
// metrics live in [server/internal/db/catalogtx]; generated queries in
// [server/internal/db/repo].
//
//atlas:group catalog
package db
