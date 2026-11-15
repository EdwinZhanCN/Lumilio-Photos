//go:build !windows

package sysproc

import "os/exec"

// HideConsole is a no-op off Windows, where spawned tools have no console window
// to suppress.
func HideConsole(cmd *exec.Cmd) {}
