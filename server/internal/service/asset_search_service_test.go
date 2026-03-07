package service

import (
	"context"
	"testing"
	"time"

	"server/internal/db/repo"

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

func TestAssetServiceSearchAssets_AutoModePreservesTopResultsAndDedupesResults(t *testing.T) {
	topOnly := testSearchAsset(t, "11111111-1111-1111-1111-111111111111", "top-only.jpg")
	shared := testSearchAsset(t, "22222222-2222-2222-2222-222222222222", "shared.jpg")
	filenameOnly := testSearchAsset(t, "33333333-3333-3333-3333-333333333333", "filename-only.jpg")

	svc := &assetService{
		queryAssetsUnifiedFn: func(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
			require.Equal(t, "filename", params.SearchType)
			require.Equal(t, "red bird", params.Query)
			return []repo.Asset{shared, filenameOnly}, 2, nil
		},
		searchAssetsClipTopResultsFn: func(ctx context.Context, params SearchAssetsParams) ([]repo.Asset, SearchTopResultsMeta) {
			require.Equal(t, SearchEnhancementModeAuto, params.EnhancementMode)
			require.Equal(t, "red bird", params.Query)
			return []repo.Asset{topOnly, shared}, SearchTopResultsMeta{
				Enabled:     true,
				SourceTypes: []string{"clip"},
			}
		},
	}

	result, err := svc.SearchAssets(context.Background(), SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{
			Query: "  red bird  ",
		},
		EnhancementMode: SearchEnhancementModeAuto,
	})
	require.NoError(t, err)

	require.Equal(t, []repo.Asset{topOnly, shared}, result.TopResults)
	require.Equal(t, []repo.Asset{filenameOnly}, result.Results)
	require.Equal(t, int64(1), result.ResultsTotal)
	require.Equal(t, SearchTopResultsMeta{
		Enabled:     true,
		SourceTypes: []string{"clip"},
	}, result.TopResultsMeta)
}

func TestAssetServiceSearchAssets_DegradesToFilenameResults(t *testing.T) {
	filenameOnly := testSearchAsset(t, "44444444-4444-4444-4444-444444444444", "filename-only.jpg")

	svc := &assetService{
		queryAssetsUnifiedFn: func(ctx context.Context, params QueryAssetsParams) ([]repo.Asset, int64, error) {
			return []repo.Asset{filenameOnly}, 1, nil
		},
		searchAssetsClipTopResultsFn: func(ctx context.Context, params SearchAssetsParams) ([]repo.Asset, SearchTopResultsMeta) {
			return []repo.Asset{}, SearchTopResultsMeta{
				Enabled:     true,
				Degraded:    true,
				Reason:      "runtime_unavailable",
				SourceTypes: []string{"clip"},
			}
		},
	}

	result, err := svc.SearchAssets(context.Background(), SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{
			Query: "sunset",
		},
		EnhancementMode: SearchEnhancementModeAuto,
	})
	require.NoError(t, err)

	require.Empty(t, result.TopResults)
	require.Equal(t, []repo.Asset{filenameOnly}, result.Results)
	require.Equal(t, int64(1), result.ResultsTotal)
	require.Equal(t, SearchTopResultsMeta{
		Enabled:     true,
		Degraded:    true,
		Reason:      "runtime_unavailable",
		SourceTypes: []string{"clip"},
	}, result.TopResultsMeta)
}
