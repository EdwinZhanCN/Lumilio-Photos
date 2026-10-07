// Package commit is the sole catalog-write capability for asynchronous work.
//
// Background stages compute outside any transaction and then hand an
// immutable result to the [Coordinator] through a typed method such as
// [Coordinator.ApplyIngestReceipt], [Coordinator.ApplyAssetMetadata],
// [Coordinator.ApplyAssetDerivatives], or [Coordinator.ApplyEnrichment]. The
// coordinator batches submissions from every worker into one bounded writer
// transaction, checks each result's fence (source content and desired
// version), and answers with an [Outcome]: applied, stale (the catalog moved
// on and the result is discarded), or duplicate.
//
// Because River workers never write the catalog themselves, a retried or
// duplicated delivery can only produce a stale or duplicate outcome, and
// writer admission stays bounded no matter how many workers run.
//
//atlas:group ingest
package commit
