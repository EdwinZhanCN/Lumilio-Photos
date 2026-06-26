package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"server/internal/db/repo"
	"server/internal/utils/imagesource"

	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
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

func (s *semanticTestLumenStub) SemanticTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	s.normalCalls++
	return &types.EmbeddingV1{ModelID: s.modelID, Vector: append([]float32(nil), s.vector...)}, nil
}

func (s *semanticTestLumenStub) SemanticTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	s.fastCalls++
	return &types.EmbeddingV1{ModelID: s.modelID, Vector: append([]float32(nil), s.vector...)}, nil
}

func (s *semanticTestLumenStub) SemanticImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) BioClipClassify(context.Context, *imagesource.MLImage, int) ([]types.Label, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) FaceRecognition(context.Context, *imagesource.MLImage) (*types.FaceV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) OCR(context.Context, *imagesource.MLImage) (*types.OCRV1, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) GetAvailableModels(context.Context) ([]*discovery.NodeInfo, error) {
	panic("not implemented")
}

func (s *semanticTestLumenStub) WarmupTasks(context.Context, []string) map[string]bool {
	panic("not implemented")
}

func (s *semanticTestLumenStub) IsTaskAvailable(taskName string) bool {
	return s.available && taskName == "semantic_text_embed"
}

func (s *semanticTestLumenStub) Start(context.Context) error {
	panic("not implemented")
}

func (s *semanticTestLumenStub) Close() error {
	panic("not implemented")
}

func (s *semanticTestLumenStub) PoolStats() PoolStats {
	return PoolStats{}
}

func (s *semanticTestLumenStub) GetNodes() []*discovery.NodeInfo {
	return nil
}

type semanticTestEmbeddingStub struct {
	resolveFn func(ctx context.Context, embeddingType EmbeddingType, model string, dimensions int) (repo.EmbeddingSpace, error)
}

func (s *semanticTestEmbeddingStub) SaveEmbedding(context.Context, pgtype.UUID, EmbeddingType, string, []float32, bool) error {
	panic("not implemented")
}

func (s *semanticTestEmbeddingStub) SaveAestheticScore(context.Context, pgtype.UUID, float32, string) error {
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

func (s *semanticTestEmbeddingStub) GetPrimaryEmbeddingVector(context.Context, pgtype.UUID, EmbeddingType) (PrimaryEmbedding, error) {
	return PrimaryEmbedding{}, nil
}

func TestResolveSemanticQueryEmbeddingUsesRequestedPath(t *testing.T) {
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

	fastEmbedding, err := svc.resolveSemanticQueryEmbedding(context.Background(), "forest", true)
	if err != nil {
		t.Fatalf("resolveSemanticQueryEmbedding fast returned error: %v", err)
	}
	if lumen.fastCalls != 1 || lumen.normalCalls != 0 {
		t.Fatalf("expected fast path only, got fast=%d normal=%d", lumen.fastCalls, lumen.normalCalls)
	}
	if len(fastEmbedding.Vector) != 3 {
		t.Fatalf("expected fast embedding dimensions to be preserved, got %d", len(fastEmbedding.Vector))
	}

	normalEmbedding, err := svc.resolveSemanticQueryEmbedding(context.Background(), "forest", false)
	if err != nil {
		t.Fatalf("resolveSemanticQueryEmbedding normal returned error: %v", err)
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

func TestBuildSemanticSearchBaseSQLTreatsUnratedAndUnlikedAsEmptyStates(t *testing.T) {
	t.Parallel()

	svc := &assetService{}
	builder := &semanticSQLBuilder{}
	vector := pgvector.NewVector([]float32{0.1, 0.2, 0.3})
	rating := 0
	liked := false
	params := QueryAssetsParams{
		Rating: &rating,
		Liked:  &liked,
	}

	baseSQL, _, err := svc.buildSemanticSearchBaseSQL(builder, params, repo.EmbeddingSpace{
		ID:         42,
		Dimensions: 768,
	}, &vector)
	if err != nil {
		t.Fatalf("buildSemanticSearchBaseSQL returned error: %v", err)
	}

	if !strings.Contains(baseSQL, "(a.rating IS NULL OR a.rating = 0)") {
		t.Fatalf("expected unrated filter to include NULL and zero ratings, got:\n%s", baseSQL)
	}
	if !strings.Contains(baseSQL, "(a.liked IS NULL OR a.liked = false)") {
		t.Fatalf("expected unliked filter to include NULL and false liked states, got:\n%s", baseSQL)
	}
}

func TestNormalizeSearchAssetsParamsCapsTopResultsAt200(t *testing.T) {
	t.Parallel()

	defaulted := normalizeSearchAssetsParams(SearchAssetsParams{})
	if defaulted.TopResultsLimit != 200 {
		t.Fatalf("expected default top results limit 200, got %d", defaulted.TopResultsLimit)
	}

	capped := normalizeSearchAssetsParams(SearchAssetsParams{TopResultsLimit: 500})
	if capped.TopResultsLimit != 200 {
		t.Fatalf("expected top results limit to cap at 200, got %d", capped.TopResultsLimit)
	}

	preserved := normalizeSearchAssetsParams(SearchAssetsParams{TopResultsLimit: 199})
	if preserved.TopResultsLimit != 199 {
		t.Fatalf("expected top results limit 199 to be preserved, got %d", preserved.TopResultsLimit)
	}
}

func TestBuildSemanticSearchBaseSQLDoesNotApplyHardDistanceThreshold(t *testing.T) {
	t.Parallel()

	svc := &assetService{}
	builder := &semanticSQLBuilder{}
	vector := pgvector.NewVector([]float32{0.1, 0.2, 0.3})

	baseSQL, _, err := svc.buildSemanticSearchBaseSQL(builder, QueryAssetsParams{}, repo.EmbeddingSpace{
		ID:         42,
		Dimensions: 768,
	}, &vector)
	if err != nil {
		t.Fatalf("buildSemanticSearchBaseSQL returned error: %v", err)
	}

	if strings.Contains(baseSQL, "<->") {
		t.Fatalf("semantic search SQL should rank by distance without filtering by distance, got:\n%s", baseSQL)
	}
	if len(builder.args) != 3 {
		t.Fatalf("expected vector, space, and default trash-state arguments, got %d", len(builder.args))
	}
	if builder.args[2] != false {
		t.Fatalf("expected semantic search to default to non-trash assets, got %v", builder.args[2])
	}
}
