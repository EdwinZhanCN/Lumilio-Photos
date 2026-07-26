package backup

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	routineSuffix = "-library.sqlite3"
	// RestorePointPrefix marks the snapshot taken after the runtime has drained
	// but before a staged restore replaces the active catalog.
	RestorePointPrefix = "restore-point-"
	// TmpSuffix marks an artifact that has not passed validation and atomic
	// finalization yet.
	TmpSuffix  = ".tmp"
	timeLayout = "20060102T150405.000000Z"
)

var nameRe = regexp.MustCompile(`^(\d{8}T\d{6}\.\d{6}Z)-library\.sqlite3$`)

// Info is the provenance encoded directly in a snapshot filename.
type Info struct {
	CreatedAt time.Time
}

// FileName returns the stable filename for a completed SQLite snapshot.
func FileName(now time.Time) string {
	return now.UTC().Format(timeLayout) + routineSuffix
}

// ManifestName returns the sidecar name for snapshotName.
func ManifestName(snapshotName string) string {
	return strings.TrimSuffix(snapshotName, ".sqlite3") + "-manifest.json"
}

// ManifestPath returns the sidecar path for a snapshot path.
func ManifestPath(snapshotPath string) string {
	return filepath.Join(filepath.Dir(snapshotPath), ManifestName(filepath.Base(snapshotPath)))
}

// ParseName extracts the creation time from a routine snapshot filename.
func ParseName(name string) (Info, bool) {
	match := nameRe.FindStringSubmatch(name)
	if match == nil {
		return Info{}, false
	}
	createdAt, err := time.Parse(timeLayout, match[1])
	if err != nil {
		return Info{}, false
	}
	return Info{CreatedAt: createdAt}, true
}

// IsRoutineName reports whether name is a completed routine snapshot.
func IsRoutineName(name string) bool {
	_, ok := ParseName(name)
	return ok
}

func trimRestorePoint(name string) (string, bool) {
	return strings.CutPrefix(name, RestorePointPrefix)
}
