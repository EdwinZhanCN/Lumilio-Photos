package controller

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/config"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/roe/changefeed"
	"server/internal/storage/roe/nodegraph"
	"server/internal/storage/roe/outbox"
)

type generatedDirectorySource struct {
	entries            int
	directories        int
	fileBytes          int64
	readCalls          int64
	entriesReturned    int64
	maxReturned        int
	directoryReadCalls map[string]int
	mutated            map[int]int
	firstDirectoryName string
}

func newGeneratedDirectorySource(entries, directories int) *generatedDirectorySource {
	return &generatedDirectorySource{
		entries: entries, directories: directories, fileBytes: 1 << 20,
		directoryReadCalls: make(map[string]int), mutated: make(map[int]int),
		firstDirectoryName: "directory-000000",
	}
}

func (source *generatedDirectorySource) ReadDirectory(
	ctx context.Context,
	repository repo.Repository,
	options storage.DirectoryReadOptions,
) (storage.DirectoryReadBatch, error) {
	batch := storage.DirectoryReadBatch{Authoritative: true, NextOffset: options.Offset}
	if err := ctx.Err(); err != nil {
		return batch, err
	}
	if options.Offset < 0 || options.Limit <= 0 || options.Limit > 256 {
		return batch, fmt.Errorf("invalid generated directory page offset=%d limit=%d", options.Offset, options.Limit)
	}
	source.readCalls++
	source.directoryReadCalls[options.Directory]++
	start := int(options.Offset)
	if options.Directory == "" {
		end := min(source.directories, start+options.Limit)
		if start >= source.directories {
			batch.Done = true
			return batch, nil
		}
		batch.Entries = make([]storage.DirectoryReadEntry, 0, end-start)
		for index := start; index < end; index++ {
			name := fmt.Sprintf("directory-%06d", index)
			if index == 0 {
				name = source.firstDirectoryName
			}
			observation, err := generatedObservation(
				repository.RepoID,
				options.ScanID,
				name,
				storage.EntryKindDirectory,
				0,
				fmt.Sprintf("volume0:directory-%d", index),
				1,
			)
			if err != nil {
				return batch, err
			}
			batch.Entries = append(batch.Entries, storage.DirectoryReadEntry{
				Observation: observation, NextOffset: int64(index + 1),
			})
		}
		batch.NextOffset = int64(end)
		batch.Done = end == source.directories
		source.recordBatch(len(batch.Entries))
		return batch, nil
	}

	directoryIndex, err := source.directoryIndex(options.Directory)
	if err != nil {
		return batch, err
	}
	fileCount := source.filesInDirectory(directoryIndex)
	end := min(fileCount, start+options.Limit)
	if start >= fileCount {
		batch.Done = true
		return batch, nil
	}
	batch.Entries = make([]storage.DirectoryReadEntry, 0, end-start)
	for localIndex := start; localIndex < end; localIndex++ {
		fileOrdinal := directoryIndex + localIndex*source.directories
		filename := fmt.Sprintf("media-%09d.jpg", fileOrdinal)
		version := 1 + source.mutated[fileOrdinal]
		observation, err := generatedObservation(
			repository.RepoID,
			options.ScanID,
			path.Join(options.Directory, filename),
			storage.EntryKindRegular,
			source.fileBytes,
			fmt.Sprintf("generated-file-%d", fileOrdinal),
			version,
		)
		if err != nil {
			return batch, err
		}
		batch.Entries = append(batch.Entries, storage.DirectoryReadEntry{
			Observation: observation, NextOffset: int64(localIndex + 1),
		})
	}
	batch.NextOffset = int64(end)
	batch.Done = end == fileCount
	source.recordBatch(len(batch.Entries))
	return batch, nil
}

func (source *generatedDirectorySource) directoryIndex(relative string) (int, error) {
	if relative == source.firstDirectoryName {
		return 0, nil
	}
	if !strings.HasPrefix(relative, "directory-") || strings.Contains(relative, "/") {
		return 0, fmt.Errorf("unknown generated directory %q", relative)
	}
	index, err := strconv.Atoi(strings.TrimPrefix(relative, "directory-"))
	if err != nil || index < 0 || index >= source.directories {
		return 0, fmt.Errorf("invalid generated directory %q", relative)
	}
	return index, nil
}

func (source *generatedDirectorySource) filesInDirectory(directory int) int {
	files := source.entries - source.directories
	if directory >= files {
		return 0
	}
	return (files-1-directory)/source.directories + 1
}

func (source *generatedDirectorySource) recordBatch(entries int) {
	source.entriesReturned += int64(entries)
	if entries > source.maxReturned {
		source.maxReturned = entries
	}
}

func generatedObservation(
	repositoryID uuid.UUID,
	scanID uuid.UUID,
	relative string,
	kind storage.EntryKind,
	size int64,
	identity string,
	version int,
) (storage.FileObservation, error) {
	repositoryPath, err := storage.ParseUserMediaPath(relative)
	if err != nil {
		return storage.FileObservation{}, err
	}
	identityKind := "generated"
	changedAt := int64(version)
	return storage.FileObservation{
		RepositoryID: repositoryID, Path: repositoryPath, EntryKind: kind,
		Size: size, ModTimeNS: int64(version), ChangeTimeNS: &changedAt,
		FileIdentityKind: &identityKind, FileIdentity: &identity,
		ObservationToken: fmt.Sprintf("generated-token:%s:%d", identity, version), ScanID: scanID,
	}, nil
}

type generatedControllerFixture struct {
	ctx        context.Context
	database   *db.DB
	repository repo.Repository
	controller *Controller
	feed       *deterministicFeed
	source     *generatedDirectorySource
}

func newGeneratedControllerFixture(tb testing.TB, entries, directories int) *generatedControllerFixture {
	tb.Helper()
	ctx := context.Background()
	databaseDirectory := tb.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		tb.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(databaseDirectory, "catalog.sqlite3")})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		tb.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "scale-owner", Password: "unused", DisplayName: "Scale Owner",
		Role: "admin", WebauthnUserHandle: []byte("scale-owner-handle"),
	})
	if err != nil {
		tb.Fatal(err)
	}
	rootID := uuid.New()
	repositoryID := uuid.New()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rootPath := tb.TempDir()
	repositoryPath := filepath.Join(rootPath, "repository")
	if err := os.Mkdir(repositoryPath, 0o700); err != nil {
		tb.Fatal(err)
	}
	if _, err := database.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "generated root", Path: rootPath,
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		tb.Fatal(err)
	}
	repositoryConfig := repocfg.NewRepositoryConfig("generated repository")
	repositoryConfig.ID = repositoryID.String()
	repository, err := database.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "generated repository", Path: repositoryPath,
		Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle, DefaultOwnerID: &owner.UserID,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		tb.Fatal(err)
	}
	feed := &deterministicFeed{}
	source := newGeneratedDirectorySource(entries, directories)
	controller := New(database, nil, Config{
		BatchSize: 48, OutboxHighWaterMark: 256,
		ChangeFeed: feed, directorySource: source,
	}, zap.NewNop())
	return &generatedControllerFixture{
		ctx: ctx, database: database, repository: repository,
		controller: controller, feed: feed, source: source,
	}
}

type generatedProfile struct {
	entries             int
	turns               int
	directoryReads      int64
	maxSourceBatch      int
	maxRows             int
	p99Transaction      time.Duration
	p99Commit           time.Duration
	additionalHeapBytes uint64
	elapsed             time.Duration
}

func runGeneratedCrawlProfile(tb testing.TB, entries, directories int) (generatedProfile, *generatedControllerFixture) {
	tb.Helper()
	fixture := newGeneratedControllerFixture(tb, entries, directories)
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	maxHeap := baseline.Alloc
	started := time.Now()
	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "scale", true)
	if err != nil {
		tb.Fatal(err)
	}
	drainer := outbox.New(fixture.database, outbox.Config{BatchSize: 64})
	profile := generatedProfile{entries: entries}
	transactionDurations := make([]time.Duration, 0, entries/32)
	commitDurations := make([]time.Duration, 0, entries/32)
	downstream := 0
	progressive := false
	var run repo.RepositoryScanRun
	for turn := 0; turn < entries*2+directories*4+1000; turn++ {
		result, turnErr := fixture.controller.RunTurn(
			fixture.ctx,
			fixture.repository.RepoID,
			receipt.OperationID,
		)
		if turnErr != nil {
			tb.Fatalf("generated crawl turn %d: %v", turn, turnErr)
		}
		profile.turns++
		if result.RowsApplied > profile.maxRows {
			profile.maxRows = result.RowsApplied
		}
		if result.TransactionDuration > 0 {
			transactionDurations = append(transactionDurations, result.TransactionDuration)
		}
		if result.CommitDuration > 0 {
			commitDurations = append(commitDurations, result.CommitDuration)
		}
		drainResult, drainErr := drainer.DrainKind(fixture.ctx, "hash", func(context.Context, repo.RepositoryOutbox) error {
			downstream++
			return nil
		})
		if drainErr != nil {
			tb.Fatal(drainErr)
		}
		if drainResult.Claimed > 0 && result.HasMore {
			progressive = true
		}
		if turn%32 == 0 {
			var current runtime.MemStats
			runtime.ReadMemStats(&current)
			if current.Alloc > maxHeap {
				maxHeap = current.Alloc
			}
		}
		if !result.HasMore {
			run, err = fixture.database.ReaderQueries.GetRepositoryScanRun(fixture.ctx, repo.GetRepositoryScanRunParams{
				RepositoryID: fixture.repository.RepoID, RunID: receipt.OperationID,
			})
			if err != nil {
				tb.Fatal(err)
			}
			break
		}
		if turn == entries*2+directories*4+999 {
			tb.Fatal("generated crawl did not terminate")
		}
	}
	profile.elapsed = time.Since(started)
	profile.directoryReads = fixture.source.readCalls
	profile.maxSourceBatch = fixture.source.maxReturned
	if maxHeap > baseline.Alloc {
		profile.additionalHeapBytes = maxHeap - baseline.Alloc
	}
	profile.p99Transaction = durationP99(transactionDurations)
	profile.p99Commit = durationP99(commitDurations)

	wantFiles := int64(entries - directories)
	if run.Status != StatusCompleted || run.FilesObserved != wantFiles || run.DirectoriesObserved != int64(directories) {
		tb.Fatalf("generated run = %+v, want completed files=%d directories=%d", run, wantFiles, directories)
	}
	if downstream != entries-directories || !progressive {
		tb.Fatalf("downstream hash delivery=%d progressive=%t, want %d/true", downstream, progressive, entries-directories)
	}
	if profile.maxSourceBatch > 48 || profile.maxRows > 256 {
		tb.Fatalf("bounded profile source_batch=%d transaction_rows=%d", profile.maxSourceBatch, profile.maxRows)
	}
	if profile.additionalHeapBytes >= 256<<20 {
		tb.Fatalf("generated crawl additional Go heap = %.1f MiB, want <256 MiB", float64(profile.additionalHeapBytes)/(1<<20))
	}
	readsBefore := fixture.source.readCalls
	var effectsBefore int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'
	`).Scan(&effectsBefore); err != nil {
		tb.Fatal(err)
	}
	incremental, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "periodic", "", false)
	if err != nil {
		tb.Fatal(err)
	}
	for turn := 0; turn < 10; turn++ {
		result, turnErr := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, incremental.OperationID)
		if turnErr != nil {
			tb.Fatal(turnErr)
		}
		if !result.HasMore {
			break
		}
		if turn == 9 {
			tb.Fatal("zero-change incremental pass did not terminate")
		}
	}
	var effectsAfter int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'
	`).Scan(&effectsAfter); err != nil {
		tb.Fatal(err)
	}
	if fixture.source.readCalls != readsBefore || effectsAfter != effectsBefore {
		tb.Fatalf("zero-change pass read directories %d->%d or hash effects %d->%d",
			readsBefore, fixture.source.readCalls, effectsBefore, effectsAfter)
	}
	return profile, fixture
}

func assertGeneratedCrawlTransactionBudget(tb testing.TB, profile generatedProfile) {
	tb.Helper()
	if profile.p99Transaction > 25*time.Millisecond {
		tb.Fatalf("generated crawl transaction p99 = %s, want <=25ms", profile.p99Transaction)
	}
}

func durationP99(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := (len(values)*99 + 99) / 100
	if index > 0 {
		index--
	}
	return values[index]
}

func bindGeneratedLocations(tb testing.TB, fixture *generatedControllerFixture) {
	tb.Helper()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	err := fixture.database.WithTx(fixture.ctx, catalogtx.OperationRepositoryMaterializeHash, func(tx *sql.Tx, queries *repo.Queries) error {
		content, err := queries.InsertContentObject(fixture.ctx, repo.InsertContentObjectParams{
			ContentID: uuid.New(), HashAlgorithm: "blake3-v1",
			FullHash: strings.Repeat("a", 64), FileSize: fixture.source.fileBytes, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		assetID := uuid.New()
		if _, err := queries.InsertOwnerContentAsset(fixture.ctx, repo.InsertOwnerContentAssetParams{
			AssetID: assetID, OwnerID: fixture.repository.DefaultOwnerID, ContentID: content.ContentID,
			Type: string(dbtypes.AssetTypePhoto), OriginalFilename: "generated.jpg", MimeType: "image/jpeg",
			UploadTime: now, TakenTime: now, Rating: int64Ptr(0),
			Status: dbtypes.JSON(`{"state":"completed"}`), UpdatedAt: now,
		}); err != nil {
			return err
		}
		_, err = tx.ExecContext(fixture.ctx, `
			INSERT INTO asset_locations (
			  location_id, node_id, asset_id, bound_observation_revision,
			  unbound_observation_revision, created_at, updated_at
			)
			SELECT node_id, node_id, ?, observation_revision, NULL, ?, ?
			FROM repository_nodes
			WHERE repository_id = ? AND lifecycle = 'active' AND kind = 'file'
		`, assetID, now, now, fixture.repository.RepoID)
		return err
	})
	if err != nil {
		tb.Fatal(err)
	}
}

func TestGeneratedCrawlUsesBoundedBatchesAndCursorOnlyIncrementalPass(t *testing.T) {
	profile, _ := runGeneratedCrawlProfile(t, 10_000, 128)
	assertGeneratedCrawlTransactionBudget(t, profile)
	if profile.entries != 10_000 || profile.directoryReads == 0 {
		t.Fatalf("generated profile = %+v", profile)
	}
}

func TestPendingOutboxDepthUsesLiveRepositoryIndex(t *testing.T) {
	fixture := newGeneratedControllerFixture(t, 100, 4)
	rows, err := fixture.database.ReaderSQL.QueryContext(fixture.ctx, `
		EXPLAIN QUERY PLAN
		SELECT count(*) FROM repository_outbox
		WHERE repository_id = ? AND status IN ('pending', 'delivering')
	`, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan += detail + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "idx_repository_outbox_pending_repository") {
		t.Fatalf("pending outbox query plan did not use live-row index:\n%s", plan)
	}
}

func BenchmarkGeneratedRepositoryCrawl(b *testing.B) {
	for _, entries := range []int{100_000, 500_000} {
		b.Run(fmt.Sprintf("entries-%dk", entries/1000), func(b *testing.B) {
			for range b.N {
				profile, _ := runGeneratedCrawlProfile(b, entries, 2_048)
				assertGeneratedCrawlTransactionBudget(b, profile)
				b.ReportMetric(float64(profile.additionalHeapBytes)/(1<<20), "additional-heap-MiB")
				b.ReportMetric(float64(profile.p99Transaction.Microseconds())/1000, "tx-p99-ms")
				b.ReportMetric(float64(profile.p99Commit.Microseconds())/1000, "commit-p99-ms")
				b.ReportMetric(float64(profile.maxSourceBatch), "max-source-rows")
				b.ReportMetric(float64(profile.turns), "controller-turns")
				b.ReportMetric(float64(entries)/profile.elapsed.Seconds(), "entries/s")
			}
		})
	}
}

func BenchmarkGeneratedRepositoryOnePercentChange(b *testing.B) {
	for range b.N {
		const entries = 100_000
		const directories = 2_048
		// The setup intentionally uses the production crawl path, but its
		// latency budget belongs to BenchmarkGeneratedRepositoryCrawl. Keeping
		// that independent assertion inside this setup made host contention
		// abort the benchmark before it could measure changed-path cardinality.
		_, fixture := runGeneratedCrawlProfile(b, entries, directories)
		bindGeneratedLocations(b, fixture)
		files := entries - directories
		changed := files / 100
		for index := 0; index < changed; index++ {
			fileOrdinal := index * 100
			fixture.source.mutated[fileOrdinal]++
			directory := fileOrdinal % directories
			filename := fmt.Sprintf("media-%09d.jpg", fileOrdinal)
			fixture.feed.publish(
				changefeed.EventModify,
				path.Join(fmt.Sprintf("directory-%06d", directory), filename),
				"",
				false,
			)
		}
		readsBefore := fixture.source.readCalls
		entriesBefore := fixture.source.entriesReturned
		started := time.Now()
		receipt, err := fixture.controller.Request(
			fixture.ctx,
			fixture.repository.RepoID,
			"watcher",
			"",
			false,
		)
		if err != nil {
			b.Fatal(err)
		}
		drainer := outbox.New(fixture.database, outbox.Config{BatchSize: 64})
		hashes := 0
		var run repo.RepositoryScanRun
		for turn := 0; turn < entries; turn++ {
			result, turnErr := fixture.controller.RunTurn(
				fixture.ctx,
				fixture.repository.RepoID,
				receipt.OperationID,
			)
			if turnErr != nil {
				b.Fatal(turnErr)
			}
			if _, drainErr := drainer.DrainKind(fixture.ctx, "hash", func(context.Context, repo.RepositoryOutbox) error {
				hashes++
				return nil
			}); drainErr != nil {
				b.Fatal(drainErr)
			}
			if !result.HasMore {
				run, err = fixture.database.ReaderQueries.GetRepositoryScanRun(fixture.ctx, repo.GetRepositoryScanRunParams{
					RepositoryID: fixture.repository.RepoID, RunID: receipt.OperationID,
				})
				if err != nil {
					b.Fatal(err)
				}
				break
			}
			if turn == entries-1 {
				b.Fatal("one-percent change profile did not terminate")
			}
		}
		elapsed := time.Since(started)
		metadataEntries := fixture.source.entriesReturned - entriesBefore
		directoryReads := fixture.source.readCalls - readsBefore
		if run.Status != StatusCompleted || hashes != changed {
			b.Fatalf("one-percent run=%+v hashes=%d want=%d", run, hashes, changed)
		}
		if metadataEntries <= int64(changed) || metadataEntries >= int64(entries) {
			b.Fatalf("one-percent metadata work=%d, want changed-path bounded below full tree", metadataEntries)
		}
		b.ReportMetric(float64(changed), "changed-files")
		b.ReportMetric(float64(hashes), "full-hash-effects")
		b.ReportMetric(float64(metadataEntries), "metadata-entries")
		b.ReportMetric(float64(directoryReads), "directory-pages")
		b.ReportMetric(elapsed.Seconds()*1000, "incremental-ms")
	}
}

func BenchmarkGeneratedDirectoryRename50k(b *testing.B) {
	for range b.N {
		const descendants = 50_000
		_, fixture := runGeneratedCrawlProfile(b, descendants+1, 1)
		var descendantID uuid.UUID
		if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
			SELECT node_id FROM repository_nodes
			WHERE name = 'media-000000000.jpg' AND lifecycle = 'active'
		`).Scan(&descendantID); err != nil {
			b.Fatal(err)
		}
		var hashEffectsBefore int
		if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
			SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'
		`).Scan(&hashEffectsBefore); err != nil {
			b.Fatal(err)
		}
		readsBefore := fixture.source.readCalls
		fixture.source.firstDirectoryName = "renamed"
		fixture.feed.publish(changefeed.EventRename, "renamed", "directory-000000", false)
		started := time.Now()
		receipt, err := fixture.controller.Request(
			fixture.ctx,
			fixture.repository.RepoID,
			"watcher",
			"",
			false,
		)
		if err != nil {
			b.Fatal(err)
		}
		var run repo.RepositoryScanRun
		for turn := 0; turn < 100; turn++ {
			result, turnErr := fixture.controller.RunTurn(
				fixture.ctx,
				fixture.repository.RepoID,
				receipt.OperationID,
			)
			if turnErr != nil {
				b.Fatal(turnErr)
			}
			if !result.HasMore {
				run, err = fixture.database.ReaderQueries.GetRepositoryScanRun(fixture.ctx, repo.GetRepositoryScanRunParams{
					RepositoryID: fixture.repository.RepoID, RunID: receipt.OperationID,
				})
				if err != nil {
					b.Fatal(err)
				}
				break
			}
			if turn == 99 {
				b.Fatal("50k directory rename did not terminate")
			}
		}
		projected, err := nodegraph.ProjectPath(
			fixture.ctx,
			fixture.database.ReaderQueries,
			fixture.repository.RepoID,
			descendantID,
		)
		if err != nil {
			b.Fatal(err)
		}
		var hashEffectsAfter int
		if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
			SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'
		`).Scan(&hashEffectsAfter); err != nil {
			b.Fatal(err)
		}
		if run.Status != StatusCompleted || run.FilesObserved != 0 ||
			projected != "renamed/media-000000000.jpg" ||
			hashEffectsAfter != hashEffectsBefore || fixture.source.readCalls-readsBefore != 1 {
			b.Fatalf("50k rename run=%+v path=%q hashes=%d->%d reads=%d",
				run, projected, hashEffectsBefore, hashEffectsAfter, fixture.source.readCalls-readsBefore)
		}
		b.ReportMetric(float64(descendants), "descendants")
		b.ReportMetric(float64(fixture.source.readCalls-readsBefore), "directory-pages")
		b.ReportMetric(float64(time.Since(started).Microseconds())/1000, "rename-ms")
	}
}
