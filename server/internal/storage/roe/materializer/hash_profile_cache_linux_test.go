//go:build linux

package materializer

import (
	"os"

	"golang.org/x/sys/unix"
)

func prepareControlledProfileRead(file *os.File) (bool, error) {
	return true, unix.Fadvise(int(file.Fd()), 0, 0, unix.FADV_DONTNEED)
}
