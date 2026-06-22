//go:build windows

package supervisor

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// InstanceLock is an advisory single-instance guard. On Windows there is no
// flock; instead the lock file is opened with an exclusive (no-sharing) handle
// via CreateFile, so a second instance's CreateFile fails with a sharing
// violation. Holding it prevents a second app instance from starting a second
// PostgreSQL against the same data directory (which would corrupt it).
type InstanceLock struct {
	handle windows.Handle
	path   string
}

// AcquireLock opens path with no sharing flags. It returns ErrAlreadyRunning
// when another instance already holds the handle (ERROR_SHARING_VIOLATION).
func AcquireLock(path string) (*InstanceLock, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("lock path %s: %w", path, err)
	}
	// dwShareMode 0 => exclusive: any concurrent open (including by us) fails.
	// FILE_FLAG_DELETE_ON_CLOSE removes the lock file when the handle closes.
	h, err := windows.CreateFile(
		p,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_DELETE_ON_CLOSE,
		0,
	)
	if err != nil {
		if err == windows.ERROR_SHARING_VIOLATION {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	return &InstanceLock{handle: h, path: path}, nil
}

// Release closes the exclusive handle (which also deletes the file via
// FILE_FLAG_DELETE_ON_CLOSE).
func (l *InstanceLock) Release() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	// Best-effort cleanup in case the delete-on-close did not fire.
	_ = os.Remove(l.path)
	return err
}
