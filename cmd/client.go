package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"relay/server"
)

var httpClient = &http.Client{
	Timeout: 3 * time.Second,
}

// CheckHealth queries the control server status endpoint.
func CheckHealth(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/status", port)
	resp, err := httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// StatusCmd fetches and displays the running daemon's status.
func StatusCmd() error {
	pid, errPid := ReadPID()
	port, errPort := ReadPort()

	if errPid != nil || errPort != nil || !isProcessRunning(pid) {
		fmt.Println("🔴 Relay daemon is not running.")
		return nil
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/status", port)
	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("⚠️  Relay PID file exists (%d) but control server at port %d is unreachable.\n", pid, port)
		return nil
	}
	defer resp.Body.Close()

	var status server.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to parse daemon response: %w", err)
	}

	fmt.Printf("🟢 Relay is ACTIVE\n")
	fmt.Printf("   • PID:               %d\n", status.PID)
	fmt.Printf("   • Port:              %d\n", port)
	fmt.Printf("   • Uptime:            %s\n", status.Uptime)
	fmt.Printf("   • Working Directory: %s\n", status.WorkingDir)
	fmt.Printf("   • Shell:             %s\n", status.ShellType)
	fmt.Printf("   • Allowed Users:     %v\n", status.AllowedUsers)
	fmt.Printf("   • Version:           %s\n", status.Version)

	return nil
}

// ReloadCmd triggers the daemon to re-read its .env file.
func ReloadCmd() error {
	port, err := ReadPort()
	if err != nil {
		return fmt.Errorf("relay daemon does not appear to be running (port file not found)")
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/reload", port)
	resp, err := httpClient.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to contact daemon on port %d: %w", port, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reload failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]string
	_ = json.Unmarshal(bodyBytes, &res)
	fmt.Printf("✅ %s\n", res["message"])
	return nil
}

// StopCmd signals the running daemon to shut down gracefully.
func StopCmd() error {
	pid, errPid := ReadPID()
	port, errPort := ReadPort()

	if errPid != nil || errPort != nil || !isProcessRunning(pid) {
		fmt.Println("ℹ️ Relay daemon is not running.")
		CleanupRuntimeFiles()
		return nil
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/stop", port)
	_, _ = httpClient.Post(url, "application/json", nil)

	fmt.Printf("Stopping relay daemon (PID: %d)...", pid)
	for i := 0; i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		if !isProcessRunning(pid) {
			CleanupRuntimeFiles()
			fmt.Println(" Stopped.")
			return nil
		}
		fmt.Print(".")
	}

	// Force kill if graceful stop timed out
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
	CleanupRuntimeFiles()
	fmt.Println(" Terminated.")
	return nil
}

// RestartCmd stops and restarts the background daemon.
func RestartCmd(envFile string) error {
	fmt.Println("Restarting relay daemon...")
	if err := StopCmd(); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return StartDaemon(envFile)
}
