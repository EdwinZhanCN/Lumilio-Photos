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
// @Param include_hidden query bool false "Include people hidden from the default grid" default(false)
// @Param limit query int false "Maximum number of results (max 100)" default(24)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} dto.ListPeopleResponseDTO "People listed successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people [get]
func (h *PeopleHandler) ListPeople(c *gin.Context) {
	limit, offset := parseListPagination(c, 24, 100)
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}
	includeHidden := parseBoolQuery(c, "include_hidden")

	people, total, err := h.faceService.ListPeople(c.Request.Context(), repositoryID, scopedOwnerIDFromContext(c), includeHidden, limit, offset)
	if err != nil {
		log.Printf("Failed to list people: %v", err)
		api.GinInternalError(c, err, "Failed to list people")
		return
	}

	items := make([]dto.PersonSummaryDTO, 0, len(people))
	for _, person := range people {
		items = append(items, dto.ToPersonSummaryDTO(person))
	}

	api.JSONOK(c, dto.ListPeopleResponseDTO{
		People: items,
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
	})
}

// RebuildPeople rebuilds recognized people with the HDBSCAN batch clusterer.
// @Summary Rebuild people clusters
// @Description Rebuild recognized people for the selected repository scope using HDBSCAN over face embeddings.
// @Tags people
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} dto.FaceClusterRebuildResponseDTO "People clusters rebuilt successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/rebuild [post]
// @Security BearerAuth
func (h *PeopleHandler) RebuildPeople(c *gin.Context) {
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}

	result, err := h.faceService.RebuildFaceClusters(c.Request.Context(), repositoryID, scopedOwnerIDFromContext(c))
	if err != nil {
		log.Printf("Failed to rebuild people clusters: %v", err)
		api.GinInternalError(c, err, "Failed to rebuild people clusters")
		return
	}

	api.JSONOK(c, dto.ToFaceClusterRebuildResponseDTO(result))
}

// GetPerson gets a single recognized person.
// @Summary Get person
// @Description Get a single recognized person by cluster ID.
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {object} dto.PersonDetailDTO "Person fetched successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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

	api.JSONOK(c, dto.ToPersonDetailDTO(*person))
}

// UpdatePerson renames a recognized person.
// @Summary Update person
// @Description Rename a recognized person. Successful updates mark the person as confirmed.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Person ID"
// @Param request body dto.UpdatePersonRequestDTO true "Person update payload"
// @Success 200 {object} dto.PersonDetailDTO "Person updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id} [patch]
// @Security BearerAuth
func (h *PeopleHandler) UpdatePerson(c *gin.Context) {
	personID, ok := parsePersonID(c)
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

	_, err := h.faceService.GetPerson(c.Request.Context(), personID, pgtype.UUID{}, scopedOwnerIDFromContext(c))
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

	person, err := h.faceService.GetPerson(c.Request.Context(), personID, pgtype.UUID{}, scopedOwnerIDFromContext(c))
	if err != nil {
		log.Printf("Failed to reload person %d after rename: %v", personID, err)
		api.GinInternalError(c, err, "Failed to reload person")
		return
	}

	api.JSONOK(c, dto.ToPersonDetailDTO(*person))
}

// ListPersonAssets lists assets belonging to a person while preserving the standard asset query shape.
// @Summary List person assets
// @Description List assets scoped to a specific person while reusing the unified asset query filters.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Person ID"
// @Param request body dto.AssetQueryRequestDTO true "Asset query parameters"
// @Success 200 {object} dto.QueryAssetsResponseDTO "Assets listed successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person not found"
// @Failure 503 {object} api.ErrorResponse "Semantic search unavailable"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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
	if err := validateAssetQuerySortBy(req.SortBy); err != nil {
		api.GinBadRequest(c, err, "sort_by must be 'recently_added' or 'date_captured'")
		return
	}
	if err := validateStackMode(req.StackMode); err != nil {
		api.GinBadRequest(c, err, "stack_mode must be 'collapsed' or 'expanded'")
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

	params := buildQueryAssetsParams(req.Query, req.SearchType, req.SortBy, req.ViewerTimezone, req.StackMode, req.Filter, req.Pagination)
	params.PersonID = &personID
	params = applyAssetOwnershipScope(c, params)

	result, err := h.assetService.QueryBrowseItems(c.Request.Context(), params)
	if err != nil {
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.GinError(c, 503, err, 503, "Semantic search is currently unavailable")
			return
		}
		log.Printf("Failed to list assets for person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to query person assets")
		return
	}

	api.JSONOK(c, toQueryBrowseResponseDTO(result, req.Pagination.Limit, req.Pagination.Offset))
}

// GetPersonCover serves the representative face crop for a person.
// @Summary Get person cover
// @Description Serve the representative face crop image for a recognized person.
// @Tags people
// @Produce image/webp
// @Param id path int true "Person ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {file} binary "Face crop"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Person cover not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
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

// ListPersonFaces lists UI-safe face crops belonging to a person.
// @Summary List person faces
// @Description List the individual face crops assigned to a person for correction workflows.
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Param limit query int false "Maximum number of results (max 200)" default(60)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} dto.ListPersonFacesResponseDTO "Faces listed successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/faces [get]
func (h *PeopleHandler) ListPersonFaces(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}
	limit, offset := parseListPagination(c, 60, 200)

	faces, total, err := h.faceService.ListPersonFaces(c.Request.Context(), personID, repositoryID, scopedOwnerIDFromContext(c), limit, offset)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		log.Printf("Failed to list faces for person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to list person faces")
		return
	}

	items := make([]dto.PersonFaceDTO, 0, len(faces))
	for _, face := range faces {
		items = append(items, dto.ToPersonFaceDTO(face))
	}

	api.JSONOK(c, dto.ListPersonFacesResponseDTO{
		Faces:  items,
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
	})
}

// GetPersonFaceCrop serves the crop image for a single face of a person.
// @Summary Get person face crop
// @Description Serve the face crop image for a single face belonging to a person.
// @Tags people
// @Produce image/webp
// @Param id path int true "Person ID"
// @Param faceId path int true "Face ID"
// @Param repository_id query string false "Optional repository UUID filter"
// @Success 200 {file} binary "Face crop"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Face crop not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/faces/{faceId}/crop [get]
func (h *PeopleHandler) GetPersonFaceCrop(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	faceID, ok := parseFaceID(c)
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

	crop, err := h.faceService.GetPersonFaceCrop(c.Request.Context(), personID, faceID, repositoryID, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Face crop not found")
			return
		}
		log.Printf("Failed to load face crop %d/%d: %v", personID, faceID, err)
		api.GinInternalError(c, err, "Failed to load face crop")
		return
	}
	if h.repoPathResolver == nil {
		api.GinInternalError(c, errors.New("repository path resolver unavailable"), "Failed to resolve face crop repository")
		return
	}

	repoPath, err := h.repoPathResolver.GetRepositoryPath(crop.RepositoryID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve face crop repository")
		return
	}

	fullPath, err := resolvePeopleRepositoryFile(repoPath, crop.FaceImagePath)
	if err != nil {
		api.GinInternalError(c, err, "Failed to resolve face crop path")
		return
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			api.GinNotFound(c, err, "Face crop file not found")
			return
		}
		api.GinInternalError(c, err, "Failed to access face crop file")
		return
	}

	etag := fmt.Sprintf(`"%d-%d-%d"`, personID, faceID, fileInfo.ModTime().Unix())
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

// MergePeople merges one or more source people into the target person.
// @Summary Merge people
// @Description Merge one or more source people into the target person. Assets remain in the library and corrections become manual.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Target person ID"
// @Param request body dto.MergePeopleRequestDTO true "Merge payload"
// @Success 200 {object} dto.PersonCorrectionResponseDTO "People merged successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/merge [post]
// @Security BearerAuth
func (h *PeopleHandler) MergePeople(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}

	var req dto.MergePeopleRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}
	if len(req.SourcePersonIDs) == 0 {
		api.GinBadRequest(c, errors.New("source_person_ids cannot be empty"), "Select at least one person to merge")
		return
	}

	ownerID := scopedOwnerIDFromContext(c)
	if err := h.faceService.MergePeople(c.Request.Context(), personID, req.SourcePersonIDs, ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		if errors.Is(err, service.ErrPeopleCrossOwner) {
			api.GinBadRequest(c, err, "People belong to different owners")
			return
		}
		log.Printf("Failed to merge people into %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to merge people")
		return
	}

	h.respondWithCorrectedPerson(c, personID, ownerID)
}

// MoveFace reassigns a single face to another person.
// @Summary Move face to another person
// @Description Reassign a single face from this person to another person as a manual correction.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Source person ID"
// @Param faceId path int true "Face ID"
// @Param request body dto.MoveFaceRequestDTO true "Move payload"
// @Success 200 {object} dto.PersonCorrectionResponseDTO "Face moved successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person or face not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/faces/{faceId}/move [post]
// @Security BearerAuth
func (h *PeopleHandler) MoveFace(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	faceID, ok := parseFaceID(c)
	if !ok {
		return
	}

	var req dto.MoveFaceRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}
	if req.TargetPersonID == personID {
		api.GinBadRequest(c, errors.New("target person must differ from source"), "Pick a different target person")
		return
	}

	ownerID := scopedOwnerIDFromContext(c)
	if err := h.faceService.MoveFace(c.Request.Context(), faceID, req.TargetPersonID, ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person or face not found")
			return
		}
		if errors.Is(err, service.ErrPeopleCrossOwner) {
			api.GinBadRequest(c, err, "Face and target person belong to different owners")
			return
		}
		log.Printf("Failed to move face %d to person %d: %v", faceID, req.TargetPersonID, err)
		api.GinInternalError(c, err, "Failed to move face")
		return
	}

	h.respondWithCorrectedPerson(c, personID, ownerID)
}

// RemoveFace detaches a face from a person.
// @Summary Remove face from person
// @Description Detach a face from this person, leaving the original asset unchanged.
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Param faceId path int true "Face ID"
// @Success 200 {object} dto.PersonCorrectionResponseDTO "Face removed successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person or face not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/faces/{faceId}/remove [post]
// @Security BearerAuth
func (h *PeopleHandler) RemoveFace(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}
	faceID, ok := parseFaceID(c)
	if !ok {
		return
	}

	ownerID := scopedOwnerIDFromContext(c)
	if err := h.faceService.RemoveFaceFromPerson(c.Request.Context(), faceID, personID, ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person or face not found")
			return
		}
		log.Printf("Failed to remove face %d from person %d: %v", faceID, personID, err)
		api.GinInternalError(c, err, "Failed to remove face")
		return
	}

	h.respondWithCorrectedPerson(c, personID, ownerID)
}

// SetPersonCover sets the representative cover face for a person.
// @Summary Set person cover
// @Description Set the representative cover face for a person. The face must belong to the person.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Person ID"
// @Param request body dto.SetPersonCoverRequestDTO true "Cover payload"
// @Success 200 {object} dto.PersonCorrectionResponseDTO "Cover updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person or face not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/cover [put]
// @Security BearerAuth
func (h *PeopleHandler) SetPersonCover(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}

	var req dto.SetPersonCoverRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ownerID := scopedOwnerIDFromContext(c)
	if err := h.faceService.SetPersonCover(c.Request.Context(), personID, req.FaceID, ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person or face not found")
			return
		}
		log.Printf("Failed to set cover for person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to set person cover")
		return
	}

	h.respondWithCorrectedPerson(c, personID, ownerID)
}

// SetPersonHidden hides or unhides a person from default people views.
// @Summary Set person hidden
// @Description Hide or unhide a person from the default people grid. Faces, assets and names are preserved.
// @Tags people
// @Accept json
// @Produce json
// @Param id path int true "Person ID"
// @Param request body dto.SetPersonHiddenRequestDTO true "Hidden payload"
// @Success 200 {object} dto.PersonCorrectionResponseDTO "Hidden state updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 404 {object} api.ErrorResponse "Person not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/people/{id}/hidden [put]
// @Security BearerAuth
func (h *PeopleHandler) SetPersonHidden(c *gin.Context) {
	personID, ok := parsePersonID(c)
	if !ok {
		return
	}

	var req dto.SetPersonHiddenRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	ownerID := scopedOwnerIDFromContext(c)
	person, err := h.faceService.SetPersonHidden(c.Request.Context(), personID, req.Hidden, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.GinNotFound(c, err, "Person not found")
			return
		}
		log.Printf("Failed to set hidden state for person %d: %v", personID, err)
		api.GinInternalError(c, err, "Failed to update person")
		return
	}

	detail := dto.ToPersonDetailDTO(*person)
	api.JSONOK(c, dto.PersonCorrectionResponseDTO{Person: &detail})
}

// respondWithCorrectedPerson reloads the target person after a correction and
// returns a focused correction response. A person that was emptied and removed
// by the correction yields a null person, which the frontend treats as a
// navigate-away signal.
func (h *PeopleHandler) respondWithCorrectedPerson(c *gin.Context, personID int32, ownerID *int32) {
	person, err := h.faceService.GetPerson(c.Request.Context(), personID, pgtype.UUID{}, ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			api.JSONOK(c, dto.PersonCorrectionResponseDTO{})
			return
		}
		log.Printf("Failed to reload person %d after correction: %v", personID, err)
		api.GinInternalError(c, err, "Failed to reload person")
		return
	}

	detail := dto.ToPersonDetailDTO(*person)
	api.JSONOK(c, dto.PersonCorrectionResponseDTO{Person: &detail})
}

func parseFaceID(c *gin.Context) (int32, bool) {
	rawID := strings.TrimSpace(c.Param("faceId"))
	faceID, err := strconv.ParseInt(rawID, 10, 32)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid face ID")
		return 0, false
	}
	return int32(faceID), true
}

func parseBoolQuery(c *gin.Context, key string) bool {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return false
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return parsed
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

	claims, err := h.authService.ValidateMediaToken(c.Request.Context(), mediaToken)
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
