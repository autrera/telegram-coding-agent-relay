# 📡 Telegram Coding Agent Relay Configuration

A lightweight, high-performance, cross-platform bridge in **Go** that relays messages from a **Telegram Bot** directly to local coding agents.

---

## ✨ Features

- **⚡ Fast & Zero-Dependency:** Written in pure Go with no external Cgo or heavy dependencies. Single standalone executable.
- **🔄 Session Continuity & `/new` Support:** Messages automatically continue previous conversation context, while `/new <prompt>` starts fresh sessions.
- **📺 Throttled Live Streaming:** Streams agent output in real-time to Telegram, rate-limited to avoid Telegram API throttle limits (429s).
- **📝 Markdown $\to$ Telegram HTML Converter:** Translates markdown (headers, bold/italic, fenced code blocks with language highlighting, blockquotes, links, bullet lists) into Telegram-compliant HTML without entity parsing failures.
- **✂️ Tag-Safe 4096-Char Message Chunker:** Long responses that exceed Telegram's message limits are split seamlessly across messages while preserving and reopening open `<pre><code>` and other tags.
- **🛡️ Whitelist Security:** Restricts execution exclusively to authorized Telegram User IDs defined in `.env`.
- **🚀 Daemon & Background CLI:** Manage background lifecycle with `relay start`, `relay stop`, `relay status`, and `relay restart`.
- **🔄 Hot Configuration Reloading:** Use `relay reload` (or the `/reload` bot command) to update `.env` settings without dropping connections or restarting the bot.
- **🌐 Cross-Platform:** Works on **Windows**, **macOS**, and **Linux** with native OS shell integration (`powershell`/`cmd` on Windows, `/bin/sh` or `/bin/bash` on Unix).

---

## 🛠️ Quick Start

### 1. Prerequisites
- **Go 1.22+** installed ([Download Go](https://go.dev/dl/))
- A **coding** CLI installed and authenticated
- A **Telegram Bot Token** from [@BotFather](https://t.me/BotFather)
- Your **Telegram User ID** (get it from [@userinfobot](https://t.me/userinfobot))

### 2. Configuration
Copy the `.env.example` file to `.env`:

```bash
cp .env.example .env
```

Edit `.env` and fill in your credentials:

```ini
TELEGRAM_BOT_TOKEN="123456789:ABCDefGhIjKlMnOpQrStUvWxYz"
ALLOWED_USER_IDS="123456789"
WORKING_DIR="C:\Users\MadGamer\Projects"

# --- Option 1: OpenCode (opencode) ---
CONTINUE_COMMAND="opencode run -c --auto \"{prompt}\""
NEW_COMMAND="opencode run --auto \"{prompt}\""

# --- Option 2: Pi (@earendil-works/pi-coding-agent) ---
# CONTINUE_COMMAND="pi -c -p \"{prompt}\" -a"
# NEW_COMMAND="pi -p \"{prompt}\" -a"

# --- Option 3: Claude Code (claude) ---
# CONTINUE_COMMAND="claude -c -p \"{prompt}\" --dangerously-skip-permissions"
# NEW_COMMAND="claude -p \"{prompt}\" --dangerously-skip-permissions"

# --- Option 4: Google Antigravity (agy) ---
# CONTINUE_COMMAND="agy -c -p \"{prompt}\" --dangerously-skip-permissions"
# NEW_COMMAND="agy -p \"{prompt}\" --dangerously-skip-permissions"
```

### 3. Build

```bash
# Build the binary
go build -o relay.exe .
```

---

## 💻 CLI Commands

| Command | Description |
| :--- | :--- |
| `relay start` | Launches the relay as a **detached background daemon**. |
| `relay run` | Runs in the foreground (useful for development & live debugging). |
| `relay status` | Displays daemon PID, port, uptime, active working directory, and allowed users. |
| `relay reload` | Re-reads the `.env` file and applies changes **on the fly**. |
| `relay stop` | Gracefully terminates the running background daemon. |
| `relay restart` | Stops and restarts the background daemon. |

---

## 🤖 Telegram Bot Commands

When chatting with your bot in Telegram:

| Command | Action |
| :--- | :--- |
| `<any message>` | Continues the active coding conversation in the working directory. |
| `/new <prompt>` | Starts a brand new agent conversation. |
| `/c <prompt>` | Explicit continue conversation (same as regular message). |
| `/cd <path>` | Changes active working directory for future agent commands. |
| `/pwd` | Displays current active working directory. |
| `/status` | Shows relay uptime, working directory, and configured command templates. |
| `/reload` | Reloads `.env` settings dynamically without restarting the bot. |
| `/cancel` | Aborts the currently running agent task. |
| `/help` | Shows command menu. |

---

## 🌍 Cross-Platform Compilation

You can compile binaries for different operating systems from your current machine:

```bash
# Windows (64-bit)
GOOS=windows GOARCH=amd64 go build -o relay.exe .

# Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -o relay-linux .

# macOS (Apple Silicon M1/M2/M3/M4)
GOOS=darwin GOARCH=arm64 go build -o relay-darwin-arm64 .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o relay-darwin-amd64 .
```

---

## 📂 Project Structure

```
relay/
├── .env.example          # Sample environment configuration
├── .gitignore            # Git ignore rules
├── go.mod                # Go module definition
├── main.go               # CLI entrypoint & signal router
├── cmd/
│   ├── client.go         # Local IPC client for CLI commands (status, stop, reload)
│   ├── daemon.go         # Detached daemon launcher & PID/port management
│   ├── sys_windows.go    # Windows-specific process detachment & inspection
│   └── sys_unix.go       # Unix/macOS-specific process detachment & inspection
├── config/
│   └── config.go         # Thread-safe .env loader, path cleaner, validator
├── runner/
│   └── executor.go       # Cross-platform shell executor & real-time stream pipe
├── server/
│   └── control.go        # Local loopback (127.0.0.1) IPC server for CLI commands
└── telegram/
    ├── bot.go            # Telegram long-polling router, auth & command handler
    ├── client.go         # Lightweight Telegram Bot API client
    ├── formatter.go      # Markdown to Telegram HTML converter & tag-safe chunker
    ├── streamer.go       # Throttled live-stream message updater
    ├── types.go          # Telegram API data structures
    └── formatter_test.go # Unit tests for markdown conversion & tag chunking
```
