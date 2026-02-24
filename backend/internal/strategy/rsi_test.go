package strategy

import (
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

// TestRSI_NoSignalDuringWarmup verifies that no signal is emitted until
// at least period+1 candles have been processed (Wilder warmup).
//
// Warmup boundary in rsi.go:
//   - warmup==1 (candle index 0):          stores prevClose; returns nil
//   - warmup 2..period (indices 1..13):    accumulates gains/losses; returns nil
//   - warmup==period+1 (index 14):         seeds initial SMA averages; still returns nil
//   - warmup>=period+2 (index 15+):        Wilder smoothing active; signals possible
//
// This test covers indices 0..14 (warmup 1..15) — all must return nil.
func TestRSI_NoSignalDuringWarmup(t *testing.T) {
	s := &RSIStrategy{}
	err := s.Init(map[string]interface{}{
		"period": 14,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Feed period candles (indices 0..13, warmup 1..14) — none should produce a signal.
	for i := 0; i < 14; i++ {
		sig, err := s.OnCandle(makeCandle(100+float64(i), base.Add(time.Duration(i)*time.Hour)), state)
		if err != nil {
			t.Fatalf("OnCandle error at i=%d: %v", i, err)
		}
		if sig != nil {
			t.Errorf("expected nil signal during warmup at candle %d, got %+v", i, sig)
		}
	}

	// Feed the boundary candle (index 14, warmup==period+1==15).
	// This candle completes the initial SMA seed but must still return nil —
	// no previous RSI exists to compare against for a crossover.
	boundarySig, err := s.OnCandle(makeCandle(100+14, base.Add(14*time.Hour)), state)
	if err != nil {
		t.Fatalf("OnCandle error at boundary candle (index 14): %v", err)
	}
	if boundarySig != nil {
		t.Errorf("expected nil signal at boundary candle (index 14, warmup=period+1), got %+v", boundarySig)
	}

	// Verify the strategy has warmed up: after period+1 candles the seeded flag must be true.
	if !s.seeded {
		t.Error("expected s.seeded==true after period+1 candles, got false")
	}
}

// TestRSI_OversoldLongSignal verifies that a long signal is generated when RSI
// crosses below the oversold threshold.
func TestRSI_OversoldLongSignal(t *testing.T) {
	s := &RSIStrategy{}
	err := s.Init(map[string]interface{}{
		"period":    14,
		"oversold":  30.0,
		"overbought": 70.0,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Build up some neutral history.
	for i := 0; i < 20; i++ {
		_, _ = s.OnCandle(makeCandle(100, base.Add(time.Duration(i)*time.Hour)), state)
	}

	// Drive RSI sharply downward: sequence of large drops.
	// Many consecutive down candles push RSI toward 0.
	var longSig *registry.Signal
	for i := 0; i < 20; i++ {
		price := 100 - float64(i)*4 // 100, 96, 92, …
		ts := base.Add(time.Duration(20+i) * time.Hour)
		sig, err := s.OnCandle(makeCandle(price, ts), state)
		if err != nil {
			t.Fatalf("OnCandle error: %v", err)
		}
		if sig != nil && sig.Direction == "long" {
			longSig = sig
			break
		}
	}

	if longSig == nil {
		t.Fatal("expected a long signal when RSI is oversold, got nil")
	}
	if longSig.Direction != "long" {
		t.Errorf("expected direction=long, got %q", longSig.Direction)
	}
}

// TestRSI_OverboughtShortSignal verifies that a short signal is generated when
// RSI crosses above the overbought threshold.
func TestRSI_OverboughtShortSignal(t *testing.T) {
	s := &RSIStrategy{}
	err := s.Init(map[string]interface{}{
		"period":    14,
		"oversold":  30.0,
		"overbought": 70.0,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Build up neutral history.
	for i := 0; i < 20; i++ {
		_, _ = s.OnCandle(makeCandle(100, base.Add(time.Duration(i)*time.Hour)), state)
	}

	// Drive RSI sharply upward: many consecutive large gains.
	var shortSig *registry.Signal
	for i := 0; i < 20; i++ {
		price := 100 + float64(i)*4 // 100, 104, 108, …
		ts := base.Add(time.Duration(20+i) * time.Hour)
		sig, err := s.OnCandle(makeCandle(price, ts), state)
		if err != nil {
			t.Fatalf("OnCandle error: %v", err)
		}
		if sig != nil && sig.Direction == "short" {
			shortSig = sig
			break
		}
	}

	if shortSig == nil {
		t.Fatal("expected a short signal when RSI is overbought, got nil")
	}
	if shortSig.Direction != "short" {
		t.Errorf("expected direction=short, got %q", shortSig.Direction)
	}
}

// TestRSI_DefaultParams verifies that Init with an empty param map
// uses the documented defaults without error.
func TestRSI_DefaultParams(t *testing.T) {
	s := &RSIStrategy{}
	err := s.Init(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Init with empty params failed: %v", err)
	}

	if s.period != 14 {
		t.Errorf("expected default period=14, got %d", s.period)
	}
	if s.overbought != 70.0 {
		t.Errorf("expected default overbought=70, got %f", s.overbought)
	}
	if s.oversold != 30.0 {
		t.Errorf("expected default oversold=30, got %f", s.oversold)
	}
}

// TestRSI_OnTick verifies that OnTick always returns nil, nil.
func TestRSI_OnTick(t *testing.T) {
	s := &RSIStrategy{}
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

// TestRSI_Reset verifies that Reset clears all internal state.
func TestRSI_Reset(t *testing.T) {
	s := &RSIStrategy{}
	_ = s.Init(map[string]interface{}{"period": 14})

	state := makeState()
	base := time.Now()

	for i := 0; i < 20; i++ {
		_, _ = s.OnCandle(makeCandle(float64(100+i), base.Add(time.Duration(i)*time.Hour)), state)
	}

	s.Reset()

	// After reset the warmup should restart — no signal on first candle.
	sig, err := s.OnCandle(makeCandle(200, base.Add(30*time.Hour)), state)
	if err != nil {
		t.Fatalf("OnCandle after Reset error: %v", err)
	}
	if sig != nil {
		t.Errorf("expected nil signal immediately after Reset, got %+v", sig)
	}
}
