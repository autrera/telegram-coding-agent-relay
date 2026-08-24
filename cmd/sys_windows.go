//go:build windows

package cmd

import (
	"os/exec"
	"syscall"
)

// configureDetachedProcess sets Windows-specific flags to detach the child process completely.
func configureDetachedProcess(cmd *exec.Cmd) {
	// CREATE_NEW_PROCESS_GROUP = 0x00000200
	// DETACHED_PROCESS         = 0x00000008
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
		HideWindow:    true,
	}
}

const (
	processQueryLimitedInformation = 0x1000
	processQueryInformation        = 0x0400
	stillActive                    = 259
)

// isProcessRunning checks if a process with the given PID is still active on Windows.
func isProcessRunning(pid int) bool {
	p, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		p, err = syscall.OpenProcess(processQueryInformation, false, uint32(pid))
		if err != nil {
			return false
		}
	}
	defer syscall.CloseHandle(p)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(p, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}
