package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/roe/changefeed"
	"server/internal/storage/roe/nodegraph"
)

type deterministicFeed struct {
	mu              sync.Mutex
	events          []changefeed.Event
	volumeIdentity  string
	volumeKind      string
	journalIdentity string
	readHealth      changefeed.Health
	readErr         error
	readLimits      []int
}

func (feed *deterministicFeed) Snapshot(context.Context, repo.Repository) (changefeed.Checkpoint, error) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	return feed.checkpoint(len(feed.events)), nil
}

func (feed *deterministicFeed) Read(
	_ context.Context,
	_ repo.Repository,
	after changefeed.Checkpoint,
	through changefeed.Checkpoint,
	limit int,
) (changefeed.Batch, error) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	feed.readLimits = append(feed.readLimits, limit)
	if feed.readErr != nil {
		next := through
		next.Health = feed.readHealth
		return changefeed.Batch{Next: next}, feed.readErr
	}
	from, err := strconv.Atoi(string(after.Cursor))
	if err != nil {
		return changefeed.Batch{}, changefeed.ErrCursorInvalid
	}
	to, err := strconv.Atoi(string(through.Cursor))
	if err != nil || from < 0 || to < from || to > len(feed.events) || !after.SameIdentity(through) {
		return changefeed.Batch{}, changefeed.ErrCursorInvalid
	}
	if limit <= 0 {
		limit = 1
	}
	end := min(to, from+limit)
	events := append([]changefeed.Event(nil), feed.events[from:end]...)
	for index := range events {
		events[index].Cursor = []byte(strconv.Itoa(from + index + 1))
	}
	return changefeed.Batch{Events: events, Next: feed.checkpoint(end), Done: end == to}, nil
}

func (feed *deterministicFeed) publish(kind changefeed.EventKind, path, oldPath string, recursive bool) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	sequence := len(feed.events) + 1
	feed.events = append(feed.events, changefeed.Event{
		Key: fmt.Sprintf("event-%d", sequence), Kind: kind, Path: path, OldPath: oldPath, Recursive: recursive,
	})
}

func (feed *deterministicFeed) publishRaw(event changefeed.Event) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	feed.events = append(feed.events, event)
}

func (feed *deterministicFeed) checkpoint(sequence int) changefeed.Checkpoint {
	volumeIdentity := feed.volumeIdentity
	if volumeIdentity == "" {
		volumeIdentity = "test-volume"
	}
	journalIdentity := feed.journalIdentity
	if journalIdentity == "" {
		journalIdentity = "test-journal"
	}
	volumeKind := feed.volumeKind
	if volumeKind == "" {
		volumeKind = "local"
	}
	return changefeed.Checkpoint{
		AdapterKind: "fsevents", Cursor: []byte(strconv.Itoa(sequence)),
		VolumeIdentity: volumeIdentity, VolumeKind: volumeKind, JournalIdentity: journalIdentity,
		Health: changefeed.HealthHealthy,
	}
}

func (feed *deterministicFeed) failReads(health changefeed.Health, err error) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	feed.readHealth = health
	feed.readErr = err
}

func (feed *deterministicFeed) replaceVolume(identity string) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	feed.volumeIdentity = identity
}

func (feed *deterministicFeed) setVolumeKind(kind string) {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	feed.volumeKind = kind
}

func TestHealthyCursorZeroChangeIncrementalPassDoesNotWalkOrHash(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "album/photo.jpg", []byte("original"))
	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	firstRun := fixture.runToTerminal(t, first.OperationID)
	if firstRun.Status != StatusCompleted {
		t.Fatalf("initial healthy-cursor run = %q", firstRun.Status)
	}

	var observationsBefore, hashesBefore int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM repository_observations),
		  (SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash')
	`).Scan(&observationsBefore, &hashesBefore); err != nil {
		t.Fatal(err)
	}
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "periodic", "", false)
	if err != nil {
		t.Fatal(err)
	}
	secondRun := fixture.runToTerminal(t, second.OperationID)
	if secondRun.Status != StatusCompleted || secondRun.DirectoriesObserved != 0 || secondRun.FilesObserved != 0 {
		t.Fatalf("zero-change incremental run = %+v", secondRun)
	}
	var observationsAfter, hashesAfter int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM repository_observations),
		  (SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash')
	`).Scan(&observationsAfter, &hashesAfter); err != nil {
		t.Fatal(err)
	}
	if observationsAfter != observationsBefore || hashesAfter != hashesBefore {
		t.Fatalf("zero-change pass added observations/hash work: observations %d->%d hashes %d->%d",
			observationsBefore, observationsAfter, hashesBefore, hashesAfter)
	}
}

func TestFixedCatchUpCoalescesManyHintsIntoOneDirectoryVerification(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	const files = 100
	for index := range files {
		fixture.writeMedia(t, fmt.Sprintf("album/photo-%03d.jpg", index), []byte("unchanged"))
	}
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	for index := range files {
		feed.publish(changefeed.EventModify, fmt.Sprintf("album/photo-%03d.jpg", index), "", false)
	}
	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, receipt.OperationID)
	if run.Status != StatusCompleted || run.FilesObserved != files {
		t.Fatalf("coalesced dirty verification = %+v, want exactly %d observed files", run, files)
	}
	var watcherFacts int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM repository_observations
		WHERE run_id = ? AND source = 'journal'
	`, receipt.OperationID).Scan(&watcherFacts); err != nil {
		t.Fatal(err)
	}
	if watcherFacts != files {
		t.Fatalf("persisted watcher facts = %d, want %d", watcherFacts, files)
	}
	feed.mu.Lock()
	defer feed.mu.Unlock()
	for _, limit := range feed.readLimits {
		if limit > 4 {
			t.Fatalf("change-feed read limit = %d, want at most 4 events per writer turn", limit)
		}
	}
}

func TestCaughtUpAuthoritativeParentFinalizesAbsence(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "gone.jpg", []byte("original"))
	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, first.OperationID)
	drainHashEffects(t, fixture)

	if err := os.Remove(filepath.Join(fixture.repositoryPath, "gone.jpg")); err != nil {
		t.Fatal(err)
	}
	feed.publish(changefeed.EventRemove, "gone.jpg", "", false)
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, second.OperationID)
	if run.Status != StatusCompleted {
		t.Fatalf("authoritative absence run = %q", run.Status)
	}
	var activeNodes, activeLocations int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM repository_nodes WHERE name = 'gone.jpg' AND lifecycle = 'active'),
		  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL)
	`).Scan(&activeNodes, &activeLocations); err != nil {
		t.Fatal(err)
	}
	if activeNodes != 0 || activeLocations != 0 {
		t.Fatalf("proven absence left active nodes=%d locations=%d", activeNodes, activeLocations)
	}
	var coveredDirectories, coverageFacts int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM repository_nodes
		   WHERE kind = 'directory' AND lifecycle = 'active'
		     AND last_authoritative_coverage_revision > 0),
		  (SELECT count(*) FROM repository_observations
		   WHERE run_id = ? AND source = 'verifier' AND authoritative_child_set = 1)
	`, second.OperationID).Scan(&coveredDirectories, &coverageFacts); err != nil {
		t.Fatal(err)
	}
	if coveredDirectories == 0 || coverageFacts == 0 {
		t.Fatalf("authoritative coverage was not persisted: directories=%d facts=%d", coveredDirectories, coverageFacts)
	}
}

func TestCaughtUpMissingDirectoryClosesDescendantLocationsInBoundedCascades(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 1, feed)
	fixture.writeMedia(t, "album/direct.jpg", []byte("direct"))
	fixture.writeMedia(t, "album/nested/first.jpg", []byte("first"))
	fixture.writeMedia(t, "album/nested/second.jpg", []byte("second"))
	initial, err := fixture.controller.Request(
		fixture.ctx,
		fixture.repository.RepoID,
		"manual",
		"test",
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	assertActiveLocationCount(t, fixture, 3)

	if err := os.RemoveAll(filepath.Join(fixture.repositoryPath, "album")); err != nil {
		t.Fatal(err)
	}
	feed.publish(changefeed.EventRemove, "album", "", false)
	receipt, err := fixture.controller.Request(
		fixture.ctx,
		fixture.repository.RepoID,
		"watcher",
		"",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	var run repo.RepositoryScanRun
	for turn := 0; turn < 100; turn++ {
		if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID); err != nil {
			t.Fatal(err)
		}
		run, err = fixture.database.ReaderQueries.GetRepositoryScanRun(fixture.ctx, repo.GetRepositoryScanRunParams{
			RepositoryID: fixture.repository.RepoID, RunID: receipt.OperationID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == StatusFinalizing {
			break
		}
		if turn == 99 {
			t.Fatal("directory absence run did not reach finalizing")
		}
	}
	firstAbsenceTurn, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if firstAbsenceTurn.RowsApplied != 1 {
		t.Fatalf("first bounded absence turn applied %d rows, want 1", firstAbsenceTurn.RowsApplied)
	}
	crashedLease := "crashed-absence-controller"
	expiredAt := time.Now().Add(-time.Minute).UnixMicro()
	if _, err := fixture.database.Queries.ClaimRepositoryAbsenceFrontier(fixture.ctx, repo.ClaimRepositoryAbsenceFrontierParams{
		RunID: receipt.OperationID, LeaseID: &crashedLease, LeaseExpiresAt: &expiredAt,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	// A new process owns no in-memory frontier or cursor state. It must reclaim
	// the expired absence lease and continue from the persisted keyset cursor.
	fixture.controller = New(
		fixture.database,
		fixture.controller.files,
		Config{BatchSize: 1, ChangeFeed: feed},
		nil,
	)
	run = fixture.runToTerminal(t, receipt.OperationID)
	if run.Status != StatusCompleted {
		t.Fatalf("directory absence run = %+v", run)
	}
	assertActiveLocationCount(t, fixture, 0)

	var activeDescendants, openCascades int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM repository_nodes
		   WHERE parent_node_id IS NOT NULL AND lifecycle = 'active'),
		  (SELECT count(*) FROM repository_scan_frontier
		   WHERE run_id = ? AND purpose = 'absence' AND absence_finalized = 0)
	`, receipt.OperationID).Scan(&activeDescendants, &openCascades); err != nil {
		t.Fatal(err)
	}
	if activeDescendants != 0 || openCascades != 0 {
		t.Fatalf(
			"directory absence left active descendants=%d open cascades=%d",
			activeDescendants,
			openCascades,
		)
	}
}

func TestAbsenceFinalizationUsesStrictBatchCap(t *testing.T) {
	const expectedAbsenceBatchSize = 4
	if maximumAbsenceBatchSize != expectedAbsenceBatchSize {
		t.Fatalf(
			"absence batch cap = %d, want contract %d",
			maximumAbsenceBatchSize,
			expectedAbsenceBatchSize,
		)
	}
	maxAbsenceRows := exerciseAbsenceFinalizationQuantum(
		t,
		maximumTransactionBudget,
		expectedAbsenceBatchSize+4,
	)
	if maxAbsenceRows != expectedAbsenceBatchSize {
		t.Fatalf(
			"largest absence turn applied %d rows, want cap %d",
			maxAbsenceRows,
			expectedAbsenceBatchSize,
		)
	}
}

func TestChangeCatchUpUsesSingleEventWriterQuantum(t *testing.T) {
	if maximumChangeBatchSize != 1 {
		t.Fatalf("change batch cap = %d, want one event per writer turn", maximumChangeBatchSize)
	}
}

func TestAbsenceFinalizationYieldsAtTransactionBudget(t *testing.T) {
	maxAbsenceRows := exerciseAbsenceFinalizationQuantum(t, time.Nanosecond, 5)
	if maxAbsenceRows != 1 {
		t.Fatalf("largest budget-limited absence turn applied %d rows, want 1", maxAbsenceRows)
	}
}

func exerciseAbsenceFinalizationQuantum(
	t *testing.T,
	transactionBudget time.Duration,
	childCount int,
) int {
	t.Helper()
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, maximumBatchSize, feed)
	for index := range childCount {
		fixture.writeMedia(
			t,
			fmt.Sprintf("album/photo-%02d.jpg", index),
			[]byte(fmt.Sprintf("photo-%d", index)),
		)
	}
	initial, err := fixture.controller.Request(
		fixture.ctx,
		fixture.repository.RepoID,
		"manual",
		"test",
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	assertActiveLocationCount(t, fixture, childCount)
	fixture.controller.cfg.TransactionBudget = transactionBudget

	if err := os.RemoveAll(filepath.Join(fixture.repositoryPath, "album")); err != nil {
		t.Fatal(err)
	}
	feed.publish(changefeed.EventRemove, "album", "", false)
	receipt, err := fixture.controller.Request(
		fixture.ctx,
		fixture.repository.RepoID,
		"watcher",
		"",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	maxAbsenceRows := 0
	var terminal repo.RepositoryScanRun
	for turn := 0; turn < 100; turn++ {
		before, readErr := fixture.database.ReaderQueries.GetRepositoryScanRun(
			fixture.ctx,
			repo.GetRepositoryScanRunParams{
				RepositoryID: fixture.repository.RepoID,
				RunID:        receipt.OperationID,
			},
		)
		if readErr != nil {
			t.Fatal(readErr)
		}
		result, runErr := fixture.controller.RunTurn(
			fixture.ctx,
			fixture.repository.RepoID,
			receipt.OperationID,
		)
		if runErr != nil {
			t.Fatalf("turn %d: %v", turn, runErr)
		}
		if before.Status == StatusFinalizing {
			if result.RowsApplied > maximumAbsenceBatchSize {
				t.Fatalf(
					"absence turn applied %d rows, cap %d",
					result.RowsApplied,
					maximumAbsenceBatchSize,
				)
			}
			maxAbsenceRows = max(maxAbsenceRows, result.RowsApplied)
		}
		if !result.HasMore {
			terminal, readErr = fixture.database.ReaderQueries.GetRepositoryScanRun(
				fixture.ctx,
				repo.GetRepositoryScanRunParams{
					RepositoryID: fixture.repository.RepoID,
					RunID:        receipt.OperationID,
				},
			)
			if readErr != nil {
				t.Fatal(readErr)
			}
			break
		}
		if turn == 99 {
			t.Fatal("bounded absence run did not reach a terminal state")
		}
	}
	if terminal.Status != StatusCompleted {
		t.Fatalf("bounded absence run = %+v", terminal)
	}
	assertActiveLocationCount(t, fixture, 0)
	return maxAbsenceRows
}

func TestNativeIdentityDirectoryRenameChangesOneEdgeWithoutDescendantWalk(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	for index := range 16 {
		fixture.writeMedia(t, fmt.Sprintf("before/photo-%02d.jpg", index), []byte(fmt.Sprintf("photo-%d", index)))
	}
	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, first.OperationID)
	var descendantID uuid.UUID
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT node_id FROM repository_nodes WHERE name = 'photo-00.jpg' AND lifecycle = 'active'
	`).Scan(&descendantID); err != nil {
		t.Fatal(err)
	}
	var hashesBefore int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'").Scan(&hashesBefore); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(fixture.repositoryPath, "before"), filepath.Join(fixture.repositoryPath, "after")); err != nil {
		t.Fatal(err)
	}
	feed.publish(changefeed.EventRename, "after", "before", false)
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, second.OperationID)
	if run.Status != StatusCompleted || run.FilesObserved != 0 {
		t.Fatalf("rename run unexpectedly walked descendants: %+v", run)
	}
	var hashesAfter int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'").Scan(&hashesAfter); err != nil {
		t.Fatal(err)
	}
	if hashesAfter != hashesBefore {
		t.Fatalf("directory rename scheduled descendant hashes: %d -> %d", hashesBefore, hashesAfter)
	}
	projected, err := nodegraph.ProjectPath(fixture.ctx, fixture.database.ReaderQueries, fixture.repository.RepoID, descendantID)
	if err != nil {
		t.Fatal(err)
	}
	if projected != "after/photo-00.jpg" {
		t.Fatalf("renamed descendant path = %q", projected)
	}
}

func TestFixedCatchUpBoundaryDefersLateCoalescedChangeToFollowUpRun(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 1, feed)
	fixture.writeMedia(t, "a.jpg", []byte("a"))
	fixture.writeMedia(t, "b.jpg", []byte("b"))
	fixture.writeMedia(t, "c.jpg", []byte("c"))
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)

	feed.publish(changefeed.EventModify, "a.jpg", "", false)
	feed.publish(changefeed.EventModify, "b.jpg", "", false)
	current, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, current.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, current.OperationID); err != nil {
		t.Fatal(err)
	}

	// Recreate the controller after C1 has been persisted. No in-memory
	// checkpoint may be needed to keep this run's catch-up boundary fixed.
	fixture.controller = New(
		fixture.database,
		fixture.controller.files,
		Config{BatchSize: 1, ChangeFeed: feed},
		nil,
	)
	feed.publish(changefeed.EventModify, "c.jpg", "", false)
	late, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !late.Coalesced || late.OperationID != current.OperationID {
		t.Fatalf("late request = %+v, want coalesced operation %s", late, current.OperationID)
	}
	finished := fixture.runToTerminal(t, current.OperationID)
	if got := string(finished.CursorTarget); got != "2" {
		t.Fatalf("fixed catch-up target = %q, want 2", got)
	}
	if got := string(finished.CursorEnd); got != "2" {
		t.Fatalf("first run cursor end = %q, want 2", got)
	}
	var lateInFirst int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM repository_observations
		WHERE run_id = ? AND source_event_key = 'event-3'`, current.OperationID,
	).Scan(&lateInFirst); err != nil {
		t.Fatal(err)
	}
	if lateInFirst != 0 {
		t.Fatalf("late event was consumed beyond fixed C1: count=%d", lateInFirst)
	}

	followUp, err := fixture.database.ReaderQueries.GetActiveRepositoryScanRun(fixture.ctx, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if followUp.RunID == current.OperationID || followUp.Mode != "recovery" {
		t.Fatalf("follow-up run = %+v", followUp)
	}
	followUp = fixture.runToTerminal(t, followUp.RunID)
	if got := string(followUp.CursorEnd); got != "3" {
		t.Fatalf("follow-up cursor end = %q, want 3", got)
	}
	var lateInFollowUp int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM repository_observations
		WHERE run_id = ? AND source_event_key = 'event-3'`, followUp.RunID,
	).Scan(&lateInFollowUp); err != nil {
		t.Fatal(err)
	}
	if lateInFollowUp != 1 {
		t.Fatalf("follow-up late event count = %d, want 1", lateInFollowUp)
	}
}

func TestChangeFeedOverflowCannotFinalizeAbsence(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "keep.jpg", []byte("keep"))
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	if err := os.Remove(filepath.Join(fixture.repositoryPath, "keep.jpg")); err != nil {
		t.Fatal(err)
	}
	feed.publish(changefeed.EventRemove, "keep.jpg", "", false)
	feed.failReads(changefeed.HealthOverflow, changefeed.ErrCursorInvalid)

	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, receipt.OperationID)
	if run.Status != StatusPartial {
		t.Fatalf("overflow run status = %q, want partial", run.Status)
	}
	assertActiveLocationCount(t, fixture, 1)
	state, err := fixture.database.ReaderQueries.GetRepositoryObservationState(fixture.ctx, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if state.CursorHealth != string(changefeed.HealthOverflow) || state.FullVerificationRequired == 0 {
		t.Fatalf("overflow adapter state = %+v", state)
	}
}

func TestMalformedChangeEventFailsClosedWithoutWedgingController(t *testing.T) {
	tests := []struct {
		name  string
		event changefeed.Event
	}{
		{
			name:  "blank event key",
			event: changefeed.Event{Kind: changefeed.EventRemove, Path: "keep.jpg"},
		},
		{
			name:  "unsupported event kind",
			event: changefeed.Event{Key: "bad-kind", Kind: "exchange", Path: "keep.jpg"},
		},
		{
			name:  "escaping path",
			event: changefeed.Event{Key: "bad-path", Kind: changefeed.EventRemove, Path: "../keep.jpg"},
		},
		{
			name: "escaping old path",
			event: changefeed.Event{
				Key: "bad-old-path", Kind: changefeed.EventRename,
				Path: "keep.jpg", OldPath: "../keep.jpg",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feed := &deterministicFeed{}
			fixture := newControllerFixtureWithFeed(t, 8, feed)
			fixture.writeMedia(t, "keep.jpg", []byte("keep"))
			initial, err := fixture.controller.Request(
				fixture.ctx,
				fixture.repository.RepoID,
				"manual",
				"test",
				true,
			)
			if err != nil {
				t.Fatal(err)
			}
			fixture.runToTerminal(t, initial.OperationID)
			drainHashEffects(t, fixture)
			if err := os.Remove(filepath.Join(fixture.repositoryPath, "keep.jpg")); err != nil {
				t.Fatal(err)
			}
			feed.publishRaw(test.event)

			receipt, err := fixture.controller.Request(
				fixture.ctx,
				fixture.repository.RepoID,
				"watcher",
				"",
				false,
			)
			if err != nil {
				t.Fatal(err)
			}
			run := fixture.runToTerminal(t, receipt.OperationID)
			if run.Status != StatusPartial || valueOrEmpty(run.FailureCode) != "change_event_invalid" {
				t.Fatalf("malformed event run = %+v", run)
			}
			assertActiveLocationCount(t, fixture, 1)
			state, err := fixture.database.ReaderQueries.GetRepositoryObservationState(
				fixture.ctx,
				fixture.repository.RepoID,
			)
			if err != nil {
				t.Fatal(err)
			}
			if state.CursorHealth != string(changefeed.HealthGap) || state.FullVerificationRequired == 0 {
				t.Fatalf("malformed event adapter state = %+v", state)
			}
		})
	}
}

func TestVolumeIdentityReplacementCannotFinalizeAbsence(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "keep.jpg", []byte("keep"))
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	if err := os.Remove(filepath.Join(fixture.repositoryPath, "keep.jpg")); err != nil {
		t.Fatal(err)
	}
	feed.replaceVolume("replacement-volume")

	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, receipt.OperationID)
	if run.Status != StatusPartial {
		t.Fatalf("replacement-volume run status = %q, want partial", run.Status)
	}
	assertActiveLocationCount(t, fixture, 1)
	state, err := fixture.database.ReaderQueries.GetRepositoryObservationState(fixture.ctx, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if state.CursorHealth != string(changefeed.HealthGap) || state.FullVerificationRequired == 0 {
		t.Fatalf("replacement-volume adapter state = %+v", state)
	}
}

func assertActiveLocationCount(t *testing.T, fixture *controllerFixture, want int) {
	t.Helper()
	var count int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL
	`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("active location count = %d, want %d", count, want)
	}
}

func TestRemovableVolumeAbsenceSettlesAcrossAuthoritativeRuns(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "settle.jpg", []byte("settle"))
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	feed.setVolumeKind("removable")
	fixture.controller.cfg.Settle = time.Hour
	fixedNow := time.Now().UTC().Truncate(time.Microsecond)
	fixture.controller.now = func() time.Time { return fixedNow }
	if err := os.Remove(filepath.Join(fixture.repositoryPath, "settle.jpg")); err != nil {
		t.Fatal(err)
	}
	feed.publish(changefeed.EventRemove, "settle.jpg", "", false)

	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "watcher", "", false)
	if err != nil {
		t.Fatal(err)
	}
	firstRun := fixture.runToTerminal(t, first.OperationID)
	if firstRun.Status != StatusCompleted {
		t.Fatalf("first removable absence run = %q", firstRun.Status)
	}
	assertActiveLocationCount(t, fixture, 1)
	var candidateAt *int64
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT absence_first_observed_at FROM repository_nodes WHERE name = 'settle.jpg'
	`).Scan(&candidateAt); err != nil {
		t.Fatal(err)
	}
	if candidateAt == nil || *candidateAt != fixedNow.UnixMicro() {
		t.Fatalf("absence settle candidate = %v, want %d", candidateAt, fixedNow.UnixMicro())
	}

	fixedNow = fixedNow.Add(2 * time.Hour)
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "periodic", "", true)
	if err != nil {
		t.Fatal(err)
	}
	secondRun := fixture.runToTerminal(t, second.OperationID)
	if secondRun.Status != StatusCompleted {
		t.Fatalf("settled removable absence run = %q", secondRun.Status)
	}
	assertActiveLocationCount(t, fixture, 0)
}
