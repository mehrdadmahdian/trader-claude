# Adding a Trading Strategy

This guide walks through creating a new trading strategy. A strategy processes market data (candles and ticks) to generate buy/sell/hold signals.

**Example:** Add a Bollinger Bands strategy alongside EMA Crossover, RSI, and MACD.

## Step 1: Understand the Interface

All strategies implement `registry.Strategy`:

```go
// internal/registry/interfaces.go

type Strategy interface {
    // Name returns the unique strategy identifier
    Name() string

    // Description returns a human-readable description
    Description() string

    // Params returns the list of configurable parameters
    Params() []ParamDefinition

    // Init initializes the strategy with the given parameters
    Init(params map[string]interface{}) error

    // OnCandle processes a new candle and optionally returns a signal
    OnCandle(candle Candle, state *StrategyState) (*Signal, error)

    // OnTick processes a real-time tick (optional, return nil to skip)
    OnTick(tick Tick, state *StrategyState) (*Signal, error)

    // Reset clears internal state
    Reset()
}
```

**Data types:**

```go
type Candle struct {
    Symbol    string
    Market    string
    Timeframe string
    Timestamp time.Time
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    float64
}

type Signal struct {
    Symbol    string
    Market    string
    Direction string  // "long", "short", "flat"
    Strength  float64 // 0.0 – 1.0
    Price     float64
    Timestamp time.Time
    Metadata  map[string]interface{}
}

type ParamDefinition struct {
    Name        string
    Type        string // "int", "float", "bool", "string", "select"
    Default     interface{}
    Min         interface{} // for numeric types
    Max         interface{} // for numeric types
    Options     []string    // for "select" type
    Description string
    Required    bool
}
```

## Step 2: Create the Strategy File

Create `backend/internal/strategy/bollinger_bands.go`:

```go
package strategy

import (
	"fmt"
	"math"

	"github.com/trader-claude/backend/internal/registry"
)

// BollingerBands implements a Bollinger Bands trading strategy.
// A long signal is emitted when price closes below the lower band (oversold),
// and a short signal when price closes above the upper band (overbought).
type BollingerBands struct {
	// Parameters
	period int
	stdDev float64

	// Internal state
	closes []float64
	warmup int
}

// Name returns the unique strategy identifier
func (b *BollingerBands) Name() string {
	return "bollinger_bands"
}

// Description returns a human-readable description
func (b *BollingerBands) Description() string {
	return "Bollinger Bands: long when price closes below lower band, short when above upper band."
}

// Params returns the list of configurable parameters
func (b *BollingerBands) Params() []registry.ParamDefinition {
	return []registry.ParamDefinition{
		{
			Name:        "period",
			Type:        "int",
			Default:     20,
			Min:         5,
			Max:         200,
			Description: "SMA period for mean calculation",
			Required:    false,
		},
		{
			Name:        "std_dev",
			Type:        "float",
			Default:     2.0,
			Min:         0.5,
			Max:         5.0,
			Description: "Number of standard deviations for bands",
			Required:    false,
		},
	}
}

// Init initializes the strategy with the provided parameters
func (b *BollingerBands) Init(params map[string]interface{}) error {
	b.period = paramInt(params, "period", 20)
	b.stdDev = paramFloat(params, "std_dev", 2.0)

	if b.period < 2 {
		return fmt.Errorf("period must be >= 2, got %d", b.period)
	}
	if b.stdDev <= 0 {
		return fmt.Errorf("std_dev must be > 0, got %f", b.stdDev)
	}

	b.Reset()
	return nil
}

// Reset clears all internal state
func (b *BollingerBands) Reset() {
	b.closes = nil
	b.warmup = 0
}

// OnCandle processes a new candle and optionally returns a signal
func (b *BollingerBands) OnCandle(
	candle registry.Candle,
	_ *registry.StrategyState,
) (*registry.Signal, error) {
	b.closes = append(b.closes, candle.Close)
	b.warmup++

	// Need at least `period` closes to calculate meaningful bands
	if b.warmup < b.period {
		return nil, nil
	}

	// Calculate SMA (simple moving average) over the last `period` closes
	var sum float64
	for i := len(b.closes) - b.period; i < len(b.closes); i++ {
		sum += b.closes[i]
	}
	sma := sum / float64(b.period)

	// Calculate standard deviation
	var variance float64
	for i := len(b.closes) - b.period; i < len(b.closes); i++ {
		diff := b.closes[i] - sma
		variance += diff * diff
	}
	variance /= float64(b.period)
	stdDeviation := math.Sqrt(variance)

	// Calculate bands
	upperBand := sma + (b.stdDev * stdDeviation)
	lowerBand := sma - (b.stdDev * stdDeviation)

	// Generate signal
	currentPrice := candle.Close

	if currentPrice < lowerBand {
		// Price below lower band: oversold, long signal
		strength := 1.0 - ((currentPrice - (lowerBand - stdDeviation)) / stdDeviation)
		if strength > 1.0 {
			strength = 1.0
		}
		if strength < 0.5 {
			strength = 0.5
		}
		return &registry.Signal{
			Symbol:    candle.Symbol,
			Market:    candle.Market,
			Direction: "long",
			Strength:  strength,
			Price:     currentPrice,
			Timestamp: candle.Timestamp,
			Metadata: map[string]interface{}{
				"sma":        sma,
				"upper_band": upperBand,
				"lower_band": lowerBand,
				"std_dev":    stdDeviation,
			},
		}, nil
	}

	if currentPrice > upperBand {
		// Price above upper band: overbought, short signal
		strength := 1.0 - ((upperBand + stdDeviation - currentPrice) / stdDeviation)
		if strength > 1.0 {
			strength = 1.0
		}
		if strength < 0.5 {
			strength = 0.5
		}
		return &registry.Signal{
			Symbol:    candle.Symbol,
			Market:    candle.Market,
			Direction: "short",
			Strength:  strength,
			Price:     currentPrice,
			Timestamp: candle.Timestamp,
			Metadata: map[string]interface{}{
				"sma":        sma,
				"upper_band": upperBand,
				"lower_band": lowerBand,
				"std_dev":    stdDeviation,
			},
		}, nil
	}

	// Price within bands: no signal
	return nil, nil
}

// OnTick processes a real-time tick (optional)
func (b *BollingerBands) OnTick(
	tick registry.Tick,
	state *registry.StrategyState,
) (*registry.Signal, error) {
	// For this strategy, we only generate signals on candle close
	return nil, nil
}
```

## Step 3: Create Unit Tests

Create `backend/internal/strategy/bollinger_bands_test.go`:

```go
package strategy

import (
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

func TestBollingerBandsName(t *testing.T) {
	bb := &BollingerBands{}
	if bb.Name() != "bollinger_bands" {
		t.Errorf("expected name 'bollinger_bands', got %q", bb.Name())
	}
}

func TestBollingerBandsDescription(t *testing.T) {
	bb := &BollingerBands{}
	desc := bb.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestBollingerBandsParams(t *testing.T) {
	bb := &BollingerBands{}
	params := bb.Params()
	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Verify period param
	if params[0].Name != "period" || params[0].Type != "int" {
		t.Errorf("expected param 'period' of type 'int'")
	}
	if params[0].Default != 20 {
		t.Errorf("expected default period 20, got %v", params[0].Default)
	}

	// Verify std_dev param
	if params[1].Name != "std_dev" || params[1].Type != "float" {
		t.Errorf("expected param 'std_dev' of type 'float'")
	}
}

func TestBollingerBandsInit(t *testing.T) {
	bb := &BollingerBands{}
	err := bb.Init(map[string]interface{}{
		"period":  20,
		"std_dev": 2.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bb.period != 20 {
		t.Errorf("expected period 20, got %d", bb.period)
	}
	if bb.stdDev != 2.0 {
		t.Errorf("expected stdDev 2.0, got %f", bb.stdDev)
	}
}

func TestBollingerBandsInitValidation(t *testing.T) {
	tests := []struct {
		params  map[string]interface{}
		wantErr bool
	}{
		{
			params:  map[string]interface{}{"period": 1, "std_dev": 2.0},
			wantErr: true, // period too small
		},
		{
			params:  map[string]interface{}{"period": 20, "std_dev": 0},
			wantErr: true, // std_dev must be positive
		},
		{
			params:  map[string]interface{}{"period": 20, "std_dev": -1.0},
			wantErr: true, // std_dev must be positive
		},
		{
			params:  map[string]interface{}{"period": 20, "std_dev": 2.0},
			wantErr: false, // valid
		},
	}

	for i, tt := range tests {
		bb := &BollingerBands{}
		err := bb.Init(tt.params)
		if (err != nil) != tt.wantErr {
			t.Errorf("test %d: unexpected error result: got %v, want error=%v", i, err, tt.wantErr)
		}
	}
}

func TestBollingerBandsOnCandle(t *testing.T) {
	bb := &BollingerBands{}
	bb.Init(map[string]interface{}{"period": 5, "std_dev": 2.0})

	now := time.Now()

	// Feed 5 candles with rising prices
	prices := []float64{100, 101, 102, 103, 104}
	for i, p := range prices {
		candle := registry.Candle{
			Symbol:    "BTC/USDT",
			Market:    "crypto",
			Timeframe: "1h",
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Open:      p,
			High:      p + 1,
			Low:       p - 1,
			Close:     p,
			Volume:    100,
		}
		signal, err := bb.OnCandle(candle, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should not generate signal until we have enough data
		// (but after 5 candles, we start checking)
		if i < 4 && signal != nil {
			t.Errorf("expected no signal before warmup, got %v", signal)
		}
	}

	// Add a candle that closes below the lower band (oversold)
	lowCandle := registry.Candle{
		Symbol:    "BTC/USDT",
		Market:    "crypto",
		Timeframe: "1h",
		Timestamp: now.Add(6 * time.Hour),
		Open:      95,
		High:      96,
		Low:       94,
		Close:     94, // Below expected lower band
		Volume:    100,
	}
	signal, err := bb.OnCandle(lowCandle, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if signal != nil && signal.Direction != "long" {
		t.Errorf("expected long signal for oversold, got %q", signal.Direction)
	}
}

func TestBollingerBandsReset(t *testing.T) {
	bb := &BollingerBands{}
	bb.Init(map[string]interface{}{"period": 20, "std_dev": 2.0})

	// Add some data
	bb.closes = []float64{100, 101, 102}
	bb.warmup = 3

	// Reset
	bb.Reset()

	if len(bb.closes) != 0 {
		t.Errorf("expected empty closes after reset, got %d", len(bb.closes))
	}
	if bb.warmup != 0 {
		t.Errorf("expected warmup=0 after reset, got %d", bb.warmup)
	}
}
```

## Step 4: Add Helper Functions

Add these to `backend/internal/strategy/helpers.go` (or at the end of your strategy file):

```go
package strategy

import (
	"strconv"
)

// paramInt safely extracts an int parameter, with fallback to default
func paramInt(params map[string]interface{}, key string, def int) int {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			i, err := strconv.Atoi(val)
			if err == nil {
				return i
			}
		}
	}
	return def
}

// paramFloat safely extracts a float parameter, with fallback to default
func paramFloat(params map[string]interface{}, key string, def float64) float64 {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case string:
			f, err := strconv.ParseFloat(val, 64)
			if err == nil {
				return f
			}
		}
	}
	return def
}

// paramBool safely extracts a bool parameter, with fallback to default
func paramBool(params map[string]interface{}, key string, def bool) bool {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return val == "true" || val == "1" || val == "yes"
		}
	}
	return def
}

// paramString safely extracts a string parameter, with fallback to default
func paramString(params map[string]interface{}, key string, def string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}
```

## Step 5: Register the Strategy

In `backend/cmd/server/main.go`, add registration in the startup sequence:

```go
// 4b. Register strategies
registry.Strategies().Register("ema_crossover", func() registry.Strategy { return &strategy.EMACrossover{} })
registry.Strategies().Register("rsi", func() registry.Strategy { return &strategy.RSIStrategy{} })
registry.Strategies().Register("macd", func() registry.Strategy { return &strategy.MACDSignal{} })
registry.Strategies().Register("bollinger_bands", func() registry.Strategy { return &strategy.BollingerBands{} })  // ← ADD THIS
log.Printf("registered strategies: %v", registry.Strategies().Names())
```

## Step 6: Test the Strategy

Run your tests:

```bash
cd backend
go test ./internal/strategy -run TestBollingerBands -v
```

Test via the API:

```bash
# List all strategies
curl http://localhost:8080/api/v1/strategies

# Get parameters for a strategy
curl http://localhost:8080/api/v1/strategies/bollinger_bands/params

# Run a backtest with the strategy (if backtest endpoint is implemented)
curl -X POST http://localhost:8080/api/v1/backtest \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTC/USDT",
    "market": "crypto",
    "adapter": "binance",
    "strategy": "bollinger_bands",
    "params": {"period": 20, "std_dev": 2.0},
    "from": "2023-01-01",
    "to": "2023-12-31"
  }'
```

## Step 7: Frontend Integration

The frontend automatically picks up strategies via the `GET /api/v1/strategies` endpoint.

To customize the UI, add to `frontend/src/types/index.ts`:

```typescript
// Already exists, just ensure bollinger_bands is recognized:
export type StrategyName = 'ema_crossover' | 'rsi' | 'macd' | 'bollinger_bands';

// Strategy parameter UI hints (optional)
export const strategyLabels: Record<string, string> = {
  ema_crossover: 'EMA Crossover',
  rsi: 'RSI',
  macd: 'MACD Signal',
  bollinger_bands: 'Bollinger Bands',
};
```

The backtest page automatically renders parameter inputs based on the `Params()` response.

## Advanced: Multi-Timeframe Strategies

For strategies that use multiple timeframes (e.g., trade on 1h but signal from 4h):

```go
type MultiTimeframeStrategy struct {
    primary   []float64 // Primary timeframe closes
    secondary []float64 // Secondary timeframe closes
    // ...
}

func (m *MultiTimeframeStrategy) OnCandle(
    candle registry.Candle,
    state *registry.StrategyState,
) (*registry.Signal, error) {
    // Check the candle's timeframe
    if candle.Timeframe == "1h" {
        m.primary = append(m.primary, candle.Close)
    } else if candle.Timeframe == "4h" {
        m.secondary = append(m.secondary, candle.Close)
    }

    // Generate signal only when conditions on both timeframes align
    // ...
}
```

## Complete Example: Simple SMA Crossover

Here's a minimal strategy skeleton for reference:

```go
package strategy

import (
	"github.com/trader-claude/backend/internal/registry"
)

type SimpleSMA struct {
	period int
	closes []float64
}

func (s *SimpleSMA) Name() string { return "simple_sma" }
func (s *SimpleSMA) Description() string { return "Simple SMA trend follower" }

func (s *SimpleSMA) Params() []registry.ParamDefinition {
	return []registry.ParamDefinition{
		{
			Name:        "period",
			Type:        "int",
			Default:     20,
			Min:         5,
			Max:         200,
			Description: "SMA period",
			Required:    false,
		},
	}
}

func (s *SimpleSMA) Init(params map[string]interface{}) error {
	s.period = paramInt(params, "period", 20)
	s.Reset()
	return nil
}

func (s *SimpleSMA) Reset() {
	s.closes = nil
}

func (s *SimpleSMA) OnCandle(candle registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	s.closes = append(s.closes, candle.Close)
	if len(s.closes) < s.period {
		return nil, nil
	}

	var sum float64
	for i := len(s.closes) - s.period; i < len(s.closes); i++ {
		sum += s.closes[i]
	}
	sma := sum / float64(s.period)

	if candle.Close > sma {
		return &registry.Signal{
			Symbol:    candle.Symbol,
			Market:    candle.Market,
			Direction: "long",
			Strength:  0.5,
			Price:     candle.Close,
			Timestamp: candle.Timestamp,
		}, nil
	}

	return nil, nil
}

func (s *SimpleSMA) OnTick(tick registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}
```

## Best Practices

1. **Warmup Period**: Don't generate signals until you have enough historical data (e.g., `warmup >= period`)
2. **State Isolation**: Use `StrategyState` for persisting state across candles; `Reset()` when restarting
3. **Metadata**: Include calculation details (SMA, bands, etc.) in `Signal.Metadata` for debugging
4. **Parameter Validation**: In `Init()`, validate that parameters make sense (e.g., fast < slow)
5. **Error Handling**: Return detailed error messages; never panic
6. **Memory Efficiency**: For long-running backtests, only keep necessary historical data (e.g., last 500 closes instead of all)
7. **Floating-Point Precision**: Be aware of rounding errors; consider using `big.Decimal` for very precise calculations
8. **Signal Strength**: Use 0.0–1.0 scale; 1.0 = highest confidence, 0.5 = moderate, 0.1 = weak
9. **Real-Time Gaps**: `OnTick()` is optional; if you don't need it, just return `nil, nil`

## Troubleshooting

**"strategy <name> not found" error:**
- Ensure you called `registry.Strategies().Register()` in `main.go`
- Check the name matches exactly (case-sensitive)

**Signal never generated:**
- Add debug logging: `log.Printf("warmup=%d, period=%d", s.warmup, s.period)`
- Verify `OnCandle()` is receiving data with realistic prices
- Check parameter validation in `Init()` isn't too restrictive

**Backtest returns no trades:**
- Strategy might be generating signals, but monitor isn't executing them
- Check that the signal `Direction` is "long" or "short", not something else

**Parameter not showing in UI:**
- Run `curl http://localhost:8080/api/v1/strategies/my_strategy/params` to verify
- Ensure `Type` is one of: `"int"`, `"float"`, `"bool"`, `"string"`, `"select"`
