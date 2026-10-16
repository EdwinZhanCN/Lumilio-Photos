package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/google/uuid"

	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/testutil"
)

func TestFaceBoxIoU(t *testing.T) {
	box := dbtypes.FaceBoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10}
	for name, tc := range map[string]struct {
		other dbtypes.FaceBoundingBox
		want  float64
	}{
		"identical": {box, 1},
		"disjoint":  {dbtypes.FaceBoundingBox{X1: 20, Y1: 20, X2: 30, Y2: 30}, 0},
		"half":      {dbtypes.FaceBoundingBox{X1: 5, Y1: 0, X2: 15, Y2: 10}, 50.0 / 150.0},
	} {
		if got := faceBoxIoU(box, tc.other); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: IoU = %v, want %v", name, got, tc.want)
		}
	}
}

func createFaceItem(t *testing.T, ctx context.Context, queries *repo.Queries, assetID, repositoryID uuid.UUID, box dbtypes.FaceBoundingBox) repo.FaceItem {
	t.Helper()
	encoded, err := json.Marshal(box)
	if err != nil {
		t.Fatal(err)
	}
	item, err := queries.CreateFaceItem(ctx, repo.CreateFaceItemParams{
		AssetID: assetID, BoundingBox: dbtypes.JSON(encoded), Confidence: 0.9, RepositoryID: repositoryID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func faceMembership(t *testing.T, ctx context.Context, database *db.DB, faceID int32) (int32, bool, bool) {
	t.Helper()
	var clusterID int32
	var manual bool
	err := database.SQL.QueryRowContext(ctx, `SELECT cluster_id, is_manual FROM face_cluster_members WHERE face_id = ?`, faceID).Scan(&clusterID, &manual)
	if err != nil {
		return 0, false, false
	}
	return clusterID, manual, true
}

// A re-detection after an in-place edit re-applies a manual person
// assignment to the overlapping new face, and surfaces one whose face moved
// as an unconfirmed member instead of dropping it.
func TestManualFaceAssignmentsCarryOverToRedetectedFaces(t *testing.T) {
	ctx := context.Background()
	database := openFaceClusteringDatabase(t, ctx)
	repositoryID := seedFaceClusteringRepository(t, ctx, database)
	assetID := uuid.New()
	if _, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
		AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1, Filename: "group.jpg", FileSize: 1,
	}); err != nil {
		t.Fatal(err)
	}
	queries := database.Queries
	if _, err := queries.CreateFaceResult(ctx, repo.CreateFaceResultParams{AssetID: assetID, ModelID: "fixture", TotalFaces: 2}); err != nil {
		t.Fatal(err)
	}
	left := createFaceItem(t, ctx, queries, assetID, repositoryID, dbtypes.FaceBoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100})
	right := createFaceItem(t, ctx, queries, assetID, repositoryID, dbtypes.FaceBoundingBox{X1: 300, Y1: 0, X2: 400, Y2: 100})
	owner := int32(1)
	people := make([]int32, 0, 2)
	for _, face := range []repo.FaceItem{left, right} {
		cluster, err := queries.CreateFaceCluster(ctx, repo.CreateFaceClusterParams{OwnerID: &owner, ConfidenceScore: 1, IsConfirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := queries.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
			ClusterID: cluster.ClusterID, FaceID: face.ID, SimilarityScore: 1, Confidence: 1, IsManual: true,
		}); err != nil {
			t.Fatal(err)
		}
		people = append(people, cluster.ClusterID)
	}

	assignments, err := captureManualFaceAssignments(ctx, queries, assetID)
	if err != nil || len(assignments) != 2 {
		t.Fatalf("captured %d manual assignments (%v), want 2", len(assignments), err)
	}
	// The edit re-detects both faces: the left one barely moved, the right
	// one moved enough to overlap less than half.
	if err := queries.DeleteFaceItemsByAsset(ctx, assetID); err != nil {
		t.Fatal(err)
	}
	newLeft := createFaceItem(t, ctx, queries, assetID, repositoryID, dbtypes.FaceBoundingBox{X1: 5, Y1: 5, X2: 105, Y2: 105})
	newRight := createFaceItem(t, ctx, queries, assetID, repositoryID, dbtypes.FaceBoundingBox{X1: 360, Y1: 0, X2: 460, Y2: 100})
	touched, err := reapplyManualFaceAssignments(ctx, queries, assignments, []repo.FaceItem{newLeft, newRight})
	if err != nil {
		t.Fatal(err)
	}
	if len(touched) != 2 {
		t.Fatalf("touched clusters %v, want both people", touched)
	}
	if cluster, manual, ok := faceMembership(t, ctx, database, newLeft.ID); !ok || cluster != people[0] || !manual {
		t.Fatalf("overlapping face: cluster %d manual %t (member %t), want person %d as manual", cluster, manual, ok, people[0])
	}
	if cluster, manual, ok := faceMembership(t, ctx, database, newRight.ID); !ok || cluster != people[1] || manual {
		t.Fatalf("moved face: cluster %d manual %t (member %t), want person %d as unconfirmed", cluster, manual, ok, people[1])
	}
}

// Re-extraction after an in-place edit keeps a description the user wrote and
// follows a caption that only came from the file.
func TestReextractedMetadataKeepsAUserEditedDescription(t *testing.T) {
	ctx := context.Background()
	database := openFaceClusteringDatabase(t, ctx)
	repositoryID := seedFaceClusteringRepository(t, ctx, database)
	edited, extracted := uuid.New(), uuid.New()
	for _, assetID := range []uuid.UUID{edited, extracted} {
		if _, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1, Filename: assetID.String() + ".jpg", FileSize: 1,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := database.SQL.ExecContext(ctx, `UPDATE assets SET specific_metadata = '{"description":"first caption"}' WHERE asset_id = ?`, assetID); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Queries.MutateAssetDescription(ctx, repo.UpdateAssetDescriptionParams{AssetID: edited, Description: "written by the user"}); err != nil {
		t.Fatal(err)
	}
	for _, assetID := range []uuid.UUID{edited, extracted} {
		tx, err := database.SQL.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ApplyAssetExtractedMetadataTx(ctx, tx, database.Queries.WithTx(tx), assetID,
			dbtypes.SpecificMetadata(`{"description":"caption rewritten by another tool"}`), dbtypes.CommonMetadata{}, nil, ""); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	description := func(assetID uuid.UUID) string {
		var value string
		if err := database.SQL.QueryRowContext(ctx, `SELECT json_extract(specific_metadata, '$.description') FROM assets WHERE asset_id = ?`, assetID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if got := description(edited); got != "written by the user" {
		t.Fatalf("user-edited description after re-extraction = %q", got)
	}
	if got := description(extracted); got != "caption rewritten by another tool" {
		t.Fatalf("extracted description after re-extraction = %q", got)
	}
}
