//go:build !darwin && !linux && !windows

package changefeed

import "errors"

func newNative() Feed { return Periodic{} }

func platformRepositoryVolume(string) (string, string, error) {
	return "", "unsupported", errors.New("native volume identity is unsupported")
}
