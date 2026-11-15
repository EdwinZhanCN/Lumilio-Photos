package sysproc

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the Win32 CREATE_NO_WINDOW process-creation flag. In the
// bundled desktop app (linked -H windowsgui, with no console of its own), every
// spawned console tool — exiftool, ffmpeg, ffprobe — would otherwise flash its
// own black console window, and media processing spawns many per import.
const createNoWindow = 0x08000000

// HideConsole suppresses the console window of a spawned console application on
// Windows, preserving any flags already set on the command. It is a no-op on
// other platforms (see hide_other.go).
func HideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
