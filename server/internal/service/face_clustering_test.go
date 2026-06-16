package service

import (
	"testing"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/require"
)

func TestDecidePendingFaceRecognition(t *testing.T) {
	t.Parallel()

	require.Equal(t, pendingFaceRecognitionDecisionDefer, decidePendingFaceRecognition(false, false, false))
	require.Equal(t, pendingFaceRecognitionDecisionAssignExisting, decidePendingFaceRecognition(false, true, true))
	require.Equal(t, pendingFaceRecognitionDecisionSkip, decidePendingFaceRecognition(false, true, false))
	require.Equal(t, pendingFaceRecognitionDecisionAssignExisting, decidePendingFaceRecognition(true, false, true))
	require.Equal(t, pendingFaceRecognitionDecisionCreateCluster, decidePendingFaceRecognition(true, false, false))
}

func TestCollectPendingFaceRecognitionScopesDeduplicatesByModel(t *testing.T) {
	t.Parallel()

	repositoryID := pgtype.UUID{
		Bytes: [16]byte{1, 2, 3, 4},
		Valid: true,
	}
	ownerID := int32(42)
	modelA := "model-a"
	modelB := "model-b"

	scopes := collectPendingFaceRecognitionScopes(repo.Asset{
		RepositoryID: repositoryID,
		OwnerID:      &ownerID,
	}, []repo.FaceItem{
		{ID: 1, Confidence: 0.95, Embedding: testVectorPtr([]float32{1, 0}), EmbeddingModel: &modelA},
		{ID: 2, Confidence: 0.90, Embedding: testVectorPtr([]float32{1, 0}), EmbeddingModel: &modelA},
		{ID: 3, Confidence: 0.92, Embedding: testVectorPtr([]float32{0, 1}), EmbeddingModel: &modelB},
		{ID: 4, Confidence: 0.20, Embedding: testVectorPtr([]float32{0, 1}), EmbeddingModel: &modelB},
	})

	require.Len(t, scopes, 2)
	require.Equal(t, normalizedName(&modelA), scopes[0].EmbeddingModel)
	require.Equal(t, normalizedName(&modelB), scopes[1].EmbeddingModel)
	require.Equal(t, ownerID, *scopes[0].OwnerID)
	require.Equal(t, ownerID, *scopes[1].OwnerID)
}

func TestCollectFaceClusteringScopesDeduplicatesByScope(t *testing.T) {
	t.Parallel()

	repositoryID := pgtype.UUID{
		Bytes: [16]byte{9, 8, 7, 6},
		Valid: true,
	}
	ownerID := int32(7)
	modelA := "model-a"
	modelB := "model-b"

	scopes := collectFaceClusteringScopes([]repo.GetFaceClusteringCandidatesRow{
		{RepositoryID: repositoryID, OwnerID: &ownerID, EmbeddingModel: &modelA},
		{RepositoryID: repositoryID, OwnerID: &ownerID, EmbeddingModel: &modelA},
		{RepositoryID: repositoryID, OwnerID: &ownerID, EmbeddingModel: &modelB},
	})

	require.Len(t, scopes, 2)
	require.Equal(t, normalizedName(&modelA), scopes[0].EmbeddingModel)
	require.Equal(t, normalizedName(&modelB), scopes[1].EmbeddingModel)
}

func testVectorPtr(values []float32) *pgvector.Vector {
	vector := pgvector.NewVector(values)
	return &vector
}
