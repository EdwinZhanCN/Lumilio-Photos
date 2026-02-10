package processors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeDiscoveredPath(t *testing.T) {
	path, err := sanitizeDiscoveredPath("albums/2026/02/a.jpg")
	require.NoError(t, err)
	require.Equal(t, "albums/2026/02/a.jpg", path)

	_, err = sanitizeDiscoveredPath(".lumilio/assets/1.jpg")
	require.Error(t, err)

	_, err = sanitizeDiscoveredPath("inbox/2026/02/a.jpg")
	require.Error(t, err)

	_, err = sanitizeDiscoveredPath("../escape/a.jpg")
	require.Error(t, err)
}
