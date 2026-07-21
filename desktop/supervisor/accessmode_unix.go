//go:build !windows

package supervisor

import "os"

func applyPrivateDirectoryMode(path string) error { return os.Chmod(path, 0o700) }

func applyPrivateFileMode(path string) error { return os.Chmod(path, 0o600) }

func isPrivatePath(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.Mode().Perm()&0o077 == 0, nil
}
