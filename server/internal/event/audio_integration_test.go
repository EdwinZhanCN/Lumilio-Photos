//go:build sqlite_fts5

package event_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/event"
	"server/internal/testutil"

	"github.com/google/uuid"
)

type eventMediaFixture struct {
	assetID, mediaID, repositoryID string
}

func audioEventCatalog(t *testing.T) (*db.DB, func(string, string, *int64, int64) eventMediaFixture) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "events.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close(ctx) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle)
VALUES(1,'owner','hash',1,1,x'01');
INSERT INTO storage_locations(storage_location_id,name,path,kind,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000004','/events','default',1,1);
INSERT INTO repositories(repo_id,name,path,reachability,activity,created_at,updated_at,default_owner_id,storage_location_id)
VALUES('00000000-0000-0000-0000-000000000001','one','/events/one','active','idle',1,1,1,'00000000-0000-0000-0000-000000000004'),
('00000000-0000-0000-0000-000000000002','two','/events/two','active','idle',1,1,1,'00000000-0000-0000-0000-000000000004'),
('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000006','/events/audio','active','idle',1,1,1,'00000000-0000-0000-0000-000000000004');
`); err != nil {
		t.Fatal(err)
	}
	return database, func(kind, repositoryID string, taken *int64, uploaded int64) eventMediaFixture {
		t.Helper()
		assetID, mediaID := uuid.NewString(), uuid.NewString()
		assetType := map[string]string{"photo": "PHOTO", "video": "VIDEO", "audio": "AUDIO"}[kind]
		if _, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: uuid.MustParse(assetID), RepositoryID: uuid.MustParse(repositoryID), OwnerID: 1,
			AssetType: assetType, UploadTime: uploaded, TakenTime: taken,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO media_items(media_item_id,owner_id,repository_id,media_kind,primary_asset_id,created_at,updated_at)
VALUES(?,1,?,?,?,1,1);
INSERT INTO media_item_assets(asset_id,media_item_id,relation,position,created_at)
VALUES(?,?,'original',0,1)`, mediaID, repositoryID, kind, assetID, assetID, mediaID); err != nil {
			t.Fatal(err)
		}
		return eventMediaFixture{assetID, mediaID, repositoryID}
	}
}

const eventRepoOne = "00000000-0000-0000-0000-000000000001"
const eventRepoTwo = "00000000-0000-0000-0000-000000000002"
const eventRepoAudio = "00000000-0000-0000-0000-000000000003"

func TestAudioNeverSeedsOrBridgesEvents(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		taken       *int64
		designation string
	}{
		{"imported music", nil, "music"},
		{"recording without capture time", nil, "other"},
		{"recording with capture time", ptrMicros(time.Hour), "other"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			database, add := audioEventCatalog(t)
			// Visuals are far enough apart to split; an audio every hour would bridge them.
			add("photo", eventRepoOne, ptrMicros(0), 1)
			add("video", eventRepoTwo, ptrMicros(4*time.Hour), 1)
			service := event.NewService(database.SQL)
			baseline, err := service.RebuildOwner(ctx, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if baseline.Events != 2 {
				t.Fatalf("baseline = %+v", baseline)
			}
			for hour := 1; hour <= 3; hour++ {
				taken := scenario.taken
				if taken != nil {
					taken = ptrMicros(time.Duration(hour) * time.Hour)
				}
				audio := add("audio", eventRepoAudio, taken, int64(time.Duration(hour)*time.Hour/time.Microsecond))
				var designation string
				if err := database.SQL.QueryRowContext(ctx, "SELECT designation FROM music_tracks WHERE track_id=?", audio.assetID).Scan(&designation); err != nil || designation != "music" {
					t.Fatalf("unclassified audio default = %q, %v", designation, err)
				}
				if _, err := database.SQL.ExecContext(ctx, "UPDATE music_tracks SET designation=? WHERE track_id=?", scenario.designation, audio.assetID); err != nil {
					t.Fatal(err)
				}
				// Geography is irrelevant even when an audio recording has trustworthy facts.
				if _, err := database.SQL.ExecContext(ctx, "UPDATE assets SET gps_latitude=45,gps_longitude=120 WHERE asset_id=?", audio.assetID); err != nil {
					t.Fatal(err)
				}
			}
			for rebuild := 0; rebuild < 2; rebuild++ {
				result, err := service.RebuildOwner(ctx, 1, false)
				if err != nil {
					t.Fatal(err)
				}
				if result.Events != 2 || result.Members != 2 || result.Retained != 2 {
					t.Fatalf("audio changed visual topology: %+v", result)
				}
			}
		})
	}
}

func ptrMicros(value time.Duration) *int64 { micros := int64(value / time.Microsecond); return &micros }

func TestAudioOnlyProducesNoEvents(t *testing.T) {
	ctx := context.Background()
	database, add := audioEventCatalog(t)
	add("audio", eventRepoAudio, nil, 1)
	add("audio", eventRepoAudio, ptrMicros(time.Hour), 1)
	service := event.NewService(database.SQL)
	result, err := service.RebuildOwner(ctx, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.SQL.QueryRowContext(ctx, "SELECT count(*) FROM events WHERE status='active'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if result.Events != 0 || result.Members != 0 || count != 0 {
		t.Fatalf("audio-only: %+v, active=%d", result, count)
	}
}

func TestMixedEventResolutionAndRebuildExcludeAudio(t *testing.T) {
	ctx := context.Background()
	database, add := audioEventCatalog(t)
	photo := add("photo", eventRepoOne, ptrMicros(time.Second), 1)
	video := add("video", eventRepoTwo, ptrMicros(2*time.Second), 1)
	service := event.NewService(database.SQL)
	if _, err := service.RebuildOwner(ctx, 1, false); err != nil {
		t.Fatal(err)
	}
	var eventID string
	if err := database.SQL.QueryRowContext(ctx, "SELECT event_id FROM events WHERE status='active'").Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	audio := add("audio", eventRepoAudio, nil, 1)
	pair := []string{photo.mediaID, audio.mediaID}
	slices.Sort(pair)
	// Simulate an old mixed Event with an audio cover and corrections on both kinds.
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO event_media_items(event_id,owner_id,media_item_id,position,source,evidence,created_at)
VALUES(?,1,?,2,'user','{}',1);
UPDATE events SET cover_override_media_item_id=?,title_override='My trip',is_hidden=1 WHERE event_id=?;
INSERT INTO event_constraints(constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000005',1,'include',?,?,1,1),('00000000-0000-0000-0000-000000000006',1,'include',?,?,1,1);
INSERT INTO event_constraints(constraint_id,owner_id,kind,left_media_item_id,right_media_item_id,created_at,updated_at)
VALUES('00000000-0000-0000-0000-000000000007',1,'must_link',?,?,1,1);
`, eventID, audio.mediaID, audio.mediaID, eventID, eventID, photo.mediaID, eventID, audio.mediaID, pair[0], pair[1]); err != nil {
		t.Fatal(err)
	}

	summary, err := service.Resolver().Resolve(ctx, 1, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MediaCount != 2 || summary.DisplayableCount != 2 || summary.CanonicalMediaCount != 2 || summary.CoverOverrideMediaItem != nil || summary.CoverAssetID == nil || *summary.CoverAssetID == audio.assetID {
		t.Fatalf("mixed summary = %+v", summary)
	}
	assets, total, err := service.Resolver().OrderedAssets(ctx, 1, eventID, 0)
	if err != nil || total != 2 || len(assets) != 2 {
		t.Fatalf("ordered assets = %+v, %d, %v", assets, total, err)
	}
	for _, asset := range assets {
		if asset.AssetID == audio.assetID {
			t.Fatal("audio in browsing/share snapshot")
		}
	}
	// Transactional share projection uses the same membership policy.
	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	shared, shareCount, err := event.OrderedAssetsTx(ctx, tx, 1, eventID, 0)
	tx.Rollback()
	if err != nil || shareCount != 2 || len(shared) != 2 {
		t.Fatalf("share projection = %+v, %d, %v", shared, shareCount, err)
	}

	for _, visual := range []eventMediaFixture{photo, video} {
		projected, err := service.Resolver().ProjectToRepository(ctx, 1, summary, visual.repositoryID)
		if err != nil || projected.MediaCount != 1 || projected.DisplayableCount != 1 || projected.CanonicalMediaCount != 2 || projected.CoverAssetID == nil || *projected.CoverAssetID != visual.assetID {
			t.Fatalf("repository summary = %+v, %v", projected, err)
		}
		scoped, total, err := service.Resolver().OrderedAssetsForRepository(ctx, 1, eventID, visual.repositoryID, 0)
		if err != nil || total != 1 || len(scoped) != 1 || scoped[0].AssetID != visual.assetID {
			t.Fatalf("repository browse = %+v, %d, %v", scoped, total, err)
		}
	}
	if _, err := service.Resolver().ProjectToRepository(ctx, 1, summary, eventRepoAudio); !errors.Is(err, event.ErrNotFound) {
		t.Fatalf("audio repository = %v", err)
	}
	if _, err := service.AddAssets(ctx, 1, eventID, []string{audio.assetID}); !errors.Is(err, event.ErrNotFound) {
		t.Fatalf("manual audio add = %v", err)
	}

	for rebuild := 0; rebuild < 2; rebuild++ {
		result, err := service.RebuildOwner(ctx, 1, false)
		if err != nil || result.Members != 2 || result.Retained != 1 {
			t.Fatalf("rebuild = %+v, %v", result, err)
		}
	}
	summary, err = service.Resolver().Resolve(ctx, 1, eventID)
	if err != nil || summary.TitleOverride == nil || *summary.TitleOverride != "My trip" || !summary.Hidden {
		t.Fatalf("visual Event edits lost: %+v, %v", summary, err)
	}
	var constraints, members int
	var cover sql.NullString
	if err := database.SQL.QueryRowContext(ctx, "SELECT count(*) FROM event_constraints").Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT count(*) FROM event_media_items WHERE event_id=?", eventID).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT cover_override_media_item_id FROM events WHERE event_id=?", eventID).Scan(&cover); err != nil {
		t.Fatal(err)
	}
	if constraints != 1 || members != 2 || cover.Valid {
		t.Fatalf("stale state: constraints=%d members=%d cover=%v", constraints, members, cover)
	}
	var visualCorrection string
	if err := database.SQL.QueryRowContext(ctx, "SELECT left_media_item_id FROM event_constraints WHERE constraint_id='00000000-0000-0000-0000-000000000005'").Scan(&visualCorrection); err != nil || visualCorrection != photo.mediaID {
		t.Fatalf("visual correction lost: %q, %v", visualCorrection, err)
	}
}

func TestStaleAudioOnlyEventRetiresAndCleansConstraints(t *testing.T) {
	ctx := context.Background()
	database, add := audioEventCatalog(t)
	audio := add("audio", eventRepoAudio, nil, 1)
	eventID := uuid.NewString()
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO events(event_id,owner_id,status,start_at,end_at,algorithm_version,created_at,updated_at)
VALUES(?,1,'active',1,1,'events-v1',1,1);
INSERT INTO event_media_items(event_id,owner_id,media_item_id,position,source,evidence,created_at)
VALUES(?,1,?,0,'user','{}',1);
UPDATE events SET generated_cover_media_item_id=? WHERE event_id=?;
INSERT INTO event_constraints(constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at)
VALUES(?,1,'include',?,?,1,1);
`, eventID, eventID, audio.mediaID, audio.mediaID, eventID, uuid.NewString(), eventID, audio.mediaID); err != nil {
		t.Fatal(err)
	}
	service := event.NewService(database.SQL)
	if _, err := service.Resolver().Resolve(ctx, 1, eventID); !errors.Is(err, event.ErrNotFound) {
		t.Fatalf("audio-only resolve = %v", err)
	}
	assets, total, err := service.Resolver().OrderedAssets(ctx, 1, eventID, 0)
	if err != nil || total != 0 || len(assets) != 0 {
		t.Fatalf("audio-only browse = %+v, %d, %v", assets, total, err)
	}
	if _, err := service.RebuildOwner(ctx, 1, false); err != nil {
		t.Fatal(err)
	}
	var status string
	var constraints, members int
	if err := database.SQL.QueryRowContext(ctx, "SELECT status FROM events WHERE event_id=?", eventID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT count(*) FROM event_constraints").Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, "SELECT count(*) FROM event_media_items").Scan(&members); err != nil {
		t.Fatal(err)
	}
	if status != "retired" || constraints != 0 || members != 0 {
		t.Fatalf("audio-only cleanup: status=%s constraints=%d members=%d", status, constraints, members)
	}
}
