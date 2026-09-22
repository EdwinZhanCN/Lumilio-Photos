// Package processing is the administrator read model for background work:
// one closed catalog of user-facing stages, each counted in one declared unit
// from Catalog desired/applied facts, with River contributing only the
// execution facts (running and retryable deliveries).
package processing

import "server/internal/pipeline"

// StageID is a closed, user-facing processing stage identity.
type StageID string

const (
	StageImport     StageID = "import"
	StageScan       StageID = "scan"
	StageMetadata   StageID = "metadata"
	StageThumbnails StageID = "thumbnails"
	StageVideo      StageID = "video"
	StageAnalysis   StageID = "analysis"
	StageEvents     StageID = "events"
	StagePlaces     StageID = "places"
	StageTextSearch StageID = "text_search"
	StageBackup     StageID = "backup"
)

// Group separates per-file media work from whole-catalog rebuilds and backup.
type Group string

const (
	GroupMedia   Group = "media"
	GroupCatalog Group = "catalog"
)

// Unit names what one count on a stage card means.
type Unit string

const (
	UnitFiles        Unit = "files"
	UnitRepositories Unit = "repositories"
	UnitUpdates      Unit = "updates"
	UnitRuns         Unit = "runs"
)

// Status is derived from a stage's counts; the first matching rule wins.
type Status string

const (
	StatusAttention Status = "attention"
	StatusWorking   Status = "working"
	StatusRetrying  Status = "retrying"
	StatusWaiting   Status = "waiting"
	StatusIdle      Status = "idle"
)

// deliveryMatch identifies the River deliveries that execute a stage.
// ProjectionKind is empty for non-projection macro kinds.
type deliveryMatch struct {
	Kind           string
	ProjectionKind string
}

// StageSpec is one entry of the closed stage catalog.
type StageSpec struct {
	ID    StageID
	Group Group
	Unit  Unit
	// AssetStage is set for per-file pipeline stages.
	AssetStage pipeline.Stage
	// Retryable reports whether a whole-stage retry exists.
	Retryable  bool
	deliveries []deliveryMatch
}

// Stages is the ordered catalog. Order is presentation order.
var Stages = []StageSpec{
	{ID: StageImport, Group: GroupMedia, Unit: UnitFiles, deliveries: []deliveryMatch{{Kind: "ingest_asset"}}},
	{ID: StageScan, Group: GroupMedia, Unit: UnitRepositories, deliveries: []deliveryMatch{{Kind: "scan_repository_batch"}}},
	{ID: StageMetadata, Group: GroupMedia, Unit: UnitFiles, AssetStage: pipeline.StageAnalyze, Retryable: true, deliveries: []deliveryMatch{{Kind: "analyze_asset"}}},
	{ID: StageThumbnails, Group: GroupMedia, Unit: UnitFiles, AssetStage: pipeline.StageDerivatives, Retryable: true, deliveries: []deliveryMatch{{Kind: "generate_asset_derivatives"}}},
	{ID: StageVideo, Group: GroupMedia, Unit: UnitFiles, AssetStage: pipeline.StageTranscode, Retryable: true, deliveries: []deliveryMatch{{Kind: "transcode_media"}}},
	{ID: StageAnalysis, Group: GroupMedia, Unit: UnitFiles, AssetStage: pipeline.StageEnrich, Retryable: true, deliveries: []deliveryMatch{{Kind: "enrich_asset"}, {Kind: "rebuild_projection_batch", ProjectionKind: "asset_reindex"}}},
	{ID: StageEvents, Group: GroupCatalog, Unit: UnitUpdates, Retryable: true, deliveries: []deliveryMatch{{Kind: "rebuild_projection_batch", ProjectionKind: "event"}}},
	{ID: StagePlaces, Group: GroupCatalog, Unit: UnitUpdates, Retryable: true, deliveries: []deliveryMatch{{Kind: "rebuild_projection_batch", ProjectionKind: "location"}, {Kind: "rebuild_projection_batch", ProjectionKind: "location_resolution"}}},
	{ID: StageTextSearch, Group: GroupCatalog, Unit: UnitUpdates, Retryable: true, deliveries: []deliveryMatch{{Kind: "rebuild_projection_batch", ProjectionKind: "ocr"}}},
	{ID: StageBackup, Group: GroupCatalog, Unit: UnitRuns, deliveries: []deliveryMatch{{Kind: "backup_catalog"}}},
}

// LookupStage returns the catalog entry for id.
func LookupStage(id StageID) (StageSpec, bool) {
	for _, spec := range Stages {
		if spec.ID == id {
			return spec, true
		}
	}
	return StageSpec{}, false
}

// stageForDelivery maps one River delivery back to its stage.
func stageForDelivery(kind, projectionKind string) (StageID, bool) {
	for _, spec := range Stages {
		for _, match := range spec.deliveries {
			if match.Kind == kind && match.ProjectionKind == projectionKind {
				return spec.ID, true
			}
		}
	}
	return "", false
}
