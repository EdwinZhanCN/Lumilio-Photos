package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTomlStringEscaping(t *testing.T) {
	cases := map[string]string{
		"/plain/path":               `"/plain/path"`,
		"/has space/dir":            `"/has space/dir"`,
		`/has"quote`:                `"/has\"quote"`,
		`/has\backslash`:            `"/has\\backslash"`,
		"/Application Support/Lumi": `"/Application Support/Lumi"`,
	}
	for in, want := range cases {
		if got := tomlString(in); got != want {
			t.Errorf("tomlString(%q) = %q, want %q", in, got, want)
		}
	}
}

// WriteServerConfig must pin the WebAuthn relying party to localhost:6680 and
// emit the resolved storage/socket/tool paths. These are load-bearing: a wrong
// RP id silently breaks passkeys, and a wrong socket host breaks the DB.
func TestWriteServerConfigInvariants(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.local.toml")

	params := ServerConfigParams{
		Port:          "6680",
		WebRoot:       "/bundle/web",
		StoragePath:   "/Volumes/Photos/Lumilio Library",
		SocketDir:     "/Users/me/Library/Application Support/Lumilio Photos/postgres/16/run",
		PGPort:        "5487",
		DBUser:        "lumilio",
		DBName:        "lumiliophotos",
		PasswordFile:  "/secrets/db_password",
		SecretKeyPath: "/secrets/lumilio_secret_key",
		ExifToolPath:  "/bundle/exiftool",
		FFmpegPath:    "/bundle/ffmpeg",
		FFprobePath:   "/bundle/ffprobe",
	}
	if err := WriteServerConfig(path, params); err != nil {
		t.Fatalf("WriteServerConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)

	mustContain := []string{
		`webauthn_rp_id = "localhost"`,
		`webauthn_rp_origins = ["http://localhost:6680"]`,
		`port = "6680"`,
		`web_root = "/bundle/web"`,
		`path = "/Volumes/Photos/Lumilio Library"`,
		`host = "/Users/me/Library/Application Support/Lumilio Photos/postgres/16/run"`,
		`exiftool_path = "/bundle/exiftool"`,
		`ffmpeg_path = "/bundle/ffmpeg"`,
		`ffprobe_path = "/bundle/ffprobe"`,
		`name = "lumiliophotos"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("generated config missing %q\n---\n%s", want, got)
		}
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
	if err := SaveSettings(path, want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestEnsureSecretIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")

	if err := ensureSecret(path); err != nil {
		t.Fatalf("ensureSecret: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read secret: %v", err)
	}
	if len(first) != 64 { // 32 random bytes hex-encoded
		t.Errorf("secret length = %d, want 64 hex chars", len(first))
	}

	// A second call must not regenerate the secret.
	if err := ensureSecret(path); err != nil {
		t.Fatalf("ensureSecret (2nd): %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read secret (2nd): %v", err)
	}
	if string(first) != string(second) {
		t.Error("ensureSecret regenerated an existing secret; keys would change across launches")
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

func TestSocketDirFallbackOnLongPath(t *testing.T) {
	// A short app-data root keeps the socket under PGRun.
	short := filepath.Join(t.TempDir(), "ld")
	t.Setenv("LUMILIO_APP_DATA", short)
	p, err := NewPaths()
	if err != nil {
		t.Fatalf("NewPaths: %v", err)
	}
	if p.SocketDir() != p.PGRun {
		t.Errorf("short path: SocketDir() = %q, want PGRun %q", p.SocketDir(), p.PGRun)
	}

	// A very long app-data root forces the /tmp fallback to keep the socket path
	// within the platform limit.
	long := filepath.Join(t.TempDir(), strings.Repeat("verylongsegment/", 8))
	t.Setenv("LUMILIO_APP_DATA", long)
	p2, err := NewPaths()
	if err != nil {
		t.Fatalf("NewPaths: %v", err)
	}
	if p2.SocketDir() == p2.PGRun {
		t.Errorf("long path: SocketDir() should fall back to /tmp, got PGRun %q", p2.SocketDir())
	}
	if !strings.HasPrefix(p2.SocketDir(), os.TempDir()) {
		t.Errorf("long path: SocketDir() = %q, want a temp-dir fallback", p2.SocketDir())
	}
}
