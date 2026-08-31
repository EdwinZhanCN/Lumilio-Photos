package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/domainoutbox"
	"server/internal/pipeline"
	"server/internal/queue/jobs"
)

type DomainAdapter struct{ client *river.Client[*sql.Tx] }

func NewDomainAdapter(client *river.Client[*sql.Tx]) *DomainAdapter {
	return &DomainAdapter{client: client}
}
func (a *DomainAdapter) InsertMany(ctx context.Context, entries []domainoutbox.Entry) error {
	if a == nil || a.client == nil {
		return errors.New("domain adapter is not configured")
	}
	if len(entries) == 0 {
		return nil
	}
	params := make([]river.InsertManyParams, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == "" || entry.Kind == "" || entry.SubjectKey == "" || entry.DesiredVersion == 0 {
			return errors.New("domain outbox entry is missing its identity")
		}
		if entry.Kind != entry.Envelope.Kind {
			return fmt.Errorf("domain outbox %s kind %q does not match envelope kind %q", entry.ID, entry.Kind, entry.Envelope.Kind)
		}
		args, err := macroArgs(entry.Envelope)
		if err != nil {
			return fmt.Errorf("map domain outbox %s: %w", entry.ID, err)
		}
		subject, desired, err := macroJobIdentity(args)
		if err != nil {
			return fmt.Errorf("validate domain outbox %s: %w", entry.ID, err)
		}
		if entry.SubjectKey != subject || entry.DesiredVersion != desired {
			return fmt.Errorf("domain outbox %s identity (%q,%d) does not match macro command (%q,%d)", entry.ID, entry.SubjectKey, entry.DesiredVersion, subject, desired)
		}
		params = append(params, river.InsertManyParams{Args: args})
	}
	_, err := a.client.InsertMany(ctx, params)
	return err
}

// macroJobIdentity returns the catalog identity that River uniqueness must use
// for a mapped macro. The outbox row carries the same identity outside the
// JSON payload; validating both copies at this seam prevents a malformed or
// hand-edited row from being admitted under a different deduplication key.
func macroJobIdentity(args river.JobArgs) (string, uint64, error) {
	switch value := args.(type) {
	case jobs.IngestAssetArgs:
		if value.CommitID == uuid.Nil || value.ReceiptID == uuid.Nil {
			return "", 0, errors.New("ingest macro has an invalid identity")
		}
		return value.CommitID.String(), 1, nil
	case jobs.AnalyzeAssetArgs:
		return value.AssetID.String(), value.DesiredVersion, nil
	case jobs.GenerateAssetDerivativesArgs:
		return value.AssetID.String(), value.DesiredVersion, nil
	case jobs.TranscodeMediaArgs:
		return value.AssetID.String(), value.DesiredVersion, nil
	case jobs.EnrichAssetArgs:
		return value.AssetID.String(), value.DesiredVersion, nil
	case jobs.ScanRepositoryBatchArgs:
		return value.RepositoryID.String(), value.DesiredVersion, nil
	case jobs.RebuildProjectionBatchArgs:
		return value.Scope, value.ProjectionVersion, nil
	case jobs.BackupCatalogArgs:
		return value.RequestID.String(), 1, nil
	default:
		return "", 0, fmt.Errorf("unsupported macro args type %T", args)
	}
}

func macroArgs(envelope pipeline.Envelope) (river.JobArgs, error) {
	if envelope.Version != pipeline.EnvelopeVersion || envelope.Kind == "" || envelope.TraceID == uuid.Nil || envelope.CorrelationID == uuid.Nil || len(envelope.Payload) == 0 {
		return nil, errors.New("invalid domain envelope")
	}
	switch envelope.Kind {
	case "ingest_asset":
		var command pipeline.IngestCommand
		if err := json.Unmarshal(envelope.Payload, &command); err != nil {
			return nil, err
		}
		if command.CommitID == uuid.Nil || command.ReceiptID == uuid.Nil || !command.Admission.Valid() {
			return nil, errors.New("ingest command has an invalid receipt fence")
		}
		return jobs.IngestAssetArgs{CommitID: command.CommitID, ReceiptID: command.ReceiptID, Admission: string(command.Admission)}, nil
	case "asset.analyze", "asset.derivatives", "asset.transcode", "asset.enrich":
		var command pipeline.AssetCommand
		if err := json.Unmarshal(envelope.Payload, &command); err != nil {
			return nil, err
		}
		if command.AssetID == uuid.Nil || command.Fence.UUID() == uuid.Nil || command.DesiredVersion == 0 || command.PipelineVersion == "" || !command.Admission.Valid() {
			return nil, errors.New("asset command has an invalid catalog fence")
		}
		fence := command.Fence.UUID()
		switch command.Stage {
		case pipeline.StageAnalyze:
			if envelope.Kind != "asset.analyze" {
				return nil, errors.New("asset command stage does not match envelope kind")
			}
			return jobs.AnalyzeAssetArgs{AssetID: command.AssetID, SourceFence: fence, DesiredVersion: command.DesiredVersion, PipelineVersion: command.PipelineVersion, Admission: string(command.Admission)}, nil
		case pipeline.StageDerivatives:
			if envelope.Kind != "asset.derivatives" {
				return nil, errors.New("asset command stage does not match envelope kind")
			}
			return jobs.GenerateAssetDerivativesArgs{AssetID: command.AssetID, SourceFence: fence, DesiredVersion: command.DesiredVersion, PipelineVersion: command.PipelineVersion, Admission: string(command.Admission)}, nil
		case pipeline.StageTranscode:
			if envelope.Kind != "asset.transcode" {
				return nil, errors.New("asset command stage does not match envelope kind")
			}
			return jobs.TranscodeMediaArgs{AssetID: command.AssetID, SourceFence: fence, DesiredVersion: command.DesiredVersion, PipelineVersion: command.PipelineVersion, Admission: string(command.Admission)}, nil
		case pipeline.StageEnrich:
			if envelope.Kind != "asset.enrich" {
				return nil, errors.New("asset command stage does not match envelope kind")
			}
			return jobs.EnrichAssetArgs{AssetID: command.AssetID, SourceFence: fence, DesiredVersion: command.DesiredVersion, PipelineVersion: command.PipelineVersion, Admission: string(command.Admission)}, nil
		}
		return nil, fmt.Errorf("unsupported asset stage %q", command.Stage)
	case "repository.scan":
		var command pipeline.RepositoryCommand
		if err := json.Unmarshal(envelope.Payload, &command); err != nil {
			return nil, err
		}
		if command.RepositoryID == uuid.Nil || command.RequestedEpoch == 0 || command.DesiredVersion == 0 || command.RequestedEpoch != command.DesiredVersion || !command.Admission.Valid() {
			return nil, errors.New("repository command has an invalid observation fence")
		}
		return jobs.ScanRepositoryBatchArgs{RepositoryID: command.RepositoryID, RequestedEpoch: command.RequestedEpoch, DesiredVersion: command.DesiredVersion, Frontier: command.Frontier, Admission: string(command.Admission)}, nil
	case "projection.event", "projection.location", "projection.ocr":
		var command pipeline.ProjectionCommand
		if err := json.Unmarshal(envelope.Payload, &command); err != nil {
			return nil, err
		}
		if command.Scope == "" || command.SourceRevision == 0 || command.ProjectionVersion == 0 || !command.Admission.Valid() {
			return nil, errors.New("projection command has an invalid revision fence")
		}
		switch command.Kind {
		case "event":
			owner, err := strconv.ParseInt(command.Scope, 10, 32)
			if err != nil || owner <= 0 {
				return nil, errors.New("event projection scope is not a valid owner ID")
			}
		case "location":
			parts := strings.Split(command.Scope, ":")
			if len(parts) != 2 {
				return nil, errors.New("location projection scope is not repository:owner")
			}
			if id, err := uuid.Parse(parts[0]); err != nil || id == uuid.Nil {
				return nil, errors.New("location projection scope has an invalid repository ID")
			}
			if owner, err := strconv.ParseInt(parts[1], 10, 32); err != nil || owner <= 0 {
				return nil, errors.New("location projection scope has an invalid owner ID")
			}
		case "location_resolution", "ocr":
			if command.Scope != "all" {
				return nil, fmt.Errorf("%s projection scope must be all", command.Kind)
			}
		default:
			return nil, fmt.Errorf("unsupported projection kind %q", command.Kind)
		}
		expectedEnvelopeKind := "projection.location"
		if command.Kind == "event" {
			expectedEnvelopeKind = "projection.event"
		} else if command.Kind == "ocr" {
			expectedEnvelopeKind = "projection.ocr"
		}
		if envelope.Kind != expectedEnvelopeKind {
			return nil, fmt.Errorf("projection command kind %q does not match envelope kind %q", command.Kind, envelope.Kind)
		}
		return jobs.RebuildProjectionBatchArgs{ProjectionKind: command.Kind, Scope: command.Scope, SourceRevision: command.SourceRevision, ProjectionVersion: command.ProjectionVersion, Cursor: command.Cursor, Admission: string(command.Admission)}, nil
	case "projection.asset_reindex":
		var command pipeline.ReindexCommand
		if err := json.Unmarshal(envelope.Payload, &command); err != nil {
			return nil, err
		}
		if command.ReceiptID == uuid.Nil || command.RequestedRevision == 0 || command.Limit < 1 || command.Limit > 500 || len(command.Tasks) == 0 || !command.Admission.Valid() {
			return nil, errors.New("reindex command has an invalid revision fence")
		}
		if command.RepositoryID != nil && *command.RepositoryID == uuid.Nil {
			return nil, errors.New("reindex command has an invalid repository")
		}
		for _, task := range command.Tasks {
			if strings.TrimSpace(task) == "" {
				return nil, errors.New("reindex command has an empty task")
			}
		}
		return jobs.RebuildProjectionBatchArgs{ProjectionKind: "asset_reindex", Scope: command.ReceiptID.String(), SourceRevision: command.RequestedRevision, ProjectionVersion: command.RequestedRevision, Cursor: command.Cursor, Admission: string(command.Admission)}, nil
	case "backup_catalog":
		var command pipeline.BackupCommand
		if err := json.Unmarshal(envelope.Payload, &command); err != nil {
			return nil, err
		}
		if command.RequestID == uuid.Nil || !command.Admission.Valid() {
			return nil, errors.New("backup command has no request identity")
		}
		return jobs.BackupCatalogArgs{RequestID: command.RequestID, Force: command.Force, Admission: string(command.Admission)}, nil
	}
	return nil, fmt.Errorf("unsupported domain command kind %q", envelope.Kind)
}
