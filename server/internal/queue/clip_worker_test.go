package queue

import (
	"context"
	"testing"

	"server/internal/db/repo"
	"server/internal/service"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type clipWorkerLumenStub struct {
	available  map[string]bool
	clipLabels []types.Label
	sceneLabel []types.Label
	bioLabels  []types.Label
}

func (s *clipWorkerLumenStub) ClipTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) ClipTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) ClipImageEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	return &types.EmbeddingV1{ModelID: "clip-image", Vector: []float32{0.1, 0.2}}, nil
}

func (s *clipWorkerLumenStub) ClipClassify(context.Context, []byte, int) ([]types.Label, error) {
	return s.clipLabels, nil
}

func (s *clipWorkerLumenStub) ClipSceneClassify(context.Context, []byte, int) ([]types.Label, error) {
	return s.sceneLabel, nil
}

func (s *clipWorkerLumenStub) BioClipClassify(context.Context, []byte, int) ([]types.Label, error) {
	return s.bioLabels, nil
}

func (s *clipWorkerLumenStub) FaceDetectEmbed(context.Context, []byte) (*types.FaceV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) OCR(context.Context, []byte) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) VLMCaption(context.Context, []byte) (string, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) VLMCaptionWithPrompt(context.Context, []byte, string) (string, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) VLMCaptionWithMetadata(context.Context, []byte, string) (*types.TextGenerationV1, error) {
	panic("not implemented")
}

func (s *clipWorkerLumenStub) GetAvailableModels(context.Context) ([]*client.NodeInfo, error) {
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

type clipWorkerConfigStub struct {
	enabled bool
}

func (s clipWorkerConfigStub) GetEffectiveMLConfig(context.Context) (struct {
	CLIPEnabled    bool
	OCREnabled     bool
	CaptionEnabled bool
	FaceEnabled    bool
}, error) {
	panic("not implemented")
}

func TestProcessClipWorkerSkipsBioClipWhenUnavailable(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	embeddingSvc := &clipWorkerEmbeddingStub{}
	tagSvc := &clipWorkerTagStub{}
	lumenSvc := &clipWorkerLumenStub{
		available: map[string]bool{
			"clip_image_embed":    true,
			"clip_classify":       true,
			"clip_scene_classify": true,
			"bioclip_classify":    false,
		},
		clipLabels: []types.Label{
			{Label: "bird", Score: 0.91},
			{Label: "tree", Score: 0.74},
			{Label: "nature", Score: 0.63},
		},
		sceneLabel: []types.Label{
			{Label: "forest", Score: 0.88},
		},
	}

	worker := &ProcessClipWorker{
		EmbeddingService: embeddingSvc,
		LumenService:     lumenSvc,
		TagService:       tagSvc,
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessClipArgs]{
		Args: ProcessClipArgs{AssetID: assetID, ImageData: []byte("image")},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if embeddingSvc.savedType != service.EmbeddingTypeCLIP {
		t.Fatalf("expected clip embedding type, got %q", embeddingSvc.savedType)
	}

	if len(tagSvc.tags) != 4 {
		t.Fatalf("expected 4 tags, got %d", len(tagSvc.tags))
	}

	if tagSvc.tags[0].Source != "clip_classify" || tagSvc.tags[3].Source != "clip_scene_classify" {
		t.Fatalf("unexpected tag sources: %#v", tagSvc.tags)
	}
}

func TestProcessClipWorkerIncludesBioClipWhenAvailable(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	tagSvc := &clipWorkerTagStub{}
	worker := &ProcessClipWorker{
		EmbeddingService: &clipWorkerEmbeddingStub{},
		LumenService: &clipWorkerLumenStub{
			available: map[string]bool{
				"clip_image_embed":    true,
				"clip_classify":       true,
				"clip_scene_classify": true,
				"bioclip_classify":    true,
			},
			clipLabels: []types.Label{{Label: "bird", Score: 0.9}},
			sceneLabel: []types.Label{{Label: "forest", Score: 0.8}},
			bioLabels:  []types.Label{{Label: "sparrow", Score: 0.7}},
		},
		TagService: tagSvc,
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessClipArgs]{
		Args: ProcessClipArgs{AssetID: assetID, ImageData: []byte("image")},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if len(tagSvc.tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tagSvc.tags))
	}

	if tagSvc.tags[2].Source != "bioclip_classify" {
		t.Fatalf("expected BioCLIP tag to be appended, got %#v", tagSvc.tags[2])
	}
}

func TestProcessClipWorkerSnoozesWithoutPrimaryClassifiers(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	worker := &ProcessClipWorker{
		LumenService: &clipWorkerLumenStub{
			available: map[string]bool{
				"clip_image_embed":    true,
				"clip_classify":       true,
				"clip_scene_classify": false,
			},
		},
		EmbeddingService: &clipWorkerEmbeddingStub{},
		TagService:       &clipWorkerTagStub{},
	}

	err := worker.Work(context.Background(), &river.Job[ProcessClipArgs]{
		Args: ProcessClipArgs{AssetID: assetID, ImageData: []byte("image")},
	})
	if err == nil {
		t.Fatal("expected snooze error")
	}
}
