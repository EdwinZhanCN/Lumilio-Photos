//go:build !darwin && !linux && !windows

package materializer

import "os"

func prepareControlledProfileRead(_ *os.File) (bool, error) { return false, nil }
