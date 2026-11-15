package repo

// BrowseQueryPlanTargets exposes the generated SQL of the hot browse queries so
// query-plan guards can run EXPLAIN QUERY PLAN against the exact text sqlc
// emits, instead of duplicating the SQL in a test and drifting from it.
//
// This is the browse read path: the two list queries the gallery pages through
// and the three counts that back total_visible / total_media_items /
// total_files. A full table scan appearing here is the difference between a
// library that stays usable at 10^6 media items and one that does not.
var BrowseQueryPlanTargets = map[string]string{
	"GetMediaItemsUnified":             getMediaItemsUnified,
	"GetCollapsedBrowseItemsUnified":   getCollapsedBrowseItemsUnified,
	"CountMediaItemsUnified":           countMediaItemsUnified,
	"CountMediaItemFilesUnified":       countMediaItemFilesUnified,
	"CountCollapsedBrowseItemsUnified": countCollapsedBrowseItemsUnified,
}
