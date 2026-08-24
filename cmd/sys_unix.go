//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// configureDetachedProcess sets POSIX-specific flags to start a new session (daemonize).
func configureDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// isProcessRunning checks if a process with the given PID is still active on POSIX.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, sending signal 0 checks for process existence without killing it
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
