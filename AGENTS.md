# Project agent memory

A lightweight cross-platform bridge in Go (standard library only) relaying Telegram bot messages to local coding agent CLIs.

## Build and test

- **Build:** `go build -o relay .` (produces standalone binary; zero Cgo or external dependencies)
- **Test:** `go test ./...`
- **Cross-compile:** `GOOS=windows/linux/darwin GOARCH=amd64/arm64 go build -o relay-<target> .`

## Architecture and layout

- `main.go`: CLI entrypoint with subcommands (`start`, `run`, `status`, `reload`, `stop`, `restart`, `version`).
- `config/config.go`: Thread-safe configuration (`Snapshot`, `.env` parsing, dynamic `WORKING_DIR` path validation and resolution via `ExpandPath`, mutex-guarded hot reloading).
- `cmd/`: Daemon lifecycle management (`daemon.go`), local IPC client (`client.go`), and OS-specific detachment (`sys_unix.go`, `sys_windows.go`).
- `server/control.go`: Loopback HTTP IPC server (`127.0.0.1:<port>`) for daemon control (`/status`, `/reload`, `/stop`, `/cd`).
- `telegram/`:
  - `bot.go`: Long-polling update loop, user whitelist authorization (`ALLOWED_USER_IDS`), command routing (`/new`, `/c`, `/cd`, `/pwd`, `/status`, `/reload`, `/cancel`, `/help`), and execution dispatch.
  - `client.go`: Lightweight pure-Go HTTP client for Telegram Bot API.
  - `streamer.go`: Throttled streaming output buffer (`StreamSession`) updating Telegram live, formatting completion footers, and splitting long output across sequential messages.
  - `formatter.go`: Markdown to Telegram HTML converter (`MarkdownToTelegramHTML`) and tag-preserving 4096-char message chunker (`SplitTelegramHTML`).
- `runner/executor.go`: Cross-platform shell executor supporting PowerShell, CMD, Bash, and POSIX sh with real-time stdout/stderr pipes and `{prompt}` / `RELAY_PROMPT` injection.

## Conventions and sharp edges

- **Zero dependencies:** Use only the Go standard library. Do not introduce third-party packages or external modules into `go.mod`.
- **Telegram HTML formatting:** All messages sent with parse mode `HTML` must be properly escaped or processed through `formatter.go`. Unescaped `<` or `&` in raw output will cause Telegram API 400 Bad Request errors.
- **Telegram length limits:** Messages hard-cap at 4096 characters; `SplitTelegramHTML` targets <=3800 characters to reserve tag and footer headroom while preserving open tags across chunks.
- **Single active task per chat:** Agent runs are serialized per chat ID via `activeTasks` and bound to a cancelable context (`/cancel` triggers context cancellation).
- **Runtime artifacts:** The daemon writes `.relay.pid` and `.relay.port` in the execution directory, cleaned up on graceful shutdown.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
