package core

import (
	"strings"
	"testing"

	"server/internal/api/dto"
	"server/internal/db/repo"

	"github.com/stretchr/testify/require"
)

func TestSanitizeReferenceToken(t *testing.T) {
	t.Parallel()

	require.Equal(t, "filter_assets", sanitizeReferenceToken("Filter Assets"))
	require.Equal(t, "bulk_like_update", sanitizeReferenceToken("bulk-like/update"))
	require.Equal(t, "asset_filter", sanitizeReferenceToken(" asset__filter "))
	require.Equal(t, "", sanitizeReferenceToken("   "))
}

func TestDefaultReferenceKind(t *testing.T) {
	t.Parallel()

	require.Equal(t, "asset_filter_dto", defaultReferenceKind(dto.AssetFilterDTO{}))
	require.Equal(t, "asset_list", defaultReferenceKind([]repo.Asset{}))
	require.Equal(t, "map_value", defaultReferenceKind(map[string]int{}))
}

func TestNewReferenceIDIncludesSemanticSegments(t *testing.T) {
	t.Parallel()

	id := newReferenceID("filter_assets", "asset_filter")
	parts := strings.Split(id, ".")

	require.Len(t, parts, 4)
	require.Equal(t, "ref", parts[0])
	require.Equal(t, "filter_assets", parts[1])
	require.Equal(t, "asset_filter", parts[2])
	require.Len(t, parts[3], 32)
}
