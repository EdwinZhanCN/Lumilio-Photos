package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	migrations "server/migrations"

	"github.com/google/uuid"
)

var insertColumnsPattern = regexp.MustCompile(`(?is)INSERT(?:\s+OR\s+\w+)?\s+INTO\s+([a-z_][a-z0-9_]*)\s*\(([^)]*)\)`)

func TestOpenMigrateAndReopenSQLiteCatalog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dir := secureTempDir(t)
	path := filepath.Join(dir, "library.sqlite3")

	first, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}

	var foreignKeys int
	var journalMode, synchronous string
	var busyTimeout int
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if foreignKeys != 1 || !strings.EqualFold(journalMode, "wal") || synchronous != "1" || busyTimeout != 5000 {
		t.Fatalf(
			"unexpected pragmas: foreign_keys=%d journal_mode=%s synchronous=%s busy_timeout=%d",
			foreignKeys,
			journalMode,
			synchronous,
			busyTimeout,
		)
	}
	if first.SQL.Stats().MaxOpenConnections != 1 {
		t.Fatalf("max open connections = %d, want 1", first.SQL.Stats().MaxOpenConnections)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatalf("close first catalog: %v", err)
	}

	reopened, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("reopen catalog: %v", err)
	}
	defer reopened.Close(context.Background())
	if err := reopened.Migrate(ctx); err != nil {
		t.Fatalf("migrate reopened catalog: %v", err)
	}
	info, err := InspectCatalog(ctx, path)
	if err != nil {
		t.Fatalf("inspect reopened catalog: %v", err)
	}
	if info.ApplicationMigration != 6 || info.RiverMigration == 0 || info.LibraryID == "" {
		t.Fatalf("unexpected catalog identity: %+v", info)
	}
}

func TestParseVec1VersionIsPortableAcrossCPUBuilds(t *testing.T) {
	for _, info := range []string{
		"version 0.7 (NEON, multi-threaded)",
		"version 0.7 (AVX2, multi-threaded)",
		"version 0.7 (scalar, single-threaded)",
	} {
		version, err := parseVec1Version(info)
		if err != nil {
			t.Fatal(err)
		}
		if version != "0.7" {
			t.Fatalf("parseVec1Version(%q) = %q, want 0.7", info, version)
		}
	}
}

func TestConnectionPolicySurvivesPhysicalConnectionReplacement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "replacement.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())

	connection, err := database.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rawErr := connection.Raw(func(any) error {
		return driver.ErrBadConn
	})
	if !errors.Is(rawErr, driver.ErrBadConn) {
		_ = connection.Close()
		t.Fatalf("discard physical connection: %v", rawErr)
	}
	if err := connection.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		want int
	}{
		{name: "foreign_keys", want: 1},
		{name: "busy_timeout", want: 5000},
		{name: "temp_store", want: 2},
		{name: "wal_autocheckpoint", want: 1000},
	}
	for _, check := range checks {
		var got int
		if err := database.SQL.QueryRowContext(ctx, "PRAGMA "+check.name).Scan(&got); err != nil {
			t.Fatalf("read replacement PRAGMA %s: %v", check.name, err)
		}
		if got != check.want {
			t.Fatalf("replacement PRAGMA %s = %d, want %d", check.name, got, check.want)
		}
	}
}

func TestMigrationLedgerRejectsHistoricalChecksumChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "migration-checksum.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		UPDATE lumilio_schema_migrations SET checksum = ? WHERE version = 3
	`, strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	err = database.Migrate(ctx)
	if err == nil || !strings.Contains(err.Error(), "historical migrations are immutable") {
		t.Fatalf("tampered migration ledger error = %v", err)
	}
}

func TestMigrationRejectsIncompatibleSchemaGeneration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "schema-generation.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	var generation int
	if err := database.SQL.QueryRowContext(ctx, "PRAGMA user_version").Scan(&generation); err != nil {
		t.Fatal(err)
	}
	if generation != schemaGeneration {
		t.Fatalf("migrated catalog user_version = %d, want %d", generation, schemaGeneration)
	}

	if _, err := database.SQL.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	err = database.Migrate(ctx)
	if err == nil || !strings.Contains(err.Error(), "incompatible experimental schema generation") {
		t.Fatalf("stale-generation catalog error = %v", err)
	}
}

func TestOpenRejectsOldGenerationBeforeDerivedModuleChecks(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(secureTempDir(t), "old-generation.sqlite3")
	catalog, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `
		CREATE TABLE assets (id INTEGER);
		CREATE TABLE media_items (id INTEGER);
		CREATE TABLE asset_stacks (id INTEGER);
		PRAGMA user_version = 3;
	`); err != nil {
		_ = catalog.Close(ctx)
		t.Fatal(err)
	}
	if err := catalog.Close(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = Open(ctx, config.DatabaseConfig{Path: path})
	if err == nil || !strings.Contains(err.Error(), "incompatible experimental schema generation") {
		t.Fatalf("old-generation open error = %v", err)
	}
}

func TestBioAlbumSchemaAndQueryLiteralsShareDomainValue(t *testing.T) {
	baseline, err := migrations.FS.ReadFile("000003_vec1_baseline.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	checkFragment := "'default', 'smart', '" + repo.AlbumTypeBio + "'"
	if !strings.Contains(string(baseline), checkFragment) {
		t.Fatalf("album_type CHECK does not include shared bio value %q", repo.AlbumTypeBio)
	}
	indexingSQL, err := os.ReadFile(filepath.Join("repo", "queries", "indexing.sql"))
	if err != nil {
		t.Fatal(err)
	}
	queryFragment := "album_type = '" + repo.AlbumTypeBio + "'"
	if count := strings.Count(string(indexingSQL), queryFragment); count != 2 {
		t.Fatalf("bio indexing query literal count = %d, want 2", count)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "bio-album.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username:           "bio-schema",
		Password:           "not-used",
		DisplayName:        "Bio Schema",
		Role:               "admin",
		WebauthnUserHandle: []byte("bio-schema-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Queries.CreateAlbum(ctx, repo.CreateAlbumParams{
		UserID:    user.UserID,
		AlbumName: "Bio",
		AlbumType: repo.AlbumTypeBio,
	}); err != nil {
		t.Fatalf("create bio album: %v", err)
	}
	if _, err := database.Queries.CountBioAlbumPhotoAssets(ctx, nil); err != nil {
		t.Fatalf("execute bio album indexing query: %v", err)
	}
}

func TestOpenRejectsEphemeralAndCorruptCatalogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, path := range []string{"", ":memory:"} {
		if _, err := Open(ctx, config.DatabaseConfig{Path: path}); err == nil {
			t.Fatalf("Open(%q) unexpectedly succeeded", path)
		}
	}

	dir := secureTempDir(t)
	path := filepath.Join(dir, "library.sqlite3")
	if err := os.WriteFile(path, []byte("not a sqlite catalog"), 0o600); err != nil {
		t.Fatalf("write corrupt catalog: %v", err)
	}
	if _, err := Open(ctx, config.DatabaseConfig{Path: path}); err == nil {
		t.Fatal("open corrupt catalog unexpectedly succeeded")
	}
}

func TestGeneratedSQLiteQueriesExecuteJSONFiltersAndNullMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	path := filepath.Join(secureTempDir(t), "queries.sqlite3")
	database, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("close catalog: %v", err)
		}
	})
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate catalog: %v", err)
	}

	user, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username:           "query-test",
		Password:           "not-used",
		DisplayName:        "Query Test",
		Role:               "admin",
		WebauthnUserHandle: []byte("query-test-handle"),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	album, err := database.Queries.CreateAlbum(ctx, repo.CreateAlbumParams{
		UserID:    user.UserID,
		AlbumName: "SQLite",
		AlbumType: repo.AlbumTypeDefault,
	})
	if err != nil {
		t.Fatalf("create album: %v", err)
	}
	if album.CreatedAt.Time.IsZero() || album.UpdatedAt.Time.IsZero() {
		t.Fatalf("album timestamps were not populated: %+v", album)
	}

	repositoryID := uuid.New()
	rootID := uuid.New()
	assetID := uuid.New()
	mediaItemID := uuid.New()
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO repository_roots (root_id, name, path, kind, created_at, updated_at)
		VALUES (?, 'Test root', '/', 'external', 1, 1)
	`, rootID); err != nil {
		t.Fatalf("insert repository root: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO repositories (
			repo_id, name, path, role, reachability, activity, created_at, updated_at, root_id
		) VALUES (?, 'Test', '/test', 'regular', 'active', 'idle', 1, 1, ?)
	`, repositoryID, rootID); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO assets (
			asset_id, owner_id, type, original_filename, mime_type, file_size,
			content_hash, upload_time, repository_id, updated_at,
			gps_latitude, gps_longitude, gps_geohash_7
		) VALUES (
			?, ?, 'PHOTO', 'IMG_0001.jpg', 'image/jpeg', 1, 'hash', 1, ?, 1,
			40.7128, -74.0060, 'dr5regw'
		)
	`, assetID, user.UserID, repositoryID); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO media_items (
			media_item_id, repository_id, media_kind, primary_asset_id, created_at, updated_at
		) VALUES (?, ?, 'photo', ?, 1, 1)
	`, mediaItemID, repositoryID, assetID); err != nil {
		t.Fatalf("insert media item: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO media_item_assets (asset_id, media_item_id, relation, created_at)
		VALUES (?, ?, 'original', 1)
	`, assetID, mediaItemID); err != nil {
		t.Fatalf("attach asset: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO tags (tag_name, is_ai_generated) VALUES ('favorite', 0)
	`); err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO asset_tags (asset_id, tag_id, confidence, source)
		SELECT ?, tag_id, 1, 'user' FROM tags WHERE tag_name = 'favorite'
	`, assetID); err != nil {
		t.Fatalf("tag asset: %v", err)
	}

	count, err := database.Queries.CountMediaItemsUnified(ctx, repo.CountMediaItemsUnifiedParams{
		IsDeleted:    false,
		RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
		AssetIds:     dbtypes.UUIDsJSONParam([]uuid.UUID{assetID}),
		AssetTypes:   dbtypes.StringsJSONParam([]string{"PHOTO"}),
		TagNames:     dbtypes.StringsJSONParam([]string{"favorite"}),
	})
	if err != nil {
		t.Fatalf("count media items with JSON filters: %v", err)
	}
	if count != 1 {
		t.Fatalf("filtered media item count = %d, want 1", count)
	}

	candidates, err := database.Queries.FindCandidatesForStackingByName(
		ctx,
		uuid.NullUUID{UUID: repositoryID, Valid: true},
	)
	if err != nil {
		t.Fatalf("find stacking candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].IsRaw != 0 {
		t.Fatalf("stacking candidates = %+v, want one non-RAW candidate", candidates)
	}

	for name, hashes := range map[string][]string{
		"empty": nil,
		"many":  {"hash", "missing"},
	} {
		rows, err := database.Queries.GetAssetsByContentHashesAndRepository(
			ctx,
			repo.GetAssetsByContentHashesAndRepositoryParams{
				ContentHashes: dbtypes.StringsJSONParam(hashes),
				RepositoryID:  uuid.NullUUID{UUID: repositoryID, Valid: true},
			},
		)
		if err != nil {
			t.Fatalf("get assets by %s content hash list: %v", name, err)
		}
		want := 0
		if name == "many" {
			want = 1
		}
		if len(rows) != want {
			t.Fatalf("assets for %s content hash list = %d, want %d", name, len(rows), want)
		}
	}
	if rows, err := database.Queries.GetAssetsByQuickFingerprintsAndRepository(
		ctx,
		repo.GetAssetsByQuickFingerprintsAndRepositoryParams{
			QuickFingerprints: dbtypes.StringsJSONParam(nil),
			RepositoryID:      uuid.NullUUID{UUID: repositoryID, Valid: true},
		},
	); err != nil || len(rows) != 0 {
		t.Fatalf("assets for empty quick fingerprint list = %d, %v; want 0, nil", len(rows), err)
	}

	assetIDsJSON := dbtypes.UUIDsJSONParam([]uuid.UUID{assetID, uuid.New()})
	authorized, err := database.Queries.GetAuthorizedAssetIDs(
		ctx,
		repo.GetAuthorizedAssetIDsParams{OwnerID: &user.UserID, AssetIds: assetIDsJSON},
	)
	if err != nil || len(authorized) != 1 || authorized[0] != assetID {
		t.Fatalf("authorized assets = %v, %v; want [%s], nil", authorized, err, assetID)
	}
	if err := database.Queries.BulkUpdateAssetLiked(ctx, repo.BulkUpdateAssetLikedParams{
		Liked: true, AssetIds: assetIDsJSON,
	}); err != nil {
		t.Fatalf("bulk like assets: %v", err)
	}
	liked, err := database.Queries.GetAssetByID(ctx, assetID)
	if err != nil || !liked.Liked {
		t.Fatalf("liked asset = %+v, %v; want liked", liked, err)
	}
	sorted, err := database.Queries.GetAssetsByOwnerAndTypesSorted(
		ctx,
		repo.GetAssetsByOwnerAndTypesSortedParams{
			OwnerID: &user.UserID,
			Types:   dbtypes.StringsJSONParam([]string{"PHOTO", "VIDEO"}),
			Limit:   10,
		},
	)
	if err != nil || len(sorted) != 1 || sorted[0].AssetID != assetID {
		t.Fatalf("owner/type sorted assets = %v, %v; want [%s], nil", sorted, err, assetID)
	}
	if _, err := database.Queries.AgentFacetTopLenses(ctx, repo.AgentFacetTopLensesParams{
		AssetIds: assetIDsJSON,
		TopN:     5,
	}); err != nil {
		t.Fatalf("query asset facet with JSON IDs and fixed limit: %v", err)
	}
	locationCandidates, err := database.Queries.ListLocationClusterCandidatesForScope(
		ctx,
		repo.ListLocationClusterCandidatesForScopeParams{
			RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
			OwnerID:      &user.UserID,
		},
	)
	if err != nil {
		t.Fatalf("list location cluster candidates: %v", err)
	}
	if len(locationCandidates) != 1 || !locationCandidates[0].RepositoryID.Valid ||
		locationCandidates[0].Geohash == nil || locationCandidates[0].CentroidLatitude == nil ||
		locationCandidates[0].CentroidLongitude == nil {
		t.Fatalf("location cluster candidates = %+v, want one complete aggregate", locationCandidates)
	}
	clusterID := uuid.New()
	cluster, err := database.Queries.CreateLocationCluster(ctx, repo.CreateLocationClusterParams{
		ClusterID:         clusterID,
		OwnerID:           locationCandidates[0].OwnerID,
		RepositoryID:      locationCandidates[0].RepositoryID.UUID,
		Geohash:           *locationCandidates[0].Geohash,
		Precision:         7,
		CentroidLatitude:  *locationCandidates[0].CentroidLatitude,
		CentroidLongitude: *locationCandidates[0].CentroidLongitude,
		PhotoCount:        locationCandidates[0].PhotoCount,
		CreatedAt:         dbtypes.NewTimestamp(time.Now().UTC()),
		UpdatedAt:         dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatalf("create location cluster: %v", err)
	}
	if cluster.ClusterID != clusterID || cluster.RepositoryID != repositoryID || cluster.PhotoCount != 1 {
		t.Fatalf("location cluster = %+v, want populated Go UUID cluster", cluster)
	}
	if err := database.Queries.InsertLocationClusterAssetsForScope(
		ctx,
		repo.InsertLocationClusterAssetsForScopeParams{
			RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
			OwnerID:      &user.UserID,
		},
	); err != nil {
		t.Fatalf("insert location cluster memberships: %v", err)
	}
	var locationMemberships int
	if err := database.SQL.QueryRowContext(
		ctx,
		"SELECT count(*) FROM location_cluster_assets WHERE cluster_id = ? AND asset_id = ?",
		cluster.ClusterID,
		assetID,
	).Scan(&locationMemberships); err != nil {
		t.Fatalf("count location cluster memberships: %v", err)
	}
	if locationMemberships != 1 {
		t.Fatalf("location cluster memberships = %d, want 1", locationMemberships)
	}
	if _, err := database.Queries.GetAssetIDsByPersonIDs(ctx, repo.GetAssetIDsByPersonIDsParams{
		UserID:    &user.UserID,
		Limit:     5,
		PersonIds: dbtypes.IntegersJSONParam([]int32{1, 2}),
	}); err != nil {
		t.Fatalf("query person IDs with JSON list and fixed parameters: %v", err)
	}
	if err := database.Queries.RemoveAssetTagsBySources(ctx, repo.RemoveAssetTagsBySourcesParams{
		AssetID: assetID,
		Sources: dbtypes.StringsJSONParam([]string{"ai", "user"}),
	}); err != nil {
		t.Fatalf("remove asset tags by JSON sources: %v", err)
	}
}

func TestSourceInsertsPopulateRequiredSQLiteColumns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	path := filepath.Join(secureTempDir(t), "insert-contracts.sqlite3")
	database, err := Open(ctx, config.DatabaseConfig{Path: path})
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("close catalog: %v", err)
		}
	})
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate catalog: %v", err)
	}

	queryFiles, err := filepath.Glob(filepath.Join("repo", "queries", "*.sql"))
	if err != nil {
		t.Fatalf("find source queries: %v", err)
	}
	var violations []string
	for _, queryFile := range queryFiles {
		querySQL, err := os.ReadFile(queryFile)
		if err != nil {
			t.Fatalf("read %s: %v", queryFile, err)
		}
		for _, match := range insertColumnsPattern.FindAllStringSubmatch(string(querySQL), -1) {
			table := strings.ToLower(match[1])
			inserted := make(map[string]struct{})
			for column := range strings.SplitSeq(match[2], ",") {
				name := strings.ToLower(strings.Trim(strings.TrimSpace(column), "`\"[]"))
				inserted[name] = struct{}{}
			}

			required, err := requiredInsertColumns(ctx, database.SQL, table)
			if err != nil {
				t.Fatalf("inspect required columns for %s: %v", table, err)
			}
			for _, column := range required {
				if _, ok := inserted[column]; !ok {
					violations = append(
						violations,
						fmt.Sprintf("%s: INSERT INTO %s omits required column %s", queryFile, table, column),
					)
				}
			}
		}
	}
	sort.Strings(violations)
	for _, violation := range violations {
		t.Error(violation)
	}
}

func requiredInsertColumns(ctx context.Context, database *sql.DB, table string) ([]string, error) {
	rows, err := database.QueryContext(ctx, `PRAGMA table_info("`+table+`")`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var required []string
	for rows.Next() {
		var (
			columnID    int
			name        string
			columnType  string
			notNull     int
			defaultExpr sql.NullString
			primaryKey  int
		)
		if err := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultExpr, &primaryKey); err != nil {
			return nil, err
		}
		isImplicitRowID := primaryKey != 0 && strings.EqualFold(columnType, "INTEGER")
		if notNull != 0 && !defaultExpr.Valid && !isImplicitRowID {
			required = append(required, strings.ToLower(name))
		}
	}
	return required, rows.Err()
}

func secureTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("secure temp directory: %v", err)
	}
	return dir
}
