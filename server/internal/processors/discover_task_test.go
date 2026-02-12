package processors

import (
	"testing"

	"server/internal/queue/jobs"

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

func TestNormalizeDiscoverOperation(t *testing.T) {
	require.Equal(t, jobs.DiscoverOperationUpsert, normalizeDiscoverOperation(""))
	require.Equal(t, jobs.DiscoverOperationUpsert, normalizeDiscoverOperation("upsert"))
	require.Equal(t, jobs.DiscoverOperationUpsert, normalizeDiscoverOperation(" UPSERT "))
	require.Equal(t, jobs.DiscoverOperationDelete, normalizeDiscoverOperation("delete"))
	require.Equal(t, jobs.DiscoverOperationDelete, normalizeDiscoverOperation(" DELETE "))
	require.Equal(t, jobs.DiscoverOperationUpsert, normalizeDiscoverOperation("unsupported"))
}
