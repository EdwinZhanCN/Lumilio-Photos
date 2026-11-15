package dbtypes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhotoSpecificMetadataOmitsMissingFieldsAndKeepsZeroExposureCompensation(t *testing.T) {
	zero := float32(0)
	encoded, err := json.Marshal(PhotoSpecificMetadata{
		CameraModel:          "NIKON Z fc",
		ExposureCompensation: &zero,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"camera_model":"NIKON Z fc","exposure_compensation":0}`, string(encoded))
}
