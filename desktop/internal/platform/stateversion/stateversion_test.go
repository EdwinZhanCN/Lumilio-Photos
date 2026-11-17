package stateversion

import (
	"errors"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	if err := Check("settings", 1, 1); err != nil {
		t.Fatalf("current version: %v", err)
	}
	if err := Check("settings", 2, 1); !errors.Is(err, ErrNewer) || !strings.Contains(err.Error(), "settings schema version 2") {
		t.Fatalf("newer version error = %v", err)
	}
	if err := Check("settings", 0, 1); err == nil || errors.Is(err, ErrNewer) || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("missing version error = %v", err)
	}
}
