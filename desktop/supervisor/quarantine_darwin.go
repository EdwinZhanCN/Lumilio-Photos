//go:build darwin

package supervisor

import "os/exec"

// stripQuarantine removes the com.apple.quarantine extended attribute from the
// bundled resources tree so Gatekeeper does not block the app's own exec calls
// to initdb/pg_ctl/postgres on first launch. The main process is already
// user-trusted (the user opened the app), so it is allowed to do this; the
// per-binary quarantine on shipped files inside the bundle is what would
// otherwise trip "cannot be opened" on each pg binary.
//
// Errors are returned for logging but are non-fatal: if the resources were
// never quarantined (e.g. a local dev build, or after the user already approved
// the app) xattr exits non-zero and that is fine.
func stripQuarantine(resourcesDir string) error {
	return exec.Command("xattr", "-dr", "com.apple.quarantine", resourcesDir).Run()
}
