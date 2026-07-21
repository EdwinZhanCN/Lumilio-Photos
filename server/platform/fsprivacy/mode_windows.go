//go:build windows

// Package fsprivacy applies owner-only filesystem access policies using the
// native mechanism of the host operating system.
package fsprivacy

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func ApplyDirectoryMode(path string, mode os.FileMode) error {
	if mode&0o077 != 0 {
		return nil
	}
	return applyPrivateDACL(path, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
}

func ApplyFileMode(path string, mode os.FileMode) error {
	if mode&0o077 != 0 {
		return nil
	}
	return applyPrivateDACL(path, 0)
}

func applyPrivateDACL(path string, inheritance uint32) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("resolve LocalSystem SID: %w", err)
	}
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("resolve Administrators SID: %w", err)
	}

	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		fullControlFor(user.User.Sid, inheritance),
		fullControlFor(system, inheritance),
		fullControlFor(administrators, inheritance),
	}, nil)
	if err != nil {
		return fmt.Errorf("build DACL: %w", err)
	}

	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil,
	); err != nil {
		return fmt.Errorf("apply DACL to %s: %w", path, err)
	}
	return nil
}

func fullControlFor(sid *windows.SID, inheritance uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

// IsPrivate reports whether path has a protected DACL. ApplyFileMode and
// ApplyDirectoryMode construct that DACL exclusively from the current user,
// LocalSystem, and Administrators.
func IsPrivate(path string) (bool, error) {
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return false, err
	}
	control, _, err := sd.Control()
	if err != nil {
		return false, err
	}
	return control&windows.SE_DACL_PROTECTED != 0, nil
}
