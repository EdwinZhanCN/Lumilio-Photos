package main

// This repository-level guard intentionally lives in Go so it can run on every
// supported development host without a POSIX shell or awk/grep pipeline.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"server/internal/api/problem"
)

var (
	queryConstPattern       = regexp.MustCompile("^const [[:alnum:]_]+ = " + string(rune(96)))
	numberedParameter       = regexp.MustCompile("\\?[[:digit:]]+")
	rawReference            = regexp.MustCompile("PhotoSpecificMetadata|dbtypes|is_raw.*(metadata|exif)|legacyBrowseStateMigration")
	stackListExclusion      = regexp.MustCompile("useAssetsList\\.ts|schema\\.d\\.ts")
	retiredRepositoryTerms  = regexp.MustCompile(`(?i)\b(?:storage|repository) root\b|\blibrary\b|图库|媒体库|存储根目录|存储根|资源库根|仓库`)
	retiredCapabilityLabels = regexp.MustCompile(`["'` + "`" + `](Semantic Search|Face Recognition|OCR|Species Recognition|语义搜索|人脸识别|物种识别)["'` + "`" + `]`)
	apiErrorKeyPattern      = regexp.MustCompile(`t\(\s*["'](apiErrors\.[[:alnum:]_.]+)["']`)
	privateProblemHelper    = regexp.MustCompile(`^(?:export[[:space:]]+)?(?:function|const)[[:space:]]+(?:normalize|parse|read)[[:alnum:]_]*(?:Problem|problem)`)
)

func main() {
	root, err := gitRoot()
	if err != nil {
		fail(err)
	}
	if err := checkSQLiteSQL(root); err != nil {
		fail(err)
	}
	fmt.Println("SQLite SQL checks passed")

	if err := checkSQLiteConnectionArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("SQLite connection architecture checks passed")

	if err := checkSQLiteTransactionInventory(root); err != nil {
		fail(err)
	}
	fmt.Println("SQLite transaction inventory checks passed")

	if err := checkBrowseArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Browse architecture checks passed")

	if err := checkDesktopArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Desktop architecture checks passed")

	if err := checkRepositoryPathArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Repository path architecture checks passed")

	if err := checkUserFacingTerminology(root); err != nil {
		fail(err)
	}
	fmt.Println("User-facing terminology checks passed")

	if err := checkAPIProblemArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("API Problem architecture checks passed")

	if err := checkAsyncPipelineArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Async pipeline architecture checks passed")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func gitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func gitGrep(root, pattern string, paths ...string) (string, error) {
	args := []string{"grep", "-n", "-E", pattern, "--"}
	args = append(args, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return "", nil
	}
	return "", fmt.Errorf("git grep %q: %w: %s", pattern, err, strings.TrimSpace(string(output)))
}

func gitFiles(root string, pathspec string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--", pathspec)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", pathspec, err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func checkSQLiteSQL(root string) error {
	matches, err := gitGrep(
		root,
		"sqlc\\.(arg|narg|slice|embed)|@[[:alpha:]_][[:alnum:]_]*",
		"server/internal/db/repo/*.sql.go",
	)
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"SQLite SQL check found an unresolved sqlc parameter macro in generated SQL:\n%s",
			strings.TrimSpace(matches),
		)
	}

	matches, err = gitGrep(
		root,
		"randomblob[[:space:]]*\\(",
		"server/internal/db/repo/queries/*.sql",
	)
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"SQLite SQL check found database-side randomness in an application query:\n%s\nGenerate UUIDs and security-sensitive random values in Go and pass them explicitly.",
			strings.TrimSpace(matches),
		)
	}

	mixed, err := mixedSliceQueries(root)
	if err != nil {
		return err
	}
	if len(mixed) > 0 {
		return fmt.Errorf(
			"SQLite SQL check found a generated sqlc slice query mixed with fixed parameters:\n%s\nUse a JSON1 list parameter instead; sqlc numbered placeholders are invalid after dynamic slice expansion.",
			strings.Join(mixed, "\n"),
		)
	}
	return nil
}

func checkSQLiteConnectionArchitecture(root string) error {
	databasePath := filepath.Join(root, "server/internal/db/db.go")
	databaseSource, err := os.ReadFile(databasePath)
	if err != nil {
		return fmt.Errorf("read SQLite connection boundary: %w", err)
	}
	databaseText := string(databaseSource)
	for _, requirement := range []struct {
		name    string
		snippet string
	}{
		{name: "one physical writer", snippet: "database.SetMaxOpenConns(1)"},
		{name: "read-only reader DSN", snippet: `"mode":          {"ro"}`},
		{name: "query-only reader guard", snippet: `"_query_only":   {"on"}`},
		{name: "bounded reader pool", snippet: "reader.SetMaxOpenConns(4)"},
		{name: "default generated query router", snippet: "Queries:       repo.New(newQueryRouter(database, reader, writerCapability, readerCapability))"},
		{name: "named writer capability", snippet: "Writer:        writerCapability"},
		{name: "named reader capability", snippet: "Reader:        readerCapability"},
		{name: "explicit checkpoint policy", snippet: `"PRAGMA wal_autocheckpoint = 0"`},
	} {
		if !strings.Contains(databaseText, requirement.snippet) {
			return fmt.Errorf(
				"SQLite connection architecture lost %s (%q); preserve one writer, query-only WAL readers, routed generated reads, and explicit checkpoint ownership",
				requirement.name,
				requirement.snippet,
			)
		}
	}

	appPath := filepath.Join(root, "server/app/app.go")
	appSource, err := os.ReadFile(appPath)
	if err != nil {
		return fmt.Errorf("read production SQLite wiring: %w", err)
	}
	appText := string(appSource)
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
		{name: "queue status reader", snippet: "handler.NewQueueHandler(queueDatabase.ReaderSQL)"},
		{name: "event planning reader", snippet: "event.NewServiceWithCatalog(database.Writer, database.Reader"},
		{name: "event HTTP catalog capabilities", snippet: "handler.NewEventHandlerWithReader(eventService, sqlDB, database.Writer, database.ReaderSQL"},
		{name: "agent library reader", snippet: "core.NewAuthorizedLibraryFactory(queries, assetService, database.ReaderSQL)"},
		{name: "classifier catalog capabilities", snippet: "service.NewClassifierServiceWithCatalog(sqlDB, readerDB, writer"},
	} {
		if !strings.Contains(appText, requirement.snippet) {
			return fmt.Errorf(
				"production SQLite wiring lost %s (%q); foreground/background planning reads must use the query-only pool",
				requirement.name,
				requirement.snippet,
			)
		}
	}

	for _, forbidden := range []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{name: "raw production read on the writer", pattern: regexp.MustCompile(`\b(?:sqlDB|database\.SQL)\.(?:Query|QueryRow|Prepare)(?:Context)?\(`)},
		{name: "queue status bound to the writer", pattern: regexp.MustCompile(`handler\.NewQueueHandler\(\s*sqlDB\s*\)`)},
		{name: "backup copy bound to the writer", pattern: regexp.MustCompile(`Source:\s+sqlDB\b`)},
		{name: "agent library bound to the writer", pattern: regexp.MustCompile(`NewAuthorizedLibraryFactory\(queries, assetService, sqlDB\)`)},
	} {
		if location := forbidden.pattern.FindStringIndex(appText); location != nil {
			line := 1 + strings.Count(appText[:location[0]], "\n")
			return fmt.Errorf(
				"SQLite connection architecture found %s at server/app/app.go:%d; bind non-transactional reads to database.ReaderSQL or the routed database.Queries surface",
				forbidden.name,
				line,
			)
		}
	}

	bootstrapPath := filepath.Join(root, "server/internal/service/bootstrap_service.go")
	bootstrapSource, err := os.ReadFile(bootstrapPath)
	if err != nil {
		return fmt.Errorf("read bootstrap SQLite boundary: %w", err)
	}
	bootstrapText := string(bootstrapSource)
	if !strings.Contains(bootstrapText, "return s.compute(ctx)") {
		return errors.New("bootstrap Phase lost its read-only derivation; setup status and initialization middleware must not wait for SQLite's writer")
	}
	if strings.Contains(bootstrapText, "return s.Reconcile(ctx)") {
		return errors.New("bootstrap Phase calls Reconcile; foreground initialization reads must never persist through SQLite's sole writer")
	}

	runtimeStatusSource, err := os.ReadFile(filepath.Join(root, "server/internal/storage/runtime_status.go"))
	if err != nil {
		return fmt.Errorf("read storage runtime status boundary: %w", err)
	}
	if strings.Contains(string(runtimeStatusSource), "rm.Reconcile") {
		return errors.New("storage runtime status performs reconciliation; GET/setup status must read the cached projection without acquiring SQLite's writer")
	}

	repositoryRootSource, err := os.ReadFile(filepath.Join(root, "server/internal/storage/repository_roots.go"))
	if err != nil {
		return fmt.Errorf("read Storage Location list boundary: %w", err)
	}
	if strings.Contains(string(repositoryRootSource), "func (rm *DefaultRepositoryManager) ListRepositoryRoots(ctx context.Context) ([]repo.RepositoryRoot, error) {\n\tif err := rm.ReconcileRepositoryRoots(ctx)") {
		return errors.New("ListRepositoryRoots reconciles on a foreground read; background storage reconciliation owns projection writes")
	}

	hostActionSource, err := os.ReadFile(filepath.Join(root, "server/internal/storage/host_action.go"))
	if err != nil {
		return fmt.Errorf("read host action list boundary: %w", err)
	}
	for _, forbidden := range []string{
		"func (rm *DefaultRepositoryManager) ListPendingHostActions(ctx context.Context) ([]HostAction, error) {\n\tif err := rm.expirePendingHostActions(ctx)",
		"func (rm *DefaultRepositoryManager) ListHostActionsForActor(ctx context.Context, actorUserID int32) ([]HostAction, error) {\n\tif err := rm.expirePendingHostActions(ctx)",
	} {
		if strings.Contains(string(hostActionSource), forbidden) {
			return errors.New("host action list persists expiry on a foreground read; materialize expiry while reading and persist cleanup only at an explicit maintenance boundary")
		}
	}
	handlerReconcile, err := scanGoLines(root, "server/internal/api/handler", func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		return strings.Contains(line, ".ReconcileAll(") || strings.Contains(line, ".ReconcileRepositoryRoots(")
	})
	if err != nil {
		return err
	}
	if len(handlerReconcile) > 0 {
		return fmt.Errorf(
			"foreground HTTP handler performs storage reconciliation:\n%s\nStartup/background controllers own projection writes; GET handlers consume the latest reader projection",
			strings.Join(handlerReconcile, "\n"),
		)
	}
	return nil
}

// checkAsyncPipelineArchitecture protects the queue/control-plane seam. These
// checks intentionally inspect the working tree (not only the Git index) so a
// newly added worker cannot bypass the guard before it is staged.
func checkAsyncPipelineArchitecture(root string) error {
	legacy := regexp.MustCompile(`\b(?:ProcessSemantic|ProcessBioClip|ProcessOcr|ProcessFace|ProcessVideoFrames|ProcessPHash|ZeroshotClassify|DetectStacks|LivePhotoMatch|ObserveRepository|HashRepositoryNode|DrainRepositoryOutbox|ScheduleRepositoryScans|EventRebuild|MetadataWorker|ThumbnailWorker|TranscodeWorker)\b`)
	legacyKinds := regexp.MustCompile(`(?i)\b(?:process_semantic|process_bioclip|process_ocr|process_face|process_video_frames|classify_zeroshot|process_phash|detect_stacks|match_live_photo|observe_repository|hash_repository_node|drain_repository_outbox|schedule_repository_scans|rebuild_events|metadata_asset|thumbnail_asset|transcode_asset)\b`)
	violations, err := scanTextPaths(root, []string{"server/internal/queue/jobs", "server/internal/queue", "server/app"}, func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		return legacy.MatchString(line) || legacyKinds.MatchString(line)
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("legacy fine-grained queue contract remains reachable:\n%s", strings.Join(violations, "\n"))
	}

	allowedKinds := map[string]bool{
		"IngestAsset": true, "AnalyzeAsset": true,
		"GenerateAssetDerivatives": true, "TranscodeMedia": true,
		"EnrichAsset": true, "ScanRepositoryBatch": true,
		"RebuildProjectionBatch": true, "BackupCatalog": true,
	}
	registrations, err := scanGoLines(root, "server/app", func(_ string, line string) bool {
		return strings.Contains(line, "river.AddWorker[")
	})
	if err != nil {
		return err
	}
	registeredKinds := make(map[string]string, len(registrations))
	for _, registration := range registrations {
		start := strings.Index(registration, "river.AddWorker[") + len("river.AddWorker[")
		end := strings.Index(registration[start:], "]")
		if end < 0 {
			return fmt.Errorf("River registration has no closed payload type: %s", registration)
		}
		kind := strings.TrimPrefix(strings.TrimSpace(registration[start:start+end]), "jobs.")
		kind = strings.TrimSuffix(kind, "Args")
		if !allowedKinds[kind] {
			return fmt.Errorf("River registration is outside the closed macro catalog: %s", registration)
		}
		if previous, exists := registeredKinds[kind]; exists {
			return fmt.Errorf("River macro kind %s is registered twice:\n%s\n%s", kind, previous, registration)
		}
		registeredKinds[kind] = registration
	}
	if len(registeredKinds) != len(allowedKinds) {
		var missing []string
		for kind := range allowedKinds {
			if _, exists := registeredKinds[kind]; !exists {
				missing = append(missing, kind)
			}
		}
		return fmt.Errorf("closed River macro catalog has %d registrations, want %d; missing=%s", len(registeredKinds), len(allowedKinds), strings.Join(missing, ","))
	}

	appSource, err := os.ReadFile(filepath.Join(root, "server/app/app.go"))
	if err != nil {
		return fmt.Errorf("read app composition root: %w", err)
	}
	appText := string(appSource)
	for _, forbidden := range []string{
		"river.AddWorker[",
		"switch args.ProjectionKind",
		"ComputeMetadataTask(",
		"ComputeThumbnailTask(",
		"ProcessTranscodeTask(",
		"runAssetEnrichment(",
	} {
		if strings.Contains(appText, forbidden) {
			return fmt.Errorf("app.go owns macro scheduling algorithm %q; keep the composition root limited to construction and lifecycle", forbidden)
		}
	}

	forbiddenImports, err := scanGoLines(root, "server/internal", func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "server/internal/queue/") || strings.HasPrefix(relative, "server/internal/db/sqlitespike/") || strings.HasPrefix(relative, "server/internal/api/handler/queue_handler.go") {
			return false
		}
		return strings.Contains(line, "river_job") || strings.Contains(line, "river_jobs")
	})
	if err != nil {
		return err
	}
	if len(forbiddenImports) > 0 {
		return fmt.Errorf("product code consults River tables for correctness:\n%s", strings.Join(forbiddenImports, "\n"))
	}

	childInserts, err := scanGoLines(root, "server/internal/queue", func(relative, line string) bool {
		if relative == "server/internal/queue/scheduler.go" || strings.HasSuffix(relative, "_test.go") {
			return false
		}
		return strings.Contains(line, ".Insert(") || strings.Contains(line, ".InsertMany(")
	})
	if err != nil {
		return err
	}
	if len(childInserts) > 0 {
		return fmt.Errorf("queue worker inserts child River work:\n%s", strings.Join(childInserts, "\n"))
	}

	controllerWrites, err := scanROEControllerWriteCapabilities(root)
	if err != nil {
		return err
	}

	producerCapabilities, err := scanAsyncProducerCapabilities(root)
	if err != nil {
		return err
	}
	if len(producerCapabilities) > 0 {
		return fmt.Errorf(
			"asynchronous pipeline producers retain broad catalog/service capability:\n%s\nPreparers must own only narrow read/compute interfaces; catalog mutation belongs to coordinator handlers",
			strings.Join(producerCapabilities, "\n"),
		)
	}
	if len(controllerWrites) > 0 {
		return fmt.Errorf(
			"asynchronous ROE controller retains direct catalog-write capability:\n%s\nOnly foreground commands and the registered coordinator commit handler may own catalog writes",
			strings.Join(controllerWrites, "\n"),
		)
	}

	if source, readErr := os.ReadFile(filepath.Join(root, "server/internal/db/migration.go")); readErr != nil {
		return fmt.Errorf("read catalog migration boundary: %w", readErr)
	} else if strings.Contains(string(source), "MigrateWithPreflight") || strings.Contains(string(source), "DestructiveMigrationPreflight") {
		return errors.New("obsolete catalog migration preflight remains in the migration boundary")
	}

	scatter, err := scanResourceLiteralScatter(root)
	if err != nil {
		return err
	}
	if len(scatter) > 0 {
		return fmt.Errorf("pipeline runtime contains raw resource literals scatter:\n%s", strings.Join(scatter, "\n"))
	}

	govSites, err := scanGovernorConstructionSites(root)
	if err != nil {
		return err
	}
	if len(govSites) > 0 {
		return fmt.Errorf("unauthorized Governor construction sites:\n%s", strings.Join(govSites, "\n"))
	}

	nakedFlags, err := scanProcessorNakedFFmpegFlags(root)
	if err != nil {
		return err
	}
	if len(nakedFlags) > 0 {
		return fmt.Errorf("processor uses naked ffmpeg flags:\n%s", strings.Join(nakedFlags, "\n"))
	}

	return nil
}

func scanAsyncProducerCapabilities(root string) ([]string, error) {
	targets := map[string]bool{
		"server/internal/sourcing/materializer.go":          true,
		"server/internal/storage/roe/materializer/hash.go":  true,
		"server/internal/storage/roe/locations/resolver.go": true,
		"server/internal/processors/asset_processor.go":     true,
		"server/internal/queue/asset_work_fence.go":         true,
		"server/internal/queue/enrichment_runner.go":        true,
		"server/internal/queue/ml_image_loader.go":          true,
		"server/internal/queue/projection_preparer.go":      true,
	}
	forbiddenServiceTypes := map[string]bool{
		"SettingsService":      true,
		"LumenService":         true,
		"ClassifierService":    true,
		"LocationService":      true,
		"AssetIndexingService": true,
	}
	var violations []string
	for relative := range targets {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("stat async producer %s: %w", relative, err)
		}
		files := token.NewFileSet()
		parsed, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", relative, err)
		}
		aliases := map[string]string{}
		for _, imported := range parsed.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			alias := filepath.Base(importPath)
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			aliases[alias] = importPath
			if importPath == "server/internal/db" {
				position := files.Position(imported.Pos())
				violations = append(violations, fmt.Sprintf("%s:%d:imports catalog database capability", relative, position.Line))
			}
		}
		seen := map[token.Pos]bool{}
		ast.Inspect(parsed, func(node ast.Node) bool {
			if node == nil || seen[node.Pos()] {
				return true
			}
			var capability string
			switch value := node.(type) {
			case *ast.StarExpr:
				selector, ok := value.X.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				qualifier, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				switch aliases[qualifier.Name] {
				case "database/sql":
					if selector.Sel.Name == "DB" {
						capability = "*sql.DB"
					}
				case "server/internal/db/repo":
					if selector.Sel.Name == "Queries" {
						capability = "*repo.Queries"
					}
				}
			case *ast.SelectorExpr:
				qualifier, ok := value.X.(*ast.Ident)
				if !ok {
					return true
				}
				if aliases[qualifier.Name] == "server/internal/service" && forbiddenServiceTypes[value.Sel.Name] {
					capability = qualifier.Name + "." + value.Sel.Name
				}
			}
			if capability != "" {
				seen[node.Pos()] = true
				position := files.Position(node.Pos())
				violations = append(violations, fmt.Sprintf("%s:%d:owns broad capability %s", relative, position.Line, capability))
			}
			return true
		})
	}
	return violations, nil
}

func scanROEControllerWriteCapabilities(root string) ([]string, error) {
	const controllerDirectory = "server/internal/storage/roe/controller"
	allowed := map[string]bool{
		controllerDirectory + "/commands.go":              true,
		controllerDirectory + "/commit_handler.go":        true,
		controllerDirectory + "/directory_publication.go": true,
	}
	var violations []string
	err := filepath.WalkDir(filepath.Join(root, controllerDirectory), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if allowed[relative] {
			return nil
		}

		files := token.NewFileSet()
		parsed, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relative, err)
		}
		repoAliases := map[string]bool{}
		for _, imported := range parsed.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			position := files.Position(imported.Pos())
			switch importPath {
			case "server/internal/db", "server/internal/db/catalogtx":
				violations = append(violations, fmt.Sprintf("%s:%d:imports catalog write capability %s", relative, position.Line, importPath))
			case "server/internal/db/repo":
				alias := "repo"
				if imported.Name != nil {
					alias = imported.Name.Name
				}
				if alias == "." {
					violations = append(violations, fmt.Sprintf("%s:%d:dot-imports catalog query capability", relative, position.Line))
				} else {
					repoAliases[alias] = true
				}
			}
		}

		seen := map[token.Pos]bool{}
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || seen[selector.Pos()] {
				return true
			}
			capability := ""
			switch selector.Sel.Name {
			case "WithTx", "Writer", "Queries":
				capability = selector.Sel.Name
			}
			if qualifier, ok := selector.X.(*ast.Ident); ok && repoAliases[qualifier.Name] &&
				(selector.Sel.Name == "New" || selector.Sel.Name == "Queries") {
				capability = qualifier.Name + "." + selector.Sel.Name
			}
			if capability != "" {
				seen[selector.Pos()] = true
				position := files.Position(selector.Pos())
				violations = append(violations, fmt.Sprintf("%s:%d:references catalog write capability %s", relative, position.Line, capability))
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan asynchronous ROE controller capabilities: %w", err)
	}
	return violations, nil
}

func mixedSliceQueries(root string) ([]string, error) {
	files, err := gitFiles(root, "server/internal/db/repo/*.sql.go")
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, relative := range files {
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			// git ls-files includes a tracked file that is intentionally deleted
			// in the working tree. Generated query cleanup is part of destructive
			// schema changes, so do not make the architecture check require an
			// intermediate index/staging step before it can run.
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("open generated query %s: %w", relative, err)
		}
		scanner := bufio.NewScanner(file)
		inQuery := false
		queryStart := 0
		hasSlice := false
		hasNumbered := false
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if queryConstPattern.MatchString(line) {
				inQuery = true
				queryStart = lineNumber
				hasSlice = false
				hasNumbered = false
			}
			if inQuery && strings.Contains(line, "/*SLICE:") && strings.Contains(line, "*/") {
				hasSlice = true
			}
			if inQuery && numberedParameter.MatchString(line) {
				hasNumbered = true
			}
			if inQuery && line == string(rune(96)) {
				if hasSlice && hasNumbered {
					violations = append(violations, fmt.Sprintf("%s:%d", relative, queryStart))
				}
				inQuery = false
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("read generated query %s: %w", relative, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close generated query %s: %w", relative, err)
		}
	}
	return violations, nil
}

func checkBrowseArchitecture(root string) error {
	matches, err := gitGrep(
		root,
		"IsRaw|filter\\.raw[^[:alnum:]_]|(^|[^[:alnum:]_])raw\\?:[[:space:]]*boolean|(^|[^[:alnum:]_])raw:[[:space:]]*(true|false)|params\\.(get|set|has|delete)\\(\"raw\"",
		"server/internal/api",
		"server/internal/service",
		"server/internal/agent",
		"web/src/features/assets",
		"web/src/features/search",
		"web/src/features/studio",
	)
	if err != nil {
		return err
	}
	if filtered := excludeLines(matches, rawReference); filtered != "" {
		return fmt.Errorf(
			"Browse architecture check found a retired RAW filter reference:\n%s\nBrowse/filter/search work on media items: use media_item.composition, not a raw boolean.",
			filtered,
		)
	}

	matches, err = gitGrep(root, "StackMode", "server/internal/service/asset_search_fused.go")
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"Browse architecture check found stack_mode in the search pipeline:\n%s\nSearch results are never stack-collapsed.",
			strings.TrimSpace(matches),
		)
	}

	matches, err = gitGrep(root, "stack_mode", "web/src")
	if err != nil {
		return err
	}
	if filtered := excludeLines(matches, stackListExclusion); filtered != "" {
		return fmt.Errorf(
			"Browse architecture check found stack_mode outside the browse list request:\n%s\nOnly useAssetsList.ts may send stack_mode; search rejects it with 400.",
			filtered,
		)
	}

	matches, err = gitGrep(root, "json:\"(results_)?total_assets", "server/internal/api/dto")
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"Browse architecture check found a retired total_assets browse count:\n%s\nUse total_visible / total_media_items / total_files.",
			strings.TrimSpace(matches),
		)
	}
	return nil
}

func excludeLines(matches string, excluded *regexp.Regexp) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(matches), "\n") {
		if line != "" && !excluded.MatchString(line) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func checkDesktopArchitecture(root string) error {
	imports, err := scanGoLines(root, "desktop", func(relative, line string) bool {
		return strings.Contains(line, "\"server/") &&
			!strings.HasPrefix(relative, "desktop/internal/runtime/")
	})
	if err != nil {
		return err
	}
	if len(imports) > 0 {
		return fmt.Errorf(
			"Desktop architecture check found a server import outside internal/runtime:\n%s",
			strings.Join(imports, "\n"),
		)
	}

	serverImports, err := scanGoLines(root, "server", func(_ string, line string) bool {
		return strings.HasPrefix(strings.TrimSpace(line), "\"desktop/")
	})
	if err != nil {
		return err
	}
	if len(serverImports) > 0 {
		return errors.New("Desktop architecture check found a server -> desktop import.")
	}
	return nil
}

func checkRepositoryPathArchitecture(root string) error {
	directoryManagerSource, err := os.ReadFile(filepath.Join(root, "server/internal/storage/directory_manager.go"))
	if err != nil {
		return fmt.Errorf("read DirectoryManager boundary: %w", err)
	}
	for _, forbidden := range []string{
		"CreateTempFile(", "CleanupTempFiles(", "ReadSidecar(", "WriteSidecar(",
		"RepairStructure(", "MoveToTrash(", "ListTrashFiles(", "RecoverFromTrash(", "PurgeTrash(",
		"type DefaultDirectoryManager", "func NewDirectoryManager() *",
	} {
		if strings.Contains(string(directoryManagerSource), forbidden) {
			return fmt.Errorf("DirectoryManager exposes forbidden repository write bypass %q; use RepositoryManager/RepositoryFS", forbidden)
		}
	}
	storageBypasses, err := scanGoLines(root, "server/internal/storage", func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		trimmed := strings.TrimSpace(line)
		for _, method := range []string{
			"CreateTempFile", "CleanupTempFiles", "ReadSidecar", "WriteSidecar",
			"RepairStructure", "MoveToTrash", "ListTrashFiles", "RecoverFromTrash", "PurgeTrash",
		} {
			if strings.HasPrefix(trimmed, "func ") && strings.Contains(trimmed, ") "+method+"(") {
				return true
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	if len(storageBypasses) > 0 {
		return fmt.Errorf("Repository path architecture check found exported raw-path destructive storage methods:\n%s", strings.Join(storageBypasses, "\n"))
	}
	violations, err := scanGoLines(root, "server", func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "server/internal/storage/") {
			return false
		}
		if !strings.Contains(line, "filepath.Join(") {
			return false
		}
		return strings.Contains(line, "repository.Path") ||
			strings.Contains(line, "repo.Path") ||
			strings.Contains(line, "repoPath") ||
			strings.Contains(line, "repositoryPath")
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Repository path architecture check found a direct repository-root join outside storage:\n%s\nUse RepositoryFS; native tools may use its explicit local-path adapter.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func checkUserFacingTerminology(root string) error {
	paths := []string{
		"README.md", "README.en.md",
		"web/src", "desktop/frontend/src",
		"site/docs/en", "site/docs/zh-cn",
		"server/internal/api/dto", "server/internal/api/handler", "server/docs",
	}
	violations, err := scanTextPaths(root, paths, func(relative, line string) bool {
		return userFacingTerminologyViolation(relative, line)
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"User-facing terminology check found retired Repository/Storage Location or Lumen capability labels:\n%s\nUse Repository/资源库, Storage Location/存储位置, and the canonical capability labels from AGENTS.md.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func checkAPIProblemArchitecture(root string) error {
	serverViolations, err := scanTextPaths(root, []string{"server/internal/api"}, func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		for _, retired := range []string{
			"ErrorResponse", "GinError", "GinBadRequest", "GinUnauthorized", "GinForbidden",
			"GinNotFound", "GinInternalError", "HandleError",
		} {
			if strings.Contains(line, retired) {
				return true
			}
		}
		if strings.Contains(line, "c.JSON(") && !successJSONWrite(line) {
			return true
		}
		if strings.Contains(line, problem.Namespace) && relative != "server/internal/api/problem/problem.go" {
			return true
		}
		if strings.Contains(line, ".Error()") && !allowedPrivateErrorUse(relative, line) {
			return true
		}
		return false
	})
	if err != nil {
		return err
	}
	if len(serverViolations) > 0 {
		return fmt.Errorf(
			"API Problem check found a legacy responder, direct non-success JSON writer, unregistered URI literal, or raw error use:\n%s\nUse the closed Problem registry and api.WriteProblem; keep private causes out of public payloads.",
			strings.Join(serverViolations, "\n"),
		)
	}

	if err := checkProblemCatalogArtifacts(root); err != nil {
		return err
	}
	if err := checkWebProblemBoundary(root); err != nil {
		return err
	}
	return nil
}

func successJSONWrite(line string) bool {
	for _, status := range []string{
		"http.StatusOK", "http.StatusCreated", "http.StatusAccepted", "http.StatusNoContent",
		"http.StatusPartialContent", "200", "201", "202", "203", "204", "205", "206",
	} {
		if strings.Contains(line, "c.JSON("+status) {
			return true
		}
	}
	return false
}

func allowedPrivateErrorUse(relative, line string) bool {
	if relative == "server/internal/api/problem/problem.go" && strings.Contains(line, "panic(") {
		return true
	}
	// Upload-session manifests are private recovery state. The public progress
	// DTO deliberately omits this diagnostic and exposes only the stable state.
	return relative == "server/internal/api/handler/asset_handler.go" && strings.Contains(line, "SetSessionError(")
}

func checkProblemCatalogArtifacts(root string) error {
	swagger, err := os.ReadFile(filepath.Join(root, "server/docs/swagger.yaml"))
	if err != nil {
		return fmt.Errorf("read generated OpenAPI Problem contract: %w", err)
	}
	webSource, err := os.ReadFile(filepath.Join(root, "web/src/lib/http-commons/problem.ts"))
	if err != nil {
		return fmt.Errorf("read Web Problem catalog: %w", err)
	}
	for _, descriptor := range problem.Registered() {
		path := strings.TrimPrefix(descriptor.Type, problem.Namespace)
		docPath := filepath.Join(root, "site/docs/public/problems", filepath.FromSlash(path), "index.html")
		doc, err := os.ReadFile(docPath)
		if err != nil {
			return fmt.Errorf("Problem type %s has no generated public page at %s: %w", descriptor.Type, docPath, err)
		}
		if !strings.Contains(string(doc), descriptor.Type) || !strings.Contains(string(doc), descriptor.Definition) {
			return fmt.Errorf("generated Problem page for %s does not match its registry descriptor", descriptor.Type)
		}
		if strings.Count(string(swagger), "const: "+descriptor.Type) < 2 {
			return fmt.Errorf("generated OpenAPI lacks exact HTTP and Reference schemas for Problem type %s", descriptor.Type)
		}
		knownLiteral := fmt.Sprintf("%q: true", descriptor.Type)
		caseLiteral := fmt.Sprintf("case %q:", descriptor.Type)
		if !strings.Contains(string(webSource), knownLiteral) || !strings.Contains(string(webSource), caseLiteral) {
			return fmt.Errorf("Web Problem catalog is not exhaustive for generated type %s", descriptor.Type)
		}
	}
	return nil
}

func checkWebProblemBoundary(root string) error {
	violations, err := scanTextPaths(root, []string{"web/src"}, func(relative, line string) bool {
		if strings.HasSuffix(relative, ".test.ts") || strings.HasSuffix(relative, ".test.tsx") ||
			strings.HasSuffix(relative, ".worker.ts") || relative == "web/src/lib/http-commons/problem.ts" ||
			relative == "web/src/lib/http-commons/schema.d.ts" {
			return false
		}
		if strings.Contains(line, `"error" in payload`) || strings.Contains(line, `"message" in payload`) {
			return true
		}
		if strings.Contains(line, "failed with status ${response.status}") ||
			strings.Contains(line, "failed with ${response.status}") {
			return true
		}
		if strings.Contains(line, "apiErrors") &&
			(strings.Contains(line, "${") || strings.Contains(line, ".replace(") || strings.Contains(line, "+")) {
			return true
		}
		return privateProblemHelper.MatchString(strings.TrimSpace(line))
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Web Problem check found a private legacy parser, status-authored Error, dynamic API-error key, or feature-owned Problem normalizer:\n%s\nPreserve structured failures through lib/http-commons/problem and localize only at the presentation boundary.",
			strings.Join(violations, "\n"),
		)
	}

	source, err := os.ReadFile(filepath.Join(root, "web/src/lib/http-commons/problem.ts"))
	if err != nil {
		return err
	}
	keys := make(map[string]struct{})
	for _, match := range apiErrorKeyPattern.FindAllStringSubmatch(string(source), -1) {
		keys[match[1]] = struct{}{}
	}
	if len(keys) == 0 {
		return errors.New("Web Problem catalog has no literal apiErrors translation keys")
	}
	for _, language := range []string{"en", "zh"} {
		catalogPath := filepath.Join(root, "web/src/locales", language, "translation.json")
		catalog, err := readJSONCatalog(catalogPath)
		if err != nil {
			return err
		}
		for key := range keys {
			if !hasTranslation(catalog, key) {
				return fmt.Errorf("Web Problem translation %s is missing from %s", key, catalogPath)
			}
		}
	}
	return nil
}

func readJSONCatalog(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read translation catalog %s: %w", path, err)
	}
	var catalog map[string]any
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parse translation catalog %s: %w", path, err)
	}
	return catalog, nil
}

func hasTranslation(catalog map[string]any, dottedKey string) bool {
	parts := strings.Split(dottedKey, ".")
	current := catalog
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	leaf := parts[len(parts)-1]
	if value, ok := current[leaf].(string); ok && strings.TrimSpace(value) != "" {
		return true
	}
	for key, value := range current {
		if strings.HasPrefix(key, leaf+"_") {
			if translation, ok := value.(string); ok && strings.TrimSpace(translation) != "" {
				return true
			}
		}
	}
	return false
}

func userFacingTerminologyViolation(relative, line string) bool {
	if strings.Contains(relative, ".test.") || strings.HasSuffix(relative, "_test.go") {
		return false
	}
	if retiredRepositoryTerms.MatchString(line) && !allowedRepositoryTermContext(relative, line) {
		return true
	}
	if filepath.Ext(relative) != ".md" && retiredCapabilityLabels.MatchString(withoutCanonicalCapabilityLabels(line)) {
		return true
	}
	return filepath.Ext(relative) == ".md" && retiredMarkdownCapabilityLabel(relative, line)
}

// allowedRepositoryTermContext keeps platform and source-code vocabulary from
// being confused with the Repository product entity. These are deliberately
// narrow: adding another exception requires naming the technical context.
func allowedRepositoryTermContext(relative, line string) bool {
	lower := strings.ToLower(line)
	trimmed := strings.TrimSpace(line)
	if (strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, ".ts") || strings.HasSuffix(relative, ".tsx")) &&
		(strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")) &&
		!strings.HasPrefix(trimmed, "// @") && !strings.Contains(relative, "schema.d.ts") &&
		!strings.HasSuffix(relative, "/doc.ts") {
		return true
	}
	if strings.HasSuffix(relative, ".go") && !strings.Contains(line, `"`) && !strings.HasPrefix(trimmed, "// @") {
		return true
	}
	if strings.Contains(lower, "/flows/library/") {
		return true
	}
	for _, allowed := range []string{
		"~/library/", "/library/application support", "library/application support",
		"library.sqlite", "library-manifest", "library manifest", "library identity",
		"source repository", "code repository", "project repository", "git repository",
		"repository source", "repository checkout", "repository convention", "repository map",
		"component library", "icon library", "media library api", "photo library api",
		"代码仓库", "源码仓库", "项目仓库", "git 仓库", "仓库根目录", "跨仓库",
		"复制仓库中的", "公开仓库", "仓库声明的", "主仓库通过", "仓库约定",
		"仓库与组件地图", "仓库中的 `", "相邻仓库", "上游仓库", "仓库权威指南",
	} {
		if strings.Contains(lower, allowed) {
			return true
		}
	}
	return false
}

func retiredMarkdownCapabilityLabel(relative, line string) bool {
	trimmed := strings.TrimSpace(line)
	retiredText := withoutCanonicalCapabilityLabels(trimmed)
	labels := []string{
		"Semantic Search", "Face Recognition", "OCR", "Species Recognition",
		"语义搜索", "人脸识别", "物种识别",
	}
	for _, label := range labels {
		if strings.EqualFold(strings.Trim(retiredText, "#*|-：: `\t"), label) {
			return true
		}
		if strings.Contains(retiredText, "| "+label+" |") || strings.Contains(retiredText, "**"+label+"**") {
			return true
		}
	}
	return false
}

func withoutCanonicalCapabilityLabels(line string) string {
	for _, label := range []string{
		"Image Semantic Analysis", "Person Recognition", "OCR Text Recognition", "BioCLIP Species Recognition",
		"图像语义分析", "人物识别", "OCR文字识别", "BioCLIP物种识别",
	} {
		line = strings.ReplaceAll(line, label, "")
	}
	return line
}

func scanTextPaths(root string, paths []string, match func(relative, line string) bool) ([]string, error) {
	allowedExtensions := map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".json": true,
		".yaml": true, ".yml": true, ".md": true, ".html": true,
	}
	var matches []string
	for _, relativeRoot := range paths {
		pathRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		err := filepath.WalkDir(pathRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" || entry.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !allowedExtensions[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(file)
			lineNumber := 0
			for scanner.Scan() {
				lineNumber++
				if match(relative, scanner.Text()) {
					matches = append(matches, fmt.Sprintf("%s:%d:%s", relative, lineNumber, scanner.Text()))
				}
			}
			scanErr := scanner.Err()
			closeErr := file.Close()
			if scanErr != nil {
				return scanErr
			}
			return closeErr
		})
		if err != nil {
			return nil, fmt.Errorf("scan user-facing terminology in %s: %w", relativeRoot, err)
		}
	}
	return matches, nil
}

func scanGoLines(root, directory string, match func(relative, line string) bool) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if match(relative, line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", relative, lineNumber, line))
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return scanErr
		}
		return closeErr
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s Go files: %w", directory, err)
	}
	return matches, nil
}

func isResourceLiteralScatter(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") ||
		relative == "server/internal/execution/demand.go" ||
		relative == "server/internal/execution/budget.go" ||
		strings.HasPrefix(relative, "server/tools/") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return false
	}
	return strings.Contains(line, "execution.Resources{") ||
		(strings.HasPrefix(relative, "server/internal/execution/") && strings.Contains(line, "Resources{") && !strings.Contains(line, "func ") && !strings.Contains(line, "type ") && !strings.Contains(line, "return Resources{"))
}

func isGovernorConstructionViolation(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") ||
		relative == "server/internal/execution/budget.go" ||
		relative == "server/internal/execution/governor.go" ||
		strings.HasPrefix(relative, "server/tools/") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return false
	}
	return strings.Contains(line, "execution.NewGovernor(") ||
		(strings.HasPrefix(relative, "server/internal/execution/") && strings.Contains(line, "NewGovernor(") && !strings.Contains(line, "func NewGovernor("))
}

func isProcessorNakedFFmpegFlagViolation(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") ||
		!strings.HasPrefix(relative, "server/internal/processors/") ||
		strings.HasPrefix(relative, "server/tools/") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return false
	}
	return strings.Contains(line, `"-threads"`) || strings.Contains(line, `"-preset"`)
}

func scanResourceLiteralScatter(root string) ([]string, error) {
	return scanGoLines(root, "server", isResourceLiteralScatter)
}

func scanGovernorConstructionSites(root string) ([]string, error) {
	return scanGoLines(root, "server", isGovernorConstructionViolation)
}

func scanProcessorNakedFFmpegFlags(root string) ([]string, error) {
	return scanGoLines(root, "server/internal/processors", isProcessorNakedFFmpegFlagViolation)
}
