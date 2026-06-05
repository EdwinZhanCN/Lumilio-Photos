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

// ErrAlreadyRunning is returned by AcquireLock / Start when another instance
// already holds the single-instance lock.
var ErrAlreadyRunning = errors.New("Lumilio Photos is already running")

// ErrStorageUnreachable indicates the persisted media library location could not
// be reached (e.g. an external drive is unmounted).
var ErrStorageUnreachable = errors.New("configured storage location is unreachable")

// Supervisor owns the desktop runtime lifecycle. Start it once on app launch and
// Stop it on quit.
type Supervisor struct {
	logf func(string, ...any)

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
}

// New constructs a Supervisor.
func New(opts Options) *Supervisor {
	logf := opts.Logf
	if logf == nil {
		logf = log.Printf
	}
	return &Supervisor{logf: logf}
}

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
	paths, err := NewPaths()
	if err != nil {
		return err
	}
	s.paths = paths
	if err := paths.EnsureDirs(); err != nil {
		return err
	}

	lock, err := AcquireLock(paths.LockFile())
	if err != nil {
		return err
	}
	s.lock = lock

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
		SocketDir:    paths.SocketDir(),
		LogsDir:      paths.PGLogs,
		Port:         pgPort,
		User:         dbUser,
		DBName:       dbName,
		PasswordFile: paths.DBPasswordFile(),
		Logf:         s.logf,
	})
	s.pg = pg

	if !pg.IsInitialized() {
		if err := pg.InitDB(ctx); err != nil {
			return err
		}
	}
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

	appConfig, err := serverconfig.NewDesktopConfig(serverconfig.DesktopParams{
		Port:          serverPort,
		WebRoot:       bundledWebRoot(resources),
		LogDir:        paths.Logs,
		StoragePath:   storagePath,
		SocketDir:     paths.SocketDir(),
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

// SetStoragePath persists a user-chosen media library location. It takes effect
// on the next launch. An empty path resets to the default.
func (s *Supervisor) SetStoragePath(path string) error {
	if s.paths == nil {
		return errors.New("supervisor not started")
	}
	return SaveSettings(s.paths.DesktopSettingsFile(), DesktopSettings{StoragePath: path})
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
