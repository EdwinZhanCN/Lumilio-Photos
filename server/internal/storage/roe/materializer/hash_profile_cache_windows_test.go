//go:build windows

package materializer

import "os"

func prepareControlledProfileRead(_ *os.File) (bool, error) {
	// Windows chooses unbuffered I/O at CreateFile time and then requires
	// sector-aligned reads. The production os.File path is buffered, so this
	// profile records the native ratio without pretending a cache-hot baseline
	// is a physical-volume limit.
	return false, nil
}
