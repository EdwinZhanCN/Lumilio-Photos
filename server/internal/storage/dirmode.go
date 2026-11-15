package storage

import (
	"os"

	"server/internal/foundation/fsprivacy"
)

func applyDirectoryMode(path string, mode os.FileMode) error {
	return fsprivacy.ApplyDirectoryMode(path, mode)
}
