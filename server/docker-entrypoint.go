package main

// Command docker-entrypoint prepares Docker bind mounts and then drops
// privileges to the app user before starting the requested Server command.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	if err := ensureAppWritable("/data/storage", "media storage", 0); err != nil {
		fmt.Fprintf(os.Stderr, "lumilio: %s\n", err)
		os.Exit(1)
	}
	if err := ensureAppWritable("/data/app-state", "application state", 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "lumilio: %s\n", err)
		os.Exit(1)
	}

	args := append([]string{"app"}, os.Args[1:]...)
	gosuPath, err := exec.LookPath("gosu")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lumilio: failed to locate gosu: %s\n", err)
		os.Exit(1)
	}
	if err := syscall.Exec(gosuPath, append([]string{"gosu"}, args...), os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "lumilio: failed to execute command as app: %s\n", err)
		os.Exit(1)
	}
}

func ensureAppWritable(target, label string, mode os.FileMode) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("cannot create %s directory at %s: %w", label, target, err)
	}
	if !writableAsApp(target) {
		// The bind mount may be owned by the host user. A failed chown is
		// intentionally ignored so the second check reports the actionable
		// permissions error below.
		_ = exec.Command("chown", "app:app", target).Run()
	}
	if !writableAsApp(target) {
		return fmt.Errorf(
			"%s directory is not writable by container uid 10001: %s; fix the bind-mount owner/permissions; the active SQLite catalog must stay in app-state",
			label,
			target,
		)
	}
	if mode != 0 {
		if err := os.Chmod(target, mode); err != nil {
			return fmt.Errorf("cannot set required permissions %o on %s directory: %s", mode, label, target)
		}
	}
	return nil
}

func writableAsApp(target string) bool {
	command := exec.Command("gosu", "app", "test", "-w", target)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run() == nil
}
