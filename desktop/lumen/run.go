package lumen

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// Hub is a running supervised lumen-hub process.
type Hub struct {
	cmd     *exec.Cmd
	logFile *os.File
	done    chan error // closed after Wait returns; carries the exit error
}

// Start regenerates the config and spawns the installed hub, directing its
// output to logPath. It returns as soon as the process is running: readiness
// is a separate concern (WaitReady) because the first start downloads model
// weights, which can take many minutes.
func Start(ctx context.Context, dir, lang, logPath string) (*Hub, error) {
	selection, err := DefaultConfigSelection(dir, lang)
	if err != nil {
		return nil, err
	}
	return StartWithConfig(ctx, dir, selection, logPath)
}

// StartWithConfig validates and writes the selected launcher-compatible config
// before spawning the installed Hub.
func StartWithConfig(ctx context.Context, dir string, selection ConfigSelection, logPath string) (*Hub, error) {
	if _, ok := Installed(dir); !ok {
		return nil, errors.New("lumen hub is not installed")
	}
	if err := WriteConfigFor(dir, selection); err != nil {
		return nil, fmt.Errorf("write lumen config: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, HubBinary(dir), "--config", ConfigPath(dir))
	cmd.Dir = hubDir(dir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	hideConsole(cmd)
	configureShutdown(cmd)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start lumen hub: %w", err)
	}

	h := &Hub{cmd: cmd, logFile: logFile, done: make(chan error, 1)}
	go func() {
		h.done <- cmd.Wait()
		close(h.done)
		logFile.Close()
	}()
	return h, nil
}

// Done reports process exit: it yields the exit error once the hub stops for
// any reason.
func (h *Hub) Done() <-chan error { return h.done }

// WaitReady blocks until the hub accepts TCP connections on its gRPC endpoint,
// the process exits, or the timeout/context ends. First start includes model
// downloads, so callers should pass a generous timeout.
func (h *Hub) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		select {
		case err := <-h.done:
			if err != nil {
				return fmt.Errorf("lumen hub exited during startup: %w", err)
			}
			return errors.New("lumen hub exited during startup")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", GRPCEndpoint, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("lumen hub not ready after %s", timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Stop shuts the hub down: a graceful signal first (unix), then a hard kill
// after the timeout. Safe to call on an already-exited hub.
func (h *Hub) Stop(timeout time.Duration) {
	select {
	case <-h.done:
		return // already exited
	default:
	}

	requestShutdown(h.cmd)
	select {
	case <-h.done:
	case <-time.After(timeout):
		_ = h.cmd.Process.Kill()
		<-h.done
	}
}
