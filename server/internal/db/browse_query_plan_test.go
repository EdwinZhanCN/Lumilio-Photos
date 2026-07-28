package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// Query-plan guards for the browse read path.
//
// These are not benchmarks: they assert *access paths*, which are deterministic
// and machine-independent, instead of wall-clock numbers, which on a developer
// laptop are dominated by noise. What they catch is the failure mode that
// actually breaks a large library — a join degrading from an index lookup to a
// full table scan, turning a page render from O(page) into O(catalog).
//
// Scope note: whether the *driving* table (media_items) is scanned is a
// legitimate planner decision that depends on selectivity — browsing a whole
// repository really does read most rows. So these guards deliberately assert
// only on the child-table joins, whose access path must stay indexed no matter
// what the filter selects.

// planScaleItems is large enough for ANALYZE to produce meaningful statistics
// while keeping the fixture build well under a second.
const planScaleItems = 2000

// seedBrowseScaleCatalog writes a realistically *distributed* catalog. The
// distribution matters far more than the row count: if every media item were a
// lone unstacked JPEG, ANALYZE would record selectivities that never occur in a
// real library and the planner would pick plans this guard never exercises.
func seedBrowseScaleCatalog(t *testing.T) (*DB, context.Context, int32, uuid.UUID) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	path := filepath.Join(secureTempDir(t), "browse-plan.sqlite3")
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
		Username:           "browse-plan",
		Password:           "not-used",
		DisplayName:        "Browse Plan",
		Role:               "admin",
		WebauthnUserHandle: []byte("browse-plan-handle"),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	repositoryID := uuid.New()

	tx, err := database.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("exec %q: %v", strings.TrimSpace(query)[:40], err)
		}
	}

	mustExec(`
		INSERT INTO repositories (repo_id, name, path, role, status, created_at, updated_at)
		VALUES (?, 'Plan', '/plan', 'regular', 'active', 1, 1)
	`, repositoryID)

	var currentStackID uuid.UUID
	var stackPosition int
	stackRemaining := 0

	for index := 0; index < planScaleItems; index++ {
		mediaItemID := uuid.New()
		takenTime := int64(2_000_000 - index*37)

		// Roughly one in six media items is a JPEG+RAW pair, one in twenty is a
		// live photo, and the rest are plain JPEGs.
		relations := []string{"jpeg_original"}
		switch {
		case index%6 == 0:
			relations = append(relations, "raw_original")
		case index%20 == 0:
			relations = []string{"live_photo_still", "live_photo_video"}
		}

		assetIDs := make([]uuid.UUID, 0, len(relations))
		for _, relation := range relations {
			assetID := uuid.New()
			assetIDs = append(assetIDs, assetID)
			assetType, mime := "PHOTO", "image/jpeg"
			if relation == "live_photo_video" {
				assetType, mime = "VIDEO", "video/quicktime"
			}
			mustExec(`
				INSERT INTO assets (
					asset_id, owner_id, type, original_filename, mime_type, file_size,
					content_hash, upload_time, taken_time, repository_id, updated_at, is_deleted
				) VALUES (?, ?, ?, ?, ?, 1024, ?, ?, ?, ?, 1, 0)
			`, assetID, user.UserID, assetType, fmt.Sprintf("IMG_%05d%s", index, relation),
				mime, assetID.String(), takenTime, takenTime, repositoryID)
		}

		mustExec(`
			INSERT INTO media_items (
				media_item_id, owner_id, repository_id, media_kind, primary_asset_id, created_at, updated_at
			) VALUES (?, ?, ?, 'photo', ?, 1, 1)
		`, mediaItemID, user.UserID, repositoryID, assetIDs[0])

		for position, relation := range relations {
			mustExec(`
				INSERT INTO media_item_assets (asset_id, media_item_id, relation, position, created_at)
				VALUES (?, ?, ?, ?, 1)
			`, assetIDs[position], mediaItemID, relation, position)
		}

		// About 4% of items live in a burst stack of 3-8 members, matching the
		// shape a real camera roll produces.
		if stackRemaining == 0 && index%25 == 0 {
			currentStackID = uuid.New()
			stackPosition = 0
			stackRemaining = 3 + index%6
			mustExec(`
				INSERT INTO asset_stacks (stack_id, owner_id, repository_id, stack_kind, created_at, updated_at)
				VALUES (?, ?, ?, 'burst', 1, 1)
			`, currentStackID, user.UserID, repositoryID)
		}
		if stackRemaining > 0 {
			mustExec(`
				INSERT INTO asset_stack_members (media_item_id, stack_id, position, created_at)
				VALUES (?, ?, ?, 1)
			`, mediaItemID, currentStackID, stackPosition)
			if stackPosition == 0 {
				mustExec(`UPDATE asset_stacks SET cover_media_item_id = ? WHERE stack_id = ?`,
					mediaItemID, currentStackID)
			}
			stackPosition++
			stackRemaining--
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// The planner only picks production-like plans once it has statistics.
	if _, err := database.SQL.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatalf("analyze: %v", err)
	}
	return database, ctx, user.UserID, repositoryID
}

// explainQueryPlan returns the EXPLAIN QUERY PLAN detail lines for a generated
// query. Every parameter is bound to NULL: SQLite does not sniff parameter
// values when planning, so the access paths this guard asserts on are the same
// ones the real call produces.
func explainQueryPlan(t *testing.T, database *DB, ctx context.Context, query string) []string {
	t.Helper()

	conn, err := database.SQL.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var argCount int
	if err := conn.Raw(func(driverConn any) error {
		preparer, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("driver connection does not implement driver.Conn")
		}
		statement, err := preparer.Prepare(query)
		if err != nil {
			return err
		}
		defer func() { _ = statement.Close() }()
		argCount = statement.NumInput()
		return nil
	}); err != nil {
		t.Fatalf("prepare for plan: %v", err)
	}

	args := make([]any, argCount)
	rows, err := conn.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail sql.NullString
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		details = append(details, strings.TrimSpace(detail.String))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return details
}

// hotBrowseJoinAliases maps every base-table alias on the browse read path to
// the table it stands for. Only base tables are listed: CTE and view scans
// (`filter_params`, `eligible`, `facts`, `paged`, `stack_matches`, `json_each`,
// …) legitimately appear as bare SCAN lines and are not access-path risks.
//
// Deriving the rule from a positive list rather than "no bare SCAN anywhere"
// keeps it stable — adding a CTE must not fail the guard — so the list is
// paired with a completeness check below that catches a silently renamed alias.
var hotBrowseJoinAliases = map[string]string{
	"mi":              "media_items",
	"mia":             "media_item_assets",
	"mia_scope":       "media_item_assets",
	"mia_query":       "media_item_assets",
	"mia_name":        "media_item_assets",
	"pa":              "assets",
	"cover_pa":        "assets",
	"a":               "assets",
	"component":       "assets",
	"component_query": "assets",
	"component_name":  "assets",
	"asm":             "asset_stack_members",
	"s":               "asset_stacks",
}

// planAlias extracts the alias a SCAN/SEARCH step operates on.
func planAlias(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", false
	}
	if fields[0] != "SCAN" && fields[0] != "SEARCH" {
		return "", false
	}
	return fields[1], true
}

// A join that loses its index turns a page render from O(page) into O(catalog):
// the cost is invisible on a development fixture and fatal on a real library.
// That degradation is a plan change, not a timing change, so it is asserted
// here rather than benchmarked.
func TestBrowseQueryPlansKeepBaseTableAccessIndexed(t *testing.T) {
	database, ctx, _, _ := seedBrowseScaleCatalog(t)

	seenAliases := make(map[string]bool, len(hotBrowseJoinAliases))
	for name, query := range repo.BrowseQueryPlanTargets {
		plan := explainQueryPlan(t, database, ctx, query)
		if len(plan) == 0 {
			t.Fatalf("%s produced an empty query plan", name)
		}

		for _, line := range plan {
			alias, ok := planAlias(line)
			if !ok {
				continue
			}
			table, hot := hotBrowseJoinAliases[alias]
			if !hot {
				continue
			}
			seenAliases[alias] = true

			// "SCAN t USING INDEX i" is an index scan and fine; a bare
			// "SCAN t" is a full heap scan of a base table.
			if !strings.Contains(line, "USING") {
				t.Errorf(
					"%s: base table %s (alias %q) is read without an index: %q\nfull plan:\n  %s",
					name, table, alias, line, strings.Join(plan, "\n  "),
				)
			}
		}
	}

	for alias, table := range hotBrowseJoinAliases {
		if !seenAliases[alias] {
			t.Errorf(
				"alias %q (%s) no longer appears in any browse plan; if it was renamed, update hotBrowseJoinAliases or this guard silently stops covering that join",
				alias, table,
			)
		}
	}
}

// The primary-asset join is the one every browse row pays for, so it gets an
// explicit assertion rather than relying on the sweep above.
func TestBrowseQueryPlansResolvePrimaryAssetByPrimaryKey(t *testing.T) {
	database, ctx, _, _ := seedBrowseScaleCatalog(t)

	for _, name := range []string{"GetMediaItemsUnified", "GetCollapsedBrowseItemsUnified"} {
		query, ok := repo.BrowseQueryPlanTargets[name]
		if !ok {
			t.Fatalf("query %q is missing from BrowseQueryPlanTargets", name)
		}

		var resolved bool
		plan := explainQueryPlan(t, database, ctx, query)
		for _, line := range plan {
			alias, ok := planAlias(line)
			if !ok || (alias != "pa" && alias != "cover_pa") {
				continue
			}
			if strings.Contains(line, "USING INDEX sqlite_autoindex_assets_1") ||
				strings.Contains(line, "USING INTEGER PRIMARY KEY") {
				resolved = true
			}
		}
		if !resolved {
			t.Errorf(
				"%s does not resolve the primary asset through the assets primary key\nfull plan:\n  %s",
				name, strings.Join(plan, "\n  "),
			)
		}
	}
}
