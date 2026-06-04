package supervisor

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDesktopRuntimeE2E exercises the whole desktop pattern end to end: the
// supervisor brings up the private PostgreSQL, runs migrations, starts the API
// server in-process, and serves the SPA — then we hit http://localhost:6680 the
// way a browser would. It is opt-in (LUMILIO_E2E=1) and needs a local
// PostgreSQL with pgvector, so it is skipped in normal/CI runs.
func TestDesktopRuntimeE2E(t *testing.T) {
	if os.Getenv("LUMILIO_E2E") != "1" {
		t.Skip("set LUMILIO_E2E=1 (and have PostgreSQL+pgvector) to run the end-to-end test")
	}
	binDir := pgBinDirForTest(t)

	appData := t.TempDir()
	logDir := t.TempDir()

	// A stand-in web bundle so we can assert the SPA is served at the root.
	webRoot := t.TempDir()
	const marker = "E2E_SPA_OK"
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LUMILIO_APP_DATA", appData)
	t.Setenv("LUMILIO_PG_BIN_DIR", binDir)
	t.Setenv("LUMILIO_WEB_ROOT", webRoot)
	t.Setenv("LOG_DIR", logDir)
	// Leave ML/discovery at the desktop defaults (the generated config enables
	// mDNS, which the Lumen client requires as a discovery backend). No ML nodes
	// will be present, but discovery is best-effort and startup degrades cleanly.

	sup := New(Options{Logf: t.Logf})

	startCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := sup.Start(startCtx); err != nil {
		t.Fatalf("supervisor.Start: %v", err)
	}
	t.Cleanup(func() {
		if err := sup.Stop(); err != nil {
			t.Errorf("supervisor.Stop: %v", err)
		}
	})

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. API reachable at localhost:6680 (the browser/webview target).
	if body, code := httpGet(t, client, sup.ServerURL()+"/api/v1/health"); code != 200 || !strings.Contains(body, "ok") {
		t.Errorf("GET /api/v1/health = %d %q, want 200 containing ok", code, body)
	}

	// 2. The SPA is served at the root (the "open in browser" target).
	if body, code := httpGet(t, client, sup.ServerURL()+"/"); code != 200 || !strings.Contains(body, marker) {
		t.Errorf("GET / = %d %q, want 200 containing %q", code, body, marker)
	}

	// 3. A client-side route falls back to index.html, not 404.
	if body, code := httpGet(t, client, sup.ServerURL()+"/photos/abc"); code != 200 || !strings.Contains(body, marker) {
		t.Errorf("GET /photos/abc = %d %q, want SPA fallback (200, %q)", code, body, marker)
	}

	t.Logf("end-to-end OK: PostgreSQL + in-process API + SPA served at %s", sup.ServerURL())
}

func httpGet(t *testing.T, c *http.Client, url string) (string, int) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.StatusCode
}
