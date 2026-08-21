//go:build windows

package lumen

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
)

func TestLumenChildCommandsHideWindowsConsole(t *testing.T) {
	intent := SetupIntent{Preset: "basic", Region: "other", CacheDir: `C:\models`}
	commands := map[string]*exec.Cmd{
		"config renderer": newConfigRendererCommand(context.Background(), "lumen-hub.exe", intent),
		"supervised Hub":  newSupervisedHubCommand("lumen-hub.exe", `C:\config.yaml`),
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			assertHiddenWindowsProcess(t, command)
		})
	}
}

func TestLumenSupervisorPreservesProcessGroupWhileHidingConsole(t *testing.T) {
	command := exec.Command("lumilio-photos.exe")
	configureProcessGroup(command)
	assertHiddenWindowsProcess(t, command)
	if command.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("Lumen supervisor command does not create a Windows process group")
	}
}

func assertHiddenWindowsProcess(t *testing.T, command *exec.Cmd) {
	t.Helper()
	if command.SysProcAttr == nil {
		t.Fatal("Lumen command has no Windows process attributes")
	}
	if !command.SysProcAttr.HideWindow {
		t.Fatal("Lumen command does not hide its Windows console")
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("Lumen command does not set CREATE_NO_WINDOW")
	}
}
