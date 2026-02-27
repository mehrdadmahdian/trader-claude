# Phase 7 — News, Events & Alerts — Atomic Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add RSS news aggregator, price alert evaluator, and full frontend UI for news panel, alerts page, and notification bell.

**Architecture:** Background goroutines (news aggregator every 15 min, alert evaluator every 60 s) persist to MySQL; WebSocket at `/ws/notifications` pushes real-time alerts to the browser; frontend adds a collapsible news side panel on Chart, a full Alerts page, and a TopBar notification dropdown.

**Tech Stack:** Go gofeed (RSS), price.Service (already exists), Radix DropdownMenu (already in package.json), React Query, Zustand stores (already set up), lucide-react icons, date-fns (already in package.json).

**Execution strategy:** Each task = one focused action on ≤3 files. Give Haiku the task text + only the files listed under "Read first". No codebase exploration needed.

---

## BACKEND TASKS

---

### Task B1: Update Alert model + add NewsItem model

**Read first:**
- `backend/internal/models/models.go` (full file — 374 lines)

**Files to modify:**
- `backend/internal/models/models.go`
- `backend/cmd/server/main.go`

**Step 1: Add `BasePrice`, `AdapterID`, `RecurringEnabled`, `CooldownMinutes`, `LastFiredAt` fields to the existing `Alert` struct**

Find this block in `models.go` (lines 304–318):
```go
// Alert stores price/volume/custom alert definitions
type Alert struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"type:varchar(200);not null" json:"name"`
	Symbol      string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Market      string         `gorm:"type:varchar(20);not null" json:"market"`
	Condition   AlertCondition `gorm:"type:varchar(30);not null" json:"condition"`
	Threshold   float64        `gorm:"type:decimal(20,8);not null" json:"threshold"`
	Status      AlertStatus    `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	Message     string         `gorm:"type:text" json:"message"`
	TriggeredAt *time.Time     `json:"triggered_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
```

Replace it with:
```go
// Alert stores price/volume/custom alert definitions
type Alert struct {
	ID               int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string         `gorm:"type:varchar(200);not null" json:"name"`
	AdapterID        string         `gorm:"type:varchar(20);not null;default:'binance'" json:"adapter_id"`
	Symbol           string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Market           string         `gorm:"type:varchar(20);not null" json:"market"`
	Condition        AlertCondition `gorm:"type:varchar(30);not null" json:"condition"`
	Threshold        float64        `gorm:"type:decimal(20,8);not null" json:"threshold"`
	BasePrice        float64        `gorm:"type:decimal(20,8);default:0" json:"base_price"`
	Status           AlertStatus    `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	Message          string         `gorm:"type:text" json:"message"`
	RecurringEnabled bool           `gorm:"default:true" json:"recurring_enabled"`
	CooldownMinutes  int            `gorm:"default:60" json:"cooldown_minutes"`
	LastFiredAt      *time.Time     `json:"last_fired_at,omitempty"`
	TriggeredAt      *time.Time     `json:"triggered_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}
```

**Step 2: Add the NewsItem struct** — insert this block after `func (Alert) TableName() string { return "alerts" }` and before `// --- Notification ---`:

```go
// --- NewsItem ---

// NewsItem stores a de-duplicated RSS article
type NewsItem struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	URL         string    `gorm:"type:varchar(2048);uniqueIndex" json:"url"`
	Title       string    `gorm:"type:varchar(512);not null" json:"title"`
	Summary     string    `gorm:"type:text" json:"summary"`
	Source      string    `gorm:"type:varchar(64);not null;index" json:"source"`
	PublishedAt time.Time `gorm:"index" json:"published_at"`
	Symbols     JSONArray `gorm:"type:json" json:"symbols"`
	Sentiment   float64   `gorm:"type:decimal(4,3)" json:"sentiment"`
	FetchedAt   time.Time `json:"fetched_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (NewsItem) TableName() string { return "news_items" }
```

**Step 3: Add `&models.NewsItem{}` to `autoMigrate` in `backend/cmd/server/main.go`**

Find the `autoMigrate` function (lines 182–197). Add `&models.NewsItem{}` after `&models.Notification{}`:

```go
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Symbol{},
		&models.Candle{},
		&models.StrategyDef{},
		&models.Backtest{},
		&models.Trade{},
		&models.Portfolio{},
		&models.Position{},
		&models.Transaction{},
		&models.Alert{},
		&models.Notification{},
		&models.NewsItem{},
		&models.WatchList{},
		&models.ReplayBookmark{},
	)
}
```

**Step 4: Verify**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go build ./...
```
Expected: no output (clean build).

**Step 5: Commit**
```bash
git add backend/internal/models/models.go backend/cmd/server/main.go
git commit -m "feat(phase7): add NewsItem model and extend Alert with recurring/cooldown fields"
```

---

### Task B2: Install gofeed dependency

**Read first:** `backend/go.mod` (to confirm current deps)

**Files modified:** `backend/go.mod`, `backend/go.sum`

**Step 1: Install**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go get github.com/mmcdole/gofeed@latest
go mod tidy
```

**Step 2: Verify it appears in go.mod**
```bash
grep gofeed go.mod
```
Expected output contains: `github.com/mmcdole/gofeed`

**Step 3: Commit**
```bash
git add go.mod go.sum
git commit -m "chore(phase7): add gofeed RSS parser dependency"
```

---

### Task B3: Create news/sentiment.go + test

**Read first:** nothing (pure logic, no deps on project code)

**Files to create:**
- `backend/internal/news/sentiment.go`
- `backend/internal/news/sentiment_test.go`

**Step 1: Create `backend/internal/news/sentiment.go`**

```go
package news

import "strings"

var positiveWords = []string{
	"surge", "rally", "soar", "gain", "gains", "rise", "rises", "bull", "bullish",
	"breakout", "adoption", "partnership", "approval", "upgrade", "growth",
	"profit", "high", "record", "milestone", "launch", "support", "recover",
	"recovery", "upside", "outperform", "buy", "accumulate",
}

var negativeWords = []string{
	"crash", "drop", "drops", "fall", "falls", "plunge", "plunges", "bear", "bearish",
	"hack", "ban", "banned", "regulation", "fine", "lawsuit", "sell-off", "selloff",
	"loss", "losses", "low", "decline", "declining", "warning", "risk", "scam",
	"fraud", "bankrupt", "bankruptcy", "collapse", "collapses", "dump", "dumps",
}

// Score returns a sentiment score in the range [-1, 1].
// +1 = fully positive, -1 = fully negative, 0 = neutral.
// It counts keyword hits in the lower-cased text and returns
// (positive - negative) / (positive + negative). Returns 0 when
// no keywords are found.
func Score(text string) float64 {
	lower := strings.ToLower(text)
	pos, neg := 0, 0
	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			pos++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			neg++
		}
	}
	total := pos + neg
	if total == 0 {
		return 0
	}
	return float64(pos-neg) / float64(total)
}
```

**Step 2: Create `backend/internal/news/sentiment_test.go`**

```go
package news

import "testing"

func TestScore_Positive(t *testing.T) {
	score := Score("Bitcoin surges to a record high milestone rally")
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}
}

func TestScore_Negative(t *testing.T) {
	score := Score("Crypto market crash as prices plunge and drop amid selloff")
	if score >= 0 {
		t.Errorf("expected negative score, got %f", score)
	}
}

func TestScore_Neutral(t *testing.T) {
	score := Score("The market opened today for trading")
	if score != 0 {
		t.Errorf("expected zero score, got %f", score)
	}
}

func TestScore_Range(t *testing.T) {
	texts := []string{
		"Bull market rally surge record high milestone",
		"crash plunge fall drop bear selloff bankruptcy",
		"",
		"news article",
	}
	for _, text := range texts {
		s := Score(text)
		if s < -1 || s > 1 {
			t.Errorf("Score(%q) = %f out of [-1,1]", text, s)
		}
	}
}
```

**Step 3: Run test**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go test ./internal/news/ -run TestScore -v
```
Expected: all 4 tests PASS.

**Step 4: Commit**
```bash
git add backend/internal/news/
git commit -m "feat(phase7): add news sentiment scorer with keyword-based scoring"
```

---

### Task B4: Create news/tagger.go + test

**Read first:** nothing (pure logic)

**Files to create:**
- `backend/internal/news/tagger.go`
- `backend/internal/news/tagger_test.go`

**Step 1: Create `backend/internal/news/tagger.go`**

```go
package news

import "strings"

// symbolEntry maps a canonical symbol to text aliases to scan for.
type symbolEntry struct {
	symbol  string
	aliases []string
}

// knownSymbols is the lookup table used for symbol tagging.
// Aliases are all lower-case; scanning is case-insensitive.
var knownSymbols = []symbolEntry{
	{symbol: "BTCUSDT", aliases: []string{"bitcoin", "btc"}},
	{symbol: "ETHUSDT", aliases: []string{"ethereum", "eth"}},
	{symbol: "SOLUSDT", aliases: []string{"solana", "sol"}},
	{symbol: "BNBUSDT", aliases: []string{"binance coin", "bnb"}},
	{symbol: "XRPUSDT", aliases: []string{"ripple", "xrp"}},
	{symbol: "ADAUSDT", aliases: []string{"cardano", "ada"}},
	{symbol: "DOGEUSDT", aliases: []string{"dogecoin", "doge"}},
	{symbol: "AVAXUSDT", aliases: []string{"avalanche", "avax"}},
	{symbol: "DOTUSDT", aliases: []string{"polkadot", "dot"}},
	{symbol: "MATICUSDT", aliases: []string{"polygon", "matic"}},
	{symbol: "AAPL", aliases: []string{"apple inc", "aapl"}},
	{symbol: "MSFT", aliases: []string{"microsoft", "msft"}},
	{symbol: "SPY", aliases: []string{"s&p 500", "s&p500", "spy etf"}},
	{symbol: "TSLA", aliases: []string{"tesla", "tsla"}},
	{symbol: "NVDA", aliases: []string{"nvidia", "nvda"}},
	{symbol: "GOOGL", aliases: []string{"google", "alphabet", "googl"}},
	{symbol: "AMZN", aliases: []string{"amazon", "amzn"}},
	{symbol: "META", aliases: []string{"meta platforms", "facebook"}},
}

// ExtractSymbols scans text for known tickers and aliases and returns
// a de-duplicated slice of canonical symbol strings (e.g. "BTCUSDT").
// Matching is case-insensitive. Returns nil if no symbols are found.
func ExtractSymbols(text string) []string {
	lower := strings.ToLower(text)
	seen := make(map[string]bool)
	var result []string

	for _, entry := range knownSymbols {
		if seen[entry.symbol] {
			continue
		}
		for _, alias := range entry.aliases {
			if strings.Contains(lower, alias) {
				seen[entry.symbol] = true
				result = append(result, entry.symbol)
				break
			}
		}
	}
	return result
}
```

**Step 2: Create `backend/internal/news/tagger_test.go`**

```go
package news

import "testing"

func TestExtractSymbols_Crypto(t *testing.T) {
	text := "Bitcoin and Ethereum continue to rally as BTC breaks 100k"
	syms := ExtractSymbols(text)
	want := map[string]bool{"BTCUSDT": true, "ETHUSDT": true}
	for _, s := range syms {
		delete(want, s)
	}
	if len(want) > 0 {
		t.Errorf("missing symbols: %v (got %v)", want, syms)
	}
}

func TestExtractSymbols_Stock(t *testing.T) {
	syms := ExtractSymbols("Apple reports record earnings beating Microsoft")
	found := map[string]bool{}
	for _, s := range syms {
		found[s] = true
	}
	if !found["AAPL"] {
		t.Error("expected AAPL")
	}
	if !found["MSFT"] {
		t.Error("expected MSFT")
	}
}

func TestExtractSymbols_NoMatch(t *testing.T) {
	syms := ExtractSymbols("The weather is nice today")
	if len(syms) != 0 {
		t.Errorf("expected no symbols, got %v", syms)
	}
}

func TestExtractSymbols_Dedup(t *testing.T) {
	// "bitcoin" and "btc" both map to BTCUSDT — should appear once
	syms := ExtractSymbols("Bitcoin (BTC) is the largest cryptocurrency")
	count := 0
	for _, s := range syms {
		if s == "BTCUSDT" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected BTCUSDT exactly once, got %d", count)
	}
}
```

**Step 3: Run tests**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go test ./internal/news/ -run TestExtractSymbols -v
```
Expected: 4 tests PASS.

**Step 4: Commit**
```bash
git add backend/internal/news/tagger.go backend/internal/news/tagger_test.go
git commit -m "feat(phase7): add symbol tagger for news articles"
```

---

### Task B5: Create news/sources.go

**Read first:** nothing

**Files to create:**
- `backend/internal/news/sources.go`

**Step 1: Create `backend/internal/news/sources.go`**

```go
package news

import (
	"context"
	"time"

	"github.com/mmcdole/gofeed"
)

// Feed defines a single RSS news source.
type Feed struct {
	Name string
	URL  string
}

// DefaultFeeds is the list of RSS feeds the aggregator polls.
var DefaultFeeds = []Feed{
	{Name: "CoinDesk", URL: "https://www.coindesk.com/arc/outboundfeeds/rss/"},
	{Name: "CoinTelegraph", URL: "https://cointelegraph.com/rss"},
	{Name: "Reuters Business", URL: "https://feeds.reuters.com/reuters/businessNews"},
}

// FeedItem is a normalized article from any RSS feed.
type FeedItem struct {
	URL         string
	Title       string
	Summary     string
	Source      string
	PublishedAt time.Time
}

// fetchFeed downloads and parses a single RSS feed.
// Returns an empty slice on any error so the caller can continue
// processing other feeds without interruption.
func fetchFeed(ctx context.Context, feed Feed) []FeedItem {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return nil
	}

	items := make([]FeedItem, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		if item.Link == "" || item.Title == "" {
			continue
		}
		pub := time.Now()
		if item.PublishedParsed != nil {
			pub = *item.PublishedParsed
		}
		summary := item.Description
		if summary == "" {
			summary = item.Content
		}
		items = append(items, FeedItem{
			URL:         item.Link,
			Title:       item.Title,
			Summary:     summary,
			Source:      feed.Name,
			PublishedAt: pub,
		})
	}
	return items
}
```

**Step 2: Verify build**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go build ./internal/news/
```
Expected: no output.

**Step 3: Commit**
```bash
git add backend/internal/news/sources.go
git commit -m "feat(phase7): add RSS feed definitions and fetchFeed function"
```

---

### Task B6: Create news/aggregator.go + aggregator_test.go

**Read first:**
- `backend/internal/models/models.go` lines 1–10 (for package + imports reference), lines 304–330 (NewsItem struct)

**Files to create:**
- `backend/internal/news/aggregator.go`
- `backend/internal/news/aggregator_test.go`

**Step 1: Create `backend/internal/news/aggregator.go`**

```go
package news

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/trader-claude/backend/internal/models"
)

const fetchInterval = 15 * time.Minute

// Aggregator runs a background goroutine that fetches RSS feeds every 15 minutes,
// de-duplicates by URL via the database UNIQUE index, tags symbols, scores sentiment,
// and persists new articles to the news_items table.
type Aggregator struct {
	db    *gorm.DB
	feeds []Feed
}

// NewAggregator creates an Aggregator. Pass DefaultFeeds for production.
func NewAggregator(db *gorm.DB, feeds []Feed) *Aggregator {
	return &Aggregator{db: db, feeds: feeds}
}

// Start launches the fetch loop in a background goroutine.
// It fetches once immediately, then repeats every 15 minutes.
// Cancel ctx to stop the loop.
func (a *Aggregator) Start(ctx context.Context) {
	go func() {
		a.fetchAndStore(ctx)
		ticker := time.NewTicker(fetchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.fetchAndStore(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// fetchAndStore fetches every feed and upserts new articles.
// Duplicate URLs are silently ignored via INSERT ... ON CONFLICT DO NOTHING.
func (a *Aggregator) fetchAndStore(ctx context.Context) {
	for _, feed := range a.feeds {
		items := fetchFeed(ctx, feed)
		for _, item := range items {
			symbols := ExtractSymbols(item.Title + " " + item.Summary)
			symbolsArr := make(models.JSONArray, len(symbols))
			for i, s := range symbols {
				symbolsArr[i] = s
			}

			record := models.NewsItem{
				URL:         item.URL,
				Title:       item.Title,
				Summary:     item.Summary,
				Source:      item.Source,
				PublishedAt: item.PublishedAt,
				Symbols:     symbolsArr,
				Sentiment:   Score(item.Title + " " + item.Summary),
				FetchedAt:   time.Now(),
			}

			if err := a.db.WithContext(ctx).
				Clauses(clause.OnConflict{DoNothing: true}).
				Create(&record).Error; err != nil {
				log.Printf("news aggregator: failed to save %q: %v", item.URL, err)
			}
		}
	}
}
```

**Step 2: Create `backend/internal/news/aggregator_test.go`**

```go
package news

import (
	"testing"
)

// TestAggregatorPureFunctions verifies the pure-function pipeline used inside fetchAndStore.
// DB persistence is tested by the database UNIQUE constraint (integration concern).
func TestAggregatorPureFunctions(t *testing.T) {
	title := "Bitcoin surges as BTC rally continues to record highs"
	summary := "Ethereum also rising amid market recovery"

	combined := title + " " + summary

	syms := ExtractSymbols(combined)
	foundBTC, foundETH := false, false
	for _, s := range syms {
		if s == "BTCUSDT" {
			foundBTC = true
		}
		if s == "ETHUSDT" {
			foundETH = true
		}
	}
	if !foundBTC {
		t.Error("expected BTCUSDT to be tagged")
	}
	if !foundETH {
		t.Error("expected ETHUSDT to be tagged")
	}

	score := Score(combined)
	if score <= 0 {
		t.Errorf("expected positive sentiment score, got %f", score)
	}
}

func TestNewAggregator(t *testing.T) {
	agg := NewAggregator(nil, DefaultFeeds)
	if agg == nil {
		t.Fatal("NewAggregator returned nil")
	}
	if len(agg.feeds) != len(DefaultFeeds) {
		t.Errorf("expected %d feeds, got %d", len(DefaultFeeds), len(agg.feeds))
	}
}
```

**Step 3: Run tests**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go test ./internal/news/ -v
```
Expected: all tests PASS (6 total across sentiment, tagger, aggregator files).

**Step 4: Commit**
```bash
git add backend/internal/news/aggregator.go backend/internal/news/aggregator_test.go
git commit -m "feat(phase7): add news aggregator with 15-min RSS fetch loop"
```

---

### Task B7: Create internal/api/news.go

**Read first:**
- `backend/internal/models/models.go` lines 304–345 (NewsItem + Notification structs)
- `backend/internal/api/health.go` (reference for handler pattern — struct + constructor)

**Files to create:**
- `backend/internal/api/news.go`

**Step 1: Create `backend/internal/api/news.go`**

```go
package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type newsHandler struct {
	db *gorm.DB
}

func newNewsHandler(db *gorm.DB) *newsHandler {
	return &newsHandler{db: db}
}

// GET /api/v1/news?limit=20&offset=0&symbol=BTCUSDT&from=RFC3339&to=RFC3339
func (h *newsHandler) listNews(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)
	symbol := c.Query("symbol")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	query := h.db.Model(&models.NewsItem{}).Order("published_at DESC")

	if symbol != "" {
		// MySQL JSON_CONTAINS: search for a JSON string value in the array column
		query = query.Where("JSON_CONTAINS(symbols, ?)", `"`+symbol+`"`)
	}
	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			query = query.Where("published_at >= ?", t)
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			query = query.Where("published_at <= ?", t)
		}
	}

	var total int64
	query.Count(&total)

	var items []models.NewsItem
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":      items,
		"total":     total,
		"page":      offset/limit + 1,
		"page_size": limit,
	})
}

// GET /api/v1/news/symbols/:symbol?limit=10
func (h *newsHandler) newsBySymbol(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	limit := c.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}

	var items []models.NewsItem
	if err := h.db.
		Where("JSON_CONTAINS(symbols, ?)", `"`+symbol+`"`).
		Order("published_at DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": items})
}
```

**Step 2: Verify build**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go build ./internal/api/
```
Expected: no output.

**Step 3: Commit**
```bash
git add backend/internal/api/news.go
git commit -m "feat(phase7): add news API handlers (list + by-symbol)"
```

---

### Task B8: Create internal/alert/evaluator.go + test

**Read first:**
- `backend/internal/models/models.go` lines 282–345 (Alert + Notification types/structs)
- `backend/internal/price/service.go` lines 44–46 (GetPrice signature)

**Files to create:**
- `backend/internal/alert/evaluator.go`
- `backend/internal/alert/evaluator_test.go`

**Step 1: Create `backend/internal/alert/evaluator.go`**

```go
package alert

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/price"
)

const (
	evalInterval    = 60 * time.Second
	redisPubChannel = "notifications:new"
)

// Evaluator checks active alerts every 60 seconds and fires notifications.
type Evaluator struct {
	db       *gorm.DB
	priceSvc *price.Service
	rdb      *redis.Client
}

// NewEvaluator creates an Evaluator.
func NewEvaluator(db *gorm.DB, priceSvc *price.Service, rdb *redis.Client) *Evaluator {
	return &Evaluator{db: db, priceSvc: priceSvc, rdb: rdb}
}

// Start launches the evaluation loop in a background goroutine.
// Cancel ctx to stop.
func (e *Evaluator) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(evalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.evaluate(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (e *Evaluator) evaluate(ctx context.Context) {
	var alerts []models.Alert
	if err := e.db.WithContext(ctx).
		Where("status = ?", models.AlertStatusActive).
		Find(&alerts).Error; err != nil {
		log.Printf("alert evaluator: failed to load alerts: %v", err)
		return
	}
	for i := range alerts {
		e.evalAlert(ctx, alerts[i])
	}
}

func (e *Evaluator) evalAlert(ctx context.Context, alert models.Alert) {
	// Cooldown check for recurring alerts
	if alert.RecurringEnabled && alert.LastFiredAt != nil {
		cooldown := time.Duration(alert.CooldownMinutes) * time.Minute
		if time.Since(*alert.LastFiredAt) < cooldown {
			return
		}
	}

	currentPrice, err := e.priceSvc.GetPrice(ctx, alert.AdapterID, alert.Symbol)
	if err != nil {
		log.Printf("alert evaluator: price fetch failed for %s/%s: %v", alert.AdapterID, alert.Symbol, err)
		return
	}

	triggered, msg := checkCondition(alert, currentPrice)
	if !triggered {
		return
	}

	// Create notification
	notif := models.Notification{
		Type:  models.NotificationTypeAlert,
		Title: alert.Name + " triggered",
		Body:  msg,
		Metadata: models.JSON{
			"alert_id": alert.ID,
			"symbol":   alert.Symbol,
			"price":    currentPrice,
		},
	}
	if err := e.db.WithContext(ctx).Create(&notif).Error; err != nil {
		log.Printf("alert evaluator: failed to create notification: %v", err)
		return
	}

	// Update alert state
	now := time.Now()
	updates := map[string]interface{}{"last_fired_at": now}
	if !alert.RecurringEnabled {
		updates["status"] = models.AlertStatusTriggered
		updates["triggered_at"] = now
	}
	if err := e.db.WithContext(ctx).Model(&alert).Updates(updates).Error; err != nil {
		log.Printf("alert evaluator: failed to update alert %d: %v", alert.ID, err)
	}

	// Publish notification ID to Redis for WebSocket push
	e.rdb.Publish(ctx, redisPubChannel, fmt.Sprintf("%d", notif.ID))
}

// checkCondition evaluates whether the current price satisfies the alert condition.
// Returns (triggered bool, human-readable message string).
// Exported for testing.
func checkCondition(alert models.Alert, currentPrice float64) (bool, string) {
	switch alert.Condition {
	case models.AlertConditionPriceAbove:
		if currentPrice > alert.Threshold {
			return true, fmt.Sprintf(
				"%s is above $%.4f (current: $%.4f)",
				alert.Symbol, alert.Threshold, currentPrice,
			)
		}
	case models.AlertConditionPriceBelow:
		if currentPrice < alert.Threshold {
			return true, fmt.Sprintf(
				"%s is below $%.4f (current: $%.4f)",
				alert.Symbol, alert.Threshold, currentPrice,
			)
		}
	case models.AlertConditionPriceChange:
		if alert.BasePrice == 0 {
			return false, ""
		}
		changePct := math.Abs((currentPrice-alert.BasePrice)/alert.BasePrice) * 100
		if changePct >= alert.Threshold {
			direction := "up"
			if currentPrice < alert.BasePrice {
				direction = "down"
			}
			return true, fmt.Sprintf(
				"%s moved %s %.2f%% from base $%.4f (current: $%.4f)",
				alert.Symbol, direction, changePct, alert.BasePrice, currentPrice,
			)
		}
	}
	return false, ""
}
```

**Step 2: Create `backend/internal/alert/evaluator_test.go`**

```go
package alert

import (
	"testing"

	"github.com/trader-claude/backend/internal/models"
)

func TestCheckCondition_PriceAbove_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceAbove,
		Threshold: 50000.0,
	}
	triggered, msg := checkCondition(a, 51000.0)
	if !triggered {
		t.Error("expected triggered")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestCheckCondition_PriceAbove_NotTriggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceAbove,
		Threshold: 50000.0,
	}
	triggered, _ := checkCondition(a, 49000.0)
	if triggered {
		t.Error("expected not triggered")
	}
}

func TestCheckCondition_PriceBelow_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceBelow,
		Threshold: 50000.0,
	}
	triggered, _ := checkCondition(a, 49000.0)
	if !triggered {
		t.Error("expected triggered")
	}
}

func TestCheckCondition_PriceBelow_NotTriggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceBelow,
		Threshold: 50000.0,
	}
	triggered, _ := checkCondition(a, 51000.0)
	if triggered {
		t.Error("expected not triggered")
	}
}

func TestCheckCondition_PriceChangePct_Up_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,    // 5% threshold
		BasePrice: 50000.0,
	}
	// 55000 = +10% change → above 5% threshold
	triggered, msg := checkCondition(a, 55000.0)
	if !triggered {
		t.Error("expected triggered at 10% change")
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestCheckCondition_PriceChangePct_NotTriggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 50000.0,
	}
	// 51000 = +2% change → below 5% threshold
	triggered, _ := checkCondition(a, 51000.0)
	if triggered {
		t.Error("expected not triggered at 2% change")
	}
}

func TestCheckCondition_PriceChangePct_Down_Triggered(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 50000.0,
	}
	// 47000 = -6% change → above 5% threshold
	triggered, _ := checkCondition(a, 47000.0)
	if !triggered {
		t.Error("expected triggered at -6% change")
	}
}

func TestCheckCondition_PriceChangePct_ZeroBase(t *testing.T) {
	a := models.Alert{
		Symbol:    "BTCUSDT",
		Condition: models.AlertConditionPriceChange,
		Threshold: 5.0,
		BasePrice: 0, // missing base price
	}
	triggered, _ := checkCondition(a, 51000.0)
	if triggered {
		t.Error("expected not triggered when base price is 0")
	}
}
```

**Step 3: Run tests**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go test ./internal/alert/ -v
```
Expected: 7 tests PASS.

**Step 4: Commit**
```bash
git add backend/internal/alert/
git commit -m "feat(phase7): add alert evaluator with price_above/below/change_pct conditions"
```

---

### Task B9: Create internal/api/alerts.go

**Read first:**
- `backend/internal/models/models.go` lines 282–320 (Alert types + struct)
- `backend/internal/price/service.go` lines 44–46 (GetPrice signature)

**Files to create:**
- `backend/internal/api/alerts.go`

**Step 1: Create `backend/internal/api/alerts.go`**

```go
package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/price"
)

type alertHandler struct {
	db       *gorm.DB
	priceSvc *price.Service
}

func newAlertHandler(db *gorm.DB, priceSvc *price.Service) *alertHandler {
	return &alertHandler{db: db, priceSvc: priceSvc}
}

type createAlertReq struct {
	Name             string                `json:"name"`
	AdapterID        string                `json:"adapter_id"`
	Symbol           string                `json:"symbol"`
	Market           string                `json:"market"`
	Condition        models.AlertCondition `json:"condition"`
	Threshold        float64               `json:"threshold"`
	RecurringEnabled bool                  `json:"recurring_enabled"`
	CooldownMinutes  int                   `json:"cooldown_minutes"`
}

// POST /api/v1/alerts
func (h *alertHandler) createAlert(c *fiber.Ctx) error {
	var req createAlertReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" || req.Symbol == "" || req.AdapterID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, symbol, and adapter_id are required",
		})
	}
	if req.CooldownMinutes == 0 {
		req.CooldownMinutes = 60
	}

	a := models.Alert{
		Name:             req.Name,
		AdapterID:        req.AdapterID,
		Symbol:           req.Symbol,
		Market:           req.Market,
		Condition:        req.Condition,
		Threshold:        req.Threshold,
		Status:           models.AlertStatusActive,
		RecurringEnabled: req.RecurringEnabled,
		CooldownMinutes:  req.CooldownMinutes,
	}

	// For price_change_pct alerts, store the current price as the base reference.
	if req.Condition == models.AlertConditionPriceChange {
		if basePrice, err := h.priceSvc.GetPrice(c.Context(), req.AdapterID, req.Symbol); err == nil {
			a.BasePrice = basePrice
		}
	}

	if err := h.db.Create(&a).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": a})
}

// GET /api/v1/alerts
func (h *alertHandler) listAlerts(c *fiber.Ctx) error {
	var alerts []models.Alert
	if err := h.db.Order("created_at DESC").Find(&alerts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": alerts})
}

// DELETE /api/v1/alerts/:id
func (h *alertHandler) deleteAlert(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.db.Delete(&models.Alert{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PATCH /api/v1/alerts/:id/toggle — toggles between active and disabled
func (h *alertHandler) toggleAlert(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var a models.Alert
	if err := h.db.First(&a, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "alert not found"})
	}
	newStatus := models.AlertStatusDisabled
	if a.Status == models.AlertStatusDisabled {
		newStatus = models.AlertStatusActive
	}
	h.db.Model(&a).Update("status", newStatus)
	a.Status = newStatus
	return c.JSON(fiber.Map{"data": a})
}
```

**Step 2: Verify build**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go build ./internal/api/
```
Expected: no output.

**Step 3: Commit**
```bash
git add backend/internal/api/alerts.go
git commit -m "feat(phase7): add alert CRUD API handlers (create/list/delete/toggle)"
```

---

### Task B10: Create internal/api/notifications.go

**Read first:**
- `backend/internal/models/models.go` lines 322–345 (Notification struct)
- `backend/internal/api/portfolio_ws.go` (reference for websocket handler pattern)

**Files to create:**
- `backend/internal/api/notifications.go`

**Step 1: Create `backend/internal/api/notifications.go`**

```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type notificationHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

func newNotificationHandler(db *gorm.DB, rdb *redis.Client) *notificationHandler {
	return &notificationHandler{db: db, rdb: rdb}
}

// GET /api/v1/notifications?limit=20&offset=0
func (h *notificationHandler) listNotifications(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)

	var total int64
	h.db.Model(&models.Notification{}).Count(&total)

	var notifs []models.Notification
	if err := h.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&notifs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":      notifs,
		"total":     total,
		"page":      offset/limit + 1,
		"page_size": limit,
	})
}

// PATCH /api/v1/notifications/:id/read
func (h *notificationHandler) markRead(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.db.Model(&models.Notification{}).
		Where("id = ?", id).
		Update("read", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/notifications/read-all
func (h *notificationHandler) markAllRead(c *fiber.Ctx) error {
	if err := h.db.Model(&models.Notification{}).
		Where("read = ?", false).
		Update("read", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /ws/notifications — subscribes to Redis pub/sub and pushes new notifications over WS.
// On connect: sends the 5 most recent unread notifications immediately.
// Ongoing: whenever the alert evaluator publishes a notification ID, fetch and push the full object.
func (h *notificationHandler) notificationsWS(conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := h.rdb.Subscribe(ctx, "notifications:new")
	defer sub.Close()

	// Send recent unread notifications on connect
	var recent []models.Notification
	h.db.Where("read = ?", false).Order("created_at DESC").Limit(5).Find(&recent)
	for i := len(recent) - 1; i >= 0; i-- { // oldest first
		if b, err := json.Marshal(recent[i]); err == nil {
			conn.WriteMessage(websocket.TextMessage, b)
		}
	}

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var notif models.Notification
			if err := h.db.First(&notif, msg.Payload).Error; err != nil {
				log.Printf("ws/notifications: notification %s not found: %v", msg.Payload, err)
				continue
			}
			b, err := json.Marshal(notif)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// unreadCount returns how many unread notifications exist (used by REST if needed later).
func (h *notificationHandler) unreadCount(c *fiber.Ctx) error {
	var count int64
	h.db.Model(&models.Notification{}).Where("read = ?", false).Count(&count)
	return c.JSON(fiber.Map{"count": count})
}

// helper used in notificationsWS
var _ = fmt.Sprintf // silence unused import lint if needed
```

**Step 2: Verify build**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go build ./internal/api/
```
Expected: no output.

**Step 3: Commit**
```bash
git add backend/internal/api/notifications.go
git commit -m "feat(phase7): add notification API handlers + WS push endpoint"
```

---

### Task B11: Wire routes.go and main.go

**Read first:**
- `backend/internal/api/routes.go` (full file — 102 lines)
- `backend/cmd/server/main.go` (full file — 198 lines)

**Files to modify:**
- `backend/internal/api/routes.go`
- `backend/cmd/server/main.go`

**Step 1: Replace stub alert/notification routes in `routes.go` and add news + WS routes**

Replace the entire `routes.go` file with this content:

```go
package api

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/indicator"
	"github.com/trader-claude/backend/internal/replay"
	"github.com/trader-claude/backend/internal/worker"
	"github.com/trader-claude/backend/internal/portfolio"
	"github.com/trader-claude/backend/internal/price"
	"github.com/trader-claude/backend/internal/ws"
)

// RegisterRoutes wires all HTTP and WebSocket routes onto the Fiber app
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager) {
	// Health
	health := newHealthHandler(db, rdb, version)
	app.Get("/health", health.check)

	// API v1 group
	v1 := app.Group("/api/v1")

	// --- Markets ---
	mh := newMarketsHandler(ds)
	v1.Get("/markets", mh.listAdapters)
	v1.Get("/markets/:adapterID/symbols", mh.listSymbols)

	// --- Candles ---
	v1.Get("/candles/timeframes", mh.listTimeframes)
	v1.Get("/candles", mh.getCandles)

	// --- Strategies ---
	bh := newBacktestHandler(db, rdb, pool, ds)
	v1.Get("/strategies", bh.listStrategies)
	v1.Get("/strategies/:id", bh.getStrategy)

	// --- Backtests ---
	v1.Post("/backtest/run", bh.runBacktest)
	v1.Get("/backtest/runs", bh.listRuns)
	v1.Get("/backtest/runs/:id", bh.getRun)
	v1.Delete("/backtest/runs/:id", bh.deleteRun)

	// --- Replay ---
	rh := newReplayHandler(db, ds, mgr)
	v1.Post("/backtest/runs/:id/replay", rh.createReplay)
	v1.Post("/replay/bookmarks", rh.createBookmark)
	v1.Get("/replay/bookmarks", rh.listBookmarks)
	v1.Get("/replay/bookmarks/:id", rh.getBookmark)
	v1.Delete("/replay/bookmarks/:id", rh.deleteBookmark)

	// --- Indicators ---
	ih := indicator.NewHandler()
	v1.Get("/indicators", ih.ListIndicators)
	v1.Post("/indicators/calculate", ih.Calculate)

	// --- Portfolios ---
	priceSvc := price.NewService(rdb, "", "")
	portfolioSvc := portfolio.NewService(db, priceSvc)
	ph := newPortfolioHandler(portfolioSvc)
	ph.registerRoutes(v1)

	// --- News ---
	nh := newNewsHandler(db)
	v1.Get("/news", nh.listNews)
	v1.Get("/news/symbols/:symbol", nh.newsBySymbol)

	// --- Alerts ---
	ah := newAlertHandler(db, priceSvc)
	v1.Post("/alerts", ah.createAlert)
	v1.Get("/alerts", ah.listAlerts)
	v1.Delete("/alerts/:id", ah.deleteAlert)
	v1.Patch("/alerts/:id/toggle", ah.toggleAlert)

	// --- Notifications ---
	nfh := newNotificationHandler(db, rdb)
	v1.Get("/notifications", nfh.listNotifications)
	v1.Patch("/notifications/:id/read", nfh.markRead)
	v1.Post("/notifications/read-all", nfh.markAllRead)
	v1.Get("/notifications/unread-count", nfh.unreadCount)

	// --- WebSocket upgrade middleware ---
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Market data WebSocket (hub)
	app.Get("/ws", websocket.New(hub.ServeWS))

	// Backtest progress WebSocket
	app.Get("/ws/backtest/:id/progress", websocket.New(bh.progressWS))

	// Replay WebSocket
	app.Get("/ws/replay/:replay_id", websocket.New(rh.replayWS))

	// Notifications WebSocket
	app.Get("/ws/notifications", websocket.New(nfh.notificationsWS))
}
```

**Step 2: Add news aggregator + alert evaluator startup to `main.go`**

In `main.go`, after the line `replayMgr := replay.NewManager()` and before `// 6. Setup Fiber app`, insert these lines:

Find:
```go
	// Initialize replay manager
	replayMgr := replay.NewManager()

	// 6. Setup Fiber app
```

Replace with:
```go
	// Initialize replay manager
	replayMgr := replay.NewManager()

	// Start news aggregator (fetches RSS every 15 min)
	newsAgg := news.NewAggregator(db, news.DefaultFeeds)
	newsAgg.Start(context.Background())

	// Start alert evaluator (checks active alerts every 60 s)
	priceSvcMain := price.NewService(rdb, "", "")
	alertEval := alertpkg.NewEvaluator(db, priceSvcMain, rdb)
	alertEval.Start(context.Background())

	// 6. Setup Fiber app
```

**Step 3: Add the two new imports to `main.go`**

Find the import block in `main.go`:
```go
import (
	...
	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/api"
	...
)
```

Add these two lines to the import block (alongside the other internal imports):
```go
	alertpkg "github.com/trader-claude/backend/internal/alert"
	"github.com/trader-claude/backend/internal/news"
	"github.com/trader-claude/backend/internal/price"
```

Note: `price` is already used in routes.go, not main.go, so only add `alertpkg` and `news`. The full import block in main.go should look like:

```go
import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	alertpkg "github.com/trader-claude/backend/internal/alert"
	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/api"
	"github.com/trader-claude/backend/internal/config"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/news"
	"github.com/trader-claude/backend/internal/price"
	"github.com/trader-claude/backend/internal/registry"
	"github.com/trader-claude/backend/internal/replay"
	"github.com/trader-claude/backend/internal/strategy"
	"github.com/trader-claude/backend/internal/worker"
	"github.com/trader-claude/backend/internal/ws"
)
```

**Step 4: Build and run all tests**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go build ./...
go test ./...
```
Expected: clean build, all tests pass.

**Step 5: Commit**
```bash
git add backend/internal/api/routes.go backend/cmd/server/main.go
git commit -m "feat(phase7): wire news aggregator, alert evaluator, and all new routes"
```

---

## FRONTEND TASKS

---

### Task F1: Update src/types/index.ts

**Read first:**
- `frontend/src/types/index.ts` lines 1–498 (full file)

**Files to modify:**
- `frontend/src/types/index.ts`

**Step 1: Add `NewsItem` interface** — insert after the existing `WatchList` interface (search for `export interface WatchList`):

```typescript
// --- News ---

export interface NewsItem {
  id: number
  url: string
  title: string
  summary: string
  source: string
  published_at: string
  symbols: string[]
  sentiment: number   // -1 (negative) to 1 (positive)
  fetched_at: string
  created_at: string
}
```

**Step 2: Update the existing `Alert` interface** — find the current `Alert` interface and replace it with:

```typescript
export interface Alert {
  id: number
  name: string
  adapter_id: string
  symbol: string
  market: string
  condition: AlertCondition
  threshold: number
  base_price: number
  status: AlertStatus
  message: string
  recurring_enabled: boolean
  cooldown_minutes: number
  last_fired_at?: string
  triggered_at?: string
  created_at: string
  updated_at: string
}
```

**Step 3: Replace the existing `AlertCreateRequest` interface** — find it and replace with:

```typescript
export interface AlertCreateRequest {
  name: string
  adapter_id: string
  symbol: string
  market: string
  condition: AlertCondition
  threshold: number
  recurring_enabled: boolean
  cooldown_minutes: number
}
```

**Step 4: Verify TypeScript compiles**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```
Expected: no errors.

**Step 5: Commit**
```bash
git add frontend/src/types/index.ts
git commit -m "feat(phase7): add NewsItem type and extend Alert interface with new fields"
```

---

### Task F2: Create src/api/news.ts

**Read first:**
- `frontend/src/api/client.ts` (full file — reference for import pattern)

**Files to create:**
- `frontend/src/api/news.ts`

**Step 1: Create `frontend/src/api/news.ts`**

```typescript
import apiClient from './client'
import type { NewsItem } from '@/types'

export interface NewsFilters {
  symbol?: string
  limit?: number
  offset?: number
  from?: string
  to?: string
}

export interface NewsListResponse {
  data: NewsItem[]
  total: number
  page: number
  page_size: number
}

export async function fetchNews(filters: NewsFilters = {}): Promise<NewsListResponse> {
  const params = new URLSearchParams()
  if (filters.symbol) params.set('symbol', filters.symbol)
  if (filters.limit != null) params.set('limit', String(filters.limit))
  if (filters.offset != null) params.set('offset', String(filters.offset))
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  const { data } = await apiClient.get<NewsListResponse>(`/api/v1/news?${params.toString()}`)
  return data
}

export async function fetchNewsBySymbol(
  symbol: string,
  limit = 10,
): Promise<{ data: NewsItem[] }> {
  const { data } = await apiClient.get<{ data: NewsItem[] }>(
    `/api/v1/news/symbols/${encodeURIComponent(symbol)}?limit=${limit}`,
  )
  return data
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```
Expected: no errors.

**Step 3: Commit**
```bash
git add frontend/src/api/news.ts
git commit -m "feat(phase7): add news API client functions"
```

---

### Task F3: Create src/api/alerts.ts

**Read first:**
- `frontend/src/api/client.ts` (full file)

**Files to create:**
- `frontend/src/api/alerts.ts`

**Step 1: Create `frontend/src/api/alerts.ts`**

```typescript
import apiClient from './client'
import type { Alert, AlertCreateRequest, Notification } from '@/types'

// --- Alerts ---

export async function fetchAlerts(): Promise<{ data: Alert[] }> {
  const { data } = await apiClient.get<{ data: Alert[] }>('/api/v1/alerts')
  return data
}

export async function createAlert(req: AlertCreateRequest): Promise<{ data: Alert }> {
  const { data } = await apiClient.post<{ data: Alert }>('/api/v1/alerts', req)
  return data
}

export async function deleteAlert(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/alerts/${id}`)
}

export async function toggleAlert(id: number): Promise<{ data: Alert }> {
  const { data } = await apiClient.patch<{ data: Alert }>(`/api/v1/alerts/${id}/toggle`)
  return data
}

// --- Notifications ---

export interface NotificationsParams {
  limit?: number
  offset?: number
}

export interface NotificationsResponse {
  data: Notification[]
  total: number
  page: number
  page_size: number
}

export async function fetchNotifications(
  params: NotificationsParams = {},
): Promise<NotificationsResponse> {
  const query = new URLSearchParams()
  if (params.limit != null) query.set('limit', String(params.limit))
  if (params.offset != null) query.set('offset', String(params.offset))
  const { data } = await apiClient.get<NotificationsResponse>(
    `/api/v1/notifications?${query.toString()}`,
  )
  return data
}

export async function markNotificationRead(id: number): Promise<void> {
  await apiClient.patch(`/api/v1/notifications/${id}/read`)
}

export async function markAllNotificationsRead(): Promise<void> {
  await apiClient.post('/api/v1/notifications/read-all')
}

export async function fetchUnreadCount(): Promise<number> {
  const { data } = await apiClient.get<{ count: number }>('/api/v1/notifications/unread-count')
  return data.count
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/api/alerts.ts
git commit -m "feat(phase7): add alerts and notifications API client functions"
```

---

### Task F4: Create src/hooks/useNews.ts

**Read first:**
- `frontend/src/hooks/useMarketData.ts` (reference for React Query hook pattern)

**Files to create:**
- `frontend/src/hooks/useNews.ts`

**Step 1: Create `frontend/src/hooks/useNews.ts`**

```typescript
import { useQuery } from '@tanstack/react-query'
import { fetchNews, fetchNewsBySymbol, type NewsFilters } from '@/api/news'

const NEWS_STALE_MS = 5 * 60 * 1000 // 5 minutes

export function useNews(filters: NewsFilters = {}) {
  return useQuery({
    queryKey: ['news', filters],
    queryFn: () => fetchNews(filters),
    staleTime: NEWS_STALE_MS,
  })
}

export function useNewsBySymbol(symbol: string | null, limit = 10) {
  return useQuery({
    queryKey: ['news', 'symbol', symbol, limit],
    queryFn: () => fetchNewsBySymbol(symbol!, limit),
    enabled: !!symbol,
    staleTime: NEWS_STALE_MS,
  })
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/hooks/useNews.ts
git commit -m "feat(phase7): add useNews and useNewsBySymbol React Query hooks"
```

---

### Task F5: Create src/hooks/useAlerts.ts

**Read first:**
- `frontend/src/hooks/useMarketData.ts` (reference for mutation pattern)
- `frontend/src/stores/index.ts` lines 1–50 (for useAlertStore)

**Files to create:**
- `frontend/src/hooks/useAlerts.ts`

**Step 1: Create `frontend/src/hooks/useAlerts.ts`**

```typescript
import { useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { fetchAlerts, createAlert, deleteAlert, toggleAlert } from '@/api/alerts'
import { useAlertStore } from '@/stores'
import type { AlertCreateRequest } from '@/types'

export function useAlerts() {
  const setAlerts = useAlertStore((s) => s.setAlerts)

  const query = useQuery({
    queryKey: ['alerts'],
    queryFn: fetchAlerts,
  })

  useEffect(() => {
    if (query.data?.data) {
      setAlerts(query.data.data)
    }
  }, [query.data, setAlerts])

  return query
}

export function useCreateAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: AlertCreateRequest) => createAlert(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}

export function useDeleteAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => deleteAlert(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}

export function useToggleAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => toggleAlert(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/hooks/useAlerts.ts
git commit -m "feat(phase7): add useAlerts, useCreateAlert, useDeleteAlert, useToggleAlert hooks"
```

---

### Task F6: Create src/hooks/useNotifications.ts

**Read first:**
- `frontend/src/stores/index.ts` (full file — for useNotificationStore interface)
- `frontend/src/hooks/usePortfolioLive.ts` (reference for WebSocket hook pattern)

**Files to create:**
- `frontend/src/hooks/useNotifications.ts`

**Step 1: Create `frontend/src/hooks/useNotifications.ts`**

```typescript
import { useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  fetchNotifications,
  markNotificationRead,
  markAllNotificationsRead,
} from '@/api/alerts'
import { useNotificationStore } from '@/stores'
import type { Notification } from '@/types'

export function useNotifications(page = 1, pageSize = 20) {
  const setNotifications = useNotificationStore((s) => s.setNotifications)

  const query = useQuery({
    queryKey: ['notifications', page, pageSize],
    queryFn: () => fetchNotifications({ limit: pageSize, offset: (page - 1) * pageSize }),
  })

  useEffect(() => {
    if (query.data?.data) {
      setNotifications(query.data.data)
    }
  }, [query.data, setNotifications])

  return query
}

export function useMarkRead() {
  const qc = useQueryClient()
  const markRead = useNotificationStore((s) => s.markRead)
  return useMutation({
    mutationFn: (id: number) => markNotificationRead(id),
    onSuccess: (_, id) => {
      markRead(id)
      qc.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

export function useMarkAllRead() {
  const qc = useQueryClient()
  const markAllRead = useNotificationStore((s) => s.markAllRead)
  return useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      markAllRead()
      qc.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

// useNotificationWS connects to /ws/notifications and adds incoming
// notifications to the Zustand store (which updates the unread badge).
export function useNotificationWS() {
  const addNotification = useNotificationStore((s) => s.addNotification)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    const wsUrl = (import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080') as string
    const ws = new WebSocket(`${wsUrl}/ws/notifications`)
    wsRef.current = ws

    ws.onmessage = (e: MessageEvent) => {
      try {
        const notif = JSON.parse(e.data as string) as Notification
        addNotification(notif)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onerror = () => {
      // suppress console errors — server may not be running locally
    }

    return () => {
      ws.close()
    }
  }, [addNotification])
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/hooks/useNotifications.ts
git commit -m "feat(phase7): add useNotifications, useMarkRead, useMarkAllRead, useNotificationWS hooks"
```

---

### Task F7: Create src/components/news/NewsSidePanel.tsx

**Read first:**
- `frontend/src/types/index.ts` — find `NewsItem` interface
- `frontend/src/lib/utils.ts` — `cn` utility signature

**Files to create:**
- `frontend/src/components/news/NewsSidePanel.tsx`

**Step 1: Create `frontend/src/components/news/NewsSidePanel.tsx`**

```tsx
import { X, ExternalLink } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { cn } from '@/lib/utils'
import type { NewsItem } from '@/types'

interface SentimentDotProps {
  sentiment: number
}

function SentimentDot({ sentiment }: SentimentDotProps) {
  const color =
    sentiment > 0.2
      ? 'bg-green-500'
      : sentiment < -0.2
        ? 'bg-red-500'
        : 'bg-yellow-400'
  const label =
    sentiment > 0.2 ? 'Positive' : sentiment < -0.2 ? 'Negative' : 'Neutral'
  return (
    <span
      className={cn('mt-1 inline-block w-2 h-2 rounded-full shrink-0', color)}
      title={label}
      aria-label={label}
    />
  )
}

interface NewsSidePanelProps {
  items: NewsItem[]
  isLoading: boolean
  onClose: () => void
}

export function NewsSidePanel({ items, isLoading, onClose }: NewsSidePanelProps) {
  return (
    <div className="w-80 flex flex-col border-l border-border bg-card shrink-0 h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border shrink-0">
        <h3 className="font-semibold text-sm">News</h3>
        <button
          onClick={onClose}
          className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          aria-label="Close news panel"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Feed */}
      <div className="flex-1 overflow-y-auto">
        {isLoading && (
          <p className="p-4 text-sm text-muted-foreground">Loading news…</p>
        )}
        {!isLoading && items.length === 0 && (
          <p className="p-4 text-sm text-muted-foreground">No news found for this symbol.</p>
        )}
        {items.map((item) => (
          <a
            key={item.id}
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex gap-2 px-4 py-3 border-b border-border hover:bg-accent/50 transition-colors group"
          >
            <SentimentDot sentiment={item.sentiment} />
            <div className="min-w-0 flex-1">
              <p className="text-xs font-medium leading-snug line-clamp-3 text-foreground group-hover:text-primary transition-colors">
                {item.title}
              </p>
              <div className="flex items-center gap-2 mt-1.5">
                <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide">
                  {item.source}
                </span>
                <span className="text-[10px] text-muted-foreground">
                  {formatDistanceToNow(new Date(item.published_at), { addSuffix: true })}
                </span>
                <ExternalLink className="w-3 h-3 text-muted-foreground ml-auto opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
              </div>
            </div>
          </a>
        ))}
      </div>
    </div>
  )
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/components/news/NewsSidePanel.tsx
git commit -m "feat(phase7): add NewsSidePanel component with sentiment dots and source badges"
```

---

### Task F8: Update Chart.tsx to add news panel

**Read first:**
- `frontend/src/pages/Chart.tsx` (full file — 381 lines)

**Files to modify:**
- `frontend/src/pages/Chart.tsx`

**Step 1: Add `Newspaper` to the lucide-react import**

Find line 2:
```tsx
import { RefreshCw, ChevronDown, Search, BarChart2 } from 'lucide-react'
```
Replace with:
```tsx
import { RefreshCw, ChevronDown, Search, BarChart2, Newspaper } from 'lucide-react'
```

**Step 2: Add news imports after the existing imports block (after line 13)**

Find:
```tsx
const TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w']
```
Insert before it:
```tsx
import { useState as useNewsState } from 'react'
import { NewsSidePanel } from '@/components/news/NewsSidePanel'
import { useNewsBySymbol } from '@/hooks/useNews'
```

Wait — `useState` is already imported on line 1. Instead, just add the two component/hook imports. The correct approach: add them to the existing import block.

Replace the top import block (lines 1–13):
```tsx
import { useState, useMemo, useCallback, useEffect } from 'react'
import { RefreshCw, ChevronDown, Search, BarChart2, Newspaper } from 'lucide-react'
import { subDays, formatISO } from 'date-fns'
import { useMutation } from '@tanstack/react-query'
import { useMarkets, useSymbols, useCandles, useTimeframes } from '@/hooks/useMarketData'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { IndicatorModal } from '@/components/chart/IndicatorModal'
import { IndicatorChips } from '@/components/chart/IndicatorChips'
import { PanelChart } from '@/components/chart/PanelChart'
import { NewsSidePanel } from '@/components/news/NewsSidePanel'
import { calculateIndicator } from '@/api/indicators'
import { useMarketStore, useThemeStore } from '@/stores'
import { useNewsBySymbol } from '@/hooks/useNews'
import type { ActiveIndicator, MarketSymbol, OHLCVCandle } from '@/types'
```

**Step 3: Add `newsOpen` state inside the `Chart` function** — after the existing state declarations (after `const [selectedAdapter, setSelectedAdapter] = useState('binance')` around line 56):

Find:
```tsx
  const [selectedAdapter, setSelectedAdapter] = useState('binance')
```
Insert after it:
```tsx
  const [newsOpen, setNewsOpen] = useState(false)
```

**Step 4: Add news data hook** — after the `useCandles` call block (after line 82 `}`), insert:

```tsx
  const { data: newsData, isFetching: newsFetching } = useNewsBySymbol(
    newsOpen ? selectedSymbol : null,
    20,
  )
  const newsItems = newsData?.data ?? []
```

**Step 5: Add News toggle button in the toolbar** — find the Indicators button block and insert the News button right after it (before the active indicator chips check):

Find:
```tsx
        {/* Indicators button */}
        <button
          onClick={() => setIndicatorModalOpen(true)}
          className="flex items-center gap-1.5 px-3 py-2 text-sm bg-card border border-border rounded-md hover:bg-accent transition-colors"
          aria-label="Open indicators"
        >
          <BarChart2 className="h-4 w-4" />
          Indicators
        </button>
```
Insert after it (before `{/* Active indicator chips */}`):
```tsx
        {/* News toggle button */}
        <button
          onClick={() => setNewsOpen((v) => !v)}
          className={`flex items-center gap-1.5 px-3 py-2 text-sm border rounded-md transition-colors ${
            newsOpen
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-card border-border hover:bg-accent'
          }`}
          aria-label="Toggle news panel"
          aria-pressed={newsOpen}
        >
          <Newspaper className="h-4 w-4" />
          News
        </button>
```

**Step 6: Restructure the chart area to sit alongside the news panel**

Find:
```tsx
      {/* ── Chart area ── */}
      <div className="flex-1 bg-card border border-border rounded-lg overflow-hidden min-h-0">
```

This `<div className="flex-1 ...">` needs to be wrapped together with the NewsSidePanel in a flex row. Replace from `{/* ── Chart area ── */}` to just before `{/* ── Panel indicators ── */}` with:

```tsx
      {/* ── Chart area + News panel row ── */}
      <div className="flex flex-1 gap-0 min-h-0 overflow-hidden">
        {/* Chart column */}
        <div className="flex flex-col flex-1 gap-4 min-h-0 min-w-0 overflow-hidden">
          <div className="flex-1 bg-card border border-border rounded-lg overflow-hidden min-h-0">
            {!selectedSymbol ? (
              /* Empty state */
              <div className="flex flex-col items-center justify-center h-full gap-3 text-muted-foreground">
                <Search className="h-12 w-12 opacity-30" />
                <p className="text-lg font-medium">Select a symbol to view chart</p>
                <p className="text-sm">Choose an adapter and search for a symbol above</p>
              </div>
            ) : isError ? (
              /* Error state */
              <div className="flex flex-col items-center justify-center h-full gap-3">
                <p className="text-destructive font-medium">Failed to load candles</p>
                <p className="text-sm text-muted-foreground">
                  {error instanceof Error ? error.message : 'Unknown error'}
                </p>
                <button
                  onClick={() => refetch()}
                  className="mt-2 px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm hover:bg-primary/90 transition-colors"
                >
                  Retry
                </button>
              </div>
            ) : (
              /* Chart with loading overlay */
              <CandlestickChart
                candles={candles ?? []}
                overlayIndicators={overlayIndicators}
                isLoading={isFetching}
                className="h-full"
              />
            )}
          </div>

          {/* ── Panel indicators ── */}
          {panelIndicators.map((ind) => (
            <PanelChart
              key={ind.meta.id}
              indicator={ind}
              onClose={() => handleRemoveIndicator(ind.meta.id)}
              isDark={isDark}
            />
          ))}
        </div>

        {/* News side panel (conditional) */}
        {newsOpen && (
          <NewsSidePanel
            items={newsItems}
            isLoading={newsFetching}
            onClose={() => setNewsOpen(false)}
          />
        )}
      </div>
```

Remove the old standalone `{/* ── Panel indicators ── */}` block (it's now inside the chart column above).

**Step 7: Verify**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```
Expected: no errors.

**Step 8: Commit**
```bash
git add frontend/src/pages/Chart.tsx
git commit -m "feat(phase7): add collapsible news side panel to Chart page"
```

---

### Task F9: Update TopBar.tsx — notification bell dropdown

**Read first:**
- `frontend/src/components/layout/TopBar.tsx` (full file — ~50 lines)
- `frontend/src/stores/index.ts` — find `useNotificationStore` interface

**Files to modify:**
- `frontend/src/components/layout/TopBar.tsx`

**Step 1: Replace the entire `TopBar.tsx` file with this implementation**

This uses `@radix-ui/react-dropdown-menu` (already in package.json) for the bell dropdown and the `useMarkAllRead` hook.

```tsx
import { useNavigate } from 'react-router-dom'
import { Menu, Moon, Sun, Bell, CheckCheck } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { useThemeStore, useNotificationStore, useSidebarStore } from '@/stores'
import { useMarkAllRead } from '@/hooks/useNotifications'
import { cn } from '@/lib/utils'

export function TopBar() {
  const { theme, toggleTheme } = useThemeStore()
  const { unreadCount, notifications } = useNotificationStore()
  const { toggle } = useSidebarStore()
  const navigate = useNavigate()
  const { mutate: markAllRead } = useMarkAllRead()

  // Show last 5 notifications in dropdown (most recent first)
  const recentNotifications = notifications.slice(0, 5)

  return (
    <header className="h-16 border-b border-border bg-card flex items-center gap-4 px-4 shrink-0">
      {/* Mobile hamburger */}
      <button
        onClick={toggle}
        className="lg:hidden p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label="Toggle sidebar"
      >
        <Menu className="w-5 h-5" />
      </button>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      >
        {theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
      </button>

      {/* Notification bell with dropdown */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button
            className="relative p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
            aria-label="Notifications"
          >
            <Bell className="w-5 h-5" />
            {unreadCount > 0 && (
              <span
                className={cn(
                  'absolute top-1 right-1 min-w-[1rem] h-4 px-0.5',
                  'flex items-center justify-center',
                  'text-[10px] font-bold text-white bg-destructive rounded-full',
                )}
              >
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </button>
        </DropdownMenu.Trigger>

        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className={cn(
              'z-50 w-80 rounded-lg border border-border bg-card shadow-lg',
              'animate-in fade-in-0 zoom-in-95',
            )}
          >
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-border">
              <span className="font-semibold text-sm">Notifications</span>
              {unreadCount > 0 && (
                <button
                  onClick={() => markAllRead()}
                  className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                  aria-label="Mark all as read"
                >
                  <CheckCheck className="w-3.5 h-3.5" />
                  Mark all read
                </button>
              )}
            </div>

            {/* Notification list */}
            {recentNotifications.length === 0 ? (
              <div className="px-4 py-6 text-center text-sm text-muted-foreground">
                No notifications yet
              </div>
            ) : (
              <div>
                {recentNotifications.map((n) => (
                  <DropdownMenu.Item
                    key={n.id}
                    className={cn(
                      'flex flex-col gap-0.5 px-4 py-3 border-b border-border last:border-0',
                      'cursor-default select-none outline-none',
                      'hover:bg-accent transition-colors',
                      !n.read && 'bg-primary/5',
                    )}
                  >
                    <div className="flex items-start gap-2">
                      {!n.read && (
                        <span className="mt-1.5 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                      )}
                      <div className={cn('min-w-0', n.read && 'pl-3.5')}>
                        <p className="text-xs font-medium truncate">{n.title}</p>
                        <p className="text-xs text-muted-foreground line-clamp-2 mt-0.5">
                          {n.body}
                        </p>
                        <p className="text-[10px] text-muted-foreground mt-1">
                          {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                        </p>
                      </div>
                    </div>
                  </DropdownMenu.Item>
                ))}
              </div>
            )}

            {/* Footer */}
            <DropdownMenu.Item
              className="flex justify-center px-4 py-2.5 text-xs text-primary hover:text-primary/80 hover:bg-accent transition-colors cursor-pointer outline-none"
              onSelect={() => navigate('/notifications')}
            >
              View all notifications →
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </header>
  )
}
```

**Step 2: Wire the WS hook into Layout.tsx** so notifications stream from startup.

Read `frontend/src/components/layout/Layout.tsx`. It currently looks like:
```tsx
export function Layout() {
  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar />
      <div className="flex flex-col flex-1 overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-y-auto p-6 animate-fade-in">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

Replace it with:
```tsx
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'
import { useNotificationWS } from '@/hooks/useNotifications'

export function Layout() {
  useNotificationWS() // connect to /ws/notifications on mount

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      <Sidebar />
      <div className="flex flex-col flex-1 overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-y-auto p-6 animate-fade-in">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

**Step 3: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```
Expected: no errors.

**Step 4: Commit**
```bash
git add frontend/src/components/layout/TopBar.tsx frontend/src/components/layout/Layout.tsx
git commit -m "feat(phase7): add notification bell dropdown with last 5 notifications and mark-all-read"
```

---

### Task F10: Replace pages/Alerts.tsx

**Read first:**
- `frontend/src/types/index.ts` — find `Alert`, `AlertCondition`, `AlertStatus`, `AlertCreateRequest`
- `frontend/src/stores/index.ts` — find `useAlertStore`

**Files to modify:**
- `frontend/src/pages/Alerts.tsx`

**Step 1: Replace the entire `Alerts.tsx` file**

```tsx
import { useState } from 'react'
import { Plus, Trash2, ToggleLeft, ToggleRight, AlertTriangle } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { useAlerts, useCreateAlert, useDeleteAlert, useToggleAlert } from '@/hooks/useAlerts'
import { cn } from '@/lib/utils'
import type { AlertCondition, AlertCreateRequest, AlertStatus } from '@/types'

// ── AddAlertModal ──────────────────────────────────────────────────────────────

interface AddAlertModalProps {
  onClose: () => void
}

const CONDITIONS: { value: AlertCondition; label: string; hint: string }[] = [
  { value: 'price_above', label: 'Price Above', hint: 'Fires when price exceeds threshold' },
  { value: 'price_below', label: 'Price Below', hint: 'Fires when price falls below threshold' },
  {
    value: 'price_change_pct',
    label: 'Price Change %',
    hint: 'Fires when price moves ±N% from current price',
  },
]

function AddAlertModal({ onClose }: AddAlertModalProps) {
  const { mutateAsync: createAlert, isPending } = useCreateAlert()

  const [form, setForm] = useState<AlertCreateRequest>({
    name: '',
    adapter_id: 'binance',
    symbol: '',
    market: 'crypto',
    condition: 'price_above',
    threshold: 0,
    recurring_enabled: true,
    cooldown_minutes: 60,
  })

  const set = <K extends keyof AlertCreateRequest>(k: K, v: AlertCreateRequest[K]) =>
    setForm((prev) => ({ ...prev, [k]: v }))

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    await createAlert(form)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-card border border-border rounded-lg shadow-xl w-full max-w-md p-6">
        <h2 className="text-lg font-semibold mb-4">New Alert</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium mb-1">Name</label>
            <input
              type="text"
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              placeholder="BTC above 100k"
              value={form.name}
              onChange={(e) => set('name', e.target.value)}
              required
            />
          </div>

          {/* Adapter */}
          <div>
            <label className="block text-sm font-medium mb-1">Adapter</label>
            <select
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              value={form.adapter_id}
              onChange={(e) => set('adapter_id', e.target.value)}
            >
              <option value="binance">Binance (Crypto)</option>
              <option value="yahoo">Yahoo Finance (Stocks)</option>
            </select>
          </div>

          {/* Symbol */}
          <div>
            <label className="block text-sm font-medium mb-1">Symbol</label>
            <input
              type="text"
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              placeholder="BTCUSDT"
              value={form.symbol}
              onChange={(e) => set('symbol', e.target.value.toUpperCase())}
              required
            />
          </div>

          {/* Condition */}
          <div>
            <label className="block text-sm font-medium mb-1">Condition</label>
            <select
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              value={form.condition}
              onChange={(e) => set('condition', e.target.value as AlertCondition)}
            >
              {CONDITIONS.map((c) => (
                <option key={c.value} value={c.value}>
                  {c.label}
                </option>
              ))}
            </select>
            <p className="mt-1 text-xs text-muted-foreground">
              {CONDITIONS.find((c) => c.value === form.condition)?.hint}
            </p>
          </div>

          {/* Threshold */}
          <div>
            <label className="block text-sm font-medium mb-1">
              {form.condition === 'price_change_pct' ? 'Change % Threshold' : 'Price Threshold ($)'}
            </label>
            <input
              type="number"
              step="any"
              min="0"
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
              value={form.threshold}
              onChange={(e) => set('threshold', parseFloat(e.target.value) || 0)}
              required
            />
          </div>

          {/* Recurring */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Recurring</p>
              <p className="text-xs text-muted-foreground">Re-fires after cooldown period</p>
            </div>
            <button
              type="button"
              onClick={() => set('recurring_enabled', !form.recurring_enabled)}
              className="text-muted-foreground hover:text-foreground transition-colors"
              aria-label="Toggle recurring"
            >
              {form.recurring_enabled ? (
                <ToggleRight className="w-8 h-8 text-primary" />
              ) : (
                <ToggleLeft className="w-8 h-8" />
              )}
            </button>
          </div>

          {/* Cooldown */}
          {form.recurring_enabled && (
            <div>
              <label className="block text-sm font-medium mb-1">Cooldown (minutes)</label>
              <input
                type="number"
                min="1"
                className="w-full bg-background border border-border rounded-md px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
                value={form.cooldown_minutes}
                onChange={(e) => set('cooldown_minutes', parseInt(e.target.value) || 60)}
              />
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 text-sm border border-border rounded-md hover:bg-accent transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="flex-1 px-4 py-2 text-sm bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {isPending ? 'Creating…' : 'Create Alert'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ── Status badge ───────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: AlertStatus }) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium',
        status === 'active' && 'bg-green-500/15 text-green-600 dark:text-green-400',
        status === 'triggered' && 'bg-yellow-500/15 text-yellow-600 dark:text-yellow-400',
        status === 'disabled' && 'bg-muted text-muted-foreground',
      )}
    >
      {status}
    </span>
  )
}

// ── Alerts page ────────────────────────────────────────────────────────────────

export function Alerts() {
  const [showModal, setShowModal] = useState(false)
  const { data, isLoading, isError } = useAlerts()
  const { mutate: deleteAlert } = useDeleteAlert()
  const { mutate: toggleAlert } = useToggleAlert()

  const alerts = data?.data ?? []

  return (
    <div>
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Alerts</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Price alerts — evaluated every 60 seconds
          </p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Alert
        </button>
      </div>

      {/* Table */}
      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading alerts…</p>
      )}
      {isError && (
        <div className="flex items-center gap-2 text-destructive text-sm">
          <AlertTriangle className="w-4 h-4" />
          Failed to load alerts
        </div>
      )}
      {!isLoading && alerts.length === 0 && (
        <div className="text-center py-16 text-muted-foreground">
          <AlertTriangle className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p className="font-medium">No alerts yet</p>
          <p className="text-sm mt-1">Create your first alert to get notified on price moves</p>
        </div>
      )}

      {alerts.length > 0 && (
        <div className="bg-card border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-muted-foreground">
                <th className="text-left px-4 py-3 font-medium">Name</th>
                <th className="text-left px-4 py-3 font-medium">Symbol</th>
                <th className="text-left px-4 py-3 font-medium">Condition</th>
                <th className="text-left px-4 py-3 font-medium">Threshold</th>
                <th className="text-left px-4 py-3 font-medium">Status</th>
                <th className="text-left px-4 py-3 font-medium">Last Fired</th>
                <th className="text-right px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {alerts.map((alert) => (
                <tr key={alert.id} className="hover:bg-accent/30 transition-colors">
                  <td className="px-4 py-3 font-medium">{alert.name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{alert.symbol}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {alert.condition.replace(/_/g, ' ')}
                  </td>
                  <td className="px-4 py-3">
                    {alert.condition === 'price_change_pct'
                      ? `±${alert.threshold}%`
                      : `$${alert.threshold.toLocaleString()}`}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={alert.status} />
                  </td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">
                    {alert.last_fired_at
                      ? formatDistanceToNow(new Date(alert.last_fired_at), { addSuffix: true })
                      : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => toggleAlert(alert.id)}
                        className="p-1.5 rounded text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                        title={alert.status === 'active' ? 'Disable' : 'Enable'}
                        aria-label={alert.status === 'active' ? 'Disable alert' : 'Enable alert'}
                      >
                        {alert.status === 'active' ? (
                          <ToggleRight className="w-4 h-4 text-primary" />
                        ) : (
                          <ToggleLeft className="w-4 h-4" />
                        )}
                      </button>
                      <button
                        onClick={() => deleteAlert(alert.id)}
                        className="p-1.5 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                        title="Delete alert"
                        aria-label="Delete alert"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Alert Modal */}
      {showModal && <AddAlertModal onClose={() => setShowModal(false)} />}
    </div>
  )
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/pages/Alerts.tsx
git commit -m "feat(phase7): implement Alerts page with table, add-alert modal, toggle and delete"
```

---

### Task F11: Replace pages/Notifications.tsx

**Read first:**
- `frontend/src/stores/index.ts` — find `useNotificationStore` interface (notifications, unreadCount, markRead, markAllRead)

**Files to modify:**
- `frontend/src/pages/Notifications.tsx`

**Step 1: Replace the entire `Notifications.tsx` file**

```tsx
import { useState } from 'react'
import { Bell, CheckCheck, Circle } from 'lucide-react'
import { formatDistanceToNow, format } from 'date-fns'
import { useNotifications, useMarkRead, useMarkAllRead } from '@/hooks/useNotifications'
import { cn } from '@/lib/utils'
import type { NotificationType } from '@/types'

const PAGE_SIZE = 20

function TypeBadge({ type }: { type: NotificationType }) {
  const colors: Record<NotificationType, string> = {
    alert: 'bg-yellow-500/15 text-yellow-600 dark:text-yellow-400',
    trade: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
    system: 'bg-muted text-muted-foreground',
    backtest: 'bg-purple-500/15 text-purple-600 dark:text-purple-400',
  }
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide',
        colors[type],
      )}
    >
      {type}
    </span>
  )
}

export function Notifications() {
  const [page, setPage] = useState(1)
  const { data, isLoading, isError } = useNotifications(page, PAGE_SIZE)
  const { mutate: markRead } = useMarkRead()
  const { mutate: markAllRead } = useMarkAllRead()

  const notifications = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div>
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Notifications</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {total} total · {data?.data.filter((n) => !n.read).length ?? 0} unread
          </p>
        </div>
        <button
          onClick={() => markAllRead()}
          className="flex items-center gap-2 px-4 py-2 text-sm border border-border rounded-md hover:bg-accent transition-colors"
        >
          <CheckCheck className="w-4 h-4" />
          Mark all read
        </button>
      </div>

      {/* Loading / error / empty */}
      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading notifications…</p>
      )}
      {isError && (
        <p className="text-sm text-destructive">Failed to load notifications.</p>
      )}
      {!isLoading && notifications.length === 0 && (
        <div className="text-center py-16 text-muted-foreground">
          <Bell className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p className="font-medium">No notifications yet</p>
        </div>
      )}

      {/* Notification list */}
      {notifications.length > 0 && (
        <div className="bg-card border border-border rounded-lg overflow-hidden divide-y divide-border">
          {notifications.map((n) => (
            <div
              key={n.id}
              className={cn(
                'flex gap-3 px-4 py-4 transition-colors',
                !n.read && 'bg-primary/5',
                'hover:bg-accent/30',
              )}
            >
              {/* Unread indicator */}
              <div className="flex-shrink-0 mt-1">
                {n.read ? (
                  <Circle className="w-2 h-2 text-muted-foreground/30" />
                ) : (
                  <Circle className="w-2 h-2 text-primary fill-primary" />
                )}
              </div>

              {/* Content */}
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-center gap-2 flex-wrap">
                    <p className="text-sm font-medium">{n.title}</p>
                    <TypeBadge type={n.type} />
                  </div>
                  <time
                    className="text-xs text-muted-foreground shrink-0"
                    title={format(new Date(n.created_at), 'PPpp')}
                  >
                    {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                  </time>
                </div>
                <p className="text-sm text-muted-foreground mt-0.5">{n.body}</p>
              </div>

              {/* Mark read action */}
              {!n.read && (
                <button
                  onClick={() => markRead(n.id)}
                  className="shrink-0 px-2 py-1 text-xs text-muted-foreground hover:text-foreground border border-border rounded hover:bg-accent transition-colors"
                  aria-label="Mark as read"
                >
                  Read
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-6">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="px-3 py-1.5 text-sm border border-border rounded-md hover:bg-accent transition-colors disabled:opacity-40"
          >
            Previous
          </button>
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
            className="px-3 py-1.5 text-sm border border-border rounded-md hover:bg-accent transition-colors disabled:opacity-40"
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
```

**Step 2: Verify TypeScript**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add frontend/src/pages/Notifications.tsx
git commit -m "feat(phase7): implement Notifications page with pagination and mark-read actions"
```

---

### Task F12: Final verification + mark Phase 7 complete

**Read first:**
- `.claude/docs/phases.md` lines 208–244 (Phase 7 section)

**Files to modify:**
- `.claude/docs/phases.md`

**Step 1: Run full backend test suite**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/backend
go test ./...
```
Expected: all packages pass, 0 failures.

**Step 2: Run full frontend type check**
```bash
cd /Users/mohammadhass.mahdian/Projects/personal-projects/trader-claude/frontend
npx tsc --noEmit
```
Expected: no errors.

**Step 3: Mark Phase 7 complete in phases.md** — find the Phase 7 section and update all `- [ ]` checkboxes to `- [x]` and the phase header:

Change:
```markdown
## Phase 7 — News, Events & Alerts 🔲
```
To:
```markdown
## Phase 7 — News, Events & Alerts ✅
```

Change all `- [ ]` under Phase 7 to `- [x]`.

**Step 4: Commit**
```bash
git add .claude/docs/phases.md
git commit -m "docs(phase7): mark Phase 7 complete"
```

---

## Summary Table

| Task | Files | Key output |
|------|-------|-----------|
| B1 | models.go, main.go | NewsItem model + Alert extended |
| B2 | go.mod, go.sum | gofeed dependency |
| B3 | news/sentiment.go + test | Score() function |
| B4 | news/tagger.go + test | ExtractSymbols() |
| B5 | news/sources.go | fetchFeed() + DefaultFeeds |
| B6 | news/aggregator.go + test | Aggregator.Start() |
| B7 | api/news.go | listNews + newsBySymbol handlers |
| B8 | alert/evaluator.go + test | Evaluator.Start() + checkCondition() |
| B9 | api/alerts.go | CRUD alert handlers |
| B10 | api/notifications.go | List/mark-read + WS handler |
| B11 | routes.go, main.go | All routes wired, services started |
| F1 | types/index.ts | NewsItem + updated Alert types |
| F2 | api/news.ts | fetchNews + fetchNewsBySymbol |
| F3 | api/alerts.ts | Alert + Notification API calls |
| F4 | hooks/useNews.ts | useNews + useNewsBySymbol |
| F5 | hooks/useAlerts.ts | useAlerts + mutations |
| F6 | hooks/useNotifications.ts | useNotifications + WS hook |
| F7 | components/news/NewsSidePanel.tsx | News panel UI |
| F8 | pages/Chart.tsx | News toggle + panel integration |
| F9 | TopBar.tsx + Layout.tsx | Bell dropdown + WS init |
| F10 | pages/Alerts.tsx | Full alerts page + modal |
| F11 | pages/Notifications.tsx | Full notifications page |
| F12 | phases.md | Phase 7 marked complete |

**Execution order:** B1 → B2 → B3 → B4 → B5 → B6 → B7 → B8 → B9 → B10 → B11 → F1 → F2 → F3 → F4 → F5 → F6 → F7 → F8 → F9 → F10 → F11 → F12

Each task is independent of subsequent tasks within the same tier (B or F), but all B tasks must complete before starting F tasks, because the frontend types mirror the backend model changes from B1.
