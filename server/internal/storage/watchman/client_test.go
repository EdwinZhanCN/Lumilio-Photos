package watchman

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQueryResult(t *testing.T) {
	resp := Response{
		"clock":             "c:123:1",
		"is_fresh_instance": true,
		"files": []any{
			map[string]any{
				"name":     "2026/02/test.jpg",
				"exists":   true,
				"new":      true,
				"type":     "f",
				"size":     float64(1234),
				"mtime_ms": float64(1700000000000),
			},
		},
	}

	parsed, err := ParseQueryResult(resp)
	require.NoError(t, err)
	require.Equal(t, "c:123:1", parsed.Clock)
	require.True(t, parsed.IsFreshInstance)
	require.Len(t, parsed.Files, 1)
	require.Equal(t, "2026/02/test.jpg", parsed.Files[0].Name)
	require.True(t, parsed.Files[0].Exists)
	require.True(t, parsed.Files[0].New)
	require.Equal(t, int64(1234), parsed.Files[0].Size)
	require.Equal(t, int64(1700000000000), parsed.Files[0].MTimeMs)
}

func TestParseQueryResult_WithWatchmanError(t *testing.T) {
	_, err := ParseQueryResult(Response{"error": "permission denied"})
	require.Error(t, err)
}
