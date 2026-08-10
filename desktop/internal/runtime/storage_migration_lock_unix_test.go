//go:build darwin || linux

package runtime_test

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func holdRootLock(rootPath string) (func(), error) {
	file, err := os.OpenFile(filepath.Join(rootPath, ".lumilioroot.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
	}, nil
}
