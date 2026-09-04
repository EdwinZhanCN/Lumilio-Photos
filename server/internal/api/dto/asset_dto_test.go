package dto

import (
	"testing"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToAssetDTOExposesLogicalContentIdentityWithoutLocationFields(t *testing.T) {
	assetID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	contentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	got := ToAssetDTO(repo.Asset{
		AssetID:          assetID,
		ContentID:        contentID,
		Type:             "PHOTO",
		OriginalFilename: "missing-path.jpg",
		MimeType:         "image/jpeg",
	})

	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.AssetID)
	require.Equal(t, contentID.String(), got.ContentID)
	require.Nil(t, got.FileSize)
	require.Nil(t, got.Hash)
	require.Equal(t, "missing-path.jpg", got.OriginalFilename)
}

func TestToAssetDTOExposesTopLevelGPS(t *testing.T) {
	latitude, longitude := 37.7749, -122.4194

	got := ToAssetDTO(repo.Asset{
		AssetID:          uuid.New(),
		ContentID:        uuid.New(),
		Type:             "PHOTO",
		OriginalFilename: "photo.jpg",
		MimeType:         "image/jpeg",
		GpsLatitude:      &latitude,
		GpsLongitude:     &longitude,
	})

	require.Equal(t, &latitude, got.GPSLatitude)
	require.Equal(t, &longitude, got.GPSLongitude)
}
