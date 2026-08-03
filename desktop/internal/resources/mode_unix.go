//go:build !windows

package resources

import "io/fs"

func fileModeMatches(actual, expected fs.FileMode) bool {
	return actual.Perm() == expected.Perm()
}
