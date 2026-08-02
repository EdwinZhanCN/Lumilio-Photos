//go:build darwin || linux

package lumen

import (
	"os"
	"os/exec"
)

func attachParentLiveness(command *exec.Cmd) (func() error, func() error, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return func() error { return nil }, func() error { return nil }, err
	}
	command.ExtraFiles = append(command.ExtraFiles, reader)
	command.Env = append(command.Env, "LUMILIO_PARENT_LIVENESS_FD=3")
	return reader.Close, writer.Close, nil
}
