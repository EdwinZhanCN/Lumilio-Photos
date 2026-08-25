package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/event"
	"server/internal/storage"

	"github.com/google/uuid"
)

type fakeRiverStopper struct {
	stopErr       error
	forcedErr     error
	stopped       chan struct{}
	stopCalls     int
	forcedCalls   int
	closeOnForced bool
}

type fakeDefaultStorageRuntimeManager struct {
	ensureRoot *repo.RepositoryRoot
	ensureErr  error
	roots      []repo.RepositoryRoot
	listErr    error
}

func (fake fakeDefaultStorageRuntimeManager) EnsureDefaultRepositoryRoot(context.Context, string, ...storage.LifecycleRequest) (*repo.RepositoryRoot, error) {
	return fake.ensureRoot, fake.ensureErr
}

func (fake fakeDefaultStorageRuntimeManager) ListRepositoryRoots(context.Context) ([]repo.RepositoryRoot, error) {
	return fake.roots, fake.listErr
}

func TestDefaultStorageRecoveryKeepsRuntimeStartableInDegradedMode(t *testing.T) {
	registered := repo.RepositoryRoot{
		RootID: uuid.New(), Kind: dbtypes.RepositoryRootKindDefault,
		Status: dbtypes.RepositoryRootStatusOffline, Path: "/missing/default",
	}
	root, degraded, err := ensureDefaultStorageForRuntime(context.Background(), fakeDefaultStorageRuntimeManager{
		ensureErr: storage.ErrRepositoryRootOffline,
		roots:     []repo.RepositoryRoot{registered},
	}, registered.Path)
	if err != nil {
		t.Fatalf("registered default storage stopped runtime startup: %v", err)
	}
	if !degraded || root == nil || root.RootID != registered.RootID {
		t.Fatalf("degraded result = root %#v degraded %t", root, degraded)
	}

	_, degraded, err = ensureDefaultStorageForRuntime(context.Background(), fakeDefaultStorageRuntimeManager{
		ensureErr: storage.ErrRepositoryRootOffline,
	}, "/fresh/unavailable")
	if err == nil || degraded {
		t.Fatalf("fresh initialization failure = %v degraded %t", err, degraded)
	}

	_, degraded, err = ensureDefaultStorageForRuntime(context.Background(), fakeDefaultStorageRuntimeManager{
		ensureErr: storage.ErrRepositoryRootInvalid,
		roots:     []repo.RepositoryRoot{registered},
	}, "/different/identity")
	if err == nil || degraded {
		t.Fatalf("invalid migration target = %v degraded %t", err, degraded)
	}
}

func TestDefaultStorageRecoveryCanonicalizesOfflinePathAliases(t *testing.T) {
	realParent := t.TempDir()
	aliasParent := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Skipf("filesystem does not permit symlink fixture: %v", err)
	}
	registered := repo.RepositoryRoot{
		RootID: uuid.New(), Kind: dbtypes.RepositoryRootKindDefault,
		Status: dbtypes.RepositoryRootStatusOffline, Path: filepath.Join(realParent, "offline-default"),
	}
	root, degraded, err := ensureDefaultStorageForRuntime(context.Background(), fakeDefaultStorageRuntimeManager{
		ensureErr: storage.ErrRepositoryRootOffline,
		roots:     []repo.RepositoryRoot{registered},
	}, filepath.Join(aliasParent, "offline-default"))
	if err != nil || !degraded || root == nil || root.RootID != registered.RootID {
		t.Fatalf("canonical offline alias was not recognized: root=%#v degraded=%t err=%v", root, degraded, err)
	}
}

func (fake *fakeRiverStopper) Stop(context.Context) error {
	fake.stopCalls++
	return fake.stopErr
}

func (fake *fakeRiverStopper) StopAndCancel(context.Context) error {
	fake.forcedCalls++
	if fake.closeOnForced {
		close(fake.stopped)
		fake.closeOnForced = false
	}
	return fake.forcedErr
}

func (fake *fakeRiverStopper) Stopped() <-chan struct{} {
	return fake.stopped
}

func TestStopRiverQueueUsesForcedCancellationAfterDrainFailure(t *testing.T) {
	fake := &fakeRiverStopper{
		stopErr:       context.DeadlineExceeded,
		stopped:       make(chan struct{}),
		closeOnForced: true,
	}
	if err := stopRiverQueue(fake, time.Millisecond, time.Millisecond); err != nil {
		t.Fatalf("stopRiverQueue: %v", err)
	}
	if fake.stopCalls != 1 || fake.forcedCalls != 1 {
		t.Fatalf("stop calls = %d/%d, want 1/1", fake.stopCalls, fake.forcedCalls)
	}
}

func TestStopRiverQueueRequiresStoppedConfirmation(t *testing.T) {
	fake := &fakeRiverStopper{
		stopped:       make(chan struct{}),
		closeOnForced: true,
	}
	if err := stopRiverQueue(fake, time.Millisecond, time.Millisecond); err != nil {
		t.Fatalf("stopRiverQueue: %v", err)
	}
	if fake.stopCalls != 1 || fake.forcedCalls != 1 {
		t.Fatalf("unconfirmed graceful stop calls = %d/%d, want 1/1", fake.stopCalls, fake.forcedCalls)
	}
}

func TestStopRiverQueueRejectsUnconfirmedStop(t *testing.T) {
	fake := &fakeRiverStopper{
		stopErr:   context.DeadlineExceeded,
		forcedErr: context.DeadlineExceeded,
		stopped:   make(chan struct{}),
	}
	err := stopRiverQueue(fake, time.Millisecond, time.Millisecond)
	if err == nil {
		t.Fatal("stopRiverQueue accepted an unconfirmed stop")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stopRiverQueue error = %v", err)
	}
}

func TestPprofHostCanRestartOnSameAddress(t *testing.T) {
	first, err := startPprofHost("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := first.server.Addr
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	if err := first.shutdown(shutdownCtx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	second, err := startPprofHost(addr)
	if err != nil {
		t.Fatalf("restart pprof host on %s: %v", addr, err)
	}
	shutdownCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := second.shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsStructLiteralConfig(t *testing.T) {
	err := Run(context.Background(), config.AppConfig{}, OperatorControls{})
	if err == nil || !strings.Contains(err.Error(), "strict manifest loader") {
		t.Fatalf("expected unvalidated config rejection, got %v", err)
	}
}

func TestProductURLUsesLoopbackForDesktopListeners(t *testing.T) {
	for _, test := range []struct {
		listen string
		want   string
	}{
		{listen: ":6680", want: "http://127.0.0.1:6680"},
		{listen: "0.0.0.0:6680", want: "http://127.0.0.1:6680"},
		{listen: "127.0.0.1:6680", want: "http://127.0.0.1:6680"},
	} {
		if got := productURL(test.listen); got != test.want {
			t.Fatalf("productURL(%q) = %q, want %q", test.listen, got, test.want)
		}
	}
}

func TestQueueRecoveryPredicatesOnlyWakeForDurableWork(t *testing.T) {
	ctx := context.Background()
	database, repository, _, _ := newCutoverPreflightFixture(t, ctx)
	now := time.Now().UTC()

	pending, err := repositoryOutboxWorkPending(ctx, database.ReaderSQL, "controller", now)
	if err != nil || pending {
		t.Fatalf("empty repository outbox pending=%t err=%v", pending, err)
	}
	pending, err = eventMaintenanceWorkPending(ctx, database.ReaderSQL, now)
	if err != nil || pending {
		t.Fatalf("converged Event state pending=%t err=%v", pending, err)
	}
	pending, err = locationProjectionWorkPending(ctx, database.ReaderSQL)
	if err != nil || pending {
		t.Fatalf("converged location projection pending=%t err=%v", pending, err)
	}
	pending, err = ocrIndexWorkPending(ctx, database.ReaderSQL)
	if err != nil || pending {
		t.Fatalf("empty OCR outbox pending=%t err=%v", pending, err)
	}

	outboxID := uuid.New()
	if _, err := database.Queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
		OutboxID:         outboxID,
		RepositoryID:     repository.RepoID,
		EffectKey:        "controller:test:1",
		EffectKind:       "controller",
		EntityID:         uuid.NewString(),
		ExpectedRevision: 1,
		Payload:          `{}`,
		CreatedAt:        dbtypes.NewTimestamp(now),
	}); err != nil {
		t.Fatal(err)
	}
	pending, err = repositoryOutboxWorkPending(ctx, database.ReaderSQL, "controller", now)
	if err != nil || !pending {
		t.Fatalf("durable repository outbox pending=%t err=%v", pending, err)
	}
	activeRepositoryJob := insertActiveRiverJob(t, ctx, database.SQL, "drain_repository_outbox", "repository_outbox", `{"effectKind":"controller"}`)
	pending, err = repositoryOutboxWorkPending(ctx, database.ReaderSQL, "controller", now)
	if err != nil || pending {
		t.Fatalf("actively owned repository outbox pending=%t err=%v", pending, err)
	}
	deleteRiverJob(t, ctx, database.SQL, activeRepositoryJob)
	futureLease := now.Add(time.Minute).UnixMicro()
	if _, err := database.SQL.ExecContext(ctx, `
UPDATE repository_outbox
SET status='delivering', lease_id='active', lease_expires_at=?
WHERE outbox_id=?`, futureLease, outboxID); err != nil {
		t.Fatal(err)
	}
	pending, err = repositoryOutboxWorkPending(ctx, database.ReaderSQL, "controller", now)
	if err != nil || pending {
		t.Fatalf("actively leased repository outbox pending=%t err=%v", pending, err)
	}
	pending, err = repositoryOutboxWorkPending(ctx, database.ReaderSQL, "controller", now.Add(2*time.Minute))
	if err != nil || !pending {
		t.Fatalf("expired repository outbox pending=%t err=%v", pending, err)
	}

	owner, err := database.Queries.GetUserByUsername(ctx, "cutover-owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.WithTx(ctx, catalogtx.OperationEventRebuildRequest, func(tx *sql.Tx, _ *repo.Queries) error {
		return event.MarkEventFactsChangedTx(ctx, tx, owner.UserID, "queue_recovery_test")
	}); err != nil {
		t.Fatal(err)
	}
	pending, err = eventMaintenanceWorkPending(ctx, database.ReaderSQL, now)
	if err != nil || !pending {
		t.Fatalf("dirty Event state pending=%t err=%v", pending, err)
	}
	activeEventJob := insertActiveRiverJob(t, ctx, database.SQL, "schedule_event_rebuilds", "event_scheduler", `{}`)
	pending, err = eventMaintenanceWorkPending(ctx, database.ReaderSQL, now)
	if err != nil || pending {
		t.Fatalf("actively owned Event maintenance pending=%t err=%v", pending, err)
	}
	deleteRiverJob(t, ctx, database.SQL, activeEventJob)

	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO location_projection_state (
  repository_id, owner_id, source_revision, published_revision, updated_at
) VALUES (?, ?, 2, 1, ?)
`, repository.RepoID, owner.UserID, now.UnixMicro()); err != nil {
		t.Fatal(err)
	}
	pending, err = locationProjectionWorkPending(ctx, database.ReaderSQL)
	if err != nil || !pending {
		t.Fatalf("dirty location projection pending=%t err=%v", pending, err)
	}
	activeLocationJob := insertActiveRiverJob(
		t,
		ctx,
		database.SQL,
		"rebuild_location_clusters",
		"rebuild_location_clusters",
		`{"repositoryId":"`+repository.RepoID.String()+`","ownerId":`+fmt.Sprint(owner.UserID)+`}`,
	)
	pending, err = locationProjectionWorkPending(ctx, database.ReaderSQL)
	if err != nil || pending {
		t.Fatalf("actively owned location projection pending=%t err=%v", pending, err)
	}
	deleteRiverJob(t, ctx, database.SQL, activeLocationJob)
	activeLocationScheduler := insertActiveRiverJob(t, ctx, database.SQL, "schedule_location_rebuilds", "rebuild_location_clusters", `{}`)
	pending, err = locationProjectionWorkPending(ctx, database.ReaderSQL)
	if err != nil || pending {
		t.Fatalf("actively owned location scheduler pending=%t err=%v", pending, err)
	}
	deleteRiverJob(t, ctx, database.SQL, activeLocationScheduler)

	if err := database.Queries.UpsertOCRIndexOutbox(ctx, repo.UpsertOCRIndexOutboxParams{
		AssetID: uuid.New(), Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	pending, err = ocrIndexWorkPending(ctx, database.ReaderSQL)
	if err != nil || !pending {
		t.Fatalf("durable OCR outbox pending=%t err=%v", pending, err)
	}
	activeOCRJob := insertActiveRiverJob(t, ctx, database.SQL, "process_ocr_outbox", "ocr_index", `{}`)
	pending, err = ocrIndexWorkPending(ctx, database.ReaderSQL)
	if err != nil || pending {
		t.Fatalf("actively owned OCR outbox pending=%t err=%v", pending, err)
	}
	deleteRiverJob(t, ctx, database.SQL, activeOCRJob)
}

func insertActiveRiverJob(t *testing.T, ctx context.Context, database *sql.DB, kind, queue, args string) int64 {
	t.Helper()
	result, err := database.ExecContext(ctx, `
INSERT INTO river_job(kind, queue, state, args)
VALUES (?, ?, 'running', jsonb(?))`, kind, queue, args)
	if err != nil {
		t.Fatalf("insert active River job %s: %v", kind, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read active River job %s ID: %v", kind, err)
	}
	return id
}

func deleteRiverJob(t *testing.T, ctx context.Context, database *sql.DB, id int64) {
	t.Helper()
	if _, err := database.ExecContext(ctx, `DELETE FROM river_job WHERE id = ?`, id); err != nil {
		t.Fatalf("delete active River job %d: %v", id, err)
	}
}

func TestPendingWorkSignalCoalescesWithoutBlockingPeriodicConstructor(t *testing.T) {
	signal := &pendingWorkSignal{}
	if signal.ConsumePending() {
		t.Fatal("new signal unexpectedly pending")
	}

	signal.Notify()
	signal.Notify()
	if !signal.ConsumePending() {
		t.Fatal("coalesced durable work signal was not consumed")
	}
	if signal.ConsumePending() {
		t.Fatal("one wakeup must consume all duplicate notifications")
	}
}

func TestPendingWorkMonitorLevelTriggersUnownedBacklog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	notifier := &countingPendingNotifier{}
	checks := 0
	monitorPendingWork(ctx, time.Microsecond, "test_backlog", func(context.Context) (bool, error) {
		checks++
		if checks == 3 {
			cancel()
		}
		return true, nil
	}, notifier, nil)
	if notifier.count != 3 {
		t.Fatalf("notifications = %d, want one for every positive unowned-work probe", notifier.count)
	}
}

type countingPendingNotifier struct {
	count int
}

func (n *countingPendingNotifier) Notify() { n.count++ }

func TestWALStateOnlyRearmsCheckpointAfterFileChanges(t *testing.T) {
	checkpointed := db.WALState{SizeBytes: 8 << 20, ModifiedAt: time.Unix(10, 20)}
	if !walStateAlreadyCheckpointed(checkpointed, checkpointed, true) {
		t.Fatal("identical WAL version must not schedule another passive checkpoint")
	}
	if walStateAlreadyCheckpointed(checkpointed, checkpointed, false) {
		t.Fatal("WAL without a completed checkpoint must remain eligible")
	}
	changed := checkpointed
	changed.ModifiedAt = changed.ModifiedAt.Add(time.Nanosecond)
	if walStateAlreadyCheckpointed(changed, checkpointed, true) {
		t.Fatal("a new WAL file version must re-arm checkpoint maintenance")
	}
}
