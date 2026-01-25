package handler

import (
	"server/internal/api"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check HTTP requests
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// Check handles health check requests
// @Summary Health check
// @Description Check if the server is healthy
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=HealthResponse} "Server is healthy"
// @Router /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	api.GinSuccess(c, HealthResponse{Status: "ok"})
}
