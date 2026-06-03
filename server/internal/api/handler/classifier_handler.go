package handler

import (
	"errors"
	"net/http"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

type ClassifierHandler struct {
	classifierService service.ClassifierService
}

func NewClassifierHandler(classifierService service.ClassifierService) *ClassifierHandler {
	return &ClassifierHandler{classifierService: classifierService}
}

// PreviewClassifier evaluates an ad-hoc zero-shot classifier over the library.
// @Summary Preview a zero-shot classifier
// @Description Embed positive/negative prompts with semantic and return library assets whose contrastive score exceeds the threshold. Used to tune prompts and thresholds before persisting a smart album. Requires the semantic embedding pipeline and a reachable semantic text-embed task.
// @Tags classifiers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ClassifierPreviewRequestDTO true "Prompts and threshold"
// @Success 200 {object} api.Result{data=dto.ClassifierPreviewResponseDTO} "Preview matches retrieved successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 503 {object} api.Result "Classification unavailable"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/classifiers/preview [post]
func (h *ClassifierHandler) PreviewClassifier(c *gin.Context) {
	var req dto.ClassifierPreviewRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	matches, err := h.classifierService.Preview(
		c.Request.Context(),
		req.PositivePrompts,
		req.NegativePrompts,
		req.Threshold,
		req.Limit,
	)
	if err != nil {
		if errors.Is(err, service.ErrSemanticSearchUnavailable) {
			api.GinError(c, http.StatusServiceUnavailable, err, http.StatusServiceUnavailable, "Classification is unavailable")
			return
		}
		api.GinInternalError(c, err, "Failed to run classifier preview")
		return
	}

	api.GinSuccess(c, dto.ToClassifierPreviewResponseDTO(matches))
}
