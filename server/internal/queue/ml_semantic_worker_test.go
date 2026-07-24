package queue

import (
	"context"
	"testing"

	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/utils/imagesource"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type semanticWorkerLumenStub struct {
	available map[string]bool
	bioLabels []types.Label
}

func (s *semanticWorkerLumenStub) SemanticTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) SemanticTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) SemanticImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error) {
	return &types.EmbeddingV1{ModelID: "clip-image", Vector: []float32{0.1, 0.2}}, nil
}

func (s *semanticWorkerLumenStub) BioClipClassify(context.Context, *imagesource.MLImage, int) ([]types.Label, error) {
	return s.bioLabels, nil
}

func (s *semanticWorkerLumenStub) FaceRecognition(context.Context, *imagesource.MLImage) (*types.FaceV1, error) {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) OCR(context.Context, *imagesource.MLImage) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) GetAvailableModels(context.Context) ([]*discovery.NodeInfo, error) {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) WarmupTasks(context.Context, []string) map[string]bool {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) PoolStats() service.PoolStats {
	return service.PoolStats{}
}

func (s *semanticWorkerLumenStub) GetNodes() []*discovery.NodeInfo {
	return nil
}

func (s *semanticWorkerLumenStub) IsTaskAvailable(taskName string) bool {
	return s.available[taskName]
}

func (s *semanticWorkerLumenStub) Start(context.Context) error {
	panic("not implemented")
}

func (s *semanticWorkerLumenStub) Close() error {
	panic("not implemented")
}

type semanticWorkerEmbeddingStub struct {
	savedType  service.EmbeddingType
	savedModel string
	savedVec   []float32
}

func (s *semanticWorkerEmbeddingStub) SaveEmbedding(_ context.Context, _ pgtype.UUID, embeddingType service.EmbeddingType, model string, vector []float32, _ bool) error {
	s.savedType = embeddingType
	s.savedModel = model
	s.savedVec = vector
	return nil
}

func (s *semanticWorkerEmbeddingStub) SaveVideoFrameEmbeddings(context.Context, pgtype.UUID, string, []service.VideoFrameEmbedding) error {
	return nil
}

func (s *semanticWorkerEmbeddingStub) SaveAestheticScore(context.Context, pgtype.UUID, float32, string) error {
	return nil
}

func (s *semanticWorkerEmbeddingStub) ResolveDefaultSearchSpace(context.Context, service.EmbeddingType, string, int) (repo.EmbeddingSpace, error) {
	panic("not implemented")
}

func (s *semanticWorkerEmbeddingStub) GetEmbedding(context.Context, pgtype.UUID, service.EmbeddingType, string) (repo.Embedding, error) {
	panic("not implemented")
}

func (s *semanticWorkerEmbeddingStub) GetAssetEmbeddingInfo(context.Context, pgtype.UUID) (map[service.EmbeddingType]service.EmbeddingInfo, error) {
	panic("not implemented")
}

func (s *semanticWorkerEmbeddingStub) DeleteEmbedding(context.Context, pgtype.UUID, service.EmbeddingType, string) error {
	panic("not implemented")
}

func (s *semanticWorkerEmbeddingStub) GetPrimaryEmbeddingVector(context.Context, pgtype.UUID, service.EmbeddingType) (service.PrimaryEmbedding, error) {
	panic("not implemented")
}

type semanticWorkerTagStub struct {
	tags    []service.AIGeneratedTag
	sources []string
}

func (s *semanticWorkerTagStub) ReplaceAssetAIGeneratedTags(_ context.Context, _ pgtype.UUID, tags []service.AIGeneratedTag, sources []string) error {
	s.tags = append([]service.AIGeneratedTag(nil), tags...)
	s.sources = append([]string(nil), sources...)
	return nil
}

type workerImageLoaderStub struct {
	data          []byte
	encodedSource []byte
	called        int
	purpose       imagesource.Purpose
	version       string
}

func (s *workerImageLoaderStub) LoadMLImage(_ context.Context, _ pgtype.UUID, purpose imagesource.Purpose, preprocessVersion string) (*imagesource.MLImage, error) {
	s.called++
	s.purpose = purpose
	s.version = preprocessVersion
	encodedSource := s.encodedSource
	if encodedSource == nil {
		encodedSource = s.data
	}
	return &imagesource.MLImage{
		Data:          append([]byte(nil), s.data...),
		EncodedSource: append([]byte(nil), encodedSource...),
		Width:         1,
		Height:        1,
		Channels:      3,
		Layout:        "HWC",
		DType:         "uint8",
		ColorSpace:    "RGB",
	}, nil
}

func TestProcessSemanticWorkerSavesImageEmbedding(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	embeddingSvc := &semanticWorkerEmbeddingStub{}
	lumenSvc := &semanticWorkerLumenStub{
		available: map[string]bool{
			"semantic_image_embed": true,
		},
	}
	imageLoader := &workerImageLoaderStub{data: []byte("image")}

	worker := &ProcessSemanticWorker{
		EmbeddingService: embeddingSvc,
		LumenService:     lumenSvc,
		ImageLoader:      imageLoader,
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessSemanticArgs]{
		Args: ProcessSemanticArgs{AssetID: assetID},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if embeddingSvc.savedType != service.EmbeddingTypeSemantic {
		t.Fatalf("expected semantic embedding type, got %q", embeddingSvc.savedType)
	}
	if imageLoader.purpose != imagesource.PurposeSemantic {
		t.Fatalf("expected semantic image purpose, got %q", imageLoader.purpose)
	}
}

func TestProcessSemanticWorkerDoesNotSnoozeWithoutTaskCheck(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	imageLoader := &workerImageLoaderStub{data: []byte("image")}
	embeddingSvc := &semanticWorkerEmbeddingStub{}
	worker := &ProcessSemanticWorker{
		LumenService: &semanticWorkerLumenStub{
			available: map[string]bool{
				"semantic_image_embed": false,
			},
		},
		EmbeddingService: embeddingSvc,
		ImageLoader:      imageLoader,
	}

	err := worker.Work(context.Background(), &river.Job[ProcessSemanticArgs]{
		Args: ProcessSemanticArgs{AssetID: assetID},
	})
	// In the new architecture, IsTaskAvailable is not checked before Infer.
	// The worker proceeds to call Infer directly. The stub always returns
	// a successful embedding, so the worker should succeed.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embeddingSvc.savedType != service.EmbeddingTypeSemantic {
		t.Fatalf("expected semantic embedding to be saved even without task check")
	}
}
