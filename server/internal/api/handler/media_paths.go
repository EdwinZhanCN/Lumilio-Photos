package handler

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"server/internal/api"
	"server/internal/artifact"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/storage"
	"server/internal/storage/roe/locations"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// assetDownloadFile pairs a resolved asset with its on-disk original path,
// used when streaming multiple assets into a zip archive. Shared by
// AssetHandler's authenticated bulk download and ShareLinkHandler's public
// share download.
type assetDownloadFile struct {
	asset repo.Asset
}

type assetLocationResolver interface {
	OpenAsset(context.Context, uuid.UUID) (*locations.OpenedMedia, error)
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
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
		return
	}
	api.WriteProblem(c, api.Internal(err))
}

func openRepositoryMedia(factory *storage.RepositoryFSFactory, repository repo.Repository, rawPath string) (*storage.RepositoryFS, *os.File, error) {
	repositoryPath, err := storage.ParseUserMediaPath(strings.TrimSpace(rawPath))
	if err != nil {
		return nil, nil, err
	}
	repositoryFS, err := factory.Open(repository)
	if err != nil {
		return nil, nil, err
	}
	file, err := repositoryFS.OpenMedia(repositoryPath)
	if err != nil {
		_ = repositoryFS.Close()
		return nil, nil, err
	}
	return repositoryFS, file, nil
}

func openRepositoryPrivate(factory *storage.RepositoryFSFactory, repository repo.Repository, rawPath string) (*storage.RepositoryFS, *os.File, error) {
	repositoryPath, err := storage.ParsePrivateRepositoryPath(strings.TrimSpace(rawPath))
	if err != nil {
		return nil, nil, err
	}
	repositoryFS, err := factory.Open(repository)
	if err != nil {
		return nil, nil, err
	}
	file, err := repositoryFS.OpenPrivate(repositoryPath)
	if err != nil {
		_ = repositoryFS.Close()
		return nil, nil, err
	}
	return repositoryFS, file, nil
}

func serveRepositoryFile(c *gin.Context, repositoryFS *storage.RepositoryFS, file *os.File, filename string) {
	defer repositoryFS.Close()
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	http.ServeContent(c.Writer, c.Request, filename, info.ModTime(), file)
}

func openWebOrOriginal(ctx context.Context, resolver assetLocationResolver, asset *repo.Asset, _ string, suffix string) (*storage.RepositoryFS, *os.File, error) {
	if resolver == nil || asset == nil {
		return nil, nil, locations.ErrAssetUnavailable
	}
	opened, err := resolver.OpenAsset(ctx, asset.AssetID)
	if err != nil {
		return nil, nil, err
	}
	if asset.ContentID != uuid.Nil {
		privatePath, parseErr := (artifact.Identity{SourceFence: asset.ContentID.String(), Stage: "transcode", PipelineVersion: pipeline.AssetPipelineVersion, Name: strings.TrimPrefix(suffix, "_")}).Path()
		if parseErr != nil {
			_ = opened.Close()
			return nil, nil, parseErr
		}
		file, openErr := opened.Repository.OpenPrivate(privatePath)
		if openErr == nil {
			_ = opened.File.Close()
			opened.File = nil
			repositoryFS := opened.Repository
			opened.Repository = nil
			return repositoryFS, file, nil
		}
		if !errors.Is(openErr, fs.ErrNotExist) {
			_ = opened.Close()
			return nil, nil, openErr
		}
	}
	repositoryFS := opened.Repository
	file := opened.File
	opened.Repository = nil
	opened.File = nil
	return repositoryFS, file, nil
}

// writeAssetToZip streams one asset's original file into an open zip writer,
// deduping archive entry names via uniqueZipArchiveName.
func writeAssetToZip(ctx context.Context, resolver assetLocationResolver, zipWriter *zip.Writer, archiveNames map[string]int, file assetDownloadFile) error {
	if resolver == nil {
		return locations.ErrAssetUnavailable
	}
	opened, err := resolver.OpenAsset(ctx, file.asset.AssetID)
	if err != nil {
		return err
	}
	defer opened.Close()

	archiveName := uniqueZipArchiveName(archiveNames, file.asset.OriginalFilename)
	entry, err := zipWriter.Create(archiveName)
	if err != nil {
		return err
	}

	_, err = io.Copy(entry, opened.File)
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
