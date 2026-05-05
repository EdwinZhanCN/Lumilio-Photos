package queue

import (
	"context"
	"testing"

	"server/internal/utils/imagesource"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

func TestProcessBioClipWorkerProcessesBioClipTags(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	tagSvc := &clipWorkerTagStub{}
	imageLoader := &workerImageLoaderStub{data: []byte("image")}
	worker := &ProcessBioClipWorker{
		LumenService: &clipWorkerLumenStub{
			available: map[string]bool{
				"bioclip_classify": true,
			},
			bioLabels: []types.Label{{Label: "sparrow", Score: 0.7}},
		},
		TagService:  tagSvc,
		ImageLoader: imageLoader,
	}

	if err := worker.Work(context.Background(), &river.Job[ProcessBioClipArgs]{
		Args: ProcessBioClipArgs{AssetID: assetID},
	}); err != nil {
		t.Fatalf("worker returned error: %v", err)
	}

	if imageLoader.purpose != imagesource.PurposeBioClip {
		t.Fatalf("expected BioCLIP image purpose, got %q", imageLoader.purpose)
	}
	if len(tagSvc.tags) != 1 || tagSvc.tags[0].Source != "bioclip_classify" {
		t.Fatalf("unexpected BioCLIP tags: %#v", tagSvc.tags)
	}
	if len(tagSvc.sources) != 1 || tagSvc.sources[0] != "bioclip_classify" {
		t.Fatalf("unexpected replacement sources: %#v", tagSvc.sources)
	}
}

func TestProcessBioClipWorkerSnoozesWhenUnavailable(t *testing.T) {
	t.Parallel()

	assetID := pgtype.UUID{}
	if err := assetID.Scan("33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("scan asset id: %v", err)
	}

	imageLoader := &workerImageLoaderStub{data: []byte("image")}
	worker := &ProcessBioClipWorker{
		LumenService: &clipWorkerLumenStub{
			available: map[string]bool{
				"bioclip_classify": false,
			},
		},
		TagService:  &clipWorkerTagStub{},
		ImageLoader: imageLoader,
	}

	err := worker.Work(context.Background(), &river.Job[ProcessBioClipArgs]{
		Args: ProcessBioClipArgs{AssetID: assetID},
	})
	if err == nil {
		t.Fatal("expected snooze error")
	}
	if imageLoader.called != 0 {
		t.Fatalf("expected image loader not to be called while task is unavailable, got %d calls", imageLoader.called)
	}
}
