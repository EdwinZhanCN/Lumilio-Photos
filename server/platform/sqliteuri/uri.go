// Package sqliteuri builds cross-platform SQLite file URIs.
package sqliteuri

import (
	"net/url"
	"path/filepath"
	"strings"
)

// DSN returns a file URI for an absolute SQLite catalog path and optional
// driver query parameters. Windows drive-letter paths need a leading slash so
// SQLite parses the drive as part of the path rather than as a URI authority.
func DSN(path string, query url.Values) string {
	uriPath := filepath.ToSlash(filepath.Clean(path))
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}

	location := &url.URL{Scheme: "file", Path: uriPath}
	if len(query) != 0 {
		location.RawQuery = query.Encode()
	}
	return location.String()
}
