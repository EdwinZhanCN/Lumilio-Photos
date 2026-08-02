//go:build windows

package lumen

import "os/exec"

// Windows uses the Job Object lifetime boundary. The supervisor executable is
// required to bind its descendants to that job; no inheritable pipe is used
// because arbitrary handles cannot safely cross this boundary.
func attachParentLiveness(*exec.Cmd) (func() error, func() error, error) {
	return func() error { return nil }, func() error { return nil }, nil
}
