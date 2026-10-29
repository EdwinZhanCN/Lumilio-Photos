package api

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStorageDiagnosticsSupportBundleAndAuditRemainAdminOnly(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "router.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, contract := range []string{
		`storageAdmin.Use(authController.RequireAdmin())`,
		`storageAdmin.GET("/diagnostics", appInitializedMiddleware, repositoryScanController.GetStorageDiagnostics)`,
		`storageAdmin.GET("/support-bundle", appInitializedMiddleware, repositoryScanController.DownloadStorageSupportBundle)`,
		`storageAdmin.GET("/audit", appInitializedMiddleware, repositoryScanController.ListLifecycleAudit)`,
	} {
		if !strings.Contains(text, contract) {
			t.Fatalf("administrator storage route contract is missing %q", contract)
		}
	}
}
