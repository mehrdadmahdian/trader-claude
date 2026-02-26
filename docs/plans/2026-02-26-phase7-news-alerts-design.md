# Phase 7 — News, Events & Alerts: Design Document

**Date:** 2026-02-26
**Status:** Approved

---

## Overview

Phase 7 adds three capabilities:
1. A background news aggregator that ingests RSS feeds, deduplicates, tags symbols, and scores sentiment.
2. A price alert evaluator that runs every 60s, supports three condition types, and fires recurring notifications with a per-alert cooldown.
3. Frontend: a collapsible news panel on the Chart page with chart markers, a full Alerts management page, and a live notification bell in the TopBar.

---

## Decisions Made

| Topic | Decision |
|---|---|
| News sources | RSS only (gofeed) — CoinDesk, CoinTelegraph, Reuters markets RSS feeds |
| No paid APIs | NewsAPI / CryptoPanic deferred; NEWSAPI_KEY remains in .env.example for future |
| Alert conditions | price_above, price_below, price_change_pct only. RSI conditions deferred. |
| Alert behavior | Recurring with per-alert cooldown (CooldownMinutes field, default 60 min) |
| News sentiment | Simple keyword scoring (bullish/bearish word lists) — no external NLP |
| Background workers | Dedicated goroutines with time.Ticker (Option A) — started in main.go |

---

## Section 1: Backend — News Aggregator

### New DB Model: `NewsItem`

```go
// internal/models/models.go — append
type NewsItem struct {
    ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    URL         string    `gorm:"type:varchar(2048);uniqueIndex" json:"url"`
    Title       string    `gorm:"type:varchar(500)" json:"title"`
    Summary     string    `gorm:"type:text" json:"summary"`
    Source      string    `gorm:"type:varchar(100);index" json:"source"`
    PublishedAt time.Time `gorm:"index" json:"published_at"`
    Symbols     JSONArray `gorm:"type:json" json:"symbols"` // e.g. ["BTC", "ETH"]
    Sentiment   string    `gorm:"type:varchar(10)" json:"sentiment"` // positive|negative|neutral
    FetchedAt   time.Time `json:"fetched_at"`
    CreatedAt   time.Time `json:"created_at"`
}

func (NewsItem) TableName() string { return "news_items" }
```

### `internal/news/` package layout

```
internal/news/
  aggregator.go       Aggregator struct, Start(ctx), 15-min ticker loop
  sources.go          Hardcoded RSS feed list (URL, source name)
  sentiment.go        Keyword scorer → "positive" | "negative" | "neutral"
  tagger.go           Symbol tagger: scans title+summary for known tickers
  aggregator_test.go  Tests: dedup logic, symbol tagging, sentiment scoring
```

**Aggregator behavior:**
1. Load symbols table from DB on init (refresh every hour)
2. Every 15 min: fetch each RSS feed via `gofeed`
3. For each item: skip if URL already in DB, tag symbols, score sentiment, insert
4. Log counts: fetched / inserted / skipped

**RSS feeds (hardcoded initial list):**
- CoinDesk: `https://www.coindesk.com/arc/outboundfeeds/rss/`
- CoinTelegraph: `https://cointelegraph.com/rss`
- Reuters markets: `https://feeds.reuters.com/reuters/businessNews`

**Sentiment scoring:**
- Positive keywords: surge, rally, breakout, ath, bullish, soar, gain, rise, buy, upside, record
- Negative keywords: crash, dump, plunge, drop, bear, hack, fraud, collapse, sell, downside, loss
- Score = positive_hits - negative_hits; > 0 → "positive", < 0 → "negative", 0 → "neutral"

### News API endpoints (7.2)

```
GET /api/v1/news
    ?symbols=BTC,ETH   (comma-separated, optional)
    &limit=20          (default 20, max 100)
    &offset=0
    &from=ISO8601
    &to=ISO8601
    → PaginatedResponse<NewsItem>

GET /api/v1/news/symbols/:symbol
    → latest 50 items tagged with that symbol
```

Note: `include_news=true` on the candles endpoint is **deferred** — news markers are surfaced via the chart page side panel, not the candle data itself.

---

## Section 2: Backend — Alert Evaluator + Notification System

### Alert Model Changes

Two new fields added to the existing `Alert` model:

```go
CooldownMinutes int        `gorm:"default:60" json:"cooldown_minutes"`
LastFiredAt     *time.Time `json:"last_fired_at,omitempty"`
```

`volume_spike` and `custom` condition constants removed (were never implemented). Final condition set:
- `price_above`
- `price_below`
- `price_change_pct`

**Future note (deferred):** `rsi_overbought` and `rsi_oversold` conditions will be added in a future "Advanced Alerts" sub-phase after Phase 8 (which wires up the indicator engine to live feeds).

### `internal/alert/` package layout

```
internal/alert/
  evaluator.go       Evaluator struct, Start(ctx), 60s ticker loop
  evaluator_test.go  Tests: all 3 conditions, cooldown, notification creation
```

**Evaluator behavior:**
1. Every 60s: load all `active` alerts from DB
2. For each alert: fetch current price via `PriceService` (already exists from Phase 6)
3. Evaluate condition:
   - `price_above`: currentPrice > alert.Threshold
   - `price_below`: currentPrice < alert.Threshold
   - `price_change_pct`: |percentChange(price24hAgo, currentPrice)| ≥ alert.Threshold
4. Check cooldown: skip if `LastFiredAt != nil && time.Since(*LastFiredAt) < CooldownMinutes`
5. On fire:
   - Create `Notification{Type: "alert", Title: ..., Body: ...}`
   - Update `alert.LastFiredAt = now()`
   - Publish notification JSON to Redis pubsub `notifications:new`

### Alert API

```
POST   /api/v1/alerts               create alert (name, symbol, market, condition, threshold, cooldown_minutes)
GET    /api/v1/alerts               list all alerts
DELETE /api/v1/alerts/:id
PATCH  /api/v1/alerts/:id/toggle    toggle active/disabled status
```

### Notification API + WebSocket

```
GET   /api/v1/notifications          list, paginated, newest-first, default limit 50
PATCH /api/v1/notifications/:id/read mark single as read
POST  /api/v1/notifications/read-all mark all as read

WS    /ws/notifications              subscribe → streams Notification JSON on new alert fires
```

The `WS /ws/notifications` handler subscribes to Redis pubsub `notifications:new`. On receive, it unmarshals and forwards the notification JSON to all connected clients. Uses the existing pattern from `portfolio_ws.go`.

---

## Section 3: Frontend

### New TypeScript type

```ts
// types/index.ts — append
export interface NewsItem {
  id: number
  url: string
  title: string
  summary: string
  source: string
  published_at: string
  symbols: string[]
  sentiment: 'positive' | 'negative' | 'neutral'
}
```

Also update `Alert` interface to include `cooldown_minutes` and `last_fired_at`.

### 7.4 — Chart Page: News Side Panel

**Location:** Right side of chart page, collapsible (default: collapsed)

**Toggle:** "News" button added to chart toolbar (between indicators and the right edge)

**Panel layout (320px wide):**
```
┌────────────────────────────────────┐
│ Market News        [×]             │
├────────────────────────────────────│
│ [CoinDesk] BTC Hits New High  3h  🟢│
│ Bitcoin surges past $80k...        │
├────────────────────────────────────│
│ [Reuters] Fed Holds Rates     5h  ⚪│
│ Federal Reserve keeps rates...     │
└────────────────────────────────────┘
```
- Source badge: colored chip per source
- Headline: 2-line clamp
- Time: "Xh ago" (relative)
- Sentiment dot: 🟢 positive, 🔴 negative, ⚪ neutral
- Click item → `window.open(url, '_blank')`

**Chart markers:**
- After candles load, fetch news for the symbol via `GET /api/v1/news/symbols/:symbol`
- Map each item to a lightweight-charts marker: `{ time, position: 'belowBar', shape: 'arrowUp', color, text }`
- On marker hover → tooltip popover: headline + "Read more →" link
- Markers refresh when symbol or timeframe changes

**React Query hooks:** `useNews(symbol)`, `useNewsBySymbol(symbol)`

### 7.5 — Alerts Page

**Layout:** Full-page table with "Add Alert" FAB

**Table columns:** Name | Symbol | Condition | Threshold | Cooldown | Status | Last Fired | Actions

**"+ Add Alert" modal fields:**
- Adapter selector (reuse existing)
- Symbol input
- Condition: segmented `price_above | price_below | price_change_pct`
- Threshold: number input (shows "$" or "%" label based on condition)
- Cooldown minutes: number input (default 60)
- Name: auto-generated or custom

**Row actions:**
- Toggle button (green=active, gray=disabled)
- Delete (with confirm)

### 7.5 — Notification Bell (TopBar)

**Current state:** Bell icon is a placeholder in `TopBar.tsx`.

**Changes:**
- On mount: `useNotificationsWS` hook opens `WS /ws/notifications`
- On WS message: call `notificationStore.addNotification(n)`, animate bell (brief shake CSS animation)
- Badge: `unreadCount > 0` shows red badge (max "99+")
- Click bell → Popover dropdown:
  - Last 5 notifications (title, body, time ago, read/unread indicator)
  - Click item → marks read, navigates to source if applicable
  - "View all" → `/notifications`

### Notifications Page

- Paginated list, date-grouped
- "Mark all read" button
- Individual mark-read on click
- Uses `GET /api/v1/notifications` + `PATCH /api/v1/notifications/:id/read`

---

## Implementation Order

The sub-phases are independently executable:

```
7.1 (news aggregator backend) — no deps
7.3 (alert evaluator backend) — no deps (parallel with 7.1)
7.2 (news API)                — requires 7.1
7.4 (frontend news panel)     — requires 7.2
7.5 (frontend alerts + bell)  — requires 7.3
```

---

## Testing Requirements

| Sub-phase | Tests |
|---|---|
| 7.1 | Dedup by URL, symbol tagging, sentiment scoring |
| 7.2 | News API endpoint integration tests |
| 7.3 | All 3 condition types, cooldown logic, notification creation |
| 7.4 | News panel open/close, marker rendering |
| 7.5 | Alert CRUD, toggle, WS notification delivery, bell update |
