//go:build sqlite_fts5

package dto

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAssetDetailFacesDecodeCatalogTimestampsAndFlags(t *testing.T) {
	catalog, assetID := openAssetDetailOCRTestCatalog(t)
	ctx := context.Background()

	_, err := catalog.SQL.ExecContext(ctx, `
INSERT INTO face_results (
    asset_id, model_id, total_faces, processing_time_ms, created_at, updated_at
) VALUES (?, 'fixture-face', 2, 12, 1, 2);
INSERT INTO face_items (
    id, asset_id, face_id, bounding_box, confidence, is_primary, created_at
) VALUES
    (11, ?, 'face-a', '[1,2,3,4]', 0.62, 0, 1),
    (12, ?, 'face-b', '[5,6,7,8]', 0.97, 1, 1);
`, assetID, assetID, assetID)
	require.NoError(t, err)

	row, err := catalog.Queries.GetAssetWithRelations(ctx, assetID)
	require.NoError(t, err)
	detail := ToAssetDetailDTO(row, AssetDetailIncludes{Faces: true})
	require.NotNil(t, detail.FaceResult, "a populated relation must not be dropped by decoding")
	require.Equal(t, "fixture-face", detail.FaceResult.ModelID)
	require.Equal(t, time.UnixMicro(1).UTC(), *detail.FaceResult.CreatedAt)
	require.Equal(t, time.UnixMicro(2).UTC(), *detail.FaceResult.UpdatedAt)
	require.Len(t, detail.FaceResult.Faces, 2)
	// Primary first, then by confidence, as the relation query orders them.
	require.Equal(t, int64(12), detail.FaceResult.Faces[0].ID)
	require.True(t, *detail.FaceResult.Faces[0].IsPrimary)
	require.False(t, *detail.FaceResult.Faces[1].IsPrimary)
	require.JSONEq(t, `[5,6,7,8]`, string(detail.FaceResult.Faces[0].BoundingBox))
}
