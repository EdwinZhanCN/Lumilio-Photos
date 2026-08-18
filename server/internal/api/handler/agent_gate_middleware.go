package handler

import (
	"errors"

	"server/internal/api"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

func RequireLLMAgentEnabled(settingsService service.SettingsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := settingsService.GetSystemSettings(c.Request.Context())
		if err != nil {
			api.WriteProblem(c, api.Internal(err))
			c.Abort()
			return
		}

		if !settings.LLM.AgentEnabled {
			api.WriteProblem(c, api.NotFound(errors.New("llm agent is disabled")))
			c.Abort()
			return
		}

		c.Next()
	}
}
