package tools

import (
	"testing"
	"time"

	"server/internal/api/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBuildFilterDTOMapsStructuredMentionFields(t *testing.T) {
	albumID := 123
	input := &AssetFilterInput{
		RepositoryID: "550e8400-e29b-41d4-a716-446655440000",
		AlbumID:      &albumID,
		CameraModel:  "ILCE-7M3",
		LensModel:    "FE 24-70mm F2.8 GM",
		DateFrom:     "2025-01-02",
		DateTo:       "2025-01-31",
		Type:         "PHOTO",
	}

	filter := buildFilterDTO(input)

	require.NotNil(t, filter.RepositoryID)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", *filter.RepositoryID)
	require.NotNil(t, filter.AlbumID)
	require.Equal(t, albumID, *filter.AlbumID)
	require.NotNil(t, filter.CameraMake)
	require.Equal(t, "ILCE-7M3", *filter.CameraMake)
	require.NotNil(t, filter.Lens)
	require.Equal(t, "FE 24-70mm F2.8 GM", *filter.Lens)
	require.NotNil(t, filter.Type)
	require.Equal(t, "PHOTO", *filter.Type)
	require.NotNil(t, filter.Date)
	require.WithinDuration(t, time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), *filter.Date.From, 0)
	require.WithinDuration(t, time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), *filter.Date.To, 0)
}

func TestConvertDTOToParamsMapsRepositoryAndAlbum(t *testing.T) {
	repositoryID := uuid.NewString()
	albumID := 42
	cameraModel := "ILCE-7M3"
	lensModel := "FE 24-70mm F2.8 GM"

	params := convertDTOToParams(dto.AssetFilterDTO{
		RepositoryID: &repositoryID,
		AlbumID:      &albumID,
		CameraMake:   &cameraModel,
		Lens:         &lensModel,
	})

	require.True(t, params.RepositoryID.Valid)
	require.Equal(t, [16]byte(uuid.MustParse(repositoryID)), params.RepositoryID.Bytes)
	require.NotNil(t, params.AlbumID)
	require.Equal(t, int32(albumID), *params.AlbumID)
	require.NotNil(t, params.CameraModel)
	require.Equal(t, cameraModel, *params.CameraModel)
	require.NotNil(t, params.LensModel)
	require.Equal(t, lensModel, *params.LensModel)
}
