package strategy

import (
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

// TestMACD_NoSignalDuringWarmup verifies that no signal is produced until
// enough candles have been fed to compute MACD + signal line.
// Warmup = slow + signal - 1 candles minimum.
func TestMACD_NoSignalDuringWarmup(t *testing.T) {
	s := &MACDSignal{}
	err := s.Init(map[string]interface{}{
		"fast":   12,
		"slow":   26,
		"signal": 9,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Feed slow-1 candles; all must return nil.
	for i := 0; i < 25; i++ {
		sig, err := s.OnCandle(makeCandle(100+float64(i), base.Add(time.Duration(i)*time.Hour)), state)
		if err != nil {
			t.Fatalf("OnCandle error at i=%d: %v", i, err)
		}
		if sig != nil {
			t.Errorf("expected nil signal during warmup at candle %d, got %+v", i, sig)
		}
	}
}

// TestMACD_BullishCross verifies that a long signal is produced when the MACD
// line crosses above the signal line.
func TestMACD_BullishCross(t *testing.T) {
	s := &MACDSignal{}
	err := s.Init(map[string]interface{}{
		"fast":                12,
		"slow":                26,
		"signal":              9,
		"histogram_threshold": 0.0,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Establish bearish history so MACD starts below signal.
	for i := 0; i < 50; i++ {
		price := 200.0 - float64(i)*0.5 // slowly declining
		_, _ = s.OnCandle(makeCandle(price, base.Add(time.Duration(i)*time.Hour)), state)
	}

	// Reverse with strong upward move to drive MACD above signal.
	var longSig *registry.Signal
	for i := 0; i < 30; i++ {
		price := 175.0 + float64(i)*5
		ts := base.Add(time.Duration(50+i) * time.Hour)
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
		t.Fatal("expected a long signal on MACD bullish cross, got nil")
	}
	if longSig.Direction != "long" {
		t.Errorf("expected direction=long, got %q", longSig.Direction)
	}
}

// TestMACD_BearishCross verifies that a short signal is produced when the MACD
// line crosses below the signal line.
func TestMACD_BearishCross(t *testing.T) {
	s := &MACDSignal{}
	err := s.Init(map[string]interface{}{
		"fast":                12,
		"slow":                26,
		"signal":              9,
		"histogram_threshold": 0.0,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Establish bullish history.
	for i := 0; i < 50; i++ {
		price := 100.0 + float64(i)*0.5 // slowly rising
		_, _ = s.OnCandle(makeCandle(price, base.Add(time.Duration(i)*time.Hour)), state)
	}

	// Reverse with strong downward move to drive MACD below signal.
	var shortSig *registry.Signal
	for i := 0; i < 30; i++ {
		price := 125.0 - float64(i)*5
		ts := base.Add(time.Duration(50+i) * time.Hour)
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
		t.Fatal("expected a short signal on MACD bearish cross, got nil")
	}
	if shortSig.Direction != "short" {
		t.Errorf("expected direction=short, got %q", shortSig.Direction)
	}
}

// TestMACD_DefaultParams verifies that Init with empty params uses documented
// defaults without error.
func TestMACD_DefaultParams(t *testing.T) {
	s := &MACDSignal{}
	err := s.Init(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Init with empty params failed: %v", err)
	}

	if s.fastPeriod != 12 {
		t.Errorf("expected default fast=12, got %d", s.fastPeriod)
	}
	if s.slowPeriod != 26 {
		t.Errorf("expected default slow=26, got %d", s.slowPeriod)
	}
	if s.signalPeriod != 9 {
		t.Errorf("expected default signal=9, got %d", s.signalPeriod)
	}
	if s.histogramThreshold != 0.0 {
		t.Errorf("expected default histogram_threshold=0, got %f", s.histogramThreshold)
	}
}

// TestMACD_OnTick verifies that OnTick always returns nil, nil.
func TestMACD_OnTick(t *testing.T) {
	s := &MACDSignal{}
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

// TestMACD_Reset verifies that Reset clears internal state.
func TestMACD_Reset(t *testing.T) {
	s := &MACDSignal{}
	_ = s.Init(map[string]interface{}{"fast": 12, "slow": 26, "signal": 9})

	state := makeState()
	base := time.Now()

	for i := 0; i < 40; i++ {
		_, _ = s.OnCandle(makeCandle(float64(100+i), base.Add(time.Duration(i)*time.Hour)), state)
	}

	s.Reset()

	// After reset, warmup restarts — first candle must return nil.
	sig, err := s.OnCandle(makeCandle(200, base.Add(50*time.Hour)), state)
	if err != nil {
		t.Fatalf("OnCandle after Reset error: %v", err)
	}
	if sig != nil {
		t.Errorf("expected nil signal immediately after Reset, got %+v", sig)
	}
}

// TestMACD_HistogramThreshold verifies that signals are suppressed when the
// absolute histogram value is at or below the threshold.
func TestMACD_HistogramThreshold(t *testing.T) {
	s := &MACDSignal{}
	// Set a very high threshold so signals should be suppressed.
	err := s.Init(map[string]interface{}{
		"fast":                12,
		"slow":                26,
		"signal":              9,
		"histogram_threshold": 1000.0,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	state := makeState()
	base := time.Now()

	// Normal data — with threshold=1000 no signal should fire.
	var anySig *registry.Signal
	for i := 0; i < 100; i++ {
		price := 100.0 + float64(i%10) // mild oscillation
		sig, err := s.OnCandle(makeCandle(price, base.Add(time.Duration(i)*time.Hour)), state)
		if err != nil {
			t.Fatalf("OnCandle error: %v", err)
		}
		if sig != nil {
			anySig = sig
		}
	}

	if anySig != nil {
		t.Errorf("expected no signal with histogram_threshold=1000, got %+v", anySig)
	}
}
