// Package supervisor orchestrates the desktop runtime: it owns machine-local
// paths and runs the existing Go API server in-process, reusing the same
// bootstrap (server/app) the CLI uses. The React UI continues to talk to the
// server over HTTP at http://localhost:6680.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"server/app"
)

const (
	serverPort = "6680"

	serverReadyTimeout = 60 * time.Second
	serverStopTimeout  = 20 * time.Second
)

// Startup stages reported through Options.OnStage so the tray can show what the
// runtime is doing instead of a single static "Starting…" that looks like a
// freeze when a stage is slow. The values are stable machine keys; the host
// maps them to localized labels.
const (
	StagePreparing      = "preparing"
	StageStartingServer = "starting_server"
	StageReady          = "ready"
)

// ErrAlreadyRunning is returned by AcquireLock / Start when another instance
// already holds the single-instance lock.
var ErrAlreadyRunning = errors.New("Lumilio Photos is already running")

// ErrStorageUnreachable indicates the persisted media library location could not
// be reached (e.g. an external drive is unmounted).
var ErrStorageUnreachable = errors.New("configured storage location is unreachable")

// ErrPortInUse indicates the fixed app port is already bound by another process
// (a stale server, a dev instance, or an unrelated app). Without this pre-flight
// the in-process server would fail to bind after initializing SQLite, and the
// browser would silently reach the foreign process instead — showing a 404.
var ErrPortInUse = errors.New("the app port is already in use")

// ErrOperationInProgress reports that another lifecycle mutation already owns
// the serialized runtime operation gate.
var ErrOperationInProgress = errors.New("a desktop runtime operation is already in progress")

// ErrRuntimeGenerationActive prevents a second server/app generation from
// starting before the previous generation has proved it released listeners,
// River workers, and SQLite.
var ErrRuntimeGenerationActive = errors.New("the previous desktop runtime generation is still active")

// ErrRuntimeStopTimeout means cancellation was requested but generation.done
// did not close within the shutdown budget. Ownership is retained.
var ErrRuntimeStopTimeout = errors.New("desktop runtime shutdown timed out")

// Supervisor owns the desktop runtime lifecycle. Start it once on app launch and
// Close it on quit. Runtime restarts do not release the host instance lock.
type Supervisor struct {
	logf             func(string, ...any)
	onStage          func(string)
	onSnapshot       func(RuntimeSnapshot)
	operatorControls app.OperatorControls

	paths           *Paths
	lock            *InstanceLock
	applyReconciled bool

	operationMu  sync.Mutex
	generation   *runtimeGeneration
	stopTimeout  time.Duration
	readyTimeout time.Duration

	snapshotMu sync.RWMutex
	snapshot   RuntimeSnapshot

	warnings []string
	// pendingStorageRoot migrates the pre-Storage-Location desktop setting after
	// the in-process repository control plane is ready.
	pendingStorageRoot string

	repositoryMu      sync.RWMutex
	repositoryManager app.RepositoryControl
}

// Options configures a Supervisor.
type Options struct {
	// Logf receives human-readable lifecycle messages. Defaults to log.Printf.
	Logf func(string, ...any)

	// OnStage, if set, is called with a Stage* key each time Start advances to a
	// new phase, letting the host surface progress. It may be called from a
	// non-UI goroutine, so the host must marshal to its UI thread itself.
	OnStage func(stage string)

	// OnSnapshot receives the same typed runtime state consumed by the private
	// panel. It is invoked outside the snapshot mutex.
	OnSnapshot func(snapshot RuntimeSnapshot)

	// OperatorControls are explicit controls for this single desktop launch.
	// They are never persisted to desktop settings or the generated manifest.
	OperatorControls app.OperatorControls
}

// New constructs a Supervisor.
func New(opts Options) *Supervisor {
	logf := opts.Logf
	if logf == nil {
		logf = log.Printf
	}
	s := &Supervisor{
		logf: logf, onStage: opts.OnStage, onSnapshot: opts.OnSnapshot,
		operatorControls: opts.OperatorControls,
		stopTimeout:      serverStopTimeout,
		readyTimeout:     serverReadyTimeout,
		snapshot:         initialRuntimeSnapshot(),
	}
	hostHook := s.operatorControls.RepositoryManagerReady
	s.operatorControls.RepositoryManagerReady = func(manager app.RepositoryControl) {
		s.repositoryMu.Lock()
		s.repositoryManager = manager
		s.repositoryMu.Unlock()
		if manager != nil {
			s.migratePendingStorageRoot(manager)
		}
		if hostHook != nil {
			hostHook(manager)
		}
	}
	return s
}

func (s *Supervisor) migratePendingStorageRoot(control app.RepositoryControl) {
	path := strings.TrimSpace(s.pendingStorageRoot)
	if path == "" || s.paths == nil || path == s.paths.DefaultLib {
		return
	}
	if _, _, err := control.AddStorageLocation(context.Background(), path, filepath.Base(path)); err != nil {
		s.warnings = append(s.warnings, fmt.Sprintf("legacy Storage Location %q needs attention: %v", path, err))
		s.logf("migrate legacy Storage Location %q: %v", path, err)
		return
	}
	settings, err := LoadSettings(s.paths.DesktopSettingsFile())
	if err == nil {
		settings.StoragePath = s.paths.DefaultLib
		err = SaveSettings(s.paths.DesktopSettingsFile(), settings)
	}
	if err != nil {
		s.logf("persist legacy Storage Location migration: %v", err)
		return
	}
	s.pendingStorageRoot = ""
}

// RepositoryControl returns the in-process storage control plane after the
// server reaches repository initialization. It is intentionally not exposed by
// the shared HTTP API.
func (s *Supervisor) RepositoryControl() (app.RepositoryControl, error) {
	s.repositoryMu.RLock()
	manager := s.repositoryManager
	s.repositoryMu.RUnlock()
	if manager == nil {
		return nil, errors.New("repository control plane is not ready")
	}
	return manager, nil
}

// reportStage logs and, if configured, notifies the host that Start has entered
// a new phase.
func (s *Supervisor) reportStage(stage string) {
	s.logf("desktop stage: %s", stage)
	s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
		snapshot.Stage = stage
	})
	if s.onStage != nil {
		s.onStage(stage)
	}
}

// ensurePaths resolves the app-data path tree once, so onboarding helpers can run
// before Start. Start also calls it; both share the same resolved Paths.
func (s *Supervisor) ensurePaths() error {
	if s.paths != nil {
		return nil
	}
	paths, err := NewPaths()
	if err != nil {
		return err
	}
	if err := paths.EnsureDirs(); err != nil {
		return err
	}
	s.paths = paths
	return nil
}

// Settings returns the persisted desktop settings (storage path, onboarding
// state, native language). Safe to call before Start.
func (s *Supervisor) Settings() (DesktopSettings, error) {
	if err := s.ensurePaths(); err != nil {
		return DesktopSettings{}, err
	}
	settings, err := LoadSettings(s.paths.DesktopSettingsFile())
	if err != nil {
		return DesktopSettings{}, err
	}
	_, runtimeErr := os.Stat(s.paths.RuntimeConfigFile())
	needsRuntimeInitialization := errors.Is(runtimeErr, os.ErrNotExist)
	if runtimeErr != nil && !needsRuntimeInitialization {
		return DesktopSettings{}, fmt.Errorf("inspect runtime intent: %w", runtimeErr)
	}
	// Host settings remain readable when an established runtime intent is
	// invalid, so a returning user reaches the recovery Dashboard.
	if needsRuntimeInitialization {
		if err := s.ensureRuntimeIntent(settings); err != nil {
			return DesktopSettings{}, err
		}
		settings, err = LoadSettings(s.paths.DesktopSettingsFile())
		if err != nil {
			return DesktopSettings{}, err
		}
	}
	settings.Version = desktopSettingsVersion
	return settings, nil
}

// SaveSettings persists the full desktop settings. Safe to call before Start.
func (s *Supervisor) SaveSettings(settings DesktopSettings) error {
	if err := s.ensurePaths(); err != nil {
		return err
	}
	return SaveSettings(s.paths.DesktopSettingsFile(), settings)
}

// NeedsOnboarding reports whether the native onboarding window should be shown:
// on first run, or when the accepted terms revision is older than the required
// one (so bumping the host's tosVersion re-prompts existing users). A read error
// is treated as "needs onboarding" so a corrupt settings file re-runs setup
// rather than booting with no validated storage location.
func (s *Supervisor) NeedsOnboarding(requiredTOSVersion string) bool {
	settings, err := s.Settings()
	if err != nil {
		s.logf("read settings for onboarding check (treating as first run): %v", err)
		return true
	}
	return !settings.OnboardingCompleted || settings.TOSAcceptedVersion != requiredTOSVersion
}

// LogDir returns the in-process server/application log directory, so the host can
// point the user at it in a failure dialog. Empty if paths cannot be resolved.
func (s *Supervisor) LogDir() string {
	if err := s.ensurePaths(); err != nil {
		return ""
	}
	return s.paths.Logs
}

// DashboardPaths returns non-secret host paths which the private Desktop
// control panel may display or reveal in Finder/Explorer.
func (s *Supervisor) DashboardPaths() (map[string]string, error) {
	if err := s.ensurePaths(); err != nil {
		return nil, err
	}
	return map[string]string{
		"appData": s.paths.AppData, "storage": s.paths.DefaultLib, "logs": s.paths.Logs,
		"backups": s.paths.Backups, "lumen": s.paths.LumenDir(),
		"serverConfig": s.paths.ServerConfigFile(),
	}, nil
}

// DefaultStoragePath is the built-in media-library location used until the user
// chooses one during onboarding (<appdata>/storage).
func (s *Supervisor) DefaultStoragePath() (string, error) {
	if err := s.ensurePaths(); err != nil {
		return "", err
	}
	return s.paths.DefaultLib, nil
}

// LumenDir is where the supervised Lumen Hub lives (build + config + model
// cache). Safe to call before Start.
func (s *Supervisor) LumenDir() (string, error) {
	if err := s.ensurePaths(); err != nil {
		return "", err
	}
	return s.paths.LumenDir(), nil
}

// StorageReachable reports whether the given media-library location exists or can
// be created (its parent exists). Exposed for the onboarding window's live
// validation.
func StorageReachable(path string) bool { return storageReachable(path) }

// ServerURL is the local browser address the Desktop host opens.
func (s *Supervisor) ServerURL() string {
	if origin := s.RuntimeSnapshot().BrowserURL; origin != "" {
		return origin
	}
	if err := s.ensurePaths(); err == nil {
		if _, cfg, loadErr := s.runtimeIntent(); loadErr == nil {
			return browserURLForListen(cfg.ServerConfig.Listen)
		}
	}
	return "http://localhost:" + serverPort
}

// Warnings returns non-fatal issues surfaced during Start (e.g. a fallback to
// the default storage location). The UI may present these to the user.
func (s *Supervisor) Warnings() []string { return s.warnings }

// LANAddresses returns routable local interface addresses for display only.
// Desktop does not modify firewall rules or derive trust from this list.
func LANAddresses() []string {
	var addresses []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return addresses
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addrs {
			raw, _, _ := strings.Cut(address.String(), "/")
			ip := net.ParseIP(raw)
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				continue
			}
			addresses = append(addresses, ip.String())
		}
	}
	return addresses
}

// Prepare acquires the host-level single-instance lock. Runtime failures and
// restarts do not release it; only Close ends the Desktop host lifetime.
func (s *Supervisor) Prepare() error {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	return s.prepareLocked()
}

func (s *Supervisor) prepareLocked() error {
	if err := s.ensurePaths(); err != nil {
		return err
	}
	_, lkgErr := os.Stat(s.paths.RuntimeLastKnownGoodFile())
	s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
		snapshot.LastKnownGoodAvailable = lkgErr == nil
	})
	if s.lock == nil {
		lock, err := AcquireLock(s.paths.LockFile())
		if err != nil {
			return err
		}
		s.lock = lock
	}
	if !s.applyReconciled {
		if err := s.reconcileRuntimeApply(); err != nil {
			return err
		}
		s.applyReconciled = true
	}
	return nil
}

// Start launches the first runtime generation. It leaves the host lock held if
// startup fails so the Wails recovery dashboard remains the sole host owner.
func (s *Supervisor) Start(ctx context.Context) error {
	if !s.operationMu.TryLock() {
		return ErrOperationInProgress
	}
	defer s.operationMu.Unlock()
	if err := s.prepareLocked(); err != nil {
		s.failRuntime(err)
		return err
	}
	return s.startRuntimeLocked(ctx, RuntimeStarting)
}

func (s *Supervisor) startRuntimeLocked(ctx context.Context, phase RuntimePhase) error {
	return s.startRuntimeLockedWithOperation(ctx, phase, false)
}

func (s *Supervisor) startRuntimeLockedWithOperation(
	ctx context.Context,
	phase RuntimePhase,
	keepOperationActive bool,
) error {
	if err := s.reapFinishedGenerationLocked(); err != nil {
		s.logf("previous desktop runtime generation exited with error: %v", err)
	}
	if s.generation != nil {
		return ErrRuntimeGenerationActive
	}

	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = phase
	snapshot.Stage = StagePreparing
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = true
	s.setSnapshot(snapshot)
	s.reportStage(StagePreparing)

	resources, err := ResourcesDir()
	if err != nil {
		err = fmt.Errorf("resolve resources dir: %w", err)
		s.failRuntime(err)
		return err
	}
	if vipsHome := bundledVipsHome(resources); vipsHome != "" && os.Getenv("VIPSHOME") == "" {
		if err := os.Setenv("VIPSHOME", vipsHome); err != nil {
			err = fmt.Errorf("set VIPSHOME: %w", err)
			s.failRuntime(err)
			return err
		}
	}
	if err := stripQuarantine(resources); err != nil {
		s.logf("quarantine cleanup (non-fatal): %v", err)
	}

	storagePath, err := s.resolveStoragePath()
	if err != nil {
		s.failRuntime(err)
		return err
	}
	if err := os.MkdirAll(storagePath, 0o755); err != nil {
		err = fmt.Errorf("create storage path %s: %w", storagePath, err)
		s.failRuntime(err)
		return err
	}

	s.reportStage(StageStartingServer)
	appConfig, err := s.materializeRuntimeConfig()
	if err != nil {
		err = fmt.Errorf("compile desktop server manifest: %w", err)
		s.failRuntime(err)
		return err
	}
	network := networkSummaryFromConfig(appConfig)
	s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
		snapshot.BrowserURL = browserURLForListen(appConfig.ServerConfig.Listen)
		snapshot.Network = network
		_, lkgErr := os.Stat(s.paths.RuntimeLastKnownGoodFile())
		snapshot.LastKnownGoodAvailable = lkgErr == nil
	})

	// Check the resolved listener immediately before generation creation. A
	// foreign listener must not be mistaken for successful readiness.
	if err := s.checkListenAvailable(appConfig.ServerConfig.Listen); err != nil {
		s.failRuntime(err)
		return err
	}

	srvCtx, cancel := context.WithCancel(ctx)
	generation := &runtimeGeneration{cancel: cancel, done: make(chan struct{})}
	s.generation = generation
	go func() {
		generation.err = app.Run(srvCtx, appConfig, s.operatorControls)
		close(generation.done)
	}()

	if err := s.waitForServer(ctx, generation, appConfig.ServerConfig.Listen); err != nil {
		if stopErr := s.stopGenerationLocked(); stopErr != nil {
			err = errors.Join(err, stopErr)
		}
		s.failRuntime(err)
		return err
	}
	s.reportStage(StageReady)
	snapshot = s.RuntimeSnapshot()
	snapshot.Phase = RuntimeRunning
	snapshot.Stage = StageReady
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = keepOperationActive
	s.setSnapshot(snapshot)
	s.logf("desktop runtime ready at %s", snapshot.BrowserURL)
	return nil
}

func browserURLForListen(listen string) string {
	_, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil || port == "" {
		port = serverPort
	}
	return "http://localhost:" + port
}

// StopRuntime drains only the current Server generation. It never releases the
// host lock. A timeout leaves generation ownership intact.
func (s *Supervisor) StopRuntime() error {
	if !s.operationMu.TryLock() {
		return ErrOperationInProgress
	}
	defer s.operationMu.Unlock()
	return s.stopRuntimeLocked()
}

func (s *Supervisor) stopRuntimeLocked() error {
	s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
		snapshot.OperationActive = true
	})
	err := s.stopGenerationLocked()
	if err != nil {
		s.failRuntime(err)
		return err
	}
	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = RuntimeStopped
	snapshot.Stage = ""
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = false
	s.setSnapshot(snapshot)
	return nil
}

func (s *Supervisor) stopGenerationLocked() error {
	generation := s.generation
	if generation == nil {
		return nil
	}
	s.clearRepositoryControl()
	generation.cancel()
	timeout := s.stopTimeout
	if timeout <= 0 {
		timeout = serverStopTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-generation.done:
		s.generation = nil
		if generation.err != nil {
			s.logf("api server shutdown error: %v", generation.err)
			return generation.err
		}
		return nil
	case <-timer.C:
		err := fmt.Errorf("%w after %s", ErrRuntimeStopTimeout, timeout)
		s.logf("%v", err)
		return err
	}
}

func (s *Supervisor) reapFinishedGenerationLocked() error {
	if s.generation == nil {
		return nil
	}
	select {
	case <-s.generation.done:
		generation := s.generation
		s.generation = nil
		return generation.err
	default:
		return nil
	}
}

func (s *Supervisor) clearRepositoryControl() {
	s.repositoryMu.Lock()
	s.repositoryManager = nil
	s.repositoryMu.Unlock()
}

func (s *Supervisor) failRuntime(err error) {
	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = RuntimeFailed
	snapshot.ErrorCode = runtimeErrorCode(err)
	snapshot.ErrorMessage = runtimeErrorMessage(err)
	snapshot.OperationActive = false
	s.setSnapshot(snapshot)
}

// Restart synchronously replaces the current generation while retaining the
// host lock. It is used by tests and internal apply flows.
func (s *Supervisor) Restart(ctx context.Context) error {
	if !s.operationMu.TryLock() {
		return ErrOperationInProgress
	}
	defer s.operationMu.Unlock()
	return s.restartLocked(ctx)
}

func (s *Supervisor) restartLocked(ctx context.Context) error {
	if err := s.prepareLocked(); err != nil {
		s.failRuntime(err)
		return err
	}
	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = RuntimeRestarting
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = true
	s.setSnapshot(snapshot)
	if err := s.stopGenerationLocked(); err != nil {
		s.failRuntime(err)
		return err
	}
	return s.startRuntimeLocked(ctx, RuntimeRestarting)
}

// RestartAsync claims the operation gate before returning so concurrent panel
// requests receive ErrOperationInProgress deterministically.
func (s *Supervisor) RestartAsync(ctx context.Context) error {
	if !s.operationMu.TryLock() {
		return ErrOperationInProgress
	}
	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = RuntimeRestarting
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = true
	s.setSnapshot(snapshot)
	go func() {
		defer s.operationMu.Unlock()
		_ = s.restartLocked(ctx)
	}()
	return nil
}

// Close drains the Runtime and releases the host lock. If the generation does
// not stop, the lock remains held until process exit so a second Desktop host
// cannot race the still-owned SQLite/listener generation.
func (s *Supervisor) Close() error {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	stopErr := s.stopGenerationLocked()
	if stopErr != nil {
		s.failRuntime(stopErr)
		return stopErr
	}
	var lockErr error
	if s.lock != nil {
		lockErr = s.lock.Release()
		s.lock = nil
	}
	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = RuntimeStopped
	snapshot.Stage = ""
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = false
	s.setSnapshot(snapshot)
	return lockErr
}

// Stop is retained as a host-shutdown compatibility alias. Runtime restart
// code must use StopRuntime/restartLocked so it cannot release the host lock.
func (s *Supervisor) Stop() error { return s.Close() }

// resolveStoragePath returns the machine-local default Storage Location. A
// pre-Storage-Location user choice is migrated as an external authorized root
// after the repository control plane is ready; it is never substituted as app
// state and an unavailable path is never silently recreated.
func (s *Supervisor) resolveStoragePath() (string, error) {
	settingsFile := s.paths.DesktopSettingsFile()
	settings, err := LoadSettings(settingsFile)
	if err != nil {
		return "", err
	}

	if settings.StoragePath == "" {
		settings.StoragePath = s.paths.DefaultLib
		if err := SaveSettings(settingsFile, settings); err != nil {
			return "", err
		}
		return s.paths.DefaultLib, nil
	}
	if settings.StoragePath != s.paths.DefaultLib {
		s.pendingStorageRoot = settings.StoragePath
		if !storageReachable(settings.StoragePath) {
			s.warnings = append(s.warnings, fmt.Sprintf(
				"%v: %q — it remains offline; the local default Storage Location is unchanged", ErrStorageUnreachable, settings.StoragePath))
			s.logf("legacy external storage %q is unreachable; leaving it offline", settings.StoragePath)
		}
	}
	return s.paths.DefaultLib, nil
}

// checkPortAvailable verifies the app port can be bound, matching the address
// the in-process server listens on (all interfaces). It returns ErrPortInUse
// (wrapping the bind error) when something else already holds the port.
func (s *Supervisor) checkPortAvailable(port string) error {
	return s.checkListenAvailable(net.JoinHostPort("", port))
}

func (s *Supervisor) checkListenAvailable(address string) error {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("%w (%s): %v", ErrPortInUse, address, err)
	}
	return ln.Close()
}

// waitForServer polls the generation's internal health endpoint until it
// responds with HTTP 200 or the exact generation exits. Redirects, 4xx, and
// 5xx must not promote a candidate to last-known-good.
func (s *Supervisor) waitForServer(ctx context.Context, generation *runtimeGeneration, listen string) error {
	url := internalHealthURL(listen)
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	timeout := s.readyTimeout
	if timeout <= 0 {
		timeout = serverReadyTimeout
	}
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-generation.done:
			if generation.err != nil {
				return fmt.Errorf("api server exited during startup: %w", generation.err)
			}
			return errors.New("api server exited during startup")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if resp, err := client.Get(url); err == nil {
			status := resp.StatusCode
			resp.Body.Close()
			if readinessStatusOK(status) {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("api server not ready after %s", timeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func readinessStatusOK(statusCode int) bool {
	return statusCode == http.StatusOK
}

func internalHealthURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return ""
	}
	ip := net.ParseIP(host)
	if host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, port),
		Path:   "/api/v1/health/ready",
	}).String()
}

// storageReachable reports whether path exists or can be created (its parent
// directory exists). An unmounted external drive fails both checks.
func storageReachable(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Dir(path)); err == nil {
		return true
	}
	return false
}
