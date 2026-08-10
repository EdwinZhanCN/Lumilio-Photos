//go:build sqlite_fts5

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/api/dto"
	"server/internal/db"
	"server/internal/event"
	"server/internal/service"

	"github.com/gin-gonic/gin"
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
INSERT INTO repository_roots(root_id,name,path,kind,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000001','root','/events','default',1,1);
INSERT INTO repositories(repo_id,name,path,reachability,activity,created_at,updated_at,default_owner_id,root_id)
VALUES
 ('00000000-0000-0000-0000-000000000002','first','/events/first','active','idle',1,1,1,
  '00000000-0000-0000-0000-000000000001'),
 ('00000000-0000-0000-0000-000000000003','second','/events/second','active','idle',1,1,1,
  '00000000-0000-0000-0000-000000000001');
INSERT INTO assets(asset_id,owner_id,type,original_filename,mime_type,file_size,content_hash,
 upload_time,taken_time,repository_id,status,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000011',1,'PHOTO','first.jpg','image/jpeg',1,'first',
  1000000,1000000,'00000000-0000-0000-0000-000000000002','{"state":"completed"}',1),
 ('00000000-0000-0000-0000-000000000012',1,'PHOTO','second.jpg','image/jpeg',1,'second',
  2000000,2000000,'00000000-0000-0000-0000-000000000003','{"state":"completed"}',1),
 ('00000000-0000-0000-0000-000000000013',1,'PHOTO','second-only.jpg','image/jpeg',1,'second-only',
  3000000,3000000,'00000000-0000-0000-0000-000000000003','{"state":"completed"}',1);
INSERT INTO media_items(media_item_id,owner_id,repository_id,media_kind,primary_asset_id,created_at,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000021',1,'00000000-0000-0000-0000-000000000002','photo',
  '00000000-0000-0000-0000-000000000011',1,1),
 ('00000000-0000-0000-0000-000000000022',1,'00000000-0000-0000-0000-000000000003','photo',
  '00000000-0000-0000-0000-000000000012',1,1),
 ('00000000-0000-0000-0000-000000000023',1,'00000000-0000-0000-0000-000000000003','photo',
  '00000000-0000-0000-0000-000000000013',1,1);
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
