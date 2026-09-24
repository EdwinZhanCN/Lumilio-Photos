package handler

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/utils/imagesource"
	"server/internal/utils/imaging"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetAssetSidecar retrieves the Lumilio edit sidecar for an asset.
// @Summary Get asset edit sidecar
// @Description Retrieve the non-destructive Studio edit sidecar stored under the asset repository .lumilio directory.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {object} dto.AssetSidecarResponseDTO "Asset sidecar"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/sidecar [get]
func (h *AssetHandler) GetAssetSidecar(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAsset(c, id, "Authentication required to access this asset", "You don't have permission to access this asset")
	if !ok {
		return
	}

	opened, err := h.locationResolver.OpenAsset(c.Request.Context(), id)
	if err != nil {
		respondRepositoryResolveError(c, err, "Failed to resolve asset location")
		return
	}
	repositoryID := opened.Catalog.RepoID.String()
	projectedPath := opened.Path.String()
	_ = opened.Close()
	source, err := h.sidecarSourceForAsset(c.Request.Context(), asset, projectedPath)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	sidecar := h.defaultSidecarForAsset(id, source)
	exists := false

	content, err := h.repoManager.ReadRepositorySidecar(c.Request.Context(), repositoryID, id.String())
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if content != nil {
		if err := json.Unmarshal(content, &sidecar); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		exists = true
	}

	if sidecar.Version == 0 {
		sidecar.Version = 1
	}
	if sidecar.AssetID == "" {
		sidecar.AssetID = id.String()
	}

	api.JSONOK(c, dto.AssetSidecarResponseDTO{
		AssetID: id.String(),
		Exists:  exists,
		Sidecar: sidecar,
	})
}

// UpdateAssetSidecar stores the Lumilio edit sidecar for an asset.
// @Summary Update asset edit sidecar
// @Description Store non-destructive Studio edit data under the asset repository .lumilio directory.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param data body dto.LumilioSidecarV1DTO true "Sidecar payload"
// @Success 200 {object} dto.AssetSidecarResponseDTO "Asset sidecar saved"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID or request body"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/sidecar [put]
func (h *AssetHandler) UpdateAssetSidecar(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAsset(c, id, "Authentication required to update this asset", "You don't have permission to update this asset")
	if !ok {
		return
	}

	var sidecar dto.LumilioSidecarV1DTO
	if err := c.ShouldBindJSON(&sidecar); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	sidecar.Version = 1
	sidecar.AssetID = id.String()
	opened, err := h.locationResolver.OpenAsset(c.Request.Context(), id)
	if err != nil {
		respondRepositoryResolveError(c, err, "Failed to resolve asset location")
		return
	}
	repositoryID := opened.Catalog.RepoID.String()
	projectedPath := opened.Path.String()
	_ = opened.Close()
	sidecar.Source, err = h.sidecarSourceForAsset(c.Request.Context(), asset, projectedPath)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	sidecar.UpdatedAt = time.Now().UTC()

	content, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	if err := h.repoManager.WriteRepositorySidecar(c.Request.Context(), repositoryID, id.String(), content); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.AssetSidecarResponseDTO{
		AssetID: id.String(),
		Exists:  true,
		Sidecar: sidecar,
	})
}

// GetAssetThumbnail retrieves a thumbnail for a specific asset by asset ID and size
// @Summary Get asset thumbnail
// @Description Retrieve a specific thumbnail image for an asset by asset ID and size parameter. Returns the image file directly.
// @Tags assets
// @Produce image/jpeg
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param size query string false "Thumbnail size" default(medium) Enums(small,medium,large,waveform)
// @Success 200 {file} string "Thumbnail image file"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID or size parameter"
// @Failure 404 {object} api.ProblemResponse "Asset or thumbnail not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/thumbnail [get]
func (h *AssetHandler) GetAssetThumbnail(c *gin.Context) {
	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	assetID, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Get size parameter from query (default to "medium")
	size := c.DefaultQuery("size", "medium")

	// Validate size parameter
	if size != "small" && size != "medium" && size != "large" && size != "waveform" {
		api.WriteProblem(c, api.BadRequest(errors.New("invalid size parameter")))
		return
	}

	_, ok := h.getAuthorizedAssetForMedia(c, assetID, "Authentication required to access this thumbnail", "You don't have permission to access this thumbnail")
	if !ok {
		return
	}

	// Get thumbnail from service
	thumbnail, err := h.assetService.GetThumbnailByAssetIDAndSize(c.Request.Context(), assetID, size)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		log.Printf("Failed to retrieve thumbnail metadata: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	repository, err := h.queries.GetRepository(c.Request.Context(), thumbnail.RepositoryID)
	if err != nil {
		log.Printf("Failed to resolve repository for thumbnail request: %v", err)
		respondRepositoryResolveError(c, err, "Failed to resolve repository")
		return
	}
	repositoryFS, file, err := openRepositoryPrivate(h.files, repository, thumbnail.StoragePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		log.Printf("Failed to open thumbnail: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		_ = repositoryFS.Close()
		api.WriteProblem(c, api.Internal(err))
		return
	}

	// Content-based ETag for cache consistency
	etag := fmt.Sprintf(`"%s-%s-%d"`,
		thumbnail.AssetID.String()[:8], // Short asset ID for uniqueness
		thumbnail.Size,
		fileInfo.ModTime().Unix())

	// Production-ready cache headers
	c.Header("ETag", etag)
	if thumbnail.MimeType != "" {
		c.Header("Content-Type", thumbnail.MimeType)
	}
	c.Header("Cache-Control", "public, max-age=86400, must-revalidate") // 24h cache with validation
	c.Header("Vary", "Accept-Encoding")

	// Check conditional request
	if match := c.GetHeader("If-None-Match"); match == etag {
		_ = file.Close()
		_ = repositoryFS.Close()
		log.Printf("Request for asset %s thumbnail (%s) - 304 Not Modified (ETag: %s)", assetID.String(), size, etag)
		c.Status(http.StatusNotModified)
		return
	}

	serveRepositoryFile(c, repositoryFS, file, thumbnail.StoragePath)
}

// GetOriginalFile serves the original file content by asset ID
// @Summary Get original file
// @Description Serve the original file content for an asset by asset ID. Returns the file as an octet-stream.
// @Tags assets
// @Produce application/octet-stream
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Success 200 {file} file "Original file content"
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/original [get]
func (h *AssetHandler) GetOriginalFile(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this file", "You don't have permission to access this file")
	if !ok {
		return
	}

	opened, err := h.locationResolver.OpenAsset(ctx, asset.AssetID)
	if err != nil {
		log.Printf("Failed to resolve active location for original file: %v", err)
		respondRepositoryResolveError(c, err, "Failed to access repository")
		return
	}

	// Set appropriate headers
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day
	c.Header("Content-Type", asset.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", asset.OriginalFilename))

	serveRepositoryFile(c, opened.Repository, opened.File, asset.OriginalFilename)
}

// clampedIntQuery parses an integer query parameter, returning def when absent
// or invalid, and clamping the result to [min, max].
func clampedIntQuery(c *gin.Context, key string, def, min, max int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ExportAsset re-encodes an asset's original file to a requested format and size.
// @Summary Export asset
// @Description Re-encode an asset's original file to JPEG, PNG, WebP, or AVIF with optional max dimensions and quality, and stream it back as a download.
// @Tags assets
// @Produce image/jpeg,image/png,image/webp,image/avif
// @Param id path string true "Asset ID"
// @Param format query string true "Output format (jpeg, png, webp, avif)"
// @Param quality query int false "Quality 1-100 for lossy formats"
// @Param max_width query int false "Maximum output width in pixels"
// @Param max_height query int false "Maximum output height in pixels"
// @Param filename query string false "Base download filename (without extension)"
// @Success 200 {file} file "Encoded image"
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Authentication required"
// @Failure 403 {object} api.ProblemResponse "Forbidden"
// @Failure 404 {object} api.ProblemResponse "Asset or original file not found"
// @Failure 422 {object} api.ProblemResponse "Source image could not be encoded"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/export [get]
func (h *AssetHandler) ExportAsset(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if !imaging.IsSupportedExportFormat(format) {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("unsupported export format %q", format)))
		return
	}

	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to export this file", "You don't have permission to export this file")
	if !ok {
		return
	}

	opened, fullPath, err := h.locationResolver.LocalAssetPath(ctx, asset.AssetID)
	if err != nil {
		log.Printf("Failed to resolve active location for export: %v", err)
		respondRepositoryResolveError(c, err, "Failed to access repository")
		return
	}
	defer opened.Close()
	_ = opened.File.Close()
	opened.File = nil

	// OpenPhoto yields a libvips-decodable source for any photo: RAW files are
	// resolved to their embedded preview (full render as fallback), non-RAW files
	// are opened directly. This is what lets the export endpoint handle RAW.
	reader, err := imagesource.OpenPhoto(ctx, fullPath, asset.OriginalFilename)
	if err != nil {
		log.Printf("Failed to open source for export of asset %s: %v", id, err)
		api.WriteProblem(c, api.StatusProblem(http.StatusUnprocessableEntity, err))
		return
	}
	defer reader.Close()

	buf, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("Failed to read source for export of asset %s: %v", id, err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	out, mime, ext, err := imaging.ExportImageBytes(buf, imaging.ExportParams{
		Format:    format,
		Quality:   clampedIntQuery(c, "quality", 0, 1, 100),
		MaxWidth:  clampedIntQuery(c, "max_width", 0, 0, 60000),
		MaxHeight: clampedIntQuery(c, "max_height", 0, 0, 60000),
	})
	if err != nil {
		log.Printf("Failed to export asset %s as %s: %v", id, format, err)
		api.WriteProblem(c, api.StatusProblem(http.StatusUnprocessableEntity, err))
		return
	}

	base := strings.TrimSuffix(asset.OriginalFilename, filepath.Ext(asset.OriginalFilename))
	if q := strings.TrimSpace(c.Query("filename")); q != "" {
		base = q
	}
	if strings.TrimSpace(base) == "" {
		base = "export"
	}

	c.Header("Cache-Control", "private, max-age=0")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", base+"."+ext))
	c.Data(http.StatusOK, mime, out)
}

// DownloadAssets serves multiple original files as a zip archive.
// @Summary Download assets
// @Description Serve original files for the requested asset IDs as a zip archive.
// @Tags assets
// @Produce application/zip
// @Param data body dto.DownloadAssetsRequestDTO true "Asset IDs to download"
// @Success 200 {file} file "Zip archive"
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Authentication required"
// @Failure 403 {object} api.ProblemResponse "Forbidden"
// @Failure 404 {object} api.ProblemResponse "Asset or original file not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/download [post]
func (h *AssetHandler) DownloadAssets(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.DownloadAssetsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	if len(req.AssetIDs) == 0 {
		api.WriteProblem(c, api.BadRequest(errors.New("asset_ids is required")))
		return
	}

	files := make([]assetDownloadFile, 0, len(req.AssetIDs))
	for _, rawAssetID := range req.AssetIDs {
		assetIDText := strings.TrimSpace(rawAssetID)
		assetID, err := uuid.Parse(assetIDText)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}

		asset, ok := h.getAuthorizedAssetForMedia(c, assetID, "Authentication required to access this file", "You don't have permission to access this file")
		if !ok {
			return
		}

		files = append(files, assetDownloadFile{asset: *asset})
	}

	filename := fmt.Sprintf("lumilio-assets-%s.zip", time.Now().Format("20060102-150405"))
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Status(http.StatusOK)

	zipWriter := zip.NewWriter(c.Writer)
	archiveNames := make(map[string]int, len(files))
	for _, file := range files {
		if err := writeAssetToZip(ctx, h.locationResolver, zipWriter, archiveNames, file); err != nil {
			log.Printf("Failed to write asset to zip: %v", err)
			_ = zipWriter.Close()
			return
		}
	}

	if err := zipWriter.Close(); err != nil {
		log.Printf("Failed to finalize asset download zip: %v", err)
	}
}

// GetWebVideo serves the web-optimized video version by asset ID
// @Summary Get web-optimized video
// @Description Serve the web-optimized MP4 video version for an asset by asset ID.
// @Tags assets
// @Produce video/mp4
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param variant query string false "Pinned representation; omitted, the request redirects to the currently available one" Enums(web, original)
// @Success 200 {file} file "Web-optimized video file"
// @Success 307
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found or not a video"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/video/web [get]
func (h *AssetHandler) GetWebVideo(c *gin.Context) {
	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Get asset metadata from service
	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this video", "You don't have permission to access this video")
	if !ok {
		return
	}

	// Check if asset is a video
	if asset.Type != "VIDEO" {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("asset is not a video")))
		return
	}

	servePinnedWebMedia(c, h.locationResolver, asset, "_web.mp4", "public, max-age=86400", func(webMediaVariant) string {
		return "video/mp4"
	})
}

// GetWebAudio serves the web-optimized audio version by asset ID
// @Summary Get web-optimized audio
// @Description Serve the web-optimized MP3 audio version for an asset by asset ID.
// @Tags assets
// @Produce audio/mpeg
// @Param id path string true "Asset ID (UUID format)" example("550e8400-e29b-41d4-a716-446655440000")
// @Param variant query string false "Pinned representation; omitted, the request redirects to the currently available one" Enums(web, original)
// @Success 200 {file} file "Web-optimized audio file"
// @Success 307
// @Failure 400 {object} api.ProblemResponse "Invalid asset ID"
// @Failure 404 {object} api.ProblemResponse "Asset not found or not audio"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/assets/{id}/audio/web [get]
func (h *AssetHandler) GetWebAudio(c *gin.Context) {
	// Parse asset ID from URL parameter
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	// Get asset metadata from service
	asset, ok := h.getAuthorizedAssetForMedia(c, id, "Authentication required to access this audio", "You don't have permission to access this audio")
	if !ok {
		return
	}

	// Check if asset is audio
	if asset.Type != "AUDIO" {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("asset is not audio")))
		return
	}

	servePinnedWebMedia(c, h.locationResolver, asset, "_web.mp3", "public, max-age=86400", func(variant webMediaVariant) string {
		if variant == webMediaVariantWeb {
			return "audio/mpeg"
		}
		return assetAudioContentType(asset)
	})
}

func (h *AssetHandler) sidecarSourceForAsset(ctx context.Context, asset *repo.Asset, projectedPath string) (dto.LumilioSidecarSourceDTO, error) {
	source := dto.LumilioSidecarSourceDTO{}
	if asset == nil {
		return source, fmt.Errorf("asset is nil")
	}
	content, err := h.queries.GetContentObjectByID(ctx, asset.ContentID)
	if err != nil {
		return source, err
	}
	source.OriginalFilename = asset.OriginalFilename
	source.MimeType = asset.MimeType
	source.FileSize = content.FileSize
	source.Hash = stringPtr(content.FullHash)
	if asset.Width != nil {
		width := int32(*asset.Width)
		source.Width = &width
	}
	if asset.Height != nil {
		height := int32(*asset.Height)
		source.Height = &height
	}
	source.StoragePath = projectedPath
	return source, nil
}

func (h *AssetHandler) defaultSidecarForAsset(assetID uuid.UUID, source dto.LumilioSidecarSourceDTO) dto.LumilioSidecarV1DTO {
	return dto.LumilioSidecarV1DTO{
		Version:     1,
		AssetID:     assetID.String(),
		Source:      source,
		Adjustments: dto.StudioEditAdjustmentsDTO{},
		UpdatedAt:   time.Now().UTC(),
	}
}
