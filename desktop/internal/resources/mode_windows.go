//go:build windows

package resources

import "io/fs"

// Windows does not expose POSIX execute or owner/group/other bits through
// os.FileMode. Executability is expressed by the .exe suffix, while the
// materialized resource tree inherits its access policy from its protected
// app-data directory.
func fileModeMatches(_, _ fs.FileMode) bool {
	return true
}
