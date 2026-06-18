package handler

import (
	"log"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type LocationHandler struct {
	locationService service.LocationService
	queueClient     *river.Client[pgx.Tx]
}

func NewLocationHandler(locationService service.LocationService, queueClient *river.Client[pgx.Tx]) *LocationHandler {
	return &LocationHandler{
		locationService: locationService,
		queueClient:     queueClient,
	}
}

// ListLocationClusters returns persisted geohash location clusters.
// @Summary Get location clusters
// @Description Return paginated persisted photo location clusters with cached labels when available.
// @Tags locations
// @Accept json
// @Produce json
// @Param limit query int false "Page size (1-1000)" default(100)
// @Param offset query int false "Page offset" default(0)
// @Param repository_id query string false "Optional repository UUID filter"
// @Param geohash query string false "Optional geohash filter"
// @Success 200 {object} dto.LocationClusterListResponseDTO "Location clusters retrieved successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request parameters"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/locations/clusters [get]
func (h *LocationHandler) ListLocationClusters(c *gin.Context) {
	limit, err := parseIntQueryWithRange(c, "limit", 100, 1, 1000)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid limit parameter")
		return
	}
	offset, err := parseIntQueryWithRange(c, "offset", 0, 0, 10000000)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid offset parameter")
		return
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(c.Query("repository_id")); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.GinBadRequest(c, err, "Invalid repository_id parameter")
			return
		}
		repositoryID = &rawRepoID
	}

	var geohash *string
	if rawGeohash := strings.TrimSpace(c.Query("geohash")); rawGeohash != "" {
		geohash = &rawGeohash
	}

	clusters, total, err := h.locationService.ListLocationClusters(c.Request.Context(), applyLocationClusterOwnershipScope(c, service.ListLocationClustersParams{
		RepositoryID: repositoryID,
		Geohash:      geohash,
		Limit:        limit,
		Offset:       offset,
	}))
	if err != nil {
		log.Printf("Failed to query location clusters: %v", err)
		api.GinInternalError(c, err, "Failed to query location clusters")
		return
	}

	dtos := make([]dto.LocationClusterDTO, 0, len(clusters))
	for _, cluster := range clusters {
		dtos = append(dtos, dto.LocationClusterDTO{
			ClusterID:         cluster.ClusterID,
			RepositoryID:      cluster.RepositoryID,
			Geohash:           cluster.Geohash,
			Precision:         cluster.Precision,
			CentroidLatitude:  cluster.CentroidLatitude,
			CentroidLongitude: cluster.CentroidLongitude,
			PhotoCount:        cluster.PhotoCount,
			Label:             cluster.Label,
			Country:           cluster.Country,
			Region:            cluster.Region,
			City:              cluster.City,
			Provider:          cluster.Provider,
			GeocodeStatus:     cluster.GeocodeStatus,
			GeocodedAt:        cluster.GeocodedAt,
		})
	}

	totalInt := int(total)
	api.JSONOK(c, dto.LocationClusterListResponseDTO{
		Clusters: dtos,
		Total:    &totalInt,
		Limit:    limit,
		Offset:   offset,
	})
}

// RebuildLocationClusters queues a location cluster rebuild.
// @Summary Queue location cluster rebuild
// @Description Queue a location cluster rebuild for all photos or one repository.
// @Tags locations
// @Accept json
// @Produce json
// @Param request body dto.RebuildLocationClustersRequestDTO false "Rebuild request"
// @Success 200 {object} dto.RebuildLocationClustersResponseDTO "Location cluster rebuild queued successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request body"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/locations/rebuild [post]
func (h *LocationHandler) RebuildLocationClusters(c *gin.Context) {
	var req dto.RebuildLocationClustersRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request")
		return
	}

	var repositoryID *string
	if rawRepoID := strings.TrimSpace(req.RepositoryID); rawRepoID != "" {
		if _, err := uuid.Parse(rawRepoID); err != nil {
			api.GinBadRequest(c, err, "Invalid repository_id")
			return
		}
		repositoryID = &rawRepoID
	}

	args := jobs.RebuildLocationClustersArgs{
		RepositoryID: repositoryID,
	}
	opts := args.InsertOpts()
	opts.Queue = "rebuild_location_clusters"
	jobResult, err := h.queueClient.Insert(c.Request.Context(), args, &opts)
	if err != nil {
		log.Printf("Failed to enqueue location cluster rebuild: %v", err)
		api.GinInternalError(c, err, "Failed to enqueue location cluster rebuild")
		return
	}

	jobID := int64(0)
	if jobResult != nil && jobResult.Job != nil {
		jobID = jobResult.Job.ID
	}
	api.JSONOK(c, dto.RebuildLocationClustersResponseDTO{
		Status:       "queued",
		Message:      "Location cluster rebuild queued successfully",
		JobID:        jobID,
		RepositoryID: repositoryID,
	})
}
