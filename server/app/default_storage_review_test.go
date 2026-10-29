package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func writeDefaultStorageMarkers(t *testing.T, root, storageLocationID, primaryID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "primary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".lumilioroot"), []byte(rootMarkerYAML(storageLocationID)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "primary", ".lumiliorepo"), []byte(repositoryMarkerYAML(primaryID, "1.0")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rootMarkerYAML(id string) string {
	return "version: \"1.0\"\nid: " + id + "\nname: Test Root\ncreated_at: 2026-01-01T00:00:00Z\n"
}

func repositoryMarkerYAML(id, version string) string {
	return "version: \"" + version + "\"\nid: " + id + "\nname: Primary\nstorage_strategy: date\nlocal_settings:\n  handle_duplicate_filenames: rename\ncreated_at: 2026-01-01T00:00:00Z\n"
}

// The Desktop asks the Server to read a candidate Default Storage Location:
// a fresh folder is accepted as fresh, the same portable identity elsewhere
// is accepted as a move, another identity is refused while the current path
// can still prove its own, a Repository folder is refused, and a marker this
// build cannot read refuses rather than guesses.
func TestReviewDefaultStorageMoveReadsMarkersForTheDesktop(t *testing.T) {
	base := t.TempDir()
	current := filepath.Join(base, "current")
	storageLocationID, primaryID := uuid.NewString(), uuid.NewString()
	writeDefaultStorageMarkers(t, current, storageLocationID, primaryID)

	for name, candidate := range map[string]string{"absent": filepath.Join(base, "absent"), "empty": filepath.Join(base, "empty")} {
		if name == "empty" {
			if err := os.MkdirAll(candidate, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		review, err := ReviewDefaultStorageMove(current, candidate)
		if err != nil || !review.Fresh || review.StorageLocationID != "" || review.PrimaryRepositoryID != "" {
			t.Fatalf("%s candidate review = %+v, %v; want fresh", name, review, err)
		}
	}
	rootOnly := filepath.Join(base, "root-only")
	if err := os.MkdirAll(rootOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootOnly, ".lumilioroot"), []byte(rootMarkerYAML(uuid.NewString())), 0o644); err != nil {
		t.Fatal(err)
	}
	if review, err := ReviewDefaultStorageMove(current, rootOnly); err != nil || !review.Fresh || review.StorageLocationID == "" {
		t.Fatalf("root-only candidate review = %+v, %v; want fresh with its root identity", review, err)
	}

	moved := filepath.Join(base, "moved")
	writeDefaultStorageMarkers(t, moved, storageLocationID, primaryID)
	review, err := ReviewDefaultStorageMove(current, moved)
	if err != nil || review.Fresh || review.StorageLocationID != storageLocationID || review.PrimaryRepositoryID != primaryID {
		t.Fatalf("moved candidate review = %+v, %v", review, err)
	}
	if _, err := ReviewDefaultStorageMove(filepath.Join(base, "gone"), moved); err != nil {
		t.Fatalf("moved candidate with the current path offline = %v, want accepted for the Server's own comparison", err)
	}

	otherPrimary := filepath.Join(base, "other-primary")
	writeDefaultStorageMarkers(t, otherPrimary, storageLocationID, uuid.NewString())
	if _, err := ReviewDefaultStorageMove(current, otherPrimary); !errors.Is(err, ErrDefaultStorageIdentityDiffers) {
		t.Fatalf("different primary identity = %v, want ErrDefaultStorageIdentityDiffers", err)
	}
	otherRoot := filepath.Join(base, "other-root")
	writeDefaultStorageMarkers(t, otherRoot, uuid.NewString(), primaryID)
	if _, err := ReviewDefaultStorageMove(current, otherRoot); !errors.Is(err, ErrDefaultStorageIdentityDiffers) {
		t.Fatalf("different root identity = %v, want ErrDefaultStorageIdentityDiffers", err)
	}

	repositoryFolder := filepath.Join(base, "repository")
	if err := os.MkdirAll(repositoryFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryFolder, ".lumiliorepo"), []byte(repositoryMarkerYAML(uuid.NewString(), "1.0")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReviewDefaultStorageMove(current, repositoryFolder); !errors.Is(err, ErrDefaultStorageCandidateIsRepository) {
		t.Fatalf("repository folder = %v, want ErrDefaultStorageCandidateIsRepository", err)
	}

	corrupt := filepath.Join(base, "corrupt")
	if err := os.MkdirAll(corrupt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corrupt, ".lumilioroot"), []byte("version: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReviewDefaultStorageMove(current, corrupt); !errors.Is(err, ErrDefaultStorageMarkerUnreadable) {
		t.Fatalf("corrupt root marker = %v, want ErrDefaultStorageMarkerUnreadable", err)
	}
	newer := filepath.Join(base, "newer")
	writeDefaultStorageMarkers(t, newer, storageLocationID, primaryID)
	if err := os.WriteFile(filepath.Join(newer, "primary", ".lumiliorepo"), []byte(repositoryMarkerYAML(primaryID, "9.0")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReviewDefaultStorageMove(current, newer); !errors.Is(err, ErrDefaultStorageMarkerUnreadable) {
		t.Fatalf("unsupported primary marker version = %v, want ErrDefaultStorageMarkerUnreadable", err)
	}
}
