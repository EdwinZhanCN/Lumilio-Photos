// Package bleveocr maintains the rebuildable Bleve full-text index over OCR
// text, stored beside the catalog at [PathForDatabase].
//
// SQLite OCR rows are authoritative. Catalog triggers append revisions to an
// outbox; [OutboxTrigger] coalesces wake-ups and [Writer] drains the outbox in
// bounded batches into the [Index], so a missed notification or restart only
// delays the index and never corrupts it. English and Chinese text are indexed
// separately ([SplitText]).
//
//atlas:group domain
package bleveocr
