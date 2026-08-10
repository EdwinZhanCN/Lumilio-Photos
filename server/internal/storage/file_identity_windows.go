//go:build windows

package storage

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func platformFileIdentity(opened *os.File, _ os.FileInfo) (*string, *string, *int64) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(opened.Fd()), &info); err != nil {
		return nil, nil, nil
	}
	kind := "windows-volume-file-index-v1"
	value := fmt.Sprintf("%d:%d:%d", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow)
	return &kind, &value, nil
}
