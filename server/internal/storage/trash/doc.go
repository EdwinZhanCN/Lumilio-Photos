// Package trash moves deleted Assets' files into their repository's trash
// and back. Delete and Restore journal every move through
// lifecycle_operations (prepared, filesystem_applied, catalog_committed) so a
// crash between a move and its catalog commit is reconciled on startup, and
// each outcome is recorded in lifecycle_audit_events. No file I/O happens
// inside a catalog transaction.
//
//atlas:group storage
package trash
