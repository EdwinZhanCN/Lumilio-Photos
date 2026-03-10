package exif

import (
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
