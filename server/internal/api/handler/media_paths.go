package handler

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"server/internal/api"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
)

// assetDownloadFile pairs a resolved asset with its on-disk original path,
// used when streaming multiple assets into a zip archive. Shared by
// AssetHandler's authenticated bulk download and ShareLinkHandler's public
// share download.
type assetDownloadFile struct {
	asset repo.Asset
	path  string
}

// getRepositoryForAsset resolves the repository row an asset belongs to.
// Shared by AssetHandler (authenticated media) and ShareLinkHandler (public
// share media) so the two never drift on how a storage path is resolved.
func getRepositoryForAsset(ctx context.Context, queries *repo.Queries, asset *repo.Asset) (*repo.Repository, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if !asset.RepositoryID.Valid {
		return nil, fmt.Errorf("asset repository id is invalid")
	}

	repository, err := queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository by id: %w", err)
	}
	// An unreachable repository must not degrade into a bare I/O error further
	// down. The UI has to be able to tell "the drive is unplugged" from "the
	// photo is gone", and only this layer still knows which one it is.
	if repository.Status == dbtypes.RepoStatusOffline || repository.Status == dbtypes.RepoStatusError {
		return nil, fmt.Errorf("%w: %s", storage.ErrRepositoryOffline, repository.Name)
	}
	return &repository, nil
}

// respondRepositoryResolveError maps a getRepositoryForAsset failure onto its
// HTTP response.
//
// An offline repository must not surface as a 500. "The drive is unplugged" is
// a recoverable condition the user can act on, while a 500 reads as a server
// fault and tells the UI nothing it can show. 409 is the same status the ingest
// path already returns for an offline repository, so a client has one code to
// recognize regardless of which endpoint it hit.
func respondRepositoryResolveError(c *gin.Context, err error, message string) {
	if errors.Is(err, storage.ErrRepositoryOffline) {
		api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Repository is unavailable")
		return
	}
	api.GinInternalError(c, err, message)
}

// resolveRepositoryPath joins a repository root with an asset's stored path,
// respecting already-absolute storage paths unchanged.
func resolveRepositoryPath(repositoryPath string, storagePath string) string {
	trimmed := strings.TrimSpace(storagePath)
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(repositoryPath, trimmed)
}

// writeAssetToZip streams one asset's original file into an open zip writer,
// deduping archive entry names via uniqueZipArchiveName.
func writeAssetToZip(zipWriter *zip.Writer, archiveNames map[string]int, file assetDownloadFile) error {
	source, err := os.Open(file.path)
	if err != nil {
		return err
	}
	defer source.Close()

	archiveName := uniqueZipArchiveName(archiveNames, file.asset.OriginalFilename)
	entry, err := zipWriter.Create(archiveName)
	if err != nil {
		return err
	}

	_, err = io.Copy(entry, source)
	return err
}

// uniqueZipArchiveName returns a filesystem-safe, collision-free archive
// entry name for filename, tracking names already used in seen.
func uniqueZipArchiveName(seen map[string]int, filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "." || name == ".." || name == string(filepath.Separator) || name == "" {
		name = "asset"
	}

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem = "asset"
	}

	candidate := name
	for index := 2; seen[candidate] > 0; index++ {
		candidate = fmt.Sprintf("%s (%d)%s", stem, index, ext)
	}
	seen[candidate] = 1
	return candidate
}
