package api

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStorageTargetsReachableWithoutAdminMiddleware(t *testing.T) {
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
		`storageRoutes.GET("/targets", appInitializedMiddleware, storageController.GetStorageTargets)`,
		`storageAdmin.Use(authController.RequireAdmin())`,
	} {
		if !strings.Contains(text, contract) {
			t.Fatalf("storage targets route contract is missing %q", contract)
		}
	}
	if strings.Contains(text, `storageRoutes.Use(authController.AuthMiddleware(), authController.RequireAdmin())`) {
		t.Fatal("storage targets must not sit behind RequireAdmin on the parent group")
	}
}
