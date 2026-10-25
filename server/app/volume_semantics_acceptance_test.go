package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"server/internal/storage"
	"server/internal/storage/pathsemantics"
	"server/internal/storage/testfixture"

	"github.com/google/uuid"
)

// R11 over the wired runtime and real foreign volumes: the filesystem-image
// CI jobs mount FAT32, exFAT, plain and case-folding ext4 (Linux) or APFS,
// case-sensitive APFS, HFS+ and ExFAT (macOS) images and name them in
// LUMILIO_VOLUME_FIXTURES. A Repository created on such a volume keys under
// the volume's own rule; one moved onto it from the host's filesystem is
// reconnected through the native task, its index rekeyed, and the scan that
// follows finds nothing missing and nothing new; a tree the destination
// cannot tell apart is refused with the pair named and nothing changed.
func TestVolumeReconnectAcrossFilesystemImages(t *testing.T) {
	fixtures := testfixture.VolumeFixtures(t, os.Getenv(testfixture.VolumeFixturesEnv))
	h := startReconnectHarness(t)
	source := h.externalDisk("host-disk")
	sourceRule := h.locationRule(source)

	for _, fixture := range fixtures {
		t.Run(fixture.Kind, func(t *testing.T) {
			// The harness reports through the running subtest, so one image's
			// failure is that image's, not the parent's.
			parent := h.t
			h.t = t
			defer func() { h.t = parent }()
			volume := h.externalDiskAt(fixture.Kind, filepath.Join(fixture.Path, "lumilio-"+fixture.Kind+"-"+uuid.NewString()[:8]))

			// A Repository created on the volume is keyed under the volume's
			// rule from its first file.
			native := h.createIn(volume, "Native "+fixture.Kind, "native")
			h.upload(native, "Native-"+fixture.Kind+".JPG", blackboxJPEG(t, 90, "native-"+fixture.Kind))
			h.waitProcessingSettled()
			if rule := h.repositoryRule(native.ID); !rule.Equal(fixture.Expected) {
				t.Fatalf("repository created on %s keys under %s, want %s", fixture.Kind, rule, fixture.Expected)
			}
			h.assertIndexKeyedUnder(native.ID, fixture.Expected)

			// A Repository moved onto the volume from the host filesystem is
			// reconnected and rekeyed; the next scan writes no absence.
			moved := h.createIn(source, "Moved "+fixture.Kind, "moved-"+fixture.Kind)
			// File names carry the image kind: the Asset listing spans every
			// Repository, and each image runs the same journey.
			beach := "Beach-" + fixture.Kind + ".JPG"
			h.upload(moved, beach, blackboxJPEG(t, 120, "beach-"+fixture.Kind))
			h.upload(moved, "Café-"+fixture.Kind+".jpg", blackboxJPEG(t, 140, "cafe-"+fixture.Kind))
			h.waitProcessingSettled()
			assets := h.listByFilename(beach)
			if len(assets) != 1 {
				t.Fatalf("uploaded asset = %+v", assets)
			}
			before := originalHashes(t, moved.Path)
			entriesBefore := h.entries(moved.ID)
			destination := filepath.Join(volume.Path, "moved-"+fixture.Kind)
			moveAcrossVolumes(t, moved.Path, destination)
			task := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": moved.ID}), destination)
			if task.Status != "succeeded" {
				t.Fatalf("reconnect onto %s = %+v problem=%+v refusal=%+v", fixture.Kind, task, task.Problem, task.Refusal)
			}
			if rule := h.repositoryRule(moved.ID); !rule.Equal(fixture.Expected) {
				t.Fatalf("repository moved onto %s keys under %s, want %s", fixture.Kind, rule, fixture.Expected)
			}
			row := h.viewRepository(moved.ID)
			if row.Reachability != "active" || row.StorageLocationID != volume.ID {
				t.Fatalf("reconnected row = %+v", row)
			}
			canonicalDestination := canonicalDir(t, destination)
			h.waitIdle(blackboxRepository{ID: moved.ID, Name: moved.Name, Path: canonicalDestination})
			h.waitProcessingSettled()
			h.assertIndexKeyedUnder(moved.ID, fixture.Expected)
			entriesAfter := h.entries(moved.ID)
			for path, entry := range entriesBefore {
				current, ok := entriesAfter[path]
				if !ok || current.EntryID != entry.EntryID || current.State != entry.State {
					t.Fatalf("the scan after the reconnect changed %s: before=%+v after=%+v", path, entry, current)
				}
			}
			if len(entriesAfter) != len(entriesBefore) {
				t.Fatalf("the scan after the reconnect changed the index: %d rows, want %d", len(entriesAfter), len(entriesBefore))
			}
			if listed := h.listByFilename(beach); len(listed) != 1 || listed[0].AssetID != assets[0].AssetID {
				t.Fatalf("asset identity after reconnect = %+v, want %s", listed, assets[0].AssetID)
			}
			if status := h.assetStatus(assets[0].AssetID); status != http.StatusOK {
				t.Fatalf("asset after reconnect = %d", status)
			}
			if after := originalHashes(t, canonicalDestination); !sameHashes(before, after) {
				t.Fatalf("reconnect changed original bytes: before=%v after=%v", before, after)
			}

			// Two names the host tells apart but the volume does not: the
			// reconnect is refused with the pair named, the row and the index
			// are unchanged, and the bytes on the volume are untouched.
			if sourceRule.Case == pathsemantics.CaseSensitive && fixture.Expected.Case == pathsemantics.CaseInsensitive {
				pair := h.createIn(source, "Pair "+fixture.Kind, "pair-"+fixture.Kind)
				h.upload(pair, "Pair-"+fixture.Kind+".jpg", blackboxJPEG(t, 30, "pair-upper-"+fixture.Kind))
				h.upload(pair, "pair-"+fixture.Kind+".jpg", blackboxJPEG(t, 40, "pair-lower-"+fixture.Kind))
				h.waitProcessingSettled()
				pairBefore := h.entries(pair.ID)
				pairDestination := filepath.Join(volume.Path, "pair-"+fixture.Kind)
				moveAcrossVolumes(t, pair.Path, pairDestination)
				refused := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": pair.ID}), pairDestination)
				if refused.Status != "failed" || !strings.HasSuffix(problemType(refused), "storage/name-collision") {
					t.Fatalf("colliding reconnect onto %s = %+v problem=%+v", fixture.Kind, refused, refused.Problem)
				}
				if got := h.repositoryPath(pair.ID); got != pair.Path {
					t.Fatalf("refused reconnect moved the row to %q", got)
				}
				if rule := h.repositoryRule(pair.ID); !rule.Equal(sourceRule) {
					t.Fatalf("refused reconnect changed the rule to %s", rule)
				}
				pairAfter := h.entries(pair.ID)
				if len(pairAfter) != len(pairBefore) {
					t.Fatalf("refused reconnect changed the index: %d rows, want %d", len(pairAfter), len(pairBefore))
				}
				for path, entry := range pairBefore {
					if current := pairAfter[path]; current.PathKey != entry.PathKey || current.Revision != entry.Revision {
						t.Fatalf("refused reconnect rewrote %s: %+v", path, current)
					}
				}
			}
		})
	}
}

// externalDiskAt grants an external Storage Location at an exact path, for
// a directory on a mounted filesystem image.
func (h *reconnectHarness) externalDiskAt(name, path string) StorageLocationInfo {
	h.t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		h.t.Fatal(err)
	}
	info, _, err := h.control.AddStorageLocation(context.Background(), "acceptance-"+name+"-"+uuid.NewString()[:8], path, name)
	if err != nil {
		h.t.Fatalf("add external Storage Location %s at %s: %v", name, path, err)
	}
	return info
}

func (h *reconnectHarness) repositoryRule(id string) pathsemantics.Semantics {
	h.t.Helper()
	repository, err := h.manager().GetRepository(id)
	if err != nil {
		h.t.Fatal(err)
	}
	return storage.RepositoryVolumeRule(*repository)
}

// locationRule measures the host filesystem a Storage Location lives on.
func (h *reconnectHarness) locationRule(location StorageLocationInfo) pathsemantics.Semantics {
	h.t.Helper()
	rule, err := pathsemantics.Probe(location.Path)
	if err != nil {
		h.t.Fatal(err)
	}
	return rule
}

type indexedEntry struct {
	EntryID  uuid.UUID
	PathKey  string
	State    string
	Revision int64
}

// entries reads the whole scan index of a repository by path, straight from
// the catalog file the running Server owns (a second read-only connection).
func (h *reconnectHarness) entries(id string) map[string]indexedEntry {
	h.t.Helper()
	catalogPath := filepath.Join(h.root, "state", "library.sqlite3")
	if _, err := os.Stat(catalogPath); err != nil {
		h.t.Fatalf("catalog file under %s: %v", h.root, err)
	}
	catalog, err := sql.Open("sqlite3", "file:"+catalogPath+"?mode=ro")
	if err != nil {
		h.t.Fatal(err)
	}
	defer catalog.Close()
	rows, err := catalog.QueryContext(context.Background(),
		`SELECT entry_id, path, path_key, state, revision FROM repository_entries WHERE repository_id = ? ORDER BY entry_id`, id)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	result := map[string]indexedEntry{}
	for rows.Next() {
		var entryID, path string
		var entry indexedEntry
		if err := rows.Scan(&entryID, &path, &entry.PathKey, &entry.State, &entry.Revision); err != nil {
			h.t.Fatal(err)
		}
		entry.EntryID = uuid.MustParse(entryID)
		result[path] = entry
	}
	if err := rows.Err(); err != nil {
		h.t.Fatal(err)
	}
	return result
}

// assertIndexKeyedUnder proves every row of the index carries the key the
// rule derives from its path, and that nothing is missing.
func (h *reconnectHarness) assertIndexKeyedUnder(id string, rule pathsemantics.Semantics) {
	h.t.Helper()
	entries := h.entries(id)
	if len(entries) == 0 {
		h.t.Fatalf("repository %s has no indexed entries", id)
	}
	for path, entry := range entries {
		want, err := rule.PathKey(path)
		if err != nil {
			h.t.Fatal(err)
		}
		if entry.PathKey != want {
			h.t.Fatalf("%s is keyed %q, want %q under %s", path, entry.PathKey, want, rule)
		}
		if entry.State == "missing" {
			h.t.Fatalf("%s is missing after the reconnect; the index was not rekeyed for the volume", path)
		}
	}
}

// moveAcrossVolumes moves a tree the way a user copies a folder to another
// disk and deletes the original: os.Rename cannot cross filesystems, so the
// bytes are copied (without the Server's lock files) and the source removed,
// which leaves the original positively absent.
func moveAcrossVolumes(t testing.TB, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	copyTree(t, from, to)
	if err := os.RemoveAll(from); err != nil {
		t.Fatal(fmt.Errorf("remove the source tree after the copy: %w", err))
	}
}
