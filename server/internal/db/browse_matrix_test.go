package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"server/config"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// The browse matrix runs the real unified SQL against a real migrated SQLite
// catalog. Everything browse-facing is media-item granular, so the fixture is
// described in logical media items: each one names its component relations,
// and stacked items name the presentation stack they belong to.

type fixtureItem struct {
	// name identifies the media item in assertions.
	name string
	// relations are the component files, in order; the first is the primary.
	relations []string
	// stack is the presentation stack name, empty for unstacked items.
	stack string
	// stackKind is only read for the first item of a stack.
	stackKind string
	// cover marks this item as its stack's designated cover.
	cover bool
	// filename overrides the generated component filename prefix.
	filename string
}

type browseFixture struct {
	t            *testing.T
	ctx          context.Context
	db           *DB
	ownerID      int32
	repositoryID uuid.UUID

	itemIDs  map[string]uuid.UUID
	itemName map[uuid.UUID]string
	assetIDs map[string][]uuid.UUID
	stackIDs map[string]uuid.UUID
}

func newBrowseFixture(t *testing.T, items []fixtureItem) *browseFixture {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	path := filepath.Join(secureTempDir(t), "browse-matrix.sqlite3")
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
		Username:           "browse-matrix",
		Password:           "not-used",
		DisplayName:        "Browse Matrix",
		Role:               "admin",
		WebauthnUserHandle: []byte("browse-matrix-handle"),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	f := &browseFixture{
		t:            t,
		ctx:          ctx,
		db:           database,
		ownerID:      user.UserID,
		repositoryID: uuid.New(),
		itemIDs:      map[string]uuid.UUID{},
		itemName:     map[uuid.UUID]string{},
		assetIDs:     map[string][]uuid.UUID{},
		stackIDs:     map[string]uuid.UUID{},
	}

	rootID := uuid.New()
	f.exec(`
		INSERT INTO repository_roots (root_id, name, path, kind, created_at, updated_at)
		VALUES (?, 'Matrix root', '/', 'external', 1, 1)
	`, rootID)
	f.exec(`
		INSERT INTO repositories (repo_id, name, path, role, reachability, activity, created_at, updated_at, root_id)
		VALUES (?, 'Matrix', '/matrix', 'regular', 'active', 'idle', 1, 1, ?)
	`, f.repositoryID, rootID)

	for index, item := range items {
		f.insertItem(index, item)
	}
	return f
}

func (f *browseFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.SQL.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatalf("exec %q: %v", query, err)
	}
}

// insertItem writes one logical media item: its components, its media item
// row, and its stack membership. `takenTime` decreases with the fixture index
// so declaration order is also newest-first browse order.
func (f *browseFixture) insertItem(index int, item fixtureItem) {
	f.t.Helper()

	mediaItemID := uuid.New()
	f.itemIDs[item.name] = mediaItemID
	f.itemName[mediaItemID] = item.name

	takenTime := int64(1_000_000 - index)
	namePrefix := item.filename
	if namePrefix == "" {
		namePrefix = item.name
	}

	assetIDs := make([]uuid.UUID, 0, len(item.relations))
	for _, relation := range item.relations {
		assetID := uuid.New()
		assetIDs = append(assetIDs, assetID)

		assetType, mime, extension := "PHOTO", "image/jpeg", ".jpg"
		switch relation {
		case "raw_original":
			mime, extension = "image/x-adobe-dng", ".dng"
		case "live_photo_video":
			assetType, mime, extension = "VIDEO", "video/quicktime", ".mov"
		}

		f.exec(`
			INSERT INTO assets (
				asset_id, owner_id, type, original_filename, mime_type, file_size,
				content_hash, upload_time, taken_time, repository_id, updated_at, is_deleted
			) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, 1, 0)
		`, assetID, f.ownerID, assetType, namePrefix+extension, mime,
			assetID.String(), takenTime, takenTime, f.repositoryID)

	}
	f.assetIDs[item.name] = assetIDs

	// media_item_assets references media_items, which in turn references its
	// primary asset — so the rows go in assets, media item, components.
	f.exec(`
		INSERT INTO media_items (
			media_item_id, owner_id, repository_id, media_kind, primary_asset_id, created_at, updated_at
		) VALUES (?, ?, ?, 'photo', ?, 1, 1)
	`, mediaItemID, f.ownerID, f.repositoryID, assetIDs[0])

	for position, relation := range item.relations {
		f.exec(`
			INSERT INTO media_item_assets (asset_id, media_item_id, relation, position, created_at)
			VALUES (?, ?, ?, ?, 1)
		`, assetIDs[position], mediaItemID, relation, position)
	}

	if item.stack == "" {
		return
	}

	stackID, known := f.stackIDs[item.stack]
	if !known {
		stackID = uuid.New()
		f.stackIDs[item.stack] = stackID
		kind := item.stackKind
		if kind == "" {
			kind = "manual"
		}
		f.exec(`
			INSERT INTO asset_stacks (stack_id, owner_id, repository_id, stack_kind, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, 1)
		`, stackID, f.ownerID, f.repositoryID, kind)
	}

	var position int
	if err := f.db.SQL.QueryRowContext(f.ctx,
		`SELECT COUNT(*) FROM asset_stack_members WHERE stack_id = ?`, stackID,
	).Scan(&position); err != nil {
		f.t.Fatalf("count stack members: %v", err)
	}
	f.exec(`
		INSERT INTO asset_stack_members (media_item_id, stack_id, position, created_at)
		VALUES (?, ?, ?, 1)
	`, mediaItemID, stackID, position)

	if item.cover {
		f.exec(`UPDATE asset_stacks SET cover_media_item_id = ? WHERE stack_id = ?`, mediaItemID, stackID)
	}
}

// browseFilter is the subset of browse predicates this matrix varies.
type browseFilter struct {
	composition string
	membership  string
	kinds       []string
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (f *browseFixture) kindsParam(kinds []string) *string {
	f.t.Helper()
	if len(kinds) == 0 {
		return nil
	}
	encoded := `["` + kinds[0] + `"`
	for _, kind := range kinds[1:] {
		encoded += `,"` + kind + `"`
	}
	encoded += `]`
	return &encoded
}

func (f *browseFixture) expandedNames(filter browseFilter, limit, offset int64) []string {
	f.t.Helper()

	rows, err := f.db.Queries.GetMediaItemsUnified(f.ctx, repo.GetMediaItemsUnifiedParams{
		IsDeleted:       false,
		OwnerID:         f.ownerID,
		RepositoryID:    uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		SortBy:          "date_captured",
		Composition:     nullableString(filter.composition),
		StackMembership: nullableString(filter.membership),
		StackKinds:      f.kindsParam(filter.kinds),
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		f.t.Fatalf("expanded browse: %v", err)
	}

	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, f.itemName[row.MediaItemID])
	}
	return names
}

// collapsedRows returns the collapsed browse rows as "type:name" labels —
// "media:<item>" for unstacked rows, "stack:<stack>" for collapsed stacks.
func (f *browseFixture) collapsedRows(filter browseFilter, limit, offset int64) []string {
	f.t.Helper()

	rows, err := f.db.Queries.GetCollapsedBrowseItemsUnified(f.ctx, repo.GetCollapsedBrowseItemsUnifiedParams{
		IsDeleted:       false,
		OwnerID:         f.ownerID,
		RepositoryID:    uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		SortBy:          "date_captured",
		Composition:     nullableString(filter.composition),
		StackMembership: nullableString(filter.membership),
		StackKinds:      f.kindsParam(filter.kinds),
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		f.t.Fatalf("collapsed browse: %v", err)
	}

	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ItemType == "stack" {
			labels = append(labels, "stack:"+f.stackName(row.StackID))
			continue
		}
		labels = append(labels, "media:"+f.itemName[row.CoverMediaItemID])
	}
	return labels
}

func (f *browseFixture) stackName(stackID uuid.UUID) string {
	for name, id := range f.stackIDs {
		if id == stackID {
			return name
		}
	}
	return stackID.String()
}

func (f *browseFixture) counts(filter browseFilter) (mediaItems, files, collapsed int64) {
	f.t.Helper()

	var err error
	mediaItems, err = f.db.Queries.CountMediaItemsUnified(f.ctx, repo.CountMediaItemsUnifiedParams{
		IsDeleted:       false,
		OwnerID:         f.ownerID,
		RepositoryID:    uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		Composition:     nullableString(filter.composition),
		StackMembership: nullableString(filter.membership),
		StackKinds:      f.kindsParam(filter.kinds),
	})
	if err != nil {
		f.t.Fatalf("count media items: %v", err)
	}

	files, err = f.db.Queries.CountMediaItemFilesUnified(f.ctx, repo.CountMediaItemFilesUnifiedParams{
		IsDeleted:       false,
		OwnerID:         f.ownerID,
		RepositoryID:    uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		Composition:     nullableString(filter.composition),
		StackMembership: nullableString(filter.membership),
		StackKinds:      f.kindsParam(filter.kinds),
	})
	if err != nil {
		f.t.Fatalf("count media item files: %v", err)
	}

	collapsed, err = f.db.Queries.CountCollapsedBrowseItemsUnified(f.ctx, repo.CountCollapsedBrowseItemsUnifiedParams{
		IsDeleted:       false,
		OwnerID:         f.ownerID,
		RepositoryID:    uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		Composition:     nullableString(filter.composition),
		StackMembership: nullableString(filter.membership),
		StackKinds:      f.kindsParam(filter.kinds),
	})
	if err != nil {
		f.t.Fatalf("count collapsed browse items: %v", err)
	}
	return mediaItems, files, collapsed
}

// matrixFixture covers the shapes the browse contract has to distinguish:
// plain JPEG, JPEG+RAW pair, unpaired RAW, edited, live photo, and each of
// those inside a burst stack, a manual stack, or unstacked.
func matrixFixture(t *testing.T) *browseFixture {
	t.Helper()

	return newBrowseFixture(t, []fixtureItem{
		{name: "solo-jpeg", relations: []string{"jpeg_original"}},
		{name: "solo-pair", relations: []string{"jpeg_original", "raw_original"}},
		{name: "solo-raw", relations: []string{"raw_original"}},
		{name: "solo-edited", relations: []string{"jpeg_original", "edited_version"}},
		{name: "solo-live", relations: []string{"live_photo_still", "live_photo_video"}},

		{name: "burst-cover", relations: []string{"jpeg_original"}, stack: "burst", stackKind: "burst", cover: true},
		{name: "burst-pair", relations: []string{"jpeg_original", "raw_original"}, stack: "burst", stackKind: "burst"},
		{name: "burst-raw", relations: []string{"raw_original"}, stack: "burst", stackKind: "burst"},

		{name: "manual-cover", relations: []string{"jpeg_original"}, stack: "manual", stackKind: "manual", cover: true},
		{name: "manual-pair", relations: []string{"jpeg_original", "raw_original"}, stack: "manual", stackKind: "manual"},
	})
}

func assertNamesEqual(t *testing.T, got, want []string, context string) {
	t.Helper()

	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)

	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("%s: got %v, want %v", context, got, want)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("%s: got %v, want %v", context, got, want)
		}
	}
}

func TestBrowseMatrixCompositionFilters(t *testing.T) {
	f := matrixFixture(t)

	cases := []struct {
		composition string
		want        []string
	}{
		{"", []string{
			"solo-jpeg", "solo-pair", "solo-raw", "solo-edited", "solo-live",
			"burst-cover", "burst-pair", "burst-raw", "manual-cover", "manual-pair",
		}},
		{"contains_raw", []string{"solo-pair", "solo-raw", "burst-pair", "burst-raw", "manual-pair"}},
		{"jpeg_raw", []string{"solo-pair", "burst-pair", "manual-pair"}},
		{"raw_unpaired", []string{"solo-raw", "burst-raw"}},
		{"no_raw", []string{"solo-jpeg", "solo-edited", "solo-live", "burst-cover", "manual-cover"}},
		// Live photos are a component-makeup fact like the RAW pairings, so they
		// select on the same axis rather than on media type.
		{"live_photo", []string{"solo-live"}},
	}

	for _, testCase := range cases {
		t.Run("composition="+testCase.composition, func(t *testing.T) {
			filter := browseFilter{composition: testCase.composition}
			got := f.expandedNames(filter, 100, 0)
			assertNamesEqual(t, got, testCase.want, "expanded rows")

			mediaItems, _, _ := f.counts(filter)
			if mediaItems != int64(len(testCase.want)) {
				t.Fatalf("total_media_items = %d, want %d", mediaItems, len(testCase.want))
			}
		})
	}
}

func TestBrowseMatrixStackFilters(t *testing.T) {
	f := matrixFixture(t)

	cases := []struct {
		name   string
		filter browseFilter
		want   []string
	}{
		{"unstacked", browseFilter{membership: "unstacked"},
			[]string{"solo-jpeg", "solo-pair", "solo-raw", "solo-edited", "solo-live"}},
		{"stacked", browseFilter{membership: "stacked"},
			[]string{"burst-cover", "burst-pair", "burst-raw", "manual-cover", "manual-pair"}},
		{"burst only", browseFilter{kinds: []string{"burst"}},
			[]string{"burst-cover", "burst-pair", "burst-raw"}},
		{"manual only", browseFilter{kinds: []string{"manual"}},
			[]string{"manual-cover", "manual-pair"}},
		{"both kinds", browseFilter{kinds: []string{"burst", "manual"}},
			[]string{"burst-cover", "burst-pair", "burst-raw", "manual-cover", "manual-pair"}},
		// A single kind already implies "stacked", so adding the membership
		// must not change the result.
		{"stacked and burst", browseFilter{membership: "stacked", kinds: []string{"burst"}},
			[]string{"burst-cover", "burst-pair", "burst-raw"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := f.expandedNames(testCase.filter, 100, 0)
			assertNamesEqual(t, got, testCase.want, "expanded rows")

			mediaItems, _, _ := f.counts(testCase.filter)
			if mediaItems != int64(len(testCase.want)) {
				t.Fatalf("total_media_items = %d, want %d", mediaItems, len(testCase.want))
			}
		})
	}
}

func TestBrowseMatrixCompositionAndStackFiltersCompose(t *testing.T) {
	f := matrixFixture(t)

	cases := []struct {
		name   string
		filter browseFilter
		want   []string
	}{
		{"jpeg_raw in a burst", browseFilter{composition: "jpeg_raw", kinds: []string{"burst"}},
			[]string{"burst-pair"}},
		{"unpaired raw unstacked", browseFilter{composition: "raw_unpaired", membership: "unstacked"},
			[]string{"solo-raw"}},
		{"no raw stacked", browseFilter{composition: "no_raw", membership: "stacked"},
			[]string{"burst-cover", "manual-cover"}},
		{"empty intersection", browseFilter{composition: "raw_unpaired", kinds: []string{"manual"}},
			[]string{}},
		{"live photos are never RAW", browseFilter{composition: "live_photo", membership: "unstacked"},
			[]string{"solo-live"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := f.expandedNames(testCase.filter, 100, 0)
			assertNamesEqual(t, got, testCase.want, "expanded rows")
		})
	}
}

func TestBrowseMatrixCollapsedRowsGroupStacks(t *testing.T) {
	f := matrixFixture(t)

	got := f.collapsedRows(browseFilter{}, 100, 0)
	assertNamesEqual(t, got, []string{
		"media:solo-jpeg", "media:solo-pair", "media:solo-raw",
		"media:solo-edited", "media:solo-live",
		"stack:burst", "stack:manual",
	}, "collapsed rows")

	// A filter matching only one stack member still yields the whole stack as
	// a single collapsed row.
	got = f.collapsedRows(browseFilter{composition: "raw_unpaired"}, 100, 0)
	assertNamesEqual(t, got, []string{"media:solo-raw", "stack:burst"}, "partially matched stack")
}

func TestBrowseMatrixListAndCountAgreeAcrossModes(t *testing.T) {
	f := matrixFixture(t)

	filters := []browseFilter{
		{},
		{composition: "contains_raw"},
		{composition: "jpeg_raw"},
		{composition: "raw_unpaired"},
		{composition: "no_raw"},
		{composition: "live_photo"},
		{membership: "stacked"},
		{membership: "unstacked"},
		{kinds: []string{"burst"}},
		{kinds: []string{"manual"}},
		{composition: "no_raw", membership: "stacked"},
	}

	for _, filter := range filters {
		mediaItems, files, collapsed := f.counts(filter)

		expanded := f.expandedNames(filter, 1000, 0)
		if int64(len(expanded)) != mediaItems {
			t.Fatalf("filter %+v: expanded list has %d rows, count says %d", filter, len(expanded), mediaItems)
		}

		collapsedRows := f.collapsedRows(filter, 1000, 0)
		if int64(len(collapsedRows)) != collapsed {
			t.Fatalf("filter %+v: collapsed list has %d rows, count says %d", filter, len(collapsedRows), collapsed)
		}
		if collapsed > mediaItems {
			t.Fatalf("filter %+v: collapsed rows (%d) exceed media items (%d)", filter, collapsed, mediaItems)
		}
		if files < mediaItems {
			t.Fatalf("filter %+v: total_files (%d) is below total_media_items (%d)", filter, files, mediaItems)
		}
	}
}

func TestBrowseMatrixPaginationCoversEveryRowExactlyOnce(t *testing.T) {
	f := matrixFixture(t)

	for _, mode := range []string{"expanded", "collapsed"} {
		t.Run(mode, func(t *testing.T) {
			page := func(limit, offset int64) []string {
				if mode == "expanded" {
					return f.expandedNames(browseFilter{}, limit, offset)
				}
				return f.collapsedRows(browseFilter{}, limit, offset)
			}

			all := page(1000, 0)
			var paged []string
			for offset := int64(0); offset < int64(len(all)); offset += 3 {
				paged = append(paged, page(3, offset)...)
			}

			if len(paged) != len(all) {
				t.Fatalf("paged through %d rows, unpaged list has %d", len(paged), len(all))
			}
			for i := range all {
				if paged[i] != all[i] {
					t.Fatalf("pagination reordered rows at %d: %v vs %v", i, paged, all)
				}
			}
			if got := page(3, int64(len(all))); len(got) != 0 {
				t.Fatalf("offset past the end returned %v", got)
			}
		})
	}
}

func TestBrowseMatrixCompositionFactsMatchComponents(t *testing.T) {
	f := matrixFixture(t)

	rows, err := f.db.Queries.GetMediaItemsUnified(f.ctx, repo.GetMediaItemsUnifiedParams{
		IsDeleted:    false,
		OwnerID:      f.ownerID,
		RepositoryID: uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		SortBy:       "date_captured",
		Limit:        1000,
	})
	if err != nil {
		t.Fatalf("expanded browse: %v", err)
	}

	want := map[string]struct {
		components              int64
		raw, jpeg, edited, live bool
	}{
		"solo-jpeg":    {1, false, true, false, false},
		"solo-pair":    {2, true, true, false, false},
		"solo-raw":     {1, true, false, false, false},
		"solo-edited":  {2, false, true, true, false},
		"solo-live":    {2, false, false, false, true},
		"burst-cover":  {1, false, true, false, false},
		"burst-pair":   {2, true, true, false, false},
		"burst-raw":    {1, true, false, false, false},
		"manual-cover": {1, false, true, false, false},
		"manual-pair":  {2, true, true, false, false},
	}

	seen := 0
	for _, row := range rows {
		name := f.itemName[row.MediaItemID]
		expected, ok := want[name]
		if !ok {
			t.Fatalf("unexpected media item %q", name)
		}
		seen++

		asBool := func(value int64) bool { return value == 1 }
		if row.ComponentCount != expected.components ||
			asBool(row.HasRaw) != expected.raw ||
			asBool(row.HasJpeg) != expected.jpeg ||
			asBool(row.HasEdited) != expected.edited ||
			asBool(row.HasLiveMotion) != expected.live {
			t.Fatalf("%s: facts = {count:%d raw:%d jpeg:%d edited:%d live:%d}, want %+v",
				name, row.ComponentCount, row.HasRaw, row.HasJpeg, row.HasEdited, row.HasLiveMotion, expected)
		}
	}
	if seen != len(want) {
		t.Fatalf("browsed %d media items, fixture has %d", seen, len(want))
	}
}

// TestBrowseMatrixInvariants asserts the structural rules the browse contract
// depends on: every asset belongs to exactly one media item, every media item
// points at one of its own components, stacks have at least two members with
// contiguous positions, and stack covers are members of their own stack.
func TestBrowseMatrixInvariants(t *testing.T) {
	f := matrixFixture(t)

	assertNoRows := func(name, query string) {
		t.Helper()
		rows, err := f.db.SQL.QueryContext(f.ctx, query)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer func() { _ = rows.Close() }()

		var offenders []string
		for rows.Next() {
			var offender sql.NullString
			if err := rows.Scan(&offender); err != nil {
				t.Fatalf("%s scan: %v", name, err)
			}
			offenders = append(offenders, offender.String)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("%s rows: %v", name, err)
		}
		if len(offenders) > 0 {
			t.Fatalf("invariant %q violated by %v", name, offenders)
		}
	}

	assertNoRows("every asset belongs to a media item", `
		SELECT a.asset_id FROM assets a
		LEFT JOIN media_item_assets mia ON mia.asset_id = a.asset_id
		WHERE mia.asset_id IS NULL
	`)

	assertNoRows("no asset belongs to two media items", `
		SELECT asset_id FROM media_item_assets
		GROUP BY asset_id HAVING COUNT(*) > 1
	`)

	assertNoRows("every media item has a primary asset", `
		SELECT media_item_id FROM media_items WHERE primary_asset_id IS NULL
	`)

	assertNoRows("the primary asset is one of the item's own components", `
		SELECT mi.media_item_id FROM media_items mi
		LEFT JOIN media_item_assets mia
		  ON mia.media_item_id = mi.media_item_id AND mia.asset_id = mi.primary_asset_id
		WHERE mia.asset_id IS NULL
	`)

	assertNoRows("a presentation stack has at least two members", `
		SELECT s.stack_id FROM asset_stacks s
		LEFT JOIN asset_stack_members m ON m.stack_id = s.stack_id
		GROUP BY s.stack_id HAVING COUNT(m.media_item_id) < 2
	`)

	assertNoRows("stack member positions are contiguous from zero", `
		SELECT stack_id FROM asset_stack_members
		GROUP BY stack_id
		HAVING MIN(position) <> 0 OR MAX(position) <> COUNT(*) - 1
	`)

	assertNoRows("a stack cover is a member of its own stack", `
		SELECT s.stack_id FROM asset_stacks s
		LEFT JOIN asset_stack_members m
		  ON m.stack_id = s.stack_id AND m.media_item_id = s.cover_media_item_id
		WHERE s.cover_media_item_id IS NOT NULL AND m.media_item_id IS NULL
	`)

	// Every browse row a filter can produce resolves to a visible primary asset.
	rows, err := f.db.Queries.GetMediaItemsUnified(f.ctx, repo.GetMediaItemsUnifiedParams{
		IsDeleted:    false,
		OwnerID:      f.ownerID,
		RepositoryID: uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		SortBy:       "date_captured",
		Limit:        1000,
	})
	if err != nil {
		t.Fatalf("expanded browse: %v", err)
	}
	for _, row := range rows {
		if row.Asset.AssetID == uuid.Nil {
			t.Fatalf("media item %s browsed without a primary asset", row.MediaItemID)
		}
	}
}

func decodeMemberNames(t *testing.T, f *browseFixture, encoded any) []string {
	t.Helper()

	var payload []byte
	switch value := encoded.(type) {
	case nil:
		return nil
	case []byte:
		payload = value
	case string:
		payload = []byte(value)
	default:
		t.Fatalf("unexpected member payload %T", encoded)
	}

	var members []struct {
		MediaItemID uuid.UUID `json:"media_item_id"`
	}
	if err := json.Unmarshal(payload, &members); err != nil {
		t.Fatalf("decode members %q: %v", payload, err)
	}

	names := make([]string, 0, len(members))
	for _, member := range members {
		names = append(names, f.itemName[member.MediaItemID])
	}
	return names
}

// A partially matched stack keeps its designated cover — the stack row stands
// for the whole stack — while matched_items narrows to the members the filter
// actually hit. That pair is what drives the "2 / 3" thumbnail badge.
func TestBrowseMatrixPartiallyMatchedStackKeepsCoverAndNarrowsMatches(t *testing.T) {
	f := newBrowseFixture(t, []fixtureItem{
		{name: "cover-jpeg", relations: []string{"jpeg_original"}, stack: "burst", stackKind: "burst", cover: true},
		{name: "member-pair", relations: []string{"jpeg_original", "raw_original"}, stack: "burst", stackKind: "burst"},
		{name: "member-raw", relations: []string{"raw_original"}, stack: "burst", stackKind: "burst"},
	})

	rows, err := f.db.Queries.GetCollapsedBrowseItemsUnified(f.ctx, repo.GetCollapsedBrowseItemsUnifiedParams{
		IsDeleted:    false,
		OwnerID:      f.ownerID,
		RepositoryID: uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		SortBy:       "date_captured",
		Composition:  "contains_raw",
		Limit:        100,
	})
	if err != nil {
		t.Fatalf("collapsed browse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("collapsed rows = %d, want 1", len(rows))
	}
	if rows[0].ItemType != "stack" {
		t.Fatalf("row type = %q, want stack", rows[0].ItemType)
	}
	if got := f.itemName[rows[0].CoverMediaItemID]; got != "cover-jpeg" {
		t.Fatalf("cover = %q, want the designated cover-jpeg", got)
	}

	assertNamesEqual(t, decodeMemberNames(t, f, rows[0].MemberItems),
		[]string{"cover-jpeg", "member-pair", "member-raw"}, "member_items")
	assertNamesEqual(t, decodeMemberNames(t, f, rows[0].MatchedItems),
		[]string{"member-pair", "member-raw"}, "matched_items")
}

// When the designated cover is not visible, the cover falls back to the
// lowest-position visible member instead of dropping the row.
func TestBrowseMatrixCollapsedCoverFallsBackWhenCoverIsDeleted(t *testing.T) {
	f := newBrowseFixture(t, []fixtureItem{
		{name: "cover-jpeg", relations: []string{"jpeg_original"}, stack: "burst", stackKind: "burst", cover: true},
		{name: "member-a", relations: []string{"jpeg_original"}, stack: "burst", stackKind: "burst"},
		{name: "member-b", relations: []string{"jpeg_original"}, stack: "burst", stackKind: "burst"},
	})

	f.exec(`UPDATE assets SET is_deleted = 1 WHERE asset_id = ?`, f.assetIDs["cover-jpeg"][0])

	rows, err := f.db.Queries.GetCollapsedBrowseItemsUnified(f.ctx, repo.GetCollapsedBrowseItemsUnifiedParams{
		IsDeleted:    false,
		OwnerID:      f.ownerID,
		RepositoryID: uuid.NullUUID{UUID: f.repositoryID, Valid: true},
		SortBy:       "date_captured",
		Limit:        100,
	})
	if err != nil {
		t.Fatalf("collapsed browse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("collapsed rows = %d, want 1", len(rows))
	}
	if got := f.itemName[rows[0].CoverMediaItemID]; got != "member-a" {
		t.Fatalf("cover fell back to %q, want member-a", got)
	}
	assertNamesEqual(t, decodeMemberNames(t, f, rows[0].MemberItems),
		[]string{"member-a", "member-b"}, "member_items")
}
