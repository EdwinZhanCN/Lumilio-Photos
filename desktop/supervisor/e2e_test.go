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

func TestDesktopNetworkRestartRollback(t *testing.T) {
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
	err = supervisor.ApplyNetworkSettings(ctx, DesktopSettings{
		NetworkMode:       NetworkExternalHTTPS,
		PrimaryOrigin:     "https://photos.example.com",
		Listen:            blocker.Addr().String(),
		TrustedProxyCIDRs: []string{"127.0.0.1/32", "::1/128"},
	})
	if err == nil || !strings.Contains(err.Error(), "restored last-known-good") {
		t.Fatalf("network change error = %v", err)
	}
	settings, err := supervisor.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.NetworkMode != NetworkLocal || supervisor.ServerURL() != "http://localhost:6680" {
		t.Fatalf("rollback settings = %+v, URL = %s", settings, supervisor.ServerURL())
	}
	assertDesktopHTTP(t, &http.Client{Timeout: 5 * time.Second}, supervisor.ServerURL(), "ROLLBACK_OK")
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
