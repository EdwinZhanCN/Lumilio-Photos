package app

// This file contains the small pieces of composition that connect the closed
// River macro catalogue to the existing domain engines.  The macro workers
// own admission and acknowledgement; these functions only run one bounded
// unit and never enqueue another River job.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/commit"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/event"
	"server/internal/execution"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/queue/jobs"
	"server/internal/search/bleveocr"
	"server/internal/service"
	"server/internal/storage"
	roecontroller "server/internal/storage/roe/controller"
	"server/internal/storage/roe/materializer"
	"server/internal/workqos"
)

type repositoryScanReader interface {
	GetRepositoryObservationState(context.Context, uuid.UUID) (repo.RepositoryObservationState, error)
	GetRepositoryScanRun(context.Context, repo.GetRepositoryScanRunParams) (repo.RepositoryScanRun, error)
	ListRepositoryMaterializationCandidates(context.Context, repo.ListRepositoryMaterializationCandidatesParams) ([]repo.ListRepositoryMaterializationCandidatesRow, error)
}

// runRepositoryScanBatch advances one durable ROE turn and materializes the
// bounded hash candidates derived from catalog observations. The active run
// and requested epoch are read from catalog state, so a stale or duplicated
// macro is a successful no-op and never needs River state to decide whether
// work exists.
func runRepositoryScanBatch(
	ctx context.Context,
	controller *roecontroller.Controller,
	hasher *materializer.HashPreparer,
	reader repositoryScanReader,
	engine *execution.Engine,
	demand execution.DemandCatalog,
	commits *commit.Coordinator,
	qos workqos.Class,
	args jobs.ScanRepositoryBatchArgs,
) (bool, error) {
	if controller == nil || hasher == nil || reader == nil || engine == nil || commits == nil {
		return false, errors.New("repository scan runtime is not configured")
	}
	state, err := reader.GetRepositoryObservationState(ctx, args.RepositoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load repository observation state: %w", err)
	}
	if !state.ActiveRunID.Valid {
		return false, nil
	}
	run, err := reader.GetRepositoryScanRun(ctx, repo.GetRepositoryScanRunParams{
		RepositoryID: args.RepositoryID,
		RunID:        state.ActiveRunID.UUID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load active repository observation run: %w", err)
	}
	if !repositoryScanCommandCurrent(state, run, args) {
		return false, nil
	}
	var turn roecontroller.TurnResult
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return false, err
	}
	err = engine.Run(ctx, class, demand.Demand(execution.StepScanRepositoryTurn, execution.MediaUnknown), func(stepCtx context.Context) error {
		var stepErr error
		turn, stepErr = controller.RunTurn(stepCtx, args.RepositoryID, run.RunID)
		return stepErr
	})
	if err != nil {
		return false, err
	}
	more := turn.HasMore
	candidates, err := reader.ListRepositoryMaterializationCandidates(ctx, repo.ListRepositoryMaterializationCandidatesParams{
		RepositoryID: args.RepositoryID,
		Limit:        32,
	})
	if err != nil {
		return false, fmt.Errorf("list repository hash candidates: %w", err)
	}
	for _, candidate := range candidates {
		err := engine.Run(ctx, class, demand.Demand(execution.StepScanRepositoryHash, execution.MediaUnknown), func(stepCtx context.Context) error {
			prepared, prepareErr := hasher.PrepareHash(stepCtx, candidate.NodeID, candidate.ObservationRevision)
			if prepareErr != nil || prepared == nil {
				return prepareErr
			}
			_, submitErr := commits.ApplyRepositoryHash(stepCtx, commit.RepositoryHashApplied{Prepared: *prepared})
			return submitErr
		})
		if err != nil {
			return false, err
		}
	}
	more = more || len(candidates) == 32
	if !more {
		err = engine.Run(ctx, class, demand.Demand(execution.StepScanRepositoryEpoch, execution.MediaUnknown), func(stepCtx context.Context) error {
			_, submitErr := commits.ApplyRepositoryEpoch(stepCtx, commit.RepositoryEpochApplied{RepositoryID: args.RepositoryID, RequestedEpoch: args.RequestedEpoch})
			return submitErr
		})
		if err != nil {
			return false, err
		}
	}
	return more, nil
}

func repositoryScanCommandCurrent(state repo.RepositoryObservationState, run repo.RepositoryScanRun, args jobs.ScanRepositoryBatchArgs) bool {
	requested := int64(args.RequestedEpoch)
	return state.ActiveRunID.Valid &&
		state.ActiveRunID.UUID == run.RunID &&
		state.DesiredEpoch >= requested &&
		state.AppliedEpoch < requested &&
		run.RequestedEpoch == requested
}

// runAssetEnrichment keeps all optional enrichment work behind the one
// enrich_asset macro. The runner is a plain compute boundary; it has no
// River job construction or child scheduling path.
func runAssetEnrichment(
	ctx context.Context,
	assetProcessor *processors.AssetProcessor,
	reader queue.EnrichmentReader,
	settingsService queue.MLConfigProvider,
	lumenService queue.EnrichmentInference,
	classifierService queue.EnrichmentClassifier,
	repositoryFiles *storage.RepositoryFSFactory,
	engine *execution.Engine,
	demand execution.DemandCatalog,
	qos workqos.Class,
	args jobs.EnrichAssetArgs,
) (queue.EnrichmentResult, error) {
	if reader == nil || repositoryFiles == nil || engine == nil {
		return queue.EnrichmentResult{}, errors.New("asset enrichment runtime is not configured")
	}
	imageLoader := queue.NewDBMLImageLoader(reader, repositoryFiles)
	runner := &queue.EnrichmentRunner{
		Reader: reader, Settings: settingsService, Lumen: lumenService,
		Classifier:  classifierService,
		ImageLoader: imageLoader, Files: repositoryFiles,
		ExecuteStep: func(stepCtx context.Context, step queue.EnrichmentStep, work func(context.Context) error) error {
			class, err := execution.ClassFromQoS(qos)
			if err != nil {
				return err
			}
			return engine.Run(stepCtx, class, enrichmentStepDemand(demand, step), work)
		},
	}
	if assetProcessor != nil {
		runner.VideoFrames = assetProcessor.ProcessVideoFramesTask
	}
	return runner.Run(ctx, args)
}

func enrichmentStepDemand(demand execution.DemandCatalog, step queue.EnrichmentStep) execution.Resources {
	switch step {
	case queue.EnrichmentStepPHash:
		return demand.Demand(execution.StepEnrichComputePHash, execution.MediaPhoto)
	case queue.EnrichmentStepVideoFrames:
		return demand.Demand(execution.StepEnrichComputeSemantic, execution.MediaVideo)
	case queue.EnrichmentStepZeroShot:
		return demand.Demand(execution.StepEnrichComputeOCR, execution.MediaPhoto)
	default:
		return demand.Demand(execution.StepEnrichComputeSemantic, execution.MediaPhoto)
	}
}

type eventProjectionPreparer interface {
	PrepareAtRevision(context.Context, int32, uint64) (event.PreparedRebuild, error)
}

type locationProjectionPreparer interface {
	PrepareLocationRebuild(context.Context, uuid.UUID, int32, uint64) (service.PreparedLocationRebuild, error)
	PrepareLocationResolution(context.Context, int64) (service.PreparedLocationResolution, error)
}

type reindexProjectionPreparer interface {
	PrepareReindexReceipt(context.Context, uuid.UUID, uint64) (service.PreparedReindex, error)
}

type ocrProjectionPreparer interface {
	PrepareBatch(context.Context, int) (bleveocr.PreparedBatch, error)
	ApplyPreparedBatch(bleveocr.PreparedBatch) error
}

// pipelineRuntime owns the closed macro algorithms. app.go constructs this
// object and manages lifecycle only; River workers validate delivery payloads
// while every bounded compute/commit step declares its own resource vector.
type pipelineRuntime struct {
	engine             *execution.Engine
	demand             execution.DemandCatalog
	commits            *commit.Coordinator
	processor          *processors.AssetProcessor
	repository         *roecontroller.Controller
	repositoryHasher   *materializer.HashPreparer
	repositoryReader   repositoryScanReader
	eventProjection    eventProjectionPreparer
	locationProjection locationProjectionPreparer
	ocrProjection      ocrProjectionPreparer
	reindexProjection  reindexProjectionPreparer
	enrichmentReader   queue.EnrichmentReader
	settings           queue.MLConfigProvider
	lumen              queue.EnrichmentInference
	classifier         queue.EnrichmentClassifier
	files              *storage.RepositoryFSFactory
}

func (runtime *pipelineRuntime) register(workers *river.Workers) error {
	if runtime == nil || workers == nil || runtime.engine == nil || runtime.commits == nil || runtime.processor == nil ||
		runtime.repository == nil || runtime.repositoryHasher == nil || runtime.repositoryReader == nil ||
		runtime.eventProjection == nil || runtime.locationProjection == nil || runtime.ocrProjection == nil ||
		runtime.reindexProjection == nil || runtime.enrichmentReader == nil || runtime.files == nil {
		return errors.New("pipeline runtime is not configured")
	}
	river.AddWorker[jobs.IngestAssetArgs](workers, &queue.IngestMacroWorker{Execute: runtime.ingest})
	river.AddWorker[jobs.AnalyzeAssetArgs](workers, queue.NewAnalyzeAssetWorker(runtime.analyze))
	river.AddWorker[jobs.GenerateAssetDerivativesArgs](workers, queue.NewGenerateAssetDerivativesWorker(runtime.derivatives))
	river.AddWorker[jobs.TranscodeMediaArgs](workers, queue.NewTranscodeMediaWorker(runtime.transcode))
	river.AddWorker[jobs.EnrichAssetArgs](workers, &queue.EnrichAssetWorker{Execute: runtime.enrich})
	river.AddWorker[jobs.ScanRepositoryBatchArgs](workers, &queue.ScanRepositoryBatchWorker{Execute: runtime.scanRepository})
	river.AddWorker[jobs.RebuildProjectionBatchArgs](workers, &queue.RebuildProjectionBatchWorker{Execute: runtime.rebuildProjection})
	return nil
}

func (runtime *pipelineRuntime) registerBackup(workers *river.Workers, execute func(context.Context, bool) error) error {
	if runtime == nil || workers == nil || runtime.engine == nil || runtime.commits == nil || execute == nil {
		return errors.New("backup pipeline runtime is not configured")
	}
	river.AddWorker[jobs.BackupCatalogArgs](workers, &queue.BackupCatalogWorker{Execute: func(ctx context.Context, qos workqos.Class, args jobs.BackupCatalogArgs) error {
		class, err := execution.ClassFromQoS(qos)
		if err != nil {
			return err
		}
		return runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepBackupCatalog, execution.MediaUnknown), func(stepCtx context.Context) error {
			if err := execute(stepCtx, args.Force); err != nil {
				return err
			}
			_, err := runtime.commits.ApplyOperationReceipt(stepCtx, commit.OperationReceiptApplied{ReceiptID: args.RequestID, Kind: "backup"})
			return err
		})
	}})
	return nil
}

func (runtime *pipelineRuntime) ingest(ctx context.Context, qos workqos.Class, args jobs.IngestAssetArgs) error {
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return err
	}
	return runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepIngestCompute, execution.MediaUnknown), func(stepCtx context.Context) error {
		if _, err := runtime.processor.IngestAsset(stepCtx, args); err != nil {
			return err
		}
		_, err := runtime.commits.ApplyIngestReceipt(stepCtx, commit.IngestReceiptApplied{ReceiptID: args.ReceiptID}, args.CommitID)
		return err
	})
}

func (runtime *pipelineRuntime) analyze(ctx context.Context, qos workqos.Class, args jobs.AnalyzeAssetArgs) error {
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return err
	}
	return runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepAnalyzeCompute, execution.MediaUnknown), func(stepCtx context.Context) error {
		result, err := runtime.processor.ComputeMetadataTask(stepCtx, processors.MetadataArgs{AssetID: args.AssetID, ExpectedContentID: args.SourceFence})
		if err != nil {
			return err
		}
		if result.AssetID != uuid.Nil {
			metadataOutcome, err := runtime.commits.ApplyAssetMetadata(stepCtx, commit.AssetMetadataApplied{
				AssetID: result.AssetID, SourceFence: result.SourceContentID,
				PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion,
				Metadata: result.Metadata, Common: result.Common, ExifRaw: result.ExifRaw,
				ComponentRelation: result.ComponentRelation,
			})
			if err != nil {
				return err
			}
			if metadataOutcome.Outcome != commit.OutcomeStale {
				if _, err := runtime.commits.ApplyAssetStack(stepCtx, commit.AssetStackApplied{
					AssetID: result.AssetID, SourceFence: result.SourceContentID,
					PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion,
				}); err != nil {
					return err
				}
			}
		}
		return runtime.submitAssetStage(stepCtx, args.AssetID, args.SourceFence, "analyze", args.PipelineVersion, args.DesiredVersion)
	})
}

func (runtime *pipelineRuntime) derivatives(ctx context.Context, qos workqos.Class, args jobs.GenerateAssetDerivativesArgs) error {
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return err
	}
	var work *processors.PreparedThumbnail
	// Load stage
	if err := runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepDerivativesLoad, execution.MediaUnknown), func(stepCtx context.Context) error {
		var loadErr error
		work, loadErr = runtime.processor.LoadThumbnailTask(stepCtx, processors.ThumbnailArgs{
			AssetID: args.AssetID, ExpectedContentID: args.SourceFence, PipelineVersion: args.PipelineVersion,
		})
		return loadErr
	}); err != nil {
		return err
	}
	if work == nil {
		return nil
	}
	// Codec stage: photos use ImageCodec; video extraction and audio waveform use VideoCodec.
	computeStep := execution.StepDerivativesComputeThumb
	mType := execution.MediaPhoto
	if work.MediaType() == dbtypes.AssetTypeVideo {
		computeStep = execution.StepDerivativesComputeVideoFrame
		mType = execution.MediaVideo
	} else if work.MediaType() == dbtypes.AssetTypeAudio {
		computeStep = execution.StepDerivativesComputeVideoFrame
		mType = execution.MediaAudio
	}
	if err := runtime.engine.Run(ctx, class, runtime.demand.Demand(computeStep, mType), func(stepCtx context.Context) error {
		return runtime.processor.ComputeThumbnailCodec(stepCtx, work)
	}); err != nil {
		return err
	}
	if mType == execution.MediaVideo {
		if err := runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepDerivativesComputeScale, mType), func(stepCtx context.Context) error {
			return runtime.processor.ComputeThumbnailScale(stepCtx, work)
		}); err != nil {
			return err
		}
	}

	// Publish stage
	return runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepDerivativesPublish, mType), func(stepCtx context.Context) error {
		result, err := runtime.processor.PublishThumbnailTask(stepCtx, work)
		if err != nil {
			return err
		}
		// A stale source is intentionally a no-op in the processor. It must not
		// acknowledge the derivative generation: no artifacts were committed, and
		// marking this stage applied would let enrichment run without its required
		// small thumbnail.
		if result.AssetID == uuid.Nil {
			return nil
		}
		artifacts := make([]commit.ThumbnailArtifact, 0, len(result.Artifacts))
		for _, artifact := range result.Artifacts {
			artifacts = append(artifacts, commit.ThumbnailArtifact{
				RepositoryID: artifact.RepositoryID, Size: artifact.Size,
				StoragePath: artifact.StoragePath, MimeType: artifact.MimeType,
			})
		}
		if _, err := runtime.commits.ApplyAssetDerivatives(stepCtx, commit.AssetDerivativesApplied{
			AssetID: result.AssetID, SourceFence: result.SourceContentID,
			PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion, Artifacts: artifacts,
		}); err != nil {
			return err
		}
		return runtime.submitAssetStage(stepCtx, args.AssetID, args.SourceFence, "derivatives", args.PipelineVersion, args.DesiredVersion)
	})
}

func (runtime *pipelineRuntime) transcode(ctx context.Context, qos workqos.Class, args jobs.TranscodeMediaArgs) error {
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return err
	}
	mediaType, _ := runtime.processor.AssetMediaType(ctx, args.AssetID)
	mType := execution.MediaVideo
	if mediaType == dbtypes.AssetTypeAudio {
		mType = execution.MediaAudio
	}

	var work *processors.PreparedTranscode
	// Load / probe stage
	if err := runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepTranscodeLoad, mType), func(stepCtx context.Context) error {
		var loadErr error
		work, loadErr = runtime.processor.LoadTranscodeTask(stepCtx, processors.TranscodeArgs{
			AssetID: args.AssetID, ExpectedContentID: args.SourceFence, PipelineVersion: args.PipelineVersion,
		})
		return loadErr
	}); err != nil {
		return err
	}
	if work == nil {
		return nil
	}
	defer work.Close()

	// Compute stage: ffmpeg transcode
	if err := runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepTranscodeCompute, mType), func(stepCtx context.Context) error {
		return runtime.processor.ComputeTranscodeTask(stepCtx, work)
	}); err != nil {
		return err
	}

	// Publish stage
	return runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepTranscodePublish, mType), func(stepCtx context.Context) error {
		if err := runtime.processor.PublishTranscodeTask(stepCtx, work); err != nil {
			return err
		}
		return runtime.submitAssetStage(stepCtx, args.AssetID, args.SourceFence, "transcode", args.PipelineVersion, args.DesiredVersion)
	})
}

func (runtime *pipelineRuntime) enrich(ctx context.Context, qos workqos.Class, args jobs.EnrichAssetArgs) error {
	result, err := runAssetEnrichment(
		ctx,
		runtime.processor,
		runtime.enrichmentReader,
		runtime.settings,
		runtime.lumen,
		runtime.classifier,
		runtime.files,
		runtime.engine,
		runtime.demand,
		qos,
		args,
	)
	if err != nil {
		return err
	}
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return err
	}
	return runtime.engine.Run(ctx, class, runtime.demand.Demand(execution.StepEnrichPublish, execution.MediaUnknown), func(stepCtx context.Context) error {
		if result.PHash != nil || result.Semantic != nil || result.Aesthetic != nil || result.Species != nil || result.OCR != nil || result.Face != nil || result.AITags != nil {
			if _, err := runtime.commits.ApplyEnrichment(stepCtx, commit.EnrichmentApplied{
				AssetID: args.AssetID, SourceFence: args.SourceFence,
				PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion,
				PHash: result.PHash, Semantic: result.Semantic, Aesthetic: result.Aesthetic,
				Species: result.Species, OCR: result.OCR, Face: result.Face, AITags: result.AITags,
			}); err != nil {
				return err
			}
		}
		if result.VideoFrames != nil {
			frames := make([]commit.VideoFrameEmbedding, 0, len(result.VideoFrames.Frames))
			for _, frame := range result.VideoFrames.Frames {
				frames = append(frames, commit.VideoFrameEmbedding{FrameTsMs: frame.FrameTsMs, Vector: frame.Vector})
			}
			if _, err := runtime.commits.ApplyVideoFrameEmbeddings(stepCtx, commit.VideoFrameEmbeddingsApplied{
				AssetID: result.VideoFrames.AssetID, SourceFence: result.VideoFrames.SourceContentID,
				PipelineVersion: args.PipelineVersion, DesiredVersion: args.DesiredVersion,
				ModelID: result.VideoFrames.ModelID, Frames: frames,
			}); err != nil {
				return err
			}
		}
		return runtime.submitAssetStage(stepCtx, args.AssetID, args.SourceFence, "enrich", args.PipelineVersion, args.DesiredVersion)
	})
}

func (runtime *pipelineRuntime) scanRepository(ctx context.Context, qos workqos.Class, args jobs.ScanRepositoryBatchArgs) (bool, error) {
	return runRepositoryScanBatch(
		ctx,
		runtime.repository,
		runtime.repositoryHasher,
		runtime.repositoryReader,
		runtime.engine,
		runtime.demand,
		runtime.commits,
		qos,
		args,
	)
}

func (runtime *pipelineRuntime) rebuildProjection(ctx context.Context, qos workqos.Class, args jobs.RebuildProjectionBatchArgs) (queue.ProjectionExecution, error) {
	var result queue.ProjectionExecution
	class, err := execution.ClassFromQoS(qos)
	if err != nil {
		return result, err
	}
	err = runtime.engine.Run(ctx, class, runtime.demand.DemandForProjection(args.ProjectionKind), func(stepCtx context.Context) error {
		apply, more, snooze, noop, err := runtime.prepareProjection(stepCtx, args)
		if err != nil {
			return err
		}
		result = queue.ProjectionExecution{More: more, Snooze: snooze, Noop: noop}
		if noop {
			return nil
		}
		if apply == nil {
			return errors.New("projection step returned no typed commit")
		}
		_, err = apply(stepCtx)
		if err == nil {
			result.Acknowledged = true
		}
		return err
	})
	return result, err
}

func (runtime *pipelineRuntime) prepareProjection(ctx context.Context, args jobs.RebuildProjectionBatchArgs) (func(context.Context) (commit.Result, error), bool, time.Duration, bool, error) {
	switch args.ProjectionKind {
	case "event":
		ownerID, err := strconv.ParseInt(args.Scope, 10, 32)
		if err != nil {
			return nil, false, 0, false, err
		}
		prepared, err := runtime.eventProjection.PrepareAtRevision(ctx, int32(ownerID), args.SourceRevision)
		if errors.Is(err, event.ErrStaleRevision) {
			return nil, false, 0, true, nil
		}
		if err != nil {
			return nil, false, 0, false, err
		}
		return func(ctx context.Context) (commit.Result, error) {
			return runtime.commits.ApplyEventProjection(ctx, commit.EventProjectionApplied{Prepared: prepared, ProjectionVersion: args.ProjectionVersion})
		}, false, 0, false, nil
	case "location":
		parts := strings.SplitN(args.Scope, ":", 2)
		if len(parts) != 2 {
			return nil, false, 0, false, fmt.Errorf("invalid location projection scope %q", args.Scope)
		}
		repositoryID, err := uuid.Parse(parts[0])
		if err != nil {
			return nil, false, 0, false, fmt.Errorf("invalid location repository scope %q: %w", parts[0], err)
		}
		ownerID, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			return nil, false, 0, false, err
		}
		prepared, err := runtime.locationProjection.PrepareLocationRebuild(ctx, repositoryID, int32(ownerID), args.SourceRevision)
		if errors.Is(err, service.ErrLocationProjectionStale) {
			return nil, false, 0, true, nil
		}
		if err != nil {
			return nil, false, 0, false, err
		}
		return func(ctx context.Context) (commit.Result, error) {
			return runtime.commits.ApplyLocationProjection(ctx, commit.LocationProjectionApplied{Prepared: prepared, ProjectionVersion: args.ProjectionVersion})
		}, !prepared.Complete, 0, false, nil
	case "location_resolution":
		prepared, err := runtime.locationProjection.PrepareLocationResolution(ctx, int64(args.SourceRevision))
		if errors.Is(err, service.ErrLocationProjectionStale) {
			return nil, false, 0, true, nil
		}
		if err != nil {
			return nil, false, 0, false, err
		}
		return func(ctx context.Context) (commit.Result, error) {
			return runtime.commits.ApplyLocationResolution(ctx, commit.LocationResolutionApplied{Prepared: prepared, ProjectionVersion: args.ProjectionVersion})
		}, !prepared.Complete, prepared.NextDelay, false, nil
	case "ocr":
		prepared, err := runtime.ocrProjection.PrepareBatch(ctx, bleveocr.DefaultOutboxBatchSize)
		if err != nil {
			return nil, false, 0, false, err
		}
		if err := runtime.ocrProjection.ApplyPreparedBatch(prepared); err != nil {
			return nil, false, 0, false, err
		}
		entries := make([]commit.OCRIndexEntry, 0, len(prepared.Entries))
		for _, entry := range prepared.Entries {
			entries = append(entries, commit.OCRIndexEntry{AssetID: entry.AssetID, Revision: entry.Revision})
		}
		return func(ctx context.Context) (commit.Result, error) {
			return runtime.commits.ApplyOCRProjection(ctx, commit.OCRProjectionApplied{
				Entries: entries, SourceRevision: args.SourceRevision,
				ProjectionVersion: args.ProjectionVersion, Complete: !prepared.More,
			})
		}, prepared.More, 0, false, nil
	case "asset_reindex":
		receiptID, err := uuid.Parse(args.Scope)
		if err != nil {
			return nil, false, 0, false, err
		}
		prepared, err := runtime.reindexProjection.PrepareReindexReceipt(ctx, receiptID, args.SourceRevision)
		if errors.Is(err, service.ErrReindexProjectionStale) {
			return nil, false, 0, true, nil
		}
		if err != nil {
			return nil, false, 0, false, err
		}
		if prepared.ReceiptID == uuid.Nil {
			return nil, false, 0, true, nil
		}
		return func(ctx context.Context) (commit.Result, error) {
			return runtime.commits.ApplyReindexProjection(ctx, commit.ReindexProjectionApplied{Prepared: prepared, ProjectionVersion: args.ProjectionVersion})
		}, prepared.HasMore, 0, false, nil
	default:
		return nil, false, 0, false, fmt.Errorf("unsupported projection kind %q", args.ProjectionKind)
	}
}

func (runtime *pipelineRuntime) submitAssetStage(ctx context.Context, assetID, sourceFence uuid.UUID, stage, pipelineVersion string, desiredVersion uint64) error {
	_, err := runtime.commits.ApplyAssetStage(ctx, commit.AssetStageApplied{
		AssetID: assetID, SourceFence: sourceFence, Stage: stage,
		PipelineVersion: pipelineVersion, DesiredVersion: desiredVersion,
	})
	return err
}
