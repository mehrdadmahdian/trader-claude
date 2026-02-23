package registry

import (
	"context"
	"time"
)

// --- Core data types ---

// Candle represents a single OHLCV candlestick
type Candle struct {
	Symbol    string
	Market    string // e.g. "crypto", "stock", "forex"
	Timeframe string // e.g. "1m", "5m", "1h", "1d"
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// Tick represents a real-time price update
type Tick struct {
	Symbol    string
	Market    string
	Price     float64
	Volume    float64
	Timestamp time.Time
	Bid       float64
	Ask       float64
}

// Symbol represents a tradeable asset
type Symbol struct {
	ID          string // e.g. "BTC/USDT"
	Market      string // e.g. "crypto"
	BaseAsset   string // e.g. "BTC"
	QuoteAsset  string // e.g. "USDT"
	Description string
	Active      bool
}

// Signal represents a buy/sell/hold signal from a strategy
type Signal struct {
	Symbol    string
	Market    string
	Direction string  // "long", "short", "flat"
	Strength  float64 // 0.0 – 1.0
	Price     float64
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// StrategyState holds serializable strategy state for live trading
type StrategyState struct {
	StrategyID string
	Symbol     string
	Market     string
	State      map[string]interface{}
	UpdatedAt  time.Time
}

// ParamDefinition describes a user-configurable strategy parameter
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

// --- Adapter interface ---

// MarketAdapter is the interface all market data providers must implement
type MarketAdapter interface {
	// Name returns the unique adapter identifier (e.g. "binance", "alpaca")
	Name() string

	// Markets returns the markets this adapter supports (e.g. ["crypto"])
	Markets() []string

	// FetchCandles fetches historical OHLCV data
	FetchCandles(ctx context.Context, symbol, market, timeframe string, from, to time.Time) ([]Candle, error)

	// FetchSymbols returns all available symbols for this adapter
	FetchSymbols(ctx context.Context, market string) ([]Symbol, error)

	// SubscribeTicks starts streaming real-time ticks; sends to returned channel
	SubscribeTicks(ctx context.Context, symbols []string, market string) (<-chan Tick, error)

	// IsHealthy returns true if the adapter can reach its data source
	IsHealthy(ctx context.Context) bool
}

// --- Strategy interface ---

// Strategy is the interface all trading strategies must implement
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
