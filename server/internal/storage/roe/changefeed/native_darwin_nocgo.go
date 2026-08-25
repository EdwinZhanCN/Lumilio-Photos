//go:build darwin && !cgo

package changefeed

import (
	"fmt"
	"os"
	"syscall"
)

func newNative() Feed { return Periodic{} }

func platformRepositoryVolume(repositoryPath string) (string, string, error) {
	info, err := os.Stat(repositoryPath)
	if err != nil {
		return "", "unknown", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "unknown", fmt.Errorf("repository stat has no Darwin device identity")
	}
	return fmt.Sprintf("darwin-device:%d", stat.Dev), "local", nil
}
