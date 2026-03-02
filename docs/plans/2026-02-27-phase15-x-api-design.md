# Phase 15 — X (Twitter) API Integration: Design Document

**Date:** 2026-02-27
**Status:** Draft

---

## Overview

Phase 15 integrates trader-claude with the **X (formerly Twitter) API v2** to enable two core capabilities:

1. **Posting**: Share backtest results, signal alerts, and custom messages directly to X as posts (tweets), optionally with social card images.
2. **Reading**: Fetch recent market-related posts from X for a given symbol/cashtag, display them in a dedicated feed panel alongside existing news, and optionally use sentiment signals.

This allows traders to share their insights publicly and monitor real-time social sentiment for their watched assets.

---

## Decisions Made

| Topic | Decision |
|---|---|
| API version | X API v2 (`https://api.x.com/2/`) — current official version |
| Auth method | OAuth 2.0 with PKCE for user-context posting; Bearer Token (App-Only) for reading |
| Go HTTP client | Standard `net/http` — no third-party X SDK to keep dependencies minimal |
| Post content | Text-only posts for v1; image upload (social cards) deferred to v1.1 (requires media upload endpoint) |
| Read scope | Recent search (`GET /2/tweets/search/recent`) — 7-day lookback, sufficient for real-time sentiment |
| Rate limiting | Client-side rate limiter respecting X API tiers (60 req/15min basic, 450 req/15min pro) |
| Credentials storage | X API tokens stored in `settings` table (from Phase 9) via keys `x.bearer_token`, `x.api_key`, `x.api_secret`, `x.access_token`, `x.access_token_secret` |
| Search queries | Cashtag-based: `$BTC`, `$ETH`, `$AAPL` — standard financial convention on X |
| Sentiment analysis | Basic positive/negative keyword scoring (no ML model — lightweight, extensible) |
| Feed refresh | Server-side polling every 5 minutes per active symbol, results cached in Redis (TTL 5 min) |
| Frontend display | New "Social" tab on Chart page, X feed panel alongside existing News panel |
| Post trigger | Manual "Post to X" button on backtest results and signal alerts (no auto-posting) |

---

## Section 1: X API Client Package

### File layout

```
internal/xapi/
  client.go           XClient struct: search, post, rate limiting
  client_test.go      Mock HTTP tests
  types.go            X API v2 response structs
  sentiment.go        Basic sentiment scorer
  sentiment_test.go   Sentiment scoring tests
```

### XClient

```go
type XClient struct {
    bearerToken string
    apiKey      string
    apiSecret   string
    accessToken string
    accessSecret string
    httpClient  *http.Client
    rateLimiter *rate.Limiter // golang.org/x/time/rate
}

func NewXClient(bearerToken string) *XClient

func NewXClientWithUserContext(apiKey, apiSecret, accessToken, accessSecret string) *XClient

// SearchRecent searches posts from the last 7 days.
// Uses Bearer Token (App-Only auth).
func (x *XClient) SearchRecent(ctx context.Context, query string, maxResults int) (*SearchResult, error)

// PostTweet creates a new post using user-context OAuth.
func (x *XClient) PostTweet(ctx context.Context, text string) (*PostResult, error)

// TestConnection verifies credentials by fetching the authenticated user.
func (x *XClient) TestConnection(ctx context.Context) (username string, err error)
```

### X API v2 types

```go
type Tweet struct {
    ID        string    `json:"id"`
    Text      string    `json:"text"`
    AuthorID  string    `json:"author_id"`
    CreatedAt time.Time `json:"created_at"`
    PublicMetrics struct {
        RetweetCount int `json:"retweet_count"`
        ReplyCount   int `json:"reply_count"`
        LikeCount    int `json:"like_count"`
        QuoteCount   int `json:"quote_count"`
    } `json:"public_metrics"`
}

type SearchResult struct {
    Data []Tweet `json:"data"`
    Meta struct {
        ResultCount int    `json:"result_count"`
        NextToken   string `json:"next_token"`
    } `json:"meta"`
}

type PostResult struct {
    Data struct {
        ID   string `json:"id"`
        Text string `json:"text"`
    } `json:"data"`
}
```

### Rate limiter

```go
// Respects X API rate limits: 60 requests per 15 minutes (basic tier)
func newRateLimiter(maxPer15Min int) *rate.Limiter {
    interval := 15 * time.Minute / time.Duration(maxPer15Min)
    return rate.NewLimiter(rate.Every(interval), 1)
}
```

---

## Section 2: Sentiment Scorer

### Basic keyword-based scoring

```go
type SentimentResult struct {
    Score     float64 `json:"score"`     // -1.0 to +1.0
    Positive  int     `json:"positive"`  // count of positive keywords
    Negative  int     `json:"negative"`  // count of negative keywords
    Label     string  `json:"label"`     // "bullish" | "bearish" | "neutral"
}

func ScoreSentiment(text string) SentimentResult
```

Keyword lists:
- **Bullish**: "buy", "bullish", "moon", "pump", "breakout", "long", "accumulate", "ATH", "surge", "rally"
- **Bearish**: "sell", "bearish", "dump", "crash", "short", "liquidation", "drop", "plunge", "decline", "correction"

Score = `(positive - negative) / max(positive + negative, 1)`, clamped to [-1, 1].

---

## Section 3: X Feed Service

### Background poller

```go
type FeedService struct {
    xClient *XClient
    rdb     *redis.Client
    db      *gorm.DB
}

func NewFeedService(xClient *XClient, rdb *redis.Client, db *gorm.DB) *FeedService

// FetchSymbolFeed searches X for a cashtag and caches results in Redis.
func (f *FeedService) FetchSymbolFeed(ctx context.Context, symbol string) ([]ScoredTweet, error)
```

Cache key: `x:feed:{symbol}` — TTL 5 minutes.

`ScoredTweet` = `Tweet` + `SentimentResult`.

### Symbol to cashtag mapping

```go
func symbolToCashtag(symbol string) string {
    // "BTC/USDT" → "$BTC"
    // "AAPL" → "$AAPL"
    parts := strings.Split(symbol, "/")
    return "$" + parts[0]
}
```

---

## Section 4: API Endpoints

### New file: `internal/api/x_handler.go`

```
GET  /api/v1/x/feed?symbol=BTC/USDT&limit=20    → { tweets: [ScoredTweet], sentiment_summary }
POST /api/v1/x/post                               → { tweet_id, text }
POST /api/v1/x/post/backtest/:runId               → { tweet_id, text }
POST /api/v1/x/post/signal/:signalId              → { tweet_id, text }
GET  /api/v1/settings/x                            → { bearer_token (masked), connected, username }
POST /api/v1/settings/x                            → { saved: true }
POST /api/v1/settings/x/test                       → { ok, username }
```

### Feed endpoint

```go
func (h *xHandler) getFeed(c *fiber.Ctx) error {
    symbol := c.Query("symbol")
    limit := c.QueryInt("limit", 20)

    tweets, err := h.feedSvc.FetchSymbolFeed(c.Context(), symbol)
    // ...
    
    // Compute aggregate sentiment
    var totalScore float64
    for _, t := range tweets {
        totalScore += t.Sentiment.Score
    }
    avgSentiment := totalScore / float64(len(tweets))

    return c.JSON(fiber.Map{
        "tweets": tweets[:min(len(tweets), limit)],
        "sentiment_summary": fiber.Map{
            "avg_score": avgSentiment,
            "label":     sentimentLabel(avgSentiment),
            "count":     len(tweets),
        },
    })
}
```

### Post backtest result

Auto-formats a backtest summary as a tweet:

```
📊 Backtest Results

Strategy: EMA Crossover
Symbol: $BTC · 1h
Period: Jan 1 – Dec 31, 2024

Return: +34.2%
Sharpe: 1.87
Max DD: -14.2%
Win Rate: 58%

Generated by trader-claude 🤖
```

### Post signal alert

```
🚨 LONG Signal — $BTC

Price: $82,150.00
Strategy: EMA Crossover
Strength: 87%

Feb 27, 2026 14:30 UTC
#trading #crypto
```

---

## Section 5: Frontend

### New TypeScript types

```ts
// types/index.ts
export interface XTweet {
  id: string
  text: string
  author_id: string
  created_at: string
  public_metrics: {
    retweet_count: number
    reply_count: number
    like_count: number
    quote_count: number
  }
  sentiment: {
    score: number
    positive: number
    negative: number
    label: 'bullish' | 'bearish' | 'neutral'
  }
}

export interface XFeedResponse {
  tweets: XTweet[]
  sentiment_summary: {
    avg_score: number
    label: string
    count: number
  }
}

export interface XSettings {
  bearer_token: string  // masked in GET response
  api_key: string
  api_secret: string
  access_token: string
  access_token_secret: string
  connected: boolean
  username?: string
}
```

### New API client

```ts
// api/x.ts
export const getXFeed = (symbol: string, limit?: number): Promise<XFeedResponse>
export const postToX = (text: string): Promise<{ tweet_id: string }>
export const postBacktestToX = (runId: number): Promise<{ tweet_id: string; text: string }>
export const postSignalToX = (signalId: number): Promise<{ tweet_id: string; text: string }>
export const getXSettings = (): Promise<XSettings>
export const saveXSettings = (settings: Partial<XSettings>): Promise<void>
export const testXConnection = (): Promise<{ ok: boolean; username: string }>
```

### Chart Page — Social Tab

Add a "Social" tab to the Chart page (alongside existing chart area):

```
┌──────────────────────────────────────────────┐
│ [Chart] [Indicators] [Social]                │
├──────────────────────────────────────────────┤
│                                              │
│  Sentiment: 🟢 Bullish (0.45)    23 posts   │
│  ─────────────────────────────               │
│                                              │
│  @trader_pro · 2h ago          🟢 bullish   │
│  $BTC looking strong, breakout above 80k    │
│  ♥ 42  🔄 12  💬 5                          │
│                                              │
│  @crypto_whale · 3h ago        🔴 bearish   │
│  Taking profits on $BTC here...              │
│  ♥ 18  🔄 3  💬 8                           │
│                                              │
│  ... more tweets ...                         │
│                                              │
│  [Refresh] [Post to X]                       │
└──────────────────────────────────────────────┘
```

### Backtest Share Integration

Extend the existing "Share" modal (Phase 9) with a "Post to X" option:

- Button: "Post to X" (bird icon)
- Preview the formatted text before posting
- Confirmation dialog
- Success toast with link to the tweet

### Settings Page — X Section

```
┌──────────────────────────────────────────────┐
│ X (Twitter) Integration                      │
├──────────────────────────────────────────────┤
│ Bearer Token: [••••••••••••••••] (for read) │
│ API Key:      [________________]             │
│ API Secret:   [________________]             │
│ Access Token: [________________]             │
│ Access Secret:[________________]             │
│                                              │
│ Status: ✅ Connected as @trader_bot          │
│                                              │
│ [Test Connection]   [Save]                   │
└──────────────────────────────────────────────┘
```

### New Files

```
frontend/src/api/x.ts                           API client
frontend/src/hooks/useXFeed.ts                   React Query hook for feed + polling
frontend/src/hooks/useXSettings.ts               React Query hooks for settings
frontend/src/components/social/XFeedPanel.tsx     Tweet feed component
frontend/src/components/social/TweetCard.tsx      Individual tweet card
frontend/src/components/social/SentimentBadge.tsx Sentiment indicator
frontend/src/components/social/PostToXModal.tsx   Confirm + post dialog
```

---

## Section 6: Environment Variables

Add to `.env.example`:

```env
# --- X (Twitter) API ---
X_BEARER_TOKEN=
X_API_KEY=
X_API_SECRET=
X_ACCESS_TOKEN=
X_ACCESS_TOKEN_SECRET=
```

These are optional — X integration is disabled if tokens are not configured. Users can also configure via the Settings page (stored in DB).

---

## Implementation Order

```
B1: internal/xapi/types.go (X API v2 response types)                   (no deps)
B2: internal/xapi/client.go (SearchRecent + PostTweet) + tests          (needs B1)
B3: internal/xapi/sentiment.go + tests                                  (no deps, parallel B2)
B4: X feed service with Redis caching                                   (needs B2, B3)
B5: api/x_handler.go (feed + post + settings endpoints)                (needs B4)
B6: Wire routes + main.go integration                                   (needs B5)
B7: Add post formatting (backtest summary, signal alert templates)      (needs B5)
F1: types/index.ts + api/x.ts                                          (no deps)
F2: hooks/useXFeed.ts + hooks/useXSettings.ts                          (needs F1)
F3: components/social/TweetCard.tsx + SentimentBadge.tsx                (needs F1)
F4: components/social/XFeedPanel.tsx                                    (needs F2, F3)
F5: Wire XFeedPanel into Chart page as "Social" tab                    (needs F4)
F6: components/social/PostToXModal.tsx                                  (needs F1)
F7: Wire PostToX into Share modal + signal actions                     (needs F6)
F8: Settings page X section                                             (needs F2)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B2 | SearchRecent: mock HTTP, parse response, handle empty results, rate limit wait |
| B2 | PostTweet: mock HTTP, verify OAuth header, handle 403 (rate limit) |
| B2 | TestConnection: mock HTTP, parse username from /2/users/me |
| B3 | Sentiment: "bullish moon pump" → positive, "crash dump sell" → negative, "bitcoin price" → neutral |
| B4 | Feed caching: first call hits API, second call hits Redis cache |
| B5 | Feed endpoint: returns tweets with sentiment scores |
| B5 | Post endpoint: requires auth, formats text correctly |
| B7 | Backtest format: includes strategy, symbol, metrics, hashtags |
| B7 | Signal format: includes direction, price, strategy, hashtags |
| F4 | XFeedPanel renders tweets, shows sentiment summary |
| F6 | PostToXModal shows preview, calls API on confirm |
