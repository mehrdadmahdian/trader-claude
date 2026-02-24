package strategy

import (
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

// makeCandle is a helper to build a registry.Candle with a given close price.
func makeCandle(close float64, ts time.Time) registry.Candle {
	return registry.Candle{
		Symbol:    "BTC/USDT",
		Market:    "crypto",
		Timeframe: "1h",
		Timestamp: ts,
		Open:      close,
		High:      close,
		Low:       close,
		Close:     close,
		Volume:    1000,
	}
}

// makeState is a helper to build an empty StrategyState.
func makeState() *registry.StrategyState {
	return &registry.StrategyState{
		StrategyID: "test-strategy",
		Symbol:     "BTC/USDT",
		Market:     "crypto",
		State:      make(map[string]interface{}),
		UpdatedAt:  time.Now(),
	}
}

// TestEMACrossover_NoSignalUntilWarmup verifies that no signal is returned
// until at least slow_period candles have been processed.
func TestEMACrossover_NoSignalUntilWarmup(t *testing.T) {
	s := &EMACrossover{}
	err := s.Init(map[string]interface{}{
		"fast_period": 3,
		"slow_period": 5,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Feed fewer than slow_period candles — all must return nil signal.
	for i := 0; i < 4; i++ {
		sig, err := s.OnCandle(makeCandle(100+float64(i), base.Add(time.Duration(i)*time.Hour)), state)
		if err != nil {
			t.Fatalf("OnCandle error at i=%d: %v", i, err)
		}
		if sig != nil {
			t.Errorf("expected nil signal at candle %d (warmup), got %+v", i, sig)
		}
	}
}

// TestEMACrossover_LongSignalOnCross verifies that a long signal is produced
// when the fast EMA crosses above the slow EMA.
func TestEMACrossover_LongSignalOnCross(t *testing.T) {
	s := &EMACrossover{}
	err := s.Init(map[string]interface{}{
		"fast_period": 3,
		"slow_period": 5,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Feed low prices first to warm up with a bearish bias.
	lowPrices := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
	for i, p := range lowPrices {
		_, _ = s.OnCandle(makeCandle(p, base.Add(time.Duration(i)*time.Hour)), state)
	}

	// Now feed high prices to force fast EMA above slow EMA.
	highPrices := []float64{100, 200, 300}
	var lastSig *registry.Signal
	for i, p := range highPrices {
		ts := base.Add(time.Duration(len(lowPrices)+i) * time.Hour)
		sig, err := s.OnCandle(makeCandle(p, ts), state)
		if err != nil {
			t.Fatalf("OnCandle error: %v", err)
		}
		if sig != nil {
			lastSig = sig
		}
	}

	if lastSig == nil {
		t.Fatal("expected a long signal after upward EMA cross, got nil")
	}
	if lastSig.Direction != "long" {
		t.Errorf("expected direction=long, got %q", lastSig.Direction)
	}
	if lastSig.Strength <= 0 || lastSig.Strength > 1 {
		t.Errorf("expected strength in (0,1], got %f", lastSig.Strength)
	}
}

// TestEMACrossover_ShortSignalOnCross verifies that a short signal is produced
// when the fast EMA crosses below the slow EMA.
func TestEMACrossover_ShortSignalOnCross(t *testing.T) {
	s := &EMACrossover{}
	err := s.Init(map[string]interface{}{
		"fast_period": 3,
		"slow_period": 5,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Feed high prices first to create a bullish EMA state.
	highPrices := []float64{200, 200, 200, 200, 200, 200, 200, 200, 200, 200}
	for i, p := range highPrices {
		_, _ = s.OnCandle(makeCandle(p, base.Add(time.Duration(i)*time.Hour)), state)
	}

	// Now feed low prices to force fast EMA below slow EMA.
	lowPrices := []float64{1, 1, 1}
	var lastSig *registry.Signal
	for i, p := range lowPrices {
		ts := base.Add(time.Duration(len(highPrices)+i) * time.Hour)
		sig, err := s.OnCandle(makeCandle(p, ts), state)
		if err != nil {
			t.Fatalf("OnCandle error: %v", err)
		}
		if sig != nil {
			lastSig = sig
		}
	}

	if lastSig == nil {
		t.Fatal("expected a short signal after downward EMA cross, got nil")
	}
	if lastSig.Direction != "short" {
		t.Errorf("expected direction=short, got %q", lastSig.Direction)
	}
}

// TestEMACrossover_DefaultParams verifies that Init with an empty param map
// uses the documented defaults without returning an error.
func TestEMACrossover_DefaultParams(t *testing.T) {
	s := &EMACrossover{}
	err := s.Init(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Init with empty params failed: %v", err)
	}

	if s.fastPeriod != 9 {
		t.Errorf("expected default fast_period=9, got %d", s.fastPeriod)
	}
	if s.slowPeriod != 21 {
		t.Errorf("expected default slow_period=21, got %d", s.slowPeriod)
	}
	if !s.signalOnClose {
		t.Errorf("expected default signal_on_close=true")
	}
}

// TestEMACrossover_Reset verifies that calling Reset clears internal state so
// that a subsequent run produces the same results as a fresh instance.
func TestEMACrossover_Reset(t *testing.T) {
	s := &EMACrossover{}
	_ = s.Init(map[string]interface{}{"fast_period": 3, "slow_period": 5})

	state := makeState()
	base := time.Now()

	// Run for several candles.
	for i := 0; i < 10; i++ {
		_, _ = s.OnCandle(makeCandle(float64(100+i), base.Add(time.Duration(i)*time.Hour)), state)
	}

	s.Reset()

	// After reset, the warmup counter must restart: first candle should return nil.
	sig, err := s.OnCandle(makeCandle(500, base.Add(20*time.Hour)), state)
	if err != nil {
		t.Fatalf("OnCandle after Reset error: %v", err)
	}
	if sig != nil {
		t.Errorf("expected nil signal immediately after Reset, got %+v", sig)
	}

	// Internal buffers must be cleared.
	if len(s.closes) != 1 {
		t.Errorf("expected 1 close price after Reset+1 candle, got %d", len(s.closes))
	}
}

// TestEMACrossover_OnTick verifies that OnTick always returns nil, nil.
func TestEMACrossover_OnTick(t *testing.T) {
	s := &EMACrossover{}
	_ = s.Init(map[string]interface{}{})

	state := makeState()
	tick := registry.Tick{
		Symbol:    "BTC/USDT",
		Market:    "crypto",
		Price:     50000,
		Timestamp: time.Now(),
	}

	sig, err := s.OnTick(tick, state)
	if err != nil {
		t.Fatalf("OnTick error: %v", err)
	}
	if sig != nil {
		t.Errorf("expected nil signal from OnTick, got %+v", sig)
	}
}
