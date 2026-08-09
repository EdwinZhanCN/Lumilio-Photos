//go:build darwin || linux

package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func platformAcquireFileLock(ctx context.Context, file *os.File, exclusive bool) error {
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := unix.Flock(int(file.Fd()), mode|unix.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return fmt.Errorf("%w: acquire advisory file lock: %v", ErrRepositoryLockUnsupported, err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ErrRepositoryLockUnavailable, ctx.Err())
		case <-ticker.C:
		}
	}
}

func platformReleaseFileLock(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
