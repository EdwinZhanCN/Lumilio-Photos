package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"server/internal/classify"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

// semanticTextEmbedTask is the Lumen task name required to build prototypes and
// run preview classification.
const semanticTextEmbedTask = "semantic_text_embed"

// classifierCacheTTL bounds how long enabled definitions (with prototypes) are
// cached in-process, so the per-asset worker doesn't hit the DB every job.
const classifierCacheTTL = 60 * time.Second

// defaultBackgroundPrompts build the generic "background" prototype — the
// "not this class" side of the zero-shot binary decision (argmax over
// {positive, background}). A classifier with no explicit negative prompts is
// scored against this.
var defaultBackgroundPrompts = []string{
	"a photo",
	"an image",
	"a picture",
	"a random photograph",
}

// ClassifierDefinition is a smart-album recipe plus its cached prototype vectors.
type ClassifierDefinition struct {
	ID                  int32
	Slug                string
	DisplayName         string
	TagName             string
	Category            string
	PositivePrompts     []string
	NegativePrompts     []string
	Threshold           float64
	Enabled             bool
	PositivePrototype   []float32
	NegativePrototype   []float32
	PrototypeModel      string
	PrototypeDimensions int
}

// ClassifierHit is a single classifier that matched an asset.
type ClassifierHit struct {
	Slug       string
	TagName    string
	Category   string
	Score      float64
	Confidence float64
}

// ClassifierPreviewMatch is one ranked asset from a preview run.
type ClassifierPreviewMatch struct {
	AssetID uuid.UUID
	Score   float64
}

// ClassifierService runs zero-shot classification: it turns prompt
// ensembles into cached prototype vectors, scores stored image embeddings
// against them, and powers a real-time preview over the library.
type ClassifierService interface {
	// EnsurePrototypes builds/refreshes cached prototypes for enabled classifiers
	// against the current semantic text model. Best-effort; no-op when ML is down.
	EnsurePrototypes(ctx context.Context) error
	// Classify scores a stored image embedding against enabled classifiers.
	Classify(ctx context.Context, embedding PrimaryEmbedding) ([]ClassifierHit, error)
	// Preview embeds ad-hoc prompts and returns library assets above the threshold.
	Preview(ctx context.Context, positivePrompts, negativePrompts []string, threshold float64, limit int) ([]ClassifierPreviewMatch, error)
}

type classifierService struct {
	pool       *pgxpool.Pool
	lumen      LumenService
	embeddings EmbeddingService
	logger     *zap.Logger

	mu            sync.Mutex
	cache         []ClassifierDefinition
	cacheExpires  time.Time
	background    []float32
	backgroundDim int
}

func NewClassifierService(pool *pgxpool.Pool, lumen LumenService, embeddings EmbeddingService, logger *zap.Logger) ClassifierService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &classifierService{
		pool:       pool,
		lumen:      lumen,
		embeddings: embeddings,
		logger:     logger,
	}
}

func (s *classifierService) textEmbedReady() bool {
	return s.lumen != nil && s.lumen.IsTaskAvailable(semanticTextEmbedTask)
}

// buildPrototype embeds each prompt with semantic and ensembles them into a single
// unit prototype vector, returning the shared model id.
func (s *classifierService) buildPrototype(ctx context.Context, prompts []string) ([]float32, string, error) {
	if len(prompts) == 0 {
		return nil, "", fmt.Errorf("no prompts")
	}
	vectors := make([][]float32, 0, len(prompts))
	model := ""
	for _, p := range prompts {
		emb, err := s.lumen.SemanticTextEmbed(ctx, []byte(p))
		if err != nil {
			return nil, "", fmt.Errorf("embed prompt %q: %w", p, err)
		}
		if len(emb.Vector) == 0 {
			return nil, "", fmt.Errorf("empty embedding for prompt %q", p)
		}
		if model == "" {
			model = emb.ModelID
		} else if model != emb.ModelID {
			return nil, "", fmt.Errorf("prompt model mismatch: %s != %s", model, emb.ModelID)
		}
		vectors = append(vectors, emb.Vector)
	}
	proto, err := classify.EnsemblePrototype(vectors)
	if err != nil {
		return nil, "", err
	}
	return proto, model, nil
}

func (s *classifierService) EnsurePrototypes(ctx context.Context) error {
	if !s.textEmbedReady() {
		s.logger.Info("zero-shot classifier: text embed task unavailable, skipping prototype build")
		return nil
	}

	// Background prototype = the "not this class" side of the decision. Built by
	// prompt-ensembling generic prompts (the zero-shot recipe), shared by every
	// classifier that defines no explicit negative prompts.
	background, currentModel, err := s.buildPrototype(ctx, defaultBackgroundPrompts)
	if err != nil {
		return fmt.Errorf("build background prototype: %w", err)
	}
	s.mu.Lock()
	s.background = background
	s.backgroundDim = len(background)
	s.mu.Unlock()

	defs, err := s.loadDefinitions(ctx, false)
	if err != nil {
		return err
	}

	for _, def := range defs {
		if def.PrototypeModel == currentModel && len(def.PositivePrototype) > 0 {
			continue // already current
		}
		pos, model, err := s.buildPrototype(ctx, def.PositivePrompts)
		if err != nil {
			s.logger.Warn("zero-shot classifier: build positive prototype failed", zap.String("slug", def.Slug), zap.Error(err))
			continue
		}
		var neg []float32
		if len(def.NegativePrompts) > 0 {
			neg, _, err = s.buildPrototype(ctx, def.NegativePrompts)
			if err != nil {
				s.logger.Warn("zero-shot classifier: build negative prototype failed", zap.String("slug", def.Slug), zap.Error(err))
				neg = nil
			}
		}
		if err := s.savePrototypes(ctx, def.ID, pos, neg, model); err != nil {
			s.logger.Warn("zero-shot classifier: persist prototype failed", zap.String("slug", def.Slug), zap.Error(err))
			continue
		}
		s.logger.Info("zero-shot classifier: built prototype", zap.String("slug", def.Slug), zap.String("model", model), zap.Int("dim", len(pos)))
	}

	s.invalidateCache()
	return nil
}

func (s *classifierService) Classify(ctx context.Context, embedding PrimaryEmbedding) ([]ClassifierHit, error) {
	if len(embedding.Vector) == 0 {
		return nil, nil
	}
	defs, err := s.enabledWithPrototypes(ctx)
	if err != nil {
		return nil, err
	}
	background := s.backgroundFor(len(embedding.Vector))

	hits := make([]ClassifierHit, 0, len(defs))
	for _, def := range defs {
		// Cross-model guard: a prototype is only comparable to embeddings produced
		// by the same model. Matching dimensionality across different models does
		// not imply a shared vector space, so a mismatched score is meaningless.
		// This skips stale prototypes after a model switch until they are rebuilt.
		if def.PrototypeModel != embedding.Model {
			s.logger.Debug("zero-shot classifier: model mismatch, skipping",
				zap.String("slug", def.Slug),
				zap.String("proto_model", def.PrototypeModel),
				zap.String("asset_model", embedding.Model))
			continue
		}
		if def.PrototypeDimensions != len(embedding.Vector) {
			s.logger.Debug("zero-shot classifier: dimension mismatch, skipping",
				zap.String("slug", def.Slug),
				zap.Int("proto_dim", def.PrototypeDimensions),
				zap.Int("asset_dim", len(embedding.Vector)))
			continue
		}
		// Zero-shot binary decision: the positive prototype must beat the
		// negative/background prototype (argmax over {positive, background}).
		// def.Threshold is the relative margin to clear — 0 is pure argmax.
		negative := def.NegativePrototype
		if len(negative) == 0 {
			negative = background
		}
		score := classify.ContrastiveScore(embedding.Vector, def.PositivePrototype, negative)
		if score < def.Threshold {
			continue
		}
		hits = append(hits, ClassifierHit{
			Slug:       def.Slug,
			TagName:    def.TagName,
			Category:   def.Category,
			Score:      score,
			Confidence: classify.ScoreToConfidence(score, def.Threshold),
		})
	}
	return hits, nil
}

// backgroundFor returns the cached background prototype when its dimensionality
// matches the asset embedding, else nil (degrades to plain positive cosine).
func (s *classifierService) backgroundFor(dim int) []float32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backgroundDim == dim {
		return s.background
	}
	return nil
}

// Preview embeds ad-hoc prompts and returns library assets whose contrastive
// margin (positive cosine − negative/background cosine) clears threshold — the
// same zero-shot binary decision Classify uses. If no negative prompts are
// given, the generic background prototype is used.
func (s *classifierService) Preview(ctx context.Context, positivePrompts, negativePrompts []string, threshold float64, limit int) ([]ClassifierPreviewMatch, error) {
	if !s.textEmbedReady() {
		return nil, fmt.Errorf("%w: semantic text embedding unavailable", ErrSemanticSearchUnavailable)
	}
	if len(positivePrompts) == 0 {
		return nil, fmt.Errorf("at least one positive prompt is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	positive, model, err := s.buildPrototype(ctx, positivePrompts)
	if err != nil {
		return nil, err
	}
	negativePrompts2 := negativePrompts
	if len(negativePrompts2) == 0 {
		negativePrompts2 = defaultBackgroundPrompts
	}
	negative, _, err := s.buildPrototype(ctx, negativePrompts2)
	if err != nil {
		return nil, err
	}

	space, err := s.embeddings.ResolveDefaultSearchSpace(ctx, EmbeddingTypeSemantic, model, len(positive))
	if err != nil {
		return nil, err
	}

	// Embeddings are unit vectors, so cosine = 1 - d^2/2. The score is the
	// contrastive margin: cos(positive) - cos(negative); membership is margin >= threshold.
	posVec := pgvector.NewVector(positive)
	negVec := pgvector.NewVector(negative)
	marginExpr := fmt.Sprintf(
		"((1 - power(e.vector::vector(%d) <-> $1::vector(%d), 2) / 2) - (1 - power(e.vector::vector(%d) <-> $2::vector(%d), 2) / 2))",
		space.Dimensions, space.Dimensions, space.Dimensions, space.Dimensions,
	)
	query := fmt.Sprintf(`
SELECT a.asset_id, %s::float8 AS score
FROM embeddings e
JOIN assets a ON a.asset_id = e.asset_id
WHERE e.space_id = $3
  AND e.is_primary = true
  AND a.is_deleted = false
  AND %s >= $4
ORDER BY score DESC, a.asset_id DESC
LIMIT $5
`, marginExpr, marginExpr)

	rows, err := s.pool.Query(ctx, query, &posVec, &negVec, space.ID, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("preview query: %w", err)
	}
	defer rows.Close()

	matches := make([]ClassifierPreviewMatch, 0, limit)
	for rows.Next() {
		var assetID pgtype.UUID
		var score float64
		if err := rows.Scan(&assetID, &score); err != nil {
			return nil, fmt.Errorf("scan preview row: %w", err)
		}
		if !assetID.Valid {
			continue
		}
		matches = append(matches, ClassifierPreviewMatch{AssetID: uuid.UUID(assetID.Bytes), Score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate preview rows: %w", err)
	}
	return matches, nil
}

func (s *classifierService) invalidateCache() {
	s.mu.Lock()
	s.cache = nil
	s.cacheExpires = time.Time{}
	s.mu.Unlock()
}

func (s *classifierService) enabledWithPrototypes(ctx context.Context) ([]ClassifierDefinition, error) {
	s.mu.Lock()
	if s.cache != nil && time.Now().Before(s.cacheExpires) {
		cached := s.cache
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	defs, err := s.loadDefinitions(ctx, true)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache = defs
	s.cacheExpires = time.Now().Add(classifierCacheTTL)
	s.mu.Unlock()
	return defs, nil
}

// loadDefinitions reads classifier rows. When requirePrototype is true only rows
// with a built positive prototype are returned (the set the worker scores).
func (s *classifierService) loadDefinitions(ctx context.Context, requirePrototype bool) ([]ClassifierDefinition, error) {
	where := "enabled = true"
	if requirePrototype {
		where += " AND positive_prototype IS NOT NULL"
	}
	query := fmt.Sprintf(`
SELECT id, slug, display_name, tag_name, category, positive_prompts, negative_prompts,
       threshold::float8, enabled, positive_prototype, negative_prototype,
       prototype_model, prototype_dimensions
FROM classifier_definitions
WHERE %s
ORDER BY id
`, where)

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load classifier definitions: %w", err)
	}
	defer rows.Close()

	defs := make([]ClassifierDefinition, 0)
	for rows.Next() {
		var (
			def       ClassifierDefinition
			pos       *pgvector.Vector
			neg       *pgvector.Vector
			model     *string
			dims      *int32
			threshold float64
			category  string
		)
		if err := rows.Scan(
			&def.ID, &def.Slug, &def.DisplayName, &def.TagName, &category,
			&def.PositivePrompts, &def.NegativePrompts, &threshold, &def.Enabled,
			&pos, &neg, &model, &dims,
		); err != nil {
			return nil, fmt.Errorf("scan classifier definition: %w", err)
		}
		def.Category = category
		def.Threshold = threshold
		if pos != nil {
			def.PositivePrototype = pos.Slice()
		}
		if neg != nil {
			def.NegativePrototype = neg.Slice()
		}
		if model != nil {
			def.PrototypeModel = *model
		}
		if dims != nil {
			def.PrototypeDimensions = int(*dims)
		}
		defs = append(defs, def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate classifier definitions: %w", err)
	}
	return defs, nil
}

func (s *classifierService) savePrototypes(ctx context.Context, id int32, positive, negative []float32, model string) error {
	posVec := pgvector.NewVector(positive)
	var negArg any
	if len(negative) > 0 {
		negVec := pgvector.NewVector(negative)
		negArg = &negVec
	}
	_, err := s.pool.Exec(ctx, `
UPDATE classifier_definitions
SET positive_prototype = $2,
    negative_prototype = $3,
    prototype_model = $4,
    prototype_dimensions = $5,
    prototype_built_at = NOW(),
    updated_at = NOW()
WHERE id = $1
`, id, &posVec, negArg, model, int32(len(positive)))
	if err != nil {
		return fmt.Errorf("save prototypes: %w", err)
	}
	return nil
}
