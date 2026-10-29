package storage

import (
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file used as an ancestor yields nil FileInfo without relying on chmod
// (which root can bypass) or native mount hardware. Unix reports ENOTDIR;
// Windows can report positive absence instead. Both outcomes are asserted.
func TestValidateRepositoryStatFailureDoesNotPanic(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	_, statErr := os.Stat(filepath.Join(parent, "child"))
	if statErr == nil {
		t.Fatal("file ancestor unexpectedly accepted")
	}
	rm := &DefaultRepositoryManager{logger: zap.NewNop()}
	result, err := rm.validateRepository(filepath.Join(parent, "child"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || len(result.Errors) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !os.IsNotExist(statErr) && strings.Contains(strings.Join(result.Errors, " "), "does not exist") {
		t.Fatalf("stat failure reduced to absence: %+v", result)
	}
}
