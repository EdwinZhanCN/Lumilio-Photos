package service

import (
	"fmt"
	"testing"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func burstCandidate(index int, filename string, capturedAt time.Time, burstID string) repo.FindMediaItemsForBurstDetectionRow {
	itemID := uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", index+1))
	assetID := uuid.MustParse(fmt.Sprintf("10000000-0000-0000-0000-%012d", index+1))
	ownerID := int32(7)
	return repo.FindMediaItemsForBurstDetectionRow{
		MediaItemID:      pgtype.UUID{Bytes: itemID, Valid: true},
		OwnerID:          &ownerID,
		RepositoryID:     pgtype.UUID{Bytes: uuid.MustParse("20000000-0000-0000-0000-000000000001"), Valid: true},
		PrimaryAssetID:   pgtype.UUID{Bytes: assetID, Valid: true},
		OriginalFilename: filename,
		TakenTime:        pgtype.Timestamptz{Time: capturedAt, Valid: true},
		UploadTime:       pgtype.Timestamptz{Time: capturedAt.Add(time.Hour), Valid: true},
		CameraModel:      "Example Camera",
		BurstID:          burstID,
	}
}

func TestBurstClustersPreferExactMetadata(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	clusters := burstClusters([]repo.FindMediaItemsForBurstDetectionRow{
		burstCandidate(0, "IMG_1001.JPG", base, "burst-a"),
		burstCandidate(1, "IMG_1030.JPG", base.Add(3*time.Second), "burst-a"),
	})
	if len(clusters) != 1 {
		t.Fatalf("expected one exact metadata burst, got %d", len(clusters))
	}
	if got := len(clusters[0].Members); got != 2 {
		t.Fatalf("expected two exact burst members, got %d", got)
	}
	if clusters[0].GroupKey == "" || clusters[0].GroupKey[:5] != "exif:" {
		t.Fatalf("expected EXIF group key, got %q", clusters[0].GroupKey)
	}
}

func TestBurstClustersConservativeFilenameFallback(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	candidates := []repo.FindMediaItemsForBurstDetectionRow{
		burstCandidate(0, "IMG_1001.JPG", base, ""),
		burstCandidate(1, "IMG_1002.JPG", base.Add(250*time.Millisecond), ""),
		burstCandidate(2, "IMG_1003.JPG", base.Add(500*time.Millisecond), ""),
	}
	clusters := burstClusters(candidates)
	if len(clusters) != 1 || len(clusters[0].Members) != 3 {
		t.Fatalf("expected one three-frame fallback burst, got %#v", clusters)
	}

	clusters = burstClusters(candidates[:2])
	if len(clusters) != 0 {
		t.Fatalf("two timestamp-only candidates must not auto-stack, got %#v", clusters)
	}

	candidates[2].OriginalFilename = "IMG_1005.JPG"
	clusters = burstClusters(candidates)
	if len(clusters) != 0 {
		t.Fatalf("non-consecutive filenames must not auto-stack, got %#v", clusters)
	}
}

func TestFilenameSequence(t *testing.T) {
	prefix, sequence, ok := filenameSequence("IMG_0042.CR3")
	if !ok || prefix != "img_" || sequence != 42 {
		t.Fatalf("unexpected sequence parse: prefix=%q sequence=%d ok=%v", prefix, sequence, ok)
	}
	if _, _, ok := filenameSequence("vacation.jpg"); ok {
		t.Fatal("filename without a numeric suffix must not qualify for burst fallback")
	}
}
