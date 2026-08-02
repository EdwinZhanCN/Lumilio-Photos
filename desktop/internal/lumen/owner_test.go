package lumen

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestOwnerLockIsExclusiveAndTokened(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lumen", "owner.lock")
	first, err := AcquireOwnerLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Token() == "" {
		t.Fatal("owner lock did not produce a launch token")
	}
	second, err := AcquireOwnerLock(path)
	if !errors.Is(err, ErrOwnerBusy) {
		t.Fatalf("second owner lock error = %v, want ErrOwnerBusy", err)
	}
	if second != nil {
		t.Fatal("second owner unexpectedly acquired the lock")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := AcquireOwnerLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if third.Token() == first.Token() {
		t.Fatal("owner launch token was reused")
	}
}
