package service

import (
	"context"

	"server/internal/db/repo"
	aggregatesearch "server/internal/search"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// The unified search pipeline: every channel is self-thresholded (calibrated
// semantic set, tsquery-matched OCR/place, filename match), the rankings are
// fused with weighted RRF and the fused set IS the search result. There is
// no TopK anywhere — Best Results is simply the confidence-ordered Top-N
// subset of this set, and the Results tier is the same set under the
// requested presentation sort. Set/subset, nothing else.

// SourceFilename is the filename channel's RRF source name.
const SourceFilename = "filename"

const (
	// fusedSetCap bounds each channel's contribution to the fused set.
	fusedSetCap = 10000
	// semanticUnavailableReason flags that the semantic channel could not
	// run; the rest of the pipeline degrades gracefully without it.
	semanticUnavailableReason = "semantic_unavailable"
)

// fusedChannelWeights mirror the aggregate RRF weights, extended with the
// filename channel.
var fusedChannelWeights = map[string]float64{
	aggregatesearch.SourceEmbedding: 1.0,
	aggregatesearch.SourcePlace:     0.8,
	aggregatesearch.SourceOCR:       0.7,
	SourceFilename:                  0.6,
}

// fusedSearchSet is the pipeline output: the complete relevance set in
// aggregate-confidence order.
type fusedSearchSet struct {
	Members []aggregatesearch.ScoredAsset
	// Sources lists the channels that contributed (succeeded, even if empty).
	Sources []string
	// SemanticDegraded is true when the semantic channel could not run at
	// all (unconfigured or errored) — distinct from "ran and matched nothing".
	SemanticDegraded bool
}

func (f fusedSearchSet) ids() []uuid.UUID {
	ids := make([]uuid.UUID, len(f.Members))
	for i, member := range f.Members {
		ids[i] = member.AssetID
	}
	return ids
}

func (f fusedSearchSet) meta() SearchTopResultsMeta {
	meta := SearchTopResultsMeta{Enabled: true, SourceTypes: f.Sources}
	if meta.SourceTypes == nil {
		meta.SourceTypes = []string{}
	}
	if f.SemanticDegraded {
		meta.Degraded = true
		meta.Reason = semanticUnavailableReason
	}
	return meta
}

func (s *assetService) runSearchAssetsFusedSet(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool) {
	if s.searchAssetsFusedSetFn != nil {
		return s.searchAssetsFusedSetFn(ctx, params)
	}
	return s.searchAssetsFusedSet(ctx, params)
}

// searchAssetsFusedSet runs all channels and fuses their rankings. ok=false
// means no channel could run at all (no search infrastructure).
func (s *assetService) searchAssetsFusedSet(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool) {
	set := fusedSearchSet{Sources: []string{}}

	filter, err := buildAggregateSearchFilter(params.QueryAssetsParams)
	if err != nil {
		return set, false
	}
	req := aggregatesearch.Request{Query: params.Query, Filter: filter}

	var all []aggregatesearch.Candidate
	ran := 0

	// Semantic membership: per-query calibrated cutoff.
	if s.semanticRetriever != nil {
		if candidates, _, err := s.semanticRetriever.RetrieveSet(ctx, req, aggregatesearch.StrictnessNormal, fusedSetCap); err == nil {
			ran++
			set.Sources = append(set.Sources, aggregatesearch.SourceEmbedding)
			all = append(all, candidates...)
		} else {
			set.SemanticDegraded = true
		}
	} else {
		set.SemanticDegraded = true
	}

	// OCR and place membership: tsquery matching is the threshold.
	textReq := req
	textReq.TopK = fusedSetCap
	if s.ocrRetriever != nil {
		if candidates, err := s.ocrRetriever.Retrieve(ctx, textReq); err == nil {
			ran++
			set.Sources = append(set.Sources, aggregatesearch.SourceOCR)
			all = append(all, candidates...)
		}
	}
	if s.placeRetriever != nil {
		if candidates, err := s.placeRetriever.Retrieve(ctx, textReq); err == nil {
			ran++
			set.Sources = append(set.Sources, aggregatesearch.SourcePlace)
			all = append(all, candidates...)
		}
	}

	// Filename membership, ranked by capture time (the query's natural order).
	if s.queries != nil {
		if rows, err := s.queries.GetAssetIDsUnified(ctx, filenameMembershipParams(params.QueryAssetsParams)); err == nil {
			ran++
			set.Sources = append(set.Sources, SourceFilename)
			rank := 1
			for _, row := range rows {
				if row.Valid {
					all = append(all, aggregatesearch.Candidate{
						AssetID: uuid.UUID(row.Bytes),
						Source:  SourceFilename,
						Rank:    rank,
					})
					rank++
				}
			}
		}
	}

	if ran == 0 {
		return set, false
	}
	set.Members = aggregatesearch.FuseSet(all, fusedChannelWeights)
	return set, true
}

func (s *assetService) runHydrateAssetsInOrder(ctx context.Context, ids []uuid.UUID) ([]repo.Asset, error) {
	if s.hydrateAssetsInOrderFn != nil {
		return s.hydrateAssetsInOrderFn(ctx, ids)
	}
	return s.hydrateAssetsInOrder(ctx, ids)
}

// hydrateAssetsInOrder fetches asset rows preserving the given id order.
func (s *assetService) hydrateAssetsInOrder(ctx context.Context, ids []uuid.UUID) ([]repo.Asset, error) {
	if len(ids) == 0 {
		return []repo.Asset{}, nil
	}
	pgIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	rows, err := s.queries.GetAssetsByIDs(ctx, pgIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]repo.Asset, len(rows))
	for _, row := range rows {
		byID[uuid.UUID(row.AssetID.Bytes)] = row
	}
	out := make([]repo.Asset, 0, len(ids))
	for _, id := range ids {
		if row, found := byID[id]; found {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *assetService) runPageAssetsBySort(ctx context.Context, ids []uuid.UUID, sortBy string, limit, offset int) ([]repo.Asset, error) {
	if s.pageAssetsBySortFn != nil {
		return s.pageAssetsBySortFn(ctx, ids, sortBy, limit, offset)
	}
	return s.pageAssetsBySort(ctx, ids, sortBy, limit, offset)
}

// pageAssetsBySort orders a membership set by the requested presentation
// sort (newest first) and returns the requested page of rows.
func (s *assetService) pageAssetsBySort(ctx context.Context, ids []uuid.UUID, sortBy string, limit, offset int) ([]repo.Asset, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 || offset >= len(ids) {
		return []repo.Asset{}, nil
	}

	pgIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	var orderedAsc []pgtype.UUID
	var err error
	if sortBy == "recently_added" {
		orderedAsc, err = s.queries.RankAssetIDsByUploadTime(ctx, pgIDs)
	} else {
		orderedAsc, err = s.queries.RankAssetIDsByTime(ctx, pgIDs)
	}
	if err != nil {
		return nil, err
	}

	ordered := make([]uuid.UUID, 0, len(orderedAsc))
	for i := len(orderedAsc) - 1; i >= 0; i-- {
		if orderedAsc[i].Valid {
			ordered = append(ordered, uuid.UUID(orderedAsc[i].Bytes))
		}
	}

	end := offset + limit
	if end > len(ordered) {
		end = len(ordered)
	}
	if offset >= end {
		return []repo.Asset{}, nil
	}
	return s.hydrateAssetsInOrder(ctx, ordered[offset:end])
}
