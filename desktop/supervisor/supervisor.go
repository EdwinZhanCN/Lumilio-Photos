// Package supervisor orchestrates the desktop runtime: it manages a private,
// bundled PostgreSQL instance and runs the existing Go API server in-process,
// reusing the same bootstrap (server/app) the CLI uses. The React UI continues
// to talk to the server over HTTP at http://localhost:6680.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"server/app"
	serverconfig "server/config"
)

const (
	dbUser     = "lumilio"
	dbName     = "lumiliophotos"
	serverPort = "6680"

	pgReadyTimeout     = 30 * time.Second
	serverReadyTimeout = 60 * time.Second
	serverStopTimeout  = 20 * time.Second
	pgStopTimeout      = 35 * time.Second
)

// Startup stages reported through Options.OnStage so the tray can show what the
// runtime is doing instead of a single static "Starting…" that looks like a
// freeze when a stage is slow. The values are stable machine keys; the host
// maps them to localized labels.
const (
	StagePreparing      = "preparing"
	StageInitDB         = "initializing_database"
	StageStartingDB     = "starting_database"
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
// the in-process server would fail to bind after a full PG startup, and the
// browser would silently reach the foreign process instead — showing a 404.
var ErrPortInUse = errors.New("the app port is already in use")

// Supervisor owns the desktop runtime lifecycle. Start it once on app launch and
// Stop it on quit.
type Supervisor struct {
	logf    func(string, ...any)
	onStage func(string)

	paths *Paths
	pg    *Postgres
	lock  *InstanceLock

	cancel    context.CancelFunc
	serverErr chan error

	warnings []string
}

// Options configures a Supervisor.
type Options struct {
	// Logf receives human-readable lifecycle messages. Defaults to log.Printf.
	Logf func(string, ...any)

	// OnStage, if set, is called with a Stage* key each time Start advances to a
	// new phase, letting the host surface progress. It may be called from a
	// non-UI goroutine, so the host must marshal to its UI thread itself.
	OnStage func(stage string)
}

// New constructs a Supervisor.
func New(opts Options) *Supervisor {
	logf := opts.Logf
	if logf == nil {
		logf = log.Printf
	}
	return &Supervisor{logf: logf, onStage: opts.OnStage}
}

// reportStage logs and, if configured, notifies the host that Start has entered
// a new phase.
func (s *Supervisor) reportStage(stage string) {
	s.logf("desktop stage: %s", stage)
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
	return LoadSettings(s.paths.DesktopSettingsFile())
}

// SaveSettings persists the full desktop settings. Safe to call before Start.
func (s *Supervisor) SaveSettings(settings DesktopSettings) error {
	if err := s.ensurePaths(); err != nil {
		return err
	}
	return SaveSettings(s.paths.DesktopSettingsFile(), settings)
}

// NeedsOnboarding reports whether the first-run native onboarding window should
// be shown. A read error is treated as "needs onboarding" so a corrupt settings
// file re-runs setup rather than booting with no validated storage location.
func (s *Supervisor) NeedsOnboarding() bool {
	settings, err := s.Settings()
	if err != nil {
		s.logf("read settings for onboarding check (treating as first run): %v", err)
		return true
	}
	return !settings.OnboardingCompleted
}

// LogDir returns the in-process server/application log directory, so the host can
// point the user at it in a failure dialog. Empty if paths cannot be resolved.
func (s *Supervisor) LogDir() string {
	if err := s.ensurePaths(); err != nil {
		return ""
	}
	return s.paths.Logs
}

// DefaultStoragePath is the built-in media-library location used until the user
// chooses one during onboarding (<appdata>/storage).
func (s *Supervisor) DefaultStoragePath() (string, error) {
	if err := s.ensurePaths(); err != nil {
		return "", err
	}
	return s.paths.DefaultLib, nil
}

// StorageReachable reports whether the given media-library location exists or can
// be created (its parent exists). Exposed for the onboarding window's live
// validation.
func StorageReachable(path string) bool { return storageReachable(path) }

// ServerURL is the address the desktop host opens in the user's browser. It is
// localhost (not 127.0.0.1, not a custom scheme) because WebAuthn/passkeys
// require localhost as the relying-party origin.
func (s *Supervisor) ServerURL() string {
	return "http://localhost:" + serverPort
}

// Warnings returns non-fatal issues surfaced during Start (e.g. a fallback to
// the default storage location). The UI may present these to the user.
func (s *Supervisor) Warnings() []string { return s.warnings }

// Start brings up PostgreSQL, generates runtime configuration, and launches the
// API server in-process. It returns once the server is accepting requests so
// the caller can show the UI window, or an error if any step fails.
func (s *Supervisor) Start(ctx context.Context) error {
	s.reportStage(StagePreparing)
	if err := s.ensurePaths(); err != nil {
		return err
	}
	paths := s.paths

	lock, err := AcquireLock(paths.LockFile())
	if err != nil {
		return err
	}
	s.lock = lock

	// Fail fast (before the expensive PG startup) if the app port is taken. The
	// single-instance lock already prevents a second desktop instance, so a busy
	// port means a foreign process — otherwise the in-process server would boot,
	// fail to bind, tear itself down, and leave the browser reaching the squatter.
	if err := s.checkPortAvailable(serverPort); err != nil {
		return err
	}

	resources, err := ResourcesDir()
	if err != nil {
		return fmt.Errorf("resolve resources dir: %w", err)
	}
	if vipsHome := bundledVipsHome(resources); vipsHome != "" && os.Getenv("VIPSHOME") == "" {
		if err := os.Setenv("VIPSHOME", vipsHome); err != nil {
			return fmt.Errorf("set VIPSHOME: %w", err)
		}
	}

	// Strip Gatekeeper quarantine from bundled resources so exec of the PG
	// binaries is not blocked. Done every launch (idempotent, non-fatal) so it
	// also covers an app update that re-quarantines the binaries.
	if err := stripQuarantine(resources); err != nil {
		s.logf("quarantine cleanup (non-fatal): %v", err)
	}

	storagePath, err := s.resolveStoragePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storagePath, 0o755); err != nil {
		return fmt.Errorf("create storage path %s: %w", storagePath, err)
	}

	if err := ensureSecret(paths.DBPasswordFile()); err != nil {
		return err
	}
	if err := ensureSecret(paths.SecretKeyFile()); err != nil {
		return err
	}

	pg := NewPostgres(PostgresOptions{
		BinDir:       pgBinDir(resources),
		DataDir:      paths.PGData,
		Host:         paths.DBHost(),
		LogsDir:      paths.PGLogs,
		Port:         pgPort,
		User:         dbUser,
		DBName:       dbName,
		PasswordFile: paths.DBPasswordFile(),
		Logf:         s.logf,
	})
	s.pg = pg

	if !pg.IsInitialized() {
		s.reportStage(StageInitDB)
		if err := pg.InitDB(ctx); err != nil {
			return err
		}
	}

	s.reportStage(StageStartingDB)
	if err := pg.WriteConfigs(); err != nil {
		return err
	}
	if err := pg.HandleStaleState(ctx); err != nil {
		return err
	}
	if err := pg.Start(ctx); err != nil {
		return err
	}
	if err := pg.WaitReady(ctx, pgReadyTimeout); err != nil {
		return err
	}
	if err := pg.CreateDB(ctx); err != nil {
		return err
	}

	s.reportStage(StageStartingServer)
	appConfig, err := serverconfig.NewDesktopConfig(serverconfig.DesktopParams{
		Port:          serverPort,
		WebRoot:       bundledWebRoot(resources),
		LogDir:        paths.Logs,
		StoragePath:   storagePath,
		DBHost:        paths.DBHost(),
		PGPort:        pgPort,
		DBUser:        dbUser,
		DBName:        dbName,
		PasswordFile:  paths.DBPasswordFile(),
		SecretKeyPath: paths.SecretKeyFile(),
		ExifToolPath:  bundledExifTool(resources),
		FFmpegPath:    bundledFFmpeg(resources),
		FFprobePath:   bundledFFprobe(resources),
	})
	if err != nil {
		return fmt.Errorf("build desktop server config: %w", err)
	}
	if err := serverconfig.WriteGeneratedTOML(paths.ServerConfigFile(), appConfig); err != nil {
		s.logf("write generated server config debug copy (non-fatal): %v", err)
	}

	// Run the API server in-process. It blocks until srvCtx is cancelled (Stop)
	// and then performs its own graceful shutdown.
	srvCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.serverErr = make(chan error, 1)
	go func() { s.serverErr <- app.Run(srvCtx, appConfig) }()

	if err := s.waitForServer(ctx); err != nil {
		return err
	}
	s.reportStage(StageReady)
	s.logf("desktop runtime ready at %s", s.ServerURL())
	return nil
}

// Stop performs an ordered shutdown: drain the API server, stop PostgreSQL, then
// release the single-instance lock. It is safe to call more than once.
func (s *Supervisor) Stop() error {
	var firstErr error
	setErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if s.cancel != nil {
		s.cancel()
		select {
		case err := <-s.serverErr:
			if err != nil {
				s.logf("api server shutdown error: %v", err)
				setErr(err)
			}
		case <-time.After(serverStopTimeout):
			s.logf("api server shutdown timed out after %s", serverStopTimeout)
		}
		s.cancel = nil
	}

	if s.pg != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), pgStopTimeout)
		if err := s.pg.Stop(stopCtx); err != nil {
			s.logf("postgres stop error: %v", err)
			setErr(err)
		}
		cancel()
		s.pg = nil
	}

	if s.lock != nil {
		setErr(s.lock.Release())
		s.lock = nil
	}

	return firstErr
}

// resolveStoragePath returns the media library location, persisting a default on
// first run. If a previously chosen location is unreachable (external drive
// unmounted), it falls back to the default and records a warning rather than
// crashing; the UI can later offer to re-pick.
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
		return settings.StoragePath, nil
	}

	if !storageReachable(settings.StoragePath) {
		s.warnings = append(s.warnings, fmt.Sprintf(
			"%v: %q — using default location instead", ErrStorageUnreachable, settings.StoragePath))
		s.logf("storage %q unreachable; falling back to default %q", settings.StoragePath, s.paths.DefaultLib)
		return s.paths.DefaultLib, nil
	}
	return settings.StoragePath, nil
}

// SetStoragePath persists a user-chosen media library location, preserving the
// other persisted settings. It takes effect on the next launch. An empty path
// resets to the default.
func (s *Supervisor) SetStoragePath(path string) error {
	settings, err := s.Settings()
	if err != nil {
		return err
	}
	settings.StoragePath = path
	return s.SaveSettings(settings)
}

// checkPortAvailable verifies the app port can be bound, matching the address
// the in-process server listens on (all interfaces). It returns ErrPortInUse
// (wrapping the bind error) when something else already holds the port.
func (s *Supervisor) checkPortAvailable(port string) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		return fmt.Errorf("%w (port %s): %v", ErrPortInUse, port, err)
	}
	return ln.Close()
}

// waitForServer polls the health endpoint until the server responds or fails.
func (s *Supervisor) waitForServer(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/health", s.ServerURL())
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(serverReadyTimeout)

	for {
		select {
		case err := <-s.serverErr:
			if err != nil {
				return fmt.Errorf("api server exited during startup: %w", err)
			}
			return errors.New("api server exited during startup")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode < http.StatusInternalServerError {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("api server not ready after %s", serverReadyTimeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
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
