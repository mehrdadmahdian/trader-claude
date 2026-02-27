# Phase 7 — News, Events & Alerts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add RSS news aggregation, price alert evaluation, and a live notification system with a news side panel on the Chart page, a full Alerts management page, and a live notification bell.

**Architecture:** Two dedicated goroutines (`news.Aggregator` on 15-min ticker, `alert.Evaluator` on 60s ticker) started in `main.go`. A Redis pubsub channel `notifications:new` bridges the evaluator to a new WebSocket endpoint. Three price conditions only (price_above, price_below, price_change_pct). RSS feeds only — no paid news APIs.

**Tech Stack:** Go 1.24, gofeed (new dep), Fiber v2, GORM, Redis pubsub, React 18, React Query, Zustand, Radix UI, Tailwind, lightweight-charts

**Design doc:** `docs/plans/2026-02-26-phase7-news-alerts-design.md`

---

## Task 1: Add NewsItem model and update autoMigrate

**Files:**
- Modify: `backend/internal/models/models.go`
- Modify: `backend/cmd/server/main.go`

**Step 1: Add NewsItem struct to models.go**

Append after the `WatchList` struct (before `ReplayBookmark`):

```go
// --- NewsItem ---

// NewsItem stores a deduplicated news article from RSS feeds
type NewsItem struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	URL         string    `gorm:"type:varchar(2048);uniqueIndex" json:"url"`
	Title       string    `gorm:"type:varchar(500)" json:"title"`
	Summary     string    `gorm:"type:text" json:"summary"`
	Source      string    `gorm:"type:varchar(100);index" json:"source"`
	PublishedAt time.Time `gorm:"index" json:"published_at"`
	Symbols     JSONArray `gorm:"type:json" json:"symbols"`
	Sentiment   string    `gorm:"type:varchar(10)" json:"sentiment"`
	FetchedAt   time.Time `json:"fetched_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (NewsItem) TableName() string { return "news_items" }
```

**Step 2: Add Alert model fields**

In `models.go`, find the `Alert` struct and make these changes:
- Remove the `AlertConditionVolume` and `AlertConditionCustom` constants
- Add 4 new fields to the `Alert` struct
- Update `Alert` struct:

```go
// AlertCondition is the type of trigger condition
type AlertCondition string

const (
	AlertConditionPriceAbove  AlertCondition = "price_above"
	AlertConditionPriceBelow  AlertCondition = "price_below"
	AlertConditionPriceChange AlertCondition = "price_change_pct"
	// TODO(future): AlertConditionRSIOverbought, AlertConditionRSIOversold
)

// Alert stores price alert definitions
type Alert struct {
	ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string         `gorm:"type:varchar(200);not null" json:"name"`
	AdapterID       string         `gorm:"type:varchar(20);not null;default:'binance'" json:"adapter_id"`
	Symbol          string         `gorm:"type:varchar(20);not null;index" json:"symbol"`
	Market          string         `gorm:"type:varchar(20);not null" json:"market"`
	Condition       AlertCondition `gorm:"type:varchar(30);not null" json:"condition"`
	Threshold       float64        `gorm:"type:decimal(20,8);not null" json:"threshold"`
	Status          AlertStatus    `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	Message         string         `gorm:"type:text" json:"message"`
	CooldownMinutes int            `gorm:"default:60" json:"cooldown_minutes"`
	LastFiredAt     *time.Time     `json:"last_fired_at,omitempty"`
	BasePrice       float64        `gorm:"type:decimal(20,8)" json:"base_price"`
	TriggeredAt     *time.Time     `json:"triggered_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}
```

**Step 3: Add NewsItem to autoMigrate in main.go**

Find the `autoMigrate` function in `backend/cmd/server/main.go` and add `&models.NewsItem{}`:

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
		&models.WatchList{},
		&models.NewsItem{},
		&models.ReplayBookmark{},
	)
}
```

**Step 4: Build to verify no compile errors**

```bash
cd backend && go build ./...
```

Expected: no output (success)

**Step 5: Commit**

```bash
git add backend/internal/models/models.go backend/cmd/server/main.go
git commit -m "feat(phase7): add NewsItem model, update Alert model with cooldown+BasePrice fields"
```

---

## Task 2: Add gofeed dependency

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`

**Step 1: Install gofeed**

```bash
cd backend && go get github.com/mmcdole/gofeed@latest
```

Expected output includes: `go: added github.com/mmcdole/gofeed ...`

**Step 2: Tidy**

```bash
cd backend && go mod tidy
```

**Step 3: Verify build still passes**

```bash
cd backend && go build ./...
```

**Step 4: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore(phase7): add gofeed RSS dependency"
```

---

## Task 3: News aggregator — sentiment scorer

**Files:**
- Create: `backend/internal/news/sentiment.go`
- Create: `backend/internal/news/sentiment_test.go`

**Step 1: Write the failing test**

Create `backend/internal/news/sentiment_test.go`:

```go
package news

import "testing"

func TestScoreSentiment(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"Bitcoin surges to new ATH, bulls rejoice", "positive"},
		{"Crypto crash wipes billions, bears dominate", "negative"},
		{"Federal Reserve holds interest rates steady", "neutral"},
		{"", "neutral"},
	}
	for _, tt := range tests {
		got := scoreSentiment(tt.text)
		if got != tt.want {
			t.Errorf("scoreSentiment(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/news/... -run TestScoreSentiment -v
```

Expected: FAIL — `no Go files in .../news`

**Step 3: Implement sentiment.go**

Create `backend/internal/news/sentiment.go`:

```go
package news

import (
	"strings"
)

var positiveWords = []string{
	"surge", "rally", "breakout", "ath", "bullish", "soar", "gain",
	"rise", "buy", "upside", "record", "high", "boost", "jump", "grows",
}

var negativeWords = []string{
	"crash", "dump", "plunge", "drop", "bear", "hack", "fraud",
	"collapse", "sell", "downside", "loss", "low", "decline", "falls", "fear",
}

// scoreSentiment returns "positive", "negative", or "neutral" based on keyword counts.
func scoreSentiment(text string) string {
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
	if pos > neg {
		return "positive"
	}
	if neg > pos {
		return "negative"
	}
	return "neutral"
}
```

**Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/news/... -run TestScoreSentiment -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/news/
git commit -m "feat(phase7): add news sentiment scorer"
```

---

## Task 4: News aggregator — symbol tagger

**Files:**
- Create: `backend/internal/news/tagger.go`
- Create: `backend/internal/news/tagger_test.go`

**Step 1: Write failing test**

Create `backend/internal/news/tagger_test.go`:

```go
package news

import (
	"testing"
)

func TestTagSymbols(t *testing.T) {
	known := []string{"BTC", "ETH", "AAPL", "MSFT"}

	tests := []struct {
		text string
		want []string
	}{
		{"BTC and ETH rally today", []string{"BTC", "ETH"}},
		{"Apple AAPL reaches new high", []string{"AAPL"}},
		{"General market news", []string{}},
		{"BTC BTC BTC", []string{"BTC"}}, // dedup
	}

	for _, tt := range tests {
		got := tagSymbols(tt.text, known)
		if len(got) != len(tt.want) {
			t.Errorf("tagSymbols(%q) = %v, want %v", tt.text, got, tt.want)
			continue
		}
		for i, s := range tt.want {
			if got[i] != s {
				t.Errorf("tagSymbols(%q)[%d] = %q, want %q", tt.text, i, got[i], s)
			}
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/news/... -run TestTagSymbols -v
```

Expected: FAIL — `tagSymbols undefined`

**Step 3: Implement tagger.go**

Create `backend/internal/news/tagger.go`:

```go
package news

import (
	"strings"
)

// tagSymbols scans text for known ticker symbols and returns matched ones (deduped, order preserved).
func tagSymbols(text string, known []string) []string {
	upper := strings.ToUpper(text)
	seen := make(map[string]bool)
	var result []string
	for _, sym := range known {
		if strings.Contains(upper, sym) && !seen[sym] {
			seen[sym] = true
			result = append(result, sym)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}
```

**Step 4: Run test**

```bash
cd backend && go test ./internal/news/... -run TestTagSymbols -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/news/
git commit -m "feat(phase7): add news symbol tagger"
```

---

## Task 5: News aggregator — RSS sources + core aggregator

**Files:**
- Create: `backend/internal/news/sources.go`
- Create: `backend/internal/news/aggregator.go`
- Create: `backend/internal/news/aggregator_test.go`

**Step 1: Create sources.go**

```go
package news

// Feed defines a single RSS feed source.
type Feed struct {
	URL    string
	Source string // display name
}

// DefaultFeeds are the RSS feeds fetched every 15 minutes.
var DefaultFeeds = []Feed{
	{URL: "https://www.coindesk.com/arc/outboundfeeds/rss/", Source: "coindesk"},
	{URL: "https://cointelegraph.com/rss", Source: "cointelegraph"},
	{URL: "https://feeds.reuters.com/reuters/businessNews", Source: "reuters"},
}
```

**Step 2: Write failing test for dedup logic**

Create `backend/internal/news/aggregator_test.go`:

```go
package news

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NewsItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSaveItem_Dedup(t *testing.T) {
	db := setupTestDB(t)
	agg := NewAggregator(db, nil, DefaultFeeds)

	item := models.NewsItem{
		URL:         "https://example.com/article-1",
		Title:       "BTC Surges",
		Source:      "test",
		PublishedAt: time.Now(),
		Symbols:     models.JSONArray{"BTC"},
		Sentiment:   "positive",
		FetchedAt:   time.Now(),
	}

	// First save succeeds
	if err := agg.saveItem(context.Background(), item); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Count = 1
	var count int64
	db.Model(&models.NewsItem{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 item, got %d", count)
	}

	// Second save same URL is silently skipped (no error, still 1 row)
	if err := agg.saveItem(context.Background(), item); err != nil {
		t.Fatalf("second save: %v", err)
	}

	db.Model(&models.NewsItem{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 item after dedup, got %d", count)
	}
}
```

**Step 3: Run test to verify it fails**

```bash
cd backend && go test ./internal/news/... -run TestSaveItem_Dedup -v
```

Expected: FAIL — `NewAggregator undefined`

**Step 4: Implement aggregator.go**

Create `backend/internal/news/aggregator.go`:

```go
package news

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// Aggregator fetches RSS feeds periodically and persists new articles.
type Aggregator struct {
	db      *gorm.DB
	symbols []string // known tickers for symbol tagging
	feeds   []Feed
	parser  *gofeed.Parser
}

// NewAggregator creates an Aggregator. Pass nil symbols to skip tagging in tests.
func NewAggregator(db *gorm.DB, symbols []string, feeds []Feed) *Aggregator {
	return &Aggregator{
		db:      db,
		symbols: symbols,
		feeds:   feeds,
		parser:  gofeed.NewParser(),
	}
}

// Start runs the aggregator loop. Call in a goroutine. Respects ctx cancellation.
func (a *Aggregator) Start(ctx context.Context) {
	log.Println("news: aggregator started")
	a.runOnce(ctx)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("news: aggregator stopped")
			return
		case <-ticker.C:
			a.runOnce(ctx)
		}
	}
}

// LoadSymbols refreshes the known ticker list from the DB.
func (a *Aggregator) LoadSymbols(ctx context.Context) {
	var symbols []models.Symbol
	a.db.WithContext(ctx).Find(&symbols)
	tickers := make([]string, 0, len(symbols))
	for _, s := range symbols {
		tickers = append(tickers, s.Ticker)
	}
	a.symbols = tickers
}

func (a *Aggregator) runOnce(ctx context.Context) {
	inserted, skipped := 0, 0
	for _, feed := range a.feeds {
		items, err := a.fetchFeed(ctx, feed)
		if err != nil {
			log.Printf("news: fetch %s error: %v", feed.Source, err)
			continue
		}
		for _, item := range items {
			if err := a.saveItem(ctx, item); err != nil {
				log.Printf("news: save error: %v", err)
				skipped++
			} else {
				inserted++
			}
		}
	}
	log.Printf("news: cycle done — inserted=%d skipped=%d", inserted, skipped)
}

func (a *Aggregator) fetchFeed(ctx context.Context, feed Feed) ([]models.NewsItem, error) {
	parsed, err := a.parser.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var items []models.NewsItem
	for _, entry := range parsed.Items {
		pub := now
		if entry.PublishedParsed != nil {
			pub = *entry.PublishedParsed
		}
		text := entry.Title + " " + entry.Description
		items = append(items, models.NewsItem{
			URL:         entry.Link,
			Title:       entry.Title,
			Summary:     entry.Description,
			Source:      feed.Source,
			PublishedAt: pub,
			Symbols:     toJSONArray(tagSymbols(text, a.symbols)),
			Sentiment:   scoreSentiment(text),
			FetchedAt:   now,
		})
	}
	return items, nil
}

// saveItem inserts a NewsItem, silently skipping duplicates (same URL).
func (a *Aggregator) saveItem(ctx context.Context, item models.NewsItem) error {
	result := a.db.WithContext(ctx).
		Where("url = ?", item.URL).
		FirstOrCreate(&item)
	return result.Error
}

func toJSONArray(s []string) models.JSONArray {
	arr := make(models.JSONArray, len(s))
	for i, v := range s {
		arr[i] = v
	}
	return arr
}
```

**Step 5: Run test**

```bash
cd backend && go test ./internal/news/... -run TestSaveItem_Dedup -v
```

Expected: PASS

**Step 6: Run all news tests**

```bash
cd backend && go test ./internal/news/... -v
```

Expected: all PASS

**Step 7: Commit**

```bash
git add backend/internal/news/
git commit -m "feat(phase7): add news aggregator with RSS fetch, dedup, tagging, sentiment"
```

---

## Task 6: News API handler + routes

**Files:**
- Create: `backend/internal/api/news.go`
- Create: `backend/internal/api/news_test.go`
- Modify: `backend/internal/api/routes.go`

**Step 1: Write failing test**

Create `backend/internal/api/news_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/models"
)

func setupNewsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&models.NewsItem{})
	return db
}

func TestNewsHandler_ListNews(t *testing.T) {
	db := setupNewsDB(t)
	db.Create(&models.NewsItem{
		URL:         "https://example.com/1",
		Title:       "BTC ATH",
		Source:      "coindesk",
		PublishedAt: time.Now(),
		Symbols:     models.JSONArray{"BTC"},
		Sentiment:   "positive",
		FetchedAt:   time.Now(),
	})

	app := fiber.New()
	nh := newNewsHandler(db)
	app.Get("/api/v1/news", nh.list)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/news", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data []models.NewsItem `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Data) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Data))
	}
}

func TestNewsHandler_BySymbol(t *testing.T) {
	db := setupNewsDB(t)
	db.Create(&models.NewsItem{
		URL:         "https://example.com/2",
		Title:       "ETH Update",
		Source:      "reuters",
		PublishedAt: time.Now(),
		Symbols:     models.JSONArray{"ETH"},
		Sentiment:   "neutral",
		FetchedAt:   time.Now(),
	})

	app := fiber.New()
	nh := newNewsHandler(db)
	app.Get("/api/v1/news/symbols/:symbol", nh.bySymbol)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/news/symbols/ETH", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/api/... -run TestNewsHandler -v
```

Expected: FAIL — `newNewsHandler undefined`

**Step 3: Implement news.go**

Create `backend/internal/api/news.go`:

```go
package api

import (
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

// list returns paginated news, optionally filtered by symbols.
// GET /api/v1/news?symbols=BTC,ETH&limit=20&offset=0&from=ISO&to=ISO
func (h *newsHandler) list(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)

	q := h.db.Order("published_at DESC").Limit(limit).Offset(offset)

	if symbols := c.Query("symbols"); symbols != "" {
		// JSON_CONTAINS works in MySQL; for SQLite tests this does a LIKE fallback
		q = q.Where("JSON_CONTAINS(symbols, JSON_ARRAY(?)) OR symbols LIKE ?",
			symbols, "%"+symbols+"%")
	}
	if from := c.Query("from"); from != "" {
		q = q.Where("published_at >= ?", from)
	}
	if to := c.Query("to"); to != "" {
		q = q.Where("published_at <= ?", to)
	}

	var items []models.NewsItem
	if err := q.Find(&items).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var total int64
	h.db.Model(&models.NewsItem{}).Count(&total)

	return c.JSON(fiber.Map{
		"data":      items,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// bySymbol returns the latest 50 news items tagged with a specific symbol.
// GET /api/v1/news/symbols/:symbol
func (h *newsHandler) bySymbol(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	var items []models.NewsItem
	h.db.Where("JSON_CONTAINS(symbols, JSON_QUOTE(?)) OR symbols LIKE ?",
		symbol, "%"+symbol+"%").
		Order("published_at DESC").
		Limit(50).
		Find(&items)
	return c.JSON(fiber.Map{"data": items})
}
```

**Step 4: Wire into routes.go**

In `backend/internal/api/routes.go`, add the news handler in `RegisterRoutes`. Find the line where `priceSvc` is declared and add after it:

```go
// --- News ---
nh := newNewsHandler(db)
v1.Get("/news", nh.list)
v1.Get("/news/symbols/:symbol", nh.bySymbol)
```

Also add `db` to the import if needed (already in scope from portfolio handler).

**Step 5: Run test**

```bash
cd backend && go test ./internal/api/... -run TestNewsHandler -v
```

Expected: PASS

**Step 6: Build**

```bash
cd backend && go build ./...
```

**Step 7: Commit**

```bash
git add backend/internal/api/news.go backend/internal/api/news_test.go backend/internal/api/routes.go
git commit -m "feat(phase7): add news API handler and routes"
```

---

## Task 7: Alert evaluator

**Files:**
- Create: `backend/internal/alert/evaluator.go`
- Create: `backend/internal/alert/evaluator_test.go`

**Step 1: Write failing tests**

Create `backend/internal/alert/evaluator_test.go`:

```go
package alert

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/models"
)

func setupAlertDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&models.Alert{}, &models.Notification{})
	return db
}

// mockPricer always returns a fixed price
type mockPricer struct{ price float64 }

func (m *mockPricer) GetPrice(_ context.Context, _, _ string) (float64, error) {
	return m.price, nil
}

func TestEvaluate_PriceAbove_Fires(t *testing.T) {
	db := setupAlertDB(t)
	alert := models.Alert{
		Name:            "BTC above 50k",
		AdapterID:       "binance",
		Symbol:          "BTCUSDT",
		Market:          "crypto",
		Condition:       models.AlertConditionPriceAbove,
		Threshold:       50000,
		Status:          models.AlertStatusActive,
		CooldownMinutes: 60,
	}
	db.Create(&alert)

	pricer := &mockPricer{price: 51000}
	eval := NewEvaluator(db, nil, pricer)
	eval.evaluateOnce(context.Background())

	// Notification should be created
	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 notification, got %d", count)
	}

	// LastFiredAt should be set
	var updated models.Alert
	db.First(&updated, alert.ID)
	if updated.LastFiredAt == nil {
		t.Error("expected LastFiredAt to be set")
	}
}

func TestEvaluate_PriceBelow_Fires(t *testing.T) {
	db := setupAlertDB(t)
	alert := models.Alert{
		Name:            "BTC below 30k",
		AdapterID:       "binance",
		Symbol:          "BTCUSDT",
		Market:          "crypto",
		Condition:       models.AlertConditionPriceBelow,
		Threshold:       30000,
		Status:          models.AlertStatusActive,
		CooldownMinutes: 60,
	}
	db.Create(&alert)

	pricer := &mockPricer{price: 29000}
	eval := NewEvaluator(db, nil, pricer)
	eval.evaluateOnce(context.Background())

	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 notification, got %d", count)
	}
}

func TestEvaluate_Cooldown_Prevents_Refiring(t *testing.T) {
	db := setupAlertDB(t)
	recent := time.Now().Add(-30 * time.Minute) // fired 30 min ago
	alert := models.Alert{
		Name:            "BTC above 50k cooldown",
		AdapterID:       "binance",
		Symbol:          "BTCUSDT",
		Market:          "crypto",
		Condition:       models.AlertConditionPriceAbove,
		Threshold:       50000,
		Status:          models.AlertStatusActive,
		CooldownMinutes: 60,
		LastFiredAt:     &recent,
	}
	db.Create(&alert)

	pricer := &mockPricer{price: 55000}
	eval := NewEvaluator(db, nil, pricer)
	eval.evaluateOnce(context.Background())

	// Should NOT fire because cooldown not elapsed
	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 notifications (cooldown), got %d", count)
	}
}

func TestEvaluate_PriceChangePct_InitializesBase(t *testing.T) {
	db := setupAlertDB(t)
	alert := models.Alert{
		Name:            "BTC change 5%",
		AdapterID:       "binance",
		Symbol:          "BTCUSDT",
		Market:          "crypto",
		Condition:       models.AlertConditionPriceChange,
		Threshold:       5.0, // 5%
		Status:          models.AlertStatusActive,
		CooldownMinutes: 60,
		BasePrice:       0, // not set yet
	}
	db.Create(&alert)

	pricer := &mockPricer{price: 40000}
	eval := NewEvaluator(db, nil, pricer)
	eval.evaluateOnce(context.Background())

	// Should set BasePrice, NOT fire
	var updated models.Alert
	db.First(&updated, alert.ID)
	if updated.BasePrice != 40000 {
		t.Errorf("expected BasePrice=40000, got %f", updated.BasePrice)
	}

	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 notifications on init, got %d", count)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/alert/... -v
```

Expected: FAIL — `NewEvaluator undefined`

**Step 3: Implement evaluator.go**

Create `backend/internal/alert/evaluator.go`:

```go
package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// PriceFetcher is the minimal interface needed to evaluate price conditions.
type PriceFetcher interface {
	GetPrice(ctx context.Context, adapterID, symbol string) (float64, error)
}

// Evaluator runs price alert checks every 60 seconds.
type Evaluator struct {
	db     *gorm.DB
	rdb    *redis.Client
	pricer PriceFetcher
}

// NewEvaluator creates an Evaluator. rdb may be nil (notifications still persist to DB).
func NewEvaluator(db *gorm.DB, rdb *redis.Client, pricer PriceFetcher) *Evaluator {
	return &Evaluator{db: db, rdb: rdb, pricer: pricer}
}

// Start runs the evaluation loop. Call in a goroutine. Respects ctx cancellation.
func (e *Evaluator) Start(ctx context.Context) {
	log.Println("alerts: evaluator started")
	e.evaluateOnce(ctx)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("alerts: evaluator stopped")
			return
		case <-ticker.C:
			e.evaluateOnce(ctx)
		}
	}
}

func (e *Evaluator) evaluateOnce(ctx context.Context) {
	var alerts []models.Alert
	e.db.WithContext(ctx).Where("status = ?", models.AlertStatusActive).Find(&alerts)

	for _, a := range alerts {
		if err := e.evaluate(ctx, a); err != nil {
			log.Printf("alerts: evaluate #%d error: %v", a.ID, err)
		}
	}
}

func (e *Evaluator) evaluate(ctx context.Context, a models.Alert) error {
	// Check cooldown
	if a.LastFiredAt != nil {
		elapsed := time.Since(*a.LastFiredAt)
		cooldown := time.Duration(a.CooldownMinutes) * time.Minute
		if elapsed < cooldown {
			return nil
		}
	}

	currentPrice, err := e.pricer.GetPrice(ctx, a.AdapterID, a.Symbol)
	if err != nil {
		return fmt.Errorf("price fetch: %w", err)
	}

	var shouldFire bool

	switch a.Condition {
	case models.AlertConditionPriceAbove:
		shouldFire = currentPrice > a.Threshold

	case models.AlertConditionPriceBelow:
		shouldFire = currentPrice < a.Threshold

	case models.AlertConditionPriceChange:
		if a.BasePrice == 0 {
			// First tick: initialize base price, don't fire
			e.db.WithContext(ctx).Model(&a).Update("base_price", currentPrice)
			return nil
		}
		changePct := math.Abs((currentPrice-a.BasePrice)/a.BasePrice*100)
		shouldFire = changePct >= a.Threshold
	}

	if !shouldFire {
		return nil
	}

	return e.fire(ctx, a, currentPrice)
}

func (e *Evaluator) fire(ctx context.Context, a models.Alert, currentPrice float64) error {
	now := time.Now()
	body := fmt.Sprintf("%s %s: current price %.8f (threshold %.8f)", a.Symbol, a.Condition, currentPrice, a.Threshold)
	if a.Message != "" {
		body = a.Message
	}

	notif := models.Notification{
		Type:      models.NotificationTypeAlert,
		Title:     a.Name,
		Body:      body,
		Read:      false,
		CreatedAt: now,
	}
	if err := e.db.WithContext(ctx).Create(&notif).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	// Update alert
	updates := map[string]interface{}{
		"last_fired_at": now,
		"triggered_at":  now,
	}
	e.db.WithContext(ctx).Model(&a).Updates(updates)

	// Publish to Redis for WS delivery
	if e.rdb != nil {
		b, _ := json.Marshal(notif)
		e.rdb.Publish(ctx, "notifications:new", string(b))
	}

	log.Printf("alerts: fired #%d (%s %s)", a.ID, a.Symbol, a.Condition)
	return nil
}
```

**Step 4: Run tests**

```bash
cd backend && go test ./internal/alert/... -v
```

Expected: all PASS

**Step 5: Build**

```bash
cd backend && go build ./...
```

**Step 6: Commit**

```bash
git add backend/internal/alert/
git commit -m "feat(phase7): add alert evaluator with price conditions and cooldown"
```

---

## Task 8: Alert + Notification API handler

**Files:**
- Create: `backend/internal/api/alert.go`
- Create: `backend/internal/api/alert_test.go`
- Modify: `backend/internal/api/routes.go`

**Step 1: Write failing test**

Create `backend/internal/api/alert_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/trader-claude/backend/internal/models"
)

func setupAlertAPIDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&models.Alert{}, &models.Notification{})
	return db
}

func TestAlertHandler_CreateAndList(t *testing.T) {
	db := setupAlertAPIDB(t)
	app := fiber.New()
	ah := newAlertHandler(db)
	app.Post("/api/v1/alerts", ah.create)
	app.Get("/api/v1/alerts", ah.list)

	// Create alert
	body := map[string]interface{}{
		"name":             "BTC Alert",
		"adapter_id":       "binance",
		"symbol":           "BTCUSDT",
		"market":           "crypto",
		"condition":        "price_above",
		"threshold":        50000.0,
		"cooldown_minutes": 60,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("create: expected 201, got %d", resp.StatusCode)
	}

	// List
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("list: expected 200, got %d", resp2.StatusCode)
	}
	var result struct {
		Data []models.Alert `json:"data"`
	}
	json.NewDecoder(resp2.Body).Decode(&result)
	if len(result.Data) != 1 {
		t.Errorf("expected 1 alert, got %d", len(result.Data))
	}
}

func TestAlertHandler_Toggle(t *testing.T) {
	db := setupAlertAPIDB(t)
	alert := models.Alert{
		Name: "test", AdapterID: "binance", Symbol: "BTCUSDT", Market: "crypto",
		Condition: models.AlertConditionPriceAbove, Threshold: 1, Status: models.AlertStatusActive,
	}
	db.Create(&alert)

	app := fiber.New()
	ah := newAlertHandler(db)
	app.Patch("/api/v1/alerts/:id/toggle", ah.toggle)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/alerts/1/toggle", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("toggle: expected 200, got %d", resp.StatusCode)
	}

	var updated models.Alert
	db.First(&updated, alert.ID)
	if updated.Status != models.AlertStatusDisabled {
		t.Errorf("expected disabled, got %s", updated.Status)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/api/... -run TestAlertHandler -v
```

Expected: FAIL

**Step 3: Implement alert.go**

Create `backend/internal/api/alert.go`:

```go
package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type alertHandler struct {
	db *gorm.DB
}

func newAlertHandler(db *gorm.DB) *alertHandler {
	return &alertHandler{db: db}
}

func (h *alertHandler) registerRoutes(v1 fiber.Router) {
	v1.Post("/alerts", h.create)
	v1.Get("/alerts", h.list)
	v1.Delete("/alerts/:id", h.delete)
	v1.Patch("/alerts/:id/toggle", h.toggle)

	v1.Get("/notifications", h.listNotifications)
	v1.Patch("/notifications/:id/read", h.markRead)
	v1.Post("/notifications/read-all", h.markAllRead)
}

// create POST /api/v1/alerts
func (h *alertHandler) create(c *fiber.Ctx) error {
	var req struct {
		Name            string                `json:"name"`
		AdapterID       string                `json:"adapter_id"`
		Symbol          string                `json:"symbol"`
		Market          string                `json:"market"`
		Condition       models.AlertCondition `json:"condition"`
		Threshold       float64               `json:"threshold"`
		Message         string                `json:"message"`
		CooldownMinutes int                   `json:"cooldown_minutes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.CooldownMinutes == 0 {
		req.CooldownMinutes = 60
	}
	if req.AdapterID == "" {
		req.AdapterID = "binance"
	}
	alert := models.Alert{
		Name:            req.Name,
		AdapterID:       req.AdapterID,
		Symbol:          req.Symbol,
		Market:          req.Market,
		Condition:       req.Condition,
		Threshold:       req.Threshold,
		Message:         req.Message,
		CooldownMinutes: req.CooldownMinutes,
		Status:          models.AlertStatusActive,
	}
	if err := h.db.Create(&alert).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(alert)
}

// list GET /api/v1/alerts
func (h *alertHandler) list(c *fiber.Ctx) error {
	var alerts []models.Alert
	h.db.Order("created_at DESC").Find(&alerts)
	return c.JSON(fiber.Map{"data": alerts})
}

// delete DELETE /api/v1/alerts/:id
func (h *alertHandler) delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	h.db.Delete(&models.Alert{}, id)
	return c.SendStatus(fiber.StatusNoContent)
}

// toggle PATCH /api/v1/alerts/:id/toggle
func (h *alertHandler) toggle(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var a models.Alert
	if err := h.db.First(&a, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	newStatus := models.AlertStatusActive
	if a.Status == models.AlertStatusActive {
		newStatus = models.AlertStatusDisabled
	}
	h.db.Model(&a).Update("status", newStatus)
	a.Status = newStatus
	return c.JSON(a)
}

// listNotifications GET /api/v1/notifications
func (h *alertHandler) listNotifications(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	var notifs []models.Notification
	h.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&notifs)
	var total int64
	h.db.Model(&models.Notification{}).Count(&total)
	return c.JSON(fiber.Map{"data": notifs, "total": total})
}

// markRead PATCH /api/v1/notifications/:id/read
func (h *alertHandler) markRead(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	h.db.Model(&models.Notification{}).Where("id = ?", id).Update("read", true)
	return c.SendStatus(fiber.StatusNoContent)
}

// markAllRead POST /api/v1/notifications/read-all
func (h *alertHandler) markAllRead(c *fiber.Ctx) error {
	h.db.Model(&models.Notification{}).Where("read = false").Update("read", true)
	return c.SendStatus(fiber.StatusNoContent)
}
```

**Step 4: Update routes.go**

Replace the stub alert and notification routes in `routes.go`:

Remove:
```go
// --- Alerts ---
v1.Get("/alerts", func(c *fiber.Ctx) error { ... })
v1.Post("/alerts", func(c *fiber.Ctx) error { ... })
v1.Delete("/alerts/:id", func(c *fiber.Ctx) error { ... })

// --- Notifications ---
v1.Get("/notifications", func(c *fiber.Ctx) error { ... })
v1.Patch("/notifications/:id/read", func(c *fiber.Ctx) error { ... })
```

Replace with:
```go
// --- Alerts + Notifications ---
ah := newAlertHandler(db)
ah.registerRoutes(v1)
```

**Step 5: Run tests**

```bash
cd backend && go test ./internal/api/... -run TestAlertHandler -v
```

Expected: PASS

**Step 6: Build**

```bash
cd backend && go build ./...
```

**Step 7: Commit**

```bash
git add backend/internal/api/alert.go backend/internal/api/alert_test.go backend/internal/api/routes.go
git commit -m "feat(phase7): add alert + notification CRUD API handler, replace stub routes"
```

---

## Task 9: WebSocket notifications handler

**Files:**
- Create: `backend/internal/api/notification_ws.go`
- Modify: `backend/internal/api/routes.go`

**Step 1: Implement notification_ws.go**

Create `backend/internal/api/notification_ws.go`:

```go
package api

import (
	"context"
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

// notificationsWS streams new notification JSON to connected clients via Redis pubsub.
// WS /ws/notifications
func notificationsWS(rdb *redis.Client) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sub := rdb.Subscribe(ctx, "notifications:new")
		defer sub.Close()

		ch := sub.Channel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
					log.Printf("notificationsWS: write error: %v", err)
					return
				}
			}
		}
	}
}
```

**Step 2: Add route in routes.go**

After the `app.Get("/ws/replay/:replay_id", ...)` line, add:

```go
// Notifications WebSocket
app.Get("/ws/notifications", websocket.New(notificationsWS(rdb)))
```

**Step 3: Build**

```bash
cd backend && go build ./...
```

**Step 4: Commit**

```bash
git add backend/internal/api/notification_ws.go backend/internal/api/routes.go
git commit -m "feat(phase7): add WS /ws/notifications Redis pubsub bridge"
```

---

## Task 10: Wire aggregator + evaluator into main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

**Step 1: Update main.go**

Add imports for the new packages and start the aggregator/evaluator goroutines. In `main.go`:

1. Add imports:
```go
"github.com/trader-claude/backend/internal/alert"
"github.com/trader-claude/backend/internal/news"
"github.com/trader-claude/backend/internal/price"
```

2. After `pool.Start()` and before `replayMgr := replay.NewManager()`, add:

```go
// Start news aggregator
newsAgg := news.NewAggregator(db, nil, news.DefaultFeeds)
newsAgg.LoadSymbols(context.Background())
go newsAgg.Start(context.Background())

// Start alert evaluator
priceSvcForAlerts := price.NewService(rdb, "", "")
alertEval := alert.NewEvaluator(db, rdb, priceSvcForAlerts)
go alertEval.Start(context.Background())
```

Note: The `price.NewService` in routes.go passes empty strings for URLs (uses production defaults). The alert evaluator creates its own instance the same way.

**Step 2: Build**

```bash
cd backend && go build ./...
```

**Step 3: Run all backend tests**

```bash
cd backend && go test ./... -v 2>&1 | tail -30
```

Expected: all PASS

**Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(phase7): wire news aggregator and alert evaluator into main.go startup"
```

---

## Task 11: Frontend — News types, API client, React Query hooks

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/api/news.ts`
- Create: `frontend/src/hooks/useNews.ts`

**Step 1: Add NewsItem type to types/index.ts**

Append after the `WatchList` interface:

```ts
// ── News types (Phase 7) ────────────────────────────────────────────────────

export type NewsSentiment = 'positive' | 'negative' | 'neutral'

export interface NewsItem {
  id: number
  url: string
  title: string
  summary: string
  source: string
  published_at: string
  symbols: string[]
  sentiment: NewsSentiment
  fetched_at: string
  created_at: string
}

export interface NewsListResponse {
  data: NewsItem[]
  total: number
  limit: number
  offset: number
}
```

Also update `Alert` interface — add new fields:

```ts
export interface Alert {
  id: number
  name: string
  adapter_id: string
  symbol: string
  market: string
  condition: AlertCondition
  threshold: number
  status: AlertStatus
  message: string
  cooldown_minutes: number
  last_fired_at?: string
  base_price: number
  triggered_at?: string
  created_at: string
  updated_at: string
}

export interface AlertCreateRequest {
  name: string
  adapter_id: string
  symbol: string
  market: string
  condition: AlertCondition
  threshold: number
  message?: string
  cooldown_minutes?: number
}
```

**Step 2: Create news API client**

Create `frontend/src/api/news.ts`:

```ts
import api from './client'
import type { NewsItem, NewsListResponse } from '@/types'

export async function fetchNews(params?: {
  symbols?: string
  limit?: number
  offset?: number
  from?: string
  to?: string
}): Promise<NewsListResponse> {
  const { data } = await api.get<NewsListResponse>('/api/v1/news', { params })
  return data
}

export async function fetchNewsBySymbol(symbol: string): Promise<NewsItem[]> {
  const { data } = await api.get<{ data: NewsItem[] }>(`/api/v1/news/symbols/${symbol}`)
  return data.data
}
```

**Step 3: Create React Query hooks**

Create `frontend/src/hooks/useNews.ts`:

```ts
import { useQuery } from '@tanstack/react-query'
import { fetchNews, fetchNewsBySymbol } from '@/api/news'

export function useNews(params?: {
  symbols?: string
  limit?: number
  offset?: number
}) {
  return useQuery({
    queryKey: ['news', params],
    queryFn: () => fetchNews(params),
    refetchInterval: 5 * 60 * 1000, // refetch every 5 min
  })
}

export function useNewsBySymbol(symbol: string | null) {
  return useQuery({
    queryKey: ['news', 'symbol', symbol],
    queryFn: () => fetchNewsBySymbol(symbol!),
    enabled: !!symbol,
    refetchInterval: 5 * 60 * 1000,
  })
}
```

**Step 4: Build frontend**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Expected: no TypeScript errors

**Step 5: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/api/news.ts frontend/src/hooks/useNews.ts
git commit -m "feat(phase7): add NewsItem types, news API client, React Query hooks"
```

---

## Task 12: Frontend — Alert API client + WS hook

**Files:**
- Create: `frontend/src/api/alerts.ts`
- Create: `frontend/src/hooks/useNotificationsWS.ts`

**Step 1: Create alerts API client**

Create `frontend/src/api/alerts.ts`:

```ts
import api from './client'
import type { Alert, AlertCreateRequest, Notification } from '@/types'

export async function fetchAlerts(): Promise<Alert[]> {
  const { data } = await api.get<{ data: Alert[] }>('/api/v1/alerts')
  return data.data
}

export async function createAlert(req: AlertCreateRequest): Promise<Alert> {
  const { data } = await api.post<Alert>('/api/v1/alerts', req)
  return data
}

export async function deleteAlert(id: number): Promise<void> {
  await api.delete(`/api/v1/alerts/${id}`)
}

export async function toggleAlert(id: number): Promise<Alert> {
  const { data } = await api.patch<Alert>(`/api/v1/alerts/${id}/toggle`)
  return data
}

export async function fetchNotifications(params?: {
  limit?: number
  offset?: number
}): Promise<{ data: Notification[]; total: number }> {
  const { data } = await api.get<{ data: Notification[]; total: number }>('/api/v1/notifications', { params })
  return data
}

export async function markNotificationRead(id: number): Promise<void> {
  await api.patch(`/api/v1/notifications/${id}/read`)
}

export async function markAllNotificationsRead(): Promise<void> {
  await api.post('/api/v1/notifications/read-all')
}
```

**Step 2: Create notifications WS hook**

Create `frontend/src/hooks/useNotificationsWS.ts`:

```ts
import { useEffect, useRef } from 'react'
import { useNotificationStore } from '@/stores'
import type { Notification } from '@/types'

const WS_URL = (import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080') + '/ws/notifications'

/**
 * Opens a WebSocket to /ws/notifications and pipes incoming notifications
 * into the Zustand notificationStore. Call once at the app root.
 */
export function useNotificationsWS() {
  const addNotification = useNotificationStore((s) => s.addNotification)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    let reconnectTimer: ReturnType<typeof setTimeout>

    function connect() {
      const ws = new WebSocket(WS_URL)
      wsRef.current = ws

      ws.onmessage = (event) => {
        try {
          const notif: Notification = JSON.parse(event.data)
          addNotification(notif)
        } catch {
          // ignore malformed messages
        }
      }

      ws.onclose = () => {
        reconnectTimer = setTimeout(connect, 5000)
      }
    }

    connect()

    return () => {
      clearTimeout(reconnectTimer)
      wsRef.current?.close()
    }
  }, [addNotification])
}
```

**Step 3: Wire hook into App.tsx**

In `frontend/src/App.tsx`, import and call `useNotificationsWS()` at the top level of the `App` component:

```tsx
import { useNotificationsWS } from '@/hooks/useNotificationsWS'

export default function App() {
  useNotificationsWS()
  // ... rest of existing code
}
```

**Step 4: Build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

**Step 5: Commit**

```bash
git add frontend/src/api/alerts.ts frontend/src/hooks/useNotificationsWS.ts frontend/src/App.tsx
git commit -m "feat(phase7): add alert API client, notifications WS hook, wire into App"
```

---

## Task 13: Frontend — News side panel component

**Files:**
- Create: `frontend/src/components/news/NewsSidePanel.tsx`

**Step 1: Create NewsSidePanel.tsx**

Create `frontend/src/components/news/NewsSidePanel.tsx`:

```tsx
import { X, ExternalLink } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { useNewsBySymbol } from '@/hooks/useNews'
import type { NewsItem, NewsSentiment } from '@/types'

interface Props {
  symbol: string | null
  onClose: () => void
}

const SOURCE_COLORS: Record<string, string> = {
  coindesk:      'bg-orange-500/20 text-orange-400',
  cointelegraph: 'bg-blue-500/20 text-blue-400',
  reuters:       'bg-red-500/20 text-red-400',
}

function SentimentDot({ sentiment }: { sentiment: NewsSentiment }) {
  const color =
    sentiment === 'positive' ? 'bg-green-500' :
    sentiment === 'negative' ? 'bg-red-500'   : 'bg-gray-500'
  return <span className={`inline-block w-2 h-2 rounded-full ${color} shrink-0`} title={sentiment} />
}

function NewsCard({ item }: { item: NewsItem }) {
  const sourceColor = SOURCE_COLORS[item.source] ?? 'bg-muted text-muted-foreground'
  const timeAgo = formatDistanceToNow(new Date(item.published_at), { addSuffix: true })

  return (
    <a
      href={item.url}
      target="_blank"
      rel="noopener noreferrer"
      className="block p-3 border-b border-border hover:bg-accent/50 transition-colors group"
    >
      <div className="flex items-start gap-2 mb-1">
        <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded shrink-0 ${sourceColor}`}>
          {item.source}
        </span>
        <SentimentDot sentiment={item.sentiment} />
        <span className="text-[10px] text-muted-foreground ml-auto shrink-0">{timeAgo}</span>
      </div>
      <p className="text-sm font-medium leading-snug line-clamp-2 group-hover:text-foreground">
        {item.title}
      </p>
      <ExternalLink className="w-3 h-3 text-muted-foreground mt-1 opacity-0 group-hover:opacity-100 transition-opacity" />
    </a>
  )
}

export function NewsSidePanel({ symbol, onClose }: Props) {
  const { data, isLoading } = useNewsBySymbol(symbol)
  const items = data ?? []

  return (
    <div className="flex flex-col h-full w-80 border-l border-border bg-card">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-border shrink-0">
        <span className="text-sm font-semibold">
          Market News {symbol ? `· ${symbol}` : ''}
        </span>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Feed */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <p className="text-sm text-muted-foreground p-4">Loading news…</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground p-4">No news found for {symbol ?? 'this symbol'}.</p>
        ) : (
          items.map((item) => <NewsCard key={item.id} item={item} />)
        )}
      </div>
    </div>
  )
}
```

**Step 2: Build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

**Step 3: Commit**

```bash
git add frontend/src/components/news/
git commit -m "feat(phase7): add NewsSidePanel component"
```

---

## Task 14: Frontend — Integrate news panel into Chart page

**Files:**
- Modify: `frontend/src/pages/Chart.tsx`

**Step 1: Add news panel state and toggle to Chart.tsx**

In `Chart.tsx`, the component is large. Make these additions:

1. Add import at top:
```tsx
import { NewsSidePanel } from '@/components/news/NewsSidePanel'
import { Newspaper } from 'lucide-react'
```

2. Add state inside `Chart()`:
```tsx
const [showNews, setShowNews] = useState(false)
```

3. Add "News" toggle button in the chart toolbar (find the area where `<IndicatorChips>` is rendered, add a button next to it):
```tsx
<button
  onClick={() => setShowNews((v) => !v)}
  className={cn(
    'flex items-center gap-1.5 px-3 py-1.5 rounded text-sm border transition-colors',
    showNews
      ? 'bg-accent border-accent-foreground/20 text-foreground'
      : 'border-border text-muted-foreground hover:text-foreground hover:bg-accent/50'
  )}
>
  <Newspaper className="w-4 h-4" />
  News
</button>
```

4. Wrap the main chart area in a flex container to accommodate the side panel. Find the div that contains `<CandlestickChart>` and `<PanelChart>` sections. Wrap that content with:
```tsx
<div className="flex flex-1 overflow-hidden">
  <div className="flex flex-col flex-1 overflow-hidden">
    {/* existing chart content */}
  </div>
  {showNews && (
    <NewsSidePanel
      symbol={selectedSymbol}
      onClose={() => setShowNews(false)}
    />
  )}
</div>
```

**Step 2: Build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Expected: no errors

**Step 3: Commit**

```bash
git add frontend/src/pages/Chart.tsx
git commit -m "feat(phase7): integrate news side panel toggle into Chart page"
```

---

## Task 15: Frontend — TopBar notification bell dropdown

**Files:**
- Modify: `frontend/src/components/layout/TopBar.tsx`

**Step 1: Rewrite TopBar.tsx with notification dropdown**

Replace the current bell button with a functional dropdown. The existing `@radix-ui/react-dropdown-menu` package is already installed.

```tsx
import { useState } from 'react'
import { Menu, Moon, Sun, Bell, Check, CheckCheck } from 'lucide-react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { formatDistanceToNow } from 'date-fns'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useThemeStore, useNotificationStore, useSidebarStore } from '@/stores'
import { fetchNotifications, markNotificationRead, markAllNotificationsRead } from '@/api/alerts'
import { cn } from '@/lib/utils'

export function TopBar() {
  const { theme, toggleTheme } = useThemeStore()
  const { unreadCount, setNotifications, markRead, markAllRead } = useNotificationStore()
  const { toggle } = useSidebarStore()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)

  const { data } = useQuery({
    queryKey: ['notifications', 'recent'],
    queryFn: () => fetchNotifications({ limit: 5 }),
    enabled: open,
  })

  const markReadMutation = useMutation({
    mutationFn: markNotificationRead,
    onSuccess: (_, id) => {
      markRead(id)
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })

  const markAllMutation = useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      markAllRead()
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })

  return (
    <header className="h-16 border-b border-border bg-card flex items-center gap-4 px-4 shrink-0">
      <button
        onClick={toggle}
        className="lg:hidden p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label="Toggle sidebar"
      >
        <Menu className="w-5 h-5" />
      </button>

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
      <DropdownMenu.Root open={open} onOpenChange={setOpen}>
        <DropdownMenu.Trigger asChild>
          <button
            className="relative p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
            aria-label="Notifications"
          >
            <Bell className="w-5 h-5" />
            {unreadCount > 0 && (
              <span className={cn(
                'absolute top-1 right-1 min-w-[1rem] h-4 px-0.5',
                'flex items-center justify-center',
                'text-[10px] font-bold text-white bg-destructive rounded-full',
              )}>
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </button>
        </DropdownMenu.Trigger>

        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            className="w-80 bg-card border border-border rounded-lg shadow-lg z-50 overflow-hidden"
          >
            {/* Header */}
            <div className="flex items-center justify-between px-3 py-2 border-b border-border">
              <span className="text-sm font-semibold">Notifications</span>
              {unreadCount > 0 && (
                <button
                  onClick={() => markAllMutation.mutate()}
                  className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  <CheckCheck className="w-3 h-3" />
                  Mark all read
                </button>
              )}
            </div>

            {/* Notification items */}
            {!data || data.data.length === 0 ? (
              <div className="px-3 py-6 text-center text-sm text-muted-foreground">
                No notifications yet
              </div>
            ) : (
              data.data.map((n) => (
                <DropdownMenu.Item
                  key={n.id}
                  className={cn(
                    'flex items-start gap-2 px-3 py-2 cursor-pointer outline-none',
                    'hover:bg-accent/50 transition-colors border-b border-border last:border-0',
                    !n.read && 'bg-accent/20',
                  )}
                  onSelect={() => {
                    if (!n.read) markReadMutation.mutate(n.id)
                  }}
                >
                  {!n.read && <span className="w-2 h-2 rounded-full bg-primary mt-1 shrink-0" />}
                  <div className={cn('flex-1 min-w-0', n.read && 'pl-4')}>
                    <p className="text-sm font-medium truncate">{n.title}</p>
                    <p className="text-xs text-muted-foreground line-clamp-2">{n.body}</p>
                    <p className="text-[10px] text-muted-foreground mt-0.5">
                      {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                    </p>
                  </div>
                </DropdownMenu.Item>
              ))
            )}

            {/* Footer */}
            <div className="border-t border-border">
              <DropdownMenu.Item
                className="px-3 py-2 text-sm text-center text-primary hover:bg-accent/50 cursor-pointer outline-none transition-colors"
                onSelect={() => {
                  setOpen(false)
                  navigate('/notifications')
                }}
              >
                View all notifications
              </DropdownMenu.Item>
            </div>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </header>
  )
}
```

**Step 2: Build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

**Step 3: Commit**

```bash
git add frontend/src/components/layout/TopBar.tsx
git commit -m "feat(phase7): add notification bell dropdown with live unread count"
```

---

## Task 16: Frontend — Alerts page

**Files:**
- Modify: `frontend/src/pages/Alerts.tsx`
- Create: `frontend/src/components/alerts/AddAlertModal.tsx`

**Step 1: Create AddAlertModal.tsx**

Create `frontend/src/components/alerts/AddAlertModal.tsx`:

```tsx
import { useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createAlert } from '@/api/alerts'
import type { AlertCondition } from '@/types'

interface Props {
  open: boolean
  onClose: () => void
}

const CONDITIONS: { value: AlertCondition; label: string }[] = [
  { value: 'price_above',     label: 'Price Above' },
  { value: 'price_below',     label: 'Price Below' },
  { value: 'price_change_pct', label: 'Price Change %' },
]

export function AddAlertModal({ open, onClose }: Props) {
  const queryClient = useQueryClient()
  const [name, setName]         = useState('')
  const [adapter, setAdapter]   = useState('binance')
  const [symbol, setSymbol]     = useState('')
  const [market, setMarket]     = useState('crypto')
  const [condition, setCondition] = useState<AlertCondition>('price_above')
  const [threshold, setThreshold] = useState('')
  const [cooldown, setCooldown] = useState('60')
  const [message, setMessage]   = useState('')

  const mutation = useMutation({
    mutationFn: createAlert,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] })
      onClose()
      resetForm()
    },
  })

  function resetForm() {
    setName(''); setSymbol(''); setThreshold(''); setMessage('')
    setAdapter('binance'); setMarket('crypto')
    setCondition('price_above'); setCooldown('60')
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    mutation.mutate({
      name: name || `${symbol} ${condition} ${threshold}`,
      adapter_id: adapter,
      symbol: symbol.toUpperCase(),
      market,
      condition,
      threshold: parseFloat(threshold),
      message,
      cooldown_minutes: parseInt(cooldown, 10),
    })
  }

  const thresholdLabel = condition === 'price_change_pct' ? '%' : '$'

  return (
    <Dialog.Root open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/50 z-40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-[420px] bg-card border border-border rounded-lg shadow-xl z-50 p-6">
          <div className="flex items-center justify-between mb-4">
            <Dialog.Title className="text-base font-semibold">Add Alert</Dialog.Title>
            <button onClick={onClose} className="p-1 rounded hover:bg-accent text-muted-foreground">
              <X className="w-4 h-4" />
            </button>
          </div>

          <form onSubmit={handleSubmit} className="space-y-3">
            {/* Adapter */}
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-1 block">Market</label>
              <select
                value={adapter}
                onChange={(e) => setAdapter(e.target.value)}
                className="w-full h-9 px-3 rounded border border-input bg-background text-sm"
              >
                <option value="binance">Binance (Crypto)</option>
                <option value="yahoo">Yahoo Finance (Stocks)</option>
              </select>
            </div>

            {/* Symbol */}
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-1 block">Symbol</label>
              <input
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
                placeholder="e.g. BTCUSDT or AAPL"
                className="w-full h-9 px-3 rounded border border-input bg-background text-sm"
                required
              />
            </div>

            {/* Condition */}
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-1 block">Condition</label>
              <div className="flex gap-1">
                {CONDITIONS.map((c) => (
                  <button
                    key={c.value}
                    type="button"
                    onClick={() => setCondition(c.value)}
                    className={`flex-1 py-1.5 text-xs rounded border transition-colors ${
                      condition === c.value
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'border-input text-muted-foreground hover:bg-accent/50'
                    }`}
                  >
                    {c.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Threshold */}
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-1 block">
                Threshold ({thresholdLabel})
              </label>
              <input
                type="number"
                step="any"
                value={threshold}
                onChange={(e) => setThreshold(e.target.value)}
                placeholder={condition === 'price_change_pct' ? '5.0' : '50000'}
                className="w-full h-9 px-3 rounded border border-input bg-background text-sm"
                required
              />
            </div>

            {/* Cooldown */}
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-1 block">
                Cooldown (minutes)
              </label>
              <input
                type="number"
                min="1"
                value={cooldown}
                onChange={(e) => setCooldown(e.target.value)}
                className="w-full h-9 px-3 rounded border border-input bg-background text-sm"
              />
            </div>

            {/* Optional name */}
            <div>
              <label className="text-xs font-medium text-muted-foreground mb-1 block">
                Name (optional)
              </label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Auto-generated if blank"
                className="w-full h-9 px-3 rounded border border-input bg-background text-sm"
              />
            </div>

            {mutation.isError && (
              <p className="text-xs text-destructive">Failed to create alert. Please try again.</p>
            )}

            <button
              type="submit"
              disabled={mutation.isPending}
              className="w-full h-9 bg-primary text-primary-foreground rounded text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              {mutation.isPending ? 'Creating…' : 'Create Alert'}
            </button>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
```

**Step 2: Rewrite Alerts.tsx**

Replace `frontend/src/pages/Alerts.tsx`:

```tsx
import { useState } from 'react'
import { Plus, Trash2, Power, PowerOff } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'
import { fetchAlerts, deleteAlert, toggleAlert } from '@/api/alerts'
import { AddAlertModal } from '@/components/alerts/AddAlertModal'
import type { Alert, AlertCondition } from '@/types'
import { cn } from '@/lib/utils'

const CONDITION_LABELS: Record<AlertCondition, string> = {
  price_above:      'Price Above',
  price_below:      'Price Below',
  price_change_pct: 'Change %',
}

const STATUS_COLORS = {
  active:    'bg-green-500/20 text-green-400',
  triggered: 'bg-yellow-500/20 text-yellow-400',
  disabled:  'bg-gray-500/20 text-gray-400',
}

function AlertRow({ alert }: { alert: Alert }) {
  const queryClient = useQueryClient()

  const deleteMutation = useMutation({
    mutationFn: deleteAlert,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alerts'] }),
  })

  const toggleMutation = useMutation({
    mutationFn: toggleAlert,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alerts'] }),
  })

  const thresholdDisplay =
    alert.condition === 'price_change_pct'
      ? `${alert.threshold}%`
      : `$${alert.threshold.toLocaleString()}`

  return (
    <tr className="border-b border-border hover:bg-accent/20 transition-colors">
      <td className="px-4 py-3 text-sm font-medium">{alert.name}</td>
      <td className="px-4 py-3 text-sm text-muted-foreground">{alert.symbol}</td>
      <td className="px-4 py-3 text-sm">{CONDITION_LABELS[alert.condition]}</td>
      <td className="px-4 py-3 text-sm font-mono">{thresholdDisplay}</td>
      <td className="px-4 py-3 text-sm text-muted-foreground">{alert.cooldown_minutes}m</td>
      <td className="px-4 py-3">
        <span className={cn('text-xs px-2 py-0.5 rounded font-medium', STATUS_COLORS[alert.status])}>
          {alert.status}
        </span>
      </td>
      <td className="px-4 py-3 text-xs text-muted-foreground">
        {alert.last_fired_at
          ? formatDistanceToNow(new Date(alert.last_fired_at), { addSuffix: true })
          : '—'}
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-1">
          <button
            onClick={() => toggleMutation.mutate(alert.id)}
            disabled={toggleMutation.isPending}
            className="p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
            title={alert.status === 'active' ? 'Disable' : 'Enable'}
          >
            {alert.status === 'active'
              ? <PowerOff className="w-4 h-4" />
              : <Power className="w-4 h-4 text-green-500" />}
          </button>
          <button
            onClick={() => deleteMutation.mutate(alert.id)}
            disabled={deleteMutation.isPending}
            className="p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-destructive transition-colors"
            title="Delete"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </td>
    </tr>
  )
}

export function Alerts() {
  const [showModal, setShowModal] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['alerts'],
    queryFn: fetchAlerts,
  })
  const alerts = data ?? []

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Alerts</h1>
          <p className="text-muted-foreground text-sm mt-0.5">
            Manage price alert rules. Alerts re-fire after the cooldown period.
          </p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add Alert
        </button>
      </div>

      <div className="bg-card border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground text-sm">Loading…</div>
        ) : alerts.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground text-sm">
            No alerts yet. Click "Add Alert" to create one.
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Name</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Symbol</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Condition</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Threshold</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Cooldown</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Status</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Last Fired</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-muted-foreground">Actions</th>
              </tr>
            </thead>
            <tbody>
              {alerts.map((a) => <AlertRow key={a.id} alert={a} />)}
            </tbody>
          </table>
        )}
      </div>

      <AddAlertModal open={showModal} onClose={() => setShowModal(false)} />
    </div>
  )
}
```

**Step 3: Build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

**Step 4: Commit**

```bash
git add frontend/src/pages/Alerts.tsx frontend/src/components/alerts/
git commit -m "feat(phase7): implement Alerts page with table and AddAlertModal"
```

---

## Task 17: Frontend — Notifications page

**Files:**
- Modify: `frontend/src/pages/Notifications.tsx`

**Step 1: Rewrite Notifications.tsx**

```tsx
import { useState } from 'react'
import { CheckCheck, Bell } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { formatDistanceToNow, format } from 'date-fns'
import { fetchNotifications, markNotificationRead, markAllNotificationsRead } from '@/api/alerts'
import { useNotificationStore } from '@/stores'
import { cn } from '@/lib/utils'
import type { NotificationType } from '@/types'

const TYPE_COLORS: Record<NotificationType, string> = {
  alert:     'bg-yellow-500/20 text-yellow-400',
  trade:     'bg-blue-500/20 text-blue-400',
  backtest:  'bg-purple-500/20 text-purple-400',
  system:    'bg-gray-500/20 text-gray-400',
}

const PAGE_SIZE = 20

export function Notifications() {
  const queryClient = useQueryClient()
  const { markRead, markAllRead } = useNotificationStore()
  const [page, setPage] = useState(0)

  const { data, isLoading } = useQuery({
    queryKey: ['notifications', page],
    queryFn: () => fetchNotifications({ limit: PAGE_SIZE, offset: page * PAGE_SIZE }),
  })

  const markReadMutation = useMutation({
    mutationFn: markNotificationRead,
    onSuccess: (_, id) => {
      markRead(id)
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })

  const markAllMutation = useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      markAllRead()
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })

  const notifications = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Notifications</h1>
          <p className="text-muted-foreground text-sm mt-0.5">{total} total</p>
        </div>
        <button
          onClick={() => markAllMutation.mutate()}
          disabled={markAllMutation.isPending}
          className="flex items-center gap-2 px-3 py-1.5 border border-border rounded text-sm text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        >
          <CheckCheck className="w-4 h-4" />
          Mark all read
        </button>
      </div>

      <div className="bg-card border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground text-sm">Loading…</div>
        ) : notifications.length === 0 ? (
          <div className="p-8 text-center">
            <Bell className="w-8 h-8 text-muted-foreground mx-auto mb-2" />
            <p className="text-sm text-muted-foreground">No notifications yet.</p>
          </div>
        ) : (
          <div>
            {notifications.map((n) => (
              <div
                key={n.id}
                onClick={() => { if (!n.read) markReadMutation.mutate(n.id) }}
                className={cn(
                  'flex items-start gap-3 px-4 py-3 border-b border-border last:border-0 transition-colors',
                  !n.read && 'bg-accent/10 cursor-pointer hover:bg-accent/20',
                  n.read && 'opacity-70',
                )}
              >
                {!n.read && <span className="w-2 h-2 rounded-full bg-primary mt-1.5 shrink-0" />}
                <div className={cn('flex-1 min-w-0', n.read && 'pl-5')}>
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className={cn('text-[10px] px-1.5 py-0.5 rounded font-medium', TYPE_COLORS[n.type])}>
                      {n.type}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                    </span>
                    <span className="text-[10px] text-muted-foreground ml-auto">
                      {format(new Date(n.created_at), 'MMM d, HH:mm')}
                    </span>
                  </div>
                  <p className="text-sm font-medium">{n.title}</p>
                  <p className="text-xs text-muted-foreground mt-0.5">{n.body}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <button
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
            className="px-3 py-1 text-sm border border-border rounded disabled:opacity-40 hover:bg-accent transition-colors"
          >
            Previous
          </button>
          <span className="text-sm text-muted-foreground">
            Page {page + 1} of {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
            className="px-3 py-1 text-sm border border-border rounded disabled:opacity-40 hover:bg-accent transition-colors"
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
```

**Step 2: Build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

**Step 3: Commit**

```bash
git add frontend/src/pages/Notifications.tsx
git commit -m "feat(phase7): implement Notifications page with pagination and mark-read"
```

---

## Task 18: Final — run all tests and update phases.md

**Step 1: Run all backend tests**

```bash
cd backend && go test ./... 2>&1 | tail -20
```

Expected: all PASS

**Step 2: Run frontend build + lint**

```bash
cd frontend && npm run build && npm run lint 2>&1 | tail -10
```

Expected: no errors

**Step 3: Mark Phase 7 complete in phases.md**

In `.claude/docs/phases.md`, change `## Phase 7 — News, Events & Alerts 🔲` to `## Phase 7 — News, Events & Alerts ✅ COMPLETE` and mark all sub-phase checkboxes as `[x]`.

Also add a note under 7.3:
```
> **Deferred:** rsi_overbought / rsi_oversold conditions (see Phase 8 Advanced Alerts backlog)
```

**Step 4: Final commit**

```bash
git add .claude/docs/phases.md
git commit -m "docs(phase7): mark Phase 7 complete in phases.md"
```

---

## Deferred / Backlog

- **RSI alert conditions** (`rsi_overbought`, `rsi_oversold`): requires live strategy engine wiring (Phase 8). Add as a Phase 8 sub-task.
- **`include_news=true` on candles endpoint**: deferred. News markers shown via Chart page side panel.
- **Lightweight-charts news markers on chart timeline**: can be added as a polish step (Task 14 focuses on the side panel).
