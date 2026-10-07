// Package trash moves deleted Assets' files into their repository's trash
// and back. Delete and Restore journal every move through
// lifecycle_operations (prepared, filesystem_applied, catalog_committed) so a
// crash between a move and its catalog commit is reconciled on startup, and
// each outcome is recorded in lifecycle_audit_events. No file I/O happens
// inside a catalog transaction.
//
// [Service.Delete] preflights the whole selection: offline repositories,
// changed stat tuples or pending hashes, and Assets with neither present nor
// trashed files refuse the request before any move. Already trashed selections
// are idempotent. Same-volume moves never overwrite; failed moves attempt
// rollback. Files live at .lumilio/trash/files/<trash_id>/<name>, with format-1
// JSON sidecars at .lumilio/trash/info/<trash_id>.json.
// [Service.Restore] keeps Asset identity, metadata, and album membership;
// occupied paths get name (restored).ext, then numbered free siblings. A new
// present copy can also reactivate a trashed Asset.
//
// [Service.Recover] reconciles interrupted journals. [Service.RebuildAll] adopts
// sidecar-only files through [server/internal/storage/scan.Scanner.AdoptTrash],
// binding to the repository owner's existing content Asset or a new trashed
// Asset. Rebuild waits for open trash/restore journals; unreadable or newer
// sidecar formats are reported and left alone.
//
// [Service.Expire], [Service.DeletePermanently], and [Service.Empty] unlink
// trashed files before purging entries; an already absent file is safe to retry.
// [Service.RemoveMissing] purges missing entries without touching files. Purges
// are audited in batches of 256 through [server/internal/lifecycle.PurgeEntriesTx];
// Assets and metadata survive while any entry remains. Offline repositories
// retain their Trash until available for expiry.
//
//atlas:group storage
package trash
