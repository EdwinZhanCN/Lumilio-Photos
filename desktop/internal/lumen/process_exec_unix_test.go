//go:build darwin || linux

package lumen

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestExecFactoryPublishesUnexpectedChildExitAndReleasesOwner(t *testing.T) {
	ownerPath := filepath.Join(t.TempDir(), "owner.lock")
	process, err := (ExecFactory{
		Binary: "/usr/bin/true", OwnerLock: ownerPath, Endpoint: "test",
		Probe: func(context.Context, string, string) error { return nil },
	}).Start(context.Background(), 1, "test")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case err := <-process.Done:
		if err != nil {
			t.Fatalf("child exit: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("child exit was not published")
	}
	select {
	case <-process.Lifetime.Done():
	default:
		t.Fatal("process lifetime remained active after exit")
	}
	owner, err := AcquireOwnerLock(ownerPath)
	if err != nil {
		t.Fatalf("owner lock was not released: %v", err)
	}
	_ = owner.Close()
}
