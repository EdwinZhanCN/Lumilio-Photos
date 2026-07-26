package queue

import (
	"context"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/utils/imagesource"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type bioClipWorkerSpeciesStub struct {
	assetID     uuid.UUID
	predictions []dbtypes.SpeciesPredictionMeta
}

func (s *bioClipWorkerSpeciesStub) SaveSpeciesPredictions(_ context.Context, assetID uuid.UUID, predictions []dbtypes.SpeciesPredictionMeta) error {
	s.assetID = assetID
	s.predictions = append([]dbtypes.SpeciesPredictionMeta(nil), predictions...)
	return nil
}

func TestProcessBioClipWorkerProcessesSpeciesPredictions(t *testing.T) {
	t.Parallel()

	assetID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	speciesSvc := &bioClipWorkerSpeciesStub{}
	imageLoader := &workerImageLoaderStub{data: []byte("image")}
	worker := &ProcessBioClipWorker{
		LumenService: &semanticWorkerLumenStub{
			available: map[string]bool{
				"bioclip_classify": true,
			},
			bioLabels: []types.Label{{Label: "sparrow", Score: 0.7}},
		},
		SpeciesService: speciesSvc,
		ImageLoader:    imageLoader,
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessBioClipArgs]{
		Args: ProcessBioClipArgs{AssetID: assetID},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if imageLoader.purpose != imagesource.PurposeBioClip {
		t.Fatalf("expected BioCLIP image purpose, got %q", imageLoader.purpose)
	}
	if speciesSvc.assetID != assetID {
		t.Fatalf("unexpected species asset id: %#v", speciesSvc.assetID)
	}
	if len(speciesSvc.predictions) != 1 || speciesSvc.predictions[0].Label != "sparrow" || speciesSvc.predictions[0].Score != 0.7 {
		t.Fatalf("unexpected species predictions: %#v", speciesSvc.predictions)
	}
}

func TestProcessBioClipWorkerDoesNotSnoozeWithoutTaskCheck(t *testing.T) {
	t.Parallel()

	assetID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	imageLoader := &workerImageLoaderStub{data: []byte("image")}
	speciesSvc := &bioClipWorkerSpeciesStub{}
	worker := &ProcessBioClipWorker{
		LumenService: &semanticWorkerLumenStub{
			available: map[string]bool{
				"bioclip_classify": false,
			},
			bioLabels: []types.Label{{Label: "sparrow", Score: 0.7}},
		},
		SpeciesService: speciesSvc,
		ImageLoader:    imageLoader,
	}

	err := worker.Work(context.Background(), &river.Job[ProcessBioClipArgs]{
		Args: ProcessBioClipArgs{AssetID: assetID},
	})
	// In the new architecture, IsTaskAvailable is not checked.
	// The worker proceeds to call Infer and should succeed.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(speciesSvc.predictions) != 1 {
		t.Fatalf("expected predictions to be saved without task check")
	}
}
