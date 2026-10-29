package testfixture_test

import (
	"encoding/json"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zeebo/blake3"

	"server/internal/api/dto"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	"server/internal/storage/testfixture"
	"server/internal/storage/trash"
)

func TestMarkerFixtures(t *testing.T) {
	for _, marker := range []testfixture.Marker{testfixture.Valid, testfixture.Corrupt, testfixture.Unsupported, testfixture.Missing} {
		t.Run(fmt.Sprint(marker), func(t *testing.T) {
			location := testfixture.NewLocation(t, marker)
			repository := location.Repository(t, "regular", marker, testfixture.Full)
			before := tree(t, location.Path)
			root, rootErr := rootcfg.Load(location.Path)
			repo, repoErr := repocfg.LoadConfigFromFile(repository.Path)
			if marker == testfixture.Valid {
				if rootErr != nil || repoErr != nil {
					t.Fatalf("read valid markers: root %v, repository %v", rootErr, repoErr)
				}
				if !reflect.DeepEqual(*root, location.Config) || !reflect.DeepEqual(*repo, repository.Config) {
					t.Fatalf("marker round trip differs: root %#v, repository %#v", root, repo)
				}
			} else if rootErr == nil || repoErr == nil {
				t.Fatalf("invalid/missing markers accepted: root %v, repository %v", rootErr, repoErr)
			}
			if marker == testfixture.Unsupported && (!strings.Contains(rootErr.Error(), "99.0") || !strings.Contains(repoErr.Error(), "99.0")) {
				t.Fatalf("newer versions not diagnosed: root %v, repository %v", rootErr, repoErr)
			}
			if after := tree(t, location.Path); !reflect.DeepEqual(before, after) {
				t.Fatal("marker reads changed fixture bytes or mtimes")
			}
			second := testfixture.NewLocation(t, marker)
			second.Repository(t, "regular", marker, testfixture.Full)
			if !reflect.DeepEqual(before, tree(t, second.Path)) {
				t.Fatal("independent builds differ in names, contents or mtimes")
			}
		})
	}
}

func TestLeftoverPrimaryFixtures(t *testing.T) {
	location, primary := testfixture.NewLeftoverPrimary(t)
	second, _ := testfixture.NewLeftoverPrimary(t)
	if !reflect.DeepEqual(tree(t, location.Path), tree(t, second.Path)) {
		t.Fatal("leftover primary is not deterministic")
	}
	if primary.Path != filepath.Join(location.Path, "primary") {
		t.Fatal("primary is not at the fixed default child")
	}
	info, err := trash.ParseSidecar(read(t, primary.Path, testfixture.TrashInfo))
	if err != nil {
		t.Fatal(err)
	}
	deleted := read(t, primary.Path, testfixture.TrashPath)
	if info.Format != 1 || info.RepositoryID != primary.Config.ID || info.TrashID != testfixture.TrashID ||
		info.AssetID != testfixture.TrashAssetID || info.Size != int64(len(deleted)) ||
		info.ContentHash != hash(deleted) || info.HashAlgorithm != "blake3-v1" ||
		info.OriginalPath != "inbox/deleted.png" || !info.DeletedAt.Equal(testfixture.Timestamp()) {
		t.Fatalf("Trash sidecar does not describe its file: %#v", info)
	}
	var studio dto.LumilioSidecarV1DTO
	if err := json.Unmarshal(read(t, primary.Path, testfixture.StudioPath), &studio); err != nil {
		t.Fatal(err)
	}
	original := read(t, primary.Path, testfixture.OriginalPath)
	if studio.Version != 1 || studio.AssetID != testfixture.AssetID || studio.Source.StoragePath != testfixture.OriginalPath ||
		studio.Source.Hash == nil || *studio.Source.Hash != hash(original) || studio.Source.FileSize != int64(len(original)) ||
		studio.Adjustments.Exposure != 0.5 || !studio.UpdatedAt.Equal(testfixture.Timestamp()) {
		t.Fatalf("Studio sidecar drifted: %#v", studio)
	}
	for _, name := range []string{testfixture.OriginalPath, testfixture.TrashPath} {
		file, err := os.Open(filepath.Join(primary.Path, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		_, decodeErr := png.Decode(file)
		file.Close()
		if decodeErr != nil {
			t.Fatalf("fixture media %s is not readable: %v", name, decodeErr)
		}
	}
	for _, name := range []string{
		".lumilio/staging/incoming/abandoned.part", ".lumilio/staging/failed/abandoned.part", ".lumilio/unknown-child/opaque.bin",
	} {
		if len(read(t, primary.Path, name)) == 0 {
			t.Fatalf("private fixture %s is empty", name)
		}
	}
	copy := testfixture.CopyRepository(t, primary)
	config, err := repocfg.LoadConfigFromFile(copy.Path)
	if err != nil || config.ID != primary.Config.ID || copy.Path == primary.Path ||
		!reflect.DeepEqual(tree(t, primary.Path), tree(t, copy.Path)) {
		t.Fatalf("same-UUID copy differs: %#v, %v", config, err)
	}
	// Current validation performs writable probes, so run it after the
	// determinism checks (those probes change directory mtimes).
	validation, err := storage.NewDirectoryManager().ValidateStructure(primary.Path)
	if err != nil || !validation.Valid || len(validation.MissingDirectories) != 0 {
		t.Fatalf("fixture private layout drifted: %#v, %v", validation, err)
	}
}

func TestMissingLayoutAndUnmarkedPrimaryFixtures(t *testing.T) {
	location := testfixture.NewLocation(t, testfixture.Valid)
	repository := location.Repository(t, "repairable", testfixture.Valid, testfixture.MissingPrivate)
	if _, err := repocfg.LoadConfigFromFile(repository.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repository.Path, ".lumilio")); !os.IsNotExist(err) {
		t.Fatalf("missing-private fixture has private state: %v", err)
	}
	read(t, repository.Path, testfixture.OriginalPath)
	other := location.Repository(t, "other", testfixture.Valid, testfixture.Full)
	if other.Config.ID == repository.Config.ID {
		t.Fatal("different Repository names have identical UUIDs")
	}
	_, unmarked := testfixture.NewUnmarkedPrimary(t)
	if _, err := os.Stat(filepath.Join(unmarked.Path, ".lumiliorepo")); !os.IsNotExist(err) {
		t.Fatalf("unmarked primary has a marker: %v", err)
	}
	if len(read(t, unmarked.Path, testfixture.OriginalPath)) == 0 {
		t.Fatal("unmarked primary is empty")
	}
}

type entry struct {
	Directory bool
	Bytes     string
	Modified  time.Time
}

func tree(t *testing.T, root string) map[string]entry {
	t.Helper()
	entries := make(map[string]entry)
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		value := entry{Directory: item.IsDir(), Modified: info.ModTime().UTC()}
		if !value.Modified.Equal(testfixture.Timestamp()) {
			t.Errorf("%s mtime = %v, want fixed fixture time", name, value.Modified)
		}
		if !item.IsDir() {
			value.Bytes = string(read(t, root, name))
		}
		entries[filepath.ToSlash(name)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func read(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hash(data []byte) string { return fmt.Sprintf("%x", blake3.Sum256(data)) }
