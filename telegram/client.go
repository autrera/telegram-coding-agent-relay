package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client interacts with the Telegram Bot API using standard HTTP.
type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Telegram API client.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
	}
}

// UpdateToken dynamically updates the bot token (for hot reloading).
func (c *Client) UpdateToken(token string) {
	c.token = token
	c.baseURL = fmt.Sprintf("https://api.telegram.org/bot%s", token)
}

func (c *Client) postJSON(endpoint string, payload any, result any) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if result != nil {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("failed to parse response %s: %w", string(respBytes), err)
		}
	}
	return nil
}

// GetMe retrieves the bot's user info.
func (c *Client) GetMe() (*User, error) {
	var res APIResponse[User]
	err := c.postJSON("getMe", map[string]any{}, &res)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, fmt.Errorf("telegram API error (%d): %s", res.ErrorCode, res.Description)
	}
	return &res.Result, nil
}

// GetUpdates fetches new updates using long polling.
func (c *Client) GetUpdates(offset int64, timeoutSec int) ([]Update, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message"},
	}

	var res APIResponse[[]Update]
	err := c.postJSON("getUpdates", payload, &res)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, fmt.Errorf("getUpdates error (%d): %s", res.ErrorCode, res.Description)
	}
	return res.Result, nil
}

// SendMessage sends a message to a chat, with automatic HTML fallback to plain text if needed.
func (c *Client) SendMessage(chatID int64, text string, parseMode string, replyToMessageID int64) (*Message, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	if replyToMessageID != 0 {
		payload["reply_to_message_id"] = replyToMessageID
	}

	var res APIResponse[Message]
	err := c.postJSON("sendMessage", payload, &res)
	if err != nil {
		return nil, err
	}

	if !res.OK {
		// If HTML parse failed, retry with stripped plain text
		if parseMode == "HTML" && strings.Contains(strings.ToLower(res.Description), "can't parse entities") {
			payload["parse_mode"] = ""
			payload["text"] = StripHTML(text)
			var retryRes APIResponse[Message]
			if retryErr := c.postJSON("sendMessage", payload, &retryRes); retryErr == nil && retryRes.OK {
				return &retryRes.Result, nil
			}
		}
		return nil, fmt.Errorf("sendMessage error (%d): %s", res.ErrorCode, res.Description)
	}
	return &res.Result, nil
}

// EditMessageText edits the text of an existing message with automatic fallback.
func (c *Client) EditMessageText(chatID int64, messageID int64, text string, parseMode string) (*Message, error) {
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}

	var res APIResponse[Message]
	err := c.postJSON("editMessageText", payload, &res)
	if err != nil {
		return nil, err
	}

	if !res.OK {
		// Ignore "message is not modified" error
		if strings.Contains(strings.ToLower(res.Description), "message is not modified") {
			return nil, nil
		}
		// If HTML entity parse error, retry with stripped plain text
		if parseMode == "HTML" && strings.Contains(strings.ToLower(res.Description), "can't parse entities") {
			payload["parse_mode"] = ""
			payload["text"] = StripHTML(text)
			var retryRes APIResponse[Message]
			if retryErr := c.postJSON("editMessageText", payload, &retryRes); retryErr == nil && retryRes.OK {
				return &retryRes.Result, nil
			}
		}
		return nil, fmt.Errorf("editMessageText error (%d): %s", res.ErrorCode, res.Description)
	}
	return &res.Result, nil
}

// SendChatAction sends a chat action like "typing".
func (c *Client) SendChatAction(chatID int64, action string) {
	payload := map[string]any{
		"chat_id": chatID,
		"action":  action,
	}
	_ = c.postJSON("sendChatAction", payload, nil)
}
