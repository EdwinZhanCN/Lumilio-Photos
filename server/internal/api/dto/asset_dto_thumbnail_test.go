//go:build sqlite_fts5

package dto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// The thumbnails relation decodes: each row carries its size and media type,
// and no storage location.
func TestAssetDetailThumbnailsDecode(t *testing.T) {
	catalog, assetID := openAssetDetailOCRTestCatalog(t)
	ctx := context.Background()

	_, err := catalog.SQL.ExecContext(ctx, `
INSERT INTO thumbnails (asset_id, size, artifact_fence, artifact_version, artifact_name, mime_type, created_at)
VALUES (?, 'medium', 'fence', 'v1', 'medium.webp', 'image/webp', 1),
       (?, 'small', 'fence', 'v1', 'small.webp', 'image/webp', 1);
`, assetID, assetID)
	require.NoError(t, err)

	row, err := catalog.Queries.GetAssetWithRelations(ctx, assetID)
	require.NoError(t, err)
	detail := ToAssetDetailDTO(row, AssetDetailIncludes{Thumbnails: true})
	require.Len(t, detail.Thumbnails, 2, "a populated relation must not be dropped by decoding")
	require.Equal(t, "small", detail.Thumbnails[0].Size)
	require.Equal(t, "medium", detail.Thumbnails[1].Size)
	require.Equal(t, "image/webp", detail.Thumbnails[0].MimeType)
	require.NotZero(t, detail.Thumbnails[0].ThumbnailID)
}
