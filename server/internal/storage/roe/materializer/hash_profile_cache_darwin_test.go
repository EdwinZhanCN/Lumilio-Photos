//go:build darwin

package materializer

import (
	"os"

	"golang.org/x/sys/unix"
)

func prepareControlledProfileRead(file *os.File) (bool, error) {
	_, err := unix.FcntlInt(file.Fd(), unix.F_NOCACHE, 1)
	return true, err
}
