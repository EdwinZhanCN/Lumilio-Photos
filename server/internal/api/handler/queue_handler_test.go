package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http/httptest"
	"os"
	"path/filepath"
	"server/internal/db/catalogtx"
	"testing"
	"time"

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

	// The combined HTTP contract includes both data domains, including empty queues.
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest("GET", "/api/v1/admin/monitor/processing?error_limit=0", nil)
	NewQueueHandler(database.SQL, database.ReaderSQL).GetProcessingMonitor(requestContext)
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	var response ProcessingMonitorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.GeneratedAt.IsZero())
	require.NotNil(t, response.Deliveries.Queues)
	require.Zero(t, response.Deliveries.Completed)
	require.Zero(t, response.Processing.PendingAssets)
	var shape map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &shape))
	require.Contains(t, shape, "processing")
	require.Contains(t, shape, "deliveries")
	require.NotContains(t, shape, "completed")

}

func TestProcessingStatsCountCurrentFilesInsteadOfStagesOrDeliveryHistory(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	require.NoError(t, os.Chmod(directory, 0700))
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "processing.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.MigrateCatalog(ctx))
	require.NoError(t, database.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle) VALUES(1,'test','unused',1,1,X'01')`); err != nil {
			return err
		}
		for index := 0; index < 4; index++ {
			assetID, contentID := uuid.New(), uuid.New()
			if _, err := tx.Exec(`INSERT INTO content_objects(content_id,hash_algorithm,full_hash,file_size,created_at) VALUES(?,'blake3-v1',?,1,1)`, contentID.String(), fmt.Sprintf("%064x", index+1)); err != nil {
				return err
			}
			deleted := index == 3
			if _, err := tx.Exec(`INSERT INTO assets(asset_id,owner_id,content_id,type,original_filename,mime_type,upload_time,updated_at,is_deleted) VALUES(?,1,?,'PHOTO','fixture.jpg','image/jpeg',1,1,?)`, assetID.String(), contentID.String(), deleted); err != nil {
				return err
			}
			for _, stage := range []string{"analyze", "derivatives", "enrich"} {
				applied := 0
				var terminal any
				if index == 1 && stage != "analyze" || deleted {
					terminal = "unsupported_media"
				}
				if index == 2 {
					applied = 1
				}
				if _, err := tx.Exec(`INSERT INTO asset_pipeline_state(asset_id,source_content_id,stage,pipeline_version,desired_version,applied_version,priority,terminal_error,updated_at) VALUES(?,?,?,'asset-v1',1,?,3,?,1)`, assetID.String(), contentID.String(), stage, applied, terminal); err != nil {
					return err
				}
			}
			if index == 0 {
				if _, err := tx.Exec(`INSERT INTO asset_pipeline_failures VALUES(?,'analyze',?,'asset-v1',1,1,?,'processing_retry_pending',1)`, assetID.String(), contentID.String(), time.Now().Add(time.Hour).UnixMicro()); err != nil {
					return err
				}
			}
		}
		return nil
	}))
	// No QueueDB connection is supplied: these counts must come only from Catalog.
	stats, err := NewQueueHandler(nil, database.ReaderSQL).loadProcessingStats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.PendingAssets)
	require.Equal(t, int64(1), stats.FailedAssets)
	require.Equal(t, int64(1), stats.RetryWaitingStages)
	require.Zero(t, stats.PendingRepositories)
	require.Zero(t, stats.PendingProjections)
	// Enrichment is reported per file beside, not instead of, the file backlog:
	// index 0 is pending, index 1 is terminal, index 2 is applied, index 3 is deleted.
	require.Equal(t, int64(1), stats.PendingAnalysisAssets)
	require.Equal(t, int64(1), stats.FailedAnalysisAssets)
	require.Zero(t, stats.PendingReindexRequests)
}
