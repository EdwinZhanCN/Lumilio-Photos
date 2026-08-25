package processors

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/utils/imaging"
)

type thumbnailAssetServiceStub struct {
	service.AssetService

	saved map[string][]byte
}

func (s *thumbnailAssetServiceStub) CreateThumbnail(_ context.Context, _, _ uuid.UUID, size, thumbnailPath string) (*repo.Thumbnail, error) {
	if s.saved == nil {
		s.saved = make(map[string][]byte)
	}
	s.saved[size] = []byte(thumbnailPath)
	return &repo.Thumbnail{}, nil
}

func TestGenerateThumbnailsDefersPHashAndKeepsAllSizes(t *testing.T) {
	imaging.StartVips()

	asset := &repo.Asset{
		AssetID:   uuid.New(),
		ContentID: uuid.New(),
	}
	assetSvc := &thumbnailAssetServiceStub{}
	ap := &AssetProcessor{
		assetService: assetSvc,
	}

	queuePHash, err := ap.generateThumbnails(context.Background(), bytes.NewReader(testJPEG(t)), openProcessorRepositoryFS(t), asset)
	if err != nil {
		t.Fatalf("generateThumbnails: %v", err)
	}
	if !queuePHash {
		t.Fatal("successful photo thumbnails must enqueue the dedicated pHash worker")
	}

	for _, size := range []string{"small", "medium", "large"} {
		if len(assetSvc.saved[size]) == 0 {
			t.Fatalf("expected %s thumbnail to be saved", size)
		}
	}
}

func openProcessorRepositoryFS(t *testing.T) *storage.RepositoryFS {
	t.Helper()
	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	config := repocfg.NewRepositoryConfig("processor test")
	config.ID = repositoryID.String()
	if err := config.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	files, err := storage.NewRepositoryFSFactory(nil, nil).Open(repo.Repository{
		RepoID:       repositoryID,
		Path:         repositoryPath,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Config:       *config,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })
	return files
}

func testJPEG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return jpegBuf.Bytes()
}

func stringPtr(s string) *string {
	return &s
}
