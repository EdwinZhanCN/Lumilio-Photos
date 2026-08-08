//go:build !darwin && !linux && !windows

package storage

import "os"

func platformFileIdentity(_ *os.File, _ os.FileInfo) (*string, *string, *int64) {
	return nil, nil, nil
}
