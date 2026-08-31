package queue

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"server/internal/commit"
	"server/internal/queue/jobs"
)

// MacroErrorHandler transfers only final macro failures into catalog truth.
// Intermediate errors retain River's bounded retry policy. The coordinator is
// installed during composition before River is started.
type MacroErrorHandler struct {
	mu      sync.RWMutex
	commits *commit.Coordinator
}

func NewMacroErrorHandler() *MacroErrorHandler { return &MacroErrorHandler{} }

func (h *MacroErrorHandler) SetCoordinator(commits *commit.Coordinator) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.commits = commits
	h.mu.Unlock()
}

func (h *MacroErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, _ error) *river.ErrorHandlerResult {
	h.commitTerminal(ctx, job, "attempts_exhausted")
	return nil
}

func (h *MacroErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, _ any, _ string) *river.ErrorHandlerResult {
	h.commitTerminal(ctx, job, "panic")
	return nil
}

func (h *MacroErrorHandler) commitTerminal(ctx context.Context, job *rivertype.JobRow, code string) {
	if h == nil || job == nil || job.MaxAttempts < 1 || job.Attempt < job.MaxAttempts {
		return
	}
	intent, ok := macroTerminalIntent(job, code)
	if !ok {
		return
	}
	h.mu.RLock()
	commits := h.commits
	h.mu.RUnlock()
	if commits == nil {
		return
	}
	commitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _ = commits.Submit(commitCtx, intent)
}

// macroTerminalIntent decodes the closed macro catalog and produces the
// corresponding typed terminal state mutation. Unknown or malformed jobs stay
// operational-only failures rather than writing ambiguous product state.
func macroTerminalIntent(job *rivertype.JobRow, code string) (commit.Intent, bool) {
	if job == nil || code == "" || job.Attempt < job.MaxAttempts || job.MaxAttempts < 1 {
		return commit.Intent{}, false
	}
	switch job.Kind {
	case "ingest_asset":
		var args jobs.IngestAssetArgs
		if json.Unmarshal(job.EncodedArgs, &args) != nil || args.CommitID == uuid.Nil || args.ReceiptID == uuid.Nil {
			return commit.Intent{}, false
		}
		return commit.Intent{Key: commit.Key{Family: commit.FamilyIngestReceipt, Subject: args.ReceiptID.String(), Fence: args.CommitID.String(), Stage: "ingest", DesiredVersion: 1}, Payload: commit.IngestReceiptApplied{ReceiptID: args.ReceiptID, TerminalError: code}}, true
	case "analyze_asset", "generate_asset_derivatives", "transcode_media", "enrich_asset":
		var args struct {
			AssetID         uuid.UUID `json:"assetId"`
			SourceFence     uuid.UUID `json:"sourceFence"`
			DesiredVersion  uint64    `json:"desiredVersion"`
			PipelineVersion string    `json:"pipelineVersion"`
		}
		if json.Unmarshal(job.EncodedArgs, &args) != nil || args.AssetID == uuid.Nil || args.SourceFence == uuid.Nil || args.DesiredVersion == 0 || args.PipelineVersion == "" {
			return commit.Intent{}, false
		}
		stage := map[string]string{"analyze_asset": "analyze", "generate_asset_derivatives": "derivatives", "transcode_media": "transcode", "enrich_asset": "enrich"}[job.Kind]
		return commit.Intent{Key: commit.Key{Family: commit.FamilyAssetStage, Subject: args.AssetID.String(), Fence: args.SourceFence.String(), Stage: stage, DesiredVersion: args.DesiredVersion}, Payload: commit.AssetStageApplied{AssetID: args.AssetID, SourceFence: args.SourceFence, Stage: stage, PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion, TerminalError: code}}, true
	case "scan_repository_batch":
		var args jobs.ScanRepositoryBatchArgs
		if json.Unmarshal(job.EncodedArgs, &args) != nil || args.RepositoryID == uuid.Nil || args.RequestedEpoch == 0 || args.DesiredVersion != args.RequestedEpoch {
			return commit.Intent{}, false
		}
		return commit.Intent{Key: commit.Key{Family: commit.FamilyRepositoryEpoch, Subject: args.RepositoryID.String(), Fence: uint64String(args.RequestedEpoch), Stage: "repository_scan", DesiredVersion: args.DesiredVersion}, Payload: commit.RepositoryEpochApplied{RepositoryID: args.RepositoryID, RequestedEpoch: args.RequestedEpoch, TerminalError: code}}, true
	case "rebuild_projection_batch":
		var args jobs.RebuildProjectionBatchArgs
		if json.Unmarshal(job.EncodedArgs, &args) != nil || args.ProjectionKind == "" || args.Scope == "" || args.SourceRevision == 0 || args.ProjectionVersion == 0 {
			return commit.Intent{}, false
		}
		return commit.Intent{Key: commit.Key{Family: commit.FamilyProjectionTerminal, Subject: args.Scope, Fence: uint64String(args.SourceRevision), Stage: args.ProjectionKind, DesiredVersion: args.ProjectionVersion}, Payload: commit.ProjectionTerminalFailure{Kind: args.ProjectionKind, Scope: args.Scope, SourceRevision: args.SourceRevision, ProjectionVersion: args.ProjectionVersion, TerminalError: code}}, true
	case "backup_catalog":
		var args jobs.BackupCatalogArgs
		if json.Unmarshal(job.EncodedArgs, &args) != nil || args.RequestID == uuid.Nil {
			return commit.Intent{}, false
		}
		return commit.Intent{Key: commit.Key{Family: commit.FamilyOperationReceipt, Subject: args.RequestID.String(), Fence: args.RequestID.String(), Stage: "backup", DesiredVersion: 1}, Payload: commit.OperationReceiptApplied{ReceiptID: args.RequestID, Kind: "backup", TerminalError: code}}, true
	default:
		return commit.Intent{}, false
	}
}

func uint64String(value uint64) string {
	// strconv.FormatUint is intentionally kept at this narrow adapter edge so
	// catalog package types do not leak into River payload handling.
	return strconv.FormatUint(value, 10)
}
