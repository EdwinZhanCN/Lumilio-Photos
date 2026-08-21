//go:build windows

package lumen

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// configureHiddenProcess prevents console-subsystem children from opening a
// window when they are launched by the Windows GUI Desktop binary. Preserve
// existing creation flags because the supervised process also owns a process
// group used by the lifecycle controller.
func configureHiddenProcess(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= createNoWindow
}

func configureProcessGroup(command *exec.Cmd) {
	configureHiddenProcess(command)
	command.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

func attachProcessGroup(command *exec.Cmd) (func() error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() error { return nil }, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return func() error { return nil }, err
	}
	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		_ = windows.CloseHandle(job)
		return func() error { return nil }, err
	}
	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		_ = windows.CloseHandle(processHandle)
		_ = windows.CloseHandle(job)
		return func() error { return nil }, err
	}
	_ = windows.CloseHandle(processHandle)
	return func() error { return windows.CloseHandle(job) }, nil
}

func terminateProcessTree(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Kill()
}
