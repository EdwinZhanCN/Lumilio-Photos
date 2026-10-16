//go:build sqlite_fts5

package processing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSummaryReportsEveryStageInOneShapeAndOrder(t *testing.T) {
	summary, err := newFixture(t).reader().Summary(t.Context())
	require.NoError(t, err)
	require.Len(t, summary.Stages, len(Stages))
	for index, stage := range summary.Stages {
		require.Equal(t, Stages[index].ID, stage.ID)
		require.Equal(t, StatusIdle, stage.Status, stage.ID)
		require.Zero(t, stage.Remaining+stage.Failed, stage.ID)
		// Done exists exactly for per-file stages, and is zero rather than absent.
		if Stages[index].AssetStage != "" {
			require.NotNil(t, stage.Done, stage.ID)
			require.Zero(t, *stage.Done, stage.ID)
		} else {
			require.Nil(t, stage.Done, stage.ID)
		}
	}
}

func TestSummaryCountsCatalogWorkAndOnlyExecutionFactsFromRiver(t *testing.T) {
	f := newFixture(t)
	retrying, plain, failed, done, deleted := f.asset(false), f.asset(false), f.asset(false), f.asset(false), f.asset(true)
	f.stage(retrying, "analyze", 2, 1, "")
	f.retryLater(retrying, "analyze", 2)
	f.stage(plain, "analyze", 1, 0, "")
	f.stage(failed, "analyze", 1, 0, "unsupported_media")
	f.stage(done, "analyze", 1, 1, "")
	f.stage(deleted, "analyze", 1, 0, "")
	f.stage(deleted, "derivatives", 1, 0, "corrupt")

	// Reindex work is attributed as a source of its stage, never its own card.
	reindex := f.receipt("reindex", "all", "pending", time.Now())
	f.stage(plain, "enrich", 3, 2, "")
	f.exec(`INSERT INTO asset_pipeline_receipt_stages(receipt_id,asset_id,stage,desired_version) VALUES(?,?,?,3)`, reindex.String(), plain.id.String(), "enrich")

	f.exec(`INSERT INTO event_projection_pipeline_state(owner_id,source_revision,projection_version,applied_revision,priority,updated_at) VALUES(1,4,1,3,3,1)`)
	f.receipt("ingest", "commit-pending", "pending", time.Now())
	f.receipt("ingest", "commit-recent", "failed", time.Now())
	f.receipt("ingest", "commit-old", "failed", time.Now().Add(-ImportFailureWindow-time.Hour))
	f.receipt("backup", "catalog", "failed", time.Now())

	f.delivery("analyze_asset", "", "running")
	f.delivery("enrich_asset", "", "retryable")
	f.delivery("rebuild_projection_batch", "event", "retryable")
	f.delivery("analyze_asset", "", "completed")

	summary, err := f.reader().Summary(t.Context())
	require.NoError(t, err)

	metadata := stageByID(t, summary, StageMetadata)
	require.Equal(t, int64(1), metadata.Running)
	require.Equal(t, int64(1), metadata.Retrying)
	require.Zero(t, metadata.Queued, "pending minus running minus retrying, clamped")
	require.Equal(t, int64(2), metadata.Remaining)
	require.Equal(t, int64(1), metadata.Failed, "deleted assets never count")
	require.Equal(t, int64(1), *metadata.Done)
	require.Equal(t, StatusAttention, metadata.Status, "failures outrank running work")
	require.NotNil(t, metadata.LastActivityAt)

	thumbnails := stageByID(t, summary, StageThumbnails)
	require.Zero(t, thumbnails.Failed, "a deleted asset's terminal failure is excluded")

	analysis := stageByID(t, summary, StageAnalysis)
	require.Zero(t, analysis.Retrying, "per-file stages take retrying from the Catalog schedule, not River")
	require.Equal(t, int64(1), analysis.Queued)
	require.Equal(t, map[string]int64{"reindex": 1}, analysis.Sources)
	require.Equal(t, StatusWaiting, analysis.Status)

	events := stageByID(t, summary, StageEvents)
	require.Equal(t, int64(1), events.Retrying, "catalog stages take retrying from River deliveries")
	require.Zero(t, events.Queued)
	require.Equal(t, int64(1), events.Remaining)
	require.Equal(t, StatusRetrying, events.Status)

	imports := stageByID(t, summary, StageImport)
	require.Equal(t, int64(1), imports.Queued)
	require.Equal(t, int64(1), imports.Failed, "import failures age out of the card")
	require.NotNil(t, imports.OldestQueuedAt)

	backup := stageByID(t, summary, StageBackup)
	require.Equal(t, int64(1), backup.Failed)
	require.False(t, backup.Retryable)

	require.Equal(t, int64(4), summary.Overview.MediaTotal)
	require.Equal(t, int64(2), summary.Overview.MediaInProgress)
	require.Equal(t, int64(1), summary.Overview.FailedMedia)
	require.Equal(t, int64(1), summary.Overview.Running)
	require.Equal(t, int64(1), summary.Overview.CatalogPending)
}

func TestSummaryWithoutQueueDBStillReportsCatalogWork(t *testing.T) {
	f := newFixture(t)
	failed := f.asset(false)
	f.stage(failed, "transcode", 1, 0, "corrupt")
	f.stage(f.asset(false), "transcode", 1, 0, "")

	summary, err := NewReader(f.database.ReaderSQL, nil).Summary(t.Context())
	require.NoError(t, err)
	video := stageByID(t, summary, StageVideo)
	require.Equal(t, int64(1), video.Failed)
	require.Equal(t, int64(1), video.Queued)
	require.Zero(t, video.Running)
	require.Nil(t, video.LastActivityAt)
}
