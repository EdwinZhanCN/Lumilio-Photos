package queue

import (
	"context"
	"testing"

	"server/config"
	"server/internal/service"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type faceWorkerLumenStub struct {
	available map[string]bool
	result    *types.FaceV1
}

func (s *faceWorkerLumenStub) ClipTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) ClipTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) ClipImageEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) ClipClassify(context.Context, []byte, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) ClipSceneClassify(context.Context, []byte, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) BioClipClassify(context.Context, []byte, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) FaceDetectEmbed(context.Context, []byte) (*types.FaceV1, error) {
	return s.result, nil
}

func (s *faceWorkerLumenStub) OCR(context.Context, []byte) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) VLMCaption(context.Context, []byte) (string, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) VLMCaptionWithPrompt(context.Context, []byte, string) (string, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) VLMCaptionWithMetadata(context.Context, []byte, string) (*types.TextGenerationV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) GetAvailableModels(context.Context) ([]*client.NodeInfo, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) WarmupTasks(context.Context, []string) map[string]bool {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) IsTaskAvailable(taskName string) bool {
	return s.available[taskName]
}

func (s *faceWorkerLumenStub) Start(context.Context) error {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) Close() error {
	panic("not implemented")
}

type faceWorkerFaceServiceStub struct {
	service.FaceService
	savedAssetID pgtype.UUID
	savedImage   []byte
	savedFaces   *types.FaceV1
}

func (s *faceWorkerFaceServiceStub) SaveFaceResults(_ context.Context, assetID pgtype.UUID, faceV1 *types.FaceV1, imageData []byte, _ int) error {
	s.savedAssetID = assetID
	s.savedFaces = faceV1
	s.savedImage = append([]byte(nil), imageData...)
	return nil
}

type faceWorkerConfigStub struct{}

func (faceWorkerConfigStub) GetEffectiveMLConfig(context.Context) (config.MLConfig, error) {
	return config.MLConfig{
		FaceEnabled: true,
	}, nil
}

func TestProcessFaceWorkerPassesImageDataToFaceService(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("44444444-4444-4444-4444-444444444444"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	faceService := &faceWorkerFaceServiceStub{}
	worker := &ProcessFaceWorker{
		FaceService: faceService,
		LumenService: &faceWorkerLumenStub{
			available: map[string]bool{
				"face_detect_and_embed": true,
			},
			result: &types.FaceV1{
				ModelID: "face-model",
				Count:   1,
				Faces: []types.Face{
					{
						BBox:       []float32{0, 0, 100, 100},
						Confidence: 0.97,
					},
				},
			},
		},
		ConfigProvider: faceWorkerConfigStub{},
	}

	imageData := []byte("face-image")
	if err := worker.Work(context.Background(), &river.Job[ProcessFaceArgs]{
		Args: ProcessFaceArgs{
			AssetID:   assetID,
			ImageData: imageData,
		},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if string(faceService.savedImage) != string(imageData) {
		t.Fatalf("expected image data to be forwarded")
	}
	if faceService.savedFaces == nil || faceService.savedFaces.ModelID != "face-model" {
		t.Fatalf("expected face result to be forwarded")
	}
}

func TestProcessFaceWorkerSnoozesWhenTaskUnavailable(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("55555555-5555-5555-5555-555555555555"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	worker := &ProcessFaceWorker{
		FaceService: &faceWorkerFaceServiceStub{},
		LumenService: &faceWorkerLumenStub{
			available: map[string]bool{
				"face_detect_and_embed": false,
			},
			result: &types.FaceV1{},
		},
		ConfigProvider: faceWorkerConfigStub{},
	}

	err := worker.Work(context.Background(), &river.Job[ProcessFaceArgs]{
		Args: ProcessFaceArgs{
			AssetID:   assetID,
			ImageData: []byte("face-image"),
		},
	})
	if err == nil {
		t.Fatal("expected snooze error")
	}
}
