//go:build windows

package storage

import (
	"testing"

	"golang.org/x/sys/windows"
)

// requireDirectoryIsPrivate asserts the platform's expression of "only the
// owner may use this directory". Windows has no POSIX mode — os.Stat reports
// 0777 for every directory — so the meaningful property is that the DACL is
// protected: inheritance from a permissive parent has been severed and the
// explicit entries applyDirectoryMode wrote are the whole policy.
func requireDirectoryIsPrivate(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("read security info for %s: %v", path, err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatalf("read security descriptor control for %s: %v", path, err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("%s has an inheritable DACL, want a protected one", path)
	}
}

// unwritableDir returns "" because chmod cannot deny directory creation on
// Windows: os.Chmod only toggles the read-only attribute, which does not stop a
// subdirectory from being created. Expressing this would need a deny-ACE, which
// is not what the code under test uses.
func unwritableDir(t *testing.T) string {
	t.Helper()
	return ""
}

// invalidStructurePath uses characters the Win32 namespace forbids, so the
// create fails with ERROR_INVALID_NAME. The Unix trick of nesting under a
// regular file has no equivalent here.
func invalidStructurePath() string {
	return `C:\lumilio-invalid<>:"|?*\path`
}
