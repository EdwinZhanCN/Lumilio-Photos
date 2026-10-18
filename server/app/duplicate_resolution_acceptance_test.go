package app

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/api/dto"
)

// originalPath finds the one original named filename in a repository,
// outside the .lumilio private directory.
func originalPath(tb testing.TB, repository blackboxRepository, filename string) string {
	tb.Helper()
	found := ""
	err := filepath.WalkDir(repository.Path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".lumilio" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && entry.Name() == filename {
			if found != "" {
				tb.Fatalf("%s exists twice: %s and %s", filename, found, path)
			}
			found = path
		}
		return nil
	})
	if err != nil {
		tb.Fatal(err)
	}
	return found
}

// #223: "Duplicate resolution moves the non-kept files into the Trash, and
// they can be restored." Two uploads with the same pixels and different bytes
// are two Assets with one perceptual hash; merging the group keeps one and
// trashes the other, the same way a Delete does, so Restore brings it back.
func TestLifecycleDuplicateResolutionTrashesTheNonKeptFileAndRestoreReturnsIt(t *testing.T) {
	server := startBlackboxServer(t)
	const keeperName, duplicateName = "duplicate-keeper.jpg", "duplicate-other.jpg"
	server.upload(server.primary, keeperName, blackboxJPEG(t, 77, "duplicate-a"))
	server.upload(server.primary, duplicateName, blackboxJPEG(t, 77, "duplicate-b"))
	var keeper, duplicate []blackboxAsset
	if !waitFor(time.Minute, func() bool {
		keeper, duplicate = server.listByFilename(keeperName), server.listByFilename(duplicateName)
		return len(keeper) == 1 && len(duplicate) == 1
	}) {
		t.Fatalf("uploads never appeared in browse: %+v %+v", keeper, duplicate)
	}
	if keeper[0].AssetID == duplicate[0].AssetID {
		t.Fatal("two files with different bytes are one Asset")
	}
	// The perceptual hash is computed by the enrichment stage.
	server.waitProcessingSettled()
	album := server.createAlbum("Duplicates")
	server.addToAlbum(album, duplicate[0].AssetID)

	var detected dto.DetectDuplicatesResponseDTO
	server.mustJSON(http.MethodPost, "/api/v1/duplicates/detect", dto.DetectDuplicatesRequestDTO{RepositoryID: server.primary.ID}, &detected)
	if detected.Groups != 1 || detected.AssetsAffected != 2 {
		t.Fatalf("detection = %+v, want one group of two Assets", detected)
	}
	var groups dto.ListDuplicateGroupsResponseDTO
	server.mustJSON(http.MethodGet, "/api/v1/duplicates/groups?repository_id="+server.primary.ID, nil, &groups)
	if len(groups.Groups) != 1 || len(groups.Groups[0].Assets) != 2 {
		t.Fatalf("groups = %+v, want one group with two members", groups)
	}
	group := groups.Groups[0]

	var merged dto.MergeDuplicateGroupResponseDTO
	server.mustJSON(http.MethodPost, "/api/v1/duplicates/groups/"+group.GroupID+"/merge",
		dto.MergeDuplicateGroupRequestDTO{KeeperAssetID: keeper[0].AssetID}, &merged)
	if merged.KeeperAssetID != keeper[0].AssetID || len(merged.MergedDuplicates) != 1 || merged.MergedDuplicates[0] != duplicate[0].AssetID {
		t.Fatalf("merge = %+v, want keeper %s and duplicate %s", merged, keeper[0].AssetID, duplicate[0].AssetID)
	}

	// The non-kept file is in the repository trash, not deleted; the keeper
	// stays where it was. The Trash view lists the duplicate; browse does not.
	if path := originalPath(t, server.primary, duplicateName); path != "" {
		t.Fatalf("non-kept file is still at %s", path)
	}
	if path := originalPath(t, server.primary, keeperName); path == "" {
		t.Fatal("the kept file is gone")
	}
	trashed := trashedFilesOf(server.primary)
	if len(trashed) != 1 || !strings.HasSuffix(trashed[0], duplicateName) {
		t.Fatalf("repository trash holds %v, want only %s", trashed, duplicateName)
	}
	if names := server.listTrashed("", server.primary.ID); len(names) != 1 || names[0] != duplicateName {
		t.Fatalf("Trash lists %v, want [%s]", names, duplicateName)
	}
	if got := server.listByFilename(duplicateName); len(got) != 0 {
		t.Fatalf("browse still lists the merged duplicate: %+v", got)
	}
	if got := server.listByFilename(keeperName); len(got) != 1 || got[0].AssetID != keeper[0].AssetID {
		t.Fatalf("browse after the merge lists %+v for the keeper", got)
	}
	// The merge unions the duplicate's album onto the keeper.
	if assets := server.albumAssets(album); len(assets) != 1 || assets[0].AssetID != keeper[0].AssetID {
		t.Fatalf("album after the merge = %+v, want the keeper", assets)
	}

	// Restore returns the duplicate to its original path and to browse.
	var restored dto.AssetLifecycleResultDTO
	server.mustJSON(http.MethodPost, "/api/v1/assets/restore", dto.AssetSelectionDTO{AssetIDs: []string{duplicate[0].AssetID}}, &restored)
	if restored.Files != 1 || len(restored.Renamed) != 0 {
		t.Fatalf("restore = %+v, want one file at its original path", restored)
	}
	if path := originalPath(t, server.primary, duplicateName); path == "" {
		t.Fatal("restored duplicate is not back in the repository")
	}
	if trashed := trashedFilesOf(server.primary); len(trashed) != 0 {
		t.Fatalf("repository trash still holds %v after the restore", trashed)
	}
	if names := server.listTrashed("", server.primary.ID); len(names) != 0 {
		t.Fatalf("Trash after the restore lists %v", names)
	}
	if got := server.listByFilename(duplicateName); len(got) != 1 || got[0].AssetID != duplicate[0].AssetID {
		t.Fatalf("browse after the restore lists %+v for the duplicate, want the same Asset", got)
	}
}
