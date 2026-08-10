package api

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLifecycleAuditDiagnosticsAndSupportBundleRemainAdminOnly(t *testing.T) {
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
		`repositories.Use(authController.AuthMiddleware(), authController.RequireAdmin())`,
		`repositories.GET("/lifecycle-audit", appInitializedMiddleware, repositoryScanController.ListLifecycleAudit)`,
		`repositories.GET("/storage-diagnostics", appInitializedMiddleware, repositoryScanController.GetStorageDiagnostics)`,
		`repositories.GET("/storage-support-bundle", appInitializedMiddleware, repositoryScanController.DownloadStorageSupportBundle)`,
	} {
		if !strings.Contains(text, contract) {
			t.Fatalf("administrator lifecycle route contract is missing %q", contract)
		}
	}
}
