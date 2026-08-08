//go:build linux

package storage

import (
	"fmt"
	"os"
	"syscall"
)

func platformFileIdentity(_ *os.File, info os.FileInfo) (*string, *string, *int64) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, nil, nil
	}
	kind := "unix-dev-inode-v1"
	value := fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
	change := stat.Ctim.Sec*1_000_000_000 + stat.Ctim.Nsec
	return &kind, &value, &change
}
