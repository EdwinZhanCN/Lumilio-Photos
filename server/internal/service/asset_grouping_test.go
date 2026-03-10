package service

import (
	"testing"
	"time"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func testGroupedAsset(takenAt, uploadAt time.Time, mimeType, assetType string) repo.Asset {
	return repo.Asset{
		Type:       assetType,
		MimeType:   mimeType,
		TakenTime:  pgtype.Timestamptz{Time: takenAt, Valid: !takenAt.IsZero()},
		UploadTime: pgtype.Timestamptz{Time: uploadAt, Valid: !uploadAt.IsZero()},
	}
}

func TestGroupAssetsPageAt_DateUsesTakenTimeWithViewerTimezone(t *testing.T) {
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	asset := testGroupedAsset(
		time.Date(2026, time.March, 10, 1, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC),
		"image/jpeg",
		AssetTypePhoto,
	)

	groups := groupAssetsPageAt([]repo.Asset{asset}, "date", "America/New_York", now)
	require.Len(t, groups, 1)
	require.Equal(t, "date:yesterday", groups[0].Key)
}

func TestGroupAssetsPageAt_DateFallsBackToUploadTime(t *testing.T) {
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	asset := testGroupedAsset(
		time.Time{},
		time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC),
		"image/jpeg",
		AssetTypePhoto,
	)

	groups := groupAssetsPageAt([]repo.Asset{asset}, "date", "UTC", now)
	require.Len(t, groups, 1)
	require.Equal(t, "date:today", groups[0].Key)
}

func TestGroupAssetsPageAt_TypeAndFlat(t *testing.T) {
	assets := []repo.Asset{
		testGroupedAsset(time.Time{}, time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC), "image/jpeg", AssetTypePhoto),
		testGroupedAsset(time.Time{}, time.Date(2026, time.March, 9, 8, 0, 0, 0, time.UTC), "", AssetTypeVideo),
	}

	typeGroups := groupAssetsPageAt(assets, "type", "UTC", time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC))
	require.Len(t, typeGroups, 2)
	require.Equal(t, "type:image/jpeg", typeGroups[0].Key)
	require.Equal(t, "type:video/*", typeGroups[1].Key)

	flatGroups := groupAssetsPageAt(assets, "flat", "UTC", time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC))
	require.Len(t, flatGroups, 1)
	require.Equal(t, "flat:all", flatGroups[0].Key)
	require.Len(t, flatGroups[0].Assets, 2)
}
