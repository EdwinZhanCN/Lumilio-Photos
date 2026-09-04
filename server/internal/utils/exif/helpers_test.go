package exif

import (
	"encoding/json"
	"server/internal/db/dbtypes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDateTimeWithCaptureOffset(t *testing.T) {
	parsedTime, offsetMinutes, err := parseDateTimeWithCaptureOffset(
		"2024:04:11 16:00:26",
		"-04:00",
	)
	require.NoError(t, err)
	require.NotNil(t, offsetMinutes)
	require.Equal(t, int16(-240), *offsetMinutes)
	require.Equal(t, time.Date(2024, time.April, 11, 20, 0, 26, 0, time.UTC), parsedTime)
}

func TestParseDateTimeWithCaptureOffset_UsesEmbeddedOffset(t *testing.T) {
	parsedTime, offsetMinutes, err := parseDateTimeWithCaptureOffset(
		"2024-04-11T16:00:26-04:00",
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, offsetMinutes)
	require.Equal(t, int16(-240), *offsetMinutes)
	require.Equal(t, time.Date(2024, time.April, 11, 20, 0, 26, 0, time.UTC), parsedTime)
}

func TestParseDateTimeWithCaptureOffsetPreservesFractionalSeconds(t *testing.T) {
	parsedTime, offsetMinutes, err := parseDateTimeWithCaptureOffset(
		"2025:09:28 03:42:29.97-05:00",
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, offsetMinutes)
	require.Equal(t, int16(-300), *offsetMinutes)
	require.Equal(t, 970_000_000, parsedTime.Nanosecond())
}

func TestParseCommonMetadataPreservesZeroGPSCoordinates(t *testing.T) {
	metadata := parseCommonMetadata(map[string]string{
		"GPSLatitude":  "0",
		"GPSLongitude": "0",
	}, nil, dbtypes.AssetTypePhoto)

	require.NotNil(t, metadata.GPSLatitude)
	require.NotNil(t, metadata.GPSLongitude)
	require.Equal(t, 0.0, *metadata.GPSLatitude)
	require.Equal(t, 0.0, *metadata.GPSLongitude)
}

func TestParsePhotoMetadataUsesStandardDescriptionAndRationalExposureCompensation(t *testing.T) {
	metadata := parsePhotoMetadata(map[string]string{
		"Make":                 "NIKON CORPORATION",
		"Description":          "XMP description",
		"Caption-Abstract":     "IPTC caption",
		"ImageDescription":     "EXIF description",
		"ExposureCompensation": "+2/3",
	})

	require.Equal(t, "NIKON CORPORATION", metadata.CameraMake)
	require.Equal(t, "XMP description", metadata.Description)
	require.NotNil(t, metadata.ExposureCompensation)
	require.InDelta(t, 2.0/3.0, *metadata.ExposureCompensation, 0.0001)
}

func TestParseCommonMetadataExtractsRatingKeywordsAndOrientedDimensions(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"Keywords":            []string{"Travel", "Night sky"},
		"Subject":             []string{"travel", "Stars"},
		"HierarchicalSubject": "Places|Iceland",
	})
	require.NoError(t, err)

	metadata := parseCommonMetadata(map[string]string{
		"Rating":      "4",
		"ImageWidth":  "4032 pixels",
		"ImageHeight": "3024",
		"Orientation": "Rotate 90 CW",
	}, raw, dbtypes.AssetTypePhoto)

	require.NotNil(t, metadata.Rating)
	require.Equal(t, int32(4), *metadata.Rating)
	require.NotNil(t, metadata.Width)
	require.NotNil(t, metadata.Height)
	require.Equal(t, int32(3024), *metadata.Width)
	require.Equal(t, int32(4032), *metadata.Height)
	require.Equal(t, []string{"Travel", "Night sky", "Stars", "Places|Iceland"}, metadata.Keywords)
}

func TestParseCommonMetadataRejectsNonStarRatings(t *testing.T) {
	for _, rating := range []string{"-1", "0", "3.5", "6", "invalid"} {
		metadata := parseCommonMetadata(map[string]string{"Rating": rating}, nil, dbtypes.AssetTypePhoto)
		require.Nil(t, metadata.Rating, rating)
	}
}
