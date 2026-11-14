package app

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The production wiring binds non-transactional reads to the query-only
// pool and keeps commit wake-ups, the WAL monitor, and checkpoints wired.
// These are text checks on app.go, moved here from architecturecheck; the
// foreground-read tests cover the HTTP paths at run time. Sequence step B2
// replaces them with constructors that take catalogtx.Reader or
// catalogtx.Writer, so a wrong pool no longer compiles, and deletes this test.
func TestProductionCatalogWiring(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, requirement := range []struct {
		name    string
		snippet string
	}{
		{name: "River executor on queue writer and listener on queue readers", snippet: "queue.New(queueDatabase.SQL, queueDatabase.ReaderSQL, workers"},
		{name: "Catalog commit scheduling wake", snippet: "db.Open(ctx, dbConfig, db.WithTransactionObserver(catalogScheduleWake))"},
		{name: "Catalog-derived scheduler", snippet: "queue.NewScheduler(database.Reader, database.Writer, queueClient"},
		{name: "Catalog scheduler wake consumption", snippet: "catalogScheduleWake.Signals()"},
		{name: "writer wait and WAL monitor", snippet: "monitorSQLiteWriter(\n\t\tctx,\n\t\tdatabase"},
		{name: "idle WAL checkpoint suppression", snippet: "walStateAlreadyCheckpointed(walState"},
		{name: "independent queue WAL checkpoint", snippet: `checkpointWAL("queue"`},
		{name: "backup source reader", snippet: "Source:   database.ReaderSQL"},
		{name: "queue status reader", snippet: "handler.NewQueueHandler(queueDatabase.ReaderSQL, database.ReaderSQL)"},
		{name: "event planning reader", snippet: "event.NewServiceWithCatalog(database.Writer, database.Reader"},
		{name: "event HTTP catalog capabilities", snippet: "handler.NewEventHandlerWithReader(eventService, sqlDB, database.Writer, database.ReaderSQL"},
		{name: "agent library reader", snippet: "core.NewAuthorizedLibraryFactory(queries, assetService, database.ReaderSQL)"},
		{name: "classifier catalog capabilities", snippet: "service.NewClassifierServiceWithCatalog(sqlDB, readerDB, writer"},
	} {
		if !strings.Contains(text, requirement.snippet) {
			t.Errorf("app.go lost %s (%q); foreground and background planning reads must use the query-only pool", requirement.name, requirement.snippet)
		}
	}

	for _, forbidden := range []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{name: "raw production read on the writer", pattern: regexp.MustCompile(`\b(?:sqlDB|database\.SQL)\.(?:Query|QueryRow|Prepare)(?:Context)?\(`)},
		{name: "queue status bound to the writer", pattern: regexp.MustCompile(`handler\.NewQueueHandler\(\s*sqlDB\s*[,)]`)},
		{name: "backup copy bound to the writer", pattern: regexp.MustCompile(`Source:\s+sqlDB\b`)},
		{name: "agent library bound to the writer", pattern: regexp.MustCompile(`NewAuthorizedLibraryFactory\(queries, assetService, sqlDB\)`)},
	} {
		if location := forbidden.pattern.FindStringIndex(text); location != nil {
			line := 1 + strings.Count(text[:location[0]], "\n")
			t.Errorf("app.go:%d has %s; bind non-transactional reads to database.ReaderSQL or the routed database.Queries", line, forbidden.name)
		}
	}
}
