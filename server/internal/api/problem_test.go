package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestProblemWriterProducesLanguageNeutralRFC9457Response(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/probe", func(c *gin.Context) {
		WriteProblem(c, Internal(errors.New("sqlite: secret database path")))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, ProblemMediaType, recorder.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "about:blank", body["type"])
	require.Equal(t, float64(http.StatusInternalServerError), body["status"])
	require.Regexp(t, `^urn:lumilio:problem:[0-9a-f]{32}$`, body["instance"])
	for _, forbidden := range []string{"title", "detail", "code", "error_code", "message", "error"} {
		_, exists := body[forbidden]
		require.Falsef(t, exists, "public Problem contains forbidden member %q", forbidden)
	}
	require.False(t, strings.Contains(recorder.Body.String(), "sqlite"))
}

func TestProblemOccurrenceCorrelatesWithExactlyOneStructuredRequestLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, observed := observer.New(zapcore.DebugLevel)
	router := gin.New()
	router.Use(requestErrorLogger(zap.New(core)))
	cause := errors.New("private database diagnostic")
	router.GET("/api/v1/probe", func(c *gin.Context) {
		WriteProblem(c, Internal(cause))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/probe", nil))

	var body ProblemResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	entries := observed.All()
	require.Len(t, entries, 1)
	require.Equal(t, "http request failed", entries[0].Message)
	fields := entries[0].ContextMap()
	require.Equal(t, body.Instance, fields["problem_instance"])
	require.Equal(t, body.Type, fields["problem_type"])
	require.Equal(t, cause.Error(), fields["error"])
	require.Equal(t, "/api/v1/probe", fields["path"])
}
