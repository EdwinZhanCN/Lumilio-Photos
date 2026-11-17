//go:build sqlite_fts5

package processing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItemsListFailedNewestFirstWithOnlyPublicReasonCodes(t *testing.T) {
	f := newFixture(t)
	unsupported, exhausted, private, queued, deleted := f.asset(false), f.asset(false), f.asset(false), f.asset(false), f.asset(true)
	f.stage(unsupported, "derivatives", 1, 0, ReasonUnsupportedMedia)
	f.stage(exhausted, "derivatives", 1, 0, ReasonRetriesExhausted)
	f.stage(private, "derivatives", 1, 0, "open /private/path: permission denied")
	f.stage(queued, "derivatives", 1, 0, "")
	f.stage(deleted, "derivatives", 1, 0, ReasonUnsupportedMedia)

	reader := f.reader()
	first, err := reader.Items(t.Context(), StageThumbnails, ItemsFailed, 2, "")
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	require.NotEmpty(t, first.NextCursor)
	second, err := reader.Items(t.Context(), StageThumbnails, ItemsFailed, 2, first.NextCursor)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.Empty(t, second.NextCursor)

	codes := map[string]string{}
	for _, item := range append(first.Items, second.Items...) {
		require.NotNil(t, item.AssetID)
		require.NotEmpty(t, item.Label)
		codes[*item.AssetID] = *item.ReasonCode
	}
	require.Equal(t, map[string]string{
		unsupported.id.String(): ReasonUnsupportedMedia,
		exhausted.id.String():   ReasonRetriesExhausted,
		private.id.String():     ReasonProcessingFailed,
	}, codes, "raw errors never leave the Catalog and deleted assets are excluded")

	pending, err := reader.Items(t.Context(), StageThumbnails, ItemsQueued, 10, "")
	require.NoError(t, err)
	require.Len(t, pending.Items, 1)
	require.Equal(t, queued.id.String(), *pending.Items[0].AssetID)
	require.Nil(t, pending.Items[0].ReasonCode)

	_, err = reader.Items(t.Context(), StageID("reindex"), ItemsFailed, 10, "")
	require.ErrorIs(t, err, ErrUnknownStage)
}

func TestRetryReRequestsFailedAssetStageOnceThroughTheCatalog(t *testing.T) {
	f := newFixture(t)
	failed, other := f.asset(false), f.asset(false)
	f.stage(failed, "analyze", 1, 0, ReasonRetriesExhausted)
	f.stage(other, "derivatives", 1, 0, ReasonUnsupportedMedia)
	retrier := NewRetrier(f.database.Writer)

	result, err := retrier.Retry(t.Context(), StageMetadata)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Accepted)
	require.Zero(t, result.Remaining)
	require.NotNil(t, result.ReceiptID)

	summary, err := f.reader().Summary(t.Context())
	require.NoError(t, err)
	metadata := stageByID(t, summary, StageMetadata)
	require.Zero(t, metadata.Failed)
	require.Equal(t, int64(1), metadata.Queued, "the file re-enters the stage as work")
	require.Equal(t, map[string]int64{"retry": 1}, metadata.Sources)
	require.Equal(t, int64(1), stageByID(t, summary, StageThumbnails).Failed, "other stages are untouched")

	again, err := retrier.Retry(t.Context(), StageMetadata)
	require.NoError(t, err)
	require.Zero(t, again.Accepted, "a repeat call is harmless")
	require.Nil(t, again.ReceiptID)
}

func TestRetryCatalogStagesClearTerminalOutcomes(t *testing.T) {
	f := newFixture(t)
	f.exec(`INSERT INTO event_projection_pipeline_state(owner_id,source_revision,projection_version,applied_revision,priority,terminal_error,updated_at) VALUES(1,4,1,3,3,'boom',1)`)
	f.exec(`INSERT INTO ocr_projection_pipeline_state(scope,source_revision,projection_version,applied_revision,terminal_error,updated_at) VALUES('all',2,2,1,'boom',1)`)
	retrier := NewRetrier(f.database.Writer)

	events, err := retrier.Retry(t.Context(), StageEvents)
	require.NoError(t, err)
	require.Equal(t, int64(1), events.Accepted)
	text, err := retrier.Retry(t.Context(), StageTextSearch)
	require.NoError(t, err)
	require.Equal(t, int64(1), text.Accepted)

	summary, err := f.reader().Summary(t.Context())
	require.NoError(t, err)
	for _, id := range []StageID{StageEvents, StageTextSearch} {
		stage := stageByID(t, summary, id)
		require.Zero(t, stage.Failed, id)
		require.Equal(t, int64(1), stage.Queued, id)
	}

	_, err = retrier.Retry(t.Context(), StageImport)
	require.ErrorIs(t, err, ErrStageNotRetryable)
	_, err = retrier.Retry(t.Context(), StageBackup)
	require.ErrorIs(t, err, ErrStageNotRetryable)
}
