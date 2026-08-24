package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GetRuntimeDir returns the path to ~/.relay
func GetRuntimeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".relay")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// SavePID writes the current PID to ~/.relay/relay.pid
func SavePID(pid int) error {
	dir, err := GetRuntimeDir()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "relay.pid"), []byte(strconv.Itoa(pid)), 0644)
}

// SavePort writes the active control port to ~/.relay/relay.port
func SavePort(port int) error {
	dir, err := GetRuntimeDir()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "relay.port"), []byte(strconv.Itoa(port)), 0644)
}

// ReadPID reads the PID from ~/.relay/relay.pid
func ReadPID() (int, error) {
	dir, err := GetRuntimeDir()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "relay.pid"))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// ReadPort reads the control port from ~/.relay/relay.port
func ReadPort() (int, error) {
	dir, err := GetRuntimeDir()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "relay.port"))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// CleanupRuntimeFiles removes ~/.relay/relay.pid and relay.port
func CleanupRuntimeFiles() {
	if dir, err := GetRuntimeDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, "relay.pid"))
		_ = os.Remove(filepath.Join(dir, "relay.port"))
	}
}

// StartDaemon launches relay in the background as a detached daemon.
func StartDaemon(envFile string) error {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return fmt.Errorf("failed to get runtime directory: %w", err)
	}

	// Check if already running
	if pid, err := ReadPID(); err == nil && isProcessRunning(pid) {
		port, _ := ReadPort()
		if port > 0 && CheckHealth(port) {
			return fmt.Errorf("relay is already running (PID: %d, Port: %d)", pid, port)
		}
	}

	// Clear stale files
	CleanupRuntimeFiles()

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate executable: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(runtimeDir, "relay.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open daemon log file: %w", err)
	}

	args := []string{"run"}
	if envFile != "" {
		args = append(args, "--env", envFile)
	}

	childCmd := exec.Command(execPath, args...)
	childCmd.Stdout = logFile
	childCmd.Stderr = logFile
	configureDetachedProcess(childCmd)

	if err := childCmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("failed to spawn background daemon: %w", err)
	}
	_ = logFile.Close()

	// Wait up to 5 seconds for daemon to initialize and write its PID & Port
	fmt.Print("Starting relay daemon in background...")
	for i := 0; i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		port, errPort := ReadPort()
		pid, errPid := ReadPID()
		if errPort == nil && errPid == nil && port > 0 && pid > 0 {
			if CheckHealth(port) {
				fmt.Println(" Done!")
				fmt.Printf("🟢 Relay daemon is running:\n")
				fmt.Printf("   • PID:  %d\n", pid)
				fmt.Printf("   • Port: %d\n", port)
				fmt.Printf("   • Logs: %s\n", filepath.Join(runtimeDir, "relay.log"))
				return nil
			}
		}
		fmt.Print(".")
	}

	fmt.Println()
	return fmt.Errorf("daemon started (PID: %d) but failed health check. Check logs at: %s", childCmd.Process.Pid, filepath.Join(runtimeDir, "relay.log"))
}
