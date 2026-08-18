package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/api"
	"server/internal/api/problem"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type notReadyBootstrap struct{}

func (notReadyBootstrap) Phase(context.Context) (string, error)     { return "catalog_ready", nil }
func (notReadyBootstrap) Reconcile(context.Context) (string, error) { return "catalog_ready", nil }
func (notReadyBootstrap) IsReady(context.Context) (bool, error)     { return false, nil }

func TestRequireAppInitializedReturnsTypedProblem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireAppInitialized(notReadyBootstrap{}))
	router.GET("/api/v1/probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil))

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, api.ProblemMediaType, recorder.Header().Get("Content-Type"))
	var body problem.Details
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, problem.AppNotInitialized.Type, body.Type)
	require.Equal(t, http.StatusConflict, body.Status)
	require.Regexp(t, `^urn:lumilio:problem:[0-9a-f]{32}$`, body.Instance)
}
