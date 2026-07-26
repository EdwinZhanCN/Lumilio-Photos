package supervisor

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	serverapp "server/app"
)

func TestCheckPortAvailable(t *testing.T) {
	s := New(Options{Logf: func(string, ...any) {}})

	// Occupy the port the same way the server binds it (all interfaces), matching
	// the real conflict the pre-flight guards against.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("sandbox does not permit loopback listeners: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	if err := s.checkPortAvailable(port); !errors.Is(err, ErrPortInUse) {
		t.Errorf("occupied port: got %v, want ErrPortInUse", err)
	}

	// Once freed, the same port is available again.
	_ = ln.Close()
	if err := s.checkPortAvailable(port); err != nil {
		t.Errorf("freed port: got %v, want nil", err)
	}
}

// The generated desktop TOML is written privately and then loaded through the
// same strict server/config boundary used by standalone.
func TestDesktopServerConfigInvariants(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.toml")
	webRoot := filepath.Join(dir, "bundle", "web")
	logDir := filepath.Join(dir, "appdata", "logs")
	storagePath := filepath.Join(dir, "library")
	databasePath := filepath.Join(dir, "appdata", "library.sqlite3")
	exifToolPath := filepath.Join(dir, "bundle", "exiftool")
	ffmpegPath := filepath.Join(dir, "bundle", "ffmpeg")
	ffprobePath := filepath.Join(dir, "bundle", "ffprobe")
	secretKeyFile := filepath.Join(dir, "secrets", "lumilio_secret_key")

	cfg, err := compileAndLoadServerManifest(path, serverManifestBindings{
		Listen: "127.0.0.1:6680", PrimaryOrigin: "http://localhost:6680", WebRoot: webRoot,
		TLSMode: "off", ProxyMode: "disabled", TrustedProxyCIDRs: []string{},
		LogDir: logDir, StoragePath: storagePath,
		CloudStatePath: filepath.Join(dir, "appdata", "cloud"), BackupsPath: filepath.Join(dir, "appdata", "backups"),
		DatabasePath: databasePath, SecretKeyFile: secretKeyFile,
		ExifToolPath: exifToolPath, FFmpegPath: ffmpegPath, FFprobePath: ffprobePath,
		LumenStaticNode: "127.0.0.1:50051",
	})
	if err != nil {
		t.Fatalf("compileAndLoadServerManifest: %v", err)
	}
	if cfg.Auth.PasskeyIdentity.RPID != "localhost" {
		t.Fatalf("webauthn rp id = %q, want localhost", cfg.Auth.PasskeyIdentity.RPID)
	}
	if got, want := cfg.Auth.PasskeyIdentity.Origin, "http://localhost:6680"; got != want {
		t.Fatalf("webauthn origin = %q, want %q", got, want)
	}
	if len(cfg.ServerConfig.CORSAllowedOrigins) != 0 {
		t.Fatalf(
			"desktop product Web is same-origin and the private control panel is outside server CORS, got origins %q",
			cfg.ServerConfig.CORSAllowedOrigins,
		)
	}
	if cfg.DatabaseConfig.Path != databasePath {
		t.Fatalf("database path = %q, want %q", cfg.DatabaseConfig.Path, databasePath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	manifest := string(data)
	for _, forbidden := range []string{"bootstrap_password", "rotated_password", "tools_bin", "host =", "port = \"5487\""} {
		if strings.Contains(manifest, forbidden) {
			t.Fatalf("generated SQLite manifest contains legacy database key %q:\n%s", forbidden, data)
		}
	}
	databaseLiteral, err := tomlLiteral(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(manifest, "schema_version = 3") || !strings.Contains(manifest, "path = "+databaseLiteral) {
		t.Fatalf("generated manifest is not schema v3 with the SQLite path:\n%s", data)
	}
	private, err := isPrivatePath(path)
	if err != nil || !private {
		t.Fatalf("manifest private = %v, err = %v", private, err)
	}
	if !cfg.LoadedFromManifest() || cfg.ManifestPath != path || cfg.ServerConfig.Listen != "127.0.0.1:6680" {
		t.Fatalf("manifest was not strict-loaded: %+v", cfg)
	}
	if cfg.Auth.PasskeyIdentity.RPID != "localhost" || cfg.Auth.PasskeyIdentity.Origin != "http://localhost:6680" {
		t.Fatalf("unexpected auth config: %+v", cfg.Auth)
	}
	if cfg.ServerConfig.WebRoot != webRoot || cfg.DatabaseConfig.Path != databasePath || cfg.Tools.FFmpegPath != ffmpegPath {
		t.Fatalf("unexpected generated config: db=%+v tools=%+v", cfg.DatabaseConfig, cfg.Tools)
	}
	network := networkSummaryFromConfig(cfg)
	if network.Mode != NetworkLocal || network.PrimaryOrigin != "http://localhost:6680" ||
		network.RPID != "localhost" || network.TLSMode != "off" ||
		network.ProxyMode != "disabled" || !network.PasskeyEnabled {
		t.Fatalf("runtime network summary was not derived from strict config: %+v", network)
	}
}

func TestDesktopManifestWriteFailureBlocksLoad(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := compileAndLoadServerManifest(filepath.Join(blocker, "server.toml"), serverManifestBindings{})
	if err == nil || !strings.Contains(err.Error(), "write desktop server manifest") {
		t.Fatalf("expected write failure, got %v", err)
	}
}

func TestDesktopTemplateRejectsMissingBindingsOnStrictReload(t *testing.T) {
	_, err := compileAndLoadServerManifest(filepath.Join(t.TempDir(), "server.toml"), serverManifestBindings{})
	if err == nil || !strings.Contains(err.Error(), "reload generated desktop server manifest") {
		t.Fatalf("expected incomplete bindings to fail strict reload, got %v", err)
	}
}

func TestDesktopSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-settings.json")

	// Missing file → zero value, no error.
	s, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("LoadSettings(missing): %v", err)
	}
	if s.StoragePath != "" {
		t.Errorf("expected empty StoragePath on first run, got %q", s.StoragePath)
	}

	want := DesktopSettings{StoragePath: "/Volumes/Photos/Lib"}
	want, err = normalizeNetworkSettings(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(path, want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	updated := DesktopSettings{StoragePath: "/Volumes/Photos/Updated", Language: "zh"}
	updated, err = normalizeNetworkSettings(updated)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(path, updated); err != nil {
		t.Fatalf("SaveSettings(replace): %v", err)
	}
	got, err = LoadSettings(path)
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatalf("replacement round trip = %+v/%v, want %+v/nil", got, err, updated)
	}
}

func TestDesktopNetworkProfiles(t *testing.T) {
	local, err := normalizeNetworkSettings(DesktopSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if local.Version != desktopSettingsVersion || local.NetworkMode != NetworkLocal ||
		local.Listen != "127.0.0.1:6680" || local.PrimaryOrigin != "http://localhost:6680" {
		t.Fatalf("local defaults = %+v", local)
	}

	lan, err := normalizeNetworkSettings(DesktopSettings{
		NetworkMode:                   NetworkLANHTTP,
		PrimaryOrigin:                 local.PrimaryOrigin,
		LANHTTPWarningAcceptedVersion: lanHTTPWarningCurrentVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if lan.Listen != "0.0.0.0:6680" || lan.PrimaryOrigin != local.PrimaryOrigin {
		t.Fatalf("LAN profile changed more than listen: %+v", lan)
	}

	external, err := normalizeNetworkSettings(DesktopSettings{
		NetworkMode:       NetworkExternalHTTPS,
		PrimaryOrigin:     "https://PHOTOS.example.com:443",
		Listen:            "127.0.0.1:6680",
		TrustedProxyCIDRs: []string{"127.0.0.1/32", "::1/128"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if external.PrimaryOrigin != "https://photos.example.com" ||
		!reflect.DeepEqual(external.TrustedProxyCIDRs, []string{"127.0.0.1/32", "::1/128"}) {
		t.Fatalf("same-host external profile = %+v", external)
	}

	remote, err := normalizeNetworkSettings(DesktopSettings{
		NetworkMode:       NetworkExternalHTTPS,
		PrimaryOrigin:     "https://photos.example.com",
		Listen:            "0.0.0.0:6680",
		TrustedProxyCIDRs: []string{"192.168.1.10/32"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := remote.TrustedProxyCIDRs; !reflect.DeepEqual(got, []string{"192.168.1.10/32"}) {
		t.Fatalf("remote trusted CIDRs = %v", got)
	}
}

func TestDesktopNetworkSettingsRejectInvalidWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-settings.json")
	if err := SaveSettings(path, DesktopSettings{}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	err = SaveSettings(path, DesktopSettings{
		NetworkMode:       NetworkExternalHTTPS,
		PrimaryOrigin:     "http://photos.example.com",
		Listen:            "0.0.0.0:6680",
		TrustedProxyCIDRs: []string{"0.0.0.0/0"},
	})
	if err == nil {
		t.Fatal("invalid external network profile was accepted")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatal("invalid network profile changed persisted settings")
	}
}

func TestInternalHealthURLIsIndependentFromPrimaryOrigin(t *testing.T) {
	if got, want := internalHealthURL("0.0.0.0:6680"), "http://127.0.0.1:6680/api/v1/health/ready"; got != want {
		t.Fatalf("health URL = %q, want %q", got, want)
	}
	if got, want := internalHealthURL("[::1]:7780"), "http://[::1]:7780/api/v1/health/ready"; got != want {
		t.Fatalf("IPv6 health URL = %q, want %q", got, want)
	}
}

func TestStopTimeoutRetainsGenerationAndBlocksSecondStart(t *testing.T) {
	t.Setenv("LUMILIO_APP_DATA", t.TempDir())
	s := New(Options{Logf: func(string, ...any) {}})
	s.stopTimeout = 10 * time.Millisecond
	generationCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	generation := &runtimeGeneration{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	s.generation = generation
	s.setSnapshot(RuntimeSnapshot{
		Phase:      RuntimeRunning,
		BrowserURL: "http://localhost:6680",
	})

	err := s.StopRuntime()
	if !errors.Is(err, ErrRuntimeStopTimeout) {
		t.Fatalf("StopRuntime error = %v, want ErrRuntimeStopTimeout", err)
	}
	if s.generation != generation {
		t.Fatal("timed-out generation ownership was cleared")
	}
	if snapshot := s.RuntimeSnapshot(); snapshot.Phase != RuntimeFailed || snapshot.ErrorCode != "stop_timeout" {
		t.Fatalf("timeout snapshot = %+v", snapshot)
	}

	err = s.Start(generationCtx)
	if !errors.Is(err, ErrRuntimeGenerationActive) {
		t.Fatalf("Start after stop timeout = %v, want ErrRuntimeGenerationActive", err)
	}
	if s.generation != generation {
		t.Fatal("second Start replaced the timed-out generation")
	}

	close(generation.done)
	if err := s.StopRuntime(); err != nil {
		t.Fatalf("StopRuntime after generation exit: %v", err)
	}
	if s.generation != nil {
		t.Fatal("completed generation was not reaped")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestHostLockSurvivesRuntimeFailure(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("LUMILIO_APP_DATA", appData)
	first := New(Options{Logf: func(string, ...any) {}})
	if err := first.Prepare(); err != nil {
		t.Fatalf("first Prepare: %v", err)
	}
	first.failRuntime(errors.New("manifest rejected"))

	second := New(Options{Logf: func(string, ...any) {}})
	if err := second.Prepare(); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Prepare = %v, want ErrAlreadyRunning", err)
	}
	if snapshot := first.RuntimeSnapshot(); snapshot.Phase != RuntimeFailed {
		t.Fatalf("first snapshot = %+v, want failed", snapshot)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := second.Prepare(); err != nil {
		t.Fatalf("second Prepare after host Close: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestConcurrentRestartReturnsOperationInProgress(t *testing.T) {
	s := New(Options{Logf: func(string, ...any) {}})
	s.operationMu.Lock()
	err := s.Restart(context.Background())
	s.operationMu.Unlock()
	if !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("Restart = %v, want ErrOperationInProgress", err)
	}
}

func TestStopRuntimeClearsRepositoryControl(t *testing.T) {
	s := New(Options{Logf: func(string, ...any) {}})
	s.repositoryManager = stubRepositoryControl{}
	done := make(chan struct{})
	close(done)
	s.generation = &runtimeGeneration{cancel: func() {}, done: done}

	if err := s.StopRuntime(); err != nil {
		t.Fatalf("StopRuntime: %v", err)
	}
	if _, err := s.RepositoryControl(); err == nil {
		t.Fatal("RepositoryControl remained available after generation stop")
	}
}

type stubRepositoryControl struct{}

func (stubRepositoryControl) ListStorageLocations(context.Context) ([]serverapp.StorageLocationInfo, error) {
	return nil, nil
}
func (stubRepositoryControl) AddStorageLocation(context.Context, string, string) (serverapp.StorageLocationInfo, []string, error) {
	return serverapp.StorageLocationInfo{}, nil, nil
}
func (stubRepositoryControl) ResolveStorageLocationConflict(context.Context, string, string) (serverapp.StorageLocationInfo, error) {
	return serverapp.StorageLocationInfo{}, nil
}
func (stubRepositoryControl) RemoveStorageLocation(context.Context, string) error { return nil }
func (stubRepositoryControl) AttachRepository(context.Context, string) (serverapp.RepositoryInfo, error) {
	return serverapp.RepositoryInfo{}, nil
}
func (stubRepositoryControl) ResolveRepositoryConflict(context.Context, string, string, string) (serverapp.RepositoryInfo, error) {
	return serverapp.RepositoryInfo{}, nil
}

func TestEnsureDirsArePrivate(t *testing.T) {
	t.Setenv("LUMILIO_APP_DATA", filepath.Join(t.TempDir(), "appdata"))
	paths, err := NewPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.AppData, paths.Logs, paths.Secrets, paths.Config, paths.Backups, paths.Cloud} {
		private, err := isPrivatePath(path)
		if err != nil || !private {
			t.Errorf("%s private = %v, err = %v", path, private, err)
		}
	}
	if paths.Database != filepath.Join(paths.AppData, "library.sqlite3") {
		t.Fatalf("database path = %q, want machine-local app-data catalog", paths.Database)
	}
	if _, err := os.Stat(paths.Database); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path initialization unexpectedly created the database: %v", err)
	}
}

func TestDesktopDatabaseParentFailureIsDiagnostic(t *testing.T) {
	root := filepath.Join(t.TempDir(), "appdata-file")
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LUMILIO_APP_DATA", root)
	paths, err := NewPaths()
	if err != nil {
		t.Fatal(err)
	}
	err = paths.EnsureDirs()
	if err == nil || !strings.Contains(err.Error(), "create "+root) {
		t.Fatalf("database parent failure = %v, want diagnostic path", err)
	}
}

func TestStorageReachable(t *testing.T) {
	dir := t.TempDir()
	if !storageReachable(dir) {
		t.Error("existing dir should be reachable")
	}
	// A not-yet-created child of an existing dir is reachable (creatable).
	if !storageReachable(filepath.Join(dir, "library")) {
		t.Error("creatable path (existing parent) should be reachable")
	}
	// A path under a non-existent parent (e.g. unmounted drive) is unreachable.
	if storageReachable(filepath.Join(dir, "missing", "library")) {
		t.Error("path under missing parent should be unreachable")
	}
}

func TestResolveStoragePathKeepsLocalDefaultAndQueuesLegacyExternal(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("LUMILIO_APP_DATA", appData)
	s := New(Options{Logf: func(string, ...any) {}})
	if err := s.ensurePaths(); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := SaveSettings(s.paths.DesktopSettingsFile(), DesktopSettings{StoragePath: external}); err != nil {
		t.Fatal(err)
	}

	got, err := s.resolveStoragePath()
	if err != nil {
		t.Fatal(err)
	}
	if got != s.paths.DefaultLib {
		t.Fatalf("resolved storage = %q, want local default %q", got, s.paths.DefaultLib)
	}
	if s.pendingStorageRoot != external {
		t.Fatalf("pending root = %q, want legacy external %q", s.pendingStorageRoot, external)
	}
	settings, err := LoadSettings(s.paths.DesktopSettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	if settings.StoragePath != external {
		t.Fatalf("legacy grant was overwritten before registration: %q", settings.StoragePath)
	}
}

func TestResolveStoragePathDoesNotRecreateUnavailableLegacyExternal(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("LUMILIO_APP_DATA", appData)
	s := New(Options{Logf: func(string, ...any) {}})
	if err := s.ensurePaths(); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "missing", "photos")
	if err := SaveSettings(s.paths.DesktopSettingsFile(), DesktopSettings{StoragePath: external}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.resolveStoragePath(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(external); !os.IsNotExist(err) {
		t.Fatalf("unavailable legacy path was recreated: %v", err)
	}
	if len(s.warnings) == 0 || !strings.Contains(s.warnings[len(s.warnings)-1], "remains offline") {
		t.Fatalf("missing explicit offline warning: %v", s.warnings)
	}
}
