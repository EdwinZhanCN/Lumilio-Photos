//go:build windows

package supervisor

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// InstanceLock is the Windows single-instance guard: the lock file is opened
// with no sharing mode, so a second instance's open fails with a sharing
// violation. The OS releases the handle automatically if the process dies, so
// a crash never leaves a stale lock.
type InstanceLock struct {
	handle windows.Handle
	path   string
}

// AcquireLock opens path exclusively. It returns ErrAlreadyRunning when
// another instance already holds it open.
func AcquireLock(path string) (*InstanceLock, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("lock path %s: %w", path, err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, // no sharing: concurrent opens fail
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	return &InstanceLock{handle: handle, path: path}, nil
}

// Release closes the handle (dropping the exclusive open) and removes the
// lock file.
func (l *InstanceLock) Release() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	_ = os.Remove(l.path)
	return err
}
