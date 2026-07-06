//go:build !windows

package main

import "syscall"

// freeBytes reports the bytes available to a non-privileged process at path
// (which must be an existing directory). Used by onboarding to show free space
// for the chosen library location.
func freeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}
