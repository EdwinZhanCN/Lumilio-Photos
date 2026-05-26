package service

import (
	"sort"
	"testing"

	"server/internal/db/repo"

	"github.com/stretchr/testify/require"
)

func TestRunHDBSCANSeparatesDenseComponents(t *testing.T) {
	points := []faceClusterCandidate{
		testFaceClusterCandidate(1, []float32{1.00, 0.00, 0.00}),
		testFaceClusterCandidate(2, []float32{0.99, 0.01, 0.00}),
		testFaceClusterCandidate(3, []float32{0.98, 0.02, 0.00}),
		testFaceClusterCandidate(4, []float32{0.00, 1.00, 0.00}),
		testFaceClusterCandidate(5, []float32{0.01, 0.99, 0.00}),
		testFaceClusterCandidate(6, []float32{0.02, 0.98, 0.00}),
		testFaceClusterCandidate(7, []float32{0.00, 0.00, 1.00}),
	}

	clusters := runHDBSCAN(points)
	require.Len(t, clusters, 2)

	actual := make([][]int32, 0, len(clusters))
	for _, cluster := range clusters {
		ids := make([]int32, 0, len(cluster.indices))
		for _, index := range cluster.indices {
			ids = append(ids, points[index].item.ID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		actual = append(actual, ids)
	}
	sort.Slice(actual, func(i, j int) bool { return actual[i][0] < actual[j][0] })

	require.Equal(t, []int32{1, 2, 3}, actual[0])
	require.Equal(t, []int32{4, 5, 6}, actual[1])
}

func TestRunHDBSCANLeavesWeakPairsAsNoise(t *testing.T) {
	points := []faceClusterCandidate{
		testFaceClusterCandidate(1, []float32{1, 0, 0}),
		testFaceClusterCandidate(2, []float32{0, 1, 0}),
	}

	clusters := runHDBSCAN(points)
	require.Empty(t, clusters)
}

func testFaceClusterCandidate(id int32, vector []float32) faceClusterCandidate {
	faceSize := faceClusterMinAreaPixels
	return faceClusterCandidate{
		item: repo.FaceItem{
			ID:         id,
			Confidence: 1,
			FaceSize:   &faceSize,
		},
		vector: vector,
	}
}
