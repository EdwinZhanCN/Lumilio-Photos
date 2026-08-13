package dto

import (
	"testing"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToAssetDTONilStoragePath(t *testing.T) {
	assetID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	got := ToAssetDTO(repo.Asset{
		AssetID:          assetID,
		Type:             "PHOTO",
		OriginalFilename: "missing-path.jpg",
		MimeType:         "image/jpeg",
		FileSize:         123,
		StoragePath:      nil,
	})

	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.AssetID)
	require.Equal(t, "", got.StoragePath)
	require.Equal(t, "missing-path.jpg", got.OriginalFilename)
}

func TestToAssetDTOExposesTopLevelGPS(t *testing.T) {
	latitude, longitude := 37.7749, -122.4194

	got := ToAssetDTO(repo.Asset{
		AssetID:          uuid.New(),
		Type:             "PHOTO",
		OriginalFilename: "photo.jpg",
		MimeType:         "image/jpeg",
		GpsLatitude:      &latitude,
		GpsLongitude:     &longitude,
	})

	require.Equal(t, &latitude, got.GPSLatitude)
	require.Equal(t, &longitude, got.GPSLongitude)
}
