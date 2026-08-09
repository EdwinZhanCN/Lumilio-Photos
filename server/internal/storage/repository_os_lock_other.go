//go:build !darwin && !linux && !windows

package storage

import (
	"context"
	"fmt"
	"os"
)

func platformAcquireFileLock(context.Context, *os.File, bool) error {
	return fmt.Errorf("%w: platform has no supported implementation", ErrRepositoryLockUnsupported)
}

func platformReleaseFileLock(*os.File) error { return nil }
