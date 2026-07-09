//go:build !windows

package lumen

import (
	"os/exec"
	"syscall"
)

// hideConsole is a no-op off Windows, where spawned tools have no console
// window to suppress.
func hideConsole(cmd *exec.Cmd) {}

// configureShutdown is a no-op on unix; SIGTERM needs no setup.
func configureShutdown(cmd *exec.Cmd) {}

// requestShutdown asks the hub to exit gracefully.
func requestShutdown(cmd *exec.Cmd) {
	_ = cmd.Process.Signal(syscall.SIGTERM)
}
