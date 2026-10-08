//go:build !windows

package marker

import "os"

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func replaceFile(from, to string) error { return os.Rename(from, to) }
