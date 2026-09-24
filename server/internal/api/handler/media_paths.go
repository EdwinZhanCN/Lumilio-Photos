package handler

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
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

// webMediaVariantQuery pins which representation a web-media URL serves.
//
// A web-media endpoint answers with the transcoded derivative once it is
// published and with the original before that. A media element fetches its
// source in byte ranges over its whole lifetime and never revalidates the
// entity between them, so a URL that changes representation mid-playback hands
// the decoder bytes (or a 416) from a different file: a track that started as
// its original FLAC/AAC hangs on its next range request once the MP3 lands. An
// unpinned request therefore redirects to a URL naming the representation it
// would serve now, and a pinned URL only ever serves that representation.
const webMediaVariantQuery = "variant"

type webMediaVariant string

const (
	webMediaVariantWeb      webMediaVariant = "web"
	webMediaVariantOriginal webMediaVariant = "original"
)

// openWebMediaVariant opens the requested representation. An empty variant
// prefers the published derivative and falls back to the original; the
// returned variant names the one opened. A pinned web variant whose derivative
// does not exist reports fs.ErrNotExist.
func openWebMediaVariant(ctx context.Context, resolver assetLocationResolver, asset *repo.Asset, suffix string, variant webMediaVariant) (*storage.RepositoryFS, *os.File, webMediaVariant, error) {
	if resolver == nil || asset == nil {
		return nil, nil, "", locations.ErrAssetUnavailable
	}
	opened, err := resolver.OpenAsset(ctx, asset.AssetID)
	if err != nil {
		return nil, nil, "", err
	}
	if variant != webMediaVariantOriginal && asset.ContentID != uuid.Nil {
		privatePath, parseErr := (artifact.Identity{SourceFence: asset.ContentID.String(), Stage: "transcode", PipelineVersion: pipeline.AssetPipelineVersion, Name: strings.TrimPrefix(suffix, "_")}).Path()
		if parseErr != nil {
			_ = opened.Close()
			return nil, nil, "", parseErr
		}
		file, openErr := opened.Repository.OpenPrivate(privatePath)
		if openErr == nil {
			_ = opened.File.Close()
			opened.File = nil
			repositoryFS := opened.Repository
			opened.Repository = nil
			return repositoryFS, file, webMediaVariantWeb, nil
		}
		if !errors.Is(openErr, fs.ErrNotExist) {
			_ = opened.Close()
			return nil, nil, "", openErr
		}
	}
	if variant == webMediaVariantWeb {
		_ = opened.Close()
		return nil, nil, "", fs.ErrNotExist
	}
	repositoryFS := opened.Repository
	file := opened.File
	opened.Repository = nil
	opened.File = nil
	return repositoryFS, file, webMediaVariantOriginal, nil
}

// servePinnedWebMedia serves an asset's web representation through a
// representation-pinned URL (see webMediaVariantQuery). contentType picks the
// Content-Type for the variant being served.
func servePinnedWebMedia(c *gin.Context, resolver assetLocationResolver, asset *repo.Asset, suffix, cacheControl string, contentType func(webMediaVariant) string) {
	requested := webMediaVariant(c.Query(webMediaVariantQuery))
	switch requested {
	case "", webMediaVariantWeb, webMediaVariantOriginal:
	default:
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("unknown %s %q", webMediaVariantQuery, requested)))
		return
	}
	repositoryFS, file, opened, err := openWebMediaVariant(c.Request.Context(), resolver, asset, suffix, requested)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
		} else {
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}
	if requested == "" {
		_ = file.Close()
		_ = repositoryFS.Close()
		// Path-relative (http.Redirect would root it) so an API mounted under
		// a proxy prefix still resolves; the rest of the query, such as the
		// media token, is preserved.
		query := c.Request.URL.Query()
		query.Set(webMediaVariantQuery, string(opened))
		c.Header("Cache-Control", "no-store")
		c.Header("Location", path.Base(c.Request.URL.Path)+"?"+query.Encode())
		c.AbortWithStatus(http.StatusTemporaryRedirect)
		return
	}
	c.Header("Cache-Control", cacheControl)
	c.Header("Content-Type", contentType(opened))
	c.Header("Accept-Ranges", "bytes")
	serveRepositoryFile(c, repositoryFS, file, asset.OriginalFilename)
}

func assetAudioContentType(asset *repo.Asset) string {
	if asset != nil && strings.TrimSpace(asset.MimeType) != "" {
		return asset.MimeType
	}
	if asset != nil {
		if contentType := mime.TypeByExtension(filepath.Ext(asset.OriginalFilename)); contentType != "" {
			return contentType
		}
	}
	return "application/octet-stream"
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
