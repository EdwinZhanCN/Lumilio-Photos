//go:build windows

package lumen

import (
	"os"
	"os/exec"
)

func supervisorParentGone() <-chan struct{} { return nil }

func supervisorSignals() []os.Signal { return []os.Signal{os.Interrupt} }

func terminateSupervisedChild(command *exec.Cmd) { killSupervisedChild(command) }

func killSupervisedChild(command *exec.Cmd) {
	if command != nil && command.Process != nil {
		_ = command.Process.Kill()
	}
}
