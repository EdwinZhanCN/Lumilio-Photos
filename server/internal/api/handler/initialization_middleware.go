package handler

import (
	"errors"

	"server/internal/api"
	"server/internal/api/problem"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// RequireAppInitialized rejects business APIs until first-run setup is complete,
// reading the single bootstrap phase rather than re-probing the individual gates.
func RequireAppInitialized(bootstrap service.BootstrapService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bootstrap == nil {
			api.WriteProblem(c, api.KnownProblem(problem.AppNotInitialized, errors.New("app_not_initialized")))
			c.Abort()
			return
		}

		ready, err := bootstrap.IsReady(c.Request.Context())
		if err != nil || !ready {
			api.WriteProblem(c, api.KnownProblem(problem.AppNotInitialized, errors.New("app_not_initialized")))
			c.Abort()
			return
		}

		c.Next()
	}
}
