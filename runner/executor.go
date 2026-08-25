package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ExecutionResult contains the outcome of a command run.
type ExecutionResult struct {
	Stdout   string
	Stderr   string
	Combined string
	ExitCode int
	Duration time.Duration
	Err      error
}

// OutputCallback is called whenever new chunks of output are read.
type OutputCallback func(chunk string, isStderr bool)

// Executor executes shell commands cross-platform with real-time streaming output.
type Executor struct{}

// NewExecutor creates a new command Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// BuildCommand constructs an exec.Cmd appropriate for the host OS.
func (e *Executor) BuildCommand(ctx context.Context, rawCmd, prompt, workingDir, shellType string) (*exec.Cmd, error) {
	// Format the command string by substituting {prompt}
	finalCmdStr := rawCmd
	if strings.Contains(rawCmd, "{prompt}") {
		// Escape double quotes inside the prompt for safe command string interpolation
		escapedPrompt := strings.ReplaceAll(prompt, "\"", "\\\"")
		finalCmdStr = strings.ReplaceAll(rawCmd, "{prompt}", escapedPrompt)
	}

	var cmd *exec.Cmd

	switch strings.ToLower(shellType) {
	case "powershell":
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", finalCmdStr)
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", finalCmdStr)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", finalCmdStr)
	case "sh":
		cmd = exec.CommandContext(ctx, "sh", "-c", finalCmdStr)
	case "auto", "":
		if runtime.GOOS == "windows" {
			// On Windows, powershell provides the best compatibility for modern CLI tools
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", finalCmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", finalCmdStr)
		}
	default:
		// Custom shell
		cmd = exec.CommandContext(ctx, shellType, "-c", finalCmdStr)
	}

	cmd.Dir = workingDir

	// Inherit system environment and inject RELAY_PROMPT
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RELAY_PROMPT=%s", prompt),
		"PYTHONIOENCODING=utf-8",
		"PYTHONUNBUFFERED=1",
	)

	return cmd, nil
}

// Execute runs the command in workingDir, invoking onOutput for streamed data.
func (e *Executor) Execute(ctx context.Context, cmdTemplate, prompt, workingDir, shellType string, onOutput OutputCallback) ExecutionResult {
	start := time.Now()

	cmd, err := e.BuildCommand(ctx, cmdTemplate, prompt, workingDir, shellType)
	if err != nil {
		log.Printf("[ERROR] Failed to start command in %s: %v", workingDir, err)
		return ExecutionResult{
			Err:      err,
			Duration: time.Since(start),
			ExitCode: 1,
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		startErr := fmt.Errorf("failed to open stdout pipe: %w", err)
		log.Printf("[ERROR] Failed to start command in %s: %v", workingDir, startErr)
		return ExecutionResult{Err: startErr, Duration: time.Since(start), ExitCode: 1}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		startErr := fmt.Errorf("failed to open stderr pipe: %w", err)
		log.Printf("[ERROR] Failed to start command in %s: %v", workingDir, startErr)
		return ExecutionResult{Err: startErr, Duration: time.Since(start), ExitCode: 1}
	}

	if err := cmd.Start(); err != nil {
		startErr := fmt.Errorf("failed to start process: %w", err)
		log.Printf("[ERROR] Failed to start command in %s: %v", workingDir, startErr)
		return ExecutionResult{
			Err:      startErr,
			Duration: time.Since(start),
			ExitCode: 1,
		}
	}

	var (
		stdoutBuf   strings.Builder
		stderrBuf   strings.Builder
		combinedBuf strings.Builder
		mu          sync.Mutex
		wg          sync.WaitGroup
	)

	// Stream stdout reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(stdoutPipe)
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				mu.Lock()
				stdoutBuf.WriteString(chunk)
				combinedBuf.WriteString(chunk)
				mu.Unlock()

				if onOutput != nil {
					onOutput(chunk, false)
				}
			}
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "closed") {
					// Read error
				}
				break
			}
		}
	}()

	// Stream stderr reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(stderrPipe)
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				mu.Lock()
				stderrBuf.WriteString(chunk)
				combinedBuf.WriteString(chunk)
				mu.Unlock()

				if onOutput != nil {
					onOutput(chunk, true)
				}
			}
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "closed") {
					// Read error
				}
				break
			}
		}
	}()

	// Wait for readers to finish after process completion
	wg.Wait()
	waitErr := cmd.Wait()

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return ExecutionResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Combined: combinedBuf.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
		Err:      waitErr,
	}
}
