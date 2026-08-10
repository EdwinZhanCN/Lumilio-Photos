//go:build windows

package storage

import "os"

// Windows has no directory fsync equivalent. The destination file itself is
// flushed before the source link or file is removed.
func syncRepositoryDirectory(*os.File) error {
	return nil
}
