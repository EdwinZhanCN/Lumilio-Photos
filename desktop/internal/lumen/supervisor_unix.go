//go:build darwin || linux

package lumen

import (
	"io"
	"os"
	"os/exec"
	"syscall"
)

func supervisorParentGone() <-chan struct{} {
	done := make(chan struct{})
	descriptor := uintptr(3)
	file := os.NewFile(descriptor, "lumen-parent-liveness")
	go func() {
		if file != nil {
			_, _ = io.Copy(io.Discard, file)
			_ = file.Close()
		}
		close(done)
	}()
	return done
}

func supervisorSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }

func terminateSupervisedChild(command *exec.Cmd) {
	if command != nil && command.Process != nil {
		_ = command.Process.Signal(syscall.SIGTERM)
	}
}

func killSupervisedChild(command *exec.Cmd) {
	if command != nil && command.Process != nil {
		_ = command.Process.Kill()
	}
}
