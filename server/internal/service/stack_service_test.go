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

func structuralCandidate(index int, filename string, capturedAt time.Time) repo.FindCandidatesForStackingByNameRow {
	itemID := uuid.MustParse(fmt.Sprintf("30000000-0000-0000-0000-%012d", index+1))
	assetID := uuid.MustParse(fmt.Sprintf("40000000-0000-0000-0000-%012d", index+1))
	ownerID := int32(7)
	return repo.FindCandidatesForStackingByNameRow{
		AssetID:          pgtype.UUID{Bytes: assetID, Valid: true},
		MediaItemID:      pgtype.UUID{Bytes: itemID, Valid: true},
		OwnerID:          &ownerID,
		OriginalFilename: filename,
		TakenTime:        pgtype.Timestamptz{Time: capturedAt, Valid: true},
		UploadTime:       pgtype.Timestamptz{Time: capturedAt.Add(time.Hour), Valid: true},
	}
}

func TestTimeClusterDoesNotTreatNumericSequenceAsEdits(t *testing.T) {
	base := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	candidates := []repo.FindCandidatesForStackingByNameRow{
		structuralCandidate(0, "scan-001.jpg", base),
		structuralCandidate(1, "scan-002.jpg", base.Add(time.Second)),
		structuralCandidate(2, "scan-003.jpg", base.Add(2*time.Second)),
	}
	if clusters := timeCluster(candidates); len(clusters) != 0 {
		t.Fatalf("ordinary numeric filename sequence must not merge as edit iterations, got %#v", clusters)
	}
}

func TestTimeClusterRequiresUnsuffixedEditAnchor(t *testing.T) {
	base := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	candidates := []repo.FindCandidatesForStackingByNameRow{
		structuralCandidate(0, "portrait.jpg", base),
		structuralCandidate(1, "portrait-1.jpg", base.Add(time.Second)),
		structuralCandidate(2, "portrait-2.jpg", base.Add(2*time.Second)),
	}
	clusters := timeCluster(candidates)
	if len(clusters) != 1 || !clusters[0].HasAnchoredIteration || len(clusters[0].Members) != 3 {
		t.Fatalf("expected anchored edit iteration cluster, got %#v", clusters)
	}
}

func TestTimeClusterKeepsNumericRawJPEGPair(t *testing.T) {
	base := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	candidates := []repo.FindCandidatesForStackingByNameRow{
		structuralCandidate(0, "DSC-001.CR3", base),
		structuralCandidate(1, "DSC-001.JPG", base.Add(time.Second)),
	}
	clusters := timeCluster(candidates)
	if len(clusters) != 1 || clusters[0].BaseName != "dsc-001" || len(clusters[0].Members) != 2 {
		t.Fatalf("expected numeric RAW/JPEG structural pair, got %#v", clusters)
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
