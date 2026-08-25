package telegram

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"relay/config"
	"relay/runner"
)

// Bot manages the Telegram polling loop, routing commands to the agent runner.
type Bot struct {
	cfg       *config.Config
	client    *Client
	runner    *runner.Executor
	startTime time.Time

	mu          sync.Mutex
	activeTasks map[int64]context.CancelFunc
}

// NewBot creates a new Telegram relay Bot.
func NewBot(cfg *config.Config) *Bot {
	snap := cfg.Get()
	return &Bot{
		cfg:         cfg,
		client:      NewClient(snap.TelegramBotToken),
		runner:      runner.NewExecutor(),
		startTime:   time.Now(),
		activeTasks: make(map[int64]context.CancelFunc),
	}
}

// Run starts the Telegram long-polling loop.
func (b *Bot) Run(ctx context.Context) error {
	snap := b.cfg.Get()
	b.client.UpdateToken(snap.TelegramBotToken)

	me, err := b.client.GetMe()
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram: %w", err)
	}

	if err := b.client.SetMyCommands(DefaultBotCommands); err != nil {
		log.Printf("[WARN] Failed to register bot commands: %v", err)
	} else {
		log.Printf("[INFO] Bot commands registered with Telegram")
	}

	log.Printf("[INFO] Telegram Bot started as @%s (ID: %d)", me.Username, me.ID)
	log.Printf("[INFO] Active Working Directory: %s", snap.WorkingDir)

	var offset int64 = 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Stopping Telegram bot loop...")
			return nil
		default:
		}

		updates, err := b.client.GetUpdates(offset, 25)
		if err != nil {
			log.Printf("[WARN] Failed to get updates: %v (retrying in 3s...)", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}

			if update.Message != nil {
				go b.handleMessage(update.Message)
			}
		}
	}
}

func (b *Bot) handleMessage(msg *Message) {
	if msg.From == nil {
		return
	}

	senderID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if text == "" {
		return
	}

	// Security / Authorization check
	if !b.cfg.IsUserAllowed(senderID) {
		log.Printf("[WARN] Unauthorized message from user ID: %d (%s %s @%s): %s",
			senderID, msg.From.FirstName, msg.From.LastName, msg.From.Username, text)
		reply := fmt.Sprintf("⛔ <b>Access Denied</b>\n\nYour Telegram User ID is: <code>%d</code>\nAdd this ID to <code>ALLOWED_USER_IDS</code> in your <code>.env</code> file to enable access.", senderID)
		_, _ = b.client.SendMessage(chatID, reply, "HTML", msg.MessageID)
		return
	}

	// Command routing
	switch {
	case text == "/start" || text == "/help":
		b.cmdHelp(chatID, msg.MessageID)
	case text == "/status":
		b.cmdStatus(chatID, msg.MessageID)
	case text == "/reload":
		b.cmdReload(chatID, msg.MessageID)
	case text == "/pwd":
		b.cmdPwd(chatID, msg.MessageID)
	case strings.HasPrefix(text, "/cd"):
		b.cmdCd(chatID, msg.MessageID, strings.TrimSpace(strings.TrimPrefix(text, "/cd")))
	case text == "/cancel":
		b.cmdCancel(chatID, msg.MessageID)
	case strings.HasPrefix(text, "/new"):
		prompt := strings.TrimSpace(strings.TrimPrefix(text, "/new"))
		if prompt == "" {
			_, _ = b.client.SendMessage(chatID, "ℹ️ Please specify a prompt after <code>/new</code>, e.g.:\n<code>/new Build a web scraper in Go</code>", "HTML", msg.MessageID)
			return
		}
		b.executeAgent(chatID, msg.MessageID, prompt, true)
	case strings.HasPrefix(text, "/c "):
		prompt := strings.TrimSpace(strings.TrimPrefix(text, "/c "))
		b.executeAgent(chatID, msg.MessageID, prompt, false)
	default:
		// Default: continue conversation
		b.executeAgent(chatID, msg.MessageID, text, false)
	}
}

func (b *Bot) cmdHelp(chatID, replyToID int64) {
	snap := b.cfg.Get()
	helpText := fmt.Sprintf(`🤖 <b>Telegram Coding Agent Relay</b>

Send any text message to communicate with the coding agent. By default, messages continue the existing session.

<b>Commands:</b>
• <code>/new &lt;prompt&gt;</code> - Start a brand new conversation
• <code>/c &lt;prompt&gt;</code> - Continue previous session (or just type regular text)
• <code>/cd &lt;path&gt;</code> - Switch working directory
• <code>/pwd</code> - View current working directory
• <code>/status</code> - View relay status and uptime
• <code>/reload</code> - Reload <code>.env</code> file changes on the fly
• <code>/cancel</code> - Abort the currently running agent command
• <code>/help</code> - Show this menu

📁 <b>Current Directory:</b> <code>%s</code>`, htmlEscape(snap.WorkingDir))

	_, _ = b.client.SendMessage(chatID, helpText, "HTML", replyToID)
}

func (b *Bot) cmdStatus(chatID, replyToID int64) {
	snap := b.cfg.Get()
	uptime := time.Since(b.startTime).Round(time.Second)

	statusText := fmt.Sprintf(`📊 <b>Relay Status</b>

• <b>Status:</b> 🟢 Running
• <b>Uptime:</b> %s
• <b>Working Directory:</b> <code>%s</code>
• <b>Shell:</b> <code>%s</code>
• <b>Timeout:</b> %v
• <b>Stream Interval:</b> %v
• <b>Allowed Users:</b> %d configured

<b>Command Templates:</b>
• <i>Continue:</i> <code>%s</code>
• <i>New:</i> <code>%s</code>`,
		uptime,
		htmlEscape(snap.WorkingDir),
		htmlEscape(snap.ShellType),
		snap.CommandTimeout,
		snap.StreamEditInterval,
		len(snap.AllowedUserIDs),
		htmlEscape(snap.ContinueCommand),
		htmlEscape(snap.NewCommand),
	)

	_, _ = b.client.SendMessage(chatID, statusText, "HTML", replyToID)
}

func (b *Bot) cmdReload(chatID, replyToID int64) {
	if err := b.cfg.Reload(); err != nil {
		_, _ = b.client.SendMessage(chatID, fmt.Sprintf("❌ <b>Failed to reload .env:</b> %s", htmlEscape(err.Error())), "HTML", replyToID)
		return
	}
	snap := b.cfg.Get()
	b.client.UpdateToken(snap.TelegramBotToken)
	if err := b.client.SetMyCommands(DefaultBotCommands); err != nil {
		log.Printf("[WARN] Failed to register bot commands: %v", err)
	}
	log.Printf("[INFO] Configuration reloaded via Telegram /reload")
	_, _ = b.client.SendMessage(chatID, "✅ <b>Configuration reloaded successfully from .env</b>", "HTML", replyToID)
}

func (b *Bot) cmdPwd(chatID, replyToID int64) {
	snap := b.cfg.Get()
	_, _ = b.client.SendMessage(chatID, fmt.Sprintf("📁 <b>Working Directory:</b>\n<code>%s</code>", htmlEscape(snap.WorkingDir)), "HTML", replyToID)
}

func (b *Bot) cmdCd(chatID, replyToID int64, newDir string) {
	if newDir == "" {
		_, _ = b.client.SendMessage(chatID, "ℹ️ Usage: <code>/cd &lt;path&gt;</code>", "HTML", replyToID)
		return
	}

	snap := b.cfg.Get()
	targetPath := newDir
	if !filepath.IsAbs(targetPath) && !strings.HasPrefix(targetPath, "~") {
		targetPath = filepath.Join(snap.WorkingDir, targetPath)
	}

	updated, err := b.cfg.SetWorkingDir(targetPath)
	if err != nil {
		_, _ = b.client.SendMessage(chatID, fmt.Sprintf("❌ <b>Cannot change directory:</b> %s", htmlEscape(err.Error())), "HTML", replyToID)
		return
	}

	log.Printf("[INFO] Working directory changed to: %s", updated)
	_, _ = b.client.SendMessage(chatID, fmt.Sprintf("📁 <b>Working directory changed to:</b>\n<code>%s</code>", htmlEscape(updated)), "HTML", replyToID)
}

func (b *Bot) cmdCancel(chatID, replyToID int64) {
	b.mu.Lock()
	cancel, exists := b.activeTasks[chatID]
	if exists {
		cancel()
		delete(b.activeTasks, chatID)
		b.mu.Unlock()
		_, _ = b.client.SendMessage(chatID, "🛑 <b>Agent execution canceled.</b>", "HTML", replyToID)
	} else {
		b.mu.Unlock()
		_, _ = b.client.SendMessage(chatID, "ℹ️ No running agent task found in this chat.", "HTML", replyToID)
	}
}

func (b *Bot) executeAgent(chatID, msgID int64, prompt string, isNew bool) {
	snap := b.cfg.Get()

	// Check if already running a task in this chat
	b.mu.Lock()
	if _, exists := b.activeTasks[chatID]; exists {
		b.mu.Unlock()
		_, _ = b.client.SendMessage(chatID, "⚠️ <i>An agent task is already running. Please wait or use /cancel to abort it.</i>", "HTML", msgID)
		return
	}

	// Create cancellable context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), snap.CommandTimeout)
	b.activeTasks[chatID] = cancel
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.activeTasks, chatID)
		b.mu.Unlock()
		cancel()
	}()

	cmdTemplate := snap.ContinueCommand
	if isNew {
		cmdTemplate = snap.NewCommand
	}

	log.Printf("[RUN] Executing (isNew=%v, dir=%s): %s", isNew, snap.WorkingDir, prompt)

	// Initialize live streaming session
	streamSession, err := NewStreamSession(b.client, chatID, msgID, snap.StreamEditInterval)
	if err != nil {
		log.Printf("[ERROR] Failed to start stream session: %v", err)
		return
	}

	streamSession.StartUpdater(ctx)

	// Execute command with real-time stream callback
	res := b.runner.Execute(ctx, cmdTemplate, prompt, snap.WorkingDir, snap.ShellType, func(chunk string, isStderr bool) {
		streamSession.Append(chunk)
	})

	log.Printf("[DONE] Execution finished in %.2fs (exit code %d)", res.Duration.Seconds(), res.ExitCode)
	streamSession.Finish(res.Duration, res.ExitCode, res.Err)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// DefaultBotCommands is the command list registered with Telegram for the
// [/] menu and slash autocomplete.
var DefaultBotCommands = []BotCommand{
	{Command: "new", Description: "Start a brand new conversation"},
	{Command: "c", Description: "Continue previous conversation"},
	{Command: "cd", Description: "Switch working directory"},
	{Command: "pwd", Description: "View current working directory"},
	{Command: "status", Description: "View relay status and uptime"},
	{Command: "reload", Description: "Reload .env configuration dynamically"},
	{Command: "cancel", Description: "Abort the currently running agent command"},
	{Command: "help", Description: "Show help and command menu"},
}
