//go:build !windows

package storage

import (
	"fmt"
	"os"
)

func syncRepositoryDirectory(directory *os.File) error {
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
