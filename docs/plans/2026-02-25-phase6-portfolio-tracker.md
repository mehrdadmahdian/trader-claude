# Phase 6 — Portfolio Tracker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a multi-asset portfolio tracker with CRUD, live PnL via WebSocket, allocation donut, equity curve, and transaction history.

**Architecture:** Extend existing `Portfolio` model with new `Position` and `Transaction` models. Add `internal/price/` shared service (Binance + Yahoo, Redis 30s TTL) and `internal/portfolio/` service. Wire 12 REST routes + 1 WebSocket endpoint. Build full frontend page in React with Recharts.

**Tech Stack:** Go/Fiber/GORM/Redis (backend), React 18/TypeScript/Zustand/React Query/Recharts (frontend), `github.com/gofiber/contrib/websocket` for WS.

**Design doc:** `docs/plans/2026-02-25-phase6-portfolio-tracker-design.md`

---

## Task 1: Extend Models — Position, Transaction, Portfolio

**Files:**
- Modify: `backend/internal/models/models.go`
- Modify: `backend/cmd/server/main.go` (autoMigrate list)

### Step 1: Add PortfolioType constant + extend Portfolio struct

In `backend/internal/models/models.go`, find the `Portfolio` struct (line ~196) and replace it:

```go
// PortfolioType distinguishes manual tracking from automated trading
type PortfolioType string

const (
	PortfolioTypeManual PortfolioType = "manual"
	PortfolioTypePaper  PortfolioType = "paper"
	PortfolioTypeLive   PortfolioType = "live"
)

// Portfolio represents a live trading, paper trading, or manually-tracked portfolio
type Portfolio struct {
	ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string         `gorm:"type:varchar(200);not null" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	Type         PortfolioType  `gorm:"type:varchar(20);not null;default:'manual'" json:"type"`
	Currency     string         `gorm:"type:varchar(10);not null;default:'USD'" json:"currency"`
	StrategyName string         `gorm:"type:varchar(100)" json:"strategy_name"`
	Symbol       string         `gorm:"type:varchar(20)" json:"symbol"`
	Market       string         `gorm:"type:varchar(20)" json:"market"`
	Timeframe    string         `gorm:"type:varchar(10)" json:"timeframe"`
	Params       JSON           `gorm:"type:json" json:"params"`
	IsLive       bool           `gorm:"default:false" json:"is_live"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	InitialCash  float64        `gorm:"type:decimal(20,8);not null;default:0" json:"initial_cash"`
	CurrentCash  float64        `gorm:"type:decimal(20,8)" json:"current_cash"`
	CurrentValue float64        `gorm:"type:decimal(20,8)" json:"current_value"`
	State        JSON           `gorm:"type:json" json:"state,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Portfolio) TableName() string { return "portfolios" }
```

### Step 2: Add Position model

After the `Portfolio` model, add:

```go
// --- Position ---

// Position tracks a single asset holding within a portfolio
type Position struct {
	ID                int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	PortfolioID       int64          `gorm:"not null;index" json:"portfolio_id"`
	AdapterID         string         `gorm:"type:varchar(20);not null" json:"adapter_id"`
	Symbol            string         `gorm:"type:varchar(20);not null" json:"symbol"`
	Market            string         `gorm:"type:varchar(20);not null" json:"market"`
	Quantity          float64        `gorm:"type:decimal(30,8);not null" json:"quantity"`
	AvgCost           float64        `gorm:"type:decimal(20,8);not null" json:"avg_cost"`
	CurrentPrice      float64        `gorm:"type:decimal(20,8)" json:"current_price"`
	CurrentValue      float64        `gorm:"type:decimal(20,8)" json:"current_value"`
	UnrealizedPnL     float64        `gorm:"type:decimal(20,8)" json:"unrealized_pnl"`
	UnrealizedPnLPct  float64        `gorm:"type:decimal(10,4)" json:"unrealized_pnl_pct"`
	OpenedAt          time.Time      `gorm:"not null" json:"opened_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (Position) TableName() string { return "positions" }
```

### Step 3: Add Transaction model and TransactionType constant

After the `Position` model, add:

```go
// --- Transaction ---

// TransactionType identifies what kind of transaction occurred
type TransactionType string

const (
	TransactionTypeBuy        TransactionType = "buy"
	TransactionTypeSell       TransactionType = "sell"
	TransactionTypeDeposit    TransactionType = "deposit"
	TransactionTypeWithdrawal TransactionType = "withdrawal"
)

// Transaction logs buy/sell/deposit/withdrawal events for a portfolio
type Transaction struct {
	ID          int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	PortfolioID int64           `gorm:"not null;index" json:"portfolio_id"`
	PositionID  *int64          `gorm:"index" json:"position_id,omitempty"`
	Type        TransactionType `gorm:"type:varchar(20);not null" json:"type"`
	AdapterID   string          `gorm:"type:varchar(20)" json:"adapter_id"`
	Symbol      string          `gorm:"type:varchar(20)" json:"symbol"`
	Quantity    float64         `gorm:"type:decimal(30,8)" json:"quantity"`
	Price       float64         `gorm:"type:decimal(20,8)" json:"price"`
	Fee         float64         `gorm:"type:decimal(20,8);default:0" json:"fee"`
	Notes       string          `gorm:"type:text" json:"notes"`
	ExecutedAt  time.Time       `gorm:"not null" json:"executed_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (Transaction) TableName() string { return "transactions" }
```

### Step 4: Register new models in autoMigrate

In `backend/cmd/server/main.go`, find the `autoMigrate` function and add the new models:

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
		&models.ReplayBookmark{},
	)
}
```

### Step 5: Verify compilation

```bash
cd backend && go build ./...
```

Expected: no errors.

### Step 6: Commit

```bash
git add backend/internal/models/models.go backend/cmd/server/main.go
git commit -m "feat(phase6): add Position and Transaction models, extend Portfolio"
```

---

## Task 2: PriceService — Binance + Yahoo + Redis cache

**Files:**
- Create: `backend/internal/price/service.go`
- Create: `backend/internal/price/service_test.go`

### Step 1: Write the failing tests first

Create `backend/internal/price/service_test.go`:

```go
package price

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// newTestRedis returns a Redis client pointing at a mock that always misses
// We use a real miniredis alternative: stub the methods via interface.
// For simplicity, use a real Redis if available, else skip.
func skipIfNoRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("redis not available, skipping cache test")
	}
	return rdb
}

func TestGetPrice_Binance(t *testing.T) {
	// Mock Binance ticker endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/ticker/price" {
			json.NewEncoder(w).Encode(map[string]string{"symbol": "BTCUSDT", "price": "42000.5"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc := NewService(nil, srv.URL, "")
	price, err := svc.GetPrice(context.Background(), "binance", "BTCUSDT")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if price != 42000.5 {
		t.Errorf("expected 42000.5, got %f", price)
	}
}

func TestGetPrice_Yahoo(t *testing.T) {
	// Mock Yahoo chart endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"chart": map[string]interface{}{
				"result": []interface{}{
					map[string]interface{}{
						"meta": map[string]interface{}{
							"regularMarketPrice": 185.5,
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	svc := NewService(nil, "", srv.URL)
	price, err := svc.GetPrice(context.Background(), "yahoo", "AAPL")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if price != 185.5 {
		t.Errorf("expected 185.5, got %f", price)
	}
}

func TestGetPrice_UnknownAdapter(t *testing.T) {
	svc := NewService(nil, "", "")
	_, err := svc.GetPrice(context.Background(), "unknown", "XYZ")
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
}

func TestGetPrice_CacheHit(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.FlushDB(context.Background())

	ctx := context.Background()
	key := "price:binance:BTCUSDT"
	rdb.Set(ctx, key, "99000.0", 30*time.Second)

	svc := NewService(rdb, "http://unreachable", "http://unreachable")
	price, err := svc.GetPrice(ctx, "binance", "BTCUSDT")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if price != 99000.0 {
		t.Errorf("expected 99000.0 from cache, got %f", price)
	}
}
```

### Step 2: Run tests to verify they fail

```bash
cd backend && go test ./internal/price/... -v
```

Expected: FAIL — package does not exist yet.

### Step 3: Implement PriceService

Create `backend/internal/price/service.go`:

```go
package price

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrPriceUnavailable is returned when a price cannot be fetched from any source
var ErrPriceUnavailable = errors.New("price unavailable")

const cacheTTL = 30 * time.Second

// Service fetches real-time prices from Binance or Yahoo Finance with Redis caching
type Service struct {
	rdb        *redis.Client
	binanceURL string
	yahooURL   string
	httpClient *http.Client
}

// NewService creates a PriceService. Pass empty strings for binanceURL/yahooURL to use production defaults.
func NewService(rdb *redis.Client, binanceURL, yahooURL string) *Service {
	if binanceURL == "" {
		binanceURL = "https://api.binance.com"
	}
	if yahooURL == "" {
		yahooURL = "https://query1.finance.yahoo.com"
	}
	return &Service{
		rdb:        rdb,
		binanceURL: binanceURL,
		yahooURL:   yahooURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetPrice returns the current price for symbol via adapterID ("binance" or "yahoo").
// Checks Redis cache first (TTL 30s), then fetches live.
func (s *Service) GetPrice(ctx context.Context, adapterID, symbol string) (float64, error) {
	// Check cache
	if s.rdb != nil {
		key := fmt.Sprintf("price:%s:%s", adapterID, symbol)
		val, err := s.rdb.Get(ctx, key).Result()
		if err == nil {
			if price, parseErr := strconv.ParseFloat(val, 64); parseErr == nil {
				return price, nil
			}
		}
	}

	// Fetch live
	var price float64
	var err error

	switch adapterID {
	case "binance":
		price, err = s.fetchBinancePrice(ctx, symbol)
	case "yahoo":
		price, err = s.fetchYahooPrice(ctx, symbol)
	default:
		return 0, fmt.Errorf("%w: unknown adapter %q", ErrPriceUnavailable, adapterID)
	}

	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrPriceUnavailable, err)
	}

	// Cache result
	if s.rdb != nil {
		key := fmt.Sprintf("price:%s:%s", adapterID, symbol)
		s.rdb.Set(ctx, key, strconv.FormatFloat(price, 'f', 8, 64), cacheTTL)
	}

	return price, nil
}

func (s *Service) fetchBinancePrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", s.binanceURL, symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(result.Price, 64)
}

func (s *Service) fetchYahooPrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/v8/finance/chart/%s?range=1d&interval=1m", s.yahooURL, symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	if len(result.Chart.Result) == 0 {
		return 0, fmt.Errorf("no result for symbol %s", symbol)
	}
	return result.Chart.Result[0].Meta.RegularMarketPrice, nil
}
```

### Step 4: Run tests to verify they pass

```bash
cd backend && go test ./internal/price/... -v
```

Expected: all 4 tests PASS (TestGetPrice_CacheHit may skip if no Redis).

### Step 5: Commit

```bash
git add backend/internal/price/
git commit -m "feat(phase6): add PriceService with Binance, Yahoo, Redis cache"
```

---

## Task 3: PortfolioService — CRUD + RecalculatePortfolio + GetEquityCurve

**Files:**
- Create: `backend/internal/portfolio/service.go`
- Create: `backend/internal/portfolio/service_test.go`

### Step 1: Write failing tests

Create `backend/internal/portfolio/service_test.go`:

```go
package portfolio

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/price"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Portfolio{}, &models.Position{}, &models.Transaction{}); err != nil {
		t.Fatalf("auto-migrate failed: %v", err)
	}
	return db
}

// stubPriceService always returns a fixed price per symbol
type stubPriceService struct {
	prices map[string]float64
}

func (s *stubPriceService) GetPrice(_ context.Context, _, symbol string) (float64, error) {
	if p, ok := s.prices[symbol]; ok {
		return p, nil
	}
	return 0, price.ErrPriceUnavailable
}

func TestCreateAndGetPortfolio(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, &stubPriceService{})

	p, err := svc.CreatePortfolio(context.Background(), CreatePortfolioReq{
		Name:        "Test Portfolio",
		Description: "desc",
		Type:        models.PortfolioTypeManual,
		Currency:    "USD",
		InitialCash: 10000,
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := svc.GetPortfolio(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if got.Name != "Test Portfolio" {
		t.Errorf("expected name 'Test Portfolio', got %q", got.Name)
	}
}

func TestAddPositionAndRecalculate(t *testing.T) {
	db := newTestDB(t)
	stub := &stubPriceService{prices: map[string]float64{"BTCUSDT": 50000.0}}
	svc := NewService(db, stub)

	p, _ := svc.CreatePortfolio(context.Background(), CreatePortfolioReq{
		Name: "BTC Portfolio", Type: models.PortfolioTypeManual, InitialCash: 100000, Currency: "USD",
	})

	pos, err := svc.AddPosition(context.Background(), p.ID, AddPositionReq{
		AdapterID: "binance",
		Symbol:    "BTCUSDT",
		Market:    "crypto",
		Quantity:  2.0,
		AvgCost:   40000.0,
		OpenedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("AddPosition: %v", err)
	}
	if pos.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", pos.Symbol)
	}

	if err := svc.RecalculatePortfolio(context.Background(), p.ID); err != nil {
		t.Fatalf("RecalculatePortfolio: %v", err)
	}

	updated, _ := svc.GetPortfolio(context.Background(), p.ID)
	// With 2 BTC at $50k each, CurrentValue = 100000
	if updated.CurrentValue != 100000.0 {
		t.Errorf("expected CurrentValue=100000, got %f", updated.CurrentValue)
	}

	// Fetch the position directly and check PnL
	var pos2 models.Position
	db.First(&pos2, pos.ID)
	// UnrealizedPnL = (50000 - 40000) * 2 = 20000
	if pos2.UnrealizedPnL != 20000.0 {
		t.Errorf("expected UnrealizedPnL=20000, got %f", pos2.UnrealizedPnL)
	}
	// UnrealizedPnLPct = 20000 / (40000*2) * 100 = 25
	if pos2.UnrealizedPnLPct != 25.0 {
		t.Errorf("expected UnrealizedPnLPct=25, got %f", pos2.UnrealizedPnLPct)
	}
}

func TestGetEquityCurve(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, &stubPriceService{})

	p, _ := svc.CreatePortfolio(context.Background(), CreatePortfolioReq{
		Name: "Eq Portfolio", Type: models.PortfolioTypeManual, InitialCash: 10000, Currency: "USD",
	})

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)

	svc.AddTransaction(context.Background(), p.ID, AddTransactionReq{
		Type: models.TransactionTypeDeposit, Price: 5000, Quantity: 1, ExecutedAt: t1,
	})
	svc.AddTransaction(context.Background(), p.ID, AddTransactionReq{
		Type: models.TransactionTypeDeposit, Price: 5000, Quantity: 1, ExecutedAt: t2,
	})

	curve, err := svc.GetEquityCurve(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetEquityCurve: %v", err)
	}
	if len(curve) != 2 {
		t.Errorf("expected 2 equity points, got %d", len(curve))
	}
	// After first deposit: value = 5000
	if curve[0].Value != 5000 {
		t.Errorf("expected first point value=5000, got %f", curve[0].Value)
	}
	// After second deposit: value = 10000
	if curve[1].Value != 10000 {
		t.Errorf("expected second point value=10000, got %f", curve[1].Value)
	}
}

func TestListTransactions_Pagination(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db, &stubPriceService{})

	p, _ := svc.CreatePortfolio(context.Background(), CreatePortfolioReq{
		Name: "TX Portfolio", Type: models.PortfolioTypeManual, InitialCash: 0, Currency: "USD",
	})

	for i := 0; i < 5; i++ {
		svc.AddTransaction(context.Background(), p.ID, AddTransactionReq{
			Type: models.TransactionTypeDeposit, Price: 100, Quantity: 1,
			ExecutedAt: time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	txs, total, err := svc.ListTransactions(context.Background(), p.ID, 1, 3)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(txs) != 3 {
		t.Errorf("expected 3 results, got %d", len(txs))
	}
}
```

### Step 2: Run tests to verify they fail

```bash
cd backend && go test ./internal/portfolio/... -v
```

Expected: FAIL — package does not exist yet.

### Step 3: Implement PortfolioService

Create `backend/internal/portfolio/service.go`:

```go
package portfolio

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// PriceFetcher is the minimal interface required by PortfolioService for price lookups
type PriceFetcher interface {
	GetPrice(ctx context.Context, adapterID, symbol string) (float64, error)
}

// EquityPoint is a single value snapshot in the equity curve
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// PortfolioSummary holds aggregated stats for the summary cards
type PortfolioSummary struct {
	PortfolioID   int64   `json:"portfolio_id"`
	TotalValue    float64 `json:"total_value"`
	TotalCost     float64 `json:"total_cost"`
	TotalPnL      float64 `json:"total_pnl"`
	TotalPnLPct   float64 `json:"total_pnl_pct"`
	DayChangePct  float64 `json:"day_change_pct"`
}

// Request types ---

type CreatePortfolioReq struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        models.PortfolioType `json:"type"`
	Currency    string              `json:"currency"`
	InitialCash float64             `json:"initial_cash"`
}

type UpdatePortfolioReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Currency    string `json:"currency"`
}

type AddPositionReq struct {
	AdapterID string    `json:"adapter_id"`
	Symbol    string    `json:"symbol"`
	Market    string    `json:"market"`
	Quantity  float64   `json:"quantity"`
	AvgCost   float64   `json:"avg_cost"`
	OpenedAt  time.Time `json:"opened_at"`
}

type UpdatePositionReq struct {
	Quantity float64 `json:"quantity"`
	AvgCost  float64 `json:"avg_cost"`
}

type AddTransactionReq struct {
	PositionID *int64                  `json:"position_id,omitempty"`
	Type       models.TransactionType  `json:"type"`
	AdapterID  string                  `json:"adapter_id"`
	Symbol     string                  `json:"symbol"`
	Quantity   float64                 `json:"quantity"`
	Price      float64                 `json:"price"`
	Fee        float64                 `json:"fee"`
	Notes      string                  `json:"notes"`
	ExecutedAt time.Time               `json:"executed_at"`
}

// Service handles all portfolio business logic
type Service struct {
	db    *gorm.DB
	price PriceFetcher
}

func NewService(db *gorm.DB, price PriceFetcher) *Service {
	return &Service{db: db, price: price}
}

// --- Portfolio CRUD ---

func (s *Service) CreatePortfolio(ctx context.Context, req CreatePortfolioReq) (*models.Portfolio, error) {
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Type == "" {
		req.Type = models.PortfolioTypeManual
	}
	p := &models.Portfolio{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Currency:    req.Currency,
		InitialCash: req.InitialCash,
		CurrentCash: req.InitialCash,
		IsActive:    true,
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) GetPortfolio(ctx context.Context, id int64) (*models.Portfolio, error) {
	var p models.Portfolio
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) ListPortfolios(ctx context.Context) ([]*models.Portfolio, error) {
	var portfolios []*models.Portfolio
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Find(&portfolios).Error; err != nil {
		return nil, err
	}
	return portfolios, nil
}

func (s *Service) UpdatePortfolio(ctx context.Context, id int64, req UpdatePortfolioReq) (*models.Portfolio, error) {
	var p models.Portfolio
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Currency != "" {
		p.Currency = req.Currency
	}
	if err := s.db.WithContext(ctx).Save(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) DeletePortfolio(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&models.Portfolio{}, id).Error
}

func (s *Service) GetPortfolioWithPositions(ctx context.Context, id int64) (*models.Portfolio, []models.Position, error) {
	p, err := s.GetPortfolio(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	var positions []models.Position
	if err := s.db.WithContext(ctx).Where("portfolio_id = ?", id).Find(&positions).Error; err != nil {
		return nil, nil, err
	}
	return p, positions, nil
}

func (s *Service) GetSummary(ctx context.Context, id int64) (*PortfolioSummary, error) {
	_, positions, err := s.GetPortfolioWithPositions(ctx, id)
	if err != nil {
		return nil, err
	}
	var totalValue, totalCost float64
	for _, pos := range positions {
		totalValue += pos.CurrentValue
		totalCost += pos.Quantity * pos.AvgCost
	}
	var totalPnL, totalPnLPct float64
	if totalCost > 0 {
		totalPnL = totalValue - totalCost
		totalPnLPct = (totalPnL / totalCost) * 100
	}
	return &PortfolioSummary{
		PortfolioID: id,
		TotalValue:  totalValue,
		TotalCost:   totalCost,
		TotalPnL:    totalPnL,
		TotalPnLPct: totalPnLPct,
	}, nil
}

// --- Position CRUD ---

func (s *Service) AddPosition(ctx context.Context, portfolioID int64, req AddPositionReq) (*models.Position, error) {
	pos := &models.Position{
		PortfolioID: portfolioID,
		AdapterID:   req.AdapterID,
		Symbol:      req.Symbol,
		Market:      req.Market,
		Quantity:    req.Quantity,
		AvgCost:     req.AvgCost,
		OpenedAt:    req.OpenedAt,
	}
	if err := s.db.WithContext(ctx).Create(pos).Error; err != nil {
		return nil, err
	}
	return pos, nil
}

func (s *Service) UpdatePosition(ctx context.Context, positionID int64, req UpdatePositionReq) (*models.Position, error) {
	var pos models.Position
	if err := s.db.WithContext(ctx).First(&pos, positionID).Error; err != nil {
		return nil, err
	}
	pos.Quantity = req.Quantity
	pos.AvgCost = req.AvgCost
	if err := s.db.WithContext(ctx).Save(&pos).Error; err != nil {
		return nil, err
	}
	return &pos, nil
}

func (s *Service) DeletePosition(ctx context.Context, positionID int64) error {
	return s.db.WithContext(ctx).Delete(&models.Position{}, positionID).Error
}

// --- Transaction CRUD ---

func (s *Service) AddTransaction(ctx context.Context, portfolioID int64, req AddTransactionReq) (*models.Transaction, error) {
	tx := &models.Transaction{
		PortfolioID: portfolioID,
		PositionID:  req.PositionID,
		Type:        req.Type,
		AdapterID:   req.AdapterID,
		Symbol:      req.Symbol,
		Quantity:    req.Quantity,
		Price:       req.Price,
		Fee:         req.Fee,
		Notes:       req.Notes,
		ExecutedAt:  req.ExecutedAt,
	}
	if err := s.db.WithContext(ctx).Create(tx).Error; err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *Service) ListTransactions(ctx context.Context, portfolioID int64, page, limit int) ([]models.Transaction, int64, error) {
	var txs []models.Transaction
	var total int64

	q := s.db.WithContext(ctx).Model(&models.Transaction{}).Where("portfolio_id = ?", portfolioID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := q.Order("executed_at DESC").Offset(offset).Limit(limit).Find(&txs).Error; err != nil {
		return nil, 0, err
	}
	return txs, total, nil
}

// --- Analytics ---

// RecalculatePortfolio fetches current prices for all positions, updates PnL fields,
// and sets Portfolio.CurrentValue to the sum of all position values.
func (s *Service) RecalculatePortfolio(ctx context.Context, portfolioID int64) error {
	var positions []models.Position
	if err := s.db.WithContext(ctx).Where("portfolio_id = ?", portfolioID).Find(&positions).Error; err != nil {
		return err
	}

	var totalValue float64
	for i := range positions {
		pos := &positions[i]
		currentPrice, err := s.price.GetPrice(ctx, pos.AdapterID, pos.Symbol)
		if err != nil {
			// Use last known price if fetch fails
			currentPrice = pos.CurrentPrice
		}
		pos.CurrentPrice = currentPrice
		pos.CurrentValue = pos.Quantity * currentPrice
		costBasis := pos.Quantity * pos.AvgCost
		pos.UnrealizedPnL = pos.CurrentValue - costBasis
		if costBasis > 0 {
			pos.UnrealizedPnLPct = (pos.UnrealizedPnL / costBasis) * 100
		}
		totalValue += pos.CurrentValue
		s.db.WithContext(ctx).Save(pos)
	}

	return s.db.WithContext(ctx).Model(&models.Portfolio{}).Where("id = ?", portfolioID).
		Update("current_value", totalValue).Error
}

// GetEquityCurve replays transactions chronologically to produce a value-over-time series.
// For deposits/withdrawals it adds/subtracts cash. For buy/sell it uses the transaction price.
func (s *Service) GetEquityCurve(ctx context.Context, portfolioID int64) ([]EquityPoint, error) {
	var txs []models.Transaction
	if err := s.db.WithContext(ctx).
		Where("portfolio_id = ?", portfolioID).
		Order("executed_at ASC").
		Find(&txs).Error; err != nil {
		return nil, err
	}

	var runningValue float64
	points := make([]EquityPoint, 0, len(txs))

	for _, tx := range txs {
		switch tx.Type {
		case models.TransactionTypeDeposit:
			runningValue += tx.Price * tx.Quantity
		case models.TransactionTypeWithdrawal:
			runningValue -= tx.Price * tx.Quantity
		case models.TransactionTypeBuy:
			// Cash goes out, asset value added at cost
			// Net effect on portfolio value at transaction time is zero
			// (we bought at market price, so value doesn't change instantaneously)
		case models.TransactionTypeSell:
			// Cash comes in at sell price
			runningValue += tx.Price*tx.Quantity - tx.Fee
		}
		points = append(points, EquityPoint{Timestamp: tx.ExecutedAt, Value: runningValue})
	}

	return points, nil
}
```

### Step 4: Add SQLite dependency for tests

```bash
cd backend && go get gorm.io/driver/sqlite
```

### Step 5: Run tests to verify they pass

```bash
cd backend && go test ./internal/portfolio/... -v
```

Expected: all 4 tests PASS.

### Step 6: Commit

```bash
git add backend/internal/portfolio/ backend/go.mod backend/go.sum
git commit -m "feat(phase6): add PortfolioService with CRUD, RecalculatePortfolio, GetEquityCurve"
```

---

## Task 4: Portfolio API Handler — 12 REST Routes

**Files:**
- Create: `backend/internal/api/portfolio.go`
- Create: `backend/internal/api/portfolio_test.go`
- Modify: `backend/internal/api/routes.go`
- Modify: `backend/cmd/server/main.go`

### Step 1: Write failing API integration tests

Create `backend/internal/api/portfolio_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/portfolio"
	"github.com/trader-claude/backend/internal/price"
)

func newPortfolioTestApp(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.Portfolio{}, &models.Position{}, &models.Transaction{})

	svc := portfolio.NewService(db, &stubPrice{})
	h := newPortfolioHandler(svc)

	app := fiber.New()
	v1 := app.Group("/api/v1")
	h.registerRoutes(v1)
	return app, db
}

type stubPrice struct{}

func (s *stubPrice) GetPrice(_ interface{}, _, _ string) (float64, error) {
	return 100.0, nil
}

func TestPortfolioCreateAndList(t *testing.T) {
	app, _ := newPortfolioTestApp(t)

	// Create
	body, _ := json.Marshal(map[string]interface{}{
		"name": "Test", "type": "manual", "currency": "USD", "initial_cash": 1000,
	})
	req := httptest.NewRequest("POST", "/api/v1/portfolios", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != 201 {
		t.Fatalf("create failed: status=%d err=%v", resp.StatusCode, err)
	}

	// List
	req2 := httptest.NewRequest("GET", "/api/v1/portfolios", nil)
	resp2, _ := app.Test(req2)
	var result map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&result)
	data := result["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 portfolio, got %d", len(data))
	}
}

func TestPortfolioGetByID(t *testing.T) {
	app, _ := newPortfolioTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{"name": "P1", "type": "manual", "currency": "USD", "initial_cash": 0})
	req := httptest.NewRequest("POST", "/api/v1/portfolios", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	id := created["data"].(map[string]interface{})["id"]

	req2 := httptest.NewRequest("GET", "/api/v1/portfolios/"+fmt.Sprintf("%v", id), nil)
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestPortfolioAddPosition(t *testing.T) {
	app, _ := newPortfolioTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{"name": "P", "type": "manual", "currency": "USD", "initial_cash": 0})
	req := httptest.NewRequest("POST", "/api/v1/portfolios", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	id := int(created["data"].(map[string]interface{})["id"].(float64))

	posBody, _ := json.Marshal(map[string]interface{}{
		"adapter_id": "binance", "symbol": "BTCUSDT", "market": "crypto",
		"quantity": 1.0, "avg_cost": 40000.0, "opened_at": "2024-01-01T00:00:00Z",
	})
	req2 := httptest.NewRequest("POST", "/api/v1/portfolios/"+strconv.Itoa(id)+"/positions", bytes.NewReader(posBody))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 201 {
		b, _ := io.ReadAll(resp2.Body)
		t.Errorf("expected 201, got %d: %s", resp2.StatusCode, b)
	}
}
```

### Step 2: Run tests to verify they fail

```bash
cd backend && go test ./internal/api/... -run TestPortfolio -v
```

Expected: FAIL — handler not yet implemented.

### Step 3: Implement portfolio handler

Create `backend/internal/api/portfolio.go`:

```go
package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/trader-claude/backend/internal/portfolio"
)

type portfolioHandler struct {
	svc *portfolio.Service
}

func newPortfolioHandler(svc *portfolio.Service) *portfolioHandler {
	return &portfolioHandler{svc: svc}
}

func (h *portfolioHandler) registerRoutes(v1 fiber.Router) {
	v1.Post("/portfolios", h.create)
	v1.Get("/portfolios", h.list)
	v1.Get("/portfolios/:id", h.get)
	v1.Put("/portfolios/:id", h.update)
	v1.Delete("/portfolios/:id", h.delete)
	v1.Get("/portfolios/:id/summary", h.summary)

	v1.Post("/portfolios/:id/positions", h.addPosition)
	v1.Put("/portfolios/:id/positions/:posId", h.updatePosition)
	v1.Delete("/portfolios/:id/positions/:posId", h.deletePosition)

	v1.Post("/portfolios/:id/transactions", h.addTransaction)
	v1.Get("/portfolios/:id/transactions", h.listTransactions)

	v1.Get("/portfolios/:id/equity-curve", h.equityCurve)
}

func parseID(c *fiber.Ctx, param string) (int64, error) {
	id, err := strconv.ParseInt(c.Params(param), 10, 64)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid id")
	}
	return id, nil
}

func (h *portfolioHandler) create(c *fiber.Ctx) error {
	var req portfolio.CreatePortfolioReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	p, err := h.svc.CreatePortfolio(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": p})
}

func (h *portfolioHandler) list(c *fiber.Ctx) error {
	portfolios, err := h.svc.ListPortfolios(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": portfolios})
}

func (h *portfolioHandler) get(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	p, positions, err := h.svc.GetPortfolioWithPositions(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "portfolio not found"})
	}
	return c.JSON(fiber.Map{"data": p, "positions": positions})
}

func (h *portfolioHandler) update(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	var req portfolio.UpdatePortfolioReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	p, err := h.svc.UpdatePortfolio(c.Context(), id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": p})
}

func (h *portfolioHandler) delete(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeletePortfolio(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *portfolioHandler) summary(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	sum, err := h.svc.GetSummary(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "portfolio not found"})
	}
	return c.JSON(fiber.Map{"data": sum})
}

func (h *portfolioHandler) addPosition(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	var req portfolio.AddPositionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	pos, err := h.svc.AddPosition(c.Context(), id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": pos})
}

func (h *portfolioHandler) updatePosition(c *fiber.Ctx) error {
	posID, err := parseID(c, "posId")
	if err != nil {
		return err
	}
	var req portfolio.UpdatePositionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	pos, err := h.svc.UpdatePosition(c.Context(), posID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": pos})
}

func (h *portfolioHandler) deletePosition(c *fiber.Ctx) error {
	posID, err := parseID(c, "posId")
	if err != nil {
		return err
	}
	if err := h.svc.DeletePosition(c.Context(), posID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *portfolioHandler) addTransaction(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	var req portfolio.AddTransactionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	tx, err := h.svc.AddTransaction(c.Context(), id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": tx})
}

func (h *portfolioHandler) listTransactions(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	txs, total, err := h.svc.ListTransactions(c.Context(), id, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": txs, "total": total, "page": page, "page_size": limit})
}

func (h *portfolioHandler) equityCurve(c *fiber.Ctx) error {
	id, err := parseID(c, "id")
	if err != nil {
		return err
	}
	points, err := h.svc.GetEquityCurve(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"points": points})
}
```

### Step 4: Update routes.go to use real portfolio handler

In `backend/internal/api/routes.go`:

1. Add imports for `price` and `portfolio` packages.
2. Add `priceSvc` and `portfolioSvc` initialization.
3. Replace the stub portfolio routes with `ph.registerRoutes(v1)`.
4. Update `RegisterRoutes` signature to accept `rdb`.

The updated `RegisterRoutes` function signature and portfolio wiring:

```go
// At top of RegisterRoutes, after existing handlers:
priceSvc := price.NewService(rdb, "", "")
portfolioSvc := portfolio.NewService(db, priceSvc)
ph := newPortfolioHandler(portfolioSvc)

// Replace stub portfolio block with:
ph.registerRoutes(v1)
```

Add imports:
```go
"github.com/trader-claude/backend/internal/portfolio"
"github.com/trader-claude/backend/internal/price"
```

### Step 5: Run all backend tests

```bash
cd backend && go test ./... -v 2>&1 | tail -30
```

Expected: all tests PASS, including the new portfolio API tests.

### Step 6: Commit

```bash
git add backend/internal/api/portfolio.go backend/internal/api/portfolio_test.go backend/internal/api/routes.go
git commit -m "feat(phase6): add portfolio REST API handler, wire 12 routes"
```

---

## Task 5: Portfolio WebSocket — Live PnL Updates

**Files:**
- Create: `backend/internal/api/portfolio_ws.go`
- Modify: `backend/internal/api/routes.go`

### Step 1: Implement portfolio live WS handler

Create `backend/internal/api/portfolio_ws.go`:

```go
package api

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/contrib/websocket"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/portfolio"
)

// PortfolioUpdateMsg is the message pushed to WS clients every 5s
type PortfolioUpdateMsg struct {
	Type        string                `json:"type"`
	PortfolioID int64                 `json:"portfolio_id"`
	TotalValue  float64               `json:"total_value"`
	TotalPnL    float64               `json:"total_pnl"`
	TotalPnLPct float64               `json:"total_pnl_pct"`
	Positions   []PositionUpdateEntry `json:"positions"`
}

type PositionUpdateEntry struct {
	ID               int64   `json:"id"`
	Symbol           string  `json:"symbol"`
	CurrentPrice     float64 `json:"current_price"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
}

func portfolioLiveWS(svc *portfolio.Service) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		idStr := conn.Params("id")
		portfolioID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "invalid id"))
			return
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Send immediately on connect
		if err := sendPortfolioUpdate(conn, svc, portfolioID); err != nil {
			return
		}

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
			case <-ticker.C:
				if err := sendPortfolioUpdate(conn, svc, portfolioID); err != nil {
					return
				}
			}
		}
	}
}

func sendPortfolioUpdate(conn *websocket.Conn, svc *portfolio.Service, portfolioID int64) error {
	ctx := conn.Locals("ctx")
	if ctx == nil {
		ctx = conn.Context()
	}

	if err := svc.RecalculatePortfolio(conn.Context(), portfolioID); err != nil {
		log.Printf("portfolio WS: recalculate error for %d: %v", portfolioID, err)
		return nil // don't disconnect on price fetch failure
	}

	_, positions, err := svc.GetPortfolioWithPositions(conn.Context(), portfolioID)
	if err != nil {
		return err
	}

	sum, err := svc.GetSummary(conn.Context(), portfolioID)
	if err != nil {
		return err
	}

	entries := make([]PositionUpdateEntry, 0, len(positions))
	for _, pos := range positions {
		entries = append(entries, PositionUpdateEntry{
			ID:               pos.ID,
			Symbol:           pos.Symbol,
			CurrentPrice:     pos.CurrentPrice,
			UnrealizedPnL:    pos.UnrealizedPnL,
			UnrealizedPnLPct: pos.UnrealizedPnLPct,
		})
	}

	msg := PortfolioUpdateMsg{
		Type:        "portfolio_update",
		PortfolioID: portfolioID,
		TotalValue:  sum.TotalValue,
		TotalPnL:    sum.TotalPnL,
		TotalPnLPct: sum.TotalPnLPct,
		Positions:   entries,
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}

// ensure models is used (Position type reference)
var _ = models.Position{}
```

### Step 2: Register portfolio WS route in routes.go

In `backend/internal/api/routes.go`, after the replay WS line, add:

```go
// Portfolio live PnL WebSocket
app.Get("/ws/portfolio/:id/live", websocket.New(portfolioLiveWS(portfolioSvc)))
```

Note: `portfolioSvc` must be in scope — move its declaration before the WS section.

### Step 3: Build to verify

```bash
cd backend && go build ./...
```

Expected: no errors.

### Step 4: Commit

```bash
git add backend/internal/api/portfolio_ws.go backend/internal/api/routes.go
git commit -m "feat(phase6): add portfolio live PnL WebSocket endpoint"
```

---

## Task 6: Frontend Types + API Client + Zustand Store

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/api/portfolio.ts`
- Modify: `frontend/src/stores/index.ts`

### Step 1: Add Portfolio types to types/index.ts

At the end of `frontend/src/types/index.ts`, replace the existing `Portfolio` interface and add new types:

```typescript
// ── Portfolio types (Phase 6) ───────────────────────────────────────────────

export type PortfolioType = 'manual' | 'paper' | 'live'
export type TransactionType = 'buy' | 'sell' | 'deposit' | 'withdrawal'

export interface Portfolio {
  id: number
  name: string
  description: string
  type: PortfolioType
  currency: string
  strategy_name?: string
  symbol?: string
  market?: string
  is_live: boolean
  is_active: boolean
  initial_cash: number
  current_cash: number
  current_value: number
  created_at: string
  updated_at: string
}

export interface Position {
  id: number
  portfolio_id: number
  adapter_id: string
  symbol: string
  market: string
  quantity: number
  avg_cost: number
  current_price: number
  current_value: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  opened_at: string
}

export interface Transaction {
  id: number
  portfolio_id: number
  position_id?: number
  type: TransactionType
  adapter_id: string
  symbol: string
  quantity: number
  price: number
  fee: number
  notes: string
  executed_at: string
  created_at: string
}

export interface PortfolioSummary {
  portfolio_id: number
  total_value: number
  total_cost: number
  total_pnl: number
  total_pnl_pct: number
  day_change_pct: number
}

export interface PortfolioUpdateMsg {
  type: 'portfolio_update'
  portfolio_id: number
  total_value: number
  total_pnl: number
  total_pnl_pct: number
  positions: Array<{
    id: number
    symbol: string
    current_price: number
    unrealized_pnl: number
    unrealized_pnl_pct: number
  }>
}

export interface CreatePortfolioReq {
  name: string
  description?: string
  type: PortfolioType
  currency: string
  initial_cash: number
}

export interface AddPositionReq {
  adapter_id: string
  symbol: string
  market: string
  quantity: number
  avg_cost: number
  opened_at: string
}

export interface AddTransactionReq {
  position_id?: number
  type: TransactionType
  adapter_id?: string
  symbol?: string
  quantity: number
  price: number
  fee?: number
  notes?: string
  executed_at: string
}
```

### Step 2: Create portfolio API client

Create `frontend/src/api/portfolio.ts`:

```typescript
import apiClient from './client'
import type {
  Portfolio,
  Position,
  PortfolioSummary,
  Transaction,
  EquityPoint,
  CreatePortfolioReq,
  AddPositionReq,
  UpdatePositionReq,
  AddTransactionReq,
  PaginatedResponse,
} from '@/types'

// Portfolios
export const fetchPortfolios = () =>
  apiClient.get<{ data: Portfolio[] }>('/api/v1/portfolios').then((r) => r.data.data)

export const fetchPortfolio = (id: number) =>
  apiClient
    .get<{ data: Portfolio; positions: Position[] }>(`/api/v1/portfolios/${id}`)
    .then((r) => r.data)

export const createPortfolio = (req: CreatePortfolioReq) =>
  apiClient.post<{ data: Portfolio }>('/api/v1/portfolios', req).then((r) => r.data.data)

export const updatePortfolio = (id: number, req: Partial<CreatePortfolioReq>) =>
  apiClient.put<{ data: Portfolio }>(`/api/v1/portfolios/${id}`, req).then((r) => r.data.data)

export const deletePortfolio = (id: number) =>
  apiClient.delete(`/api/v1/portfolios/${id}`)

export const fetchPortfolioSummary = (id: number) =>
  apiClient.get<{ data: PortfolioSummary }>(`/api/v1/portfolios/${id}/summary`).then((r) => r.data.data)

// Positions
export const addPosition = (portfolioId: number, req: AddPositionReq) =>
  apiClient.post<{ data: Position }>(`/api/v1/portfolios/${portfolioId}/positions`, req).then((r) => r.data.data)

export const updatePosition = (portfolioId: number, posId: number, req: Partial<AddPositionReq>) =>
  apiClient.put<{ data: Position }>(`/api/v1/portfolios/${portfolioId}/positions/${posId}`, req).then((r) => r.data.data)

export const deletePosition = (portfolioId: number, posId: number) =>
  apiClient.delete(`/api/v1/portfolios/${portfolioId}/positions/${posId}`)

// Transactions
export const addTransaction = (portfolioId: number, req: AddTransactionReq) =>
  apiClient.post<{ data: Transaction }>(`/api/v1/portfolios/${portfolioId}/transactions`, req).then((r) => r.data.data)

export const fetchTransactions = (portfolioId: number, page = 1, limit = 20) =>
  apiClient
    .get<PaginatedResponse<Transaction>>(`/api/v1/portfolios/${portfolioId}/transactions`, {
      params: { page, limit },
    })
    .then((r) => r.data)

// Equity curve
export const fetchEquityCurve = (portfolioId: number) =>
  apiClient
    .get<{ points: EquityPoint[] }>(`/api/v1/portfolios/${portfolioId}/equity-curve`)
    .then((r) => r.data.points)
```

### Step 3: Update portfolioStore in stores/index.ts

Find the existing `portfolioStore` section in `frontend/src/stores/index.ts` and replace it:

```typescript
// ── Portfolio store ─────────────────────────────────────────────────────────

import type { Portfolio, Position, PortfolioSummary, PortfolioUpdateMsg } from '@/types'

interface PortfolioStore {
  portfolios: Portfolio[]
  activePortfolioId: number | null
  positions: Position[]
  summary: PortfolioSummary | null
  setPortfolios: (portfolios: Portfolio[]) => void
  setActivePortfolioId: (id: number | null) => void
  setPositions: (positions: Position[]) => void
  setSummary: (summary: PortfolioSummary | null) => void
  applyLiveUpdate: (msg: PortfolioUpdateMsg) => void
}

export const usePortfolioStore = create<PortfolioStore>()((set, get) => ({
  portfolios: [],
  activePortfolioId: null,
  positions: [],
  summary: null,
  setPortfolios: (portfolios) => set({ portfolios }),
  setActivePortfolioId: (id) => set({ activePortfolioId: id }),
  setPositions: (positions) => set({ positions }),
  setSummary: (summary) => set({ summary }),
  applyLiveUpdate: (msg) => {
    set((state) => ({
      summary: state.summary
        ? {
            ...state.summary,
            total_value: msg.total_value,
            total_pnl: msg.total_pnl,
            total_pnl_pct: msg.total_pnl_pct,
          }
        : null,
      positions: state.positions.map((pos) => {
        const update = msg.positions.find((p) => p.id === pos.id)
        if (!update) return pos
        return {
          ...pos,
          current_price: update.current_price,
          unrealized_pnl: update.unrealized_pnl,
          unrealized_pnl_pct: update.unrealized_pnl_pct,
          current_value: pos.quantity * update.current_price,
        }
      }),
    }))
  },
}))
```

### Step 4: Verify TypeScript compiles

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

### Step 5: Commit

```bash
git add frontend/src/types/index.ts frontend/src/api/portfolio.ts frontend/src/stores/index.ts
git commit -m "feat(phase6): add portfolio types, API client, and Zustand store"
```

---

## Task 7: Portfolio Page — Layout + Positions Table + Allocation Donut

**Files:**
- Modify: `frontend/src/pages/Portfolio.tsx`
- Create: `frontend/src/components/portfolio/PortfolioSelector.tsx`
- Create: `frontend/src/components/portfolio/SummaryCards.tsx`
- Create: `frontend/src/components/portfolio/PositionsTable.tsx`
- Create: `frontend/src/components/portfolio/AllocationDonut.tsx`

### Step 1: Create PortfolioSelector component

Create `frontend/src/components/portfolio/PortfolioSelector.tsx`:

```tsx
import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { fetchPortfolios } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'

interface Props {
  onNewPortfolio: () => void
}

export function PortfolioSelector({ onNewPortfolio }: Props) {
  const { activePortfolioId, setActivePortfolioId } = usePortfolioStore()
  const { data: portfolios = [] } = useQuery({
    queryKey: ['portfolios'],
    queryFn: fetchPortfolios,
  })

  return (
    <div className="flex items-center gap-3">
      <Select
        value={activePortfolioId?.toString() ?? ''}
        onValueChange={(v) => setActivePortfolioId(Number(v))}
      >
        <SelectTrigger className="w-52">
          <SelectValue placeholder="Select portfolio…" />
        </SelectTrigger>
        <SelectContent>
          {portfolios.map((p) => (
            <SelectItem key={p.id} value={p.id.toString()}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button variant="outline" size="sm" onClick={onNewPortfolio}>
        <Plus className="h-4 w-4 mr-1" />
        New Portfolio
      </Button>
    </div>
  )
}
```

### Step 2: Create SummaryCards component

Create `frontend/src/components/portfolio/SummaryCards.tsx`:

```tsx
import type { PortfolioSummary } from '@/types'

interface Props {
  summary: PortfolioSummary | null
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-lg border bg-card p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-2xl font-bold mt-1">{value}</p>
      {sub && <p className="text-xs text-muted-foreground mt-0.5">{sub}</p>}
    </div>
  )
}

function formatCurrency(v: number) {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(v)
}

function formatPct(v: number) {
  const sign = v >= 0 ? '+' : ''
  return `${sign}${v.toFixed(2)}%`
}

export function SummaryCards({ summary }: Props) {
  if (!summary) {
    return (
      <div className="grid grid-cols-4 gap-4">
        {['Total Value', 'Total PnL', 'PnL %', 'Day Change %'].map((l) => (
          <div key={l} className="rounded-lg border bg-card p-4 animate-pulse h-24" />
        ))}
      </div>
    )
  }

  const pnlColor = summary.total_pnl >= 0 ? 'text-green-500' : 'text-red-500'

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
      <StatCard label="Total Value" value={formatCurrency(summary.total_value)} />
      <StatCard
        label="Total PnL"
        value={<span className={pnlColor}>{formatCurrency(summary.total_pnl)}</span> as any}
      />
      <StatCard
        label="PnL %"
        value={<span className={pnlColor}>{formatPct(summary.total_pnl_pct)}</span> as any}
      />
      <StatCard
        label="Day Change"
        value={<span className={summary.day_change_pct >= 0 ? 'text-green-500' : 'text-red-500'}>{formatPct(summary.day_change_pct)}</span> as any}
      />
    </div>
  )
}
```

### Step 3: Create PositionsTable component

Create `frontend/src/components/portfolio/PositionsTable.tsx`:

```tsx
import { useState } from 'react'
import { Pencil, Trash2, Plus } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { deletePosition } from '@/api/portfolio'
import type { Position } from '@/types'

interface Props {
  portfolioId: number
  positions: Position[]
  highlightedSymbol?: string
  onAddPosition: () => void
  onEditPosition: (pos: Position) => void
}

function pnlClass(v: number) {
  return v >= 0 ? 'text-green-500' : 'text-red-500'
}

function fmt(v: number, decimals = 2) {
  return v.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
}

export function PositionsTable({ portfolioId, positions, highlightedSymbol, onAddPosition, onEditPosition }: Props) {
  const qc = useQueryClient()
  const totalValue = positions.reduce((sum, p) => sum + p.current_value, 0)

  const deleteMut = useMutation({
    mutationFn: (posId: number) => deletePosition(portfolioId, posId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['portfolio', portfolioId] }),
  })

  return (
    <div className="rounded-lg border bg-card">
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <h3 className="font-semibold">Positions</h3>
        <Button size="sm" onClick={onAddPosition}>
          <Plus className="h-4 w-4 mr-1" />
          Add Position
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Asset</TableHead>
            <TableHead className="text-right">Qty</TableHead>
            <TableHead className="text-right">Avg Cost</TableHead>
            <TableHead className="text-right">Price</TableHead>
            <TableHead className="text-right">Value</TableHead>
            <TableHead className="text-right">PnL</TableHead>
            <TableHead className="text-right">PnL %</TableHead>
            <TableHead className="text-right">Weight</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {positions.length === 0 && (
            <TableRow>
              <TableCell colSpan={9} className="text-center text-muted-foreground py-8">
                No positions yet. Add one to get started.
              </TableCell>
            </TableRow>
          )}
          {positions.map((pos) => {
            const weight = totalValue > 0 ? (pos.current_value / totalValue) * 100 : 0
            const isHighlighted = highlightedSymbol === pos.symbol
            return (
              <TableRow
                key={pos.id}
                className={isHighlighted ? 'bg-primary/10' : ''}
              >
                <TableCell className="font-medium">{pos.symbol}</TableCell>
                <TableCell className="text-right">{fmt(pos.quantity, 4)}</TableCell>
                <TableCell className="text-right">${fmt(pos.avg_cost)}</TableCell>
                <TableCell className="text-right">${fmt(pos.current_price)}</TableCell>
                <TableCell className="text-right">${fmt(pos.current_value)}</TableCell>
                <TableCell className={`text-right ${pnlClass(pos.unrealized_pnl)}`}>
                  ${fmt(pos.unrealized_pnl)}
                </TableCell>
                <TableCell className={`text-right ${pnlClass(pos.unrealized_pnl_pct)}`}>
                  {fmt(pos.unrealized_pnl_pct)}%
                </TableCell>
                <TableCell className="text-right">{fmt(weight)}%</TableCell>
                <TableCell>
                  <div className="flex gap-1 justify-end">
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onEditPosition(pos)}>
                      <Pencil className="h-3 w-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-destructive"
                      onClick={() => deleteMut.mutate(pos.id)}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
```

### Step 4: Create AllocationDonut component

Create `frontend/src/components/portfolio/AllocationDonut.tsx`:

```tsx
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts'
import type { Position } from '@/types'

const COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#3b82f6', '#a855f7', '#ec4899', '#14b8a6']

interface Props {
  positions: Position[]
  onHover: (symbol: string | null) => void
}

export function AllocationDonut({ positions, onHover }: Props) {
  const totalValue = positions.reduce((sum, p) => sum + p.current_value, 0)

  const data = positions.map((p) => ({
    name: p.symbol,
    value: totalValue > 0 ? (p.current_value / totalValue) * 100 : 0,
  }))

  if (data.length === 0) {
    return (
      <div className="rounded-lg border bg-card h-full flex items-center justify-center p-8">
        <p className="text-muted-foreground text-sm">Add positions to see allocation</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border bg-card p-4 h-full">
      <h3 className="font-semibold mb-3">Allocation</h3>
      <ResponsiveContainer width="100%" height={260}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={60}
            outerRadius={100}
            paddingAngle={2}
            dataKey="value"
            onMouseEnter={(_, index) => onHover(data[index].name)}
            onMouseLeave={() => onHover(null)}
          >
            {data.map((_, index) => (
              <Cell key={index} fill={COLORS[index % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
        </PieChart>
      </ResponsiveContainer>
      <div className="flex flex-wrap gap-2 mt-2">
        {data.map((d, i) => (
          <div key={d.name} className="flex items-center gap-1 text-xs">
            <span className="h-2 w-2 rounded-full" style={{ background: COLORS[i % COLORS.length] }} />
            <span>{d.name}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
```

### Step 5: Assemble Portfolio page (layout + tables + donut)

Replace `frontend/src/pages/Portfolio.tsx`:

```tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { PortfolioSelector } from '@/components/portfolio/PortfolioSelector'
import { SummaryCards } from '@/components/portfolio/SummaryCards'
import { PositionsTable } from '@/components/portfolio/PositionsTable'
import { AllocationDonut } from '@/components/portfolio/AllocationDonut'
import { fetchPortfolio, fetchPortfolioSummary } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'
import type { Position } from '@/types'

export function Portfolio() {
  const { activePortfolioId, setActivePortfolioId, setPositions, setSummary, summary, positions } =
    usePortfolioStore()
  const [highlightedSymbol, setHighlightedSymbol] = useState<string | null>(null)
  const [showNewPortfolioModal, setShowNewPortfolioModal] = useState(false)
  const [editingPosition, setEditingPosition] = useState<Position | null>(null)
  const [showAddPosition, setShowAddPosition] = useState(false)

  // Load portfolio + positions
  useQuery({
    queryKey: ['portfolio', activePortfolioId],
    queryFn: () => fetchPortfolio(activePortfolioId!),
    enabled: !!activePortfolioId,
    onSuccess: (data) => setPositions(data.positions),
  })

  // Load summary
  useQuery({
    queryKey: ['portfolio-summary', activePortfolioId],
    queryFn: () => fetchPortfolioSummary(activePortfolioId!),
    enabled: !!activePortfolioId,
    refetchInterval: 30_000,
    onSuccess: setSummary,
  })

  return (
    <div className="space-y-6">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Portfolio</h1>
        <PortfolioSelector onNewPortfolio={() => setShowNewPortfolioModal(true)} />
      </div>

      {/* Summary cards */}
      <SummaryCards summary={activePortfolioId ? summary : null} />

      {!activePortfolioId && (
        <div className="rounded-lg border bg-card p-12 text-center text-muted-foreground">
          Select or create a portfolio to get started.
        </div>
      )}

      {activePortfolioId && (
        <>
          {/* Main split: table + donut */}
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
            <div className="lg:col-span-3">
              <PositionsTable
                portfolioId={activePortfolioId}
                positions={positions}
                highlightedSymbol={highlightedSymbol ?? undefined}
                onAddPosition={() => setShowAddPosition(true)}
                onEditPosition={setEditingPosition}
              />
            </div>
            <div className="lg:col-span-2">
              <AllocationDonut positions={positions} onHover={setHighlightedSymbol} />
            </div>
          </div>

          {/* Bottom tabs */}
          <Tabs defaultValue="equity">
            <TabsList>
              <TabsTrigger value="equity">Equity Curve</TabsTrigger>
              <TabsTrigger value="transactions">Transactions</TabsTrigger>
            </TabsList>
            <TabsContent value="equity" className="mt-4">
              <div className="rounded-lg border bg-card p-6 text-muted-foreground text-sm text-center">
                Equity curve — implemented in Task 8
              </div>
            </TabsContent>
            <TabsContent value="transactions" className="mt-4">
              <div className="rounded-lg border bg-card p-6 text-muted-foreground text-sm text-center">
                Transaction history — implemented in Task 8
              </div>
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  )
}
```

### Step 6: Verify frontend builds

```bash
cd frontend && npx tsc --noEmit
```

Expected: no type errors.

### Step 7: Commit

```bash
git add frontend/src/pages/Portfolio.tsx frontend/src/components/portfolio/
git commit -m "feat(phase6): add portfolio page layout, positions table, allocation donut"
```

---

## Task 8: Equity Curve + Transaction Table + Live WS Hook + Modals

**Files:**
- Create: `frontend/src/components/portfolio/EquityCurveChart.tsx`
- Create: `frontend/src/components/portfolio/TransactionTable.tsx`
- Create: `frontend/src/components/portfolio/NewPortfolioModal.tsx`
- Create: `frontend/src/components/portfolio/AddPositionModal.tsx`
- Create: `frontend/src/hooks/usePortfolioLive.ts`
- Modify: `frontend/src/pages/Portfolio.tsx`

### Step 1: Create EquityCurveChart

Create `frontend/src/components/portfolio/EquityCurveChart.tsx`:

```tsx
import { useQuery } from '@tanstack/react-query'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { fetchEquityCurve } from '@/api/portfolio'

interface Props {
  portfolioId: number
}

export function EquityCurveChart({ portfolioId }: Props) {
  const { data: points = [], isLoading } = useQuery({
    queryKey: ['equity-curve', portfolioId],
    queryFn: () => fetchEquityCurve(portfolioId),
  })

  if (isLoading) return <div className="h-48 animate-pulse rounded bg-muted" />

  if (points.length === 0) {
    return (
      <div className="h-48 flex items-center justify-center text-muted-foreground text-sm">
        No transaction history yet
      </div>
    )
  }

  const chartData = points.map((p) => ({
    time: new Date(p.timestamp).toLocaleDateString(),
    value: p.value,
  }))

  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
        <XAxis dataKey="time" tick={{ fontSize: 11 }} />
        <YAxis
          tick={{ fontSize: 11 }}
          tickFormatter={(v) => `$${(v / 1000).toFixed(0)}k`}
        />
        <Tooltip formatter={(v: number) => [`$${v.toLocaleString()}`, 'Portfolio Value']} />
        <Line type="monotone" dataKey="value" stroke="#6366f1" dot={false} strokeWidth={2} />
      </LineChart>
    </ResponsiveContainer>
  )
}
```

### Step 2: Create TransactionTable

Create `frontend/src/components/portfolio/TransactionTable.tsx`:

```tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { fetchTransactions } from '@/api/portfolio'

interface Props {
  portfolioId: number
  onAddTransaction: () => void
}

export function TransactionTable({ portfolioId, onAddTransaction }: Props) {
  const [page, setPage] = useState(1)
  const { data } = useQuery({
    queryKey: ['transactions', portfolioId, page],
    queryFn: () => fetchTransactions(portfolioId, page, 10),
  })

  const txs = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / 10)

  const typeColor = (t: string) => {
    if (t === 'buy' || t === 'deposit') return 'text-green-500'
    return 'text-red-500'
  }

  return (
    <div className="rounded-lg border bg-card">
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <h3 className="font-semibold">Transaction History</h3>
        <Button size="sm" variant="outline" onClick={onAddTransaction}>
          + Log Transaction
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Date</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Symbol</TableHead>
            <TableHead className="text-right">Qty</TableHead>
            <TableHead className="text-right">Price</TableHead>
            <TableHead className="text-right">Fee</TableHead>
            <TableHead>Notes</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {txs.length === 0 && (
            <TableRow>
              <TableCell colSpan={7} className="text-center text-muted-foreground py-8">
                No transactions yet.
              </TableCell>
            </TableRow>
          )}
          {txs.map((tx) => (
            <TableRow key={tx.id}>
              <TableCell className="text-sm">
                {new Date(tx.executed_at).toLocaleDateString()}
              </TableCell>
              <TableCell className={`font-medium capitalize ${typeColor(tx.type)}`}>
                {tx.type}
              </TableCell>
              <TableCell>{tx.symbol || '—'}</TableCell>
              <TableCell className="text-right">{tx.quantity > 0 ? tx.quantity : '—'}</TableCell>
              <TableCell className="text-right">${tx.price.toFixed(2)}</TableCell>
              <TableCell className="text-right">{tx.fee > 0 ? `$${tx.fee.toFixed(2)}` : '—'}</TableCell>
              <TableCell className="text-muted-foreground text-sm">{tx.notes || '—'}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {totalPages > 1 && (
        <div className="flex justify-end gap-2 p-3 border-t">
          <Button variant="outline" size="sm" disabled={page === 1} onClick={() => setPage((p) => p - 1)}>
            Previous
          </Button>
          <span className="text-sm self-center text-muted-foreground">
            {page} / {totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={page === totalPages} onClick={() => setPage((p) => p + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  )
}
```

### Step 3: Create NewPortfolioModal

Create `frontend/src/components/portfolio/NewPortfolioModal.tsx`:

```tsx
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { createPortfolio } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'
import type { PortfolioType } from '@/types'

interface Props {
  open: boolean
  onClose: () => void
}

export function NewPortfolioModal({ open, onClose }: Props) {
  const qc = useQueryClient()
  const { setActivePortfolioId } = usePortfolioStore()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [type, setType] = useState<PortfolioType>('manual')
  const [currency, setCurrency] = useState('USD')
  const [initialCash, setInitialCash] = useState('0')

  const mut = useMutation({
    mutationFn: createPortfolio,
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['portfolios'] })
      setActivePortfolioId(p.id)
      onClose()
      setName('')
      setDescription('')
      setInitialCash('0')
    },
  })

  const handleSubmit = () => {
    if (!name.trim()) return
    mut.mutate({
      name: name.trim(),
      description,
      type,
      currency,
      initial_cash: parseFloat(initialCash) || 0,
    })
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New Portfolio</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label>Name *</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="My Crypto Portfolio" />
          </div>
          <div className="space-y-1">
            <Label>Description</Label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Type</Label>
              <Select value={type} onValueChange={(v) => setType(v as PortfolioType)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="manual">Manual</SelectItem>
                  <SelectItem value="paper">Paper</SelectItem>
                  <SelectItem value="live">Live</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Currency</Label>
              <Select value={currency} onValueChange={setCurrency}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="USD">USD</SelectItem>
                  <SelectItem value="EUR">EUR</SelectItem>
                  <SelectItem value="BTC">BTC</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1">
            <Label>Initial Cash</Label>
            <Input
              type="number"
              min="0"
              step="100"
              value={initialCash}
              onChange={(e) => setInitialCash(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name.trim() || mut.isPending}>
            {mut.isPending ? 'Creating…' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

### Step 4: Create AddPositionModal

Create `frontend/src/components/portfolio/AddPositionModal.tsx`:

```tsx
import { useState } from 'react'
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { addPosition } from '@/api/portfolio'
import { fetchMarkets } from '@/api/markets'
import type { Position } from '@/types'

interface Props {
  open: boolean
  portfolioId: number
  editingPosition?: Position | null
  onClose: () => void
}

export function AddPositionModal({ open, portfolioId, editingPosition, onClose }: Props) {
  const qc = useQueryClient()
  const [adapterID, setAdapterID] = useState(editingPosition?.adapter_id ?? 'binance')
  const [symbol, setSymbol] = useState(editingPosition?.symbol ?? '')
  const [market, setMarket] = useState(editingPosition?.market ?? 'crypto')
  const [quantity, setQuantity] = useState(editingPosition?.quantity?.toString() ?? '')
  const [avgCost, setAvgCost] = useState(editingPosition?.avg_cost?.toString() ?? '')
  const [openedAt, setOpenedAt] = useState(
    editingPosition?.opened_at?.slice(0, 10) ?? new Date().toISOString().slice(0, 10)
  )

  const mut = useMutation({
    mutationFn: () =>
      addPosition(portfolioId, {
        adapter_id: adapterID,
        symbol: symbol.trim().toUpperCase(),
        market,
        quantity: parseFloat(quantity),
        avg_cost: parseFloat(avgCost),
        opened_at: new Date(openedAt).toISOString(),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['portfolio', portfolioId] })
      onClose()
    },
  })

  const isValid = symbol.trim() && parseFloat(quantity) > 0 && parseFloat(avgCost) > 0

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editingPosition ? 'Edit Position' : 'Add Position'}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Adapter</Label>
              <Select value={adapterID} onValueChange={setAdapterID}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="binance">Binance (Crypto)</SelectItem>
                  <SelectItem value="yahoo">Yahoo (Stocks)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>Market</Label>
              <Select value={market} onValueChange={setMarket}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="crypto">Crypto</SelectItem>
                  <SelectItem value="stock">Stock</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-1">
            <Label>Symbol *</Label>
            <Input
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              placeholder={adapterID === 'binance' ? 'BTCUSDT' : 'AAPL'}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label>Quantity *</Label>
              <Input type="number" min="0" step="any" value={quantity} onChange={(e) => setQuantity(e.target.value)} />
            </div>
            <div className="space-y-1">
              <Label>Avg Cost *</Label>
              <Input type="number" min="0" step="any" value={avgCost} onChange={(e) => setAvgCost(e.target.value)} />
            </div>
          </div>
          <div className="space-y-1">
            <Label>Opened At</Label>
            <Input type="date" value={openedAt} onChange={(e) => setOpenedAt(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={() => mut.mutate()} disabled={!isValid || mut.isPending}>
            {mut.isPending ? 'Saving…' : editingPosition ? 'Save' : 'Add'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

### Step 5: Create usePortfolioLive hook

Create `frontend/src/hooks/usePortfolioLive.ts`:

```typescript
import { useEffect, useRef } from 'react'
import { usePortfolioStore } from '@/stores'
import type { PortfolioUpdateMsg } from '@/types'

const WS_URL = (import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080') as string

export function usePortfolioLive(portfolioId: number | null) {
  const { applyLiveUpdate } = usePortfolioStore()
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!portfolioId) return

    const ws = new WebSocket(`${WS_URL}/ws/portfolio/${portfolioId}/live`)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg: PortfolioUpdateMsg = JSON.parse(event.data)
        if (msg.type === 'portfolio_update') {
          applyLiveUpdate(msg)
        }
      } catch {
        // ignore parse errors
      }
    }

    ws.onerror = (e) => console.warn('portfolio WS error', e)

    return () => {
      ws.close()
    }
  }, [portfolioId, applyLiveUpdate])
}
```

### Step 6: Wire everything into Portfolio.tsx

Replace `frontend/src/pages/Portfolio.tsx` with the complete version:

```tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { PortfolioSelector } from '@/components/portfolio/PortfolioSelector'
import { SummaryCards } from '@/components/portfolio/SummaryCards'
import { PositionsTable } from '@/components/portfolio/PositionsTable'
import { AllocationDonut } from '@/components/portfolio/AllocationDonut'
import { EquityCurveChart } from '@/components/portfolio/EquityCurveChart'
import { TransactionTable } from '@/components/portfolio/TransactionTable'
import { NewPortfolioModal } from '@/components/portfolio/NewPortfolioModal'
import { AddPositionModal } from '@/components/portfolio/AddPositionModal'
import { fetchPortfolio, fetchPortfolioSummary } from '@/api/portfolio'
import { usePortfolioStore } from '@/stores'
import { usePortfolioLive } from '@/hooks/usePortfolioLive'
import type { Position } from '@/types'

export function Portfolio() {
  const { activePortfolioId, setPositions, setSummary, summary, positions } =
    usePortfolioStore()
  const [highlightedSymbol, setHighlightedSymbol] = useState<string | null>(null)
  const [showNewPortfolioModal, setShowNewPortfolioModal] = useState(false)
  const [editingPosition, setEditingPosition] = useState<Position | null>(null)
  const [showAddPosition, setShowAddPosition] = useState(false)
  const [showAddTransaction, setShowAddTransaction] = useState(false)

  usePortfolioLive(activePortfolioId)

  useQuery({
    queryKey: ['portfolio', activePortfolioId],
    queryFn: () => fetchPortfolio(activePortfolioId!),
    enabled: !!activePortfolioId,
    onSuccess: (data) => setPositions(data.positions),
  })

  useQuery({
    queryKey: ['portfolio-summary', activePortfolioId],
    queryFn: () => fetchPortfolioSummary(activePortfolioId!),
    enabled: !!activePortfolioId,
    refetchInterval: 30_000,
    onSuccess: setSummary,
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Portfolio</h1>
        <PortfolioSelector onNewPortfolio={() => setShowNewPortfolioModal(true)} />
      </div>

      <SummaryCards summary={activePortfolioId ? summary : null} />

      {!activePortfolioId && (
        <div className="rounded-lg border bg-card p-12 text-center text-muted-foreground">
          Select or create a portfolio to get started.
        </div>
      )}

      {activePortfolioId && (
        <>
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
            <div className="lg:col-span-3">
              <PositionsTable
                portfolioId={activePortfolioId}
                positions={positions}
                highlightedSymbol={highlightedSymbol ?? undefined}
                onAddPosition={() => setShowAddPosition(true)}
                onEditPosition={setEditingPosition}
              />
            </div>
            <div className="lg:col-span-2">
              <AllocationDonut positions={positions} onHover={setHighlightedSymbol} />
            </div>
          </div>

          <Tabs defaultValue="equity">
            <TabsList>
              <TabsTrigger value="equity">Equity Curve</TabsTrigger>
              <TabsTrigger value="transactions">Transactions</TabsTrigger>
            </TabsList>
            <TabsContent value="equity" className="mt-4">
              <div className="rounded-lg border bg-card p-6">
                <EquityCurveChart portfolioId={activePortfolioId} />
              </div>
            </TabsContent>
            <TabsContent value="transactions" className="mt-4">
              <TransactionTable
                portfolioId={activePortfolioId}
                onAddTransaction={() => setShowAddTransaction(true)}
              />
            </TabsContent>
          </Tabs>
        </>
      )}

      <NewPortfolioModal
        open={showNewPortfolioModal}
        onClose={() => setShowNewPortfolioModal(false)}
      />

      <AddPositionModal
        open={showAddPosition || !!editingPosition}
        portfolioId={activePortfolioId ?? 0}
        editingPosition={editingPosition}
        onClose={() => {
          setShowAddPosition(false)
          setEditingPosition(null)
        }}
      />
    </div>
  )
}
```

### Step 7: Final build check

```bash
cd frontend && npx tsc --noEmit && npx vite build --mode development 2>&1 | tail -20
```

Expected: no errors, build succeeds.

### Step 8: Run all backend tests

```bash
cd backend && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

### Step 9: Final commit

```bash
git add frontend/src/components/portfolio/ frontend/src/hooks/ frontend/src/pages/Portfolio.tsx
git commit -m "feat(phase6): add equity curve, transaction table, live WS hook, and modals"
```

---

## Final Verification

```bash
# Backend: all tests pass
cd backend && go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)"

# Frontend: type check passes
cd frontend && npx tsc --noEmit

# Backend: compiles clean
cd backend && go build ./...
```

After all tasks complete, update `docs/phases.md` to mark Phase 6 as ✅ COMPLETE.

```bash
git add docs/phases.md
git commit -m "docs(phase6): mark Phase 6 complete"
```
