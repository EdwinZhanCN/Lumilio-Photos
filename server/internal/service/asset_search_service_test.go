package service

import (
	"context"
	"testing"
	"time"

	"server/internal/db/repo"
	aggregatesearch "server/internal/search"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func testSearchAsset(t *testing.T, rawID string, filename string) repo.Asset {
	t.Helper()

	var assetID pgtype.UUID
	require.NoError(t, assetID.Scan(rawID))

	path := "/tmp/" + filename
	return repo.Asset{
		AssetID:          assetID,
		Type:             "PHOTO",
		OriginalFilename: filename,
		StoragePath:      &path,
		MimeType:         "image/jpeg",
		UploadTime:       pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true},
	}
}

func scoredSet(ids ...uuid.UUID) fusedSearchSet {
	members := make([]aggregatesearch.ScoredAsset, len(ids))
	for i, id := range ids {
		members[i] = aggregatesearch.ScoredAsset{AssetID: id, Score: float64(len(ids) - i)}
	}
	return fusedSearchSet{Members: members, Sources: []string{"embedding", "filename"}}
}

// Best Results is the confidence-ordered Top-N subset of the fused set;
// Results is the whole set under the presentation sort — a literal superset.
func TestSearchAssets_FusedSet_TopSubsetAndFullResults(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	assets := map[uuid.UUID]repo.Asset{
		ids[0]: testSearchAsset(t, "11111111-1111-1111-1111-111111111111", "a.jpg"),
		ids[1]: testSearchAsset(t, "22222222-2222-2222-2222-222222222222", "b.jpg"),
		ids[2]: testSearchAsset(t, "33333333-3333-3333-3333-333333333333", "c.jpg"),
	}

	var gotTopIDs []uuid.UUID
	var gotSort string
	svc := &assetService{
		searchAssetsFusedSetFn: func(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool) {
			require.Equal(t, "red bird", params.Query)
			return scoredSet(ids...), true
		},
		hydrateAssetsInOrderFn: func(ctx context.Context, in []uuid.UUID) ([]repo.Asset, error) {
			gotTopIDs = in
			out := make([]repo.Asset, len(in))
			for i, id := range in {
				out[i] = assets[id]
			}
			return out, nil
		},
		pageAssetsBySortFn: func(ctx context.Context, in []uuid.UUID, sortBy string, limit, offset int) ([]repo.Asset, error) {
			gotSort = sortBy
			out := make([]repo.Asset, len(in))
			for i, id := range in {
				out[i] = assets[id]
			}
			return out, nil
		},
	}

	result, err := svc.SearchAssets(context.Background(), SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{Query: "  red bird  ", SortBy: "date_captured", Limit: 50},
		EnhancementMode:   SearchEnhancementModeAuto,
		TopResultsLimit:   2,
	})
	require.NoError(t, err)

	// Top Results: confidence-ordered first 2 (subset).
	require.Equal(t, ids[:2], gotTopIDs)
	require.Len(t, result.TopResults, 2)
	// Results: the whole set (superset), under the requested sort.
	require.Len(t, result.Results, 3)
	require.Equal(t, int64(3), result.ResultsTotal)
	require.Equal(t, "date_captured", gotSort)
	require.True(t, result.TopResultsMeta.Enabled)
	require.False(t, result.TopResultsMeta.Degraded)
}

// When the fused set is smaller than the showcase size there is no Best
// Results section — only Results.
func TestSearchAssets_FusedSet_NoTopWhenBelowLimit(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	hydrateCalled := false
	svc := &assetService{
		searchAssetsFusedSetFn: func(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool) {
			return scoredSet(ids...), true
		},
		hydrateAssetsInOrderFn: func(ctx context.Context, in []uuid.UUID) ([]repo.Asset, error) {
			hydrateCalled = true
			return nil, nil
		},
		pageAssetsBySortFn: func(ctx context.Context, in []uuid.UUID, sortBy string, limit, offset int) ([]repo.Asset, error) {
			return make([]repo.Asset, len(in)), nil
		},
	}

	result, err := svc.SearchAssets(context.Background(), SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{Query: "red bird", Limit: 50},
		EnhancementMode:   SearchEnhancementModeAuto,
		TopResultsLimit:   9,
	})
	require.NoError(t, err)
	require.False(t, hydrateCalled, "no Best Results section below the showcase size")
	require.Empty(t, result.TopResults)
	require.Len(t, result.Results, 2)
	require.Equal(t, int64(2), result.ResultsTotal)
}

// Semantic channel down but others ran: meta is flagged degraded but Results
// still come from the fused set — no "switched to regular results" fallback.
func TestSearchAssets_FusedSet_SemanticDegradedStillReturnsResults(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	svc := &assetService{
		searchAssetsFusedSetFn: func(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool) {
			set := scoredSet(ids...)
			set.Sources = []string{"ocr", "filename"}
			set.SemanticDegraded = true
			return set, true
		},
		pageAssetsBySortFn: func(ctx context.Context, in []uuid.UUID, sortBy string, limit, offset int) ([]repo.Asset, error) {
			return make([]repo.Asset, len(in)), nil
		},
	}

	result, err := svc.SearchAssets(context.Background(), SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{Query: "red bird", Limit: 50},
		EnhancementMode:   SearchEnhancementModeAuto,
		TopResultsLimit:   9,
	})
	require.NoError(t, err)
	require.True(t, result.TopResultsMeta.Degraded)
	require.Equal(t, semanticUnavailableReason, result.TopResultsMeta.Reason)
	require.Len(t, result.Results, 2)
}

// No search channel could run at all: fall back to the legacy filename path.
func TestSearchAssets_FallsBackToFilenameWhenNoChannel(t *testing.T) {
	filenameOnly := testSearchAsset(t, "44444444-4444-4444-4444-444444444444", "filename-only.jpg")
	svc := &assetService{
		searchAssetsFusedSetFn: func(ctx context.Context, params SearchAssetsParams) (fusedSearchSet, bool) {
			return fusedSearchSet{}, false
		},
		queryAssetsUnifiedFn: func(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
			require.Equal(t, "filename", params.SearchType)
			return []repo.Asset{filenameOnly}, 1, nil
		},
	}

	result, err := svc.SearchAssets(context.Background(), SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{Query: "sunset"},
		EnhancementMode:   SearchEnhancementModeAuto,
	})
	require.NoError(t, err)
	require.Empty(t, result.TopResults)
	require.Equal(t, []repo.Asset{filenameOnly}, result.Results)
	require.Equal(t, int64(1), result.ResultsTotal)
	require.True(t, result.TopResultsMeta.Degraded)
	require.Equal(t, semanticUnavailableReason, result.TopResultsMeta.Reason)
}
