package search

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	DefaultRRFK                = 60
	DefaultCandidatePoolMin    = 50
	DefaultCandidatePoolMax    = 1000
	DefaultCandidateMultiplier = 4
)

type Service interface {
	Search(ctx context.Context, req Request) (Response, error)
}

type AggregateService struct {
	pool       *pgxpool.Pool
	retrievers []Retriever
	logger     *zap.Logger
	rrfK       float64
}

func NewAggregateService(pool *pgxpool.Pool, retrievers []Retriever, logger *zap.Logger) *AggregateService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AggregateService{
		pool:       pool,
		retrievers: retrievers,
		logger:     logger,
		rrfK:       DefaultRRFK,
	}
}

func (s *AggregateService) Search(ctx context.Context, req Request) (Response, error) {
	if s == nil || s.pool == nil {
		return Response{}, fmt.Errorf("aggregate search service is not configured")
	}
	if !hasTextQuery(req) {
		return Response{}, ErrEmptyQuery
	}
	req.Query = strings.TrimSpace(req.Query)
	req.Limit = normalizeLimit(req.Limit)
	req.Offset = normalizeOffset(req.Offset)
	req.TopK = normalizeTopK(req.TopK, req.Limit, req.Offset)

	started := time.Now()
	type retrieverResult struct {
		source     string
		weight     float64
		candidates []Candidate
		duration   time.Duration
		err        error
	}

	results := make(chan retrieverResult, len(s.retrievers))
	var wg sync.WaitGroup
	for _, retriever := range s.retrievers {
		if retriever == nil {
			continue
		}
		wg.Add(1)
		go func(r Retriever) {
			defer wg.Done()
			sourceStart := time.Now()
			candidates, err := r.Retrieve(ctx, req)
			results <- retrieverResult{
				source:     r.Source(),
				weight:     r.Weight(),
				candidates: candidates,
				duration:   time.Since(sourceStart),
				err:        err,
			}
		}(retriever)
	}
	wg.Wait()
	close(results)

	sourceMetas := make([]SourceMeta, 0, len(s.retrievers))
	successes := 0
	failures := []error{}
	allCandidates := make([]Candidate, 0)
	weights := make(map[string]float64)
	successfulSources := make(map[string]struct{})
	for result := range results {
		meta := SourceMeta{
			Type:           result.source,
			Weight:         result.weight,
			CandidateCount: len(result.candidates),
			Duration:       result.duration,
			DurationMs:     result.duration.Milliseconds(),
		}
		weights[result.source] = result.weight
		if result.err != nil {
			meta.Error = result.err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", result.source, result.err))
			s.logger.Warn("aggregate search retriever failed",
				zap.String("source", result.source),
				zap.Duration("duration", result.duration),
				zap.Error(result.err),
			)
		} else {
			successes++
			successfulSources[result.source] = struct{}{}
			allCandidates = append(allCandidates, result.candidates...)
			s.logger.Debug("aggregate search retriever completed",
				zap.String("source", result.source),
				zap.Int("candidates", len(result.candidates)),
				zap.Duration("duration", result.duration),
			)
		}
		sourceMetas = append(sourceMetas, meta)
	}
	sort.Slice(sourceMetas, func(i, j int) bool {
		return sourceMetas[i].Type < sourceMetas[j].Type
	})

	if successes == 0 {
		return Response{Sources: sourceMetas}, fmt.Errorf("aggregate search failed: %w", errors.Join(failures...))
	}

	fused := fuseWeightedRRF(allCandidates, weights, s.rrfK)
	totalCandidates := len(fused)
	if req.CountTotal {
		total, err := s.countTotalCandidates(ctx, req, successfulSources)
		if err != nil {
			return Response{Sources: sourceMetas, TotalCandidates: totalCandidates, CandidatePoolSize: req.TopK}, err
		}
		totalCandidates = total
	}
	page := pageRanked(fused, req.Limit, req.Offset)
	rankedIDs := make([]uuid.UUID, 0, len(page))
	for _, item := range page {
		rankedIDs = append(rankedIDs, item.assetID)
	}

	assets, err := HydrateAssets(ctx, s.pool, rankedIDs)
	if err != nil {
		return Response{Sources: sourceMetas, TotalCandidates: totalCandidates, CandidatePoolSize: req.TopK}, err
	}

	response := Response{
		Assets:            assets,
		TotalCandidates:   totalCandidates,
		CandidatePoolSize: req.TopK,
		Sources:           sourceMetas,
	}
	if req.Debug {
		response.Debug = buildDebug(page)
	}

	s.logger.Info("aggregate search completed",
		zap.Int("assets", len(assets)),
		zap.Int("candidates", totalCandidates),
		zap.Int("top_k", req.TopK),
		zap.Int("successes", successes),
		zap.Int("failures", len(failures)),
		zap.Duration("duration", time.Since(started)),
	)

	return response, nil
}

type countQueryRetriever interface {
	Retriever
	CountQuery(ctx context.Context, builder *sqlBuilder, req Request) (string, error)
}

func (s *AggregateService) countTotalCandidates(ctx context.Context, req Request, successfulSources map[string]struct{}) (int, error) {
	builder := &sqlBuilder{}
	subqueries := []string{}
	for _, retriever := range s.retrievers {
		if retriever == nil {
			continue
		}
		if _, ok := successfulSources[retriever.Source()]; !ok {
			continue
		}
		countRetriever, ok := retriever.(countQueryRetriever)
		if !ok {
			continue
		}
		subquery, err := countRetriever.CountQuery(ctx, builder, req)
		if err != nil {
			return 0, fmt.Errorf("%s count: %w", retriever.Source(), err)
		}
		subqueries = append(subqueries, subquery)
	}
	if len(subqueries) == 0 {
		return 0, nil
	}

	query := fmt.Sprintf(`
SELECT COUNT(DISTINCT asset_id)
FROM (
  %s
) aggregate_candidates
`, strings.Join(subqueries, "\nUNION\n"))

	var total int64
	if err := s.pool.QueryRow(ctx, query, builder.args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count aggregate candidates: %w", err)
	}
	return int(total), nil
}

type fusedCandidate struct {
	assetID       uuid.UUID
	score         float64
	contributions map[string]Contribution
	bestRank      int
}

func fuseWeightedRRF(candidates []Candidate, weights map[string]float64, rrfK float64) []fusedCandidate {
	byAsset := make(map[uuid.UUID]*fusedCandidate)
	for _, candidate := range candidates {
		if candidate.AssetID == uuid.Nil || candidate.Rank <= 0 {
			continue
		}
		weight := weights[candidate.Source]
		if weight <= 0 {
			weight = 1
		}
		rrfScore := weight / (rrfK + float64(candidate.Rank))
		item, ok := byAsset[candidate.AssetID]
		if !ok {
			item = &fusedCandidate{
				assetID:       candidate.AssetID,
				contributions: make(map[string]Contribution),
				bestRank:      candidate.Rank,
			}
			byAsset[candidate.AssetID] = item
		}
		if existing, exists := item.contributions[candidate.Source]; exists && existing.Rank <= candidate.Rank {
			continue
		}
		if existing, exists := item.contributions[candidate.Source]; exists {
			item.score -= existing.RRFScore
		}
		item.score += rrfScore
		item.contributions[candidate.Source] = Contribution{
			Rank:     candidate.Rank,
			Weight:   weight,
			RRFScore: rrfScore,
			RawScore: candidate.RawScore,
		}
		if candidate.Rank < item.bestRank {
			item.bestRank = candidate.Rank
		}
	}

	fused := make([]fusedCandidate, 0, len(byAsset))
	for _, item := range byAsset {
		fused = append(fused, *item)
	}
	sort.Slice(fused, func(i, j int) bool {
		if fused[i].score == fused[j].score {
			if fused[i].bestRank == fused[j].bestRank {
				return fused[i].assetID.String() > fused[j].assetID.String()
			}
			return fused[i].bestRank < fused[j].bestRank
		}
		return fused[i].score > fused[j].score
	})
	return fused
}

func pageRanked(items []fusedCandidate, limit, offset int) []fusedCandidate {
	if len(items) == 0 || limit <= 0 {
		return []fusedCandidate{}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []fusedCandidate{}
	}
	end := offset + limit
	if end < offset || end > len(items) {
		end = len(items)
	}
	page := make([]fusedCandidate, end-offset)
	copy(page, items[offset:end])
	return page
}

func buildDebug(items []fusedCandidate) []AssetDebug {
	debug := make([]AssetDebug, 0, len(items))
	for _, item := range items {
		debug = append(debug, AssetDebug{
			AssetID:       item.assetID.String(),
			Score:         item.score,
			Contributions: item.contributions,
		})
	}
	return debug
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > DefaultCandidatePoolMax {
		return DefaultCandidatePoolMax
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func normalizeTopK(topK, limit, offset int) int {
	if topK > 0 {
		if topK > DefaultCandidatePoolMax {
			return DefaultCandidatePoolMax
		}
		return topK
	}
	needed := (limit + offset) * DefaultCandidateMultiplier
	if needed < DefaultCandidatePoolMin {
		needed = DefaultCandidatePoolMin
	}
	if needed > DefaultCandidatePoolMax {
		needed = DefaultCandidatePoolMax
	}
	return needed
}
