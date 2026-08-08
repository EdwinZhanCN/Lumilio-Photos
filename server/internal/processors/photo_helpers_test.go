package processors

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/utils/imaging"
	"server/internal/utils/phash"
)

type pHashEmbeddingStub struct {
	err        error
	savedType  service.EmbeddingType
	savedModel string
	savedVec   []float32
}

func (s *pHashEmbeddingStub) SaveEmbedding(_ context.Context, _ uuid.UUID, embeddingType service.EmbeddingType, model string, vector []float32, _ bool) error {
	if s.err != nil {
		return s.err
	}
	s.savedType = embeddingType
	s.savedModel = model
	s.savedVec = append([]float32(nil), vector...)
	return nil
}

func (s *pHashEmbeddingStub) SaveVideoFrameEmbeddings(context.Context, uuid.UUID, string, []service.VideoFrameEmbedding) error {
	return nil
}

func (s *pHashEmbeddingStub) SaveAestheticScore(context.Context, uuid.UUID, float32, string) error {
	return nil
}

func (s *pHashEmbeddingStub) GetEmbedding(context.Context, uuid.UUID, service.EmbeddingType, string) (repo.Embedding, error) {
	panic("not implemented")
}

func (s *pHashEmbeddingStub) GetAssetEmbeddingInfo(context.Context, uuid.UUID) (map[service.EmbeddingType]service.EmbeddingInfo, error) {
	panic("not implemented")
}

func (s *pHashEmbeddingStub) DeleteEmbedding(context.Context, uuid.UUID, service.EmbeddingType, string) error {
	panic("not implemented")
}

func (s *pHashEmbeddingStub) ResolveDefaultSearchSpace(context.Context, service.EmbeddingType, string, int) (repo.EmbeddingSpace, error) {
	panic("not implemented")
}

func (s *pHashEmbeddingStub) GetPrimaryEmbeddingVector(context.Context, uuid.UUID, service.EmbeddingType) (service.PrimaryEmbedding, error) {
	panic("not implemented")
}

type thumbnailAssetServiceStub struct {
	service.AssetService

	saved map[string][]byte
}

func (s *thumbnailAssetServiceStub) CreateThumbnail(_ context.Context, _ uuid.UUID, size, thumbnailPath string) (*repo.Thumbnail, error) {
	if s.saved == nil {
		s.saved = make(map[string][]byte)
	}
	s.saved[size] = []byte(thumbnailPath)
	return &repo.Thumbnail{}, nil
}

func TestGenerateThumbnailsStoresInlinePHashAndKeepsLarge(t *testing.T) {
	imaging.StartVips()

	asset := &repo.Asset{
		AssetID:     uuid.New(),
		ContentHash: "asset-hash",
	}
	assetSvc := &thumbnailAssetServiceStub{}
	embedding := &pHashEmbeddingStub{}
	ap := &AssetProcessor{
		assetService:     assetSvc,
		embeddingService: embedding,
	}

	fallback, err := ap.generateThumbnails(context.Background(), bytes.NewReader(testJPEG(t)), openProcessorRepositoryFS(t), asset)
	if err != nil {
		t.Fatalf("generateThumbnails: %v", err)
	}
	if fallback {
		t.Fatal("expected inline pHash success without fallback")
	}

	for _, size := range []string{"small", "medium", "large"} {
		if len(assetSvc.saved[size]) == 0 {
			t.Fatalf("expected %s thumbnail to be saved", size)
		}
	}
	if embedding.savedType != service.EmbeddingTypePHash {
		t.Fatalf("embedding type = %q, want %q", embedding.savedType, service.EmbeddingTypePHash)
	}
}

func TestGenerateThumbnailsFallsBackWhenInlinePHashSaveFails(t *testing.T) {
	imaging.StartVips()

	asset := &repo.Asset{
		AssetID:     uuid.New(),
		ContentHash: "asset-hash",
	}
	ap := &AssetProcessor{
		assetService:     &thumbnailAssetServiceStub{},
		embeddingService: &pHashEmbeddingStub{err: fmt.Errorf("boom")},
	}

	fallback, err := ap.generateThumbnails(context.Background(), bytes.NewReader(testJPEG(t)), openProcessorRepositoryFS(t), asset)
	if err != nil {
		t.Fatalf("generateThumbnails: %v", err)
	}
	if !fallback {
		t.Fatal("expected pHash fallback when SaveEmbedding fails")
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
		RepoID: repositoryID,
		Path:   repositoryPath,
		Status: dbtypes.RepoStatusActive,
		Config: *config,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })
	return files
}

func TestSavePHashEmbeddingFromReaderStoresPHashVector(t *testing.T) {
	imaging.StartVips()

	webp := testSmallWebP(t)
	embedding := &pHashEmbeddingStub{}
	ap := &AssetProcessor{embeddingService: embedding}

	if err := ap.savePHashEmbeddingFromReader(context.Background(), uuid.New(), bytes.NewReader(webp)); err != nil {
		t.Fatalf("savePHashEmbeddingFromReader: %v", err)
	}

	if embedding.savedType != service.EmbeddingTypePHash {
		t.Fatalf("embedding type = %q, want %q", embedding.savedType, service.EmbeddingTypePHash)
	}
	if embedding.savedModel != phash.ModelDCTPHashV1 {
		t.Fatalf("embedding model = %q, want %q", embedding.savedModel, phash.ModelDCTPHashV1)
	}
	if len(embedding.savedVec) != 64 {
		t.Fatalf("embedding vector length = %d, want 64", len(embedding.savedVec))
	}
}

func TestSavePHashEmbeddingFromReaderReturnsSaveError(t *testing.T) {
	imaging.StartVips()

	embedding := &pHashEmbeddingStub{err: fmt.Errorf("boom")}
	ap := &AssetProcessor{embeddingService: embedding}

	if err := ap.savePHashEmbeddingFromReader(context.Background(), uuid.New(), bytes.NewReader(testSmallWebP(t))); err == nil {
		t.Fatal("expected save error")
	}
}

func testSmallWebP(t *testing.T) []byte {
	t.Helper()

	var small bytes.Buffer
	if err := imaging.StreamThumbnails(bytes.NewReader(testJPEG(t)), map[string][2]int{
		"small": {400, 400},
	}, map[string]io.Writer{"small": &small}); err != nil {
		t.Fatalf("create webp thumbnail: %v", err)
	}
	return small.Bytes()
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
