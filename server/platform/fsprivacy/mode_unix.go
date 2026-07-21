//go:build !windows

// Package fsprivacy applies owner-only filesystem access policies using the
// native mechanism of the host operating system.
package fsprivacy

import "os"

func ApplyDirectoryMode(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func ApplyFileMode(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

// IsPrivate reports whether group and other have no access to path.
func IsPrivate(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.Mode().Perm()&0o077 == 0, nil
}
