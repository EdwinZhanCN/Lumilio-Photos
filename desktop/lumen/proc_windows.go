package lumen

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the Win32 CREATE_NO_WINDOW process-creation flag: the host
// app is linked -H windowsgui, so the spawned console hub would otherwise pop
// its own console window.
const createNoWindow = 0x08000000

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// configureShutdown is a no-op on Windows.
func configureShutdown(cmd *exec.Cmd) {}

// requestShutdown kills outright: there is no graceful console signal to
// deliver to a detached windowless process on Windows, and waiting out Stop's
// timeout first would only delay quit.
func requestShutdown(cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
}
