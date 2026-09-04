package service

import (
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreserveSpecificMetadataDescriptionKeepsExistingEmptyValue(t *testing.T) {
	existing := dbtypes.SpecificMetadata(`{"description":"","exposure":1}`)
	incoming := dbtypes.SpecificMetadata(`{"description":"embedded","camera_model":"X-T5"}`)

	merged, err := preserveSpecificMetadataDescription(existing, incoming)
	require.NoError(t, err)
	require.JSONEq(t, `{"description":"","camera_model":"X-T5"}`, string(merged))
}

func TestPreserveSpecificMetadataDescriptionImportsWhenMissing(t *testing.T) {
	existing := dbtypes.SpecificMetadata(`{"camera_model":"X-T5"}`)
	incoming := dbtypes.SpecificMetadata(`{"description":"embedded","camera_model":"X-T5"}`)

	merged, err := preserveSpecificMetadataDescription(existing, incoming)
	require.NoError(t, err)
	require.JSONEq(t, string(incoming), string(merged))
}

func TestHasStoredExifRaw(t *testing.T) {
	require.False(t, hasStoredExifRaw(nil))
	require.False(t, hasStoredExifRaw(dbtypes.JSON(`null`)))
	require.False(t, hasStoredExifRaw(dbtypes.JSON(`{}`)))
	require.True(t, hasStoredExifRaw(dbtypes.JSON(`{"Make":"Canon"}`)))
}

func TestPrepareImportedCommonMetadataOnlyImportsRatingAndKeywordsOnce(t *testing.T) {
	rating := int32(4)
	common := dbtypes.CommonMetadata{Rating: &rating, Keywords: []string{"Travel"}}

	first, firstExtraction := prepareImportedCommonMetadata(repo.Asset{}, common)
	require.True(t, firstExtraction)
	require.Equal(t, &rating, first.Rating)
	require.Equal(t, []string{"Travel"}, first.Keywords)

	retry, firstExtraction := prepareImportedCommonMetadata(repo.Asset{
		ExifRaw: dbtypes.JSON(`{"Rating":4}`),
	}, common)
	require.False(t, firstExtraction)
	require.Nil(t, retry.Rating)
	require.Nil(t, retry.Keywords)
}

func TestPrepareImportedCommonMetadataPreservesExistingUserRating(t *testing.T) {
	embeddedRating := int32(4)
	userRating := int64(5)

	common, firstExtraction := prepareImportedCommonMetadata(repo.Asset{Rating: &userRating}, dbtypes.CommonMetadata{
		Rating: &embeddedRating,
	})

	require.True(t, firstExtraction)
	require.Nil(t, common.Rating)
}
