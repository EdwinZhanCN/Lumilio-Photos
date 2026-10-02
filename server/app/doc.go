// Package app is the only Server runtime: [Run] wires configuration, logging,
// libvips, the catalog and QueueDB, migrations, settings, repository storage,
// the River client and workers, ML services, processors, handlers, and the
// router, then serves until the context is cancelled.
//
// It is invoked by the CLI host (server/cmd) and imported in-process by the
// Desktop App, so it owns its full lifecycle — startup and graceful shutdown —
// without calling os.Exit; fatal startup conditions are returned as errors.
// Single-run host controls (pprof address, agent audit log, agent ref memory
// budgets, break-glass recovery) arrive separately from configuration in
// [OperatorControls].
//
// Startup order is: logging; libvips; the single-writer catalog and the
// independent QueueDB; catalog and River migrations in their own files; the
// generated query layer; settings, repository storage, queues, ML, processors,
// handlers, and router; finally the listeners for server.listen and
// server.tls.mode. Missing, corrupt, or post-restore OCR indexes are rebuilt
// before HTTP starts.
//
// The optional pprof listener belongs to the outer Run and stays stable across
// in-process database restore generations. The latest bounded SQLite
// telemetry, pool statistics, WAL state, and checkpoint result are published
// as an owner-only sqlite-runtime.json under logging.dir for host-side
// sampling; there is deliberately no HTTP debug API over private data.
//
//atlas:group runtime
package app
