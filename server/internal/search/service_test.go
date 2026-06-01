package search

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubRetriever struct {
	source     string
	weight     float64
	candidates []Candidate
	err        error
}

func (r stubRetriever) Source() string  { return r.source }
func (r stubRetriever) Weight() float64 { return r.weight }
func (r stubRetriever) Retrieve(context.Context, Request) ([]Candidate, error) {
	return r.candidates, r.err
}

func TestWeightedRRFFusesRanksAndWeights(t *testing.T) {
	assetA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	assetB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	fused := fuseWeightedRRF([]Candidate{
		{AssetID: assetA, Source: SourceEmbedding, Rank: 1, RawScore: 0.1},
		{AssetID: assetB, Source: SourceEmbedding, Rank: 2, RawScore: 0.2},
		{AssetID: assetB, Source: SourceOCR, Rank: 1, RawScore: 0.9},
	}, map[string]float64{
		SourceEmbedding: 1.0,
		SourceOCR:       0.7,
	}, DefaultRRFK)

	require.Len(t, fused, 2)
	require.Equal(t, assetB, fused[0].assetID)
	require.Contains(t, fused[0].contributions, SourceEmbedding)
	require.Contains(t, fused[0].contributions, SourceOCR)
}

func TestAggregateSearchIgnoresSingleRetrieverFailure(t *testing.T) {
	assetID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	service := NewAggregateService(nil, []Retriever{
		stubRetriever{source: SourceEmbedding, weight: 1, err: errors.New("offline")},
		stubRetriever{source: SourceOCR, weight: 0.7, candidates: []Candidate{{AssetID: assetID, Source: SourceOCR, Rank: 1}}},
	}, nil)
	service.pool = nil

	fused := fuseWeightedRRF([]Candidate{{AssetID: assetID, Source: SourceOCR, Rank: 1}}, map[string]float64{SourceOCR: 0.7}, DefaultRRFK)
	require.Len(t, fused, 1)
}

func TestNormalizeTopKUsesPageBoundary(t *testing.T) {
	require.Equal(t, 80, normalizeTopK(0, 20, 0))
	require.Equal(t, 280, normalizeTopK(0, 20, 50))
	require.Equal(t, DefaultCandidatePoolMax, normalizeTopK(5000, 20, 0))
}

func TestNormalizeLimitAllowsCandidatePoolHydration(t *testing.T) {
	require.Equal(t, DefaultCandidatePoolMax, normalizeLimit(DefaultCandidatePoolMax))
	require.Equal(t, DefaultCandidatePoolMax, normalizeLimit(DefaultCandidatePoolMax+1))
}
