// Package app is the only Server runtime: [Run] wires configuration, logging,
// libvips, the catalog and QueueDB, migrations, settings, repository storage,
// the River client and workers, ML services, processors, handlers, and the
// router, then serves until the context is cancelled.
//
// It is invoked by the CLI host (server/cmd) and imported in-process by the
// Desktop App, so it owns its full lifecycle — startup and graceful shutdown —
// without calling os.Exit; fatal startup conditions are returned as errors.
// Single-run host controls (pprof address, agent audit log, agent ref memory
// budgets, break-glass recovery, an optional change-watch backend) arrive
// separately from configuration in [OperatorControls].
//
// Startup order is: logging; libvips; the single-writer catalog and the
// independent QueueDB; catalog and River migrations in their own files; the
// generated query layer; settings, repository storage, queues, ML, processors,
// handlers, and router; finally the listeners for server.listen and
// server.tls.mode. Missing, corrupt, or post-restore OCR indexes are rebuilt
// before HTTP starts.
//
// Startup is catalog-first: a usable catalog whose configured Default Storage
// Location is offline or carries a marker this build cannot adopt serves in a
// restricted recovery runtime (setup and the Storage page name the way out;
// no marker is provisioned or replaced), while a pre-release or newer catalog
// or manifest, a configured path that claims to be a move of an established
// default without proving it, or a first run that cannot create its default
// at all are fatal. [ClassifyStartupFailure] and [DescribeStartupFailure] are
// the Server-owned facade both hosts present: the CLI prints the code and the
// operator steps, Desktop maps the code to its recovery cause.
//
// The optional pprof listener belongs to the outer Run and stays stable across
// in-process database restore generations. The latest bounded SQLite
// telemetry, pool statistics, WAL state, and checkpoint result are published
// as an owner-only sqlite-runtime.json under logging.dir for host-side
// sampling; there is deliberately no HTTP debug API over private data.
//
// runTrashMaintenanceLoop runs at startup and hourly: retry deferred move
// recovery, run private recovery (rebuild Trash from sidecars under a hold,
// reassociate proven Studio records), then expire unheld files using the
// manifest's required repository_trash.retention_days. Repository verification runs at
// startup and periodically with 3/4–5/4 interval jitter; watcher hints never
// replace authoritative full scans.
//
//atlas:group runtime
package app
