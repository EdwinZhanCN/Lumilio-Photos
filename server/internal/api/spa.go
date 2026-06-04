package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterSPA serves a single-page-app bundle from webRoot for any request the
// API router does not handle, with an index.html fallback so client-side routes
// resolve. When webRoot is empty it is a no-op, preserving the API-only behavior
// docker/web rely on (a separate static server hosts the bundle there). The
// desktop build points webRoot at the bundled web build so http://localhost:6680
// serves the full app in the system browser and the in-app webview alike.
//
// Requests under /api and /swagger never fall back to the SPA, so their normal
// 404 semantics are preserved.
func RegisterSPA(r *gin.Engine, webRoot string) {
	webRoot = strings.TrimSpace(webRoot)
	if webRoot == "" {
		return
	}
	indexPath := filepath.Join(webRoot, "index.html")

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/swagger") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if file, ok := resolveStaticFile(webRoot, p); ok {
			c.File(file)
			return
		}
		// SPA fallback: let the client-side router handle the path.
		c.File(indexPath)
	})
}

// resolveStaticFile maps a request path to an existing regular file inside
// webRoot, guarding against path traversal. It returns ("", false) when the
// path does not resolve to a file under webRoot.
func resolveStaticFile(webRoot, reqPath string) (string, bool) {
	// Clean as an absolute path so any ".." is collapsed before joining, which
	// prevents escaping webRoot via crafted paths.
	clean := filepath.Clean("/" + strings.TrimPrefix(reqPath, "/"))
	full := filepath.Join(webRoot, clean)

	rel, err := filepath.Rel(webRoot, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", false
	}
	return full, true
}
