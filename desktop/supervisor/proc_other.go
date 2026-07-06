//go:build !windows

package supervisor

import "os/exec"

// hideConsole is a no-op off Windows, where spawned tools have no console
// window to suppress.
func hideConsole(cmd *exec.Cmd) {}
