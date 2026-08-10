package scanner

import (
	"testing"

	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/google/uuid"
)

func TestMatchMovesRequiresOneToOneFullContentMatch(t *testing.T) {
	t.Parallel()

	asset := repo.Asset{
		AssetID:     uuid.New(),
		FileSize:    128,
		ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	missingPath := mustRepositoryPath(t, "inbox/old.jpg")
	newPath := mustRepositoryPath(t, "Trips/new.jpg")
	contentHash := asset.ContentHash
	missing := map[string]missingCandidate{
		missingPath.String(): {
			storagePath: missingPath.String(),
			path:        missingPath,
			asset:       &asset,
		},
	}
	newFiles := map[string]observedCandidate{
		newPath.String(): {
			observation: storage.FileObservation{
				RepositoryID: uuid.New(),
				Path:         newPath,
				Size:         asset.FileSize,
				ContentHash:  &contentHash,
			},
		},
	}

	moves, ambiguousOld, ambiguousNew := matchMoves(missing, newFiles)
	if len(moves) != 1 || moves[0].oldPath != missingPath.String() || moves[0].newPath != newPath.String() {
		t.Fatalf("unexpected move decisions: %#v", moves)
	}
	if len(ambiguousOld) != 0 || len(ambiguousNew) != 0 {
		t.Fatalf("one-to-one match became ambiguous: old=%v new=%v", ambiguousOld, ambiguousNew)
	}
}

func TestMatchMovesDefersWholeDuplicateGroup(t *testing.T) {
	t.Parallel()

	contentHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	asset := repo.Asset{AssetID: uuid.New(), FileSize: 256, ContentHash: contentHash}
	oldPath := mustRepositoryPath(t, "inbox/old.jpg")
	firstNew := mustRepositoryPath(t, "Trips/one.jpg")
	secondNew := mustRepositoryPath(t, "Trips/two.jpg")
	missing := map[string]missingCandidate{
		oldPath.String(): {storagePath: oldPath.String(), path: oldPath, asset: &asset},
	}
	newFiles := map[string]observedCandidate{
		firstNew.String(): {
			observation: storage.FileObservation{Path: firstNew, Size: 256, ContentHash: &contentHash},
		},
		secondNew.String(): {
			observation: storage.FileObservation{Path: secondNew, Size: 256, ContentHash: &contentHash},
		},
	}

	moves, ambiguousOld, ambiguousNew := matchMoves(missing, newFiles)
	if len(moves) != 0 {
		t.Fatalf("ambiguous group produced move: %#v", moves)
	}
	oldGroup := ambiguousOld[oldPath.String()]
	if oldGroup == "" || ambiguousNew[firstNew.String()] != oldGroup || ambiguousNew[secondNew.String()] != oldGroup {
		t.Fatalf("group was not deferred together: old=%v new=%v", ambiguousOld, ambiguousNew)
	}
}

func TestMatchMovesDoesNotTreatExistingOldPathAsCandidate(t *testing.T) {
	t.Parallel()

	contentHash := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	newPath := mustRepositoryPath(t, "Trips/copy.jpg")
	newFiles := map[string]observedCandidate{
		newPath.String(): {
			observation: storage.FileObservation{Path: newPath, Size: 512, ContentHash: &contentHash},
		},
	}

	moves, ambiguousOld, ambiguousNew := matchMoves(nil, newFiles)
	if len(moves) != 0 || len(ambiguousOld) != 0 || len(ambiguousNew) != 0 {
		t.Fatalf("copy candidate was consumed: moves=%v old=%v new=%v", moves, ambiguousOld, ambiguousNew)
	}
}

func mustRepositoryPath(t *testing.T, value string) storage.RepositoryPath {
	t.Helper()
	parsed, err := storage.ParseUserMediaPath(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
