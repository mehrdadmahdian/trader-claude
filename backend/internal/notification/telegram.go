package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const telegramBaseURL = "https://api.telegram.org/bot"

// TelegramSender sends messages via the Telegram Bot API.
type TelegramSender struct {
	botToken   string
	httpClient *http.Client
}

// NewTelegramSender creates a sender with the given bot token.
func NewTelegramSender(botToken string) *TelegramSender {
	return &TelegramSender{
		botToken:   botToken,
		httpClient: &http.Client{},
	}
}

// SetHTTPClient replaces the default HTTP client (useful for testing).
func (t *TelegramSender) SetHTTPClient(c *http.Client) {
	t.httpClient = c
}

// telegramAPIURL allows overriding the Telegram API URL for testing.
var telegramAPIURL = telegramBaseURL

// SendText sends a plain-text message to the given chat.
func (t *TelegramSender) SendText(ctx context.Context, chatID, text string) error {
	body := map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	b, _ := json.Marshal(body)
	url := fmt.Sprintf("%s%s/sendMessage", telegramAPIURL, t.botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram: sendMessage failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SendPhoto sends an image with an optional caption.
func (t *TelegramSender) SendPhoto(ctx context.Context, chatID string, image []byte, caption string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	_ = w.WriteField("chat_id", chatID)
	if caption != "" {
		_ = w.WriteField("caption", caption)
	}

	part, err := w.CreateFormFile("photo", "card.png")
	if err != nil {
		return fmt.Errorf("telegram: create form file: %w", err)
	}
	if _, err := part.Write(image); err != nil {
		return fmt.Errorf("telegram: write image: %w", err)
	}
	w.Close()

	url := fmt.Sprintf("%s%s/sendPhoto", telegramAPIURL, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram: sendPhoto failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// TestConnection verifies the bot token by calling getMe.
func (t *TelegramSender) TestConnection(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s%s/getMe", telegramAPIURL, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram: getMe failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("telegram: decode getMe: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("telegram: getMe returned ok=false")
	}
	return result.Result.Username, nil
}
