package handler

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ShareLinkHandler serves both the authenticated share-link management API
// and the public (token-authorized) share viewer API. Public endpoints never
// require authController middleware; the share token itself is the
// capability, validated via service.ResolvePublic on every request.
type ShareLinkHandler struct {
	service      service.ShareLinkService
	assetService service.AssetService
	queries      *repo.Queries
	files        *storage.RepositoryFSFactory
}

// NewShareLinkHandler constructs the share link handler.
func NewShareLinkHandler(shareService service.ShareLinkService, assetService service.AssetService, queries *repo.Queries, files *storage.RepositoryFSFactory) *ShareLinkHandler {
	return &ShareLinkHandler{service: shareService, assetService: assetService, queries: queries, files: files}
}

// --- Authenticated (owner) endpoints -------------------------------------

// NewShareLink creates a share link from a snapshot, album, person, utility
// query, or pin source.
// @Summary Create a share link
// @Description Resolve a source into an asset snapshot and create a revocable, time-limited public share link. The raw token is returned only in this response.
// @Tags share-links
// @Accept json
// @Produce json
// @Param request body dto.CreateShareLinkRequestDTO true "Share link creation data"
// @Success 200 {object} dto.CreateShareLinkResponseDTO "Share link created successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request or source"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Failed to create share link"
// @Router /api/v1/share-links [post]
// @Security BearerAuth
func (h *ShareLinkHandler) NewShareLink(c *gin.Context) {
	var req dto.CreateShareLinkRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	explicitIDs := make([]uuid.UUID, 0, len(req.AssetIDs))
	for _, raw := range req.AssetIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			api.WriteProblem(c, api.BadRequest(err))
			return
		}
		explicitIDs = append(explicitIDs, id)
	}

	link, rawToken, err := h.service.Create(c.Request.Context(), service.ShareLinkCreateParams{
		OwnerID:          int32(user.UserID),
		OwnerScope:       ownerScopeID(c),
		Title:            req.Title,
		Description:      req.Description,
		SourceKind:       req.SourceKind,
		SourceRef:        req.SourceRef,
		ExplicitAssetIDs: explicitIDs,
		ExpiresInDays:    req.ExpiresInDays,
		AllowDownload:    req.AllowDownload,
		IncludeOriginals: req.IncludeOriginals,
	})
	if err != nil {
		writeShareLinkCreateError(c, err)
		return
	}

	api.JSONOK(c, dto.CreateShareLinkResponseDTO{
		ShareLinkDTO: dto.ToShareLinkDTO(link),
		Token:        rawToken,
	})
}

// ListShareLinks lists the current user's share links.
// @Summary List share links
// @Description List all share links owned by the current user, newest first.
// @Tags share-links
// @Produce json
// @Success 200 {object} dto.ListShareLinksResponseDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Router /api/v1/share-links [get]
// @Security BearerAuth
func (h *ShareLinkHandler) ListShareLinks(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	links, err := h.service.List(c.Request.Context(), int32(user.UserID))
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	items := make([]dto.ShareLinkDTO, 0, len(links))
	for _, l := range links {
		items = append(items, dto.ToShareLinkDTO(l))
	}
	api.JSONOK(c, dto.ListShareLinksResponseDTO{Items: items})
}

// GetShareLink returns one share link's owner-facing metadata.
// @Summary Get a share link
// @Description Get one share link's metadata (never the token).
// @Tags share-links
// @Produce json
// @Param id path string true "Share ID"
// @Success 200 {object} dto.ShareLinkDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Share link not found"
// @Router /api/v1/share-links/{id} [get]
// @Security BearerAuth
func (h *ShareLinkHandler) GetShareLink(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	shareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	link, err := h.service.Get(c.Request.Context(), int32(user.UserID), shareID)
	if err != nil {
		writeShareLinkLookupError(c, err)
		return
	}
	api.JSONOK(c, dto.ToShareLinkDTO(link))
}

// UpdateShareLink patches a share link's settings and/or extends its expiry.
// @Summary Update a share link
// @Description Patch a share link's title/description/download settings, and/or extend its expiry by extend_days.
// @Tags share-links
// @Accept json
// @Produce json
// @Param id path string true "Share ID"
// @Param request body dto.UpdateShareLinkRequestDTO true "Share link update"
// @Success 200 {object} dto.ShareLinkDTO
// @Failure 400 {object} api.ProblemResponse "Invalid request"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Share link not found"
// @Router /api/v1/share-links/{id} [patch]
// @Security BearerAuth
func (h *ShareLinkHandler) UpdateShareLink(c *gin.Context) {
	var req dto.UpdateShareLinkRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	shareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	link, err := h.service.UpdateSettings(c.Request.Context(), int32(user.UserID), shareID, service.ShareLinkUpdateParams{
		Title:            req.Title,
		Description:      req.Description,
		AllowDownload:    req.AllowDownload,
		IncludeOriginals: req.IncludeOriginals,
		ExtendDays:       req.ExtendDays,
	})
	if err != nil {
		writeShareLinkLookupError(c, err)
		return
	}
	api.JSONOK(c, dto.ToShareLinkDTO(link))
}

// RevokeShareLink immediately disables public access for a share link.
// @Summary Revoke a share link
// @Description Immediately disable public access for a share link.
// @Tags share-links
// @Produce json
// @Param id path string true "Share ID"
// @Success 200 {object} dto.ShareLinkDTO
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Share link not found"
// @Router /api/v1/share-links/{id}/revoke [post]
// @Security BearerAuth
func (h *ShareLinkHandler) RevokeShareLink(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	shareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	link, err := h.service.Revoke(c.Request.Context(), int32(user.UserID), shareID)
	if err != nil {
		writeShareLinkLookupError(c, err)
		return
	}
	api.JSONOK(c, dto.ToShareLinkDTO(link))
}

// DeleteShareLink permanently removes a share link record. Only
// expired/revoked links may be deleted.
// @Summary Delete a share link
// @Description Permanently remove a share link record. Only expired or revoked links may be deleted.
// @Tags share-links
// @Produce json
// @Param id path string true "Share ID"
// @Success 200 {object} api.SuccessResponse
// @Failure 400 {object} api.ProblemResponse "Share link must be expired or revoked first"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Share link not found"
// @Router /api/v1/share-links/{id} [delete]
// @Security BearerAuth
func (h *ShareLinkHandler) DeleteShareLink(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	shareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	if err := h.service.Delete(c.Request.Context(), int32(user.UserID), shareID); err != nil {
		switch {
		case errors.Is(err, service.ErrShareLinkNotFound):
			api.WriteProblem(c, api.NotFound(err))
		case errors.Is(err, service.ErrShareLinkNotDeletable):
			api.WriteProblem(c, api.BadRequest(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}
	api.JSONOK(c, api.SuccessResponse{Message: "Share link deleted"})
}

func writeShareLinkCreateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrShareLinkTooLarge),
		errors.Is(err, service.ErrShareLinkSourceEmpty),
		errors.Is(err, service.ErrShareLinkInvalidSource):
		api.WriteProblem(c, api.BadRequest(err))
	default:
		api.WriteProblem(c, api.Internal(err))
	}
}

func writeShareLinkLookupError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrShareLinkNotFound) {
		api.WriteProblem(c, api.NotFound(err))
		return
	}
	api.WriteProblem(c, api.Internal(err))
}

// --- Public (token-authorized) endpoints ---------------------------------

// resolvePublicShare authorizes the :token path param. Every public handler
// must call this first; expired, revoked, and unknown tokens are
// deliberately indistinguishable to avoid token-probing feedback.
func (h *ShareLinkHandler) resolvePublicShare(c *gin.Context) (repo.ShareLink, bool) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		api.WriteProblem(c, api.NotFound(errors.New("missing token")))
		return repo.ShareLink{}, false
	}
	link, err := h.service.ResolvePublic(c.Request.Context(), token)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return repo.ShareLink{}, false
	}
	return link, true
}

// resolvePublicShareAsset authorizes :assetId against the resolved share's
// asset snapshot before any media is ever touched.
func (h *ShareLinkHandler) resolvePublicShareAsset(c *gin.Context, link repo.ShareLink) (*repo.Asset, bool) {
	assetID, err := uuid.Parse(c.Param("assetId"))
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return nil, false
	}
	if !h.service.AssetInShare(link, assetID) {
		api.WriteProblem(c, api.NotFound(errors.New("asset not in share")))
		return nil, false
	}
	asset, err := h.assetService.GetAssetAny(c.Request.Context(), assetID)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return nil, false
	}
	return asset, true
}

// GetPublicShare returns de-sensitized share metadata and records a view.
// @Summary Get public share metadata
// @Description Get a public share's de-sensitized metadata (title, asset count, expiry, download policy). Records one view.
// @Tags public-shares
// @Produce json
// @Param token path string true "Share token"
// @Success 200 {object} dto.PublicShareMetadataDTO
// @Failure 404 {object} api.ProblemResponse "Share not found or no longer available"
// @Router /api/v1/public/shares/{token} [get]
func (h *ShareLinkHandler) GetPublicShare(c *gin.Context) {
	link, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	if err := h.service.RecordView(c.Request.Context(), link.ShareID); err != nil {
		log.Printf("Failed to record share view: %v", err)
	}
	c.Header("Cache-Control", "private, max-age=0, no-store")
	api.JSONOK(c, dto.ToPublicShareMetadataDTO(link))
}

// ListPublicShareAssets returns one page of a public share's asset list.
// @Summary List public share assets
// @Description Browse a public share's assets in date order. v1 is browse-only: no filter/search/sort.
// @Tags public-shares
// @Accept json
// @Produce json
// @Param token path string true "Share token"
// @Param request body dto.PublicShareAssetListRequestDTO true "Pagination"
// @Success 200 {object} dto.PublicShareAssetListResponseDTO
// @Failure 404 {object} api.ProblemResponse "Share not found or no longer available"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/public/shares/{token}/assets/list [post]
func (h *ShareLinkHandler) ListPublicShareAssets(c *gin.Context) {
	link, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}

	var req dto.PublicShareAssetListRequestDTO
	_ = c.ShouldBindJSON(&req)

	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	result, err := h.assetService.QueryBrowseItems(c.Request.Context(), service.QueryAssetsParams{
		Source:    h.service.PublicAssetSource(link),
		StackMode: service.StackModeExpanded,
		SortBy:    "date_captured",
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		log.Printf("Failed to list public share assets: %v", err)
		api.WriteProblem(c, api.Internal(err))
		return
	}

	items := make([]dto.PublicAssetDTO, 0, len(result.Items))
	for _, item := range result.Items {
		if item.MediaItem == nil {
			continue
		}
		items = append(items, dto.ToPublicAssetDTO(item.MediaItem.PrimaryAsset))
	}

	c.Header("Cache-Control", "private, max-age=0, no-store")
	api.JSONOK(c, dto.PublicShareAssetListResponseDTO{
		Items:  items,
		Total:  int(result.TotalVisible),
		Limit:  limit,
		Offset: offset,
	})
}

// GetPublicShareThumbnail serves a share asset's thumbnail.
// @Summary Get public share asset thumbnail
// @Description Serve a thumbnail for an asset that belongs to this share.
// @Tags public-shares
// @Produce image/jpeg
// @Param token path string true "Share token"
// @Param assetId path string true "Asset ID"
// @Param size query string false "Thumbnail size" default(medium) Enums(small,medium,large)
// @Success 200 {file} string "Thumbnail image file"
// @Failure 404 {object} api.ProblemResponse "Not found"
// @Router /api/v1/public/shares/{token}/assets/{assetId}/thumbnail [get]
func (h *ShareLinkHandler) GetPublicShareThumbnail(c *gin.Context) {
	link, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	asset, ok := h.resolvePublicShareAsset(c, link)
	if !ok {
		return
	}

	size := c.DefaultQuery("size", "medium")
	if size != "small" && size != "medium" && size != "large" {
		api.WriteProblem(c, api.BadRequest(errors.New("invalid size parameter")))
		return
	}

	thumbnail, err := h.assetService.GetThumbnailByAssetIDAndSize(c.Request.Context(), asset.AssetID, size)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	repository, err := getRepositoryForAsset(c.Request.Context(), h.queries, asset)
	if err != nil {
		respondRepositoryResolveError(c, err, "Failed to resolve repository")
		return
	}
	repositoryFS, file, err := openRepositoryPrivate(h.files, *repository, thumbnail.StoragePath)
	if err != nil {
		api.WriteProblem(c, api.NotFound(err))
		return
	}

	// Short private caching only: this is a token-gated asset, not a
	// permanently public URL, so it must never be cached at a shared proxy.
	c.Header("Cache-Control", "private, max-age=300")
	serveRepositoryFile(c, repositoryFS, file, thumbnail.StoragePath)
}

// GetPublicShareWebVideo serves a share asset's web-optimized video, falling
// back to the original file when no web version has been generated yet.
// @Summary Get public share asset web video
// @Description Serve the web-optimized video for an asset that belongs to this share.
// @Tags public-shares
// @Produce video/mp4
// @Param token path string true "Share token"
// @Param assetId path string true "Asset ID"
// @Success 200 {file} file "Web-optimized video file"
// @Failure 404 {object} api.ProblemResponse "Not found"
// @Router /api/v1/public/shares/{token}/assets/{assetId}/web-video [get]
func (h *ShareLinkHandler) GetPublicShareWebVideo(c *gin.Context) {
	h.servePublicShareWebMedia(c, "VIDEO", "videos", "_web.mp4", "video/mp4", "Video file not found")
}

// GetPublicShareWebAudio serves a share asset's web-optimized audio, falling
// back to the original file when no web version has been generated yet.
// @Summary Get public share asset web audio
// @Description Serve the web-optimized audio for an asset that belongs to this share.
// @Tags public-shares
// @Produce audio/mpeg
// @Param token path string true "Share token"
// @Param assetId path string true "Asset ID"
// @Success 200 {file} file "Web-optimized audio file"
// @Failure 404 {object} api.ProblemResponse "Not found"
// @Router /api/v1/public/shares/{token}/assets/{assetId}/web-audio [get]
func (h *ShareLinkHandler) GetPublicShareWebAudio(c *gin.Context) {
	h.servePublicShareWebMedia(c, "AUDIO", "audios", "_web.mp3", "audio/mpeg", "Audio file not found")
}

// servePublicShareWebMedia mirrors AssetHandler's GetWebVideo/GetWebAudio web
// version + fallback-to-original logic, scoped to a share's asset set.
func (h *ShareLinkHandler) servePublicShareWebMedia(c *gin.Context, assetType, derivedKind, webSuffix, contentType, notFoundMessage string) {
	link, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	asset, ok := h.resolvePublicShareAsset(c, link)
	if !ok {
		return
	}
	if asset.Type != assetType {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("asset is not %s", strings.ToLower(assetType))))
		return
	}
	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		api.WriteProblem(c, api.NotFound(errors.New("asset storage path is empty")))
		return
	}

	repository, err := getRepositoryForAsset(c.Request.Context(), h.queries, asset)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	repositoryFS, file, err := openWebOrOriginal(h.files, *repository, asset, derivedKind, webSuffix)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
		} else {
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	serveRepositoryFile(c, repositoryFS, file, asset.OriginalFilename)
}

// GetPublicShareOriginal serves a share asset's original file. Requires the
// share to have both allow_download and include_originals enabled.
// @Summary Get public share original file
// @Description Serve the original file for an asset that belongs to this share. Requires allow_download and include_originals to both be enabled.
// @Tags public-shares
// @Produce application/octet-stream
// @Param token path string true "Share token"
// @Param assetId path string true "Asset ID"
// @Success 200 {file} file "Original file content"
// @Failure 403 {object} api.ProblemResponse "Original downloads are not enabled for this share"
// @Failure 404 {object} api.ProblemResponse "Not found"
// @Router /api/v1/public/shares/{token}/assets/{assetId}/original [get]
func (h *ShareLinkHandler) GetPublicShareOriginal(c *gin.Context) {
	link, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	if !link.AllowDownload || !link.IncludeOriginals {
		api.WriteProblem(c, api.Forbidden(errors.New("original downloads are not enabled for this share")))
		return
	}
	asset, ok := h.resolvePublicShareAsset(c, link)
	if !ok {
		return
	}
	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		api.WriteProblem(c, api.NotFound(errors.New("asset storage path is empty")))
		return
	}

	repository, err := getRepositoryForAsset(c.Request.Context(), h.queries, asset)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	repositoryFS, file, err := openRepositoryMedia(h.files, *repository, *asset.StoragePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.WriteProblem(c, api.NotFound(err))
		} else {
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	c.Header("Cache-Control", "private, max-age=0, no-store")
	c.Header("Content-Type", asset.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", asset.OriginalFilename))
	serveRepositoryFile(c, repositoryFS, file, asset.OriginalFilename)
}

// DownloadPublicShare serves the share's assets (or a requested subset) as a
// zip archive. Requires the share to have allow_download enabled.
// @Summary Download public share
// @Description Serve the share's original files (optionally scoped to a subset via asset_ids) as a zip archive. Requires allow_download to be enabled.
// @Tags public-shares
// @Accept json
// @Produce application/zip
// @Param token path string true "Share token"
// @Param request body dto.PublicShareDownloadRequestDTO false "Optional asset ID subset"
// @Success 200 {file} file "Zip archive"
// @Failure 403 {object} api.ProblemResponse "Downloads are not enabled for this share"
// @Failure 404 {object} api.ProblemResponse "Not found"
// @Router /api/v1/public/shares/{token}/download [post]
func (h *ShareLinkHandler) DownloadPublicShare(c *gin.Context) {
	link, ok := h.resolvePublicShare(c)
	if !ok {
		return
	}
	if !link.AllowDownload {
		api.WriteProblem(c, api.Forbidden(errors.New("downloads are not enabled for this share")))
		return
	}

	var req dto.PublicShareDownloadRequestDTO
	_ = c.ShouldBindJSON(&req)

	targetIDs := link.AssetIds
	if len(req.AssetIDs) > 0 {
		filtered := make(dbtypes.UUIDs, 0, len(req.AssetIDs))
		for _, raw := range req.AssetIDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				continue
			}
			if h.service.AssetInShare(link, id) {
				filtered = append(filtered, id)
			}
		}
		targetIDs = filtered
	}

	files := make([]assetDownloadFile, 0, len(targetIDs))
	for _, assetID := range targetIDs {
		asset, err := h.assetService.GetAssetAny(c.Request.Context(), assetID)
		if err != nil {
			continue
		}
		if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
			continue
		}
		repository, err := getRepositoryForAsset(c.Request.Context(), h.queries, asset)
		if err != nil {
			continue
		}
		repositoryPath, parseErr := storage.ParseUserMediaPath(*asset.StoragePath)
		if parseErr != nil {
			continue
		}
		repositoryFS, openErr := h.files.Open(*repository)
		if openErr != nil {
			continue
		}
		opened, openErr := repositoryFS.OpenMedia(repositoryPath)
		if openErr != nil {
			_ = repositoryFS.Close()
			continue
		}
		_ = opened.Close()
		_ = repositoryFS.Close()
		files = append(files, assetDownloadFile{asset: *asset, repository: *repository, path: repositoryPath})
	}

	if len(files) == 0 {
		api.WriteProblem(c, api.NotFound(errors.New("no downloadable files in this share")))
		return
	}

	filename := fmt.Sprintf("lumilio-share-%s.zip", time.Now().Format("20060102-150405"))
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Status(http.StatusOK)

	zipWriter := zip.NewWriter(c.Writer)
	archiveNames := make(map[string]int, len(files))
	for _, file := range files {
		if err := writeAssetToZip(h.files, zipWriter, archiveNames, file); err != nil {
			log.Printf("Failed to write share asset to zip: %v", err)
			_ = zipWriter.Close()
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		log.Printf("Failed to finalize share download zip: %v", err)
	}
}
