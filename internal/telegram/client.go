package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	botToken   string
	chatID     int64
	httpClient *http.Client
}

type sendMessageRequest struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

func NewClient(botToken string, chatID int64) *Client {
	return &Client{
		botToken: botToken,
		chatID:   chatID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage sends an HTML or plain text message to the configured Telegram chat.
func (c *Client) SendMessage(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)

	reqPayload := sendMessageRequest{
		ChatID:                c.chatID,
		Text:                  text,
		ParseMode:             "HTML",
		DisableWebPagePreview: false,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request to telegram failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read telegram response: %w", err)
	}

	var tgResp telegramResponse
	if err := json.Unmarshal(respBytes, &tgResp); err != nil {
		return fmt.Errorf("failed to parse telegram response (%s): %w", string(respBytes), err)
	}

	if !tgResp.OK {
		return fmt.Errorf("telegram API error [%d]: %s", tgResp.ErrorCode, tgResp.Description)
	}

	log.Printf("[Telegram] Message sent successfully to chat %d", c.chatID)
	return nil
}
