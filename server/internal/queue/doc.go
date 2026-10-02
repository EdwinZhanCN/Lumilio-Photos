// Package queue adapts River to Lumilio's catalog-owned work model.
//
// [Scheduler] is the bounded pass that derives disposable River macro jobs
// from catalog desired/applied state; [SchedulerWake] coalesces commit hints
// so new work is noticed promptly while the periodic pass remains the recovery
// path. [New] builds the River client over QueueDB, and the macro workers
// ([IngestMacroWorker], [AnalyzeAssetWorker],
// [GenerateAssetDerivativesWorker], [TranscodeMediaWorker],
// [EnrichAssetWorker], [ScanRepositoryBatchWorker],
// [RebuildProjectionBatchWorker], [BackupCatalogWorker]) run processors and
// hand results to the commit coordinator. [EnrichmentRunner] executes ML
// enrichment steps against Lumen.
//
// River never decides whether product work exists; a lost QueueDB only delays
// work until the next scheduler pass.
//
//atlas:group ingest
package queue
