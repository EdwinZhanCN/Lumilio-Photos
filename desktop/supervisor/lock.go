//go:build unix

package supervisor

import (
	"fmt"
	"os"
	"syscall"
)

// InstanceLock is an advisory single-instance guard backed by flock. Holding it
// prevents a second app instance from starting a second PostgreSQL against the
// same data directory (which would corrupt it).
type InstanceLock struct {
	file *os.File
}

// AcquireLock takes a non-blocking exclusive flock on path. It returns
// ErrAlreadyRunning when another instance already holds the lock.
func AcquireLock(path string) (*InstanceLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("flock %s: %w", path, err)
	}
	return &InstanceLock{file: f}, nil
}

// Release drops the lock and removes the lock file.
func (l *InstanceLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	path := l.file.Name()
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	l.file = nil
	_ = os.Remove(path)
	return err
}
