//go:build sqlite_fts5

package search

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/testutil"

	"github.com/google/uuid"
)

func TestEmbeddingRetrieverUsesVec1FiltersAndExactReranking(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(directory, "search.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		ownerAsset   = "00000000-0000-0000-0000-000000000011"
		otherAsset   = "00000000-0000-0000-0000-000000000012"
		deletedAsset = "00000000-0000-0000-0000-000000000013"
		farAsset     = "00000000-0000-0000-0000-000000000014"
	)
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at, webauthn_user_handle
		) VALUES
			(1, 'owner-one', 'hash', 1, 1, x'01'),
			(2, 'owner-two', 'hash', 1, 1, x'02');
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000001',
			'root', '/media', 'default', 1, 1
		);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at, default_owner_id, root_id
		) VALUES (
			'00000000-0000-0000-0000-000000000002',
			'repo', '/media/repo', 1, 1, 1,
			'00000000-0000-0000-0000-000000000001'
		);
		INSERT INTO embedding_spaces (
			id, embedding_type, model_id, dimensions, distance_metric,
			search_enabled, is_default_search, created_at, updated_at
		) VALUES (1, 'semantic', 'fixture', 768, 'l2', 1, 1, 1, 1);
	`); err != nil {
		t.Fatal(err)
	}
	repositoryID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	for _, fixture := range []struct {
		id, filename, assetType, mime string
		ownerID                       int32
		deleted                       bool
	}{
		{id: ownerAsset, filename: "owner.jpg", assetType: "PHOTO", mime: "image/jpeg", ownerID: 1},
		{id: otherAsset, filename: "other.jpg", assetType: "PHOTO", mime: "image/jpeg", ownerID: 2},
		{id: deletedAsset, filename: "deleted.jpg", assetType: "PHOTO", mime: "image/jpeg", ownerID: 1, deleted: true},
		{id: farAsset, filename: "far.mp4", assetType: "VIDEO", mime: "video/mp4", ownerID: 1},
	} {
		if _, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
			AssetID: uuid.MustParse(fixture.id), RepositoryID: repositoryID, OwnerID: fixture.ownerID,
			AssetType: fixture.assetType, Filename: fixture.filename, MIMEType: fixture.mime,
			FileSize: 1, IsDeleted: fixture.deleted,
		}); err != nil {
			t.Fatal(err)
		}
	}

	queryVector := unitVector768(0)
	farVector := unitVector768(1)
	for _, fixture := range []struct {
		asset string
		vec   []float32
	}{
		{ownerAsset, queryVector},
		{otherAsset, queryVector},
		{deletedAsset, queryVector},
		{farAsset, farVector},
	} {
		if _, err := catalog.SQL.ExecContext(ctx, `
			INSERT INTO search_embeddings (
				asset_id, space_id, vector, model_id, created_at
			) VALUES (?, 1, ?, 'fixture', 1)
		`, fixture.asset, dbtypes.NewVector(fixture.vec)); err != nil {
			t.Fatal(err)
		}
	}

	retriever := NewEmbeddingRetriever(
		catalog.SQL,
		func(context.Context, string, bool) (QueryEmbedding, error) {
			return QueryEmbedding{Model: "fixture", Vector: queryVector}, nil
		},
		func(context.Context, string, int) (repo.EmbeddingSpace, error) {
			return repo.EmbeddingSpace{
				ID:             1,
				ModelID:        "fixture",
				Dimensions:     768,
				DistanceMetric: "l2",
			}, nil
		},
		1,
	)

	ownerID := int32(1)
	candidates, err := retriever.Retrieve(ctx, Request{
		Query:  "fixture",
		TopK:   2,
		Filter: Filter{OwnerID: &ownerID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(candidates))
	}
	if candidates[0].AssetID != uuid.MustParse(ownerAsset) || candidates[1].AssetID != uuid.MustParse(farAsset) {
		t.Fatalf("candidate order = %v, want owner then far", candidates)
	}
	if math.Abs(candidates[0].RawScore) > 1e-6 ||
		math.Abs(candidates[1].RawScore-math.Sqrt2) > 1e-6 {
		t.Fatalf("exact reranked distances = %v, want 0 and sqrt(2)", candidates)
	}

	photos, err := retriever.Retrieve(ctx, Request{
		Query: "fixture",
		TopK:  2,
		Filter: Filter{
			OwnerID:    &ownerID,
			AssetTypes: []string{"PHOTO"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 1 || photos[0].AssetID != uuid.MustParse(ownerAsset) {
		t.Fatalf("photo-filtered Vec1 candidates = %v, want owner photo only", photos)
	}

	strict, meta, err := retriever.RetrieveSet(
		ctx,
		Request{Query: "fixture", Filter: Filter{OwnerID: &ownerID}},
		StrictnessStrict,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(strict) != 1 || strict[0].AssetID != uuid.MustParse(ownerAsset) {
		t.Fatalf("strict semantic set = %v, want owner asset only", strict)
	}
	if !meta.Exact || !meta.Complete {
		t.Fatalf("strict semantic metadata = %+v, want exact and complete", meta)
	}
}

func unitVector768(index int) []float32 {
	vector := make([]float32, 768)
	vector[index] = 1
	return vector
}
