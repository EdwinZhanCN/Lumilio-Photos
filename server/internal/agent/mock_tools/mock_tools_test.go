package mocktools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogHasMediaManagementTools(t *testing.T) {
	t.Parallel()

	definitions := Catalog()
	require.GreaterOrEqual(t, len(definitions), 8)
	require.Contains(t, definitions[0].Tags, "media")

	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}

	require.Contains(t, names, "mock_filter_assets")
	require.Contains(t, names, "mock_find_duplicate_assets")
	require.Contains(t, names, "mock_bulk_archive_assets")
	require.Contains(t, names, "mock_create_album")
	require.NotContains(t, names, "mock_run_training")
}
