// Package catalogtx is the closed application transaction capability for the
// catalog. Every runtime transaction has a compile-time operation name and
// role (writer or reader); the observed connector also names standalone writer
// statements and returned-row lifetimes.
//
// Bounded HDR histograms record admission, body, commit, total, cancellation,
// outcome, and cursor lifetime without retaining SQL text, arguments, or
// entity IDs, and slow named transactions are logged against the write budget.
// `task architecture:check` rejects raw production Begin, BeginTx, and
// standalone writer Exec calls outside the migration and driver boundary.
//
// A write transaction contains only bounded SQL and in-memory validation.
// Multi-statement planning that needs one snapshot uses a short reader
// transaction and closes all rows before CPU, filesystem, media, network,
// serialization, sleep, or River work. Large atomic membership changes are
// set-based; restartable derived projections publish in bounded,
// revision-checked turns instead of monopolizing the writer.
//
//atlas:group catalog
package catalogtx
