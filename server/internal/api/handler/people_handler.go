package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PeopleHandler struct {
	assetService     service.AssetService
	faceService      service.FaceService
	authService      *service.AuthService
	repoPathResolver peopleRepositoryPathResolver
}

type peopleRepositoryPathResolver interface {
	GetRepositoryPath(repoID string) (string, error)
}

func NewPeopleHandler(
	assetService service.AssetService,
	faceService service.FaceService,
	authService *service.AuthService,
	repoPathResolver peopleRepositoryPathResolver,
) *PeopleHandler {
	return &PeopleHandler{
		assetService:     assetService,
		faceService:      faceService,
		authService:      authService,
		repoPathResolver: repoPathResolver,
	}
}

// ListPeople lists repository-scoped recognized people.
// @Summary List people
// @Description List recognized people for the current repository scope.
// @Tags people
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Param limit query int false "Maximum number of results (max 100)" default(24)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} api.Result{data=dto.ListPeopleResponseDTO} "People listed successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/people [get]
func (h *PeopleHandler) ListPeople(c *gin.Context) {
	limit, offset := parseListPagination(c, 24, 100)
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	people, total, err := h.faceService.ListPeople(c.Request.Context(), repositoryID, scopedOwnerIDFromContext(c), limit, offset)
	if err != nil {
		log.Printf("Failed to list people: %v", err)
		api.GinInternalError(c, err, "Failed to list people")
		return
	}

	items := make([]dto.PersonSummaryDTO, 0, len(people))
	for _, person := range people {
		items = append(items, dto.ToPersonSummaryDTO(person))
	}

	api.GinSuccess(c, dto.ListPeopleResponseDTO{
		People: items,
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
	})
}

// GetPerson gets a single recognized person.
// @Summary Get person
// @Description Get a single recognized person by cluster ID.
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} api.Result{data=dto.PersonDetailDTO} "Person fetched successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 404 {object} api.Result "Person not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/people/{id} [get]
func (h *PeopleHandler) GetPerson(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	person, err := h.faceService.GetPerson(c.Request.Context(), personID, repositoryID, scopedOwnerIDFromContext(c))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		log.Printf("Failed to fetch person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to fetch person")
		return
	}

	api.GinSuccess(c, dto.ToPersonDetailDTO(*person))
}

// UpdatePerson renames a recognized person.
// @Summary Update person
// @Description Rename a recognized person. Successful updates mark the person as confirmed.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Person ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Param request body dto.UpdatePersonRequestDTO true "Person update payload"
// @Success 200 {object} api.Result{data=dto.PersonDetailDTO} "Person updated successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 404 {object} api.Result "Person not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/people/{id} [patch]
// @Security BearerAuth
func (h *PeopleHandler) UpdatePerson(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	var req dto.UpdatePersonRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		api.GinBadRequest(c, errors.New("name cannot be empty"), "Name cannot be empty")
		return
	}

	_, err := h.faceService.GetPerson(c.Request.Context(), personID, repositoryID, scopedOwnerIDFromContext(c))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		log.Printf("Failed to authorize person update %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to load person")
		return
	}

	if _, err := h.faceService.RenamePerson(c.Request.Context(), personID, name); err != nil {
		log.Printf("Failed to rename person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to update person")
		return
	}

	person, err := h.faceService.GetPerson(c.Request.Context(), personID, repositoryID, scopedOwnerIDFromContext(c))
	if err != nil {
		log.Printf("Failed to reload person %d after rename: %v", personID, err)
		api.GinInternalError(c, err, "Failed to reload person")
		return
	}

	api.GinSuccess(c, dto.ToPersonDetailDTO(*person))
}

// ListPersonAssets lists assets belonging to a person while preserving the standard asset query shape.
// @Summary List person assets
// @Description List assets scoped to a specific person while reusing the unified asset query filters.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Person ID"
// @Param request body dto.AssetQueryRequestDTO true "Asset query parameters"
// @Success 200 {object} api.Result{data=dto.QueryAssetsResponseDTO} "Assets listed successfully"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 404 {object} api.Result "Person not found"
// @Failure 503 {object} api.Result "Semantic search unavailable"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/people/{id}/assets/list [post]
func (h *PeopleHandler) ListPersonAssets(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}

	var req dto.AssetQueryRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	normalizeAssetQueryPagination(&req.Pagination)
	if err := validateAssetQuerySearchType(req.SearchType); err != nil {
		api.GinBadRequest(c, err, "Search type must be 'filename' or 'semantic'")
		return
	}
	if err := validateAssetQueryGroupBy(req.GroupBy); err != nil {
		api.GinBadRequest(c, err, "group_by must be 'date', 'type', or 'flat'")
		return
	}
	if req.SearchType == "" {
		req.SearchType = "filename"
	}

	repositoryID, err := parseRepositoryUUIDFromAssetFilter(req.Filter)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid repository_id filter")
		return
	}

	if _, err := h.faceService.GetPerson(c.Request.Context(), personID, repositoryID, scopedOwnerIDFromContext(c)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		log.Printf("Failed to authorize person assets %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to load person")
		return
	}

	params := buildQueryAssetsParams(req.Query, req.SearchType, req.GroupBy, req.ViewerTimezone, req.Filter, req.Pagination)
	params.PersonID = &personID
	params = applyAssetOwnershipScope(c, params)

	assets, total, err := h.assetService.QueryAssets(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.GinError(c, 503, err, 503, "Semantic search is currently unavailable")
			return
		}
		log.Printf("Failed to list assets for person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to query person assets")
		return
	}

	totalInt := int(total)
	api.GinSuccess(c, dto.QueryAssetsResponseDTO{
		Groups: toAssetGroupDTOs(service.GroupAssetsPage(assets, params.GroupBy, params.ViewerTimeZone)),
		Total:  &totalInt,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	})
}

// GetPersonCover serves the representative face crop for a person.
// @Summary Get person cover
// @Description Serve the representative face crop image for a recognized person.
// @Tags people
// @Produce image/webp
// @Param id path int true "Person ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {file} binary "Face crop"
// @Failure 400 {object} api.Result "Invalid request parameters"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 404 {object} api.Result "Person cover not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/people/{id}/cover [get]
func (h *PeopleHandler) GetPersonCover(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	ownerID, ok := h.resolveMediaOwnerScope(c)
	if !ok {
		return
	}

	person, err := h.faceService.GetPerson(c.Request.Context(), personID, repositoryID, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		log.Printf("Failed to fetch person cover %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to load person")
		return
	}
	if person.RepresentativeAssetID == nil || person.CoverFaceImagePath == nil {
		api.GinNotFound(c, errors.New("person cover not found"), "Person cover not found")
		return
	}

	representativeAssetID, err := uuid.Parse(*person.RepresentativeAssetID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve person cover")
		return
	}

	asset, err := h.assetService.GetAsset(c.Request.Context(), representativeAssetID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve person cover asset")
		return
	}
	if !asset.RepositoryID.Valid {
		api.GinInternalError(c, errors.New("cover asset has no repository"), "Failed to resolve person cover repository")
		return
	}
	if h.repoPathResolver == nil {
		api.GinInternalError(c, errors.New("repository path resolver unavailable"), "Failed to resolve person cover repository")
		return
	}

	repoPath, err := h.repoPathResolver.GetRepositoryPath(uuid.UUID(asset.RepositoryID.Bytes).String())
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve person cover repository")
		return
	}

	fullPath, err := resolvePeopleRepositoryFile(repoPath, *person.CoverFaceImagePath)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve person cover path")
		return
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			api.GinNotFound(c, err, "Person cover file not found")
			return
		}
		api.GinInternalError(c, err, "Failed to access person cover file")
		return
	}

	etag := fmt.Sprintf(`"%d-%d"`, personID, fileInfo.ModTime().Unix())
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
	c.Header("Vary", "Accept-Encoding")
	c.Header("Content-Type", "image/webp")
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.File(fullPath)
}

func parsePersonID(c *gin.Context) (int32, bool) {
	rawID := strings.TrimSpace(c.Param("id"))
	personID, err := strconv.ParseInt(rawID, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid person ID")
		return 0, false
	}
	return int32(personID), true
}

func parseListPagination(c *gin.Context, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	if raw := strings.TrimSpace(c.DefaultQuery("limit", strconv.Itoa(defaultLimit))); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if raw := strings.TrimSpace(c.DefaultQuery("offset", "0")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

func scopedOwnerIDFromContext(c *gin.Context) *int32 {
	user, ok := currentUserFromContext(c)
	if !ok || service.IsAdminRole(user.Role) {
		return nil
	}

	ownerID := int32(user.UserID)
	return &ownerID
}

func parseRepositoryUUIDFromAssetFilter(filter dto.AssetFilterDTO) (pgtype.UUID, error) {
	if filter.RepositoryID == nil || strings.TrimSpace(*filter.RepositoryID) == "" {
		return pgtype.UUID{}, nil
	}

	repositoryID, err := uuid.Parse(strings.TrimSpace(*filter.RepositoryID))
	if err != nil {
		return pgtype.UUID{}, err
	}

	return pgtype.UUID{Bytes: repositoryID, Valid: true}, nil
}

func (h *PeopleHandler) resolveMediaOwnerScope(c *gin.Context) (*int32, bool) {
	user, ok := currentUserFromContext(c)
	if ok {
		if service.IsAdminRole(user.Role) {
			return nil, true
		}
		ownerID := int32(user.UserID)
		return &ownerID, true
	}

	mediaToken := strings.TrimSpace(c.Query("mt"))
	if mediaToken == "" {
		api.GinUnauthorized(c, errors.New("authentication required"), "Authentication required to access this face crop")
		return nil, false
	}
	if h.authService == nil {
		api.GinUnauthorized(c, errors.New("media token authentication unavailable"), "Authentication required to access this face crop")
		return nil, false
	}

	claims, err := h.authService.ValidateMediaToken(mediaToken)
	if err != nil {
		api.GinUnauthorized(c, errors.New("invalid or expired media token"), "Authentication required to access this face crop")
		return nil, false
	}
	if service.IsAdminRole(claims.Role) {
		return nil, true
	}

	ownerID := int32(claims.UserID)
	return &ownerID, true
}

func resolvePeopleRepositoryFile(repoPath, relativePath string) (string, error) {
	cleanRel := filepath.Clean(relativePath)
	if strings.HasPrefix(cleanRel, "..") {
		return "", errors.New("person cover path escapes repository root")
	}
	return filepath.Join(repoPath, cleanRel), nil
}
