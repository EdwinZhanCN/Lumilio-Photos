package testfixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zeebo/blake3"
	"gopkg.in/yaml.v3"

	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

// Marker selects an on-disk marker state. Unsupported is a complete marker
// with version 99.0; Corrupt is malformed YAML; Missing writes no marker.
type Marker int

const (
	Valid Marker = iota
	Corrupt
	Unsupported
	Missing
)

// Layout selects the private structure; MissingPrivate retains inbox and
// originals but omits .lumilio entirely.
type Layout int

const (
	Full Layout = iota
	MissingPrivate
)

const (
	OriginalPath = "inbox/original.png"
	TrashID      = "22222222-2222-4222-8222-222222222222"
	AssetID      = "33333333-3333-4333-8333-333333333333"
	TrashAssetID = "44444444-4444-4444-8444-444444444444"
	TrashPath    = ".lumilio/trash/files/" + TrashID + "/deleted.png"
	TrashInfo    = ".lumilio/trash/info/" + TrashID + ".json"
	StudioPath   = ".lumilio/sidecars/" + AssetID + ".lumilio-sidecar"
)

// Timestamp is the stable timestamp used in markers, sidecars and file mtimes.
// Return a value rather than exposing mutable global state.
func Timestamp() time.Time { return time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC) }

// Location contains its on-disk path and the expected identity. Config stays
// valid even when the selected marker state is invalid or missing.
type Location struct {
	Path   string
	Config rootcfg.RootConfig
}

// Repository contains its path and expected portable configuration.
type Repository struct {
	Path   string
	Config repocfg.RepositoryConfig
}

// NewLocation creates an isolated Location in t.TempDir().
func NewLocation(t testing.TB, marker Marker) Location {
	t.Helper()
	location := Location{
		Path: filepath.Join(t.TempDir(), "location"),
		Config: rootcfg.RootConfig{
			Version: rootcfg.CurrentVersion, ID: "11111111-1111-4111-8111-111111111111",
			Name: "Fixture Location", CreatedAt: Timestamp(),
		},
	}
	mkdir(t, location.Path, 0o755)
	writeMarker(t, location.Path, rootcfg.FileName, marker, func() error {
		return location.Config.Save(location.Path)
	}, func() ([]byte, error) {
		config := location.Config
		config.Version = "99.0"
		return yaml.Marshal(config)
	})
	stamp(t, location.Path)
	return location
}

// Repository creates a named direct child. Repeated names in independent
// Locations have the same UUID; different names have different UUIDs. A caller
// must select a fresh child, so fixture construction cannot overwrite data.
func (location Location) Repository(t testing.TB, name string, marker Marker, layout Layout) Repository {
	t.Helper()
	if !filepath.IsLocal(name) || filepath.Base(name) != name || name == "." || name == rootcfg.FileName {
		t.Fatalf("fixture Repository name must be one local child segment: %q", name)
	}
	if layout != Full && layout != MissingPrivate {
		t.Fatalf("unknown fixture layout %d", layout)
	}
	repository := Repository{
		Path: filepath.Join(location.Path, name),
		Config: repocfg.RepositoryConfig{
			Version: repocfg.CurrentVersion,
			ID:      uuid.NewSHA1(uuid.NameSpaceOID, []byte("lumilio-test-repository/"+name)).String(),
			Name:    name, CreatedAt: Timestamp(), StorageStrategy: "date",
			LocalSettings: repocfg.LocalSettings{HandleDuplicateFilenames: "uuid"},
		},
	}
	if err := os.Mkdir(repository.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMarker(t, repository.Path, ".lumiliorepo", marker, func() error {
		return repository.Config.SaveConfigToFile(repository.Path)
	}, func() ([]byte, error) {
		config := repository.Config
		config.Version = "99.0"
		return yaml.Marshal(config)
	})
	write(t, repository.Path, OriginalPath, media(t, 0x40))
	if layout == Full {
		// Exact directory_manager.go layout. The external self-test checks it
		// against the current production DirectoryManager to detect drift.
		for _, name := range []string{
			".lumilio/assets/faces", ".lumilio/sidecars", ".lumilio/logs",
		} {
			mkdir(t, filepath.Join(repository.Path, name), 0o755)
		}
		for _, name := range []string{".lumilio/staging", ".lumilio/staging/incoming", ".lumilio/staging/failed"} {
			mkdir(t, filepath.Join(repository.Path, name), 0o700)
		}
		write(t, repository.Path, ".lumilio/logs/error.log", nil)
		write(t, repository.Path, ".lumilio/logs/operations.log", nil)
	}
	stamp(t, location.Path)
	return repository
}

// NewLeftoverPrimary models retained media with lost app state: no catalog is
// created. The fixed child is primary, but the portable marker has no role.
func NewLeftoverPrimary(t testing.TB) (Location, Repository) {
	t.Helper()
	location := NewLocation(t, Valid)
	primary := location.Repository(t, "primary", Valid, Full)
	original, deleted := media(t, 0x40), media(t, 0x80)
	write(t, primary.Path, TrashPath, deleted)
	// Exact format-1 Trash wire format. Keep this independent of trash, which
	// imports storage; otherwise storage's own tests would get an import cycle.
	writeJSON(t, primary.Path, TrashInfo, map[string]any{
		"format": 1, "trash_id": TrashID, "repository_id": primary.Config.ID,
		"original_path": "inbox/deleted.png", "asset_id": TrashAssetID,
		"hash_algorithm": "blake3-v1", "content_hash": contentHash(deleted),
		"size": len(deleted), "mtime_ns": Timestamp().UnixNano(),
		"deleted_at": Timestamp(), "actor": "fixture-host-owner",
	})
	// Exact Studio v1 wire format, decoded by the production DTO in the
	// external self-test. It contains a content match for the retained original.
	writeJSON(t, primary.Path, StudioPath, map[string]any{
		"version": 1, "asset_id": AssetID, "updated_at": Timestamp(),
		"source": map[string]any{
			"original_filename": "original.png", "storage_path": OriginalPath,
			"mime_type": "image/png", "file_size": len(original), "hash": contentHash(original),
			"width": 2, "height": 2,
		},
		"adjustments": map[string]any{"exposure": 0.5},
	})
	write(t, primary.Path, ".lumilio/staging/incoming/abandoned.part", []byte("uncommitted incoming bytes\n"))
	write(t, primary.Path, ".lumilio/staging/failed/abandoned.part", []byte("failed staging bytes\n"))
	write(t, primary.Path, ".lumilio/unknown-child/opaque.bin", []byte("unknown private data\x00\xff"))
	stamp(t, location.Path)
	return location, primary
}

// NewUnmarkedPrimary creates a nonempty, unmarked primary directory. Nothing
// in it may be initialized over by a future setup command.
func NewUnmarkedPrimary(t testing.TB) (Location, Repository) {
	t.Helper()
	location := NewLocation(t, Valid)
	return location, location.Repository(t, "primary", Missing, MissingPrivate)
}

// CopyRepository makes an independent on-disk copy with the same UUID and
// private bytes. It deliberately performs no registration or identity minting.
func CopyRepository(t testing.TB, source Repository) Repository {
	t.Helper()
	copy := source
	copy.Path = filepath.Join(t.TempDir(), "copy")
	if err := os.CopyFS(copy.Path, os.DirFS(source.Path)); err != nil {
		t.Fatal(err)
	}
	stamp(t, copy.Path)
	return copy
}

func writeMarker(t testing.TB, directory, name string, marker Marker, save func() error, newer func() ([]byte, error)) {
	t.Helper()
	switch marker {
	case Valid:
		if err := save(); err != nil {
			t.Fatal(err)
		}
	case Corrupt:
		write(t, directory, name, []byte("version: [unterminated\n"))
	case Unsupported:
		data, err := newer()
		if err != nil {
			t.Fatal(err)
		}
		write(t, directory, name, data)
	case Missing:
	default:
		t.Fatalf("unknown fixture marker state %d", marker)
	}
}

func mkdir(t testing.TB, directory string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(directory, mode); err != nil {
		t.Fatal(err)
	}
}

func write(t testing.TB, root, name string, data []byte) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	mkdir(t, filepath.Dir(target), 0o755)
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t testing.TB, root, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, name, append(data, '\n'))
}

func stamp(t testing.TB, root string) {
	t.Helper()
	if err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, Timestamp(), Timestamp())
	}); err != nil {
		t.Fatal(err)
	}
}

func media(t testing.TB, shade uint8) []byte {
	t.Helper()
	frame := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			frame.SetNRGBA(x, y, color.NRGBA{R: shade, G: 0x20, B: 0x10, A: 0xff})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, frame); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func contentHash(data []byte) string { return fmt.Sprintf("%x", blake3.Sum256(data)) }
