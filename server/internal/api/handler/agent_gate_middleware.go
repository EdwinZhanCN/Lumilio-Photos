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
			api.GinInternalError(c, err, "Failed to load system settings")
			c.Abort()
			return
		}

		if !settings.LLM.AgentEnabled {
			api.GinNotFound(c, errors.New("llm agent is disabled"), "Agent is disabled")
			c.Abort()
			return
		}

		c.Next()
	}
}
