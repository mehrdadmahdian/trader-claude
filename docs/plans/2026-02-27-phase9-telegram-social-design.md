# Phase 9 — Telegram Bot & Social Card Generator: Design Document

**Date:** 2026-02-27
**Status:** Draft

---

## Overview

Phase 9 adds two outbound notification channels beyond in-app: a **Telegram bot** that sends text and image messages, and a **social card generator** that produces shareable PNG images summarising backtest results and live signals. A **webhook sender** enables arbitrary integrations. A **Settings page** lets users configure Telegram credentials and test the connection.

---

## Decisions Made

| Topic | Decision |
|---|---|
| Image generation library | `gg` (pure Go 2D graphics) — no CGo dependencies, Docker-friendly |
| Telegram integration | Direct Bot API via `net/http` — no third-party SDK; keeps dependencies minimal |
| Webhook security | HMAC-SHA256 signature in `X-TraderClaude-Signature` header |
| Webhook retry | 3 attempts, exponential backoff (1s, 2s, 4s) |
| Card themes | Dark and light; caller chooses via query param `?theme=dark\|light` |
| Card dimensions | 1200×630 px (OG image standard, fits Telegram/Twitter/Discord previews) |
| Settings storage | `settings` MySQL table (key-value JSON per user_id) |
| Signal card trigger | Generated on-demand via API, not auto-generated on every signal |
| Fonts | Embedded via `go:embed` — Inter (or system fallback) baked into the binary |
| PNG delivery | Returned as `image/png` response body (not stored in DB) |

---

## Section 1: New DB Model

### `settings` table

```go
// internal/models/models.go
type Setting struct {
    ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID    int64     `gorm:"uniqueIndex;default:1" json:"user_id"`
    Key       string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_user_key" json:"key"`
    Value     JSON      `gorm:"type:json" json:"value"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string { return "settings" }
```

Settings keys:
- `notifications.telegram.bot_token` — Telegram bot token string
- `notifications.telegram.chat_id` — Telegram chat ID string
- `notifications.telegram.enabled` — bool
- `notifications.webhook.url` — URL string
- `notifications.webhook.secret` — HMAC secret string
- `notifications.webhook.enabled` — bool

---

## Section 2: Backend — `internal/notification/` Package

### File layout

```
internal/notification/
  telegram.go       TelegramSender struct: SendText, SendPhoto
  telegram_test.go  Mock Bot API tests
  webhook.go        WebhookSender struct: Send with HMAC + retry
  webhook_test.go   Retry logic + signature tests
```

### TelegramSender

```go
type TelegramSender struct {
    botToken   string
    httpClient *http.Client
}

func NewTelegramSender(botToken string) *TelegramSender

func (t *TelegramSender) SendText(ctx context.Context, chatID, text string) error
func (t *TelegramSender) SendPhoto(ctx context.Context, chatID string, image []byte, caption string) error
func (t *TelegramSender) TestConnection(ctx context.Context) (botName string, err error)
```

`SendText` calls `https://api.telegram.org/bot{token}/sendMessage` with `chat_id` + `text` (Markdown parse mode).

`SendPhoto` calls `https://api.telegram.org/bot{token}/sendPhoto` with multipart form: `chat_id`, `photo` (file upload), `caption`.

`TestConnection` calls `getMe` to verify the token is valid, returns the bot username.

### WebhookSender

```go
type WebhookSender struct {
    httpClient *http.Client
}

func NewWebhookSender() *WebhookSender

func (w *WebhookSender) Send(ctx context.Context, url, secret string, payload []byte) error
```

Send:
1. Compute `HMAC-SHA256(secret, payload)` → hex digest
2. POST to `url` with `Content-Type: application/json`, `X-TraderClaude-Signature: sha256={digest}`
3. Retry up to 3 times with backoff (1s, 2s, 4s) on 5xx or timeout

---

## Section 3: Backend — `internal/social/` Package

### File layout

```
internal/social/
  card.go           GenerateBacktestCard, GenerateSignalCard
  card_test.go      Dimension + non-blank pixel tests
  fonts/            Embedded font files (Inter-Regular.ttf, Inter-Bold.ttf)
```

### GenerateBacktestCard

```go
func GenerateBacktestCard(opts BacktestCardOpts) ([]byte, error)

type BacktestCardOpts struct {
    Theme        string // "dark" | "light"
    StrategyName string
    Symbol       string
    Timeframe    string
    DateRange    string // "Jan 1 – Dec 31, 2024"
    TotalReturn  float64
    SharpeRatio  float64
    MaxDrawdown  float64
    WinRate      float64
    EquityCurve  []float64 // y-values for sparkline
}
```

Card layout (1200×630):
```
┌─────────────────────────────────────────────┐
│  StratosMarket                         🌙/☀ │
│                                             │
│  EMA Crossover · BTCUSDT · 1h              │
│  Jan 1 – Dec 31, 2024                      │
│                                             │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐      │
│  │+34.2%│ │ 1.87 │ │-14.2%│ │ 58%  │      │
│  │Return│ │Sharpe│ │MaxDD │ │WinRt │      │
│  └──────┘ └──────┘ └──────┘ └──────┘      │
│                                             │
│  ╱‾‾‾╲___╱‾‾‾‾‾╲  (equity sparkline)      │
│                                             │
│  trader-claude              generated at... │
└─────────────────────────────────────────────┘
```

### GenerateSignalCard

```go
func GenerateSignalCard(opts SignalCardOpts) ([]byte, error)

type SignalCardOpts struct {
    Theme        string
    Symbol       string
    Direction    string // "LONG" | "SHORT"
    Price        float64
    StrategyName string
    Strength     float64
    Timestamp    time.Time
}
```

Signal card layout (1200×630):
```
┌─────────────────────────────────────────────┐
│  StratosMarket                              │
│                                             │
│         ▲ LONG SIGNAL                       │
│         BTCUSDT                             │
│         $82,150.00                          │
│                                             │
│  Strategy: EMA Crossover                    │
│  Strength: 87%                              │
│  Feb 27, 2026 14:30 UTC                     │
│                                             │
│  trader-claude                              │
└─────────────────────────────────────────────┘
```

Colors: LONG = green (#22c55e), SHORT = red (#ef4444).

---

## Section 4: Social + Settings API

### New file: `internal/api/social_handler.go`

```
POST /api/v1/social/backtest-card/:runId?theme=dark  → image/png
POST /api/v1/social/signal-card/:signalId?theme=dark → image/png
POST /api/v1/social/send-telegram                    → { success: true }
```

### New file: `internal/api/settings_handler.go`

```
GET  /api/v1/settings/notifications          → { telegram: {...}, webhook: {...} }
POST /api/v1/settings/notifications          → { saved: true }
POST /api/v1/settings/notifications/test     → { telegram: { ok, bot_name }, webhook: { ok, status_code } }
```

`send-telegram` body:
```json
{
  "chat_id": "optional-override",
  "text": "optional text",
  "image_base64": "optional base64 PNG",
  "caption": "optional caption"
}
```

---

## Section 5: Frontend

### New TypeScript types (append to `types/index.ts`)

```ts
export interface NotificationSettings {
  telegram: {
    bot_token: string
    chat_id: string
    enabled: boolean
  }
  webhook: {
    url: string
    secret: string
    enabled: boolean
  }
}

export interface TelegramTestResult {
  ok: boolean
  bot_name?: string
  error?: string
}
```

### Settings Page "Notifications" Section

Add to `pages/Settings.tsx`:
- Telegram section: bot token input, chat ID input, enabled toggle, "Test Connection" button
- Webhook section: URL input, secret input, enabled toggle
- Save button (POST to settings API)

### Share Modal

New component: `components/social/ShareModal.tsx`
- Triggered from "Share" button on backtest results
- Dark/light theme toggle
- Card preview (fetched as image from backend)
- "Download PNG" button (saves to local disk)
- "Send to Telegram" button (calls send-telegram API)
- "Copy formatted text" button (copies summary to clipboard)

### New Files

```
frontend/src/api/social.ts          generateBacktestCard(), sendTelegram()
frontend/src/api/settings.ts        getNotificationSettings(), saveNotificationSettings(), testConnection()
frontend/src/components/social/ShareModal.tsx
frontend/src/hooks/useSettings.ts   React Query hooks
```

---

## Implementation Order

```
B1: Add Setting model → autoMigrate                           (no deps)
B2: internal/notification/telegram.go + tests                  (no deps)
B3: internal/notification/webhook.go + tests                   (no deps, parallel with B2)
B4: internal/social/card.go + GenerateBacktestCard + tests     (no deps, parallel with B2-B3)
B5: internal/social/card.go + GenerateSignalCard + tests       (needs B4)
B6: api/social_handler.go (card endpoints)                     (needs B4, B5)
B7: api/settings_handler.go (settings CRUD + test endpoint)    (needs B1, B2, B3)
B8: Wire routes + main.go                                      (needs B6, B7)
F1: types/index.ts + api/social.ts + api/settings.ts           (no deps)
F2: hooks/useSettings.ts                                       (needs F1)
F3: Settings page Notifications section                        (needs F2)
F4: components/social/ShareModal.tsx                            (needs F1)
F5: Wire ShareModal into Backtest results page                 (needs F4)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B2 | Mock Telegram Bot API: SendText, SendPhoto, TestConnection |
| B3 | Webhook: HMAC signature verification, retry on 500, success on 200 |
| B4 | Backtest card: output is PNG, dimensions 1200×630, non-blank pixels |
| B5 | Signal card: output is PNG, dimensions 1200×630, LONG=green/SHORT=red text |
| B7 | Settings CRUD: save/load/test-connection endpoints |
| F3 | Settings form renders, save calls API, test button shows result |
| F4 | ShareModal renders card preview, download triggers blob save |
