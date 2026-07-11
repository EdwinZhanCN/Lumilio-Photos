package backup

import (
	"regexp"
	"strings"
	"time"
)

// Backup dumps carry their provenance in the filename only — the restore UI
// and the upgrade orchestrator must never need to open a dump to know what it
// is. Format:
//
//	lumilio-db-backup-20260711T020000-v1.2.3-pg17.5.sql.gz
//
// Restore points (Phase 2) reuse the same format with the RestorePointPrefix
// prepended; they are excluded from routine retention.
const (
	routinePrefix = "lumilio-db-backup-"
	// RestorePointPrefix marks pre-restore/pre-upgrade safety dumps. They are
	// not counted or pruned as routine backups.
	RestorePointPrefix = "restore-point-"
	suffix             = ".sql.gz"
	// TmpSuffix marks an in-progress dump; finished dumps are renamed
	// atomically, so a lingering .tmp is always a failed run.
	TmpSuffix  = ".tmp"
	timeLayout = "20060102T150405"
)

// nameRe parses the routine filename. The app-version group is greedy but the
// pg version admits only digits and dots, so the final "-pgX.Y" anchor is
// unambiguous even when the app version itself contains dashes.
var nameRe = regexp.MustCompile(`^lumilio-db-backup-(\d{8}T\d{6})-v(.+)-pg([\d.]+)\.sql\.gz$`)

// unsafeChars collapses anything that would break the filename grammar (or a
// filesystem) out of the embedded version strings.
var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Info is the provenance parsed back out of a backup filename.
type Info struct {
	CreatedAt  time.Time
	AppVersion string
	PGVersion  string
}

// FileName builds the routine backup filename for a dump taken at now.
func FileName(now time.Time, appVersion, pgVersion string) string {
	return routinePrefix + now.Format(timeLayout) +
		"-v" + sanitizeVersion(appVersion) +
		"-pg" + sanitizePGVersion(pgVersion) + suffix
}

// ParseName extracts provenance from a routine backup filename. It returns
// false for restore points, temp files, and anything it did not generate.
func ParseName(name string) (Info, bool) {
	m := nameRe.FindStringSubmatch(name)
	if m == nil {
		return Info{}, false
	}
	createdAt, err := time.ParseInLocation(timeLayout, m[1], time.Local)
	if err != nil {
		return Info{}, false
	}
	return Info{CreatedAt: createdAt, AppVersion: m[2], PGVersion: m[3]}, true
}

// IsRoutineName reports whether name is a completed routine backup (subject to
// retention). Restore points and .tmp leftovers are not routine backups.
func IsRoutineName(name string) bool {
	_, ok := ParseName(name)
	return ok
}

func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return unsafeChars.ReplaceAllString(v, "_")
}

// sanitizePGVersion reduces a server_version string ("17.5 (Debian ...)") to
// the leading numeric version so it satisfies the filename grammar.
func sanitizePGVersion(v string) string {
	v = strings.TrimSpace(v)
	if fields := strings.Fields(v); len(fields) > 0 {
		v = fields[0]
	}
	trimmed := strings.TrimRight(unsafeChars.ReplaceAllString(v, ""), ".")
	if m := regexp.MustCompile(`^[\d.]+`).FindString(trimmed); m != "" {
		return strings.TrimRight(m, ".")
	}
	return "0"
}
