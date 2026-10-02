// Package backup creates, validates, retains, and restores consistent SQLite
// catalog snapshots. It never copies a live WAL database directly: snapshots
// are taken through Online Backup on a reader connection ([CreateSnapshot])
// and pruned by retention ([Prune]), which never removes a protected
// [PreUpgradePrefix] snapshot taken before a schema upgrade. The backup
// manifest format starts at version 1.
//
// Restore is a generation boundary, not a live-handle operation. Its journal
// ([RestoreOperationStatus]) moves through staged, previous preserved, active
// installed, verified, and completed, with matching durable rollback phases
// ([ApplyPendingRestore], [RollbackPendingRestore]); startup reconciles the
// marker with active, staged, previous, and failed files after an interrupted
// rename. A snapshot at an older supported schema or config version is
// installed unchanged and upgraded by the next start; a newer one is rejected.
//
//atlas:group catalog
package backup
