//go:build !windows

package platform

import "os"

func applyPrivateDirectoryMode(path string) error {
	return os.Chmod(path, 0o700)
}

func directoryIsPrivate(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.Mode().Perm()&0o077 == 0, nil
}
