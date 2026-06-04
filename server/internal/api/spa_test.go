package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSPATestRouter(t *testing.T, webRoot string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// A representative API route so we can assert it is not shadowed by the SPA.
	r.GET("/api/v1/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	RegisterSPA(r, webRoot)
	return r
}

func writeSPABundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("INDEX"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("JS"), 0o644))
	return dir
}

func do(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestRegisterSPAServesBundleWithFallback(t *testing.T) {
	r := newSPATestRouter(t, writeSPABundle(t))

	// Root serves index.html.
	if w := do(r, http.MethodGet, "/"); w.Code != http.StatusOK || w.Body.String() != "INDEX" {
		t.Errorf("GET / = %d %q, want 200 INDEX", w.Code, w.Body.String())
	}
	// A real static asset is served directly.
	if w := do(r, http.MethodGet, "/assets/app.js"); w.Code != http.StatusOK || w.Body.String() != "JS" {
		t.Errorf("GET /assets/app.js = %d %q, want 200 JS", w.Code, w.Body.String())
	}
	// An unknown (client-side) route falls back to index.html, not 404.
	if w := do(r, http.MethodGet, "/gallery/123"); w.Code != http.StatusOK || w.Body.String() != "INDEX" {
		t.Errorf("GET /gallery/123 = %d %q, want 200 INDEX (SPA fallback)", w.Code, w.Body.String())
	}
}

func TestRegisterSPADoesNotShadowAPIOrSwagger(t *testing.T) {
	r := newSPATestRouter(t, writeSPABundle(t))

	// Matched API route still works.
	if w := do(r, http.MethodGet, "/api/v1/health"); w.Code != http.StatusOK {
		t.Errorf("GET /api/v1/health = %d, want 200", w.Code)
	}
	// Unknown API path 404s as JSON instead of returning the SPA shell.
	w := do(r, http.MethodGet, "/api/v1/does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/does-not-exist = %d, want 404", w.Code)
	}
	if body := w.Body.String(); body == "INDEX" {
		t.Error("API miss returned the SPA shell; it must 404 instead")
	}
	// Swagger misses also must not return the SPA shell.
	if w := do(r, http.MethodGet, "/swagger/missing"); w.Body.String() == "INDEX" {
		t.Error("swagger miss returned the SPA shell; it must 404 instead")
	}
}

func TestRegisterSPANoopWhenWebRootEmpty(t *testing.T) {
	r := newSPATestRouter(t, "")
	// No NoRoute handler registered → gin's default 404 for an unknown route.
	if w := do(r, http.MethodGet, "/"); w.Code != http.StatusNotFound {
		t.Errorf("GET / with empty webRoot = %d, want 404 (API-only)", w.Code)
	}
}

func TestResolveStaticFileGuardsTraversal(t *testing.T) {
	dir := writeSPABundle(t)

	if got, ok := resolveStaticFile(dir, "/assets/app.js"); !ok || got != filepath.Join(dir, "assets", "app.js") {
		t.Errorf("resolveStaticFile(/assets/app.js) = %q,%v, want the asset path,true", got, ok)
	}
	if _, ok := resolveStaticFile(dir, "/nope.js"); ok {
		t.Error("resolveStaticFile(/nope.js) should be false")
	}
	// Path traversal must not escape webRoot (maps inside dir where nothing exists).
	if _, ok := resolveStaticFile(dir, "/../../../../etc/passwd"); ok {
		t.Error("resolveStaticFile traversal must not resolve outside webRoot")
	}
}
