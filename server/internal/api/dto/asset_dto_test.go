package dto

import (
	"testing"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestToAssetDTONilStoragePath(t *testing.T) {
	var assetID pgtype.UUID
	require.NoError(t, assetID.Scan("11111111-1111-1111-1111-111111111111"))

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
