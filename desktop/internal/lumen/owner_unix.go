//go:build darwin || linux

package lumen

import (
	"errors"
	"os"
	"syscall"
)

func lockOwnerFile(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return ErrOwnerBusy
		}
		return err
	}
	return nil
}

func unlockOwnerFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
