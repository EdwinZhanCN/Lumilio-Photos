package main

import (
	"syscall"
	"unsafe"
)

// freeBytes reports the bytes available to the caller at path via
// GetDiskFreeSpaceExW. path must be an existing directory.
func freeBytes(path string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	var freeAvailable uint64
	r, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeAvailable)),
		0, 0,
	)
	if r == 0 {
		return 0, callErr
	}
	return freeAvailable, nil
}
