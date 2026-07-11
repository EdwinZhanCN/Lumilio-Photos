package backup

import (
	"testing"
	"time"
)

func TestFileNameRoundTrip(t *testing.T) {
	at := time.Date(2026, 7, 11, 2, 0, 0, 0, time.Local)

	cases := []struct {
		app, pg         string
		wantApp, wantPG string
	}{
		{"v1.2.3", "17.5", "v1.2.3", "17.5"},
		{"dev", "17.5 (Debian 17.5-1.pgdg13+1)", "dev", "17.5"},
		{"1.0.0-rc.1", "18.1", "1.0.0-rc.1", "18.1"},
		{"weird ver//sion", "17.5", "weird_ver_sion", "17.5"},
		{"", "not-a-version", "unknown", "0"},
	}

	for _, c := range cases {
		name := FileName(at, c.app, c.pg)
		info, ok := ParseName(name)
		if !ok {
			t.Errorf("FileName(%q, %q) = %q: not parseable", c.app, c.pg, name)
			continue
		}
		if !info.CreatedAt.Equal(at) {
			t.Errorf("%q: CreatedAt = %v, want %v", name, info.CreatedAt, at)
		}
		if info.AppVersion != c.wantApp || info.PGVersion != c.wantPG {
			t.Errorf("%q: parsed (%q, pg%q), want (%q, pg%q)", name, info.AppVersion, info.PGVersion, c.wantApp, c.wantPG)
		}
	}
}

func TestParseNameRejectsForeignFiles(t *testing.T) {
	for _, name := range []string{
		"immich-db-backup-20260711T020000-v1-pg17.sql.gz",
		"lumilio-db-backup-20260711T020000-v1-pg17.5.sql.gz" + TmpSuffix,
		RestorePointPrefix + "lumilio-db-backup-20260711T020000-v1-pg17.5.sql.gz",
		"lumilio-db-backup-2026x711T020000-v1-pg17.5.sql.gz",
		"notes.txt",
	} {
		if IsRoutineName(name) {
			t.Errorf("IsRoutineName(%q) = true, want false", name)
		}
	}
}

func TestFileNamesSortChronologically(t *testing.T) {
	older := FileName(time.Date(2026, 7, 10, 23, 59, 59, 0, time.Local), "v1", "17.5")
	newer := FileName(time.Date(2026, 7, 11, 0, 0, 0, 0, time.Local), "v1", "17.5")
	if !(older < newer) {
		t.Errorf("lexicographic order must match chronology: %q !< %q", older, newer)
	}
}
