//go:build !linux && !darwin && !windows

package storage

import "errors"

func inspectVolume(string) (uint64, uint64, string, error) {
	return 0, 0, "", errors.New("volume capacity inspection is unsupported on this platform")
}

func inspectPathPlatform(string) pathPlatformInfo { return pathPlatformInfo{} }

func platformPlaceholderUnavailable(string) (bool, error) { return false, nil }
