package handler

import (
	"errors"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

type SpeciesHandler struct {
	speciesReferenceService service.SpeciesReferenceService
}

func NewSpeciesHandler(speciesReferenceService service.SpeciesReferenceService) *SpeciesHandler {
	return &SpeciesHandler{
		speciesReferenceService: speciesReferenceService,
	}
}

// GetSpeciesReference fetches wiki/reference image data from iNaturalist.
// @Summary Get species reference
// @Description Fetch a species wiki summary and reference image from iNaturalist by scientific name, with optional common name fallback.
// @Tags species
// @Accept json
// @Produce json
// @Param scientific_name query string false "Scientific name" example("Rucervus duvaucelii")
// @Param common_name query string false "Common name fallback" example("Barasingha")
// @Param locale query string false "iNaturalist locale for localized common names and wiki summaries" example("zh")
// @Success 200 {object} dto.SpeciesReferenceResponseDTO "Species reference retrieved successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid query"
// @Failure 404 {object} api.ProblemResponse "Species reference not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/species/reference [get]
func (h *SpeciesHandler) GetSpeciesReference(c *gin.Context) {
	scientificName := strings.TrimSpace(c.Query("scientific_name"))
	commonName := strings.TrimSpace(c.Query("common_name"))
	locale := strings.TrimSpace(c.Query("locale"))
	if scientificName == "" && commonName == "" {
		api.WriteProblem(c, api.BadRequest(errors.New("scientific_name or common_name is required")))
		return
	}

	ref, err := h.speciesReferenceService.FetchReference(c.Request.Context(), service.SpeciesReferenceQuery{
		ScientificName: scientificName,
		CommonName:     commonName,
		Locale:         locale,
	})
	if err != nil {
		if errors.Is(err, service.ErrSpeciesReferenceNotFound) {
			api.WriteProblem(c, api.NotFound(err))
			return
		}
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.ToSpeciesReferenceResponseDTO(ref))
}
