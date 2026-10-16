// Package jobs is the closed River macro-job catalog. [RuntimeJobCatalog]
// lists every job kind River may carry and [NewArgs] rebuilds typed arguments
// from a kind, so an unknown payload can never enter an implicit default queue
// after a QueueDB rebuild.
//
// Macro jobs ([IngestAssetArgs], [AnalyzeAssetArgs],
// [GenerateAssetDerivativesArgs], [TranscodeMediaArgs], [EnrichAssetArgs],
// [ScanRepositoryBatchArgs], [RebuildProjectionBatchArgs],
// [BackupCatalogArgs]) carry stable identities and expected revisions only.
// Product truth stays in the catalog's desired/applied records; River is
// disposable delivery state.
//
//atlas:group work
package jobs
