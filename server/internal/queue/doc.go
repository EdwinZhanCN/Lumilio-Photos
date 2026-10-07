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
// River has one catalog_macro queue and exactly eight durable kinds (see
// [server/internal/queue/jobs.RuntimeJobCatalog]). Request QoS stays in the
// catalog and is emitted as River priority, never as job arguments. River
// uniqueness is by immutable macro arguments, never a time window; discarded
// deliveries do not keep uniqueness, so a still-runnable catalog generation
// starts a fresh bounded delivery. asset_pipeline_failures is catalog product
// state fenced by source content, pipeline version, and desired version: River
// delivery attempts never decide retry or terminal outcomes.
//
//atlas:group ingest
package queue
