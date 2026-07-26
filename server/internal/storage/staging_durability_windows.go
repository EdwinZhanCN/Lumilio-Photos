//go:build windows

package storage

// Windows has no directory fsync equivalent. The destination file itself is
// flushed before the source link or file is removed.
func syncStagingDirectory(string) error {
	return nil
}
