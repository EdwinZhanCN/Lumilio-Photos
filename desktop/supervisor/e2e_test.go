package supervisor

import (
	"context"
	"database/sql"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pelletier/go-toml/v2"
)

// TestDesktopRuntimeFirstAndSecondLaunch exercises the complete desktop
// boundary twice against the same machine-local SQLite catalog. The first
// launch creates and migrates the catalog; the second proves it reopens with
// the same durable library identity while the API and SPA remain reachable.
func TestDesktopRuntimeFirstAndSecondLaunch(t *testing.T) {
	appData := t.TempDir()
	webRoot := t.TempDir()
	resources := t.TempDir()
	const marker = "E2E_SPA_OK"
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffmpeg")))
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffprobe")))
	writeStubTool(t, filepath.Join(resources, "exiftool", toolExe("exiftool")))

	t.Setenv("LUMILIO_APP_DATA", appData)
	t.Setenv("LUMILIO_WEB_ROOT", webRoot)
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)

	client := &http.Client{Timeout: 5 * time.Second}
	first := startDesktopRuntime(t)
	assertDesktopHTTP(t, client, first.ServerURL(), marker)
	if err := first.Stop(); err != nil {
		t.Fatalf("first supervisor.Stop: %v", err)
	}

	databasePath := filepath.Join(appData, "library.sqlite3")
	firstLibraryID := readLibraryID(t, databasePath)
	assertPrivateCatalog(t, databasePath)

	second := startDesktopRuntime(t)
	assertDesktopHTTP(t, client, second.ServerURL(), marker)
	if err := second.Stop(); err != nil {
		t.Fatalf("second supervisor.Stop: %v", err)
	}
	secondLibraryID := readLibraryID(t, databasePath)
	if secondLibraryID != firstLibraryID {
		t.Fatalf("library identity changed across launches: first=%q second=%q", firstLibraryID, secondLibraryID)
	}

	t.Logf("desktop SQLite first/second launch OK: library_id=%s", firstLibraryID)
}

func TestDesktopRuntimeConfigRollback(t *testing.T) {
	appData := t.TempDir()
	webRoot := t.TempDir()
	resources := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("ROLLBACK_OK"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffmpeg")))
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffprobe")))
	writeStubTool(t, filepath.Join(resources, "exiftool", toolExe("exiftool")))
	t.Setenv("LUMILIO_APP_DATA", appData)
	t.Setenv("LUMILIO_WEB_ROOT", webRoot)
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)

	supervisor := startDesktopRuntime(t)
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	view, err := supervisor.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := supervisor.PatchRuntimeNetwork(view.BaseFingerprint, view.CandidateTOML, NetworkCandidatePatch{
		Mode: NetworkExternalHTTPS, PrimaryOrigin: "https://photos.example.com",
		Listen: blocker.Addr().String(), ProxyLocation: "remote",
		TrustedProxyCIDRs: []string{"127.0.0.1/32", "::1/128"},
	})
	if err != nil || !candidate.Valid {
		t.Fatalf("patch candidate = %+v, %v", candidate, err)
	}
	if _, err := supervisor.ApplyRuntimeConfigAsync(
		ctx,
		view.BaseFingerprint,
		candidate.CandidateTOML,
		false,
	); err != nil {
		t.Fatalf("apply candidate: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		snapshot := supervisor.RuntimeSnapshot()
		if snapshot.Phase == RuntimeRunning && snapshot.ErrorCode == "candidate_rolled_back" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("candidate rollback did not settle: %+v", snapshot)
		}
		time.Sleep(50 * time.Millisecond)
	}
	view, err = supervisor.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if view.Network.Mode != NetworkLocal || supervisor.ServerURL() != "http://localhost:6680" {
		t.Fatalf("rollback runtime network = %+v, URL = %s", view.Network, supervisor.ServerURL())
	}
	assertDesktopHTTP(t, &http.Client{Timeout: 5 * time.Second}, supervisor.ServerURL(), "ROLLBACK_OK")
}

func TestDesktopRuntimeConfigApplySuccess(t *testing.T) {
	appData := t.TempDir()
	webRoot := t.TempDir()
	resources := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("APPLY_OK"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffmpeg")))
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffprobe")))
	writeStubTool(t, filepath.Join(resources, "exiftool", toolExe("exiftool")))
	t.Setenv("LUMILIO_APP_DATA", appData)
	t.Setenv("LUMILIO_WEB_ROOT", webRoot)
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)

	supervisor := startDesktopRuntime(t)
	view, err := supervisor.ReadRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	document, _ := parseRuntimeDocument([]byte(view.CandidateTOML))
	setRuntimePath(document, "logging.level", "debug")
	candidate, _ := toml.Marshal(document)
	if _, err := supervisor.ApplyRuntimeConfigAsync(
		context.Background(),
		view.BaseFingerprint,
		string(candidate),
		false,
	); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		snapshot := supervisor.RuntimeSnapshot()
		active, readErr := os.ReadFile(filepath.Join(appData, "config", "runtime.toml"))
		if readErr == nil &&
			runtimeFingerprint(active) != view.BaseFingerprint &&
			snapshot.Phase == RuntimeRunning &&
			!snapshot.OperationActive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime apply did not settle: %+v, read=%v", snapshot, readErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
	active, err := os.ReadFile(filepath.Join(appData, "config", "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	lkg, err := os.ReadFile(filepath.Join(appData, "config", "runtime.last-known-good.toml"))
	if err != nil {
		t.Fatal(err)
	}
	activeDocument, err := parseRuntimeDocument(active)
	if err != nil {
		t.Fatal(err)
	}
	logLevel, _ := runtimePathValue(activeDocument, "logging.level")
	if string(active) != string(lkg) || logLevel != "debug" {
		t.Fatalf(
			"active/LKG mismatch after apply: active=%s lkg=%s logging.level=%v",
			runtimeFingerprint(active),
			runtimeFingerprint(lkg),
			logLevel,
		)
	}
	for _, name := range []string{"runtime.candidate.toml", "runtime-apply.json"} {
		if _, err := os.Stat(filepath.Join(appData, "config", name)); !os.IsNotExist(err) {
			t.Fatalf("successful apply left %s: %v", name, err)
		}
	}
}

func TestDesktopInvalidRuntimeCanRestoreLastKnownGood(t *testing.T) {
	appData := t.TempDir()
	webRoot := t.TempDir()
	resources := t.TempDir()
	const marker = "RESTORE_INVALID_OK"
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffmpeg")))
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffprobe")))
	writeStubTool(t, filepath.Join(resources, "exiftool", toolExe("exiftool")))
	t.Setenv("LUMILIO_APP_DATA", appData)
	t.Setenv("LUMILIO_WEB_ROOT", webRoot)
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)

	supervisor := startDesktopRuntime(t)
	if err := supervisor.StopRuntime(); err != nil {
		t.Fatal(err)
	}
	invalid := []byte("[server\ninvalid")
	if err := writeAtomicPrivate(filepath.Join(appData, "config", "runtime.toml"), invalid); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Restart(context.Background()); err == nil {
		t.Fatal("syntactically invalid runtime intent unexpectedly restarted")
	}
	if snapshot := supervisor.RuntimeSnapshot(); snapshot.Phase != RuntimeFailed {
		t.Fatalf("invalid runtime snapshot = %+v", snapshot)
	}
	if _, err := supervisor.RestoreLastKnownGoodAsync(context.Background()); err != nil {
		t.Fatalf("restore LKG from invalid active intent: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		snapshot := supervisor.RuntimeSnapshot()
		if snapshot.Phase == RuntimeRunning && !snapshot.OperationActive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("LKG restore did not settle: %+v", snapshot)
		}
		time.Sleep(50 * time.Millisecond)
	}
	active, err := os.ReadFile(filepath.Join(appData, "config", "runtime.toml"))
	if err != nil {
		t.Fatal(err)
	}
	lkg, err := os.ReadFile(filepath.Join(appData, "config", "runtime.last-known-good.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != string(lkg) {
		t.Fatal("restored active intent does not match readiness-confirmed LKG")
	}
	assertDesktopHTTP(t, &http.Client{Timeout: 5 * time.Second}, supervisor.ServerURL(), marker)

	other := New(Options{Logf: t.Logf})
	if err := other.Prepare(); err != ErrAlreadyRunning {
		t.Fatalf("host lock after recovery = %v, want ErrAlreadyRunning", err)
	}
}

func TestDesktopRuntimeRestart(t *testing.T) {
	appData := t.TempDir()
	webRoot := t.TempDir()
	resources := t.TempDir()
	const marker = "RESTART_OK"
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffmpeg")))
	writeStubTool(t, filepath.Join(resources, "ffmpeg", toolExe("ffprobe")))
	writeStubTool(t, filepath.Join(resources, "exiftool", toolExe("exiftool")))
	t.Setenv("LUMILIO_APP_DATA", appData)
	t.Setenv("LUMILIO_WEB_ROOT", webRoot)
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)

	supervisor := startDesktopRuntime(t)
	if err := supervisor.Restart(context.Background()); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	snapshot := supervisor.RuntimeSnapshot()
	if snapshot.Phase != RuntimeRunning || !snapshot.CanOpen ||
		snapshot.BrowserURL != "http://localhost:6680" {
		t.Fatalf("restart snapshot = %+v", snapshot)
	}
	assertDesktopHTTP(t, &http.Client{Timeout: 5 * time.Second}, snapshot.BrowserURL, marker)

	other := New(Options{Logf: t.Logf})
	if err := other.Prepare(); err != ErrAlreadyRunning {
		t.Fatalf("host lock after restart = %v, want ErrAlreadyRunning", err)
	}
}

func writeStubTool(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func startDesktopRuntime(t *testing.T) *Supervisor {
	t.Helper()
	sup := New(Options{Logf: t.Logf})
	startCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	if err := sup.Start(startCtx); err != nil {
		t.Fatalf("supervisor.Start: %v", err)
	}
	t.Cleanup(func() {
		if err := sup.Stop(); err != nil {
			t.Errorf("supervisor.Stop cleanup: %v", err)
		}
	})
	return sup
}

func assertDesktopHTTP(t *testing.T, client *http.Client, serverURL, marker string) {
	t.Helper()
	if body, code := httpGet(t, client, serverURL+"/api/v1/health"); code != http.StatusOK || !strings.Contains(body, "ok") {
		t.Errorf("GET /api/v1/health = %d %q, want 200 containing ok", code, body)
	}
	if body, code := httpGet(t, client, serverURL+"/"); code != http.StatusOK || !strings.Contains(body, marker) {
		t.Errorf("GET / = %d %q, want 200 containing %q", code, body, marker)
	}
	if body, code := httpGet(t, client, serverURL+"/photos/abc"); code != http.StatusOK || !strings.Contains(body, marker) {
		t.Errorf("GET /photos/abc = %d %q, want SPA fallback (200, %q)", code, body, marker)
	}
}

func readLibraryID(t *testing.T, path string) string {
	t.Helper()
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open SQLite catalog: %v", err)
	}
	defer database.Close()
	var libraryID string
	if err := database.QueryRow("SELECT library_id FROM system_state WHERE id = 1").Scan(&libraryID); err != nil {
		t.Fatalf("read library identity: %v", err)
	}
	if len(libraryID) != 32 {
		t.Fatalf("library identity = %q, want 32 lowercase hex characters", libraryID)
	}
	return libraryID
}

func assertPrivateCatalog(t *testing.T, path string) {
	t.Helper()
	private, err := isPrivatePath(path)
	if err != nil || !private {
		t.Fatalf("SQLite catalog private = %v, err = %v", private, err)
	}
}

func httpGet(t *testing.T, client *http.Client, url string) (string, int) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode
}
