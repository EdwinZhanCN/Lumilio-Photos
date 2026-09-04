//go:build sqlite_fts5

package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	aggregatesearch "server/internal/search"
	"server/internal/testutil"
	"server/internal/utils/imagesource"
	"server/internal/utils/imaging"

	"github.com/google/uuid"
)

func TestSearchVisualSimilar_RanksExcludesAndSkipsLumen(t *testing.T) {
	ctx := context.Background()
	catalog := openVisualSearchCatalog(t)
	lumen := &semanticTestLumenStub{available: true, modelID: "fixture", vector: unitVector768(0)}
	svc, err := NewAssetService(catalog.Queries, catalog.SQL, lumen, NewEmbeddingService(catalog.Queries, catalog.SQL), nil)
	if err != nil {
		t.Fatal(err)
	}

	queryID := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	pairID := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	nearID := uuid.MustParse("00000000-0000-0000-0000-000000000013")
	farID := uuid.MustParse("00000000-0000-0000-0000-000000000014")
	missingID := uuid.MustParse("00000000-0000-0000-0000-000000000015")
	videoID := uuid.MustParse("00000000-0000-0000-0000-000000000016")
	earlyFrameMatchID := uuid.MustParse("00000000-0000-0000-0000-000000000017")

	ownerID := int32(1)
	result, err := svc.SearchBrowseItems(ctx, SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{
			OwnerID: &ownerID,
			Limit:   50,
			Offset:  0,
		},
		TopResultsLimit:  200,
		SimilarToAssetID: &queryID,
	})
	if err != nil {
		t.Fatalf("SearchBrowseItems: %v", err)
	}
	if lumen.fastCalls != 0 || lumen.normalCalls != 0 {
		t.Fatalf("catalog similar search called lumen text embed: fast=%d normal=%d", lumen.fastCalls, lumen.normalCalls)
	}
	if !result.TopResultsMeta.Enabled || len(result.TopResultsMeta.SourceTypes) != 1 || result.TopResultsMeta.SourceTypes[0] != aggregatesearch.SourceEmbedding {
		t.Fatalf("meta = %+v, want embedding-only enabled", result.TopResultsMeta)
	}
	if len(result.TopResults) != 0 {
		t.Fatalf("top results = %d, want empty", len(result.TopResults))
	}
	got := browsePrimaryIDs(result)
	if len(got) != 1 || got[0] != nearID {
		t.Fatalf("ranked ids = %v, want near only (self/pair excluded, far below floor)", got)
	}

	_, err = svc.SearchBrowseItems(ctx, SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{OwnerID: &ownerID, Limit: 50},
		TopResultsLimit:   200,
		SimilarToAssetID:  &missingID,
	})
	if !errors.Is(err, ErrEmbeddingMissing) {
		t.Fatalf("missing embedding error = %v, want ErrEmbeddingMissing", err)
	}

	videoResult, err := svc.SearchBrowseItems(ctx, SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{OwnerID: &ownerID, Limit: 50},
		TopResultsLimit:   200,
		SimilarToAssetID:  &videoID,
	})
	if err != nil {
		t.Fatalf("video similar search: %v", err)
	}
	videoIDs := browsePrimaryIDs(videoResult)
	if len(videoIDs) == 0 || videoIDs[0] != earlyFrameMatchID {
		t.Fatalf("video query ranked ids = %v, want earliest-frame match first", videoIDs)
	}

	_ = pairID
	_ = farID
}

func TestSearchByImage_ThumbnailsBeforeLumen(t *testing.T) {
	imaging.StartVips()

	ctx := context.Background()
	catalog := openVisualSearchCatalog(t)
	jpeg := visualSearchJPEG(t)
	lumen := &semanticTestLumenStub{
		imageEmbedAvailable: true,
		modelID:             "fixture",
		vector:              unitVector768(0),
	}
	svc, err := NewAssetService(catalog.Queries, catalog.SQL, lumen, NewEmbeddingService(catalog.Queries, catalog.SQL), nil)
	if err != nil {
		t.Fatal(err)
	}

	ownerID := int32(1)
	result, err := svc.SearchBrowseItems(ctx, SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{
			OwnerID: &ownerID,
			Limit:   50,
		},
		TopResultsLimit:    200,
		QueryImage:         jpeg,
		QueryImageFilename: "query.jpg",
	})
	if err != nil {
		t.Fatalf("SearchBrowseItems bytes: %v", err)
	}
	if lumen.imageCalls != 1 {
		t.Fatalf("image embed calls = %d, want 1", lumen.imageCalls)
	}
	assertSemanticQueryWasThumbnailed(t, lumen.lastImage)
	got := browsePrimaryIDs(result)
	if len(got) != 2 {
		t.Fatalf("ranked ids = %v, want query+near (same vector, query media not excluded for file search)", got)
	}

	path := filepath.Join(t.TempDir(), "query.jpg")
	if err := os.WriteFile(path, jpeg, 0o600); err != nil {
		t.Fatal(err)
	}
	lumen.imageCalls = 0
	lumen.lastImage = nil
	if _, err := svc.SearchBrowseItems(ctx, SearchAssetsParams{
		QueryAssetsParams: QueryAssetsParams{
			OwnerID: &ownerID,
			Limit:   50,
		},
		TopResultsLimit:    200,
		QueryImagePath:     path,
		QueryImageFilename: "query.jpg",
	}); err != nil {
		t.Fatalf("SearchBrowseItems path: %v", err)
	}
	if lumen.imageCalls != 1 {
		t.Fatalf("path image embed calls = %d, want 1", lumen.imageCalls)
	}
	assertSemanticQueryWasThumbnailed(t, lumen.lastImage)

	_, err = svc.SearchBrowseItems(ctx, SearchAssetsParams{
		QueryAssetsParams:  QueryAssetsParams{OwnerID: &ownerID, Limit: 50},
		QueryImage:         []byte("not-an-image"),
		QueryImageFilename: "query.jpg",
	})
	if !errors.Is(err, ErrInvalidImageQuery) {
		t.Fatalf("invalid image error = %v, want ErrInvalidImageQuery", err)
	}
}

func assertSemanticQueryWasThumbnailed(t *testing.T, image *imagesource.MLImage) {
	t.Helper()
	if image == nil {
		t.Fatal("lumen received no image")
	}
	if image.Width != 224 || image.Height != 224 {
		t.Fatalf("tensor shape = %dx%d, want 224x224", image.Width, image.Height)
	}
	src := image.EncodedSource
	if len(src) < 12 || string(src[:4]) != "RIFF" || string(src[8:12]) != "WEBP" {
		t.Fatalf("Lumen source is not the medium WebP thumbnail")
	}
}

func visualSearchJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func browsePrimaryIDs(result SearchBrowseResult) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(result.Results))
	for _, item := range result.Results {
		if item.MediaItem == nil {
			continue
		}
		ids = append(ids, item.MediaItem.PrimaryAsset.AssetID)
	}
	return ids
}

func openVisualSearchCatalog(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(directory, "visual-search.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	queryID := "00000000-0000-0000-0000-000000000011"
	pairID := "00000000-0000-0000-0000-000000000012"
	nearID := "00000000-0000-0000-0000-000000000013"
	farID := "00000000-0000-0000-0000-000000000014"
	missingID := "00000000-0000-0000-0000-000000000015"
	videoID := "00000000-0000-0000-0000-000000000016"
	earlyMatchID := "00000000-0000-0000-0000-000000000017"
	queryItem := "00000000-0000-0000-0000-000000000021"
	nearItem := "00000000-0000-0000-0000-000000000023"
	farItem := "00000000-0000-0000-0000-000000000024"
	missingItem := "00000000-0000-0000-0000-000000000025"
	videoItem := "00000000-0000-0000-0000-000000000026"
	earlyItem := "00000000-0000-0000-0000-000000000027"
	repoID := "00000000-0000-0000-0000-000000000002"
	rootID := "00000000-0000-0000-0000-000000000001"

	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at, webauthn_user_handle
		) VALUES (1, 'owner-one', 'hash', 1, 1, x'01');
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES (?, 'root', '/media', 'default', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at, default_owner_id, root_id
		) VALUES (?, 'repo', '/media/repo', 1, 1, 1, ?);
		INSERT INTO embedding_spaces (
			id, embedding_type, model_id, dimensions, distance_metric,
			search_enabled, is_default_search, created_at, updated_at
		) VALUES (1, 'semantic', 'fixture', 768, 'l2', 1, 1, 1, 1);
	`, rootID, repoID, rootID); err != nil {
		t.Fatal(err)
	}

	insertAsset := func(id, filename, assetType string) {
		t.Helper()
		mimeType := "image/jpeg"
		if assetType == "VIDEO" {
			mimeType = "video/mp4"
		}
		if _, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
			AssetID: uuid.MustParse(id), RepositoryID: uuid.MustParse(repoID), OwnerID: 1,
			AssetType: assetType, Filename: filename, MIMEType: mimeType, FileSize: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	insertAsset(queryID, "query.jpg", "PHOTO")
	insertAsset(pairID, "query.dng", "PHOTO")
	insertAsset(nearID, "near.jpg", "PHOTO")
	insertAsset(farID, "far.jpg", "PHOTO")
	insertAsset(missingID, "missing.jpg", "PHOTO")
	insertAsset(videoID, "clip.mp4", "VIDEO")
	insertAsset(earlyMatchID, "early.jpg", "PHOTO")

	insertMedia := func(itemID, primaryID, kind string, members ...string) {
		t.Helper()
		if _, err := catalog.SQL.ExecContext(ctx, `
			INSERT INTO media_items (
				media_item_id, owner_id, repository_id, media_kind, primary_asset_id, created_at, updated_at
			) VALUES (?, 1, ?, ?, ?, 1, 1)
		`, itemID, repoID, kind, primaryID); err != nil {
			t.Fatal(err)
		}
		for i, member := range members {
			if _, err := catalog.SQL.ExecContext(ctx, `
				INSERT INTO media_item_assets (asset_id, media_item_id, relation, position, created_at)
				VALUES (?, ?, 'original', ?, 1)
			`, member, itemID, i); err != nil {
				t.Fatal(err)
			}
		}
	}
	insertMedia(queryItem, queryID, "photo", queryID, pairID)
	insertMedia(nearItem, nearID, "photo", nearID)
	insertMedia(farItem, farID, "photo", farID)
	insertMedia(missingItem, missingID, "photo", missingID)
	insertMedia(videoItem, videoID, "video", videoID)
	insertMedia(earlyItem, earlyMatchID, "photo", earlyMatchID)

	insertVector := func(assetID string, vec []float32, frame *int64) {
		t.Helper()
		if _, err := catalog.SQL.ExecContext(ctx, `
			INSERT INTO search_embeddings (
				asset_id, space_id, frame_ts_ms, vector, model_id, created_at
			) VALUES (?, 1, ?, ?, 'fixture', 1)
		`, assetID, frame, dbtypes.NewVector(vec)); err != nil {
			t.Fatal(err)
		}
	}
	insertVector(queryID, unitVector768(0), nil)
	insertVector(pairID, unitVector768(0), nil)
	insertVector(nearID, unitVector768(0), nil)
	insertVector(farID, unitVector768(1), nil)
	late := int64(500)
	early := int64(100)
	insertVector(videoID, unitVector768(1), &late)
	insertVector(videoID, unitVector768(2), &early)
	insertVector(earlyMatchID, unitVector768(2), nil)

	return catalog
}

func unitVector768(index int) []float32 {
	vector := make([]float32, 768)
	vector[index] = 1
	return vector
}
