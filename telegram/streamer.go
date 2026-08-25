package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"
)

// StreamSession manages live-streaming of command output to Telegram messages.
type StreamSession struct {
	client       *Client
	chatID       int64
	replyToID    int64
	interval     time.Duration
	mu           sync.Mutex
	accumulated  strings.Builder
	lastRendered string
	initialMsgID int64
	done         chan struct{}
}

// NewStreamSession initializes and starts a live stream session in Telegram.
func NewStreamSession(client *Client, chatID int64, replyToID int64, interval time.Duration) (*StreamSession, error) {
	initialMsg, err := client.SendMessage(chatID, "⏳ <i>Agent is thinking...</i>", "HTML", replyToID)
	if err != nil {
		return nil, fmt.Errorf("failed to send initial message: %w", err)
	}

	session := &StreamSession{
		client:       client,
		chatID:       chatID,
		replyToID:    replyToID,
		interval:     interval,
		initialMsgID: initialMsg.MessageID,
		done:         make(chan struct{}),
	}

	return session, nil
}

// StartUpdater runs the background update ticker until Finish is called or ctx is canceled.
func (s *StreamSession) StartUpdater(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		// Send initial typing action
		s.client.SendChatAction(s.chatID, "typing")

		for {
			select {
			case <-s.done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.client.SendChatAction(s.chatID, "typing")
				s.flushUpdate(false)
			}
		}
	}()
}

// Append adds newly streamed output chunks to the session buffer.
func (s *StreamSession) Append(chunk string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accumulated.WriteString(chunk)
}

// flushUpdate updates the telegram message if content changed.
func (s *StreamSession) flushUpdate(isFinal bool) {
	s.mu.Lock()
	rawText := s.accumulated.String()
	s.mu.Unlock()

	if strings.TrimSpace(rawText) == "" {
		return
	}

	if rawText == s.lastRendered && !isFinal {
		return
	}

	s.lastRendered = rawText

	// Format markdown to HTML
	formattedHTML := MarkdownToTelegramHTML(rawText)
	if !isFinal {
		formattedHTML += "\n\n⏳ <i>Streaming output...</i>"
	}

	// Keep within single message size during live streaming
	if len(formattedHTML) > 3900 {
		chunks := SplitTelegramHTML(formattedHTML, 3900)
		if len(chunks) > 0 {
			formattedHTML = chunks[0] + "\n\n<i>... (output continues)</i>"
		}
	}

	_, _ = s.client.EditMessageText(s.chatID, s.initialMsgID, formattedHTML, "HTML")
}

// Finish finalizes the streaming output, splitting across multiple messages if output exceeds 4096 chars.
func (s *StreamSession) Finish(duration time.Duration, exitCode int, execErr error) {
	close(s.done)

	s.mu.Lock()
	rawText := s.accumulated.String()
	s.mu.Unlock()

	var footer string
	if execErr != nil {
		if strings.TrimSpace(rawText) == "" {
			footer = fmt.Sprintf("\n\n❌ <b>Process failed to start:</b>\n<code>%s</code>\n\n❌ <b>Process exited with code %d</b> (took %.1fs)", html.EscapeString(execErr.Error()), exitCode, duration.Seconds())
		} else {
			footer = fmt.Sprintf("\n\n❌ <b>Error:</b> <code>%s</code>\n❌ <b>Process exited with code %d</b> (took %.1fs)", html.EscapeString(execErr.Error()), exitCode, duration.Seconds())
		}
	} else if exitCode != 0 {
		footer = fmt.Sprintf("\n\n❌ <b>Process exited with code %d</b> (took %.1fs)", exitCode, duration.Seconds())
	} else {
		footer = fmt.Sprintf("\n\n✅ <b>Completed</b> in %.1fs", duration.Seconds())
	}

	if strings.TrimSpace(rawText) == "" {
		finalMsg := "<i>(Command completed with no output)</i>" + footer
		_, _ = s.client.EditMessageText(s.chatID, s.initialMsgID, finalMsg, "HTML")
		return
	}

	// Convert full output to HTML
	fullHTML := MarkdownToTelegramHTML(rawText)
	chunks := SplitTelegramHTML(fullHTML, 3800)

	if len(chunks) == 0 {
		_, _ = s.client.EditMessageText(s.chatID, s.initialMsgID, footer, "HTML")
		return
	}

	// Edit first chunk into the initial placeholder message
	firstChunk := chunks[0]
	if len(chunks) == 1 {
		firstChunk += footer
	}
	_, _ = s.client.EditMessageText(s.chatID, s.initialMsgID, firstChunk, "HTML")

	// Send remaining chunks as sequential reply messages
	for i := 1; i < len(chunks); i++ {
		chunkText := chunks[i]
		if i == len(chunks)-1 {
			chunkText += footer
		}
		_, _ = s.client.SendMessage(s.chatID, chunkText, "HTML", 0)
		time.Sleep(300 * time.Millisecond) // gentle spacing
	}
}
