// Package db owns the catalog and queue SQLite runtimes.
//
// [Open] opens the catalog as a [DB]: exactly one physical writer for product
// facts and durable work intent, plus a bounded query-only WAL reader pool for
// foreground reads. Every connection gets one verified pragma policy and the
// statically linked Vec1 extension. [OpenQueue] opens [QueueDB], River's
// separate execution-state file, which may be recreated without touching the
// catalog ([OpenQueueWithRecovery]).
//
// Automatic WAL checkpoints are disabled; the runtime monitor asks each
// database's sole writer for explicit passive checkpoints. [InspectCatalog]
// reads catalog identity and version without opening a runtime. Named
// transactions and admission metrics live in [server/internal/db/catalogtx];
// generated queries live in [server/internal/db/repo].
//
//atlas:group catalog
package db
