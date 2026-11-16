package service

import (
	"context"
	"fmt"
	"testing"

	"server/internal/catalog/dbtypes"
	"server/internal/catalog/repo"
	"server/internal/foundation/testutil"
	"server/internal/storage/artifact"

	"github.com/google/uuid"
)

// A person's cover is a member face that has a crop. A representative face
// without a crop, or one that is not a member, gives way to the best member
// face with a crop; a person whose faces have no crops has no cover.
func TestPersonCoverIsAMemberFaceWithACrop(t *testing.T) {
	ctx := context.Background()
	database := openFaceClusteringDatabase(t, ctx)
	repositoryID := seedFaceClusteringRepository(t, ctx, database)
	modelID := "fixture-face-v1"
	face := func(name string, crop bool, confidence float64) (int32, uuid.UUID, artifact.Identity) {
		t.Helper()
		assetID := uuid.New()
		occurrence, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
			Filename: name + ".jpg", MIMEType: "image/jpeg", FileSize: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Queries.CreateFaceResult(ctx, repo.CreateFaceResultParams{AssetID: assetID, ModelID: modelID, TotalFaces: 1}); err != nil {
			t.Fatal(err)
		}
		params := repo.CreateFaceItemParams{
			AssetID: assetID, BoundingBox: dbtypes.JSON(`{"x":0,"y":0,"width":10,"height":10}`),
			Confidence: confidence, EmbeddingModel: &modelID, RepositoryID: repositoryID,
		}
		identity := artifact.Identity{SourceFence: occurrence.ContentID.String(), Stage: artifact.StageFaces, PipelineVersion: faceCropVersion, Name: "0-" + name + ".webp"}
		if crop {
			params.CropFence, params.CropVersion, params.CropName = &identity.SourceFence, &identity.PipelineVersion, &identity.Name
		}
		item, err := database.Queries.CreateFaceItem(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		return item.ID, assetID, identity
	}
	cluster := func(id, representative int32, members ...int32) {
		t.Helper()
		if _, err := database.SQL.ExecContext(ctx, `INSERT INTO face_clusters (cluster_id, owner_id, representative_face_id, created_at, updated_at) VALUES (?, 1, ?, 1, 1)`, id, representative); err != nil {
			t.Fatal(err)
		}
		for _, member := range members {
			if _, err := database.SQL.ExecContext(ctx, `INSERT INTO face_cluster_members (cluster_id, face_id, similarity_score, confidence, created_at) VALUES (?, ?, 1, 1, 1)`, id, member); err != nil {
				t.Fatal(err)
			}
		}
	}
	uncropped, _, _ := face("uncropped", false, 0.99)
	cropped, croppedAsset, croppedArtifact := face("cropped", true, 0.5)
	alone, _, _ := face("alone", false, 0.9)
	cluster(1, uncropped, uncropped, cropped)
	cluster(2, cropped, alone)

	ownerID := int32(1)
	faces := NewFaceService(database.Queries, nil, database.Writer)
	people, _, err := faces.ListPeople(ctx, uuid.NullUUID{}, &ownerID, false, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	covers := map[int32]string{}
	for _, person := range people {
		cover := "none"
		if person.CoverFaceID != nil {
			cover = fmt.Sprint(*person.CoverFaceID)
		}
		covers[person.PersonID] = cover
	}
	if covers[1] != fmt.Sprint(cropped) || covers[2] != "none" {
		t.Fatalf("covers = %v; want person 1 on face %d and person 2 without a cover", covers, cropped)
	}

	person, err := faces.GetPerson(ctx, 1, uuid.NullUUID{}, &ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if person.CoverFaceID == nil || *person.CoverFaceID != cropped || person.RepresentativeAssetID == nil || *person.RepresentativeAssetID != croppedAsset.String() {
		t.Fatalf("person 1 = %+v; want the cropped face and its asset", person)
	}
	crop, err := faces.GetPersonFaceCrop(ctx, 1, *person.CoverFaceID, uuid.NullUUID{}, &ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if crop.Artifact != croppedArtifact || crop.RepositoryID != repositoryID.String() {
		t.Fatalf("cover crop = %+v; want %+v", crop, croppedArtifact)
	}
}
