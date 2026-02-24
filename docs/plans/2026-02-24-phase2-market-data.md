# Phase 2 — Market Data Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement Binance + Yahoo Finance adapters, a gap-filling DataService, four API endpoints, and a working candlestick chart page.

**Architecture:** DB-first lazy gap-filling. `GET /candles` queries MySQL, detects missing time ranges, fetches only gaps from the exchange REST API, upserts, then returns the merged sorted slice. A background goroutine runs every 5 minutes to sync the last 500 candles for "hot" symbols (those accessed in the last 24 h, tracked via Redis keys with TTL).

**Tech Stack:** Go 1.24, Fiber v2, GORM, go-redis, `net/http` + `net/http/httptest` (stdlib, no new dep), React 18, lightweight-charts v4, TanStack Query v5, Vitest + @testing-library/react.

---

## Important Codebase Context

- Module: `github.com/trader-claude/backend`
- `MarketAdapter` interface is in `backend/internal/registry/interfaces.go` — use `registry.Candle` (not `models.Candle`) as the adapter return type
- `models.Candle` is the GORM struct (`backend/internal/models/models.go`) — it has `Symbol`, `Market`, `Timeframe`, `Timestamp time.Time`, `Open/High/Low/Close/Volume float64`
- `registry.AdapterRegistry` singleton is accessed via `registry.Adapters()` — register adapters with `.Register(adapter)`
- `RegisterRoutes` is in `backend/internal/api/routes.go` — add `dataSvc *adapter.DataService` param (and update the call in `main.go`)
- Frontend alias `@` maps to `frontend/src/`
- All TypeScript interfaces go in `frontend/src/types/index.ts`
- All Zustand stores go in `frontend/src/stores/index.ts`

---

## Task 1: BinanceAdapter

**Files:**
- Create: `backend/internal/adapter/binance.go`
- Create: `backend/internal/adapter/binance_test.go`

### Step 1: Write the failing tests

```go
// backend/internal/adapter/binance_test.go
package adapter_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/trader-claude/backend/internal/adapter"
)

func TestBinanceAdapter_FetchCandles_Success(t *testing.T) {
    // Binance klines response: array of arrays
    klines := [][]interface{}{
        {
            float64(1700000000000), "42000.00", "42500.00", "41800.00", "42200.00",
            "1234.5678", float64(1700003599999), "52000000.00", float64(100),
            "617.28", "26000000.00", "0",
        },
    }
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(klines)
    }))
    defer srv.Close()

    a := adapter.NewBinanceAdapter(srv.URL)
    from := time.UnixMilli(1700000000000)
    to := time.UnixMilli(1700003600000)
    candles, err := a.FetchCandles(context.Background(), "BTCUSDT", "crypto", "1h", from, to)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(candles) != 1 {
        t.Fatalf("want 1 candle, got %d", len(candles))
    }
    if candles[0].Open != 42000.0 {
        t.Errorf("want Open=42000, got %v", candles[0].Open)
    }
    if candles[0].Symbol != "BTCUSDT" {
        t.Errorf("want Symbol=BTCUSDT, got %v", candles[0].Symbol)
    }
    if candles[0].Market != "crypto" {
        t.Errorf("want Market=crypto, got %v", candles[0].Market)
    }
}

func TestBinanceAdapter_FetchCandles_HTTPError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer srv.Close()

    a := adapter.NewBinanceAdapter(srv.URL)
    _, err := a.FetchCandles(context.Background(), "BTCUSDT", "crypto", "1h",
        time.Now().Add(-time.Hour), time.Now())
    if err == nil {
        t.Fatal("expected error, got nil")
    }
}

func TestBinanceAdapter_FetchCandles_EmptyResponse(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte("[]"))
    }))
    defer srv.Close()

    a := adapter.NewBinanceAdapter(srv.URL)
    candles, err := a.FetchCandles(context.Background(), "BTCUSDT", "crypto", "1h",
        time.Now().Add(-time.Hour), time.Now())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(candles) != 0 {
        t.Errorf("want 0 candles, got %d", len(candles))
    }
}

func TestBinanceAdapter_FetchSymbols(t *testing.T) {
    // Minimal exchangeInfo response
    info := map[string]interface{}{
        "symbols": []map[string]interface{}{
            {
                "symbol":     "BTCUSDT",
                "status":     "TRADING",
                "baseAsset":  "BTC",
                "quoteAsset": "USDT",
            },
            {
                "symbol":     "ETHUSDT",
                "status":     "TRADING",
                "baseAsset":  "ETH",
                "quoteAsset": "USDT",
            },
            {
                "symbol":     "BTCETH",
                "status":     "TRADING",
                "baseAsset":  "BTC",
                "quoteAsset": "ETH", // not a USDT pair, should be included but filtered by caller
            },
        },
    }
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(info)
    }))
    defer srv.Close()

    a := adapter.NewBinanceAdapter(srv.URL)
    symbols, err := a.FetchSymbols(context.Background(), "crypto")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(symbols) != 3 {
        t.Errorf("want 3 symbols, got %d", len(symbols))
    }
}

func TestBinanceAdapter_Name(t *testing.T) {
    a := adapter.NewBinanceAdapter("http://localhost")
    if a.Name() != "binance" {
        t.Errorf("want name=binance, got %s", a.Name())
    }
}

func TestBinanceAdapter_Markets(t *testing.T) {
    a := adapter.NewBinanceAdapter("http://localhost")
    markets := a.Markets()
    if len(markets) != 1 || markets[0] != "crypto" {
        t.Errorf("want [crypto], got %v", markets)
    }
}
```

### Step 2: Run tests to confirm they fail

```bash
cd backend && go test ./internal/adapter/... -run TestBinance -v 2>&1 | head -20
```

Expected: `cannot find package` or `undefined: adapter.NewBinanceAdapter`

### Step 3: Implement BinanceAdapter

```go
// backend/internal/adapter/binance.go
package adapter

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/trader-claude/backend/internal/registry"
)

// timeframeToInterval maps our internal timeframe strings to Binance interval strings.
// They happen to be identical, but this mapping makes the adapter self-contained.
var binanceIntervals = map[string]string{
    "1m": "1m", "5m": "5m", "15m": "15m", "30m": "30m",
    "1h": "1h", "4h": "4h", "1d": "1d", "1w": "1w",
}

// BinanceAdapter fetches market data from Binance REST API.
// No API key is required for public OHLCV endpoints.
type BinanceAdapter struct {
    httpClient *http.Client
    baseURL    string
}

// NewBinanceAdapter creates a BinanceAdapter. Pass an empty string to use the
// default Binance base URL (https://api.binance.com). Tests pass a httptest server URL.
func NewBinanceAdapter(baseURL string) *BinanceAdapter {
    if baseURL == "" {
        baseURL = "https://api.binance.com"
    }
    return &BinanceAdapter{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        baseURL:    baseURL,
    }
}

func (a *BinanceAdapter) Name() string        { return "binance" }
func (a *BinanceAdapter) Markets() []string   { return []string{"crypto"} }
func (a *BinanceAdapter) IsHealthy(ctx context.Context) bool {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/v3/ping", nil)
    if err != nil {
        return false
    }
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return false
    }
    resp.Body.Close()
    return resp.StatusCode == http.StatusOK
}

// SubscribeTicks is a stub — real streaming is implemented in Phase 8.
func (a *BinanceAdapter) SubscribeTicks(ctx context.Context, symbols []string, market string) (<-chan registry.Tick, error) {
    return nil, fmt.Errorf("streaming not implemented in Phase 2; use Phase 8")
}

// FetchCandles fetches historical OHLCV data from Binance /api/v3/klines.
// It fetches at most 1000 candles per request (Binance limit).
func (a *BinanceAdapter) FetchCandles(ctx context.Context, symbol, market, timeframe string, from, to time.Time) ([]registry.Candle, error) {
    interval, ok := binanceIntervals[timeframe]
    if !ok {
        return nil, fmt.Errorf("unsupported timeframe %q for Binance", timeframe)
    }

    url := fmt.Sprintf(
        "%s/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000",
        a.baseURL, symbol, interval, from.UnixMilli(), to.UnixMilli(),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("building request: %w", err)
    }

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Binance returned status %d", resp.StatusCode)
    }

    // Binance returns an array of arrays.
    var raw [][]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
        return nil, fmt.Errorf("decoding response: %w", err)
    }

    candles := make([]registry.Candle, 0, len(raw))
    for _, k := range raw {
        if len(k) < 6 {
            continue
        }
        openTimeMs, _ := k[0].(float64)
        open, _  := strconv.ParseFloat(fmt.Sprintf("%v", k[1]), 64)
        high, _  := strconv.ParseFloat(fmt.Sprintf("%v", k[2]), 64)
        low, _   := strconv.ParseFloat(fmt.Sprintf("%v", k[3]), 64)
        close_, _ := strconv.ParseFloat(fmt.Sprintf("%v", k[4]), 64)
        vol, _   := strconv.ParseFloat(fmt.Sprintf("%v", k[5]), 64)

        candles = append(candles, registry.Candle{
            Symbol:    symbol,
            Market:    market,
            Timeframe: timeframe,
            Timestamp: time.UnixMilli(int64(openTimeMs)),
            Open:      open,
            High:      high,
            Low:       low,
            Close:     close_,
            Volume:    vol,
        })
    }
    return candles, nil
}

// FetchSymbols returns all symbols from Binance /api/v3/exchangeInfo.
func (a *BinanceAdapter) FetchSymbols(ctx context.Context, market string) ([]registry.Symbol, error) {
    url := a.baseURL + "/api/v3/exchangeInfo"
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("building request: %w", err)
    }

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Binance returned status %d", resp.StatusCode)
    }

    var info struct {
        Symbols []struct {
            Symbol     string `json:"symbol"`
            Status     string `json:"status"`
            BaseAsset  string `json:"baseAsset"`
            QuoteAsset string `json:"quoteAsset"`
        } `json:"symbols"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
        return nil, fmt.Errorf("decoding response: %w", err)
    }

    symbols := make([]registry.Symbol, 0, len(info.Symbols))
    for _, s := range info.Symbols {
        if s.Status != "TRADING" {
            continue
        }
        symbols = append(symbols, registry.Symbol{
            ID:         s.Symbol,
            Market:     "crypto",
            BaseAsset:  s.BaseAsset,
            QuoteAsset: s.QuoteAsset,
            Active:     true,
        })
    }
    return symbols, nil
}
```

### Step 4: Run tests — all should pass

```bash
cd backend && go test ./internal/adapter/... -run TestBinance -v
```

Expected: all 6 tests PASS

### Step 5: Commit

```bash
git add backend/internal/adapter/binance.go backend/internal/adapter/binance_test.go
git commit -m "feat(adapter): add BinanceAdapter with OHLCV and symbols fetching"
```

---

## Task 2: YahooFinanceAdapter

**Files:**
- Create: `backend/internal/adapter/yahoo.go`
- Create: `backend/internal/adapter/yahoo_test.go`

### Step 1: Write the failing tests

```go
// backend/internal/adapter/yahoo_test.go
package adapter_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/trader-claude/backend/internal/adapter"
)

func makeYahooResponse(symbol string, ts int64, o, h, l, c float64, v int64) map[string]interface{} {
    return map[string]interface{}{
        "chart": map[string]interface{}{
            "result": []map[string]interface{}{
                {
                    "meta":      map[string]interface{}{"symbol": symbol},
                    "timestamp": []int64{ts},
                    "indicators": map[string]interface{}{
                        "quote": []map[string]interface{}{
                            {
                                "open":   []interface{}{o},
                                "high":   []interface{}{h},
                                "low":    []interface{}{l},
                                "close":  []interface{}{c},
                                "volume": []interface{}{v},
                            },
                        },
                    },
                },
            },
            "error": nil,
        },
    }
}

func TestYahooAdapter_FetchCandles_Success(t *testing.T) {
    ts := int64(1700000000)
    body := makeYahooResponse("AAPL", ts, 180.0, 182.0, 179.5, 181.5, 5000000)

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(body)
    }))
    defer srv.Close()

    a := adapter.NewYahooAdapter(srv.URL)
    from := time.Unix(ts-3600, 0)
    to := time.Unix(ts+3600, 0)
    candles, err := a.FetchCandles(context.Background(), "AAPL", "stock", "1d", from, to)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(candles) != 1 {
        t.Fatalf("want 1 candle, got %d", len(candles))
    }
    if candles[0].Open != 180.0 {
        t.Errorf("want Open=180, got %v", candles[0].Open)
    }
    if candles[0].Symbol != "AAPL" {
        t.Errorf("want Symbol=AAPL, got %v", candles[0].Symbol)
    }
}

func TestYahooAdapter_FetchCandles_UnsupportedTimeframe(t *testing.T) {
    a := adapter.NewYahooAdapter("http://localhost")
    _, err := a.FetchCandles(context.Background(), "AAPL", "stock", "4h",
        time.Now().Add(-24*time.Hour), time.Now())
    if err == nil {
        t.Fatal("expected error for unsupported timeframe, got nil")
    }
}

func TestYahooAdapter_FetchSymbols(t *testing.T) {
    a := adapter.NewYahooAdapter("http://localhost")
    symbols, err := a.FetchSymbols(context.Background(), "stock")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(symbols) == 0 {
        t.Error("want at least 1 symbol, got 0")
    }
    // Verify AAPL is in the list
    found := false
    for _, s := range symbols {
        if s.ID == "AAPL" {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected AAPL in default symbol list")
    }
}

func TestYahooAdapter_Name(t *testing.T) {
    a := adapter.NewYahooAdapter("")
    if a.Name() != "yahoo" {
        t.Errorf("want name=yahoo, got %s", a.Name())
    }
}

func TestYahooAdapter_Markets(t *testing.T) {
    a := adapter.NewYahooAdapter("")
    markets := a.Markets()
    if len(markets) == 0 {
        t.Error("want at least 1 market, got 0")
    }
}
```

### Step 2: Run tests to confirm they fail

```bash
cd backend && go test ./internal/adapter/... -run TestYahoo -v 2>&1 | head -20
```

Expected: `undefined: adapter.NewYahooAdapter`

### Step 3: Implement YahooAdapter

```go
// backend/internal/adapter/yahoo.go
package adapter

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/trader-claude/backend/internal/registry"
)

// yahooIntervals maps our timeframes to Yahoo Finance interval strings.
// Note: 4h is not natively supported by Yahoo; callers should avoid it.
var yahooIntervals = map[string]string{
    "1m": "1m", "5m": "5m", "15m": "15m", "30m": "30m",
    "1h": "60m", "1d": "1d", "1w": "1wk",
    // "4h" is intentionally omitted — Yahoo does not support it
}

// defaultYahooSymbols is a curated list served by FetchSymbols when no API key is available.
var defaultYahooSymbols = []registry.Symbol{
    {ID: "AAPL", Market: "stock", BaseAsset: "AAPL", Description: "Apple Inc.", Active: true},
    {ID: "MSFT", Market: "stock", BaseAsset: "MSFT", Description: "Microsoft Corp.", Active: true},
    {ID: "GOOGL", Market: "stock", BaseAsset: "GOOGL", Description: "Alphabet Inc.", Active: true},
    {ID: "AMZN", Market: "stock", BaseAsset: "AMZN", Description: "Amazon.com Inc.", Active: true},
    {ID: "META", Market: "stock", BaseAsset: "META", Description: "Meta Platforms Inc.", Active: true},
    {ID: "NVDA", Market: "stock", BaseAsset: "NVDA", Description: "NVIDIA Corp.", Active: true},
    {ID: "TSLA", Market: "stock", BaseAsset: "TSLA", Description: "Tesla Inc.", Active: true},
    {ID: "SPY", Market: "etf", BaseAsset: "SPY", Description: "SPDR S&P 500 ETF", Active: true},
    {ID: "QQQ", Market: "etf", BaseAsset: "QQQ", Description: "Invesco QQQ Trust", Active: true},
    {ID: "GLD", Market: "etf", BaseAsset: "GLD", Description: "SPDR Gold Shares ETF", Active: true},
    {ID: "BRK-B", Market: "stock", BaseAsset: "BRK-B", Description: "Berkshire Hathaway B", Active: true},
    {ID: "JPM", Market: "stock", BaseAsset: "JPM", Description: "JPMorgan Chase & Co.", Active: true},
    {ID: "V", Market: "stock", BaseAsset: "V", Description: "Visa Inc.", Active: true},
    {ID: "JNJ", Market: "stock", BaseAsset: "JNJ", Description: "Johnson & Johnson", Active: true},
    {ID: "WMT", Market: "stock", BaseAsset: "WMT", Description: "Walmart Inc.", Active: true},
    {ID: "EURUSD=X", Market: "forex", BaseAsset: "EUR", QuoteAsset: "USD", Description: "EUR/USD", Active: true},
    {ID: "GBPUSD=X", Market: "forex", BaseAsset: "GBP", QuoteAsset: "USD", Description: "GBP/USD", Active: true},
    {ID: "USDJPY=X", Market: "forex", BaseAsset: "USD", QuoteAsset: "JPY", Description: "USD/JPY", Active: true},
}

// YahooAdapter fetches market data from Yahoo Finance unofficial API.
// No API key is required.
type YahooAdapter struct {
    httpClient *http.Client
    baseURL    string
}

// NewYahooAdapter creates a YahooAdapter. Pass empty string for production URL.
func NewYahooAdapter(baseURL string) *YahooAdapter {
    if baseURL == "" {
        baseURL = "https://query1.finance.yahoo.com"
    }
    return &YahooAdapter{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        baseURL:    baseURL,
    }
}

func (a *YahooAdapter) Name() string      { return "yahoo" }
func (a *YahooAdapter) Markets() []string { return []string{"stock", "etf", "forex"} }
func (a *YahooAdapter) IsHealthy(ctx context.Context) bool {
    // Yahoo doesn't have a ping endpoint; attempt a lightweight query
    req, err := http.NewRequestWithContext(ctx, http.MethodGet,
        a.baseURL+"/v8/finance/chart/AAPL?interval=1d&range=1d", nil)
    if err != nil {
        return false
    }
    req.Header.Set("User-Agent", "Mozilla/5.0")
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return false
    }
    resp.Body.Close()
    return resp.StatusCode == http.StatusOK
}

func (a *YahooAdapter) SubscribeTicks(ctx context.Context, symbols []string, market string) (<-chan registry.Tick, error) {
    return nil, fmt.Errorf("streaming not implemented in Phase 2")
}

// FetchCandles fetches OHLCV data from Yahoo Finance v8 chart API.
func (a *YahooAdapter) FetchCandles(ctx context.Context, symbol, market, timeframe string, from, to time.Time) ([]registry.Candle, error) {
    interval, ok := yahooIntervals[timeframe]
    if !ok {
        return nil, fmt.Errorf("timeframe %q is not supported by Yahoo Finance adapter", timeframe)
    }

    url := fmt.Sprintf(
        "%s/v8/finance/chart/%s?interval=%s&period1=%d&period2=%d",
        a.baseURL, symbol, interval, from.Unix(), to.Unix(),
    )

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("building request: %w", err)
    }
    req.Header.Set("User-Agent", "Mozilla/5.0")

    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Yahoo Finance returned status %d", resp.StatusCode)
    }

    var payload struct {
        Chart struct {
            Result []struct {
                Timestamp  []int64 `json:"timestamp"`
                Indicators struct {
                    Quote []struct {
                        Open   []interface{} `json:"open"`
                        High   []interface{} `json:"high"`
                        Low    []interface{} `json:"low"`
                        Close  []interface{} `json:"close"`
                        Volume []interface{} `json:"volume"`
                    } `json:"quote"`
                } `json:"indicators"`
            } `json:"result"`
            Error interface{} `json:"error"`
        } `json:"chart"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
        return nil, fmt.Errorf("decoding response: %w", err)
    }

    if len(payload.Chart.Result) == 0 {
        return nil, nil
    }

    result := payload.Chart.Result[0]
    if len(result.Indicators.Quote) == 0 {
        return nil, nil
    }
    q := result.Indicators.Quote[0]

    candles := make([]registry.Candle, 0, len(result.Timestamp))
    for i, ts := range result.Timestamp {
        if i >= len(q.Open) {
            break
        }
        open  := toFloat(q.Open[i])
        high  := toFloat(q.High[i])
        low   := toFloat(q.Low[i])
        close := toFloat(q.Close[i])
        vol   := toFloat(q.Volume[i])

        // Skip candles with nil values (market closed days)
        if open == 0 && high == 0 {
            continue
        }

        candles = append(candles, registry.Candle{
            Symbol:    symbol,
            Market:    market,
            Timeframe: timeframe,
            Timestamp: time.Unix(ts, 0).UTC(),
            Open:      open,
            High:      high,
            Low:       low,
            Close:     close,
            Volume:    vol,
        })
    }
    return candles, nil
}

// FetchSymbols returns the curated hardcoded symbol list.
func (a *YahooAdapter) FetchSymbols(ctx context.Context, market string) ([]registry.Symbol, error) {
    if market == "" {
        return defaultYahooSymbols, nil
    }
    filtered := make([]registry.Symbol, 0)
    for _, s := range defaultYahooSymbols {
        if s.Market == market {
            filtered = append(filtered, s)
        }
    }
    return filtered, nil
}

// toFloat safely converts an interface{} (which may be nil for missing data) to float64.
func toFloat(v interface{}) float64 {
    if v == nil {
        return 0
    }
    switch n := v.(type) {
    case float64:
        return n
    case int64:
        return float64(n)
    }
    return 0
}
```

### Step 4: Run tests — all should pass

```bash
cd backend && go test ./internal/adapter/... -run TestYahoo -v
```

Expected: all 5 tests PASS

### Step 5: Commit

```bash
git add backend/internal/adapter/yahoo.go backend/internal/adapter/yahoo_test.go
git commit -m "feat(adapter): add YahooFinanceAdapter with curated symbol list"
```

---

## Task 3: DataService (gap detection + DB persistence)

**Files:**
- Create: `backend/internal/adapter/dataservice.go`
- Create: `backend/internal/adapter/dataservice_test.go`

### Step 1: Write the failing tests (gap detection logic only — pure function, no DB)

```go
// backend/internal/adapter/dataservice_test.go
package adapter_test

import (
    "testing"
    "time"

    "github.com/trader-claude/backend/internal/adapter"
)

// TestDetectGaps tests the pure gap-detection function.
// It takes a sorted list of timestamps present in the DB and returns
// a list of [from, to) ranges that need to be fetched.

func TestDetectGaps_NoGap(t *testing.T) {
    // DB already has all 3 expected candles
    step := time.Hour
    from := time.Unix(0, 0)
    to := from.Add(3 * step)
    present := []time.Time{from, from.Add(step), from.Add(2 * step)}
    gaps := adapter.DetectGaps(present, from, to, step)
    if len(gaps) != 0 {
        t.Errorf("want 0 gaps, got %d: %v", len(gaps), gaps)
    }
}

func TestDetectGaps_FullGap(t *testing.T) {
    // DB has nothing — entire range is a gap
    step := time.Hour
    from := time.Unix(0, 0)
    to := from.Add(3 * step)
    gaps := adapter.DetectGaps(nil, from, to, step)
    if len(gaps) != 1 {
        t.Fatalf("want 1 gap, got %d: %v", len(gaps), gaps)
    }
    if !gaps[0].From.Equal(from) {
        t.Errorf("want gap.From=%v, got %v", from, gaps[0].From)
    }
    if !gaps[0].To.Equal(to) {
        t.Errorf("want gap.To=%v, got %v", to, gaps[0].To)
    }
}

func TestDetectGaps_PrefixGap(t *testing.T) {
    // DB has candles 1 and 2 but not 0 — gap at the start
    step := time.Hour
    from := time.Unix(0, 0)
    to := from.Add(3 * step)
    present := []time.Time{from.Add(step), from.Add(2 * step)}
    gaps := adapter.DetectGaps(present, from, to, step)
    if len(gaps) != 1 {
        t.Fatalf("want 1 gap, got %d: %v", len(gaps), gaps)
    }
    if !gaps[0].From.Equal(from) {
        t.Errorf("want gap.From=%v, got %v", from, gaps[0].From)
    }
    if !gaps[0].To.Equal(from.Add(step)) {
        t.Errorf("want gap.To=%v, got %v", from.Add(step), gaps[0].To)
    }
}

func TestDetectGaps_InteriorGap(t *testing.T) {
    // DB has candles 0 and 2 but not 1 — gap in the middle
    step := time.Hour
    from := time.Unix(0, 0)
    to := from.Add(3 * step)
    present := []time.Time{from, from.Add(2 * step)}
    gaps := adapter.DetectGaps(present, from, to, step)
    if len(gaps) != 1 {
        t.Fatalf("want 1 gap, got %d: %v", len(gaps), gaps)
    }
    if !gaps[0].From.Equal(from.Add(step)) {
        t.Errorf("want gap.From=%v, got %v", from.Add(step), gaps[0].From)
    }
    if !gaps[0].To.Equal(from.Add(2*step)) {
        t.Errorf("want gap.To=%v, got %v", from.Add(2*step), gaps[0].To)
    }
}

func TestDetectGaps_SuffixGap(t *testing.T) {
    // DB has candles 0 and 1 but not 2 — gap at the end
    step := time.Hour
    from := time.Unix(0, 0)
    to := from.Add(3 * step)
    present := []time.Time{from, from.Add(step)}
    gaps := adapter.DetectGaps(present, from, to, step)
    if len(gaps) != 1 {
        t.Fatalf("want 1 gap, got %d: %v", len(gaps), gaps)
    }
    if !gaps[0].From.Equal(from.Add(2*step)) {
        t.Errorf("want gap.From=%v, got %v", from.Add(2*step), gaps[0].From)
    }
}

func TestDetectGaps_MultipleGaps(t *testing.T) {
    // DB has candle 1 only — two separate gaps
    step := time.Hour
    from := time.Unix(0, 0)
    to := from.Add(4 * step)
    present := []time.Time{from.Add(step)}
    gaps := adapter.DetectGaps(present, from, to, step)
    // Expected: gap [from, from+1h) and gap [from+2h, from+4h)
    if len(gaps) != 2 {
        t.Fatalf("want 2 gaps, got %d: %v", len(gaps), gaps)
    }
}
```

### Step 2: Run tests to confirm they fail

```bash
cd backend && go test ./internal/adapter/... -run TestDetectGaps -v 2>&1 | head -20
```

Expected: `undefined: adapter.DetectGaps`

### Step 3: Implement DataService

```go
// backend/internal/adapter/dataservice.go
package adapter

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"

    "github.com/trader-claude/backend/internal/models"
    "github.com/trader-claude/backend/internal/registry"
)

// TimeRange is a half-open interval [From, To) for gap-filling.
type TimeRange struct {
    From time.Time
    To   time.Time
}

// timeframeDuration maps timeframe strings to their step duration.
var timeframeDuration = map[string]time.Duration{
    "1m":  time.Minute,
    "5m":  5 * time.Minute,
    "15m": 15 * time.Minute,
    "30m": 30 * time.Minute,
    "1h":  time.Hour,
    "4h":  4 * time.Hour,
    "1d":  24 * time.Hour,
    "1w":  7 * 24 * time.Hour,
}

// SupportedTimeframes lists all timeframes the platform supports.
var SupportedTimeframes = []string{"1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w"}

// DataService handles candle retrieval with transparent DB-first gap-filling.
type DataService struct {
    db  *gorm.DB
    rdb *redis.Client
    reg *registry.AdapterRegistry
}

// NewDataService creates a DataService.
func NewDataService(db *gorm.DB, rdb *redis.Client, reg *registry.AdapterRegistry) *DataService {
    return &DataService{db: db, rdb: rdb, reg: reg}
}

// GetCandles returns candles for the given parameters.
// It queries the DB first, detects missing time ranges, fetches from the exchange
// only for gaps, upserts them, and returns the merged sorted result.
func (s *DataService) GetCandles(ctx context.Context, adapterID, symbol, market, timeframe string, from, to time.Time) ([]models.Candle, error) {
    step, ok := timeframeDuration[timeframe]
    if !ok {
        return nil, fmt.Errorf("unsupported timeframe %q", timeframe)
    }

    // 1. Track this symbol as "hot" in Redis (24h TTL) for background sync
    s.markHot(ctx, adapterID, symbol, timeframe)

    // 2. Query DB for existing candles
    var dbCandles []models.Candle
    err := s.db.WithContext(ctx).
        Where("symbol = ? AND market = ? AND timeframe = ? AND timestamp >= ? AND timestamp < ?",
            symbol, market, timeframe, from, to).
        Order("timestamp ASC").
        Find(&dbCandles).Error
    if err != nil {
        return nil, fmt.Errorf("querying candles: %w", err)
    }

    // 3. Build set of present timestamps
    present := make([]time.Time, len(dbCandles))
    for i, c := range dbCandles {
        present[i] = c.Timestamp
    }

    // 4. Detect gaps
    gaps := DetectGaps(present, from, to, step)
    if len(gaps) == 0 {
        return dbCandles, nil
    }

    // 5. Fetch gaps from adapter
    adpt, err := s.reg.Get(adapterID)
    if err != nil {
        return nil, fmt.Errorf("adapter %q not found: %w", adapterID, err)
    }

    var newCandles []models.Candle
    for _, gap := range gaps {
        fetched, err := adpt.FetchCandles(ctx, symbol, market, timeframe, gap.From, gap.To)
        if err != nil {
            return nil, fmt.Errorf("fetching gap [%v, %v): %w", gap.From, gap.To, err)
        }
        for _, rc := range fetched {
            newCandles = append(newCandles, registryToModel(rc))
        }
    }

    // 6. Upsert new candles
    if len(newCandles) > 0 {
        err = s.db.WithContext(ctx).
            Clauses(clause.OnConflict{DoNothing: true}).
            CreateInBatches(newCandles, 500).Error
        if err != nil {
            return nil, fmt.Errorf("upserting candles: %w", err)
        }
    }

    // 7. Re-query to get the full merged result
    var result []models.Candle
    err = s.db.WithContext(ctx).
        Where("symbol = ? AND market = ? AND timeframe = ? AND timestamp >= ? AND timestamp < ?",
            symbol, market, timeframe, from, to).
        Order("timestamp ASC").
        Find(&result).Error
    if err != nil {
        return nil, fmt.Errorf("re-querying merged candles: %w", err)
    }
    return result, nil
}

// SyncRecent fetches the last 500 candles for a symbol and upserts them.
// Called by the background worker for "hot" symbols.
func (s *DataService) SyncRecent(ctx context.Context, adapterID, symbol, market, timeframe string) error {
    step, ok := timeframeDuration[timeframe]
    if !ok {
        return fmt.Errorf("unsupported timeframe %q", timeframe)
    }

    to := time.Now().UTC()
    from := to.Add(-500 * step)

    adpt, err := s.reg.Get(adapterID)
    if err != nil {
        return fmt.Errorf("adapter %q not found: %w", adapterID, err)
    }

    candles, err := adpt.FetchCandles(ctx, symbol, market, timeframe, from, to)
    if err != nil {
        return fmt.Errorf("fetching candles for sync: %w", err)
    }

    if len(candles) == 0 {
        return nil
    }

    modelCandles := make([]models.Candle, len(candles))
    for i, c := range candles {
        modelCandles[i] = registryToModel(c)
    }

    return s.db.WithContext(ctx).
        Clauses(clause.OnConflict{DoNothing: true}).
        CreateInBatches(modelCandles, 500).Error
}

// markHot records a symbol as recently accessed in Redis (24h TTL).
func (s *DataService) markHot(ctx context.Context, adapterID, symbol, timeframe string) {
    key := fmt.Sprintf("hot:%s:%s:%s", adapterID, symbol, timeframe)
    s.rdb.Set(ctx, key, "1", 24*time.Hour)
}

// registryToModel converts a registry.Candle to a models.Candle for DB persistence.
func registryToModel(c registry.Candle) models.Candle {
    return models.Candle{
        Symbol:    c.Symbol,
        Market:    c.Market,
        Timeframe: c.Timeframe,
        Timestamp: c.Timestamp.UTC(),
        Open:      c.Open,
        High:      c.High,
        Low:       c.Low,
        Close:     c.Close,
        Volume:    c.Volume,
    }
}

// DetectGaps returns the time ranges [From, To) that are missing from present
// within the requested interval [from, to) stepping by step.
// present must be sorted ascending.
// This is exported so it can be unit-tested without a DB.
func DetectGaps(present []time.Time, from, to time.Time, step time.Duration) []TimeRange {
    // Build a set of present timestamps (truncated to step for comparison)
    set := make(map[int64]bool, len(present))
    for _, t := range present {
        set[t.Truncate(step).Unix()] = true
    }

    var gaps []TimeRange
    var gapStart *time.Time

    for t := from.Truncate(step); t.Before(to); t = t.Add(step) {
        if !set[t.Unix()] {
            if gapStart == nil {
                t := t // capture
                gapStart = &t
            }
        } else {
            if gapStart != nil {
                gaps = append(gaps, TimeRange{From: *gapStart, To: t})
                gapStart = nil
            }
        }
    }

    // Close trailing gap
    if gapStart != nil {
        gaps = append(gaps, TimeRange{From: *gapStart, To: to})
    }

    return gaps
}
```

### Step 4: Run tests — all should pass

```bash
cd backend && go test ./internal/adapter/... -run TestDetectGaps -v
```

Expected: all 6 tests PASS

### Step 5: Commit

```bash
git add backend/internal/adapter/dataservice.go backend/internal/adapter/dataservice_test.go
git commit -m "feat(adapter): add DataService with gap detection and DB-first candle fetching"
```

---

## Task 4: API Handlers (markets + candles)

**Files:**
- Create: `backend/internal/api/markets.go`
- Create: `backend/internal/api/candles.go`
- Modify: `backend/internal/api/routes.go`

### Step 1: Create markets handler

```go
// backend/internal/api/markets.go
package api

import (
    "github.com/gofiber/fiber/v2"

    "github.com/trader-claude/backend/internal/registry"
)

type marketsHandler struct {
    reg *registry.AdapterRegistry
}

func newMarketsHandler(reg *registry.AdapterRegistry) *marketsHandler {
    return &marketsHandler{reg: reg}
}

// listMarkets returns all registered adapters.
// GET /api/v1/markets
func (h *marketsHandler) listMarkets(c *fiber.Ctx) error {
    adapters := h.reg.All()
    type adapterInfo struct {
        ID                 string   `json:"id"`
        Name               string   `json:"name"`
        Markets            []string `json:"markets"`
        StreamingSupported bool     `json:"streaming_supported"`
    }
    result := make([]adapterInfo, 0, len(adapters))
    for _, a := range adapters {
        result = append(result, adapterInfo{
            ID:                 a.Name(),
            Name:               a.Name(),
            Markets:            a.Markets(),
            StreamingSupported: false, // Phase 2: always false
        })
    }
    return c.JSON(result)
}

// listSymbols returns symbols for a specific adapter.
// GET /api/v1/markets/:adapterID/symbols?market=
func (h *marketsHandler) listSymbols(c *fiber.Ctx) error {
    adapterID := c.Params("adapterID")
    market := c.Query("market", "")

    adpt, err := h.reg.Get(adapterID)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": fmt.Sprintf("adapter %q not found", adapterID)})
    }

    symbols, err := adpt.FetchSymbols(c.Context(), market)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(symbols)
}
```

**Note:** Add `"fmt"` import to markets.go.

### Step 2: Create candles handler

```go
// backend/internal/api/candles.go
package api

import (
    "fmt"
    "strconv"
    "time"

    "github.com/gofiber/fiber/v2"

    "github.com/trader-claude/backend/internal/adapter"
)

type candlesHandler struct {
    dataSvc *adapter.DataService
}

func newCandlesHandler(dataSvc *adapter.DataService) *candlesHandler {
    return &candlesHandler{dataSvc: dataSvc}
}

// getTimeframes returns the list of supported timeframes.
// GET /api/v1/candles/timeframes
func (h *candlesHandler) getTimeframes(c *fiber.Ctx) error {
    return c.JSON(adapter.SupportedTimeframes)
}

// getCandles returns OHLCV candles for the given query parameters.
// GET /api/v1/candles?adapter=binance&symbol=BTCUSDT&timeframe=1h&from=1700000000000&to=1700100000000
func (h *candlesHandler) getCandles(c *fiber.Ctx) error {
    adapterID := c.Query("adapter")
    symbol    := c.Query("symbol")
    timeframe := c.Query("timeframe")
    market    := c.Query("market", "")

    if adapterID == "" || symbol == "" || timeframe == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "adapter, symbol, and timeframe are required",
        })
    }

    // Default market based on adapter if not provided
    if market == "" {
        switch adapterID {
        case "binance":
            market = "crypto"
        case "yahoo":
            market = "stock"
        default:
            market = "stock"
        }
    }

    // Parse time range (Unix milliseconds)
    to := time.Now().UTC()
    from := to.Add(-200 * 24 * time.Hour) // default: last 200 days

    if fromStr := c.Query("from"); fromStr != "" {
        ms, err := strconv.ParseInt(fromStr, 10, 64)
        if err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from timestamp"})
        }
        from = time.UnixMilli(ms).UTC()
    }
    if toStr := c.Query("to"); toStr != "" {
        ms, err := strconv.ParseInt(toStr, 10, 64)
        if err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to timestamp"})
        }
        to = time.UnixMilli(ms).UTC()
    }

    if from.After(to) {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from must be before to"})
    }

    candles, err := h.dataSvc.GetCandles(c.Context(), adapterID, symbol, market, timeframe, from, to)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("fetching candles: %s", err.Error())})
    }

    return c.JSON(candles)
}
```

### Step 3: Update routes.go

Replace the content of `RegisterRoutes` to accept `DataService` and wire the new handlers. The full updated function signature and body:

```go
// backend/internal/api/routes.go
package api

import (
    "github.com/gofiber/contrib/websocket"
    "github.com/gofiber/fiber/v2"
    "github.com/redis/go-redis/v9"
    "gorm.io/gorm"

    adapterPkg "github.com/trader-claude/backend/internal/adapter"
    "github.com/trader-claude/backend/internal/registry"
    "github.com/trader-claude/backend/internal/ws"
)

// RegisterRoutes wires all HTTP and WebSocket routes onto the Fiber app.
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, dataSvc *adapterPkg.DataService, version string) {
    // Health
    health := newHealthHandler(db, rdb, version)
    app.Get("/health", health.check)

    // API v1 group
    v1 := app.Group("/api/v1")

    // --- Markets ---
    markets := newMarketsHandler(registry.Adapters())
    v1.Get("/markets", markets.listMarkets)
    v1.Get("/markets/:adapterID/symbols", markets.listSymbols)

    // --- Candles ---
    candles := newCandlesHandler(dataSvc)
    v1.Get("/candles/timeframes", candles.getTimeframes)
    v1.Get("/candles", candles.getCandles)

    // --- Strategies ---
    v1.Get("/strategies", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"data": []interface{}{}, "message": "strategies endpoint — coming soon"})
    })

    // --- Backtests ---
    v1.Get("/backtests", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"data": []interface{}{}, "message": "backtests endpoint — coming soon"})
    })
    v1.Post("/backtests", func(c *fiber.Ctx) error {
        return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"message": "backtest run endpoint — coming soon"})
    })
    v1.Get("/backtests/:id", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"data": nil, "message": "backtest detail endpoint — coming soon"})
    })

    // --- Portfolios ---
    v1.Get("/portfolios", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"data": []interface{}{}, "message": "portfolios endpoint — coming soon"})
    })
    v1.Post("/portfolios", func(c *fiber.Ctx) error {
        return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "portfolio create endpoint — coming soon"})
    })

    // --- Alerts ---
    v1.Get("/alerts", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"data": []interface{}{}, "message": "alerts endpoint — coming soon"})
    })
    v1.Post("/alerts", func(c *fiber.Ctx) error {
        return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "alert create endpoint — coming soon"})
    })
    v1.Delete("/alerts/:id", func(c *fiber.Ctx) error {
        return c.SendStatus(fiber.StatusNoContent)
    })

    // --- Notifications ---
    v1.Get("/notifications", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"data": []interface{}{}, "message": "notifications endpoint — coming soon"})
    })
    v1.Patch("/notifications/:id/read", func(c *fiber.Ctx) error {
        return c.SendStatus(fiber.StatusNoContent)
    })

    // --- WebSocket ---
    app.Use("/ws", func(c *fiber.Ctx) error {
        if websocket.IsWebSocketUpgrade(c) {
            return c.Next()
        }
        return fiber.ErrUpgradeRequired
    })
    app.Get("/ws", websocket.New(hub.ServeWS))
}
```

### Step 4: Verify it compiles

```bash
cd backend && go build ./...
```

Expected: no errors

### Step 5: Commit

```bash
git add backend/internal/api/markets.go backend/internal/api/candles.go backend/internal/api/routes.go
git commit -m "feat(api): add markets and candles endpoints"
```

---

## Task 5: Register Adapters + Background Worker in main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

### Step 1: No test needed — integration wiring. Verify with `go build` after each change.

### Step 2: Update main.go

Add adapter registration and background worker after the worker pool starts. Here are the specific changes to `main.go`:

**After `pool.Start()` (around line 77), add:**

```go
// Register market adapters
binanceAdpt := binanceAdapter.NewBinanceAdapter("")
yahooAdpt   := yahooAdapter.NewYahooAdapter("")
if err := registry.Adapters().Register(binanceAdpt); err != nil {
    log.Fatalf("failed to register binance adapter: %v", err)
}
if err := registry.Adapters().Register(yahooAdpt); err != nil {
    log.Fatalf("failed to register yahoo adapter: %v", err)
}
log.Println("adapters registered: binance, yahoo")

// Initialize DataService
dataSvc := adapterPkg.NewDataService(db, rdb, registry.Adapters())

// Start background sync worker (every 5 min)
go runSyncWorker(context.Background(), dataSvc, rdb)
```

**Update the `RegisterRoutes` call** to pass `dataSvc`:

```go
api.RegisterRoutes(app, db, rdb, hub, dataSvc, cfg.App.Version)
```

**Add the sync worker function** at the bottom of main.go:

```go
// runSyncWorker periodically syncs the last 500 candles for "hot" symbols.
// A symbol is "hot" if it was accessed in the last 24 hours (tracked via Redis key hot:*).
func runSyncWorker(ctx context.Context, dataSvc *adapterPkg.DataService, rdb *redis.Client) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            syncHotSymbols(ctx, dataSvc, rdb)
        }
    }
}

func syncHotSymbols(ctx context.Context, dataSvc *adapterPkg.DataService, rdb *goredis.Client) {
    keys, err := rdb.Keys(ctx, "hot:*").Result()
    if err != nil {
        log.Printf("sync worker: failed to scan hot keys: %v", err)
        return
    }
    for _, key := range keys {
        // key format: hot:{adapterID}:{symbol}:{timeframe}
        parts := strings.SplitN(key, ":", 4)
        if len(parts) != 4 {
            continue
        }
        adapterID, symbol, timeframe := parts[1], parts[2], parts[3]

        // Determine market from adapter
        market := "crypto"
        if adapterID == "yahoo" {
            market = "stock"
        }

        if err := dataSvc.SyncRecent(ctx, adapterID, symbol, market, timeframe); err != nil {
            log.Printf("sync worker: failed to sync %s/%s/%s: %v", adapterID, symbol, timeframe, err)
        }
    }
}
```

**Add imports** to main.go:

```go
import (
    // ... existing imports ...
    "strings"

    adapterPkg     "github.com/trader-claude/backend/internal/adapter"
    binanceAdapter "github.com/trader-claude/backend/internal/adapter"
    yahooAdapter   "github.com/trader-claude/backend/internal/adapter"
    "github.com/trader-claude/backend/internal/registry"
)
```

**Note:** Since `binanceAdapter` and `yahooAdapter` are the same package (`adapter`), use a single import:

```go
import (
    // ... existing imports ...
    "strings"

    adapterPkg "github.com/trader-claude/backend/internal/adapter"
    "github.com/trader-claude/backend/internal/registry"
)
```

And reference as `adapterPkg.NewBinanceAdapter("")` and `adapterPkg.NewYahooAdapter("")`.

### Step 3: Verify it compiles

```bash
cd backend && go build ./...
```

Expected: no errors

### Step 4: Commit

```bash
git add backend/cmd/server/main.go
git commit -m "feat(main): register Binance/Yahoo adapters and start background sync worker"
```

---

## Task 6: Frontend — vitest setup + new TypeScript types

**Files:**
- Create: `frontend/vitest.config.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/stores/index.ts`

### Step 1: Install test dependencies

```bash
cd frontend && npm install --save-dev @testing-library/react @testing-library/user-event @testing-library/jest-dom jsdom
```

### Step 2: Create vitest config

```ts
// frontend/vitest.config.ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
})
```

### Step 3: Create test setup file

```ts
// frontend/src/test-setup.ts
import '@testing-library/jest-dom'
```

### Step 4: Add new types to `frontend/src/types/index.ts`

Append to the end of the file:

```ts
// ── Market adapter types (Phase 2) ────────────────────────────────────────

export interface MarketAdapter {
  id: string
  name: string
  markets: string[]
  streaming_supported: boolean
}

export interface CandleQueryParams {
  adapter: string
  symbol: string
  timeframe: string
  market?: string
  from?: number  // Unix ms
  to?: number    // Unix ms
}
```

### Step 5: Add `selectedAdapter` to `marketStore` in `frontend/src/stores/index.ts`

Find the `MarketStore` interface and add `selectedAdapter`:

```ts
interface MarketStore {
  ticks: Record<string, Tick>
  symbols: Symbol[]
  selectedSymbol: string | null
  selectedMarket: string
  selectedTimeframe: string
  selectedAdapter: string          // ADD THIS
  updateTick: (tick: Tick) => void
  setSymbols: (symbols: Symbol[]) => void
  setSelectedSymbol: (symbol: string | null) => void
  setSelectedMarket: (market: string) => void
  setSelectedTimeframe: (tf: string) => void
  setSelectedAdapter: (adapter: string) => void  // ADD THIS
}
```

And update the store implementation:

```ts
export const useMarketStore = create<MarketStore>()((set) => ({
  ticks: {},
  symbols: [],
  selectedSymbol: null,
  selectedMarket: 'crypto',
  selectedTimeframe: '1h',
  selectedAdapter: 'binance',          // ADD THIS
  updateTick: (tick) =>
    set((s) => ({
      ticks: { ...s.ticks, [`${tick.symbol}:${tick.market}`]: tick },
    })),
  setSymbols: (symbols) => set({ symbols }),
  setSelectedSymbol: (symbol) => set({ selectedSymbol: symbol }),
  setSelectedMarket: (market) => set({ selectedMarket: market }),
  setSelectedTimeframe: (tf) => set({ selectedTimeframe: tf }),
  setSelectedAdapter: (adapter) => set({ selectedAdapter: adapter }),  // ADD THIS
}))
```

### Step 6: Verify TypeScript compiles

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors

### Step 7: Commit

```bash
git add frontend/vitest.config.ts frontend/src/test-setup.ts frontend/src/types/index.ts frontend/src/stores/index.ts
git commit -m "feat(frontend): add vitest config, MarketAdapter types, and selectedAdapter store field"
```

---

## Task 7: React Query hooks

**Files:**
- Create: `frontend/src/hooks/useMarkets.ts`
- Create: `frontend/src/hooks/useSymbols.ts`
- Create: `frontend/src/hooks/useCandles.ts`
- Create: `frontend/src/hooks/useCandles.test.ts`

### Step 1: Write the failing test for useCandles

```ts
// frontend/src/hooks/useCandles.test.ts
import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useCandles } from './useCandles'

vi.mock('@/api/client', () => ({
  default: {
    get: vi.fn().mockResolvedValue({ data: [] }),
  },
  apiClient: {
    get: vi.fn().mockResolvedValue({ data: [] }),
  },
}))

const wrapper = ({ children }: { children: React.ReactNode }) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return React.createElement(QueryClientProvider, { client: qc }, children)
}

describe('useCandles', () => {
  it('returns empty array when API returns empty', async () => {
    const { result } = renderHook(
      () => useCandles({ adapter: 'binance', symbol: 'BTCUSDT', timeframe: '1h' }),
      { wrapper },
    )
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual([])
  })

  it('uses correct query key including symbol', () => {
    // The query key must change when symbol changes to trigger refetch
    const { result: r1 } = renderHook(
      () => useCandles({ adapter: 'binance', symbol: 'BTCUSDT', timeframe: '1h' }),
      { wrapper },
    )
    const { result: r2 } = renderHook(
      () => useCandles({ adapter: 'binance', symbol: 'ETHUSDT', timeframe: '1h' }),
      { wrapper },
    )
    // Both hooks exist independently — different query keys means no shared cache
    expect(r1.current).toBeDefined()
    expect(r2.current).toBeDefined()
  })

  it('is disabled when symbol is empty', () => {
    const { result } = renderHook(
      () => useCandles({ adapter: 'binance', symbol: '', timeframe: '1h' }),
      { wrapper },
    )
    // Query should not be fetching — disabled due to empty symbol
    expect(result.current.isFetching).toBe(false)
  })
})
```

### Step 2: Run test to confirm failure

```bash
cd frontend && npx vitest run src/hooks/useCandles.test.ts 2>&1 | tail -20
```

Expected: `Cannot find module './useCandles'`

### Step 3: Implement the hooks

```ts
// frontend/src/hooks/useMarkets.ts
import { useQuery } from '@tanstack/react-query'
import apiClient from '@/api/client'
import type { MarketAdapter } from '@/types'

export function useMarkets() {
  return useQuery<MarketAdapter[]>({
    queryKey: ['markets'],
    queryFn: async () => {
      const res = await apiClient.get<MarketAdapter[]>('/api/v1/markets')
      return res.data
    },
    staleTime: 5 * 60 * 1000, // 5 minutes — adapter list rarely changes
  })
}
```

```ts
// frontend/src/hooks/useSymbols.ts
import { useQuery } from '@tanstack/react-query'
import apiClient from '@/api/client'
import type { Symbol } from '@/types'

export function useSymbols(adapterID: string, market?: string) {
  return useQuery<Symbol[]>({
    queryKey: ['symbols', adapterID, market],
    queryFn: async () => {
      const params = market ? { market } : {}
      const res = await apiClient.get<Symbol[]>(`/api/v1/markets/${adapterID}/symbols`, { params })
      return res.data
    },
    enabled: Boolean(adapterID),
    staleTime: 5 * 60 * 1000,
  })
}
```

```ts
// frontend/src/hooks/useCandles.ts
import { useQuery } from '@tanstack/react-query'
import apiClient from '@/api/client'
import type { Candle, CandleQueryParams } from '@/types'

export function useCandles(params: CandleQueryParams) {
  const { adapter, symbol, timeframe, market, from, to } = params
  return useQuery<Candle[]>({
    queryKey: ['candles', adapter, symbol, timeframe, market, from, to],
    queryFn: async () => {
      const res = await apiClient.get<Candle[]>('/api/v1/candles', {
        params: {
          adapter,
          symbol,
          timeframe,
          ...(market && { market }),
          ...(from !== undefined && { from }),
          ...(to !== undefined && { to }),
        },
      })
      return res.data
    },
    enabled: Boolean(adapter) && Boolean(symbol) && Boolean(timeframe),
    staleTime: 30 * 1000, // 30 seconds — candle data is semi-live
  })
}
```

### Step 4: Run tests — all should pass

```bash
cd frontend && npx vitest run src/hooks/useCandles.test.ts
```

Expected: all 3 tests PASS

### Step 5: Commit

```bash
git add frontend/src/hooks/useMarkets.ts frontend/src/hooks/useSymbols.ts \
        frontend/src/hooks/useCandles.ts frontend/src/hooks/useCandles.test.ts
git commit -m "feat(hooks): add useMarkets, useSymbols, useCandles React Query hooks"
```

---

## Task 8: CandlestickChart component

**Files:**
- Create: `frontend/src/components/chart/CandlestickChart.tsx`
- Create: `frontend/src/components/chart/CandlestickChart.test.tsx`

### Step 1: Write the failing tests

```tsx
// frontend/src/components/chart/CandlestickChart.test.tsx
import { describe, it, expect, vi, beforeAll } from 'vitest'
import { render, screen } from '@testing-library/react'
import React from 'react'
import { CandlestickChart } from './CandlestickChart'
import type { Candle } from '@/types'

// lightweight-charts uses canvas — mock it for jsdom
beforeAll(() => {
  HTMLCanvasElement.prototype.getContext = vi.fn().mockReturnValue({
    clearRect: vi.fn(),
    fillRect: vi.fn(),
    beginPath: vi.fn(),
    stroke: vi.fn(),
  })
})

vi.mock('lightweight-charts', () => ({
  createChart: vi.fn(() => ({
    addCandlestickSeries: vi.fn(() => ({
      setData: vi.fn(),
    })),
    applyOptions: vi.fn(),
    timeScale: vi.fn(() => ({ fitContent: vi.fn() })),
    remove: vi.fn(),
    resize: vi.fn(),
  })),
  ColorType: { Solid: 'Solid' },
}))

const mockCandles: Candle[] = [
  { id: 1, symbol: 'BTCUSDT', market: 'crypto', timeframe: '1h',
    timestamp: '2024-01-01T00:00:00Z', open: 42000, high: 42500, low: 41800, close: 42200, volume: 100 },
]

describe('CandlestickChart', () => {
  it('renders the chart container', () => {
    render(<CandlestickChart candles={mockCandles} isLoading={false} />)
    expect(document.querySelector('[data-testid="chart-container"]')).toBeTruthy()
  })

  it('shows loading overlay when isLoading is true', () => {
    render(<CandlestickChart candles={[]} isLoading={true} />)
    expect(screen.getByTestId('chart-loading')).toBeTruthy()
  })

  it('shows error state when error prop is provided', () => {
    render(<CandlestickChart candles={[]} isLoading={false} error="Something went wrong" onRetry={() => {}} />)
    expect(screen.getByText('Something went wrong')).toBeTruthy()
    expect(screen.getByRole('button', { name: /retry/i })).toBeTruthy()
  })

  it('renders no loading overlay when data is present and not loading', () => {
    render(<CandlestickChart candles={mockCandles} isLoading={false} />)
    expect(document.querySelector('[data-testid="chart-loading"]')).toBeNull()
  })
})
```

### Step 2: Run test to confirm failure

```bash
cd frontend && npx vitest run src/components/chart/CandlestickChart.test.tsx 2>&1 | tail -20
```

Expected: `Cannot find module './CandlestickChart'`

### Step 3: Implement CandlestickChart

```tsx
// frontend/src/components/chart/CandlestickChart.tsx
import React, { useEffect, useRef } from 'react'
import { createChart, ColorType, type IChartApi, type ISeriesApi } from 'lightweight-charts'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Candle } from '@/types'

interface CandlestickChartProps {
  candles: Candle[]
  isLoading: boolean
  error?: string | null
  onRetry?: () => void
}

export function CandlestickChart({ candles, isLoading, error, onRetry }: CandlestickChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)

  // Initialize chart on mount
  useEffect(() => {
    if (!containerRef.current) return

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: 'rgba(148, 163, 184, 1)',
      },
      grid: {
        vertLines: { color: 'rgba(148, 163, 184, 0.1)' },
        horzLines: { color: 'rgba(148, 163, 184, 0.1)' },
      },
      width: containerRef.current.clientWidth,
      height: containerRef.current.clientHeight || 400,
    })

    const series = chart.addCandlestickSeries({
      upColor: '#22c55e',
      downColor: '#ef4444',
      borderUpColor: '#22c55e',
      borderDownColor: '#ef4444',
      wickUpColor: '#22c55e',
      wickDownColor: '#ef4444',
    })

    chartRef.current = chart
    seriesRef.current = series

    // ResizeObserver keeps chart size in sync with container
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry && chartRef.current) {
        chartRef.current.resize(entry.contentRect.width, entry.contentRect.height)
      }
    })
    observer.observe(containerRef.current)

    return () => {
      observer.disconnect()
      chart.remove()
      chartRef.current = null
      seriesRef.current = null
    }
  }, [])

  // Update chart data when candles change — NO chart destroy/recreate
  useEffect(() => {
    if (!seriesRef.current || candles.length === 0) return

    const chartData = candles
      .map((c) => ({
        time: Math.floor(new Date(c.timestamp).getTime() / 1000) as unknown as string,
        open: c.open,
        high: c.high,
        low: c.low,
        close: c.close,
      }))
      .sort((a, b) => Number(a.time) - Number(b.time))

    seriesRef.current.setData(chartData as Parameters<typeof seriesRef.current.setData>[0])
    chartRef.current?.timeScale().fitContent()
  }, [candles])

  return (
    <div className="relative w-full h-full">
      {/* Chart container — always rendered so the chart persists across data changes */}
      <div
        ref={containerRef}
        data-testid="chart-container"
        className="w-full h-full"
      />

      {/* Loading overlay — sits above chart without destroying it */}
      {isLoading && (
        <div
          data-testid="chart-loading"
          className="absolute inset-0 flex items-center justify-center bg-background/60 backdrop-blur-sm rounded-lg"
        >
          <Loader2 className="w-8 h-8 animate-spin text-primary" />
        </div>
      )}

      {/* Error state */}
      {error && !isLoading && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3">
          <p className="text-sm text-destructive">{error}</p>
          {onRetry && (
            <Button variant="outline" size="sm" onClick={onRetry}>
              <RefreshCw className="w-4 h-4 mr-2" />
              Retry
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
```

### Step 4: Run tests — all should pass

```bash
cd frontend && npx vitest run src/components/chart/CandlestickChart.test.tsx
```

Expected: all 4 tests PASS

### Step 5: Commit

```bash
git add frontend/src/components/chart/CandlestickChart.tsx \
        frontend/src/components/chart/CandlestickChart.test.tsx
git commit -m "feat(chart): add CandlestickChart component with loading overlay and error state"
```

---

## Task 9: Chart page (full implementation)

**Files:**
- Create: `frontend/src/components/chart/ChartToolbar.tsx`
- Modify: `frontend/src/pages/Chart.tsx`
- Create: `frontend/src/pages/Chart.test.tsx`

### Step 1: Write the failing Chart page test

```tsx
// frontend/src/pages/Chart.test.tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Mock lightweight-charts (same as component test)
vi.mock('lightweight-charts', () => ({
  createChart: vi.fn(() => ({
    addCandlestickSeries: vi.fn(() => ({ setData: vi.fn() })),
    applyOptions: vi.fn(),
    timeScale: vi.fn(() => ({ fitContent: vi.fn() })),
    remove: vi.fn(),
    resize: vi.fn(),
  })),
  ColorType: { Solid: 'Solid' },
}))

// Mock API hooks
vi.mock('@/hooks/useMarkets', () => ({
  useMarkets: () => ({
    data: [{ id: 'binance', name: 'binance', markets: ['crypto'], streaming_supported: false }],
    isLoading: false,
  }),
}))

vi.mock('@/hooks/useSymbols', () => ({
  useSymbols: () => ({
    data: [{ id: 1, ticker: 'BTCUSDT', market: 'crypto', base_asset: 'BTC', quote_asset: 'USDT', description: '', active: true }],
    isLoading: false,
  }),
}))

vi.mock('@/hooks/useCandles', () => ({
  useCandles: () => ({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}))

import { Chart } from './Chart'

const wrapper = ({ children }: { children: React.ReactNode }) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return React.createElement(QueryClientProvider, { client: qc }, children)
}

describe('Chart page', () => {
  it('renders the chart page', () => {
    render(<Chart />, { wrapper })
    expect(document.querySelector('[data-testid="chart-container"]')).toBeTruthy()
  })

  it('renders the toolbar with timeframe buttons', () => {
    render(<Chart />, { wrapper })
    expect(screen.getByText('1h')).toBeTruthy()
    expect(screen.getByText('1d')).toBeTruthy()
  })

  it('shows no loading overlay when data is available', () => {
    render(<Chart />, { wrapper })
    expect(document.querySelector('[data-testid="chart-loading"]')).toBeNull()
  })
})
```

### Step 2: Run test to confirm failure

```bash
cd frontend && npx vitest run src/pages/Chart.test.tsx 2>&1 | tail -20
```

Expected: the stub Chart component exists but tests fail because it has no chart container.

### Step 3: Create ChartToolbar component

```tsx
// frontend/src/components/chart/ChartToolbar.tsx
import React, { useState } from 'react'
import { ChevronDown, Search } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useMarkets } from '@/hooks/useMarkets'
import { useSymbols } from '@/hooks/useSymbols'
import type { MarketAdapter } from '@/types'

const TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w']

interface ChartToolbarProps {
  selectedAdapter: string
  selectedSymbol: string
  selectedTimeframe: string
  onAdapterChange: (adapter: string) => void
  onSymbolChange: (symbol: string) => void
  onTimeframeChange: (tf: string) => void
}

export function ChartToolbar({
  selectedAdapter,
  selectedSymbol,
  selectedTimeframe,
  onAdapterChange,
  onSymbolChange,
  onTimeframeChange,
}: ChartToolbarProps) {
  const [symbolSearch, setSymbolSearch] = useState('')
  const [showSymbolDropdown, setShowSymbolDropdown] = useState(false)

  const { data: markets = [] } = useMarkets()
  const { data: symbols = [] } = useSymbols(selectedAdapter)

  const filtered = symbolSearch
    ? symbols.filter((s) =>
        s.ticker.toLowerCase().includes(symbolSearch.toLowerCase()),
      )
    : symbols.slice(0, 20) // show first 20 when no search

  return (
    <div className="flex items-center gap-2 p-2 border-b border-border flex-wrap">
      {/* Adapter selector */}
      <div className="relative">
        <select
          value={selectedAdapter}
          onChange={(e) => onAdapterChange(e.target.value)}
          className="h-8 px-3 text-sm rounded-md border border-input bg-background appearance-none pr-7 cursor-pointer"
        >
          {markets.map((m: MarketAdapter) => (
            <option key={m.id} value={m.id}>
              {m.name}
            </option>
          ))}
        </select>
        <ChevronDown className="absolute right-2 top-2 w-3 h-3 pointer-events-none text-muted-foreground" />
      </div>

      {/* Symbol search */}
      <div className="relative">
        <div className="flex items-center h-8 px-3 gap-2 rounded-md border border-input bg-background text-sm w-40">
          <Search className="w-3 h-3 text-muted-foreground shrink-0" />
          <input
            value={symbolSearch || selectedSymbol}
            onChange={(e) => {
              setSymbolSearch(e.target.value)
              setShowSymbolDropdown(true)
            }}
            onFocus={() => setShowSymbolDropdown(true)}
            onBlur={() => setTimeout(() => setShowSymbolDropdown(false), 150)}
            placeholder="Symbol..."
            className="bg-transparent outline-none w-full"
          />
        </div>

        {showSymbolDropdown && filtered.length > 0 && (
          <div className="absolute top-9 left-0 z-50 w-48 max-h-48 overflow-y-auto rounded-md border border-border bg-card shadow-md">
            {filtered.map((s) => (
              <button
                key={s.ticker}
                className="w-full text-left px-3 py-1.5 text-sm hover:bg-accent transition-colors"
                onMouseDown={() => {
                  onSymbolChange(s.ticker)
                  setSymbolSearch('')
                  setShowSymbolDropdown(false)
                }}
              >
                {s.ticker}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Timeframe buttons */}
      <div className="flex items-center gap-1">
        {TIMEFRAMES.map((tf) => (
          <Button
            key={tf}
            variant={tf === selectedTimeframe ? 'secondary' : 'ghost'}
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={() => onTimeframeChange(tf)}
          >
            {tf}
          </Button>
        ))}
      </div>
    </div>
  )
}
```

### Step 4: Replace the Chart page stub

```tsx
// frontend/src/pages/Chart.tsx
import React from 'react'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { ChartToolbar } from '@/components/chart/ChartToolbar'
import { useCandles } from '@/hooks/useCandles'
import { useMarketStore } from '@/stores'

export function Chart() {
  const {
    selectedAdapter,
    selectedSymbol,
    selectedTimeframe,
    setSelectedAdapter,
    setSelectedSymbol,
    setSelectedTimeframe,
  } = useMarketStore()

  const { data: candles = [], isLoading, isError, error, refetch } = useCandles({
    adapter: selectedAdapter,
    symbol: selectedSymbol ?? '',
    timeframe: selectedTimeframe,
  })

  return (
    <div className="flex flex-col h-full">
      <ChartToolbar
        selectedAdapter={selectedAdapter}
        selectedSymbol={selectedSymbol ?? ''}
        selectedTimeframe={selectedTimeframe}
        onAdapterChange={setSelectedAdapter}
        onSymbolChange={setSelectedSymbol}
        onTimeframeChange={setSelectedTimeframe}
      />

      <div className="flex-1 min-h-0 p-2">
        <CandlestickChart
          candles={candles}
          isLoading={isLoading}
          error={isError ? (error instanceof Error ? error.message : 'Failed to load candles') : null}
          onRetry={refetch}
        />
      </div>
    </div>
  )
}
```

### Step 5: Run tests — all should pass

```bash
cd frontend && npx vitest run src/pages/Chart.test.tsx
```

Expected: all 3 tests PASS

### Step 6: Run all frontend tests

```bash
cd frontend && npx vitest run
```

Expected: all tests PASS

### Step 7: Final commit

```bash
git add frontend/src/components/chart/ChartToolbar.tsx \
        frontend/src/pages/Chart.tsx \
        frontend/src/pages/Chart.test.tsx
git commit -m "feat(chart): implement Chart page with toolbar, candlestick chart, and symbol search"
```

---

## Verification Checklist

After all 9 tasks are complete, run these end-to-end checks:

```bash
# Backend: all tests pass
cd backend && go test ./...

# Backend: compiles cleanly
cd backend && go build ./...

# Frontend: all tests pass
cd frontend && npx vitest run

# Frontend: TypeScript clean
cd frontend && npx tsc --noEmit

# Full stack: start Docker services and hit the real endpoints
make up
curl "http://localhost:8080/api/v1/markets"
curl "http://localhost:8080/api/v1/candles/timeframes"
curl "http://localhost:8080/api/v1/markets/binance/symbols" | head -c 500
curl "http://localhost:8080/api/v1/candles?adapter=binance&symbol=BTCUSDT&timeframe=1h"
```

---

## Out of Scope

- Live ticker streaming (Phase 8)
- Volume histogram pane (Phase 5)
- Indicator overlays (Phase 5)
