//go:build sqlite_fts5

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"server/config"
	"server/internal/api/dto"
	"server/internal/db"
	"server/internal/event"
	"server/internal/service"
	"server/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEventHandlerListEventsProjectsToRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "events.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))

	_, err = database.SQL.ExecContext(ctx, `
INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle)
VALUES(1,'owner','hash',1,1,x'01');
INSERT INTO storage_locations(storage_location_id,name,path,kind,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000001','root','/events','default',1,1);
INSERT INTO repositories(repo_id,name,path,reachability,activity,created_at,updated_at,default_owner_id,storage_location_id)
VALUES
 ('00000000-0000-0000-0000-000000000002','first','/events/first','active','idle',1,1,1,
  '00000000-0000-0000-0000-000000000001'),
 ('00000000-0000-0000-0000-000000000003','second','/events/second','active','idle',1,1,1,
  '00000000-0000-0000-0000-000000000001');
INSERT INTO content_objects(content_id,hash_algorithm,full_hash,file_size,created_at)
VALUES
 ('00000000-0000-0000-0000-000000000041','blake3-v1','1111111111111111111111111111111111111111111111111111111111111111',1,1),
 ('00000000-0000-0000-0000-000000000042','blake3-v1','2222222222222222222222222222222222222222222222222222222222222222',1,1),
 ('00000000-0000-0000-0000-000000000043','blake3-v1','3333333333333333333333333333333333333333333333333333333333333333',1,1);
INSERT INTO assets(asset_id,owner_id,content_id,type,original_filename,mime_type,
 upload_time,taken_time,status,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000011',1,'00000000-0000-0000-0000-000000000041',
  'PHOTO','first.jpg','image/jpeg',1000000,1000000,'{"state":"completed"}',1),
 ('00000000-0000-0000-0000-000000000012',1,'00000000-0000-0000-0000-000000000042',
  'PHOTO','second.jpg','image/jpeg',2000000,2000000,'{"state":"completed"}',1),
 ('00000000-0000-0000-0000-000000000013',1,'00000000-0000-0000-0000-000000000043',
  'PHOTO','second-only.jpg','image/jpeg',3000000,3000000,'{"state":"completed"}',1);
INSERT INTO repository_nodes(node_id,repository_id,parent_node_id,name,name_key,kind,
 observation_revision,file_size,created_at,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000051','00000000-0000-0000-0000-000000000002',NULL,'','','directory',1,NULL,1,1),
 ('00000000-0000-0000-0000-000000000052','00000000-0000-0000-0000-000000000003',NULL,'','','directory',1,NULL,1,1),
 ('00000000-0000-0000-0000-000000000061','00000000-0000-0000-0000-000000000002','00000000-0000-0000-0000-000000000051','first.jpg','first.jpg','file',1,1,1,1),
 ('00000000-0000-0000-0000-000000000062','00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000052','second.jpg','second.jpg','file',1,1,1,1),
 ('00000000-0000-0000-0000-000000000063','00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000052','second-only.jpg','second-only.jpg','file',1,1,1,1);
INSERT INTO asset_locations(location_id,node_id,asset_id,bound_observation_revision,created_at,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000071','00000000-0000-0000-0000-000000000061','00000000-0000-0000-0000-000000000011',1,1,1),
 ('00000000-0000-0000-0000-000000000072','00000000-0000-0000-0000-000000000062','00000000-0000-0000-0000-000000000012',1,1,1),
 ('00000000-0000-0000-0000-000000000073','00000000-0000-0000-0000-000000000063','00000000-0000-0000-0000-000000000013',1,1,1);
INSERT INTO media_items(media_item_id,owner_id,repository_id,media_kind,primary_asset_id,created_at,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000021',1,'00000000-0000-0000-0000-000000000002','photo',
  '00000000-0000-0000-0000-000000000011',1,1),
 ('00000000-0000-0000-0000-000000000022',1,'00000000-0000-0000-0000-000000000003','photo',
  '00000000-0000-0000-0000-000000000012',1,1),
 ('00000000-0000-0000-0000-000000000023',1,'00000000-0000-0000-0000-000000000003','photo',
  '00000000-0000-0000-0000-000000000013',1,1);
INSERT INTO media_item_assets(asset_id,media_item_id,relation,position,created_at)
VALUES
 ('00000000-0000-0000-0000-000000000011','00000000-0000-0000-0000-000000000021','original',0,1),
 ('00000000-0000-0000-0000-000000000012','00000000-0000-0000-0000-000000000022','original',0,1),
 ('00000000-0000-0000-0000-000000000013','00000000-0000-0000-0000-000000000023','original',0,1);
INSERT INTO events(event_id,owner_id,status,start_at,end_at,generated_cover_media_item_id,
 is_hidden,algorithm_version,created_at,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000031',1,'active',2000000,2000000,
  NULL,0,'events-v1',1,1),
 ('00000000-0000-0000-0000-000000000032',1,'active',3000000,3000000,
  NULL,1,'events-v1',1,1);
INSERT INTO event_media_items(event_id,owner_id,media_item_id,position,source,evidence,created_at)
VALUES
 ('00000000-0000-0000-0000-000000000031',1,'00000000-0000-0000-0000-000000000021',0,'automatic','{}',1),
 ('00000000-0000-0000-0000-000000000031',1,'00000000-0000-0000-0000-000000000022',1,'automatic','{}',1),
 ('00000000-0000-0000-0000-000000000032',1,'00000000-0000-0000-0000-000000000023',0,'automatic','{}',1);
UPDATE events
SET generated_cover_media_item_id = CASE event_id
  WHEN '00000000-0000-0000-0000-000000000031' THEN '00000000-0000-0000-0000-000000000022'
  ELSE '00000000-0000-0000-0000-000000000023'
END;`)
	require.NoError(t, err)

	handler := NewEventHandler(event.NewService(database.SQL), database.SQL, nil)
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Set("current_user", &service.UserResponse{UserID: 1, Username: "owner"})
	requestContext.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/events?repository_id=00000000-0000-0000-0000-000000000002",
		nil,
	)

	handler.ListEvents(requestContext)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response dto.EventListPageDTO
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Events, 1)
	require.Equal(t, "00000000-0000-0000-0000-000000000031", response.Events[0].EventID)
	require.Equal(t, 1, response.Events[0].MediaCount)
	require.Equal(t, 1, response.Events[0].DisplayableCount)
	require.Equal(t, "00000000-0000-0000-0000-000000000021", *response.Events[0].CoverMediaItemID)
	require.Equal(t, "00000000-0000-0000-0000-000000000011", *response.Events[0].CoverAssetID)

	allRecorder := httptest.NewRecorder()
	allContext, _ := gin.CreateTestContext(allRecorder)
	allContext.Set("current_user", &service.UserResponse{UserID: 1, Username: "owner"})
	allContext.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/events?repository_id=00000000-0000-0000-0000-000000000003&include_hidden=true",
		nil,
	)
	handler.ListEvents(allContext)
	require.Equal(t, http.StatusOK, allRecorder.Code)
	require.NoError(t, json.Unmarshal(allRecorder.Body.Bytes(), &response))
	require.Len(t, response.Events, 2)
	require.True(t, response.Events[0].IsHidden)
}

func TestEventHandlerListEventsRejectsInvalidRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Set("current_user", &service.UserResponse{UserID: 1, Username: "owner"})
	requestContext.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/events?repository_id=not-a-uuid",
		nil,
	)

	(&EventHandler{}).ListEvents(requestContext)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

// A projection snapshot copies each retained Event's user state (title, cover
// override, hidden) and republishes it. A rename committed while that
// projection is in flight, for example while late EXIF keeps invalidating an
// import, must fence the stale snapshot out instead of being silently reverted.
func TestEventHandlerPatchFencesInFlightProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "events.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))

	_, err = database.SQL.ExecContext(ctx, `
INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle)
VALUES(1,'owner','hash',1,1,x'01');
INSERT INTO storage_locations(storage_location_id,name,path,kind,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000001','root','/events','default',1,1);
INSERT INTO repositories(repo_id,name,path,reachability,activity,created_at,updated_at,default_owner_id,storage_location_id)
VALUES('00000000-0000-0000-0000-000000000002','repo','/events/repo','active','idle',1,1,1,
       '00000000-0000-0000-0000-000000000001');`)
	require.NoError(t, err)
	repositoryID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	for i, assetID := range []string{
		"00000000-0000-0000-0000-000000000011",
		"00000000-0000-0000-0000-000000000012",
	} {
		taken := int64(1_000_000 + i*1_000_000)
		_, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: uuid.MustParse(assetID), RepositoryID: repositoryID, OwnerID: 1,
			MIMEType: "image/jpeg", FileSize: 1, UploadTime: taken, TakenTime: &taken,
		})
		require.NoError(t, err)
		mediaItemID := uuid.NewString()
		_, err = database.SQL.ExecContext(ctx, `
INSERT INTO media_items(media_item_id,owner_id,repository_id,media_kind,primary_asset_id,created_at,updated_at)
VALUES(?,1,?,'photo',?,1,1)`, mediaItemID, repositoryID.String(), assetID)
		require.NoError(t, err)
		_, err = database.SQL.ExecContext(ctx, `
INSERT INTO media_item_assets(asset_id,media_item_id,relation,position,created_at)
VALUES(?,?,'original',0,1)`, assetID, mediaItemID)
		require.NoError(t, err)
	}
	markFactsChanged := func() {
		tx, err := database.SQL.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, event.MarkEventFactsChangedTx(ctx, tx, 1, "asset_metadata_changed"))
		require.NoError(t, tx.Commit())
	}

	eventService := event.NewService(database.SQL)
	markFactsChanged()
	_, err = eventService.RebuildOwner(ctx, 1, false)
	require.NoError(t, err)
	var eventID string
	require.NoError(t, database.SQL.QueryRowContext(ctx,
		`SELECT event_id FROM events WHERE owner_id=1 AND status='active'`).Scan(&eventID))

	// Late EXIF invalidates the owner; the projection worker snapshots the
	// catalog (title still unset) before the user renames the Event.
	markFactsChanged()
	var revision uint64
	require.NoError(t, database.SQL.QueryRowContext(ctx,
		`SELECT source_revision FROM event_owner_state WHERE owner_id=1`).Scan(&revision))
	prepared, err := eventService.PrepareAtRevision(ctx, 1, revision)
	require.NoError(t, err)

	handler := NewEventHandler(eventService, database.SQL, nil)
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Set("current_user", &service.UserResponse{UserID: 1, Username: "owner"})
	requestContext.Params = gin.Params{{Key: "id", Value: eventID}}
	requestContext.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/events/"+eventID,
		strings.NewReader(`{"title_override":"Renamed"}`))
	requestContext.Request.Header.Set("Content-Type", "application/json")
	handler.PatchEvent(requestContext)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	// The in-flight projection now tries to publish its pre-rename snapshot.
	tx, err := database.SQL.BeginTx(ctx, nil)
	require.NoError(t, err)
	applyErr := eventService.ApplyPreparedRebuildTx(ctx, tx, prepared)
	if applyErr == nil {
		require.NoError(t, tx.Commit())
	} else {
		require.NoError(t, tx.Rollback())
	}

	var title sql.NullString
	require.NoError(t, database.SQL.QueryRowContext(ctx,
		`SELECT title_override FROM events WHERE event_id=?`, eventID).Scan(&title))
	require.Equal(t, "Renamed", title.String, "a stale projection reverted the user's rename")
	require.ErrorIs(t, applyErr, event.ErrStaleRevision)

	// The re-requested projection reads the rename and keeps it.
	_, err = eventService.RebuildOwner(ctx, 1, false)
	require.NoError(t, err)
	require.NoError(t, database.SQL.QueryRowContext(ctx,
		`SELECT title_override FROM events WHERE event_id=? AND status='active'`, eventID).Scan(&title))
	require.Equal(t, "Renamed", title.String)
}
