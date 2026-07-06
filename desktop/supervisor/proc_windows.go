package supervisor

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the Win32 CREATE_NO_WINDOW process-creation flag. The host
// app is linked -H windowsgui and has no console of its own, so every spawned
// console tool (initdb, pg_ctl, pg_isready, createdb) would otherwise pop its
// own black console window. The flag also propagates to grandchildren, so the
// postmaster that pg_ctl launches stays windowless too.
const createNoWindow = 0x08000000

// hideConsole suppresses the console window of a spawned console application.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
