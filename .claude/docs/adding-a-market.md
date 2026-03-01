# Adding a Market Adapter

This guide walks through creating a new market data adapter (e.g., Kraken, Yahoo Finance, Interactive Brokers). An adapter provides historical candles, symbol lists, and real-time ticks for a data source.

**Example:** Add a Kraken crypto adapter alongside the existing Binance adapter.

## Step 1: Understand the Interface

All adapters implement `registry.MarketAdapter`:

```go
// internal/registry/interfaces.go

type MarketAdapter interface {
    // Name returns the unique adapter identifier (e.g. "binance", "kraken")
    Name() string

    // Markets returns the markets this adapter supports (e.g. ["crypto"])
    Markets() []string

    // FetchCandles fetches historical OHLCV data
    FetchCandles(ctx context.Context, symbol, market, timeframe string,
        from, to time.Time) ([]Candle, error)

    // FetchSymbols returns all available symbols for this adapter
    FetchSymbols(ctx context.Context, market string) ([]Symbol, error)

    // SubscribeTicks starts streaming real-time ticks; sends to returned channel
    SubscribeTicks(ctx context.Context, symbols []string, market string)
        (<-chan Tick, error)

    // IsHealthy returns true if the adapter can reach its data source
    IsHealthy(ctx context.Context) bool
}
```

**Data types:**

```go
type Candle struct {
    Symbol    string
    Market    string // e.g. "crypto", "stock"
    Timeframe string // e.g. "1m", "5m", "1h", "1d"
    Timestamp time.Time
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    float64
}

type Tick struct {
    Symbol    string
    Market    string
    Price     float64
    Volume    float64
    Timestamp time.Time
    Bid       float64
    Ask       float64
}

type Symbol struct {
    ID          string // e.g. "BTC/USDT"
    Market      string // e.g. "crypto"
    BaseAsset   string // e.g. "BTC"
    QuoteAsset  string // e.g. "USDT"
    Description string
    Active      bool
}
```

## Step 2: Create the Adapter File

Create `backend/internal/adapter/kraken.go`:

```go
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

const (
	krakenBaseURL   = "https://api.kraken.com"
	krakenWSBaseURL = "wss://ws.kraken.com"
)

// KrakenAdapter implements registry.MarketAdapter for Kraken crypto exchange
type KrakenAdapter struct {
	client  *http.Client
	baseURL string
	apiKey  string // optional: for authenticated endpoints
	secret  string
}

// NewKrakenAdapter creates a KrakenAdapter
func NewKrakenAdapter(apiKey, secret string) *KrakenAdapter {
	return &KrakenAdapter{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: krakenBaseURL,
		apiKey:  apiKey,
		secret:  secret,
	}
}

// Name returns the unique adapter identifier
func (k *KrakenAdapter) Name() string {
	return "kraken"
}

// Markets returns the markets this adapter supports
func (k *KrakenAdapter) Markets() []string {
	return []string{"crypto"}
}

// IsHealthy returns true if Kraken API is reachable
func (k *KrakenAdapter) IsHealthy(ctx context.Context) bool {
	url := k.baseURL + "/0/public/Time"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// FetchCandles fetches historical OHLCV data from Kraken
func (k *KrakenAdapter) FetchCandles(
	ctx context.Context,
	symbol, market, timeframe string,
	from, to time.Time,
) ([]registry.Candle, error) {
	// Example: convert trader-claude timeframe (e.g., "1h") to Kraken format (e.g., "60")
	interval, err := krakenMapTimeframe(timeframe)
	if err != nil {
		return nil, fmt.Errorf("kraken FetchCandles: %w", err)
	}

	// Convert symbol format: "BTC/USDT" -> "XBTUSDT" (Kraken format)
	krakenSym := toKrakenSymbol(symbol)

	// Build REST URL
	url := fmt.Sprintf(
		"%s/0/public/OHLC?pair=%s&interval=%d&since=%d",
		k.baseURL, krakenSym, interval, from.Unix(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kraken API returned %d", resp.StatusCode)
	}

	// Parse response and convert to []registry.Candle
	var result KrakenOHLCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Error) > 0 {
		return nil, fmt.Errorf("kraken error: %s", strings.Join(result.Error, ", "))
	}

	// Convert Kraken format to registry.Candle
	candles := make([]registry.Candle, 0)
	for _, pair := range result.Result {
		for _, kLine := range pair {
			c := registry.Candle{
				Symbol:    symbol,
				Market:    market,
				Timeframe: timeframe,
				Timestamp: time.Unix(int64(kLine[0]), 0),
				Open:      kLine[1],
				High:      kLine[2],
				Low:       kLine[3],
				Close:     kLine[4],
				Volume:    kLine[6],
			}
			if c.Timestamp.Before(to) {
				candles = append(candles, c)
			}
		}
	}

	return candles, nil
}

// FetchSymbols returns all available symbols from Kraken
func (k *KrakenAdapter) FetchSymbols(ctx context.Context, market string) ([]registry.Symbol, error) {
	url := k.baseURL + "/0/public/AssetPairs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kraken API returned %d", resp.StatusCode)
	}

	var result KrakenAssetPairsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	symbols := make([]registry.Symbol, 0)
	for _, pair := range result.Result {
		sym := registry.Symbol{
			ID:         pair.AltName, // e.g., "XBTUSDT"
			Market:     "crypto",
			BaseAsset:  pair.Base,
			QuoteAsset: pair.Quote,
			Active:     true,
		}
		symbols = append(symbols, sym)
	}

	return symbols, nil
}

// SubscribeTicks streams real-time ticks via WebSocket
func (k *KrakenAdapter) SubscribeTicks(
	ctx context.Context,
	symbols []string,
	market string,
) (<-chan registry.Tick, error) {
	tickChan := make(chan registry.Tick, 256)

	go func() {
		defer close(tickChan)
		// TODO: Implement WebSocket connection to Kraken feed
		// 1. Dial wss://ws.kraken.com
		// 2. Send subscribe message for each symbol
		// 3. Parse incoming messages
		// 4. Send to tickChan
		// 5. Reconnect on disconnect
	}()

	return tickChan, nil
}

// --- Helper functions ---

// krakenMapTimeframe converts timeframe strings (e.g., "1h") to Kraken format (e.g., "60")
func krakenMapTimeframe(tf string) (int, error) {
	mapping := map[string]int{
		"1m":  1,
		"5m":  5,
		"15m": 15,
		"30m": 30,
		"1h":  60,
		"4h":  240,
		"1d":  1440,
		"1w":  10080,
	}
	if v, ok := mapping[tf]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("unsupported timeframe: %s", tf)
}

// toKrakenSymbol converts "BTC/USDT" to Kraken format "XBTUSDT"
func toKrakenSymbol(symbol string) string {
	// Kraken uses X prefix for crypto: BTC -> XBTUSDT
	// This is a simplification; real implementation would use asset info API
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return symbol
	}
	base := parts[0]
	quote := parts[1]
	if base == "BTC" {
		base = "XBT"
	}
	return base + quote
}

// --- Response types (for JSON unmarshaling) ---

type KrakenOHLCResponse struct {
	Error  []string `json:"error"`
	Result map[string][][]float64 `json:"result"`
}

type KrakenAssetPairsResponse struct {
	Error  []string `json:"error"`
	Result map[string]KrakenAssetPair `json:"result"`
}

type KrakenAssetPair struct {
	AltName string `json:"altname"`
	Base    string `json:"base"`
	Quote   string `json:"quote"`
}
```

## Step 3: Create Unit Tests

Create `backend/internal/adapter/kraken_test.go`:

```go
package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKrakenAdapterName(t *testing.T) {
	adapter := NewKrakenAdapter("", "")
	if adapter.Name() != "kraken" {
		t.Errorf("expected name 'kraken', got %q", adapter.Name())
	}
}

func TestKrakenAdapterMarkets(t *testing.T) {
	adapter := NewKrakenAdapter("", "")
	markets := adapter.Markets()
	if len(markets) != 1 || markets[0] != "crypto" {
		t.Errorf("expected markets ['crypto'], got %v", markets)
	}
}

func TestKrakenFetchCandles(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return mock OHLC data
		w.Write([]byte(`{
			"error": [],
			"result": {
				"XBTUSDT": [
					[1609459200, 29000, 29500, 28500, 29100, 5000]
				]
			}
		}`))
	}))
	defer server.Close()

	adapter := NewKrakenAdapter("", "")
	adapter.baseURL = server.URL

	ctx := context.Background()
	candles, err := adapter.FetchCandles(
		ctx,
		"BTC/USDT",
		"crypto",
		"1h",
		time.Now().Add(-24*time.Hour),
		time.Now(),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candles) == 0 {
		t.Fatal("expected at least one candle")
	}

	c := candles[0]
	if c.Close != 29100 {
		t.Errorf("expected close 29100, got %v", c.Close)
	}
}

func TestKrakenMapTimeframe(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"1m", 1},
		{"5m", 5},
		{"1h", 60},
		{"1d", 1440},
	}

	for _, tt := range tests {
		v, err := krakenMapTimeframe(tt.input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tt.input, err)
		}
		if v != tt.expected {
			t.Errorf("krakenMapTimeframe(%q) = %d, want %d", tt.input, v, tt.expected)
		}
	}

	// Test invalid timeframe
	_, err := krakenMapTimeframe("999m")
	if err == nil {
		t.Error("expected error for invalid timeframe")
	}
}

func TestToKrakenSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC/USDT", "XBTUSDT"},
		{"ETH/USD", "ETHUSDT"},
	}

	for _, tt := range tests {
		got := toKrakenSymbol(tt.input)
		if got != tt.expected {
			t.Errorf("toKrakenSymbol(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
```

## Step 4: Register the Adapter

In `backend/cmd/server/main.go`, add registration in the startup sequence:

```go
// 4. Register market adapters
registry.Adapters().Register(adapter.NewBinanceAdapter(""))
registry.Adapters().Register(adapter.NewYahooAdapter())
registry.Adapters().Register(adapter.NewKrakenAdapter("", ""))  // ← ADD THIS LINE
log.Printf("registered adapters: %v", registry.Adapters().Names())
```

If you need API credentials, read from environment:

```go
import "github.com/trader-claude/backend/internal/config"

// In main():
cfg, err := config.Load()
// ...
registry.Adapters().Register(
    adapter.NewKrakenAdapter(cfg.Kraken.APIKey, cfg.Kraken.Secret),
)
```

And add to `backend/internal/config/config.go`:

```go
type Config struct {
    App    AppConfig
    DB     DBConfig
    Redis  RedisConfig
    Kraken KrakenConfig  // ← ADD THIS
    // ...
}

type KrakenConfig struct {
    APIKey string
    Secret string
}

func Load() (*Config, error) {
    cfg := &Config{
        // ...
        Kraken: KrakenConfig{
            APIKey: os.Getenv("KRAKEN_API_KEY"),
            Secret: os.Getenv("KRAKEN_API_SECRET"),
        },
        // ...
    }
    return cfg, nil
}
```

## Step 5: Add Environment Variables

Update `.env.example`:

```env
# Kraken
KRAKEN_API_KEY=your-api-key-here
KRAKEN_API_SECRET=your-api-secret-here
```

## Step 6: Test the Adapter

Run your tests:

```bash
cd backend
go test ./internal/adapter -run TestKraken -v
```

Test via the API:

```bash
# List all adapters (including kraken)
curl http://localhost:8080/api/v1/adapters

# Fetch candles from Kraken
curl "http://localhost:8080/api/v1/candles?symbol=BTC/USDT&market=crypto&adapter=kraken&timeframe=1h"

# Fetch symbols from Kraken
curl "http://localhost:8080/api/v1/symbols?adapter=kraken&market=crypto"
```

## Step 7: Document

Add to `.claude/docs/api.md`:

```markdown
## Adapters Endpoint

### List Adapters
**GET** `/api/v1/adapters`

Returns all registered market adapters.

**Response:**
```json
{
  "adapters": [
    {"name": "binance", "markets": ["crypto"]},
    {"name": "kraken", "markets": ["crypto"]},
    {"name": "yahoo", "markets": ["stock"]}
  ]
}
```
```

## Complete Example: Minimal Adapter

Here's a minimal adapter skeleton for reference:

```go
package adapter

import (
	"context"
	"time"
	"github.com/trader-claude/backend/internal/registry"
)

type MinimalAdapter struct{}

func (m *MinimalAdapter) Name() string { return "minimal" }
func (m *MinimalAdapter) Markets() []string { return []string{"stock"} }

func (m *MinimalAdapter) IsHealthy(ctx context.Context) bool {
	return true
}

func (m *MinimalAdapter) FetchCandles(
	ctx context.Context,
	symbol, market, timeframe string,
	from, to time.Time,
) ([]registry.Candle, error) {
	// TODO: Implement
	return []registry.Candle{}, nil
}

func (m *MinimalAdapter) FetchSymbols(
	ctx context.Context,
	market string,
) ([]registry.Symbol, error) {
	// TODO: Implement
	return []registry.Symbol{}, nil
}

func (m *MinimalAdapter) SubscribeTicks(
	ctx context.Context,
	symbols []string,
	market string,
) (<-chan registry.Tick, error) {
	// TODO: Implement
	return make(chan registry.Tick), nil
}
```

## Best Practices

1. **Timeframe Mapping**: Always support `1m, 5m, 15m, 30m, 1h, 4h, 1d, 1w` if the exchange has data
2. **Error Handling**: Return detailed error messages with API status codes
3. **Rate Limiting**: Implement backoff for API retries (avoid hammering the exchange)
4. **Caching**: Leverage the `DataService` for automatic candle caching in Redis
5. **Symbol Precision**: Use the exchange's native symbol format; convert from trader-claude format in helper functions
6. **Decimal Precision**: Always use `float64` internally; ensure accuracy for prices (use `big.Decimal` if needed for non-Go systems)
7. **Timeframe Validation**: Reject unsupported timeframes early; don't guess
8. **Health Checks**: Implement `IsHealthy()` to be called by health endpoint; use a lightweight ping endpoint

## Troubleshooting

**"adapter <name> not found" error:**
- Ensure you called `registry.Adapters().Register()` in `main.go`
- Check the `Name()` method returns the expected string

**Candle data has timezone issues:**
- Kraken/Binance return UTC timestamps; ensure you convert to the right timezone
- Use `time.Unix()` for Unix timestamps

**WebSocket reconnection fails:**
- Implement exponential backoff before reconnecting
- Use context cancellation (`<-ctx.Done()`) to gracefully shut down

**Rate limits hit:**
- Add caching via `DataService`
- Implement request queuing with delays
- Consider using cached candles instead of live requests
