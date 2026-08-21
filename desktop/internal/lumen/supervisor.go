package lumen

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

const supervisorMode = "--lumen-supervise-child"

func SupervisorArgs(hubBinary, configPath string) []string {
	return []string{supervisorMode, "--hub", hubBinary, "--config", configPath}
}

// RunSupervisorMode turns the Desktop executable into a tiny crash-containment
// wrapper around Lumen Hub. ExecFactory owns this wrapper; on Unix the inherited
// liveness pipe also covers abrupt parent death, while Windows' Job Object owns
// the complete descendant tree.
func RunSupervisorMode(args []string) (bool, error) {
	if len(args) == 0 || args[0] != supervisorMode {
		return false, nil
	}
	if len(args) != 5 || args[1] != "--hub" || args[3] != "--config" {
		return true, errors.New("invalid Lumen supervisor arguments")
	}
	hubBinary := strings.TrimSpace(args[2])
	configPath := strings.TrimSpace(args[4])
	if hubBinary == "" || configPath == "" {
		return true, errors.New("Lumen supervisor requires hub and config paths")
	}
	if os.Getenv("LUMILIO_PARENT_LIVENESS") != "required" || strings.TrimSpace(os.Getenv("LUMILIO_LAUNCH_TOKEN")) == "" {
		return true, errors.New("Lumen supervisor parent contract is missing")
	}

	command := newSupervisedHubCommand(hubBinary, configPath)
	command.Stdin = nil
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return true, err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, supervisorSignals()...)
	defer signal.Stop(signals)
	parentGone := supervisorParentGone()
	select {
	case err := <-done:
		if err != nil {
			return true, fmt.Errorf("Lumen Hub exited: %w", err)
		}
		return true, nil
	case <-parentGone:
	case <-signals:
	}

	terminateSupervisedChild(command)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return true, nil
	case <-timer.C:
		killSupervisedChild(command)
		<-done
		return true, nil
	}
}

func newSupervisedHubCommand(hubBinary, configPath string) *exec.Cmd {
	command := exec.Command(hubBinary, "--config", configPath)
	configureHiddenProcess(command)
	return command
}
