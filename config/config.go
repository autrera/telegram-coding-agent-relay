package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Snapshot represents an immutable point-in-time configuration.
type Snapshot struct {
	TelegramBotToken   string
	AllowedUserIDs     map[int64]bool
	WorkingDir         string
	ContinueCommand    string
	NewCommand         string
	CommandTimeout     time.Duration
	StreamEditInterval time.Duration
	ShellType          string
	ControlPort        int
	EnvFilePath        string
}

// Config manages the application configuration with thread-safe hot-reloading.
type Config struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

// ExpandPath expands leading '~' with the user's home directory and normalizes paths.
func ExpandPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				path = home
			} else if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	cleaned := filepath.Clean(path)
	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs
	}
	return cleaned
}

// ParseEnvFile parses simple KEY=VALUE pairs from a .env file.
func ParseEnvFile(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	envMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimPrefix(line, "export ")
			line = strings.TrimSpace(line)
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip surrounding quotes
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				if len(val) >= 2 {
					val = val[1 : len(val)-1]
				}
			}
			envMap[key] = val
		}
	}
	return envMap, scanner.Err()
}

// Load loads configuration from a .env file and environment variables.
func Load(envPath string) (*Config, error) {
	cfg := &Config{}
	if envPath == "" {
		envPath = ".env"
	}
	absEnvPath := ExpandPath(envPath)
	cfg.snapshot.EnvFilePath = absEnvPath

	if err := cfg.Reload(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Reload re-reads the .env file and updates the in-memory configuration safely.
func (c *Config) Reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	envMap := make(map[string]string)
	if _, err := os.Stat(c.snapshot.EnvFilePath); err == nil {
		parsed, err := ParseEnvFile(c.snapshot.EnvFilePath)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", c.snapshot.EnvFilePath, err)
		}
		envMap = parsed
	}

	getEnv := func(key, fallback string) string {
		if val, ok := envMap[key]; ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
		if val := os.Getenv(key); val != "" {
			return strings.TrimSpace(val)
		}
		return fallback
	}

	botToken := getEnv("TELEGRAM_BOT_TOKEN", "")
	if botToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required in %s or environment", c.snapshot.EnvFilePath)
	}

	rawUsers := getEnv("ALLOWED_USER_IDS", "")
	allowedUsers := make(map[int64]bool)
	if rawUsers != "" {
		for _, part := range strings.Split(rawUsers, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid user ID in ALLOWED_USER_IDS: %q", part)
			}
			allowedUsers[id] = true
		}
	}

	workingDir := getEnv("WORKING_DIR", ".")
	workingDir = ExpandPath(workingDir)
	if stat, err := os.Stat(workingDir); err != nil || !stat.IsDir() {
		return fmt.Errorf("WORKING_DIR does not exist or is not a directory: %q", workingDir)
	}

	continueCmd := getEnv("CONTINUE_COMMAND", "")
	if continueCmd == "" {
		return fmt.Errorf("CONTINUE_COMMAND is required in %s or environment", c.snapshot.EnvFilePath)
	}

	newCmd := getEnv("NEW_COMMAND", "")
	if newCmd == "" {
		return fmt.Errorf("NEW_COMMAND is required in %s or environment", c.snapshot.EnvFilePath)
	}

	timeoutSec, _ := strconv.Atoi(getEnv("COMMAND_TIMEOUT_SECONDS", "600"))
	if timeoutSec <= 0 {
		timeoutSec = 600
	}

	streamIntervalMs, _ := strconv.Atoi(getEnv("STREAM_EDIT_INTERVAL_MS", "1500"))
	if streamIntervalMs < 500 {
		streamIntervalMs = 500
	}

	shellType := strings.ToLower(getEnv("SHELL_TYPE", "auto"))

	controlPort, _ := strconv.Atoi(getEnv("CONTROL_PORT", "47999"))
	if controlPort <= 0 {
		controlPort = 47999
	}

	c.snapshot = Snapshot{
		TelegramBotToken:   botToken,
		AllowedUserIDs:     allowedUsers,
		WorkingDir:         workingDir,
		ContinueCommand:    continueCmd,
		NewCommand:         newCmd,
		CommandTimeout:     time.Duration(timeoutSec) * time.Second,
		StreamEditInterval: time.Duration(streamIntervalMs) * time.Millisecond,
		ShellType:          shellType,
		ControlPort:        controlPort,
		EnvFilePath:        c.snapshot.EnvFilePath,
	}

	return nil
}

// Get returns an immutable snapshot of the current configuration.
func (c *Config) Get() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return shallow copy (maps need a copy to prevent mutation)
	s := c.snapshot
	userMap := make(map[int64]bool, len(c.snapshot.AllowedUserIDs))
	for k, v := range c.snapshot.AllowedUserIDs {
		userMap[k] = v
	}
	s.AllowedUserIDs = userMap
	return s
}

// IsUserAllowed checks if a Telegram user ID is whitelisted.
func (c *Config) IsUserAllowed(userID int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.snapshot.AllowedUserIDs) == 0 {
		return false
	}
	return c.snapshot.AllowedUserIDs[userID]
}

// SetWorkingDir dynamically updates the active working directory.
func (c *Config) SetWorkingDir(dir string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	expanded := ExpandPath(dir)
	stat, err := os.Stat(expanded)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("%s is not a directory", expanded)
	}
	c.snapshot.WorkingDir = expanded
	return expanded, nil
}
