package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"relay/cmd"
	"relay/config"
	"relay/server"
	"relay/telegram"
)

const version = "1.0.0"

func printUsage() {
	fmt.Printf(`Telegram Coding Agent Relay v%s
A lightweight cross-platform bridge between Telegram and your local coding agents.

Usage:
  relay <command> [flags]

Available Commands:
  start     Launch the relay in background as a detached daemon
  run       Run the relay in the foreground (useful for development and debugging)
  status    Check the status, PID, uptime, and configuration of the running daemon
  reload    Reload the .env configuration file on the fly without stopping the bot
  stop      Gracefully stop the running background daemon
  restart   Restart the background daemon
  version   Display version information

Flags:
  -e, --env <path>   Path to .env configuration file (default: .env)

Examples:
  relay start                # Starts daemon in background
  relay run --env .env.local # Runs directly in terminal
  relay status               # Checks if running
  relay reload               # Reloads modified .env file
  relay stop                 # Stops background process
`, version)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	// Handle flags
	subCmdArgs := os.Args[2:]
	var envPath string

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	fs.StringVar(&envPath, "env", ".env", "Path to .env configuration file")
	fs.StringVar(&envPath, "e", ".env", "Path to .env configuration file (shorthand)")

	_ = fs.Parse(subCmdArgs)

	switch command {
	case "help", "-h", "--help":
		printUsage()

	case "version", "-v", "--version":
		fmt.Printf("Telegram Coding Agent Relay v%s\n", version)

	case "start":
		if err := cmd.StartDaemon(envPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "status":
		if err := cmd.StatusCmd(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "reload":
		if err := cmd.ReloadCmd(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "stop":
		if err := cmd.StopCmd(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "restart":
		if err := cmd.RestartCmd(envPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "run":
		runForeground(envPath)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runForeground(envPath string) {
	log.Printf("[INFO] Initializing Telegram Coding Agent Relay v%s...", version)

	cfg, err := config.Load(envPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to load configuration: %v", err)
	}

	// Setup OS signal handling for graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	// Start local control server
	ctrlServer := server.NewControlServer(cfg, shutdownCh)
	actualPort, err := ctrlServer.Start()
	if err != nil {
		log.Fatalf("[FATAL] Failed to start control server: %v", err)
	}

	// Record PID and Port for CLI status/stop/reload
	pid := os.Getpid()
	if err := cmd.SavePID(pid); err != nil {
		log.Printf("[WARN] Failed to write PID file: %v", err)
	}
	if err := cmd.SavePort(actualPort); err != nil {
		log.Printf("[WARN] Failed to write Port file: %v", err)
	}
	defer cmd.CleanupRuntimeFiles()

	log.Printf("[INFO] Control IPC listening on 127.0.0.1:%d (PID: %d)", actualPort, pid)

	// Create cancellable context for Telegram bot
	ctx, cancel := context.WithCancel(context.Background())

	// Start Telegram Bot
	bot := telegram.NewBot(cfg)
	errCh := make(chan error, 1)
	go func() {
		errCh <- bot.Run(ctx)
	}()

	// Wait for shutdown signal or fatal error
	select {
	case sig := <-shutdownCh:
		log.Printf("[INFO] Received signal %v, shutting down...", sig)
	case err := <-errCh:
		if err != nil {
			log.Printf("[ERROR] Bot stopped with error: %v", err)
		}
	}

	// Graceful shutdown
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = ctrlServer.Stop(shutdownCtx)

	log.Printf("[INFO] Telegram Coding Agent Relay stopped.")
}
