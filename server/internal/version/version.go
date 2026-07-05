// Package version carries the product version stamped at build time:
//
//	go build -ldflags "-X server/internal/version.Version=v1.0.0"
//
// Release builds (Docker image, desktop bundle) stamp the git tag; local and
// test builds report "dev". Surfaced through GET /api/v1/health.
package version

// Version is the product version for this build.
var Version = "dev"
