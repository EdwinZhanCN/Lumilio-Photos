package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	serverapp "server/app"

	"github.com/pelletier/go-toml/v2"
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
	if s.Version != desktopSettingsVersion {
		t.Errorf("missing settings version = %d, want %d", s.Version, desktopSettingsVersion)
	}

	want := DesktopSettings{Version: desktopSettingsVersion, StoragePath: "/Volumes/Photos/Lib"}
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

	updated := DesktopSettings{
		Version: desktopSettingsVersion, StoragePath: "/Volumes/Photos/Updated", Language: "zh",
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
	if local.Version != 1 || local.NetworkMode != NetworkLocal ||
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

func TestDesktopSettingsV2DoesNotPersistRuntimeNetworkFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-settings.json")
	if err := SaveSettings(path, DesktopSettings{
		NetworkMode:       NetworkExternalHTTPS,
		PrimaryOrigin:     "https://photos.example.com",
		Listen:            "0.0.0.0:6680",
		TrustedProxyCIDRs: []string{"192.168.1.10/32"},
		Language:          "zh",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"network_mode", "primary_origin", "trusted_proxy_cidrs"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("v2 settings persisted runtime field %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"version": 2`) || !strings.Contains(text, `"language": "zh"`) {
		t.Fatalf("v2 host settings missing expected fields:\n%s", text)
	}
}

func TestDesktopSettingsV1MigratesExternalNetworkToRuntimeIntent(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("LUMILIO_APP_DATA", appData)
	configDir := filepath.Join(appData, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "version": 1,
  "network_mode": "external_https",
  "primary_origin": "https://photos.example.com",
  "listen": "127.0.0.1:6680",
  "trusted_proxy_cidrs": ["127.0.0.1/32", "::1/128"],
  "language": "zh",
  "onboarding_completed": true
}`
	settingsPath := filepath.Join(configDir, "desktop-settings.json")
	if err := os.WriteFile(settingsPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	s := New(Options{Logf: func(string, ...any) {}})
	settings, err := s.Settings()
	if err != nil {
		t.Fatalf("migrate v1 settings: %v", err)
	}
	if settings.Version != desktopSettingsVersion ||
		settings.NetworkMode != NetworkExternalHTTPS ||
		settings.PrimaryOrigin != "https://photos.example.com" ||
		settings.Language != "zh" {
		t.Fatalf("migrated settings = %+v", settings)
	}
	disk, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(disk), "network_mode") || !strings.Contains(string(disk), `"version": 2`) {
		t.Fatalf("v2 settings did not remove runtime source fields:\n%s", disk)
	}
	view, err := s.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if view.Network.Mode != NetworkExternalHTTPS ||
		view.Network.PrimaryOrigin != "https://photos.example.com" ||
		!view.LastKnownGoodAvailable {
		t.Fatalf("migrated runtime view = %+v", view)
	}
}

func TestRuntimeCandidateFingerprintHostProjectionAndSemantics(t *testing.T) {
	t.Setenv("LUMILIO_APP_DATA", filepath.Join(t.TempDir(), "appdata"))
	s := New(Options{Logf: func(string, ...any) {}})
	view, err := s.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(view.BaseFingerprint, "sha256:") || len(view.HostManagedPaths) == 0 {
		t.Fatalf("runtime view missing provenance: %+v", view)
	}

	patched, err := s.PatchRuntimeNetwork(view.BaseFingerprint, view.CandidateTOML, NetworkCandidatePatch{
		Mode: NetworkExternalHTTPS, PrimaryOrigin: "https://photos.example.com",
		Listen: "127.0.0.1:6680", ProxyLocation: "same_host",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !patched.Valid || patched.Network.Mode != NetworkExternalHTTPS ||
		len(patched.SemanticChanges) == 0 {
		t.Fatalf("network candidate = %+v", patched)
	}
	if _, err := s.ValidateRuntimeConfig("sha256:stale", patched.CandidateTOML); !errors.Is(err, ErrStaleRuntimeConfig) {
		t.Fatalf("stale fingerprint error = %v", err)
	}

	document, err := parseRuntimeDocument([]byte(view.CandidateTOML))
	if err != nil {
		t.Fatal(err)
	}
	setRuntimePath(document, "database.path", "/tmp/not-desktop-owned.sqlite3")
	hostChanged, err := toml.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	validation, err := s.ValidateRuntimeConfig(view.BaseFingerprint, string(hostChanged))
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || len(validation.Issues) == 0 ||
		validation.Issues[0].Code != "host_managed" {
		t.Fatalf("host-owned change accepted: %+v", validation)
	}

	document, _ = parseRuntimeDocument([]byte(view.CandidateTOML))
	setRuntimePath(document, "server.tls.mode", "acme")
	acme, _ := toml.Marshal(document)
	validation, err = s.ValidateRuntimeConfig(view.BaseFingerprint, string(acme))
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || !hasConfigIssue(validation.Issues, "unsupported_desktop_tls") {
		t.Fatalf("Desktop ACME accepted: %+v", validation)
	}
}

func TestRuntimeApplyStopTimeoutDoesNotPromoteCandidate(t *testing.T) {
	t.Setenv("LUMILIO_APP_DATA", filepath.Join(t.TempDir(), "appdata"))
	s := New(Options{Logf: func(string, ...any) {}})
	if err := s.Prepare(); err != nil {
		t.Fatal(err)
	}
	view, err := s.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	document, _ := parseRuntimeDocument([]byte(view.CandidateTOML))
	setRuntimePath(document, "logging.level", "debug")
	candidate, _ := toml.Marshal(document)
	s.stopTimeout = 10 * time.Millisecond
	_, cancel := context.WithCancel(context.Background())
	generation := &runtimeGeneration{cancel: cancel, done: make(chan struct{})}
	s.generation = generation

	if _, err := s.ApplyRuntimeConfigAsync(
		context.Background(),
		view.BaseFingerprint,
		string(candidate),
		false,
	); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for s.RuntimeSnapshot().ErrorCode != "stop_timeout" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if snapshot := s.RuntimeSnapshot(); snapshot.ErrorCode != "stop_timeout" {
		t.Fatalf("stop timeout snapshot = %+v", snapshot)
	}
	active, err := os.ReadFile(s.paths.RuntimeConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != view.CurrentTOML {
		t.Fatal("stop timeout promoted the staged candidate")
	}
	for _, path := range []string{s.paths.RuntimeCandidateFile(), s.paths.RuntimeApplyJournalFile()} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stop timeout left staged artifact %s: %v", path, err)
		}
	}
	close(generation.done)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeApplyJournalReconciliation(t *testing.T) {
	for _, test := range []struct {
		name       string
		phase      runtimeApplyPhase
		wantActive string
	}{
		{name: "candidate staged keeps current", phase: applyCandidateStaged, wantActive: "current"},
		{name: "candidate promoted restores LKG", phase: applyCandidatePromoted, wantActive: "lkg"},
		{name: "rolling back restores LKG", phase: applyRollingBack, wantActive: "lkg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LUMILIO_APP_DATA", filepath.Join(t.TempDir(), "appdata"))
			s := New(Options{Logf: func(string, ...any) {}})
			if err := s.ensurePaths(); err != nil {
				t.Fatal(err)
			}
			if err := writeAtomicPrivate(s.paths.RuntimeConfigFile(), []byte("current")); err != nil {
				t.Fatal(err)
			}
			if err := writeAtomicPrivate(s.paths.RuntimeLastKnownGoodFile(), []byte("lkg")); err != nil {
				t.Fatal(err)
			}
			if err := writeAtomicPrivate(s.paths.RuntimeCandidateFile(), []byte("candidate")); err != nil {
				t.Fatal(err)
			}
			if err := s.writeApplyJournal(runtimeApplyJournal{Phase: test.phase}); err != nil {
				t.Fatal(err)
			}
			if err := s.reconcileRuntimeApply(); err != nil {
				t.Fatal(err)
			}
			active, err := os.ReadFile(s.paths.RuntimeConfigFile())
			if err != nil {
				t.Fatal(err)
			}
			if string(active) != test.wantActive {
				t.Fatalf("active = %q, want %q", active, test.wantActive)
			}
			for _, path := range []string{s.paths.RuntimeCandidateFile(), s.paths.RuntimeApplyJournalFile()} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("reconciliation left %s: %v", path, err)
				}
			}
		})
	}
}

func TestRuntimeApplyRollbackFailurePreservesJournal(t *testing.T) {
	t.Setenv("LUMILIO_APP_DATA", filepath.Join(t.TempDir(), "appdata"))
	s := New(Options{Logf: func(string, ...any) {}})
	if err := s.Prepare(); err != nil {
		t.Fatal(err)
	}
	view, err := s.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	document, _ := parseRuntimeDocument([]byte(view.CandidateTOML))
	setRuntimePath(document, "logging.level", "debug")
	candidate, _ := toml.Marshal(document)
	blocker, err := net.Listen("tcp", "127.0.0.1:6680")
	if err != nil {
		t.Skipf("fixed runtime port unavailable for rollback-failure test: %v", err)
	}
	defer blocker.Close()

	if _, err := s.ApplyRuntimeConfigAsync(
		context.Background(),
		view.BaseFingerprint,
		string(candidate),
		false,
	); err != nil {
		t.Fatal(err)
	}
	var journal runtimeApplyJournal
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(s.paths.RuntimeApplyJournalFile())
		if readErr == nil {
			if err := json.Unmarshal(data, &journal); err != nil {
				t.Fatal(err)
			}
			if journal.RollbackError != "" && !s.RuntimeSnapshot().OperationActive {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	if snapshot := s.RuntimeSnapshot(); snapshot.Phase != RuntimeFailed || snapshot.OperationActive {
		t.Fatalf("rollback failure snapshot = %+v", snapshot)
	}
	if journal.Phase != applyRollingBack || journal.CandidateError == "" || journal.RollbackError == "" {
		t.Fatalf("rollback failure journal = %+v", journal)
	}
	_ = s.cleanupRuntimeApply()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func hasConfigIssue(issues []ConfigIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
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
