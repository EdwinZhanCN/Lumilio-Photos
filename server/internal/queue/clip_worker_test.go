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

type clipWorkerLumenStub struct {
	available map[string]bool
	bioLabels []types.Label
}

func (s *clipWorkerLumenStub) ClipTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) ClipTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) ClipImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error) {
	return &types.EmbeddingV1{ModelID: "clip-image", Vector: []float32{0.1, 0.2}}, nil
}

func (s *clipWorkerLumenStub) BioClipClassify(context.Context, *imagesource.MLImage, int) ([]types.Label, error) {
	return s.bioLabels, nil
}

func (s *clipWorkerLumenStub) FaceDetectEmbed(context.Context, *imagesource.MLImage) (*types.FaceV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) OCR(context.Context, *imagesource.MLImage) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) VLMCaption(context.Context, *imagesource.MLImage) (string, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) VLMCaptionWithPrompt(context.Context, *imagesource.MLImage, string) (string, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) VLMCaptionWithMetadata(context.Context, *imagesource.MLImage, string) (*types.TextGenerationV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) GetAvailableModels(context.Context) ([]*discovery.NodeInfo, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) WarmupTasks(context.Context, []string) map[string]bool {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) IsTaskAvailable(taskName string) bool {
	return s.available[taskName]
}

func (s *clipWorkerLumenStub) Start(context.Context) error {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) Close() error {
	panic("not implemented")
}

type clipWorkerEmbeddingStub struct {
	savedType  service.EmbeddingType
	savedModel string
	savedVec   []float32
}

func (s *clipWorkerEmbeddingStub) SaveEmbedding(_ context.Context, _ pgtype.UUID, embeddingType service.EmbeddingType, model string, vector []float32, _ bool) error {
	s.savedType = embeddingType
	s.savedModel = model
	s.savedVec = vector
	return nil
}

func (s *clipWorkerEmbeddingStub) ResolveDefaultSearchSpace(context.Context, service.EmbeddingType, string, int) (repo.EmbeddingSpace, error) {
	panic("not implemented")
}

func (s *clipWorkerEmbeddingStub) GetEmbedding(context.Context, pgtype.UUID, service.EmbeddingType, string) (repo.Embedding, error) {
	panic("not implemented")
}

func (s *clipWorkerEmbeddingStub) GetAssetEmbeddingInfo(context.Context, pgtype.UUID) (map[service.EmbeddingType]service.EmbeddingInfo, error) {
	panic("not implemented")
}

func (s *clipWorkerEmbeddingStub) DeleteEmbedding(context.Context, pgtype.UUID, service.EmbeddingType, string) error {
	panic("not implemented")
}

type clipWorkerTagStub struct {
	tags    []service.AIGeneratedTag
	sources []string
}

func (s *clipWorkerTagStub) ReplaceAssetAIGeneratedTags(_ context.Context, _ pgtype.UUID, tags []service.AIGeneratedTag, sources []string) error {
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

func TestProcessClipWorkerSavesImageEmbedding(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	embeddingSvc := &clipWorkerEmbeddingStub{}
	lumenSvc := &clipWorkerLumenStub{
		available: map[string]bool{
			"clip_image_embed": true,
		},
	}
	imageLoader := &workerImageLoaderStub{data: []byte("image")}

	worker := &ProcessClipWorker{
		EmbeddingService: embeddingSvc,
		LumenService:     lumenSvc,
		ImageLoader:      imageLoader,
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessClipArgs]{
		Args: ProcessClipArgs{AssetID: assetID},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if embeddingSvc.savedType != service.EmbeddingTypeCLIP {
		t.Fatalf("expected clip embedding type, got %q", embeddingSvc.savedType)
	}
	if imageLoader.purpose != imagesource.PurposeClip {
		t.Fatalf("expected CLIP image purpose, got %q", imageLoader.purpose)
	}
}

func TestProcessClipWorkerSnoozesWithoutImageEmbeddingTask(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	worker := &ProcessClipWorker{
		LumenService: &clipWorkerLumenStub{
			available: map[string]bool{
				"clip_image_embed": false,
			},
		},
		EmbeddingService: &clipWorkerEmbeddingStub{},
		ImageLoader:      &workerImageLoaderStub{data: []byte("image")},
	}

	err := worker.Work(context.Background(), &river.Job[ProcessClipArgs]{
		Args: ProcessClipArgs{AssetID: assetID},
	})
	if err == nil {
		t.Fatal("expected snooze error")
	}

	if loader := worker.ImageLoader.(*workerImageLoaderStub); loader.called != 0 {
		t.Fatalf("expected image loader not to be called while task is unavailable, got %d calls", loader.called)
	}
}
