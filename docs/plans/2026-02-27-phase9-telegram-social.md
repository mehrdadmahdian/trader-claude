# Phase 9 — Telegram Bot & Social Card Generator — Atomic Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Telegram bot sender, a webhook sender with HMAC-SHA256 signatures, a social card image generator for backtest results and signals, and a settings page for configuring notification channels.

**Architecture:** Two new backend packages: `internal/notification/` (Telegram + webhook senders) and `internal/social/` (PNG card generation via the `gg` library). A `settings` table stores per-user notification configuration. The frontend adds a Share modal on backtest results and a Notifications section on the Settings page.

**Tech Stack:** Go 1.24 (Fiber v2, GORM, `gg` for 2D graphics), React 18 (React Query v5, Zustand, Tailwind, shadcn/ui). New Go dependency: `github.com/fogleman/gg`.

**Execution strategy:** Each task = one focused action on ≤ 3 files. Give Haiku the task text + only the files listed under "Read first".

**Design doc:** `docs/plans/2026-02-27-phase9-telegram-social-design.md`

---

## BACKEND TASKS

---

### Task B1: Add Setting model to models.go + autoMigrate

**Read first:**
- `backend/internal/models/models.go` (last 30 lines)
- `backend/cmd/server/main.go` (autoMigrate function)

**Files to modify:**
- `backend/internal/models/models.go`
- `backend/cmd/server/main.go`

---

**Step 1: Append Setting struct to models.go**

After the last model in `models.go`, append:

```go
// --- Setting ---

// Setting stores per-user configuration as key-value JSON pairs.
type Setting struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"default:1;uniqueIndex:idx_user_key" json:"user_id"`
	Key       string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_user_key" json:"key"`
	Value     JSON      `gorm:"type:json" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string { return "settings" }
```

---

**Step 2: Add `&models.Setting{}` to autoMigrate in main.go**

---

**Step 3: Verify compile**

```bash
make backend-fmt
docker compose exec backend go build ./...
```

---

**Step 4: Commit**

```bash
git add backend/internal/models/models.go backend/cmd/server/main.go
git commit -m "feat(phase9): add Setting model and autoMigrate"
```

---

### Task B2: Create internal/notification/telegram.go + tests

**Read first:** Nothing — standalone package.

**Files to create:**
- `backend/internal/notification/telegram.go`
- `backend/internal/notification/telegram_test.go`

---

**Step 1: Create `telegram.go`**

```go
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

// SetBaseURL allows overriding the Telegram API URL for testing.
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
```

---

**Step 2: Create `telegram_test.go`**

```go
package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendText_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("test-token")
	err := sender.SendText(context.Background(), "123", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendText_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("bad-token")
	err := sender.SendText(context.Background(), "123", "Hello")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestSendPhoto_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			t.Error("expected Content-Type header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("test-token")
	err := sender.SendPhoto(context.Background(), "123", []byte{0x89, 0x50, 0x4e, 0x47}, "test caption")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{"username":"test_bot"}}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("test-token")
	name, err := sender.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "test_bot" {
		t.Errorf("expected test_bot, got %s", name)
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/notification/... -v
```

Expected: all 4 tests PASS.

---

**Step 4: Commit**

```bash
git add backend/internal/notification/
git commit -m "feat(phase9): add TelegramSender with SendText, SendPhoto, TestConnection"
```

---

### Task B3: Create internal/notification/webhook.go + tests

**Read first:** Nothing — standalone.

**Files to create:**
- `backend/internal/notification/webhook.go`
- `backend/internal/notification/webhook_test.go`

---

**Step 1: Create `webhook.go`**

```go
package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookSender sends JSON payloads to a webhook URL with HMAC-SHA256 signatures.
type WebhookSender struct {
	httpClient *http.Client
}

// NewWebhookSender creates a WebhookSender.
func NewWebhookSender() *WebhookSender {
	return &WebhookSender{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts the payload to the URL, signed with HMAC-SHA256.
// Retries up to 3 times on 5xx errors with exponential backoff.
func (w *WebhookSender) Send(ctx context.Context, url, secret string, payload []byte) error {
	sig := computeHMAC(secret, payload)

	var lastErr error
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoffs[attempt-1]):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("webhook: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-TraderClaude-Signature", "sha256="+sig)

		resp, err := w.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("webhook: request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("webhook: status %d: %s", resp.StatusCode, string(body))

		if resp.StatusCode < 500 {
			return lastErr
		}
	}

	return fmt.Errorf("webhook: exhausted retries: %w", lastErr)
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of payload using secret.
func computeHMAC(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks an incoming webhook signature. Useful for documentation/examples.
func VerifySignature(secret string, payload []byte, signature string) bool {
	expected := "sha256=" + computeHMAC(secret, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}
```

---

**Step 2: Create `webhook_test.go`**

```go
package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestWebhookSend_Success(t *testing.T) {
	var gotSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-TraderClaude-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookSender()
	payload := []byte(`{"event":"test"}`)
	err := ws.Send(context.Background(), server.URL, "my-secret", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSig == "" {
		t.Error("expected signature header")
	}
	if !VerifySignature("my-secret", payload, gotSig) {
		t.Error("signature verification failed")
	}
}

func TestWebhookSend_RetryOn500(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookSender()
	err := ws.Send(context.Background(), server.URL, "secret", []byte(`{}`))
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestWebhookSend_NoRetryOn4xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	ws := NewWebhookSender()
	err := ws.Send(context.Background(), server.URL, "secret", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestVerifySignature(t *testing.T) {
	payload := []byte(`{"test":true}`)
	sig := "sha256=" + computeHMAC("secret", payload)
	if !VerifySignature("secret", payload, sig) {
		t.Error("expected signature to verify")
	}
	if VerifySignature("wrong-secret", payload, sig) {
		t.Error("expected signature to fail with wrong secret")
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/notification/... -v
```

Expected: all tests PASS.

---

**Step 4: Commit**

```bash
git add backend/internal/notification/webhook.go backend/internal/notification/webhook_test.go
git commit -m "feat(phase9): add WebhookSender with HMAC-SHA256 and retry logic"
```

---

### Task B4: Add `gg` dependency + create internal/social/card.go with GenerateBacktestCard

**Read first:**
- `backend/internal/backtest/engine.go` (EquityPoint struct — first 30 lines)

**Files to create:**
- `backend/internal/social/card.go`
- `backend/internal/social/card_test.go`

---

**Step 1: Add `gg` dependency**

```bash
docker compose exec backend go get github.com/fogleman/gg
```

---

**Step 2: Create `card.go`**

Create `backend/internal/social/card.go`:

```go
package social

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"math"

	"github.com/fogleman/gg"
)

const (
	cardWidth  = 1200
	cardHeight = 630
)

// BacktestCardOpts holds the data for a backtest social card.
type BacktestCardOpts struct {
	Theme        string    // "dark" | "light"
	StrategyName string
	Symbol       string
	Timeframe    string
	DateRange    string
	TotalReturn  float64
	SharpeRatio  float64
	MaxDrawdown  float64
	WinRate      float64
	EquityCurve  []float64 // y-values for sparkline
}

// SignalCardOpts holds the data for a signal social card.
type SignalCardOpts struct {
	Theme        string
	Symbol       string
	Direction    string // "LONG" | "SHORT"
	Price        float64
	StrategyName string
	Strength     float64
	Timestamp    string
}

// GenerateBacktestCard produces a 1200×630 PNG summarising a backtest.
func GenerateBacktestCard(opts BacktestCardOpts) ([]byte, error) {
	dc := gg.NewContext(cardWidth, cardHeight)

	bg, fg, accent, muted := themeColors(opts.Theme)
	dc.SetColor(bg)
	dc.Clear()

	// Brand text (top-left)
	dc.SetColor(accent)
	dc.DrawStringAnchored("trader-claude", 40, 40, 0, 1)

	// Strategy + symbol header
	dc.SetColor(fg)
	header := fmt.Sprintf("%s · %s · %s", opts.StrategyName, opts.Symbol, opts.Timeframe)
	dc.DrawStringAnchored(header, 40, 90, 0, 1)

	// Date range
	dc.SetColor(muted)
	dc.DrawStringAnchored(opts.DateRange, 40, 120, 0, 1)

	// Metric boxes (4 across)
	metrics := []struct {
		label string
		value string
		color color.Color
	}{
		{"Return", fmt.Sprintf("%+.1f%%", opts.TotalReturn*100), metricColor(opts.TotalReturn, accent)},
		{"Sharpe", fmt.Sprintf("%.2f", opts.SharpeRatio), accent},
		{"Max DD", fmt.Sprintf("%.1f%%", opts.MaxDrawdown*100), metricColor(opts.MaxDrawdown, accent)},
		{"Win Rate", fmt.Sprintf("%.0f%%", opts.WinRate*100), accent},
	}

	boxW := 240.0
	boxH := 80.0
	startX := 40.0
	startY := 170.0

	for i, m := range metrics {
		x := startX + float64(i)*(boxW+20)
		dc.SetColor(muted)
		dc.DrawRoundedRectangle(x, startY, boxW, boxH, 8)
		dc.Fill()

		dc.SetColor(m.color)
		dc.DrawStringAnchored(m.value, x+boxW/2, startY+25, 0.5, 0.5)
		dc.SetColor(fg)
		dc.DrawStringAnchored(m.label, x+boxW/2, startY+55, 0.5, 0.5)
	}

	// Equity sparkline
	if len(opts.EquityCurve) > 1 {
		drawSparkline(dc, opts.EquityCurve, 40, 300, float64(cardWidth)-80, 200, accent)
	}

	// Footer
	dc.SetColor(muted)
	dc.DrawStringAnchored("trader-claude · generated by AI", 40, float64(cardHeight)-30, 0, 0.5)

	return encodePNG(dc)
}

// GenerateSignalCard produces a 1200×630 PNG for a trading signal.
func GenerateSignalCard(opts SignalCardOpts) ([]byte, error) {
	dc := gg.NewContext(cardWidth, cardHeight)

	bg, fg, _, muted := themeColors(opts.Theme)
	dc.SetColor(bg)
	dc.Clear()

	// Direction color
	dirColor := color.RGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff} // green
	if opts.Direction == "SHORT" {
		dirColor = color.RGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff} // red
	}

	// Brand
	dc.SetColor(muted)
	dc.DrawStringAnchored("trader-claude", 40, 40, 0, 1)

	// Direction label (large)
	arrow := "▲"
	if opts.Direction == "SHORT" {
		arrow = "▼"
	}
	dc.SetColor(dirColor)
	dc.DrawStringAnchored(fmt.Sprintf("%s %s SIGNAL", arrow, opts.Direction), float64(cardWidth)/2, 180, 0.5, 0.5)

	// Symbol (large)
	dc.SetColor(fg)
	dc.DrawStringAnchored(opts.Symbol, float64(cardWidth)/2, 250, 0.5, 0.5)

	// Price
	dc.SetColor(dirColor)
	dc.DrawStringAnchored(fmt.Sprintf("$%.2f", opts.Price), float64(cardWidth)/2, 320, 0.5, 0.5)

	// Strategy + strength
	dc.SetColor(muted)
	dc.DrawStringAnchored(fmt.Sprintf("Strategy: %s", opts.StrategyName), float64(cardWidth)/2, 400, 0.5, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Strength: %.0f%%", opts.Strength*100), float64(cardWidth)/2, 440, 0.5, 0.5)

	// Timestamp
	dc.DrawStringAnchored(opts.Timestamp, float64(cardWidth)/2, 500, 0.5, 0.5)

	// Footer
	dc.DrawStringAnchored("trader-claude", float64(cardWidth)/2, float64(cardHeight)-30, 0.5, 0.5)

	return encodePNG(dc)
}

// --- helpers ---

func themeColors(theme string) (bg, fg, accent, muted color.Color) {
	if theme == "light" {
		return color.White,
			color.RGBA{R: 0x1f, G: 0x29, B: 0x37, A: 0xff},
			color.RGBA{R: 0x37, G: 0x99, B: 0xff, A: 0xff},
			color.RGBA{R: 0x9c, G: 0xa3, B: 0xaf, A: 0x44}
	}
	return color.RGBA{R: 0x0f, G: 0x17, B: 0x2a, A: 0xff},
		color.RGBA{R: 0xf3, G: 0xf4, B: 0xf6, A: 0xff},
		color.RGBA{R: 0x37, G: 0x99, B: 0xff, A: 0xff},
		color.RGBA{R: 0x37, G: 0x41, B: 0x51, A: 0xff}
}

func metricColor(value float64, defaultColor color.Color) color.Color {
	if value >= 0 {
		return color.RGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff}
	}
	return color.RGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff}
}

func drawSparkline(dc *gg.Context, values []float64, x, y, w, h float64, lineColor color.Color) {
	minV, maxV := values[0], values[0]
	for _, v := range values {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	rangeV := maxV - minV
	if rangeV == 0 {
		rangeV = 1
	}

	dc.SetColor(lineColor)
	dc.SetLineWidth(2)
	for i, v := range values {
		px := x + (float64(i)/float64(len(values)-1))*w
		py := y + h - ((v-minV)/rangeV)*h
		if i == 0 {
			dc.MoveTo(px, py)
		} else {
			dc.LineTo(px, py)
		}
	}
	dc.Stroke()
}

func encodePNG(dc *gg.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, fmt.Errorf("social: encode PNG: %w", err)
	}
	return buf.Bytes(), nil
}

// suppress unused import warning
var _ = math.Abs
```

---

**Step 3: Create `card_test.go`**

```go
package social

import (
	"image/png"
	"bytes"
	"testing"
)

func TestGenerateBacktestCard_DarkTheme(t *testing.T) {
	data, err := GenerateBacktestCard(BacktestCardOpts{
		Theme:        "dark",
		StrategyName: "EMA Crossover",
		Symbol:       "BTCUSDT",
		Timeframe:    "1h",
		DateRange:    "Jan 1 – Dec 31, 2024",
		TotalReturn:  0.342,
		SharpeRatio:  1.87,
		MaxDrawdown:  -0.142,
		WinRate:      0.58,
		EquityCurve:  []float64{10000, 10200, 10100, 10500, 10800, 11200, 11000, 11500, 12000, 13420},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200×630, got %d×%d", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateBacktestCard_LightTheme(t *testing.T) {
	data, err := GenerateBacktestCard(BacktestCardOpts{
		Theme:        "light",
		StrategyName: "RSI",
		Symbol:       "ETHUSDT",
		Timeframe:    "4h",
		DateRange:    "Jun 1 – Dec 31, 2025",
		TotalReturn:  -0.05,
		SharpeRatio:  0.45,
		MaxDrawdown:  -0.22,
		WinRate:      0.42,
		EquityCurve:  []float64{10000, 9800, 9600, 9700, 9500},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}
}

func TestGenerateSignalCard_Long(t *testing.T) {
	data, err := GenerateSignalCard(SignalCardOpts{
		Theme:        "dark",
		Symbol:       "BTCUSDT",
		Direction:    "LONG",
		Price:        82150.00,
		StrategyName: "EMA Crossover",
		Strength:     0.87,
		Timestamp:    "Feb 27, 2026 14:30 UTC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200×630, got %d×%d", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateSignalCard_Short(t *testing.T) {
	data, err := GenerateSignalCard(SignalCardOpts{
		Theme:     "dark",
		Symbol:    "ETHUSDT",
		Direction: "SHORT",
		Price:     2100.50,
		StrategyName: "RSI",
		Strength:  0.65,
		Timestamp: "Feb 27, 2026 16:00 UTC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty PNG data")
	}
}
```

---

**Step 4: Run tests**

```bash
docker compose exec backend go test ./internal/social/... -v
```

Expected: all 4 tests PASS.

---

**Step 5: Commit**

```bash
git add backend/internal/social/
git commit -m "feat(phase9): add social card generator (backtest + signal cards)"
```

---

### Task B5: Create api/social_handler.go + api/settings_handler.go

**Read first:**
- `backend/internal/api/alerts.go` (handler pattern)
- `backend/internal/models/models.go` (Setting + Backtest structs)
- `backend/internal/social/card.go` (function signatures)

**Files to create:**
- `backend/internal/api/social_handler.go`
- `backend/internal/api/settings_handler.go`

---

**Step 1: Create `social_handler.go`**

Endpoints:
- `POST /api/v1/social/backtest-card/:runId` → image/png
- `POST /api/v1/social/signal-card/:signalId` → image/png
- `POST /api/v1/social/send-telegram` → JSON

Handler struct uses `db *gorm.DB`.

Key logic:
- `backtestCard`: Load backtest by ID, extract metrics + equity curve, call `social.GenerateBacktestCard`, return `c.Type("image/png")` + raw bytes
- `signalCard`: Load MonitorSignal + Monitor by ID, call `social.GenerateSignalCard`, return PNG
- `sendTelegram`: Load Telegram settings from DB, create TelegramSender, send text or photo

---

**Step 2: Create `settings_handler.go`**

Endpoints:
- `GET /api/v1/settings/notifications` → JSON
- `POST /api/v1/settings/notifications` → JSON
- `POST /api/v1/settings/notifications/test` → JSON

Handler loads/saves Setting records grouped by `notifications.*` keys.

---

**Step 3: Wire routes in `routes.go`**

Add after the existing notification routes:

```go
// --- Social Cards ---
sh := newSocialHandler(db)
v1.Post("/social/backtest-card/:runId", sh.backtestCard)
v1.Post("/social/signal-card/:signalId", sh.signalCard)
v1.Post("/social/send-telegram", sh.sendTelegram)

// --- Settings ---
seth := newSettingsHandler(db)
v1.Get("/settings/notifications", seth.getNotificationSettings)
v1.Post("/settings/notifications", seth.saveNotificationSettings)
v1.Post("/settings/notifications/test", seth.testNotificationSettings)
```

---

**Step 4: Verify compile + run tests**

```bash
make backend-fmt
docker compose exec backend go build ./...
make backend-test
```

---

**Step 5: Commit**

```bash
git add backend/internal/api/social_handler.go backend/internal/api/settings_handler.go backend/internal/api/routes.go
git commit -m "feat(phase9): add social card API, settings CRUD, and Telegram send endpoint"
```

---

## FRONTEND TASKS

---

### Task F1: Add types + API client functions

**Files to modify/create:**
- `frontend/src/types/index.ts`
- `frontend/src/api/social.ts` (create)
- `frontend/src/api/settings.ts` (create)

Append `NotificationSettings`, `TelegramTestResult` types.
Create API client functions for social cards, Telegram send, and settings CRUD.

---

### Task F2: Create hooks/useSettings.ts

React Query hooks for:
- `useNotificationSettings()` — GET settings
- `useSaveNotificationSettings()` — POST mutation
- `useTestNotificationConnection()` — POST test mutation

---

### Task F3: Add Notifications section to Settings page

**Files to modify:**
- `frontend/src/pages/Settings.tsx`

Add a "Notifications" section with:
- Telegram: bot token input, chat ID input, enabled toggle, "Test" button
- Webhook: URL input, secret input, enabled toggle
- Save button

---

### Task F4: Create components/social/ShareModal.tsx

**Files to create:**
- `frontend/src/components/social/ShareModal.tsx`

Features:
- Dark/light theme toggle for card preview
- Card preview image (fetched from backend as blob)
- "Download PNG" button
- "Send to Telegram" button
- "Copy text" button (copies formatted summary to clipboard)

---

### Task F5: Wire ShareModal into Backtest results

**Files to modify:**
- `frontend/src/pages/Backtest.tsx`

Add "Share" button next to "Replay" button on completed backtest results.
Mount ShareModal when open.

---

**Final Verification:**

```bash
make backend-test
make frontend-lint
make frontend-test
```

---

## Summary

| Task | Files | Description |
|---|---|---|
| B1 | models.go, main.go | Setting model + autoMigrate |
| B2 | notification/telegram.go | Telegram Bot API sender + tests |
| B3 | notification/webhook.go | HMAC-SHA256 webhook sender + tests |
| B4 | social/card.go | PNG card generator (backtest + signal) + tests |
| B5 | api/social_handler.go, api/settings_handler.go | Social card API + settings CRUD |
| F1 | types/index.ts, api/social.ts, api/settings.ts | Types + API clients |
| F2 | hooks/useSettings.ts | React Query hooks |
| F3 | pages/Settings.tsx | Notifications config section |
| F4 | components/social/ShareModal.tsx | Share card modal |
| F5 | pages/Backtest.tsx | Wire Share button |
