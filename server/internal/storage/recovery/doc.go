// Package recovery makes the private state a Repository carried into a new
// registration reachable again. An open, copy, or adoption leaves a
// format-1 receipt under .lumilio/recovery/ that lists what it found by
// class; this package reads those receipts back and acts on them.
//
// [Service.Reconcile] runs after a registration's scan and on every Trash
// maintenance pass: it adopts the inherited Trash through
// [server/internal/storage/trash.Service.Rebuild], which holds every adopted
// file out of automatic expiry, and it reassociates Studio records whose
// source is proven by content. A record attaches only when exactly one Asset
// in the Repository is bound to a present file with the record's hash and
// size and no file at the record's source path carries different content;
// the record is rewritten under the Asset's name and never guessed from a
// filename. An ambiguous or newer record stays where it is and is listed for
// review or export. Reconciliation is idempotent; a retry after a crash
// finishes the step it was in without moving anything twice.
//
// [Service.Inventory] is the recovery inventory the Storage page shows: the
// receipts, every listed item with its current disposition and presence, and
// the held Trash counts. [Service.OpenItem] exports an item's bytes and
// [Service.DeleteItem] removes one explicitly; both are audited. Trash holds
// are released through [server/internal/storage/trash.Service.AcceptRetention].
//
//atlas:group storage
package recovery
