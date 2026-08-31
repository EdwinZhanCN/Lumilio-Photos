package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"

	"server/internal/commit"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/processors"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/settings"
	"server/internal/storage"
	"server/internal/utils/imagesource"
	"server/internal/utils/phash"
)

// MLPreprocessVersionV1 identifies the image tensor preparation contract. It
// belongs to the compute boundary, not to River payload definitions.
const MLPreprocessVersionV1 = "ml-image-v1"

// ErrEnrichmentNotReady is returned when a dependency (for example a derived
// thumbnail or Lumen backend) has not become available yet. The enclosing
// macro retries the whole fenced stage; no child River job or snooze value is
// leaked from this runner.
var ErrEnrichmentNotReady = errors.New("asset enrichment dependency is not ready")

type EnrichmentStep string

const (
	EnrichmentStepPHash       EnrichmentStep = "phash"
	EnrichmentStepSemantic    EnrichmentStep = "semantic"
	EnrichmentStepBioCLIP     EnrichmentStep = "bioclip"
	EnrichmentStepOCR         EnrichmentStep = "ocr"
	EnrichmentStepFace        EnrichmentStep = "face"
	EnrichmentStepZeroShot    EnrichmentStep = "zero_shot"
	EnrichmentStepVideoFrames EnrichmentStep = "video_frames"
)

type EnrichmentStepExecutor func(context.Context, EnrichmentStep, func(context.Context) error) error

type EnrichmentResult struct {
	PHash       *EmbeddingResult
	Semantic    *EmbeddingResult
	Aesthetic   *AestheticResult
	Species     *SpeciesResult
	OCR         *types.OCRV1
	Face        *FaceResult
	AITags      *TagsResult
	VideoFrames *processors.VideoFramesResult
}

// EmbeddingResult is an immutable vector produced by enrichment. It is
// intentionally free of catalog handles; the commit coordinator owns the
// transaction that persists it.
type EmbeddingResult = commit.EnrichmentEmbedding
type AestheticResult = commit.EnrichmentAesthetic
type SpeciesResult = commit.EnrichmentSpecies
type TagsResult = commit.EnrichmentTags

// FaceResult carries the recognition response and the decoded source bytes
// needed to publish immutable face crops during the catalog commit.
type FaceResult = commit.EnrichmentFace

type EnrichmentReader interface {
	MLImageReader
	GetAssetByIDAny(context.Context, uuid.UUID) (repo.Asset, error)
}

type EnrichmentInference interface {
	SemanticImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error)
	BioClipClassify(context.Context, *imagesource.MLImage, int) ([]types.Label, error)
	OCR(context.Context, *imagesource.MLImage) (*types.OCRV1, error)
	FaceRecognition(context.Context, *imagesource.MLImage) (*types.FaceV1, error)
}

type EnrichmentClassifier interface {
	Classify(context.Context, service.PrimaryEmbedding) ([]service.ClassifierHit, error)
}

// EnrichmentRunner executes the complete enrichment DAG for one asset fence.
// It deliberately has no River dependency. Persistence services remain behind
// interfaces as read-only compute dependencies; the River adapter owns only
// admission and commit ACKs.
type EnrichmentRunner struct {
	Reader      EnrichmentReader
	Settings    MLConfigProvider
	Lumen       EnrichmentInference
	Classifier  EnrichmentClassifier
	ImageLoader MLImageLoader
	Files       *storage.RepositoryFSFactory
	VideoFrames func(context.Context, processors.VideoFramesArgs) (processors.VideoFramesResult, error)
	ExecuteStep EnrichmentStepExecutor
}

func (r *EnrichmentRunner) Run(ctx context.Context, args jobs.EnrichAssetArgs) (EnrichmentResult, error) {
	var output EnrichmentResult
	if r == nil || r.Reader == nil || r.Files == nil {
		return output, errors.New("asset enrichment runner is not configured")
	}
	asset, err := r.Reader.GetAssetByIDAny(ctx, args.AssetID)
	if errors.Is(err, sql.ErrNoRows) || asset.IsDeleted || asset.ContentID != args.SourceFence {
		return output, nil
	}
	if err != nil {
		return output, fmt.Errorf("load enrichment asset: %w", err)
	}
	var phashResult *EmbeddingResult
	err = r.executeStep(ctx, EnrichmentStepPHash, func(stepCtx context.Context) error {
		var stepErr error
		phashResult, stepErr = r.runPHash(stepCtx, asset)
		return stepErr
	})
	if err != nil {
		return output, err
	}
	output.PHash = phashResult
	cfg, err := r.mlConfig(ctx)
	if err != nil {
		return output, err
	}
	if dbtypes.AssetType(asset.Type) == dbtypes.AssetTypePhoto {
		loader := r.ImageLoader
		if loader == nil {
			loader = NewDBMLImageLoader(r.Reader, r.Files)
		}
		if cfg.SemanticEnabled {
			var semanticResult *semanticResult
			semanticErr := r.executeStep(ctx, EnrichmentStepSemantic, func(stepCtx context.Context) error {
				var stepErr error
				semanticResult, stepErr = r.runSemantic(stepCtx, loader, args, cfg)
				return stepErr
			})
			if semanticErr != nil {
				return output, semanticErr
			}
			if semanticResult != nil {
				output.Semantic = semanticResult.Embedding
				output.Aesthetic = semanticResult.Aesthetic
			}
		}
		if cfg.BioCLIPEnabled {
			var species []dbtypes.SpeciesPredictionMeta
			speciesErr := r.executeStep(ctx, EnrichmentStepBioCLIP, func(stepCtx context.Context) error {
				var stepErr error
				species, stepErr = r.runBioClip(stepCtx, loader, args)
				return stepErr
			})
			if speciesErr != nil {
				return output, speciesErr
			}
			output.Species = &SpeciesResult{Predictions: species}
		}
		if cfg.OCREnabled {
			var ocr *types.OCRV1
			ocrErr := r.executeStep(ctx, EnrichmentStepOCR, func(stepCtx context.Context) error {
				var stepErr error
				ocr, stepErr = r.runOCR(stepCtx, loader, args)
				return stepErr
			})
			if ocrErr != nil {
				return output, ocrErr
			}
			output.OCR = ocr
		}
		if cfg.FaceEnabled {
			var face *FaceResult
			faceErr := r.executeStep(ctx, EnrichmentStepFace, func(stepCtx context.Context) error {
				var stepErr error
				face, stepErr = r.runFace(stepCtx, loader, args)
				return stepErr
			})
			if faceErr != nil {
				return output, faceErr
			}
			output.Face = face
		}
		// Zero-shot classification consumes the semantic embedding and is
		// therefore attempted after semantic work when that capability exists.
		if cfg.SemanticEnabled && output.Semantic != nil {
			var tags []service.AIGeneratedTag
			tagErr := r.executeStep(ctx, EnrichmentStepZeroShot, func(stepCtx context.Context) error {
				var stepErr error
				tags, stepErr = r.runZeroShot(stepCtx, args, service.PrimaryEmbedding{Vector: output.Semantic.Vector, Model: output.Semantic.Model, Dimensions: len(output.Semantic.Vector)})
				return stepErr
			})
			if tagErr != nil {
				return output, tagErr
			}
			output.AITags = &TagsResult{Tags: tags}
		}
	}
	if dbtypes.AssetType(asset.Type) == dbtypes.AssetTypeVideo && cfg.SemanticEnabled && cfg.VideoSemanticEnabled && r.VideoFrames != nil {
		var frames processors.VideoFramesResult
		err := r.executeStep(ctx, EnrichmentStepVideoFrames, func(stepCtx context.Context) error {
			var stepErr error
			frames, stepErr = r.VideoFrames(stepCtx, processors.VideoFramesArgs{AssetID: args.AssetID, ExpectedContentID: args.SourceFence, PreprocessVersion: MLPreprocessVersionV1})
			return stepErr
		})
		if err != nil {
			return output, err
		}
		if frames.AssetID != uuid.Nil {
			output.VideoFrames = &frames
		}
	}
	return output, nil
}

func (r *EnrichmentRunner) executeStep(ctx context.Context, step EnrichmentStep, work func(context.Context) error) error {
	if r.ExecuteStep == nil {
		return work(ctx)
	}
	return r.ExecuteStep(ctx, step, work)
}

func (r *EnrichmentRunner) mlConfig(ctx context.Context) (settings.ML, error) {
	if r.Settings == nil {
		return settings.ML{SemanticEnabled: true, BioCLIPEnabled: true, OCREnabled: true, FaceEnabled: true, VideoSemanticEnabled: true}, nil
	}
	cfg, err := r.Settings.GetEffectiveMLConfig(ctx)
	if err != nil {
		return settings.ML{}, fmt.Errorf("load ML settings: %w", err)
	}
	return cfg, nil
}

func (r *EnrichmentRunner) runPHash(ctx context.Context, asset repo.Asset) (*EmbeddingResult, error) {
	if dbtypes.AssetType(asset.Type) != dbtypes.AssetTypePhoto {
		return nil, nil
	}
	thumbnail, err := r.Reader.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{AssetID: asset.AssetID, Size: "small"})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEnrichmentNotReady
	}
	if err != nil {
		return nil, fmt.Errorf("get small thumbnail: %w", err)
	}
	thumbnailPath, err := storage.ParsePrivateRepositoryPath(thumbnail.StoragePath)
	if err != nil {
		return nil, err
	}
	if err := validateThumbnailContent(asset, "small", thumbnailPath); err != nil {
		if errors.Is(err, ErrDerivedAssetStale) {
			return nil, nil
		}
		return nil, err
	}
	if thumbnail.RepositoryID == uuid.Nil {
		return nil, fmt.Errorf("small thumbnail has no repository")
	}
	repository, err := r.Reader.GetRepository(ctx, thumbnail.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("get thumbnail repository: %w", err)
	}
	fs, err := r.Files.Open(repository)
	if err != nil {
		return nil, err
	}
	defer fs.Close()
	file, err := fs.OpenPrivate(thumbnailPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrEnrichmentNotReady
	}
	if err != nil {
		return nil, fmt.Errorf("open small thumbnail: %w", err)
	}
	defer file.Close()
	hash, err := phash.ComputeFromReader(file)
	if err != nil {
		return nil, err
	}
	if err := r.ensureCurrent(ctx, asset.AssetID, asset.ContentID); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return nil, nil
		}
		return nil, err
	}
	return &EmbeddingResult{Type: service.EmbeddingTypePHash, Model: phash.ModelDCTPHashV1, Vector: phash.ToVector(hash), IsPrimary: true}, nil
}

type semanticResult struct {
	Embedding *EmbeddingResult
	Aesthetic *AestheticResult
}

func (r *EnrichmentRunner) runSemantic(ctx context.Context, loader MLImageLoader, args jobs.EnrichAssetArgs, _ settings.ML) (*semanticResult, error) {
	if r.Lumen == nil {
		return nil, ErrEnrichmentNotReady
	}
	current, err := validateLoaderAssetWork(ctx, loader, args.AssetID, args.SourceFence)
	if err != nil || !current {
		return nil, err
	}
	imageData, err := loader.LoadMLImage(ctx, args.AssetID, imagesource.PurposeSemantic, MLPreprocessVersionV1)
	if err != nil {
		return nil, enrichmentDependencyError(err)
	}
	embedding, err := r.Lumen.SemanticImageEmbed(ctx, imageData)
	if err != nil {
		return nil, fmt.Errorf("generate semantic embedding: %w", err)
	}
	if err := r.ensureCurrent(ctx, args.AssetID, args.SourceFence); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return nil, nil
		}
		return nil, err
	}
	if embedding == nil || len(embedding.Vector) == 0 {
		return nil, errors.New("semantic embedding is empty")
	}
	result := &semanticResult{Embedding: &EmbeddingResult{Type: service.EmbeddingTypeSemantic, Model: embedding.ModelID, Vector: embedding.Vector, IsPrimary: true}}
	if score, ok := embedding.AestheticScoreValue(); ok {
		result.Aesthetic = &AestheticResult{Score: score, Model: embedding.ModelID}
	}
	return result, nil
}

func (r *EnrichmentRunner) runBioClip(ctx context.Context, loader MLImageLoader, args jobs.EnrichAssetArgs) ([]dbtypes.SpeciesPredictionMeta, error) {
	if r.Lumen == nil {
		return nil, ErrEnrichmentNotReady
	}
	current, err := validateLoaderAssetWork(ctx, loader, args.AssetID, args.SourceFence)
	if err != nil || !current {
		return nil, err
	}
	imageData, err := loader.LoadMLImage(ctx, args.AssetID, imagesource.PurposeBioClip, MLPreprocessVersionV1)
	if err != nil {
		return nil, enrichmentDependencyError(err)
	}
	labels, err := r.Lumen.BioClipClassify(ctx, imageData, 3)
	if err != nil {
		return nil, fmt.Errorf("classify BioCLIP: %w", err)
	}
	if err := r.ensureCurrent(ctx, args.AssetID, args.SourceFence); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return nil, nil
		}
		return nil, err
	}
	return labelsToSpeciesPredictions(labels), nil
}

func (r *EnrichmentRunner) runOCR(ctx context.Context, loader MLImageLoader, args jobs.EnrichAssetArgs) (*types.OCRV1, error) {
	if r.Lumen == nil {
		return nil, ErrEnrichmentNotReady
	}
	current, err := validateLoaderAssetWork(ctx, loader, args.AssetID, args.SourceFence)
	if err != nil || !current {
		return nil, err
	}
	imageData, err := loader.LoadMLImage(ctx, args.AssetID, imagesource.PurposeOCR, MLPreprocessVersionV1)
	if err != nil {
		return nil, enrichmentDependencyError(err)
	}
	result, err := r.Lumen.OCR(ctx, imageData)
	if err != nil {
		return nil, fmt.Errorf("run OCR: %w", err)
	}
	if err := r.ensureCurrent(ctx, args.AssetID, args.SourceFence); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func (r *EnrichmentRunner) runFace(ctx context.Context, loader MLImageLoader, args jobs.EnrichAssetArgs) (*FaceResult, error) {
	if r.Lumen == nil {
		return nil, ErrEnrichmentNotReady
	}
	current, err := validateLoaderAssetWork(ctx, loader, args.AssetID, args.SourceFence)
	if err != nil || !current {
		return nil, err
	}
	imageData, err := loader.LoadMLImage(ctx, args.AssetID, imagesource.PurposeFace, MLPreprocessVersionV1)
	if err != nil {
		return nil, enrichmentDependencyError(err)
	}
	started := time.Now()
	result, err := r.Lumen.FaceRecognition(ctx, imageData)
	if err != nil {
		return nil, fmt.Errorf("run face recognition: %w", err)
	}
	if err := r.ensureCurrent(ctx, args.AssetID, args.SourceFence); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return nil, nil
		}
		return nil, err
	}
	return &FaceResult{Payload: result, ImageData: imageData.EncodedSource, ProcessingTimeMs: int(time.Since(started).Milliseconds())}, nil
}

func (r *EnrichmentRunner) runZeroShot(ctx context.Context, args jobs.EnrichAssetArgs, embedding service.PrimaryEmbedding) ([]service.AIGeneratedTag, error) {
	if r.Classifier == nil || len(embedding.Vector) == 0 {
		return nil, nil
	}
	hits, err := r.Classifier.Classify(ctx, embedding)
	if err != nil {
		return nil, fmt.Errorf("classify asset: %w", err)
	}
	if err := r.ensureCurrent(ctx, args.AssetID, args.SourceFence); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return nil, nil
		}
		return nil, err
	}
	tags := make([]service.AIGeneratedTag, 0, len(hits))
	for _, hit := range hits {
		tags = append(tags, service.AIGeneratedTag{Name: hit.TagName, Confidence: float32(hit.Confidence), Source: service.AssetTagSourceZeroshot, Category: hit.Category})
	}
	return tags, nil
}

func (r *EnrichmentRunner) ensureCurrent(ctx context.Context, assetID, expected uuid.UUID) error {
	_, err := validateCurrentAssetWork(ctx, r.Reader, assetID, expected)
	return err
}

func enrichmentDependencyError(err error) error {
	if errors.Is(err, ErrAssetWorkStale) {
		return nil
	}
	if errors.Is(err, ErrDerivedAssetNotReady) {
		return ErrEnrichmentNotReady
	}
	return err
}

func labelsToSpeciesPredictions(labels []types.Label) []dbtypes.SpeciesPredictionMeta {
	predictions := make([]dbtypes.SpeciesPredictionMeta, 0, len(labels))
	for _, label := range labels {
		predictions = append(predictions, dbtypes.SpeciesPredictionMeta{Label: label.Label, Score: label.Score})
	}
	return predictions
}
