//go:build sqlite_fts5

package event_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/event"
)

func TestRebuildOwnerPublishesAndRetainsStableIdentity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "events.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle)
VALUES(1,'owner','hash',1,1,x'01');
INSERT INTO repository_roots(root_id,name,path,kind,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000001','root','/events','default',1,1);
INSERT INTO repositories(repo_id,name,path,reachability,activity,created_at,updated_at,default_owner_id,root_id)
VALUES('00000000-0000-0000-0000-000000000002','repo','/events/repo','active','idle',1,1,1,
       '00000000-0000-0000-0000-000000000001');
INSERT INTO assets(asset_id,owner_id,type,original_filename,mime_type,file_size,content_hash,
 upload_time,taken_time,repository_id,status,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000011',1,'PHOTO','a.jpg','image/jpeg',1,'a',
  1000000,1000000,'00000000-0000-0000-0000-000000000002','{"state":"completed"}',1),
 ('00000000-0000-0000-0000-000000000012',1,'PHOTO','b.jpg','image/jpeg',1,'b',
  2000000,2000000,'00000000-0000-0000-0000-000000000002','{"state":"completed"}',1);
INSERT INTO media_items(media_item_id,owner_id,repository_id,media_kind,primary_asset_id,created_at,updated_at)
VALUES
 ('00000000-0000-0000-0000-000000000021',1,'00000000-0000-0000-0000-000000000002','photo',
  '00000000-0000-0000-0000-000000000011',1,1),
 ('00000000-0000-0000-0000-000000000022',1,'00000000-0000-0000-0000-000000000002','photo',
  '00000000-0000-0000-0000-000000000012',1,1);
INSERT INTO media_item_assets(asset_id,media_item_id,relation,position,created_at)
VALUES
 ('00000000-0000-0000-0000-000000000011','00000000-0000-0000-0000-000000000021','original',0,1),
 ('00000000-0000-0000-0000-000000000012','00000000-0000-0000-0000-000000000022','original',0,1);
`); err != nil {
		t.Fatal(err)
	}

	service := event.NewService(database.SQL)
	first, err := service.RebuildOwner(ctx, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created != 1 || first.Members != 2 {
		t.Fatalf("first rebuild = %+v", first)
	}
	var eventID string
	if err := database.SQL.QueryRowContext(ctx, `SELECT event_id FROM events WHERE owner_id=1 AND status='active'`).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	second, err := service.RebuildOwner(ctx, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Retained != 1 || second.Created != 0 {
		t.Fatalf("second rebuild = %+v", second)
	}
	summary, err := service.Resolver().Resolve(ctx, 1, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EventID != eventID || summary.MediaCount != 2 || summary.DisplayableCount != 2 {
		t.Fatalf("resolved Event = %+v", summary)
	}
	if _, err := service.Resolver().Resolve(ctx, 2, eventID); err != event.ErrNotFound {
		t.Fatalf("cross-owner resolve = %v", err)
	}
	if _, err := service.Split(ctx, 1, eventID, "00000000-0000-0000-0000-000000000022"); err != nil {
		t.Fatalf("split Event: %v", err)
	}
	var splitID string
	if err := database.SQL.QueryRowContext(ctx, `
SELECT event_id FROM events WHERE owner_id=1 AND status='active' AND event_id<>?`, eventID).Scan(&splitID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Merge(ctx, 1, []string{eventID, splitID}, eventID); err != nil {
		t.Fatalf("merge Events: %v", err)
	}
	if _, err := service.RebuildOwner(ctx, 1, false); err != nil {
		t.Fatalf("rebuild split-then-merged Event: %v", err)
	}
	redirected, err := service.Resolver().Resolve(ctx, 1, splitID)
	if err != nil || redirected.EventID != eventID || redirected.RedirectedFrom != splitID {
		t.Fatalf("redirect resolution = %+v, %v", redirected, err)
	}
	if _, err := service.RemoveMember(ctx, 1, eventID, "00000000-0000-0000-0000-000000000022"); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if _, err := service.RemoveMember(ctx, 1, eventID, "00000000-0000-0000-0000-000000000021"); err != event.ErrWouldBeEmpty {
		t.Fatalf("last-member removal = %v", err)
	}
}
