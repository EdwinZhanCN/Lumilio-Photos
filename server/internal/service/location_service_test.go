package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/settings"
	"server/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNormalizedGPSKeepsZeroCoordinates(t *testing.T) {
	lat := 0.0
	lng := 0.0

	gotLat, gotLng := normalizedGPS(&lat, &lng)

	require.NotNil(t, gotLat)
	require.NotNil(t, gotLng)
	require.Equal(t, 0.0, *gotLat)
	require.Equal(t, 0.0, *gotLng)
}

func TestNormalizedGPSRejectsInvalidCoordinates(t *testing.T) {
	lat := 91.0
	lng := 120.0

	gotLat, gotLng := normalizedGPS(&lat, &lng)

	require.Nil(t, gotLat)
	require.Nil(t, gotLng)
}

func TestGeohashesForGPSKeepsValidCoordinates(t *testing.T) {
	lat := 37.7749
	lng := -122.4194

	got5, got7 := geohashesForGPS(&lat, &lng)

	require.NotNil(t, got5)
	require.NotNil(t, got7)
	require.Equal(t, "9q8yy", *got5)
	require.Equal(t, "9q8yyk8", *got7)
}

func TestGeohashesForGPSRejectsInvalidCoordinates(t *testing.T) {
	lat := 91.0
	lng := 120.0
	normalizedLat, normalizedLng := normalizedGPS(&lat, &lng)

	got5, got7 := geohashesForGPS(normalizedLat, normalizedLng)

	require.Nil(t, got5)
	require.Nil(t, got7)
}

func TestReverseGeocoderDefaultsToDisabled(t *testing.T) {
	geocoder := newReverseGeocoder(settings.Geocoding{})

	require.Equal(t, geocoderProviderDisabled, geocoder.Provider())
	_, err := geocoder.Reverse(context.Background(), 0, 0)
	require.Error(t, err)
}

func TestNominatimGeocoderUsesMockEndpoint(t *testing.T) {
	var requested bool
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not permit loopback listeners: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		require.Equal(t, "Lumilio-Test/1.0", r.Header.Get("User-Agent"))
		require.Equal(t, "jsonv2", r.URL.Query().Get("format"))
		require.Equal(t, "en", r.URL.Query().Get("accept-language"))
		require.Equal(t, "0.00000000", r.URL.Query().Get("lat"))
		require.Equal(t, "0.00000000", r.URL.Query().Get("lon"))
		fmt.Fprint(w, `{"display_name":"Null Island","address":{"country":"Ocean","state":"Equator","city":"Prime Meridian"}}`)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	geocoder := newReverseGeocoder(settings.Geocoding{
		Provider:          "nominatim",
		NominatimEndpoint: server.URL,
		Language:          "en",
		UserAgent:         "Lumilio-Test/1.0",
	})
	result, err := geocoder.Reverse(context.Background(), 0, 0)

	require.NoError(t, err)
	require.True(t, requested)
	require.NotNil(t, result.Label)
	require.Equal(t, "Null Island", *result.Label)
	require.NotNil(t, result.Country)
	require.Equal(t, "Ocean", *result.Country)
	require.NotEmpty(t, result.RawResponse)
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	require.Equal(t, 30*time.Second, parseRetryAfter("30"))
	require.Equal(t, time.Hour, parseRetryAfter("9223372036"))
	require.Equal(t, 0*time.Second, parseRetryAfter("not-a-date"))
}

func TestProviderRetryDelay(t *testing.T) {
	t.Parallel()

	require.Equal(t, 5*time.Second, providerRetryDelay(1, 0))
	require.Equal(t, 10*time.Second, providerRetryDelay(2, 0))
	require.Equal(t, 5*time.Minute, providerRetryDelay(10, 0))
	require.Equal(t, time.Hour, providerRetryDelay(1, 2*time.Hour))
}

func TestLocationRevisionCheckPropagatesDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	require.NoError(t, err)
	require.NoError(t, catalog.Migrate(ctx))
	service := &locationService{queries: catalog.Queries}
	require.NoError(t, catalog.Close(ctx))

	_, err = service.revisionIsCurrent(ctx, 1)
	require.ErrorContains(t, err, "check geocoding revision")
}

func TestRebuildLocationClustersBulkPublishesTopology(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	require.NoError(t, err)
	defer catalog.Close(context.Background())
	require.NoError(t, catalog.Migrate(ctx))

	repositoryID := uuid.New()
	rootID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at,
			display_name, role, webauthn_user_handle
		) VALUES (1, 'owner', 'unused', 1, 1, 'Owner', 'admin', x'01');
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES (?, 'Root', '/test/root', 'default', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at,
			default_owner_id, role, root_id
		) VALUES (?, 'Repository', '/test/root/repository', 1, 1, 1, 'primary', ?);
	`, rootID, repositoryID, rootID)
	require.NoError(t, err)

	geohashes := []string{"9q8yyk8", "9q8yyk8", "9q8yyk9"}
	for index, geohash := range geohashes {
		assetID := uuid.New()
		_, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
			Filename: fmt.Sprintf("photo-%d.jpg", index), FileSize: 100,
		})
		require.NoError(t, err)
		_, err = catalog.SQL.ExecContext(ctx, `
			UPDATE assets
			SET gps_latitude = ?, gps_longitude = ?, gps_geohash_7 = ?
			WHERE asset_id = ?
		`, 37.77+float64(index)/100, -122.41, geohash, assetID)
		require.NoError(t, err)
	}

	repositoryText := repositoryID.String()
	ownerID := int32(1)
	locationService := NewLocationService(catalog.Queries, catalog.SQL, nil)
	moreWork, err := locationService.RebuildLocationClusters(ctx, &repositoryText, &ownerID)
	require.NoError(t, err)
	require.False(t, moreWork)

	rows, err := catalog.ReaderSQL.QueryContext(ctx, `
		SELECT cluster_id, photo_count
		FROM location_clusters
		ORDER BY geohash
	`)
	require.NoError(t, err)
	defer rows.Close()
	var counts []int
	for rows.Next() {
		var clusterID string
		var count int
		require.NoError(t, rows.Scan(&clusterID, &count))
		require.NoError(t, uuid.Validate(clusterID))
		counts = append(counts, count)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []int{2, 1}, counts)

	var memberships int
	require.NoError(t, catalog.ReaderSQL.QueryRowContext(ctx,
		"SELECT count(*) FROM location_cluster_assets",
	).Scan(&memberships))
	require.Equal(t, 3, memberships)
}

func TestLocationProjectionRevisionTracksSourceFacts(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	require.NoError(t, err)
	defer catalog.Close(context.Background())
	require.NoError(t, catalog.Migrate(ctx))

	repositoryID := uuid.New()
	rootID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at,
			display_name, role, webauthn_user_handle
		) VALUES (1, 'owner', 'unused', 1, 1, 'Owner', 'admin', x'01');
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES (?, 'Root', '/test/root', 'default', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at,
			default_owner_id, role, root_id
		) VALUES (?, 'Repository', '/test/root/repository', 1, 1, 1, 'primary', ?);
	`, rootID, repositoryID, rootID)
	require.NoError(t, err)

	assetID := uuid.New()
	_, err = testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
		AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
		Filename: "photo.jpg", FileSize: 100,
	})
	require.NoError(t, err)

	revision := func(ownerID int32) int64 {
		t.Helper()
		var value int64
		require.NoError(t, catalog.ReaderSQL.QueryRowContext(ctx, `
		SELECT source_revision
		FROM location_projection_state
		WHERE repository_id = ? AND owner_id = ?
	`, repositoryID, ownerID).Scan(&value))
		return value
	}

	before := revision(1)

	_, err = catalog.SQL.ExecContext(ctx, `
		UPDATE assets
		SET gps_latitude = 37.7749,
		    gps_longitude = -122.4194,
		    gps_geohash_7 = '9q8yyk8'
		WHERE asset_id = ?
	`, assetID)
	require.NoError(t, err)

	after := revision(1)
	require.Greater(t, after, before)

	// Metadata retries commonly write the same facts again. The IS NOT trigger
	// fence must keep those retries from manufacturing queue work.
	_, err = catalog.SQL.ExecContext(ctx, `
		UPDATE assets
		SET gps_latitude = 37.7749,
		    gps_longitude = -122.4194,
		    gps_geohash_7 = '9q8yyk8'
		WHERE asset_id = ?
	`, assetID)
	require.NoError(t, err)
	require.Equal(t, after, revision(1))

	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at,
			display_name, role, webauthn_user_handle
		) VALUES (2, 'second-owner', 'unused', 1, 1, 'Second Owner', 'user', x'02')
	`)
	require.NoError(t, err)
	_, err = catalog.SQL.ExecContext(ctx, "UPDATE assets SET owner_id = 2 WHERE asset_id = ?", assetID)
	require.NoError(t, err)
	require.Greater(t, revision(1), after, "the old owner scope must be invalidated")
	newOwnerRevision := revision(2)
	require.Greater(t, newOwnerRevision, int64(0), "the new owner scope must be created")

	_, err = catalog.SQL.ExecContext(ctx, `
		UPDATE asset_locations
		SET unbound_observation_revision = bound_observation_revision + 1
		WHERE asset_id = ?
	`, assetID)
	require.NoError(t, err)
	require.Greater(t, revision(2), newOwnerRevision, "unbinding a Location must invalidate its scope")

	oldOwnerBeforeNodeChange := revision(1)
	newOwnerBeforeNodeChange := revision(2)
	_, err = catalog.SQL.ExecContext(ctx, `
		UPDATE repository_nodes
		SET lifecycle = 'tombstoned'
		WHERE node_id IN (SELECT node_id FROM asset_locations WHERE asset_id = ?)
	`, assetID)
	require.NoError(t, err)
	require.Greater(t, revision(1), oldOwnerBeforeNodeChange)
	require.Greater(t, revision(2), newOwnerBeforeNodeChange)
}

func TestNextLocationMembershipBatchIsStrictlyBounded(t *testing.T) {
	t.Parallel()

	deletes := make([]uuid.UUID, 20)
	inserts := make([]uuid.UUID, 20)
	for index := range deletes {
		deletes[index] = uuid.New()
		inserts[index] = uuid.New()
	}

	deleteBatch, insertBatch := nextLocationMembershipBatch(deletes, inserts)
	require.Len(t, deleteBatch, maxLocationMembershipMutationsPerTransaction)
	require.Empty(t, insertBatch)
	require.LessOrEqual(t, len(deleteBatch)+len(insertBatch), maxLocationMembershipMutationsPerTransaction)

	deleteBatch, insertBatch = nextLocationMembershipBatch(deletes[:5], inserts)
	require.Len(t, deleteBatch, 5)
	require.Len(t, insertBatch, maxLocationMembershipMutationsPerTransaction-5)
	require.LessOrEqual(t, len(deleteBatch)+len(insertBatch), maxLocationMembershipMutationsPerTransaction)
}

func TestLocationRebuildYieldsAndRecoversFromNewSourceRevision(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	require.NoError(t, err)
	defer catalog.Close(context.Background())
	require.NoError(t, catalog.Migrate(ctx))

	repositoryID := uuid.New()
	rootID := uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at,
			display_name, role, webauthn_user_handle
		) VALUES (1, 'owner', 'unused', 1, 1, 'Owner', 'admin', x'01');
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES (?, 'Root', '/test/root', 'default', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at,
			default_owner_id, role, root_id
		) VALUES (?, 'Repository', '/test/root/repository', 1, 1, 1, 'primary', ?);
	`, rootID, repositoryID, rootID)
	require.NoError(t, err)

	seedTx, err := catalog.SQL.BeginTx(ctx, nil)
	require.NoError(t, err)
	for index := 0; index < maxLocationMembershipMutationsPerTransaction*maxLocationWriteTransactionsPerTurn+2; index++ {
		assetID := uuid.New()
		_, err := testutil.InsertAssetOccurrence(ctx, seedTx, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
			Filename: fmt.Sprintf("photo-%03d.jpg", index), FileSize: 100,
		})
		require.NoError(t, err)
		_, err = seedTx.ExecContext(ctx, `
			UPDATE assets
			SET gps_latitude = 37.7749,
			    gps_longitude = -122.4194,
			    gps_geohash_7 = '9q8yyk8'
			WHERE asset_id = ?
		`, assetID)
		require.NoError(t, err)
	}
	require.NoError(t, seedTx.Commit())

	repositoryText := repositoryID.String()
	ownerID := int32(1)
	locationService := NewLocationService(catalog.Queries, catalog.SQL, nil)
	moreWork, err := locationService.RebuildLocationClusters(ctx, &repositoryText, &ownerID)
	require.NoError(t, err)
	require.True(t, moreWork, "one River turn must yield after its fixed transaction quantum")

	var firstTurnMembers int
	require.NoError(t, catalog.ReaderSQL.QueryRowContext(ctx,
		"SELECT count(*) FROM location_cluster_assets",
	).Scan(&firstTurnMembers))
	require.Equal(t,
		maxLocationMembershipMutationsPerTransaction*maxLocationWriteTransactionsPerTurn,
		firstTurnMembers,
	)

	// Simulate a fact committed while the unique rebuild job is snoozed/running.
	// Its trigger advances source_revision; the same job must re-plan and include
	// it before published_revision can catch up.
	lateAssetID := uuid.New()
	_, err = testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{
		AssetID: lateAssetID, RepositoryID: repositoryID, OwnerID: 1,
		Filename: "late-photo.jpg", FileSize: 100,
	})
	require.NoError(t, err)
	_, err = catalog.SQL.ExecContext(ctx, `
		UPDATE assets
		SET gps_latitude = 37.7749,
		    gps_longitude = -122.4194,
		    gps_geohash_7 = '9q8yyk8'
		WHERE asset_id = ?
	`, lateAssetID)
	require.NoError(t, err)

	for turn := 0; turn < 4; turn++ {
		moreWork, err = locationService.RebuildLocationClusters(ctx, &repositoryText, &ownerID)
		require.NoError(t, err)
		if !moreWork {
			break
		}
	}
	require.False(t, moreWork)

	var finalMembers int
	require.NoError(t, catalog.ReaderSQL.QueryRowContext(ctx,
		"SELECT count(*) FROM location_cluster_assets",
	).Scan(&finalMembers))
	require.Equal(t, maxLocationMembershipMutationsPerTransaction*maxLocationWriteTransactionsPerTurn+3, finalMembers)

	var sourceRevision, publishedRevision int64
	require.NoError(t, catalog.ReaderSQL.QueryRowContext(ctx, `
		SELECT source_revision, published_revision
		FROM location_projection_state
		WHERE repository_id = ? AND owner_id = 1
	`, repositoryID).Scan(&sourceRevision, &publishedRevision))
	require.Equal(t, sourceRevision, publishedRevision)
}

func TestNominatimGeocoderRejectsOversizedResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not permit loopback listeners: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", maxReverseGeocodeBody+1))
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	geocoder := newReverseGeocoder(settings.Geocoding{
		Provider:          geocoderProviderNominatim,
		NominatimEndpoint: server.URL,
		Language:          "en",
		UserAgent:         settings.DefaultGeocodingUserAgent,
	})
	_, err = geocoder.Reverse(context.Background(), 1, 2)

	var providerErr *geocodeProviderError
	require.ErrorAs(t, err, &providerErr)
	require.False(t, providerErr.retryable)
}
