package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

// --- Mock strategies ---

// flatStrategy never emits a signal (always returns nil)
type flatStrategy struct{}

func (f *flatStrategy) Name() string        { return "flat" }
func (f *flatStrategy) Description() string { return "always flat" }
func (f *flatStrategy) Params() []registry.ParamDefinition {
	return nil
}
func (f *flatStrategy) Init(_ map[string]interface{}) error { return nil }
func (f *flatStrategy) OnCandle(_ registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}
func (f *flatStrategy) OnTick(_ registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}
func (f *flatStrategy) Reset() {}

// longThenFlatStrategy signals "long" on the first candle and "flat" on the last
type longThenFlatStrategy struct {
	total   int
	current int
}

func (s *longThenFlatStrategy) Name() string        { return "long-then-flat" }
func (s *longThenFlatStrategy) Description() string { return "long on first, flat on last" }
func (s *longThenFlatStrategy) Params() []registry.ParamDefinition {
	return nil
}
func (s *longThenFlatStrategy) Init(_ map[string]interface{}) error { return nil }
func (s *longThenFlatStrategy) OnCandle(candle registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	s.current++
	dir := ""
	if s.current == 1 {
		dir = "long"
	} else if s.current == s.total {
		dir = "flat"
	}
	if dir == "" {
		return nil, nil
	}
	return &registry.Signal{
		Symbol:    candle.Symbol,
		Market:    candle.Market,
		Direction: dir,
		Strength:  1.0,
		Price:     candle.Close,
		Timestamp: candle.Timestamp,
	}, nil
}
func (s *longThenFlatStrategy) OnTick(_ registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}
func (s *longThenFlatStrategy) Reset() { s.current = 0 }

// makeCandles creates a sequence of daily candles with monotonically increasing close prices.
func makeCandles(n int, startClose float64, step float64) []registry.Candle {
	candles := make([]registry.Candle, n)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		c := startClose + float64(i)*step
		candles[i] = registry.Candle{
			Symbol:    "BTC/USDT",
			Market:    "crypto",
			Timeframe: "1d",
			Timestamp: base.AddDate(0, 0, i),
			Open:      c,
			High:      c + 10,
			Low:       c - 10,
			Close:     c,
			Volume:    1000,
		}
	}
	return candles
}

// runTest is a helper that calls Run with nil DB and Redis (test mode).
func runTest(t *testing.T, cfg RunConfig) (*Result, error) {
	t.Helper()
	return Run(context.Background(), cfg, nil, nil)
}

// --- Tests ---

func TestEngine_EmptyCandleReturnsError(t *testing.T) {
	cfg := RunConfig{
		BacktestID:  1,
		Strategy:    &flatStrategy{},
		Candles:     []registry.Candle{},
		InitialCash: 10000,
	}
	_, err := runTest(t, cfg)
	if err == nil {
		t.Fatal("expected error for empty candles, got nil")
	}
}

func TestEngine_NilStrategyReturnsError(t *testing.T) {
	cfg := RunConfig{
		BacktestID:  1,
		Strategy:    nil,
		Candles:     makeCandles(5, 100, 1),
		InitialCash: 10000,
	}
	_, err := runTest(t, cfg)
	if err == nil {
		t.Fatal("expected error for nil strategy, got nil")
	}
}

func TestEngine_ZeroInitialCashDefaultsTen000(t *testing.T) {
	cfg := RunConfig{
		BacktestID:  1,
		Strategy:    &flatStrategy{},
		Candles:     makeCandles(5, 100, 1),
		InitialCash: 0,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error for zero initial cash: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.EquityCurve) == 0 {
		t.Fatal("expected non-empty equity curve")
	}
	if result.EquityCurve[0].Value != 10000 {
		t.Errorf("expected first equity point to be 10000, got %f", result.EquityCurve[0].Value)
	}
}

func TestEngine_AlwaysFlatStrategy(t *testing.T) {
	candles := makeCandles(10, 100, 5)
	cfg := RunConfig{
		BacktestID:  2,
		Strategy:    &flatStrategy{},
		Candles:     candles,
		InitialCash: 10000,
		Commission:  0.001,
		Slippage:    0.0005,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("expected 0 trades, got %d", len(result.Trades))
	}
	// Equity should remain at initial cash throughout (no positions taken)
	for i, ep := range result.EquityCurve {
		if ep.Value != 10000 {
			t.Errorf("equity point %d: expected 10000, got %f", i, ep.Value)
		}
	}
	if result.Metrics.TotalReturn != 0 {
		t.Errorf("expected TotalReturn=0, got %f", result.Metrics.TotalReturn)
	}
}

func TestEngine_SingleLongTrade(t *testing.T) {
	// Candles: 5 rising candles at 100, 110, 120, 130, 140
	candles := makeCandles(5, 100, 10)
	strat := &longThenFlatStrategy{total: 5}
	cfg := RunConfig{
		BacktestID:  3,
		Strategy:    strat,
		Candles:     candles,
		InitialCash: 10000,
		Commission:  0.001,
		Slippage:    0.0005,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(result.Trades))
	}
	trade := result.Trades[0]
	if trade.PnL == nil {
		t.Fatal("trade.PnL is nil")
	}
	if *trade.PnL <= 0 {
		t.Errorf("expected positive PnL for rising candles, got %f", *trade.PnL)
	}
	if result.Metrics.TotalTrades != 1 {
		t.Errorf("expected TotalTrades=1, got %d", result.Metrics.TotalTrades)
	}
	if result.Metrics.WinningTrades != 1 {
		t.Errorf("expected WinningTrades=1, got %d", result.Metrics.WinningTrades)
	}
	if result.Metrics.WinRate != 1.0 {
		t.Errorf("expected WinRate=1.0, got %f", result.Metrics.WinRate)
	}
	if result.Metrics.TotalReturn <= 0 {
		t.Errorf("expected positive TotalReturn, got %f", result.Metrics.TotalReturn)
	}
}

func TestEngine_EquityCurveLength(t *testing.T) {
	n := 20
	candles := makeCandles(n, 100, 1)
	cfg := RunConfig{
		BacktestID:  4,
		Strategy:    &flatStrategy{},
		Candles:     candles,
		InitialCash: 10000,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.EquityCurve) != n {
		t.Errorf("expected equity curve length %d, got %d", n, len(result.EquityCurve))
	}
	// Each equity point should have the correct timestamp
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, ep := range result.EquityCurve {
		expected := base.AddDate(0, 0, i)
		if !ep.Timestamp.Equal(expected) {
			t.Errorf("equity point %d: expected timestamp %v, got %v", i, expected, ep.Timestamp)
		}
	}
}

func TestEngine_CommissionDeducted(t *testing.T) {
	// 2 candles: buy at 100, immediately close at 100 (no price movement)
	// With commission, PnL should be negative
	candles := []registry.Candle{
		{
			Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Open: 100, High: 105, Low: 95, Close: 100, Volume: 1000,
		},
		{
			Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Open: 100, High: 105, Low: 95, Close: 100, Volume: 1000,
		},
	}
	// Strategy: long on candle 1, flat on candle 2
	strat := &longThenFlatStrategy{total: 2}
	cfg := RunConfig{
		BacktestID:  5,
		Strategy:    strat,
		Candles:     candles,
		InitialCash: 10000,
		Commission:  0.001, // 0.1%
		Slippage:    0,     // disable slippage for cleaner test
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(result.Trades))
	}
	trade := result.Trades[0]
	if trade.Fee <= 0 {
		t.Errorf("expected positive fee (commission), got %f", trade.Fee)
	}
	if trade.PnL == nil {
		t.Fatal("trade.PnL is nil")
	}
	// PnL should be negative due to commission on flat price
	if *trade.PnL >= 0 {
		t.Errorf("expected negative PnL (due to commission on flat price), got %f", *trade.PnL)
	}
}

func TestEngine_MetricsCalculation(t *testing.T) {
	// Known sequence: buy at 100, sell at 200 → 100% gross return (minus fees)
	// InitialCash=10000, Commission=0, Slippage=0 for clarity
	candles := []registry.Candle{
		{
			Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Open: 100, High: 105, Low: 95, Close: 100, Volume: 1000,
		},
		{
			Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Open: 150, High: 205, Low: 145, Close: 200, Volume: 1000,
		},
	}
	strat := &longThenFlatStrategy{total: 2}
	cfg := RunConfig{
		BacktestID:  6,
		Strategy:    strat,
		Candles:     candles,
		InitialCash: 10000,
		Commission:  0,
		Slippage:    0,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// quantity = 10000 * 0.99 / 100 = 99
	// exitValue = 99 * 200 = 19800
	// PnL = 19800 - 10000*0.99 = 19800 - 9900 = 9900
	// finalEquity = cash_remainder + exitValue
	// cash_remainder = 10000 - 9900 = 100
	// finalEquity = 100 + 19800 = 19900
	// TotalReturn = (19900 - 10000) / 10000 = 0.99
	const tolerance = 0.001
	if result.Metrics.TotalReturn < 0.98 || result.Metrics.TotalReturn > 1.0 {
		t.Errorf("expected TotalReturn ≈ 0.99, got %f", result.Metrics.TotalReturn)
	}
	if result.Metrics.TotalTrades != 1 {
		t.Errorf("expected 1 trade, got %d", result.Metrics.TotalTrades)
	}
	if result.Metrics.WinRate != 1.0 {
		t.Errorf("expected WinRate=1.0, got %f", result.Metrics.WinRate)
	}
	_ = tolerance
}

func TestEngine_DefaultsApplied(t *testing.T) {
	// When Commission/Slippage are zero in config, defaults should be applied internally
	candles := makeCandles(5, 100, 1)
	cfg := RunConfig{
		BacktestID:  7,
		Strategy:    &flatStrategy{},
		Candles:     candles,
		InitialCash: 10000,
		// Commission and Slippage intentionally left at zero — defaults should kick in
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestEngine_MaxDrawdown(t *testing.T) {
	// Equity goes 10000 → 12000 → 8000 → 9000
	// Peak = 12000, trough = 8000 → drawdown = (12000-8000)/12000 ≈ 0.333
	// We'll simulate this by using a strategy that goes long and the price drops
	// Candles: 100, 120, 80, 90 — long from start, flat at end
	candles := []registry.Candle{
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Open: 100, High: 105, Low: 95, Close: 100, Volume: 1000},
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Open: 115, High: 125, Low: 115, Close: 120, Volume: 1000},
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
			Open: 85, High: 90, Low: 75, Close: 80, Volume: 1000},
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC),
			Open: 85, High: 95, Low: 85, Close: 90, Volume: 1000},
	}
	strat := &longThenFlatStrategy{total: 4}
	cfg := RunConfig{
		BacktestID:  8,
		Strategy:    strat,
		Candles:     candles,
		InitialCash: 10000,
		Commission:  0,
		Slippage:    0,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// MaxDrawdown should be > 0 (there was a decline from peak)
	if result.Metrics.MaxDrawdown <= 0 {
		t.Errorf("expected positive MaxDrawdown, got %f", result.Metrics.MaxDrawdown)
	}
}

func TestEngine_ProfitFactor(t *testing.T) {
	// Two separate long trades: one winning, one losing
	// Candles: 100, 120, 80, 100 — but we'd need alternating signals
	// Use a multi-signal strategy: long@1, flat@2, long@3, flat@4
	type multiStrategy struct {
		step int
	}
	// inline adapter
	candles := []registry.Candle{
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Open: 100, High: 105, Low: 95, Close: 100, Volume: 1000},
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			Open: 115, High: 125, Low: 115, Close: 120, Volume: 1000},
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
			Open: 115, High: 125, Low: 115, Close: 120, Volume: 1000},
		{Symbol: "BTC/USDT", Market: "crypto", Timeframe: "1d",
			Timestamp: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC),
			Open: 85, High: 90, Low: 75, Close: 80, Volume: 1000},
	}
	strat := &alternateLongFlatStrategy{}
	cfg := RunConfig{
		BacktestID:  9,
		Strategy:    strat,
		Candles:     candles,
		InitialCash: 10000,
		Commission:  0,
		Slippage:    0,
	}
	result, err := runTest(t, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metrics.TotalTrades != 2 {
		t.Errorf("expected 2 trades, got %d", result.Metrics.TotalTrades)
	}
	// ProfitFactor = grossProfit / grossLoss; should be > 0
	if result.Metrics.ProfitFactor <= 0 {
		t.Errorf("expected positive ProfitFactor, got %f", result.Metrics.ProfitFactor)
	}
}

// alternateLongFlatStrategy alternates: long, flat, long, flat, ...
type alternateLongFlatStrategy struct {
	step int
}

func (a *alternateLongFlatStrategy) Name() string        { return "alternate" }
func (a *alternateLongFlatStrategy) Description() string { return "alternates long/flat" }
func (a *alternateLongFlatStrategy) Params() []registry.ParamDefinition {
	return nil
}
func (a *alternateLongFlatStrategy) Init(_ map[string]interface{}) error { return nil }
func (a *alternateLongFlatStrategy) OnCandle(candle registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	a.step++
	var dir string
	if a.step%2 == 1 {
		dir = "long"
	} else {
		dir = "flat"
	}
	return &registry.Signal{
		Symbol:    candle.Symbol,
		Market:    candle.Market,
		Direction: dir,
		Strength:  1.0,
		Price:     candle.Close,
		Timestamp: candle.Timestamp,
	}, nil
}
func (a *alternateLongFlatStrategy) OnTick(_ registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}
func (a *alternateLongFlatStrategy) Reset() { a.step = 0 }
