//go:build windows

package platform

import "golang.org/x/sys/windows"

func replaceFile(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		from,
		to,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

// Windows has no directory fsync equivalent. replaceFile uses
// MOVEFILE_WRITE_THROUGH so every atomic finalization is flushed before it
// returns.
func syncDirectory(string) error {
	return nil
}
