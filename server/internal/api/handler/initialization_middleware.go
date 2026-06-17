package handler

import (
	"errors"
	"net/http"

	"server/internal/api"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// RequireAppInitialized rejects business APIs until first-run setup is complete,
// reading the single bootstrap phase rather than re-probing the individual gates.
func RequireAppInitialized(bootstrap service.BootstrapService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bootstrap == nil {
			api.GinError(c, http.StatusConflict, errors.New("app_not_initialized"), http.StatusConflict, "app_not_initialized")
			c.Abort()
			return
		}

		ready, err := bootstrap.IsReady(c.Request.Context())
		if err != nil || !ready {
			api.GinError(c, http.StatusConflict, errors.New("app_not_initialized"), http.StatusConflict, "app_not_initialized")
			c.Abort()
			return
		}

		c.Next()
	}
}
