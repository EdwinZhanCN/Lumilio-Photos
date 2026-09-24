// Package stateversion checks the schema version of a persisted Desktop state
// file. Version 1 of every file is the v26.1.0-rc.1 compatibility baseline.
// A reader that changes its file's version handles each older supported
// version before calling Check, so later builds read older files forward;
// a file from a newer Desktop is rejected rather than misread.
package stateversion

import (
	"errors"
	"fmt"
)

// ErrNewer marks a file written by a newer Lumilio Photos Desktop.
var ErrNewer = errors.New("written by a newer Lumilio Photos Desktop")

// Check accepts only the current version and says why any other one fails.
func Check(kind string, got, current int) error {
	switch {
	case got == current:
		return nil
	case got > current:
		return fmt.Errorf("%s schema version %d is newer than this build supports (%d): %w", kind, got, current, ErrNewer)
	default:
		return fmt.Errorf("%s schema version %d is not supported (want %d)", kind, got, current)
	}
}
