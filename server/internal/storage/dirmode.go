package storage

import (
	"os"

	"server/platform/fsprivacy"
)

func applyDirectoryMode(path string, mode os.FileMode) error {
	return fsprivacy.ApplyDirectoryMode(path, mode)
}
