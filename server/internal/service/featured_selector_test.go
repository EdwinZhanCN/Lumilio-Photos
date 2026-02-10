package service

import (
	"fmt"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSelectFeaturedPhotos_DeterministicWithSameSeed(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	assets := make([]repo.Asset, 0, 20)
	for i := range 20 {
		assets = append(assets, testPhotoAsset(testPhotoAssetInput{
			Name:      fmt.Sprintf("asset-%d", i),
			TakenTime: now.AddDate(0, 0, -i),
			Width:     4000,
			Height:    3000,
			Rating:    3 + (i % 3),
			Liked:     i%4 == 0,
			Camera:    "Sony A7",
		}))
	}

	opts := FeaturedSelectionOptions{
		Count: 8,
		Seed:  "2026-02-10",
		Now:   now,
	}

	selectedA := SelectFeaturedPhotos(assets, opts)
	selectedB := SelectFeaturedPhotos(assets, opts)

	idsA := assetIDs(selectedA)
	idsB := assetIDs(selectedB)
	if len(idsA) != 8 {
		t.Fatalf("expected 8 selected assets, got %d", len(idsA))
	}
	if !equalStringSlices(idsA, idsB) {
		t.Fatalf("expected deterministic selection, got %v vs %v", idsA, idsB)
	}
}

func TestSelectFeaturedPhotos_DifferentSeedChangesSelection(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	assets := make([]repo.Asset, 0, 24)
	for i := range 24 {
		assets = append(assets, testPhotoAsset(testPhotoAssetInput{
			Name:      fmt.Sprintf("asset-%d", i),
			TakenTime: now.AddDate(0, 0, -i),
			Width:     4608,
			Height:    3072,
			Rating:    3,
			Liked:     i%5 == 0,
			Camera:    "Canon R6",
		}))
	}

	selectedA := SelectFeaturedPhotos(assets, FeaturedSelectionOptions{Count: 8, Seed: "seed-A", Now: now})
	selectedB := SelectFeaturedPhotos(assets, FeaturedSelectionOptions{Count: 8, Seed: "seed-B", Now: now})

	idsA := assetIDs(selectedA)
	idsB := assetIDs(selectedB)
	if equalStringSlices(idsA, idsB) {
		t.Fatalf("expected different selection for different seeds, got same %v", idsA)
	}
}

func TestSelectFeaturedPhotos_DeduplicatesByAssetID(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	base := testPhotoAsset(testPhotoAssetInput{
		Name:      "dup",
		TakenTime: now,
		Width:     4000,
		Height:    3000,
		Rating:    5,
		Liked:     true,
		Camera:    "Nikon Z7",
	})

	assets := []repo.Asset{base, base, base}
	for i := range 5 {
		assets = append(assets, testPhotoAsset(testPhotoAssetInput{
			Name:      fmt.Sprintf("asset-%d", i),
			TakenTime: now.AddDate(0, 0, -i-1),
			Width:     2000,
			Height:    1500,
			Rating:    2,
			Camera:    "Nikon Z7",
		}))
	}

	selected := SelectFeaturedPhotos(assets, FeaturedSelectionOptions{Count: 6, Seed: "seed", Now: now})
	ids := assetIDs(selected)

	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id selected: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestSelectFeaturedPhotos_RespectsDayCapWhenPossible(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	targetDay := time.Date(2026, 2, 9, 10, 0, 0, 0, time.UTC)
	assets := make([]repo.Asset, 0, 10)

	for i := range 6 {
		assets = append(assets, testPhotoAsset(testPhotoAssetInput{
			Name:      fmt.Sprintf("same-day-%d", i),
			TakenTime: targetDay.Add(time.Duration(i) * time.Minute),
			Width:     6000,
			Height:    4000,
			Rating:    5,
			Liked:     true,
			Camera:    "Fuji X-T5",
		}))
	}

	for i := range 4 {
		assets = append(assets, testPhotoAsset(testPhotoAssetInput{
			Name:      fmt.Sprintf("other-day-%d", i),
			TakenTime: now.AddDate(0, 0, -10-i),
			Width:     2400,
			Height:    1600,
			Rating:    1,
			Liked:     false,
			Camera:    fmt.Sprintf("Camera-%d", i),
		}))
	}

	selected := SelectFeaturedPhotos(assets, FeaturedSelectionOptions{
		Count:     4,
		Seed:      "day-cap-seed",
		Now:       now,
		MaxPerDay: 1,
	})

	if len(selected) != 4 {
		t.Fatalf("expected 4 selected assets, got %d", len(selected))
	}

	targetBucket := targetDay.Format("2006-01-02")
	hits := 0
	for _, a := range selected {
		meta, _ := decodePhotoMetadata(a)
		if buildDayBucket(a, meta) == targetBucket {
			hits++
		}
	}
	if hits > 1 {
		t.Fatalf("expected at most 1 selection from same day, got %d", hits)
	}
}

type testPhotoAssetInput struct {
	Name      string
	TakenTime time.Time
	Width     int32
	Height    int32
	Rating    int
	Liked     bool
	Camera    string
}

func testPhotoAsset(in testPhotoAssetInput) repo.Asset {
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(in.Name))
	photoMeta := dbtypes.PhotoSpecificMetadata{
		TakenTime:   timePtr(in.TakenTime),
		CameraModel: in.Camera,
	}
	meta, _ := dbtypes.MarshalMeta(photoMeta)

	rating := int32(in.Rating)
	liked := in.Liked
	width := in.Width
	height := in.Height

	return repo.Asset{
		AssetID:          pgtype.UUID{Bytes: id, Valid: true},
		Type:             string(dbtypes.AssetTypePhoto),
		Width:            &width,
		Height:           &height,
		Rating:           &rating,
		Liked:            &liked,
		SpecificMetadata: meta,
		TakenTime: pgtype.Timestamptz{
			Time:  in.TakenTime,
			Valid: true,
		},
		UploadTime: pgtype.Timestamptz{
			Time:  in.TakenTime.Add(2 * time.Hour),
			Valid: true,
		},
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func assetIDs(assets []repo.Asset) []string {
	out := make([]string, 0, len(assets))
	for _, a := range assets {
		out = append(out, assetUUIDString(a))
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
