package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"server/config"
	"server/internal/db"

	"github.com/stretchr/testify/require"
)

func TestQueueHandlerLoadQueueSummariesRunsAgainstMigratedSQLiteSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	directory := t.TempDir()
	require.NoError(t, os.Chmod(directory, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(directory, "queue-handler.sqlite3"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, database.Close(context.Background()))
	})
	require.NoError(t, database.Migrate(ctx))

	summaries, err := NewQueueHandler(database.SQL, database.ReaderSQL).loadQueueSummaries(ctx)
	require.NoError(t, err)
	require.Empty(t, summaries)

	handler := NewQueueHandler(database.SQL, database.ReaderSQL)
	serve := func(method, target string, invoke func(*gin.Context), params ...gin.Param) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		requestContext, _ := gin.CreateTestContext(recorder)
		requestContext.Request = httptest.NewRequest(method, target, nil)
		requestContext.Params = params
		invoke(requestContext)
		return recorder
	}

	// Stage cards and delivery diagnostics are separate contracts.
	recorder := serve("GET", "/api/v1/admin/processing", handler.GetProcessing)
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	var shape map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &shape))
	require.Contains(t, shape, "stages")
	require.Contains(t, shape, "overview")
	require.NotContains(t, shape, "deliveries")

	recorder = serve("GET", "/api/v1/admin/processing/diagnostics?error_limit=0", handler.GetProcessingDiagnostics)
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	var deliveries DeliveryStatsDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &deliveries))
	require.NotNil(t, deliveries.Queues)

	recorder = serve("GET", "/api/v1/admin/processing/stages/reindex/items?state=failed", handler.GetProcessingStageItems, gin.Param{Key: "stage", Value: "reindex"})
	require.Equal(t, 404, recorder.Code, "reindex is a source, not a stage")
	recorder = serve("GET", "/api/v1/admin/processing/stages/metadata/items?state=done", handler.GetProcessingStageItems, gin.Param{Key: "stage", Value: "metadata"})
	require.Equal(t, 400, recorder.Code)

	recorder = serve("POST", "/api/v1/admin/processing/stages/metadata/retry", handler.RetryProcessingStage, gin.Param{Key: "stage", Value: "metadata"})
	require.Equal(t, 503, recorder.Code, "retry requires an explicit writer binding")
	handler.SetRetryWriter(database.Writer)
	recorder = serve("POST", "/api/v1/admin/processing/stages/backup/retry", handler.RetryProcessingStage, gin.Param{Key: "stage", Value: "backup"})
	require.Equal(t, 409, recorder.Code)
	recorder = serve("POST", "/api/v1/admin/processing/stages/metadata/retry", handler.RetryProcessingStage, gin.Param{Key: "stage", Value: "metadata"})
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
}
