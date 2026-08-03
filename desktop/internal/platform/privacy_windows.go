//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func applyPrivateDirectoryMode(path string) error {
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

	inheritance := uint32(windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		fullDirectoryControlFor(user.User.Sid, inheritance),
		fullDirectoryControlFor(system, inheritance),
		fullDirectoryControlFor(administrators, inheritance),
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
		return fmt.Errorf("apply DACL: %w", err)
	}
	return nil
}

func fullDirectoryControlFor(sid *windows.SID, inheritance uint32) windows.EXPLICIT_ACCESS {
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

func directoryIsPrivate(path string) (bool, error) {
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
