//go:build !unix && !windows

package supervisor

import "fmt"

// InstanceLock is a no-op placeholder on platforms without flock or Windows
// file locking; this stub only exists so the package compiles when
// cross-compiled for other GOOS values.
type InstanceLock struct{}

// AcquireLock is unsupported off unix and always errors so the caller does not
// silently run without single-instance protection.
func AcquireLock(path string) (*InstanceLock, error) {
	return nil, fmt.Errorf("single-instance lock is not supported on this platform")
}

// Release is a no-op.
func (l *InstanceLock) Release() error { return nil }
