package queue

import (
	"context"
	"testing"

	"server/internal/service"
	"server/internal/settings"
	"server/internal/utils/imagesource"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

type faceWorkerLumenStub struct {
	available map[string]bool
	result    *types.FaceV1
}

func (s *faceWorkerLumenStub) SemanticTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) SemanticTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) SemanticImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) BioClipClassify(context.Context, *imagesource.MLImage, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) FaceRecognition(context.Context, *imagesource.MLImage) (*types.FaceV1, error) {
	return s.result, nil
}

func (s *faceWorkerLumenStub) OCR(context.Context, *imagesource.MLImage) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *faceWorkerLumenStub) GetAvailableModels(context.Context) ([]*discovery.NodeInfo, error) {
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

func (s *faceWorkerLumenStub) PoolStats() service.PoolStats {
	return service.PoolStats{}
}

func (s *faceWorkerLumenStub) GetNodes() []*discovery.NodeInfo {
	return nil
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

func (faceWorkerConfigStub) GetEffectiveMLConfig(context.Context) (settings.ML, error) {
	return settings.ML{
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
				"face_recognition": true,
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
		ImageLoader:    &workerImageLoaderStub{data: []byte("face-rgb"), encodedSource: []byte("face-source")},
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessFaceArgs]{
		Args: ProcessFaceArgs{
			AssetID: assetID,
		},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if string(faceService.savedImage) != "face-source" {
		t.Fatalf("expected encoded source image to be forwarded")
	}
	if faceService.savedFaces == nil || faceService.savedFaces.ModelID != "face-model" {
		t.Fatalf("expected face result to be forwarded")
	}
}

func TestProcessFaceWorkerDoesNotSnoozeWithoutTaskCheck(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("55555555-5555-5555-5555-555555555555"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	faceService := &faceWorkerFaceServiceStub{}
	worker := &ProcessFaceWorker{
		FaceService: faceService,
		LumenService: &faceWorkerLumenStub{
			available: map[string]bool{
				"face_recognition": false,
			},
			result: &types.FaceV1{ModelID: "face-model", Count: 0},
		},
		ConfigProvider: faceWorkerConfigStub{},
		ImageLoader:    &workerImageLoaderStub{data: []byte("face-rgb"), encodedSource: []byte("face-source")},
	}

	err := worker.Work(context.Background(), &river.Job[ProcessFaceArgs]{
		Args: ProcessFaceArgs{AssetID: assetID},
	})
	// In the new architecture, IsTaskAvailable is not checked.
	// The worker proceeds to call Infer and forward results.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if faceService.savedFaces == nil || faceService.savedFaces.ModelID != "face-model" {
		t.Fatalf("expected face result to be saved even without task check")
	}
}
