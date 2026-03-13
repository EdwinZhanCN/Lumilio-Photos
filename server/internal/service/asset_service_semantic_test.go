package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"server/internal/db/repo"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

type semanticTestLumenStub struct {
	fastCalls   int
	normalCalls int
	available   bool
	modelID     string
	vector      []float32
}

func (s *semanticTestLumenStub) ClipTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	s.normalCalls++
	return &types.EmbeddingV1{ModelID: s.modelID, Vector: append([]float32(nil), s.vector...)}, nil
}

func (s *semanticTestLumenStub) ClipTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	s.fastCalls++
	return &types.EmbeddingV1{ModelID: s.modelID, Vector: append([]float32(nil), s.vector...)}, nil
}

func (s *semanticTestLumenStub) ClipImageEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) ClipClassify(context.Context, []byte, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) ClipSceneClassify(context.Context, []byte, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) BioClipClassify(context.Context, []byte, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) FaceDetectEmbed(context.Context, []byte) (*types.FaceV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) OCR(context.Context, []byte) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) VLMCaption(context.Context, []byte) (string, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) VLMCaptionWithPrompt(context.Context, []byte, string) (string, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) VLMCaptionWithMetadata(context.Context, []byte, string) (*types.TextGenerationV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) GetAvailableModels(context.Context) ([]*client.NodeInfo, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) WarmupTasks(context.Context, []string) map[string]bool {
	panic("not implemented")
}

func (s *semanticTestLumenStub) IsTaskAvailable(taskName string) bool {
	return s.available && taskName == "clip_text_embed"
}

func (s *semanticTestLumenStub) Start(context.Context) error {
	panic("not implemented")
}

func (s *semanticTestLumenStub) Close() error {
	panic("not implemented")
}

type semanticTestEmbeddingStub struct {
	resolveFn func(ctx context.Context, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error)
}

func (s *semanticTestEmbeddingStub) SaveEmbedding(context.Context, pgtype.UUID, EmbeddingType, string, []float32, bool) error {
	panic("not implemented")
}

func (s *semanticTestEmbeddingStub) GetEmbedding(context.Context, pgtype.UUID, EmbeddingType, string) (repo.Embedding, error) {
	panic("not implemented")
}

func (s *semanticTestEmbeddingStub) GetAssetEmbeddingInfo(context.Context, pgtype.UUID) (map[EmbeddingType]EmbeddingInfo, error) {
	panic("not implemented")
}

func (s *semanticTestEmbeddingStub) DeleteEmbedding(context.Context, pgtype.UUID, EmbeddingType, string) error {
	panic("not implemented")
}

func (s *semanticTestEmbeddingStub) ResolveDefaultSearchSpace(ctx context.Context, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error) {
	return s.resolveFn(ctx, embeddingType, model, dimensions)
}

func TestResolveClipQueryEmbeddingUsesRequestedPath(t *testing.T) {
	t.Parallel()

	lumen := &semanticTestLumenStub{
		available: true,
		modelID:   "CN-CLIP_ViT-L-14_onnx",
		vector:    []float32{0.1, 0.2, 0.3},
	}
	svc := &assetService{
		lumen:            lumen,
		embeddingService: &semanticTestEmbeddingStub{},
	}

	fastEmbedding, err := svc.resolveClipQueryEmbedding(context.Background(), "forest", true)
	if err != nil {
		t.Fatalf("resolveClipQueryEmbedding fast returned error: %v", err)
	}
	if lumen.fastCalls != 1 || lumen.normalCalls != 0 {
		t.Fatalf("expected fast path only, got fast=%d normal=%d", lumen.fastCalls, lumen.normalCalls)
	}
	if len(fastEmbedding.Vector) != 3 {
		t.Fatalf("expected fast embedding dimensions to be preserved, got %d", len(fastEmbedding.Vector))
	}

	normalEmbedding, err := svc.resolveClipQueryEmbedding(context.Background(), "forest", false)
	if err != nil {
		t.Fatalf("resolveClipQueryEmbedding normal returned error: %v", err)
	}
	if lumen.fastCalls != 1 || lumen.normalCalls != 1 {
		t.Fatalf("expected one fast and one normal call, got fast=%d normal=%d", lumen.fastCalls, lumen.normalCalls)
	}
	if normalEmbedding.ModelID != "CN-CLIP_ViT-L-14_onnx" {
		t.Fatalf("unexpected model id: %s", normalEmbedding.ModelID)
	}
}

func TestQueryAssetsVectorReturnsSemanticUnavailableOnSpaceMismatch(t *testing.T) {
	t.Parallel()

	mismatchErr := errors.Join(ErrSemanticSearchUnavailable, errors.New("space mismatch"))
	svc := &assetService{
		lumen: &semanticTestLumenStub{
			available: true,
			modelID:   "query-model",
			vector:    []float32{0.1, 0.2},
		},
		embeddingService: &semanticTestEmbeddingStub{
			resolveFn: func(context.Context, EmbeddingType, string, int) (repo.EmbeddingSpace, error) {
				return repo.EmbeddingSpace{}, mismatchErr
			},
		},
	}

	_, _, err := svc.queryAssetsVector(context.Background(), QueryAssetsParams{
		Query: "sunset",
		Limit: 10,
	})
	if err == nil {
		t.Fatal("expected semantic search error")
	}
	if !errors.Is(err, ErrSemanticSearchUnavailable) {
		t.Fatalf("expected ErrSemanticSearchUnavailable, got %v", err)
	}
}

func TestBuildSemanticSearchBaseSQLUsesSpaceIsolation(t *testing.T) {
	t.Parallel()

	svc := &assetService{}
	builder := &semanticSQLBuilder{}
	vector := pgvector.NewVector([]float32{0.1, 0.2, 0.3})
	assetType := AssetTypePhoto
	cameraModel := "X100VI"
	params := QueryAssetsParams{
		AssetType:   &assetType,
		AssetTypes:  []string{AssetTypePhoto, AssetTypeVideo},
		CameraModel: &cameraModel,
	}

	baseSQL, distanceExpr, err := svc.buildSemanticSearchBaseSQL(builder, params, repo.EmbeddingSpace{
		ID:         42,
		Dimensions: 768,
	}, &vector)
	if err != nil {
		t.Fatalf("buildSemanticSearchBaseSQL returned error: %v", err)
	}

	if !strings.Contains(baseSQL, "e.space_id = $2") {
		t.Fatalf("expected search SQL to filter by space_id, got:\n%s", baseSQL)
	}
	if strings.Contains(baseSQL, "e.embedding_type") {
		t.Fatalf("search SQL should not filter by embedding_type anymore, got:\n%s", baseSQL)
	}
	if !strings.Contains(distanceExpr, "vector(768)") {
		t.Fatalf("expected distance expression to cast to vector(768), got %s", distanceExpr)
	}
	if !strings.Contains(baseSQL, "a.type = ANY(") {
		t.Fatalf("expected asset type array filter to be preserved, got:\n%s", baseSQL)
	}
	if !strings.Contains(baseSQL, "a.specific_metadata->>'camera_model'") {
		t.Fatalf("expected camera model filter to be preserved, got:\n%s", baseSQL)
	}
}

func TestSemanticMaxDistanceHonorsEnvironment(t *testing.T) {
	const key = "SEMANTIC_MAX_DISTANCE"
	t.Setenv(key, "0.33")
	if got := semanticMaxDistance(); got != 0.33 {
		t.Fatalf("expected semantic max distance 0.33, got %v", got)
	}
}
