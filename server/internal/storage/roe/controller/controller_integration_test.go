package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/roe/changefeed"
	"server/internal/storage/rootcfg"
)

type controllerFixture struct {
	ctx            context.Context
	database       *db.DB
	controller     *Controller
	repository     repo.Repository
	repositoryPath string
}

func newControllerFixture(t *testing.T, batchSize int) *controllerFixture {
	return newControllerFixtureWithFeed(t, batchSize, nil)
}

func newControllerFixtureWithFeed(t *testing.T, batchSize int, feed changefeed.Feed) *controllerFixture {
	t.Helper()
	if feed == nil {
		feed = changefeed.Periodic{}
	}
	ctx := context.Background()
	databaseDirectory := t.TempDir()
	if err := os.Chmod(databaseDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(databaseDirectory, "catalog.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := database.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "controller-owner", Password: "unused", DisplayName: "Controller Owner",
		Role: "admin", WebauthnUserHandle: []byte("controller-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}

	rootPath := t.TempDir()
	repositoryPath := filepath.Join(rootPath, "repository")
	if err := os.Mkdir(repositoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	rootID := uuid.New()
	rootConfig := rootcfg.New("controller root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(rootPath); err != nil {
		t.Fatal(err)
	}
	repositoryID := uuid.New()
	repositoryConfig := repocfg.NewRepositoryConfig("controller repository")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := database.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "controller root", Path: rootPath,
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repository, err := database.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "controller repository", Path: repositoryPath,
		Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive,
		Activity:     dbtypes.RepositoryActivityIdle, DefaultOwnerID: &owner.UserID,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(nil, database.Queries)
	return &controllerFixture{
		ctx: ctx, database: database,
		controller: New(database, files, Config{BatchSize: batchSize, ChangeFeed: feed}, nil),
		repository: repository, repositoryPath: repositoryPath,
	}
}

func (f *controllerFixture) writeMedia(t *testing.T, relative string, content []byte) {
	t.Helper()
	filename := filepath.Join(f.repositoryPath, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *controllerFixture) runToTerminal(t *testing.T, operationID uuid.UUID) repo.RepositoryScanRun {
	t.Helper()
	// Catch-up intentionally commits one native event per writer turn. Keep the
	// test bound above the largest deterministic feed while still failing fast
	// on a cursor that stops making progress.
	for turn := 0; turn < 500; turn++ {
		result, err := f.controller.RunTurn(f.ctx, f.repository.RepoID, operationID)
		if err != nil {
			t.Fatalf("turn %d: %v", turn, err)
		}
		if !result.HasMore {
			run, err := f.database.ReaderQueries.GetRepositoryScanRun(f.ctx, repo.GetRepositoryScanRunParams{
				RepositoryID: f.repository.RepoID, RunID: operationID,
			})
			if err != nil {
				t.Fatal(err)
			}
			return run
		}
	}
	t.Fatal("repository controller did not reach a terminal state")
	return repo.RepositoryScanRun{}
}

func TestControllerRequestCoalescesAndPublishesProgressively(t *testing.T) {
	fixture := newControllerFixture(t, 2)
	fixture.writeMedia(t, "wide/a.jpg", []byte("a"))
	fixture.writeMedia(t, "wide/b.jpg", []byte("b"))
	fixture.writeMedia(t, "wide/c.jpg", []byte("c"))

	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Inserted || first.Coalesced || !second.Coalesced || second.Inserted {
		t.Fatalf("receipts = first %+v second %+v", first, second)
	}
	if second.OperationID != first.OperationID {
		t.Fatalf("coalesced operation = %s, want %s", second.OperationID, first.OperationID)
	}

	progressive := false
	for turn := 0; turn < 100; turn++ {
		result, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, first.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		var hashes int
		if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
			"SELECT count(*) FROM repository_outbox WHERE repository_id = ? AND effect_kind = 'hash'",
			fixture.repository.RepoID,
		).Scan(&hashes); err != nil {
			t.Fatal(err)
		}
		if hashes > 0 && result.HasMore {
			progressive = true
		}
		if !result.HasMore {
			break
		}
	}
	if !progressive {
		t.Fatal("hash work was not published before crawl completion")
	}
	run, err := fixture.database.ReaderQueries.GetRepositoryScanRun(fixture.ctx, repo.GetRepositoryScanRunParams{
		RepositoryID: fixture.repository.RepoID, RunID: first.OperationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != StatusPartial {
		t.Fatalf("periodic-adapter run status = %q, want partial without a valid cursor", run.Status)
	}
	if run.FilesObserved != 3 {
		t.Fatalf("observed files = %d, want 3", run.FilesObserved)
	}
}

func TestControllerCoalescedQueuedRequestPublishesCurrentEpochWakeup(t *testing.T) {
	fixture := newControllerFixture(t, 8)

	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Coalesced || second.OperationID != first.OperationID || second.RequestedEpoch <= first.RequestedEpoch {
		t.Fatalf("receipts = first %+v second %+v", first, second)
	}

	var wakeups int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*)
		FROM repository_outbox
		WHERE repository_id = ?
		  AND effect_kind = 'controller'
		  AND entity_id = ?
		  AND expected_revision = ?`,
		fixture.repository.RepoID, first.OperationID.String(), second.RequestedEpoch,
	).Scan(&wakeups); err != nil {
		t.Fatal(err)
	}
	if wakeups != 1 {
		t.Fatalf("current-epoch controller wakeups = %d, want 1", wakeups)
	}
}

func TestControllerReclaimsFrontierAndReplayIsIdempotent(t *testing.T) {
	fixture := newControllerFixture(t, 16)
	fixture.writeMedia(t, "a.jpg", []byte("a"))
	fixture.writeMedia(t, "b.jpg", []byte("b"))
	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID); err != nil {
		t.Fatal(err)
	}
	root, err := fixture.database.ReaderQueries.GetRepositoryRootNode(fixture.ctx, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	expiredLease := "crashed-controller"
	expiredAt := time.Now().Add(-time.Minute).UnixMicro()
	if _, err := fixture.database.Queries.ClaimRepositoryScanFrontier(fixture.ctx, repo.ClaimRepositoryScanFrontierParams{
		RunID: receipt.OperationID, LeaseID: &expiredLease, LeaseExpiresAt: &expiredAt,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().Add(-2 * time.Minute)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID); err != nil {
		t.Fatal(err)
	}
	var beforeObservations, beforeEffects int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM repository_observations WHERE run_id = ?", receipt.OperationID,
	).Scan(&beforeObservations); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'",
	).Scan(&beforeEffects); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(fixture.ctx, `
		UPDATE repository_scan_frontier
		SET state = 'pending', continuation_offset = 0, lease_id = NULL, lease_expires_at = NULL
		WHERE run_id = ? AND directory_node_id = ?`, receipt.OperationID, root.NodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID); err != nil {
		t.Fatal(err)
	}
	var afterObservations, afterEffects int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM repository_observations WHERE run_id = ?", receipt.OperationID,
	).Scan(&afterObservations); err != nil {
		t.Fatal(err)
	}
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT count(*) FROM repository_outbox WHERE effect_kind = 'hash'",
	).Scan(&afterEffects); err != nil {
		t.Fatal(err)
	}
	if afterObservations != beforeObservations || afterEffects != beforeEffects {
		t.Fatalf("replay duplicated state: observations %d->%d effects %d->%d",
			beforeObservations, afterObservations, beforeEffects, afterEffects)
	}
}

func TestControllerPartialVerificationCannotFinalizeAbsence(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	fixture.writeMedia(t, "keep.jpg", []byte("original"))
	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, first.OperationID)
	root, err := fixture.database.ReaderQueries.GetRepositoryRootNode(fixture.ctx, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	node, err := fixture.database.ReaderQueries.GetActiveRepositoryChildByName(fixture.ctx, repo.GetActiveRepositoryChildByNameParams{
		RepositoryID: fixture.repository.RepoID,
		ParentNodeID: uuid.NullUUID{UUID: root.NodeID, Valid: true}, NameKey: "keep.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.repositoryPath, "keep.jpg")); err != nil {
		t.Fatal(err)
	}
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "periodic", "", true)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, second.OperationID)
	if run.Status != StatusPartial {
		t.Fatalf("verification status = %q, want partial", run.Status)
	}
	current, err := fixture.database.ReaderQueries.GetRepositoryNode(fixture.ctx, repo.GetRepositoryNodeParams{
		RepositoryID: fixture.repository.RepoID, NodeID: node.NodeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if current.Lifecycle != "active" {
		t.Fatalf("unverified absence changed lifecycle to %q", current.Lifecycle)
	}
}

func TestControllerCancellationPreservesUnverifiedLocationAndRequiresRecovery(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 1, feed)
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

	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID); err != nil {
		t.Fatal(err)
	}
	requested, err := fixture.controller.CancelScanRun(
		fixture.ctx, fixture.repository.RepoID.String(), receipt.OperationID.String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if requested.CancellationRequested == 0 {
		t.Fatal("cancellation request was not persisted")
	}
	result, err := fixture.controller.RunTurn(fixture.ctx, fixture.repository.RepoID, receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasMore || result.Status != StatusCancelled {
		t.Fatalf("cancel turn = %+v", result)
	}
	var activeLocations int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL
	`).Scan(&activeLocations); err != nil {
		t.Fatal(err)
	}
	if activeLocations != 1 {
		t.Fatalf("cancelled verification left active locations=%d, want 1", activeLocations)
	}
	state, err := fixture.database.ReaderQueries.GetRepositoryObservationState(fixture.ctx, fixture.repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if state.FullVerificationRequired == 0 || state.AppliedEpoch != requested.RequestedEpoch {
		t.Fatalf("post-cancel observation state = %+v", state)
	}
}

func TestRepositoryOfflineCannotFinalizeAbsence(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "keep.jpg", []byte("keep"))
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	offlinePath := fixture.repositoryPath + ".offline"
	if err := os.Rename(fixture.repositoryPath, offlinePath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(offlinePath, fixture.repositoryPath) })

	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "recovery", "", true)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, receipt.OperationID)
	if run.Status != StatusPartial || run.ErrorDirectories == 0 {
		t.Fatalf("offline repository run = %+v", run)
	}
	assertActiveLocationCount(t, fixture, 1)
}

func TestAccessDeniedSubtreePreservesPriorLocation(t *testing.T) {
	feed := &deterministicFeed{}
	fixture := newControllerFixtureWithFeed(t, 8, feed)
	fixture.writeMedia(t, "locked/keep.jpg", []byte("keep"))
	initial, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, initial.OperationID)
	drainHashEffects(t, fixture)
	lockedPath := filepath.Join(fixture.repositoryPath, "locked")
	if err := os.Chmod(lockedPath, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedPath, 0o755) })
	if opened, openErr := os.Open(lockedPath); openErr == nil {
		_ = opened.Close()
		t.Skip("test process can bypass directory permissions")
	}

	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "recovery", "", true)
	if err != nil {
		t.Fatal(err)
	}
	run := fixture.runToTerminal(t, receipt.OperationID)
	if run.Status != StatusPartial || run.ErrorDirectories == 0 {
		t.Fatalf("access-denied repository run = %+v", run)
	}
	assertActiveLocationCount(t, fixture, 1)
}
