package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
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
