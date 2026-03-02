# Phase 15 — X (Twitter) API Integration Design

**Date:** 2026-03-02
**Status:** Approved

---

## Overview

Add X (Twitter) social feed and posting capability to the platform. Users can:
- View live sentiment-scored tweets for any symbol on the Chart page
- Post backtest summaries and monitor signals to X
- Test their X API credentials from the Settings page

---

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Credential storage | Environment variables | Consistent with existing DB/Redis/JWT pattern; no sensitive data in DB |
| Mock/real split | Single struct with `mockMode bool` | Simpler than interface split; mock activated automatically when `X_BEARER_TOKEN` absent |
| Rate limiting | In-process token bucket (struct fields + mutex) | No Redis overhead for a non-critical 60/15min limit |
| Feed caching | Redis `x:feed:{cashtag}` with 5-min TTL | Prevents hammering X API; matches pattern in price/news packages |

---

## Backend

### Package: `internal/xapi/`

**`types.go`**
```go
type Tweet struct {
    ID        string    `json:"id"`
    Text      string    `json:"text"`
    AuthorID  string    `json:"author_id"`
    Username  string    `json:"username"`
    CreatedAt time.Time `json:"created_at"`
    Likes     int       `json:"like_count"`
    Retweets  int       `json:"retweet_count"`
}

type ScoredTweet struct {
    Tweet
    Sentiment float64 `json:"sentiment"` // [-1, 1]
    Label     string  `json:"label"`     // "bullish" | "bearish" | "neutral"
}

type SentimentSummary struct {
    BullishPct  float64 `json:"bullish_pct"`
    BearishPct  float64 `json:"bearish_pct"`
    NeutralPct  float64 `json:"neutral_pct"`
    AvgScore    float64 `json:"avg_score"`
    TweetCount  int     `json:"tweet_count"`
}

type SearchResponse struct {
    Tweets  []ScoredTweet    `json:"tweets"`
    Summary SentimentSummary `json:"summary"`
}

type PostResponse struct {
    TweetID string `json:"tweet_id"`
    URL     string `json:"url"`
}
```

**`client.go`**

`XClient` struct fields:
- `mockMode bool` — true when `X_BEARER_TOKEN` is empty
- `bearerToken string`
- `apiKey, apiSecret, accessToken, accessSecret string`
- Rate limiter: `reqCount int`, `windowStart time.Time`, `mu sync.Mutex`

Methods:
- `SearchRecent(cashtag string, limit int) ([]Tweet, error)` — GET `https://api.twitter.com/2/tweets/search/recent?query={cashtag}&max_results={limit}&tweet.fields=created_at,public_metrics,author_id&expansions=author_id&user.fields=username`
- `PostTweet(text string) (PostResponse, error)` — POST `https://api.twitter.com/2/tweets` with OAuth 1.0a user context
- `TestConnection() (username string, err error)` — GET `https://api.twitter.com/2/users/me`
- `MockMode() bool`

Mock mode behavior:
- `SearchRecent` returns 8 fixture tweets with varied sentiment (mix of BTC/ETH references, some bullish, some bearish)
- `PostTweet` returns a fake tweet ID `mock-tweet-{timestamp}` and URL
- `TestConnection` returns username `"mock_user"`, no error

Rate limiter: 60 requests per 15-minute window. Enforced per `SearchRecent` call. Returns `ErrRateLimited` when exceeded.

**`sentiment.go`**

```go
var bullishKeywords = []string{
    "moon", "pump", "ath", "bullish", "buy", "long", "breakout",
    "rally", "bull", "surge", "gain", "upside", "green", "hodl",
    "accumulate", "support", "launch", "soar",
}

var bearishKeywords = []string{
    "crash", "dump", "bear", "short", "sell", "drop", "collapse",
    "rekt", "dip", "downside", "red", "fear", "panic", "bottom",
    "resistance", "rug", "bearish",
}

func Score(text string) float64
func Label(score float64) string // "bullish" | "bearish" | "neutral"
```

Algorithm: tokenize to lowercase words, count hits in each list. `score = (bullishHits - bearishHits) / max(totalTokens, 1)`, clamped to `[-1, 1]`.

**`feed.go`**

`FeedService` wraps `XClient` and Redis. `GetFeed(symbol string, limit int) (*SearchResponse, error)`:
1. Convert `symbol` to cashtag: strip `/`, `/USDT` etc → `BTC/USDT` → `$BTC`
2. Check Redis `x:feed:{cashtag}` — return cached if hit
3. Call `client.SearchRecent`, score each tweet, compute summary
4. Cache result as JSON, 5-min TTL
5. Return

### Config additions (`config.go`)

```go
XBearerToken      string // X_BEARER_TOKEN
XAPIKey           string // X_API_KEY
XAPISecret        string // X_API_SECRET
XAccessToken      string // X_ACCESS_TOKEN
XAccessSecret     string // X_ACCESS_TOKEN_SECRET
```

### API Endpoints (all protected, under `/api/v1/`)

| Method | Path | Description |
|---|---|---|
| GET | `/x/feed` | `?symbol=&limit=` → `{tweets, summary}` |
| POST | `/x/post` | `{text}` → `{tweet_id, url}` |
| POST | `/x/post/backtest/:runId` | Formatted backtest summary post |
| POST | `/x/post/signal/:signalId` | Formatted signal alert post |
| GET | `/settings/x` | `{connected, mock_mode, username?}` |
| POST | `/settings/x/test` | `{ok, username?, error?}` |

**Formatted post templates:**

Backtest summary:
```
📊 {Strategy} on {Symbol} ({Timeframe})
Return: {TotalReturn}% | Sharpe: {Sharpe}
Win rate: {WinRate}% | {TotalTrades} trades
Generated with StratosMarket
```

Signal:
```
🚨 {BUY/SELL} Signal — {Symbol}
Price: ${Price} | Strategy: {Strategy}
{Timestamp}
Generated with StratosMarket
```

### Tests

- `sentiment_test.go`: bullish text, bearish text, mixed, empty, edge cases
- `client_test.go`: mock mode search, mock mode post, real mode rate limiter, `TestConnection` mock
- `feed_test.go`: cashtag conversion, cache hit skips client call, cache miss populates

---

## Frontend

### New: `frontend/src/api/x.ts`

Thin axios wrappers: `getXFeed(symbol, limit)`, `postToX(text)`, `postBacktestToX(runId)`, `postSignalToX(signalId)`, `getXSettings()`, `testXConnection()`.

### New: `frontend/src/hooks/useXFeed.ts`

```ts
export function useXFeed(symbol: string | null)   // refetchInterval: 5 * 60 * 1000
export function usePostToX()
export function useTestXConnection()
export function useXSettings()
```

### New: `frontend/src/components/x/`

| Component | Description |
|---|---|
| `SentimentBadge.tsx` | Colored chip: bullish=green, bearish=red, neutral=muted |
| `TweetCard.tsx` | Avatar placeholder, @username, time-ago, text, like/RT counts, SentimentBadge |
| `XFeedPanel.tsx` | Header with summary bar (bull%/bear%/neutral% + avg score indicator), tweet list, refresh button, "Mock mode" badge |
| `PostToXModal.tsx` | Pre-filled textarea (editable), char counter /280, post button, success state with link |

### Modified pages

**`Chart.tsx`** — Add "Social" tab (alongside existing indicator/news tabs). Renders `<XFeedPanel symbol={activeSymbol} />`.

**`Backtest.tsx`** — Add "Post to X" button in results header alongside existing "Share" button. Opens `<PostToXModal>` pre-filled with formatted backtest summary.

**`Monitor.tsx`** — Add "Post Signal to X" in signal card action menu. Opens `<PostToXModal>` pre-filled with signal text.

**`Settings.tsx`** — Add "X (Twitter)" section:
- Displays 5 env var names as read-only labels with current "configured / not set" status
- "Test Connection" button → shows `{ok, username}` or error
- Mock mode indicator when credentials absent

### Types additions (`types/index.ts`)

```ts
export interface Tweet { id, text, username, created_at, like_count, retweet_count }
export interface ScoredTweet extends Tweet { sentiment: number; label: 'bullish' | 'bearish' | 'neutral' }
export interface SentimentSummary { bullish_pct, bearish_pct, neutral_pct, avg_score, tweet_count }
export interface XFeedResponse { tweets: ScoredTweet[]; summary: SentimentSummary }
export interface XSettings { connected: boolean; mock_mode: boolean; username?: string }
```

---

## Sub-phase Order

```
15.1 (client + types)  ─┐
15.2 (sentiment)        ─┼→ 15.3 (feed + API) → 15.4 (Social tab + feed panel)
                                                → 15.5 (Post to X modal)
                                                → 15.6 (Settings section)
```

15.1 and 15.2 are independent and can be built in parallel.

---

## Environment Variables

```bash
# .env.example additions
X_BEARER_TOKEN=         # Required for reading tweets
X_API_KEY=              # Required for posting
X_API_SECRET=           # Required for posting
X_ACCESS_TOKEN=         # Required for posting
X_ACCESS_TOKEN_SECRET=  # Required for posting
```

When all 5 are absent → mock mode active, all endpoints work with fixture data.
