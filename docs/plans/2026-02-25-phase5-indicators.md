# Phase 5 — Technical Indicators Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 13 technical indicators (7 overlay + 6 panel) with a stateless backend calc engine, a REST API, and a full frontend (modal, chips, overlay/panel chart rendering).

**Architecture:** Pure Go calc package (`internal/indicator/`) accepts `[]registry.Candle`, returns named parallel float arrays. Two backend files (overlay/panel) are written in parallel worktrees then merged. The frontend POSTs already-loaded candles to `/api/v1/indicators/calculate` and renders results directly onto the lightweight-charts instance.

**Tech Stack:** Go 1.24 (math, no external deps), Fiber v2, lightweight-charts v4, React Query v5, Zustand, Tailwind, shadcn/ui, lucide-react.

**Design doc:** `docs/plans/2026-02-25-phase5-indicators-design.md`

---

## EXECUTION STRUCTURE

```
Pre-step  (this session): Task 0 — write types.go

Wave 1    (TWO parallel worktrees — run simultaneously):
  Worktree A:  Tasks A1, A2, A3  →  overlay.go + overlay_test.go
  Worktree B:  Tasks B1, B2, B3  →  panel.go   + panel_test.go

Wave 2    (main branch after merge, sequential):
  Tasks 1–9   →  registry, handler, routes, API tests, full frontend
```

After Wave 1 is dispatched: check both worktrees pass `go test ./internal/indicator/... -v`, then merge into main branch before starting Wave 2.

---

## Task 0 — Shared Types (`types.go`)

**Files:**
- Create: `backend/internal/indicator/types.go`

Write this file exactly before dispatching Wave 1 agents. Both worktrees will pick it up automatically.

```go
// Package indicator provides stateless technical indicator calculations.
// All functions accept []registry.Candle and return a CalcResult.
package indicator

import "github.com/trader-claude/backend/internal/registry"

// CalcFunc is the common signature for every indicator function.
type CalcFunc func(candles []registry.Candle, params map[string]interface{}) (CalcResult, error)

// CalcResult holds parallel arrays of timestamps and named output series.
// NaN values represent the warm-up period before the indicator stabilises.
// len(Timestamps) == len(each series slice).
type CalcResult struct {
	Timestamps []int64              `json:"timestamps"`
	Series     map[string][]float64 `json:"series"`
}

// IndicatorMeta describes one indicator for the catalog endpoint.
type IndicatorMeta struct {
	ID       string                    `json:"id"`
	Name     string                    `json:"name"`
	FullName string                    `json:"full_name"`
	Type     string                    `json:"type"`    // "overlay" | "panel"
	Group    string                    `json:"group"`   // "trend" | "momentum" | "volatility" | "volume"
	Params   []registry.ParamDefinition `json:"params"`
	Outputs  []OutputDef               `json:"outputs"`
}

// OutputDef describes one named series within a CalcResult.
type OutputDef struct {
	Name  string `json:"name"`
	Color string `json:"color"` // default hex colour, e.g. "#2962FF"
}
```

**Commit:**
```bash
git add backend/internal/indicator/types.go
git commit -m "feat(phase5): add indicator package with shared types"
```

---

## WAVE 1 — WORKTREE A: Overlay Indicators

> Worktree branch: `phase5/overlay`
> Only touch: `backend/internal/indicator/overlay.go` and `backend/internal/indicator/overlay_test.go`

---

### Task A1 — SMA, EMA, WMA

**Files:**
- Create: `backend/internal/indicator/overlay.go`
- Create: `backend/internal/indicator/overlay_test.go`

**Step 1: Write the failing tests**

```go
// overlay_test.go
package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

// makeCandles builds a minimal candle slice from close prices.
// Timestamp starts at Unix 0, increments by 3600 per candle.
func makeCandles(closes []float64) []registry.Candle {
	out := make([]registry.Candle, len(closes))
	for i, c := range closes {
		out[i] = registry.Candle{
			Timestamp: time.Unix(int64(i)*3600, 0),
			Open: c, High: c, Low: c, Close: c, Volume: 1000,
		}
	}
	return out
}

func approxEq(a, b, tol float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Abs(a-b) < tol
}

func TestSMA(t *testing.T) {
	candles := makeCandles([]float64{1, 2, 3, 4, 5})
	res, err := SMA(candles, map[string]interface{}{"period": 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []float64{math.NaN(), math.NaN(), 2.0, 3.0, 4.0}
	if len(res.Series["value"]) != len(want) {
		t.Fatalf("len mismatch: got %d, want %d", len(res.Series["value"]), len(want))
	}
	for i, w := range want {
		if !approxEq(res.Series["value"][i], w, 0.001) {
			t.Errorf("[%d] got %f, want %f", i, res.Series["value"][i], w)
		}
	}
}

func TestEMA(t *testing.T) {
	// k = 2/(3+1) = 0.5; seed = SMA([1,2,3]) = 2.0
	// EMA[2]=2.0, EMA[3]=4*0.5+2.0*0.5=3.0, EMA[4]=5*0.5+3.0*0.5=4.0
	candles := makeCandles([]float64{1, 2, 3, 4, 5})
	res, err := EMA(candles, map[string]interface{}{"period": 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []float64{math.NaN(), math.NaN(), 2.0, 3.0, 4.0}
	for i, w := range want {
		if !approxEq(res.Series["value"][i], w, 0.001) {
			t.Errorf("EMA[%d] got %f, want %f", i, res.Series["value"][i], w)
		}
	}
}

func TestWMA(t *testing.T) {
	// WMA(3) on [1,2,3]: weights [1,2,3], denom=6
	// WMA[2] = (1*1 + 2*2 + 3*3)/6 = 14/6 ≈ 2.333
	// WMA[3] = (1*2 + 2*3 + 3*4)/6 = 20/6 ≈ 3.333
	candles := makeCandles([]float64{1, 2, 3, 4, 5})
	res, err := WMA(candles, map[string]interface{}{"period": 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsNaN(res.Series["value"][0]) || !math.IsNaN(res.Series["value"][1]) {
		t.Error("expected NaN for warm-up period")
	}
	if !approxEq(res.Series["value"][2], 14.0/6.0, 0.001) {
		t.Errorf("WMA[2] got %f, want %f", res.Series["value"][2], 14.0/6.0)
	}
	if !approxEq(res.Series["value"][3], 20.0/6.0, 0.001) {
		t.Errorf("WMA[3] got %f, want %f", res.Series["value"][3], 20.0/6.0)
	}
}
```

**Step 2: Run tests — expect compile failure**
```bash
cd backend && go test ./internal/indicator/... -run "TestSMA|TestEMA|TestWMA" -v 2>&1 | head -20
```
Expected: `undefined: SMA` (or similar compile error).

**Step 3: Implement SMA, EMA, WMA**

```go
// overlay.go
package indicator

import (
	"fmt"
	"math"

	"github.com/trader-claude/backend/internal/registry"
)

// param helpers

func intParam(params map[string]interface{}, key string, def int) (int, error) {
	v, ok := params[key]
	if !ok {
		return def, nil
	}
	switch x := v.(type) {
	case int:
		return x, nil
	case float64:
		return int(x), nil
	case int64:
		return int(x), nil
	}
	return def, fmt.Errorf("param %q must be numeric, got %T", key, v)
}

func floatParam(params map[string]interface{}, key string, def float64) (float64, error) {
	v, ok := params[key]
	if !ok {
		return def, nil
	}
	switch x := v.(type) {
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	}
	return def, fmt.Errorf("param %q must be numeric, got %T", key, v)
}

func timestamps(candles []registry.Candle) []int64 {
	ts := make([]int64, len(candles))
	for i, c := range candles {
		ts[i] = c.Timestamp.Unix()
	}
	return ts
}

func nanSlice(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = math.NaN()
	}
	return s
}

// SMA computes Simple Moving Average.
// params: period int (default 20)
func SMA(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 20)
	if err != nil {
		return CalcResult{}, err
	}
	if period < 1 {
		return CalcResult{}, fmt.Errorf("period must be >= 1")
	}
	n := len(candles)
	vals := nanSlice(n)
	if n < period {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}
	// seed: sum of first window
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	vals[period-1] = sum / float64(period)
	for i := period; i < n; i++ {
		sum += candles[i].Close - candles[i-period].Close
		vals[i] = sum / float64(period)
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}

// EMA computes Exponential Moving Average.
// params: period int (default 20)
// Seed = SMA of first period candles; then EMA = price*k + prevEMA*(1-k), k=2/(period+1)
func EMA(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 20)
	if err != nil {
		return CalcResult{}, err
	}
	if period < 1 {
		return CalcResult{}, fmt.Errorf("period must be >= 1")
	}
	n := len(candles)
	vals := nanSlice(n)
	if n < period {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}
	k := 2.0 / float64(period+1)
	// seed
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	prev := sum / float64(period)
	vals[period-1] = prev
	for i := period; i < n; i++ {
		prev = candles[i].Close*k + prev*(1-k)
		vals[i] = prev
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}

// WMA computes Weighted Moving Average (linearly weighted).
// params: period int (default 20)
func WMA(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 20)
	if err != nil {
		return CalcResult{}, err
	}
	if period < 1 {
		return CalcResult{}, fmt.Errorf("period must be >= 1")
	}
	n := len(candles)
	vals := nanSlice(n)
	denom := float64(period*(period+1)) / 2.0
	for i := period - 1; i < n; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += candles[i-period+1+j].Close * float64(j+1)
		}
		vals[i] = sum / denom
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}
```

**Step 4: Run tests**
```bash
cd backend && go test ./internal/indicator/... -run "TestSMA|TestEMA|TestWMA" -v
```
Expected: all PASS.

**Step 5: Commit**
```bash
git add backend/internal/indicator/overlay.go backend/internal/indicator/overlay_test.go
git commit -m "feat(phase5): add SMA, EMA, WMA overlay indicators with tests"
```

---

### Task A2 — Bollinger Bands + VWAP

**Step 1: Add tests to `overlay_test.go`**

```go
func TestBollingerBands(t *testing.T) {
	// period=3, std_dev=2 on [2,2,2,2,2] → all equal closes
	// middle=2, std=0, upper=2, lower=2
	candles := makeCandles([]float64{2, 2, 2, 2, 2})
	res, err := BollingerBands(candles, map[string]interface{}{"period": 3, "std_dev": 2.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"upper", "middle", "lower"} {
		if _, ok := res.Series[k]; !ok {
			t.Fatalf("missing series %q", k)
		}
	}
	// first two NaN
	if !math.IsNaN(res.Series["middle"][0]) {
		t.Error("expected NaN at index 0")
	}
	// index 2 onwards: all equal
	for i := 2; i < 5; i++ {
		if !approxEq(res.Series["middle"][i], 2.0, 0.001) {
			t.Errorf("middle[%d]=%f", i, res.Series["middle"][i])
		}
		if !approxEq(res.Series["upper"][i], 2.0, 0.001) {
			t.Errorf("upper[%d]=%f", i, res.Series["upper"][i])
		}
		if !approxEq(res.Series["lower"][i], 2.0, 0.001) {
			t.Errorf("lower[%d]=%f", i, res.Series["lower"][i])
		}
	}
}

func TestVWAP(t *testing.T) {
	// VWAP: cumulative sum(close*vol) / sum(vol)
	// candle 0: close=10, vol=100 → vwap=10
	// candle 1: close=20, vol=100 → vwap=(10*100+20*100)/200=15
	candles := []registry.Candle{
		{Timestamp: time.Unix(0, 0), Close: 10, Volume: 100, Open: 10, High: 10, Low: 10},
		{Timestamp: time.Unix(3600, 0), Close: 20, Volume: 100, Open: 20, High: 20, Low: 20},
	}
	res, err := VWAP(candles, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approxEq(res.Series["value"][0], 10.0, 0.001) {
		t.Errorf("vwap[0] got %f, want 10.0", res.Series["value"][0])
	}
	if !approxEq(res.Series["value"][1], 15.0, 0.001) {
		t.Errorf("vwap[1] got %f, want 15.0", res.Series["value"][1])
	}
}
```

**Step 2: Run tests — expect compile failure**
```bash
cd backend && go test ./internal/indicator/... -run "TestBollinger|TestVWAP" -v 2>&1 | head -10
```

**Step 3: Implement BollingerBands + VWAP in `overlay.go`**

```go
// BollingerBands computes BB with middle (SMA), upper, and lower bands.
// params: period int (default 20), std_dev float (default 2.0)
func BollingerBands(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 20)
	if err != nil {
		return CalcResult{}, err
	}
	stdMult, err := floatParam(params, "std_dev", 2.0)
	if err != nil {
		return CalcResult{}, err
	}
	n := len(candles)
	upper := nanSlice(n)
	middle := nanSlice(n)
	lower := nanSlice(n)

	for i := period - 1; i < n; i++ {
		sum, sumSq := 0.0, 0.0
		for j := i - period + 1; j <= i; j++ {
			c := candles[j].Close
			sum += c
			sumSq += c * c
		}
		mean := sum / float64(period)
		variance := sumSq/float64(period) - mean*mean
		if variance < 0 {
			variance = 0
		}
		std := math.Sqrt(variance)
		middle[i] = mean
		upper[i] = mean + stdMult*std
		lower[i] = mean - stdMult*std
	}
	return CalcResult{
		Timestamps: timestamps(candles),
		Series:     map[string][]float64{"upper": upper, "middle": middle, "lower": lower},
	}, nil
}

// VWAP computes cumulative Volume-Weighted Average Price (no params, no warm-up).
func VWAP(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	n := len(candles)
	vals := make([]float64, n)
	cumPV, cumV := 0.0, 0.0
	for i, c := range candles {
		cumPV += c.Close * c.Volume
		cumV += c.Volume
		if cumV == 0 {
			vals[i] = math.NaN()
		} else {
			vals[i] = cumPV / cumV
		}
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}
```

**Step 4: Run tests**
```bash
cd backend && go test ./internal/indicator/... -v
```
Expected: all PASS.

**Step 5: Commit**
```bash
git add backend/internal/indicator/overlay.go backend/internal/indicator/overlay_test.go
git commit -m "feat(phase5): add Bollinger Bands and VWAP overlay indicators"
```

---

### Task A3 — Parabolic SAR + Ichimoku

**Step 1: Add tests**

```go
func TestParabolicSAR_TrendFlip(t *testing.T) {
	// Rising prices → downtrend SAR should appear above
	// We only verify: no panic, correct length, no NaN after warm-up
	closes := []float64{10, 11, 12, 13, 14, 13, 12, 11, 10, 9}
	highs  := []float64{10.5, 11.5, 12.5, 13.5, 14.5, 13.5, 12.5, 11.5, 10.5, 9.5}
	lows   := []float64{9.5,  10.5, 11.5, 12.5, 13.5, 12.5, 11.5, 10.5, 9.5,  8.5}
	candles := make([]registry.Candle, len(closes))
	for i := range closes {
		candles[i] = registry.Candle{
			Timestamp: time.Unix(int64(i)*3600, 0),
			Open: closes[i], High: highs[i], Low: lows[i], Close: closes[i], Volume: 1000,
		}
	}
	res, err := ParabolicSAR(candles, map[string]interface{}{"step": 0.02, "max": 0.2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Series["value"]) != len(candles) {
		t.Fatalf("length mismatch")
	}
	// After warm-up (index 1+), values must be finite
	for i := 1; i < len(candles); i++ {
		if math.IsNaN(res.Series["value"][i]) {
			t.Errorf("unexpected NaN at index %d", i)
		}
	}
}

func TestIchimoku_OutputKeys(t *testing.T) {
	// Need at least senkou_b + displacement candles (52+26=78) for all outputs
	n := 100
	closes := make([]float64, n)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	candles := makeCandles(closes)
	res, err := Ichimoku(candles, map[string]interface{}{
		"tenkan": 9, "kijun": 26, "senkou_b": 52, "displacement": 26,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, key := range []string{"tenkan", "kijun", "senkou_a", "senkou_b", "chikou"} {
		if _, ok := res.Series[key]; !ok {
			t.Errorf("missing output series %q", key)
		}
		if len(res.Series[key]) != n {
			t.Errorf("series %q length %d, want %d", key, len(res.Series[key]), n)
		}
	}
}
```

**Step 2: Run — expect compile failure**
```bash
cd backend && go test ./internal/indicator/... -run "TestParabolicSAR|TestIchimoku" -v 2>&1 | head -10
```

**Step 3: Implement ParabolicSAR + Ichimoku**

```go
// ParabolicSAR computes Parabolic Stop-And-Reverse.
// params: step float (default 0.02), max float (default 0.2)
// Output series: "value" (one SAR value per candle, NaN for index 0).
func ParabolicSAR(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	step, err := floatParam(params, "step", 0.02)
	if err != nil {
		return CalcResult{}, err
	}
	maxAF, err := floatParam(params, "max", 0.2)
	if err != nil {
		return CalcResult{}, err
	}
	n := len(candles)
	vals := nanSlice(n)
	if n < 2 {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}

	// Determine initial trend: bullish if close[1] > close[0]
	bull := candles[1].Close > candles[0].Close
	af := step
	var ep, sar float64
	if bull {
		ep = candles[0].High
		sar = candles[0].Low
	} else {
		ep = candles[0].Low
		sar = candles[0].High
	}
	vals[0] = math.NaN()

	for i := 1; i < n; i++ {
		c := candles[i]
		// Advance SAR
		newSAR := sar + af*(ep-sar)

		// Clamp SAR behind the two previous candles
		if bull {
			if i >= 2 {
				newSAR = math.Min(newSAR, math.Min(candles[i-1].Low, candles[i-2].Low))
			} else {
				newSAR = math.Min(newSAR, candles[i-1].Low)
			}
		} else {
			if i >= 2 {
				newSAR = math.Max(newSAR, math.Max(candles[i-1].High, candles[i-2].High))
			} else {
				newSAR = math.Max(newSAR, candles[i-1].High)
			}
		}

		// Check for reversal
		if bull && c.Low < newSAR {
			bull = false
			newSAR = ep
			ep = c.Low
			af = step
		} else if !bull && c.High > newSAR {
			bull = true
			newSAR = ep
			ep = c.High
			af = step
		} else {
			// Update EP and AF
			if bull && c.High > ep {
				ep = c.High
				af = math.Min(af+step, maxAF)
			} else if !bull && c.Low < ep {
				ep = c.Low
				af = math.Min(af+step, maxAF)
			}
		}
		vals[i] = newSAR
		sar = newSAR
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}

// highLow returns highest high and lowest low over candles[start:end+1].
func highLow(candles []registry.Candle, start, end int) (float64, float64) {
	hi, lo := candles[start].High, candles[start].Low
	for i := start + 1; i <= end; i++ {
		if candles[i].High > hi {
			hi = candles[i].High
		}
		if candles[i].Low < lo {
			lo = candles[i].Low
		}
	}
	return hi, lo
}

// Ichimoku computes all five Ichimoku Cloud components.
// params: tenkan int (9), kijun int (26), senkou_b int (52), displacement int (26)
// Outputs: "tenkan", "kijun", "senkou_a", "senkou_b", "chikou"
// All series have length == len(candles). Values outside computable range are NaN.
func Ichimoku(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	tenkan, _ := intParam(params, "tenkan", 9)
	kijun, _ := intParam(params, "kijun", 26)
	senkouBPeriod, _ := intParam(params, "senkou_b", 52)
	disp, _ := intParam(params, "displacement", 26)
	n := len(candles)

	tenkanVals  := nanSlice(n)
	kijunVals   := nanSlice(n)
	senkouAVals := nanSlice(n)
	senkouBVals := nanSlice(n)
	chikouVals  := nanSlice(n)

	for i := 0; i < n; i++ {
		// Tenkan-sen
		if i >= tenkan-1 {
			hi, lo := highLow(candles, i-tenkan+1, i)
			tenkanVals[i] = (hi + lo) / 2
		}
		// Kijun-sen
		if i >= kijun-1 {
			hi, lo := highLow(candles, i-kijun+1, i)
			kijunVals[i] = (hi + lo) / 2
		}
		// Senkou Span A = (tenkan + kijun) / 2, plotted disp periods ahead
		if i >= kijun-1 { // tenkan <= kijun assumed
			t := tenkanVals[i]
			k := kijunVals[i]
			if !math.IsNaN(t) && !math.IsNaN(k) {
				target := i + disp
				if target < n {
					senkouAVals[target] = (t + k) / 2
				}
			}
		}
		// Senkou Span B, plotted disp periods ahead
		if i >= senkouBPeriod-1 {
			hi, lo := highLow(candles, i-senkouBPeriod+1, i)
			target := i + disp
			if target < n {
				senkouBVals[target] = (hi + lo) / 2
			}
		}
		// Chikou = current close plotted disp periods behind
		target := i - disp
		if target >= 0 {
			chikouVals[target] = candles[i].Close
		}
	}

	return CalcResult{
		Timestamps: timestamps(candles),
		Series: map[string][]float64{
			"tenkan":   tenkanVals,
			"kijun":    kijunVals,
			"senkou_a": senkouAVals,
			"senkou_b": senkouBVals,
			"chikou":   chikouVals,
		},
	}, nil
}
```

**Step 4: Run all overlay tests**
```bash
cd backend && go test ./internal/indicator/... -v
```
Expected: all PASS.

**Step 5: Commit**
```bash
git add backend/internal/indicator/overlay.go backend/internal/indicator/overlay_test.go
git commit -m "feat(phase5): add Parabolic SAR and Ichimoku Cloud overlay indicators"
```

---

## WAVE 1 — WORKTREE B: Panel Indicators

> Worktree branch: `phase5/panel`
> Only touch: `backend/internal/indicator/panel.go` and `backend/internal/indicator/panel_test.go`

---

### Task B1 — RSI + MACD

**Files:**
- Create: `backend/internal/indicator/panel.go`
- Create: `backend/internal/indicator/panel_test.go`

**Step 1: Write failing tests**

```go
// panel_test.go
package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

func makePanelCandles(closes []float64) []registry.Candle {
	out := make([]registry.Candle, len(closes))
	for i, c := range closes {
		out[i] = registry.Candle{
			Timestamp: time.Unix(int64(i)*3600, 0),
			Open: c, High: c + 0.5, Low: c - 0.5, Close: c, Volume: 1000,
		}
	}
	return out
}

func approxEqP(a, b, tol float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Abs(a-b) < tol
}

func TestRSI_AllGains_Returns100(t *testing.T) {
	// All closes rising → RSI should approach 100
	closes := make([]float64, 20)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	candles := makePanelCandles(closes)
	res, err := RSI(candles, map[string]interface{}{"period": 14})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First 14 values NaN (warm-up)
	for i := 0; i < 14; i++ {
		if !math.IsNaN(res.Series["value"][i]) {
			t.Errorf("expected NaN at index %d", i)
		}
	}
	// After warm-up: should be 100 (no losses)
	for i := 14; i < 20; i++ {
		if !approxEqP(res.Series["value"][i], 100.0, 0.001) {
			t.Errorf("RSI[%d]=%f, want 100", i, res.Series["value"][i])
		}
	}
}

func TestRSI_AllLosses_Returns0(t *testing.T) {
	closes := make([]float64, 20)
	for i := range closes {
		closes[i] = float64(100 - i)
	}
	candles := makePanelCandles(closes)
	res, err := RSI(candles, map[string]interface{}{"period": 14})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 14; i < 20; i++ {
		if !approxEqP(res.Series["value"][i], 0.0, 0.001) {
			t.Errorf("RSI[%d]=%f, want 0", i, res.Series["value"][i])
		}
	}
}

func TestMACD_OutputKeys(t *testing.T) {
	closes := make([]float64, 40)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	candles := makePanelCandles(closes)
	res, err := MACD(candles, map[string]interface{}{"fast": 12, "slow": 26, "signal": 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"macd", "signal", "histogram"} {
		if _, ok := res.Series[k]; !ok {
			t.Fatalf("missing series %q", k)
		}
		if len(res.Series[k]) != len(candles) {
			t.Fatalf("series %q length %d, want %d", k, len(res.Series[k]), len(candles))
		}
	}
	// MACD line = EMA(fast) - EMA(slow); rising price → MACD > 0 after slow period
	for i := 26; i < 40; i++ {
		if !math.IsNaN(res.Series["macd"][i]) && res.Series["macd"][i] < 0 {
			t.Errorf("MACD[%d]=%f, expected >= 0 for rising prices", i, res.Series["macd"][i])
		}
	}
}
```

**Step 2: Run — expect compile failure**
```bash
cd backend && go test ./internal/indicator/... -run "TestRSI|TestMACD" -v 2>&1 | head -10
```

**Step 3: Implement RSI + MACD**

```go
// panel.go
package indicator

import (
	"fmt"
	"math"

	"github.com/trader-claude/backend/internal/registry"
)

// panelEMA is a helper EMA used inside panel calculations.
// Returns parallel array, first (period-1) values NaN.
func panelEMA(vals []float64, period int) []float64 {
	n := len(vals)
	out := nanSlice(n)
	if n < period {
		return out
	}
	k := 2.0 / float64(period+1)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += vals[i]
	}
	prev := sum / float64(period)
	out[period-1] = prev
	for i := period; i < n; i++ {
		prev = vals[i]*k + prev*(1-k)
		out[i] = prev
	}
	return out
}

// RSI computes Relative Strength Index using Wilder smoothing.
// params: period int (default 14)
// Output: "value" (0–100 scale)
func RSI(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 14)
	if err != nil {
		return CalcResult{}, err
	}
	if period < 1 {
		return CalcResult{}, fmt.Errorf("period must be >= 1")
	}
	n := len(candles)
	vals := nanSlice(n)
	if n <= period {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}

	// Seed: average gain/loss over first period changes
	avgGain, avgLoss := 0.0, 0.0
	for i := 1; i <= period; i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			avgGain += change
		} else {
			avgLoss -= change
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	rsiVal := func(g, l float64) float64 {
		if l == 0 {
			return 100
		}
		return 100 - 100/(1+g/l)
	}
	vals[period] = rsiVal(avgGain, avgLoss)

	// Wilder smoothing
	for i := period + 1; i < n; i++ {
		change := candles[i].Close - candles[i-1].Close
		gain, loss := 0.0, 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		vals[i] = rsiVal(avgGain, avgLoss)
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}

// MACD computes MACD line, signal line, and histogram.
// params: fast int (12), slow int (26), signal int (9)
// Outputs: "macd", "signal", "histogram"
func MACD(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	fast, _ := intParam(params, "fast", 12)
	slow, _ := intParam(params, "slow", 26)
	sig, _ := intParam(params, "signal", 9)
	n := len(candles)

	closes := make([]float64, n)
	for i, c := range candles {
		closes[i] = c.Close
	}

	fastEMA := panelEMA(closes, fast)
	slowEMA := panelEMA(closes, slow)

	macdLine := nanSlice(n)
	for i := slow - 1; i < n; i++ {
		if !math.IsNaN(fastEMA[i]) && !math.IsNaN(slowEMA[i]) {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}

	// Signal = EMA(macdLine, sig) — but only over non-NaN segment
	signalLine := nanSlice(n)
	histogram := nanSlice(n)

	// Find first valid MACD index
	start := slow - 1
	if start >= n {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{
			"macd": macdLine, "signal": signalLine, "histogram": histogram,
		}}, nil
	}

	// Seed signal EMA
	if start+sig-1 < n {
		sum := 0.0
		for i := start; i < start+sig; i++ {
			sum += macdLine[i]
		}
		prev := sum / float64(sig)
		signalLine[start+sig-1] = prev
		histogram[start+sig-1] = macdLine[start+sig-1] - prev
		k := 2.0 / float64(sig+1)
		for i := start + sig; i < n; i++ {
			prev = macdLine[i]*k + prev*(1-k)
			signalLine[i] = prev
			histogram[i] = macdLine[i] - prev
		}
	}

	return CalcResult{
		Timestamps: timestamps(candles),
		Series:     map[string][]float64{"macd": macdLine, "signal": signalLine, "histogram": histogram},
	}, nil
}
```

**Step 4: Run tests**
```bash
cd backend && go test ./internal/indicator/... -run "TestRSI|TestMACD" -v
```
Expected: all PASS.

**Step 5: Commit**
```bash
git add backend/internal/indicator/panel.go backend/internal/indicator/panel_test.go
git commit -m "feat(phase5): add RSI and MACD panel indicators with tests"
```

---

### Task B2 — Stochastic + ATR

**Step 1: Add tests to `panel_test.go`**

```go
func TestStochastic_Overbought(t *testing.T) {
	// All prices rising to max → %K should approach 100
	closes := make([]float64, 20)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	candles := make([]registry.Candle, len(closes))
	for i, c := range closes {
		candles[i] = registry.Candle{
			Timestamp: time.Unix(int64(i)*3600, 0),
			Open: c, High: c + 1, Low: float64(100), Close: c, Volume: 1000,
		}
	}
	res, err := Stochastic(candles, map[string]interface{}{"k_period": 5, "d_period": 3, "smooth": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"k", "d"} {
		if _, ok := res.Series[k]; !ok {
			t.Fatalf("missing series %q", k)
		}
	}
	// After warm-up, %K should be > 80 (overbought) since close is always near high
	for i := 10; i < len(candles); i++ {
		if !math.IsNaN(res.Series["k"][i]) && res.Series["k"][i] < 80 {
			t.Errorf("stoch_k[%d]=%f, expected overbought (>80)", i, res.Series["k"][i])
		}
	}
}

func TestATR_ConstantRange(t *testing.T) {
	// Candles with constant range of 2 (high-low=2, no gaps) → ATR should converge to 2
	candles := make([]registry.Candle, 20)
	for i := range candles {
		c := float64(100)
		candles[i] = registry.Candle{
			Timestamp: time.Unix(int64(i)*3600, 0),
			Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1000,
		}
	}
	res, err := ATR(candles, map[string]interface{}{"period": 14})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After seed period, ATR should be 2
	for i := 14; i < 20; i++ {
		if !approxEqP(res.Series["value"][i], 2.0, 0.01) {
			t.Errorf("ATR[%d]=%f, want ~2.0", i, res.Series["value"][i])
		}
	}
}
```

**Step 2: Run — expect compile failure**
```bash
cd backend && go test ./internal/indicator/... -run "TestStochastic|TestATR" -v 2>&1 | head -10
```

**Step 3: Implement Stochastic + ATR**

```go
// Stochastic computes Fast Stochastic Oscillator (%K and %D).
// params: k_period int (14), d_period int (3), smooth int (3)
// %K raw = (close - lowest_low) / (highest_high - lowest_low) * 100
// %K smoothed = SMA(%K raw, smooth); %D = SMA(%K, d_period)
func Stochastic(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	kPeriod, _ := intParam(params, "k_period", 14)
	dPeriod, _ := intParam(params, "d_period", 3)
	smooth, _ := intParam(params, "smooth", 3)
	n := len(candles)

	rawK := nanSlice(n)
	for i := kPeriod - 1; i < n; i++ {
		hi, lo := candles[i-kPeriod+1].High, candles[i-kPeriod+1].Low
		for j := i - kPeriod + 2; j <= i; j++ {
			if candles[j].High > hi {
				hi = candles[j].High
			}
			if candles[j].Low < lo {
				lo = candles[j].Low
			}
		}
		denom := hi - lo
		if denom == 0 {
			rawK[i] = 50
		} else {
			rawK[i] = (candles[i].Close - lo) / denom * 100
		}
	}

	// %K = SMA(rawK, smooth)
	kVals := nanSlice(n)
	for i := kPeriod + smooth - 2; i < n; i++ {
		sum := 0.0
		for j := i - smooth + 1; j <= i; j++ {
			if math.IsNaN(rawK[j]) {
				sum = math.NaN()
				break
			}
			sum += rawK[j]
		}
		kVals[i] = sum / float64(smooth)
	}

	// %D = SMA(%K, dPeriod)
	dVals := nanSlice(n)
	for i := kPeriod + smooth + dPeriod - 3; i < n; i++ {
		sum := 0.0
		count := 0
		for j := i - dPeriod + 1; j <= i; j++ {
			if !math.IsNaN(kVals[j]) {
				sum += kVals[j]
				count++
			}
		}
		if count == dPeriod {
			dVals[i] = sum / float64(dPeriod)
		}
	}

	return CalcResult{
		Timestamps: timestamps(candles),
		Series:     map[string][]float64{"k": kVals, "d": dVals},
	}, nil
}

// ATR computes Average True Range using Wilder smoothing.
// params: period int (default 14)
func ATR(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 14)
	if err != nil {
		return CalcResult{}, err
	}
	n := len(candles)
	vals := nanSlice(n)
	if n < 2 {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}

	trueRange := func(i int) float64 {
		c := candles[i]
		prev := candles[i-1].Close
		return math.Max(c.High-c.Low, math.Max(math.Abs(c.High-prev), math.Abs(c.Low-prev)))
	}

	// Seed: simple average of first period TRs
	if n < period+1 {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trueRange(i)
	}
	atr := sum / float64(period)
	vals[period] = atr

	// Wilder smoothing
	for i := period + 1; i < n; i++ {
		atr = (atr*float64(period-1) + trueRange(i)) / float64(period)
		vals[i] = atr
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}
```

**Step 4: Run tests**
```bash
cd backend && go test ./internal/indicator/... -v
```
Expected: all PASS.

**Step 5: Commit**
```bash
git add backend/internal/indicator/panel.go backend/internal/indicator/panel_test.go
git commit -m "feat(phase5): add Stochastic and ATR panel indicators"
```

---

### Task B3 — OBV + Volume

**Step 1: Add tests**

```go
func TestOBV(t *testing.T) {
	candles := []registry.Candle{
		{Timestamp: time.Unix(0, 0),    Open: 10, High: 11, Low: 9,  Close: 10, Volume: 1000},
		{Timestamp: time.Unix(3600, 0), Open: 10, High: 12, Low: 9,  Close: 11, Volume: 500},  // up
		{Timestamp: time.Unix(7200, 0), Open: 11, High: 12, Low: 8,  Close: 9,  Volume: 300},  // down
		{Timestamp: time.Unix(10800, 0),Open: 9,  High: 10, Low: 8,  Close: 9,  Volume: 200},  // flat
		{Timestamp: time.Unix(14400, 0),Open: 9,  High: 13, Low: 8,  Close: 12, Volume: 800},  // up
	}
	res, err := OBV(candles, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []float64{0, 500, 200, 200, 1000}
	for i, w := range want {
		if !approxEqP(res.Series["value"][i], w, 0.001) {
			t.Errorf("OBV[%d]=%f, want %f", i, res.Series["value"][i], w)
		}
	}
}

func TestVolume_Length(t *testing.T) {
	candles := makePanelCandles([]float64{1, 2, 3, 4, 5})
	res, err := Volume(candles, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Series["value"]) != 5 {
		t.Fatalf("expected length 5, got %d", len(res.Series["value"]))
	}
	for i, c := range candles {
		if !approxEqP(res.Series["value"][i], c.Volume, 0.001) {
			t.Errorf("volume[%d]=%f, want %f", i, res.Series["value"][i], c.Volume)
		}
	}
}
```

**Step 2: Run — expect compile failure**
```bash
cd backend && go test ./internal/indicator/... -run "TestOBV|TestVolume" -v 2>&1 | head -10
```

**Step 3: Implement OBV + Volume**

```go
// OBV computes On-Balance Volume.
// No params. Starts at 0.
func OBV(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	n := len(candles)
	vals := make([]float64, n)
	if n == 0 {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}
	vals[0] = 0
	for i := 1; i < n; i++ {
		switch {
		case candles[i].Close > candles[i-1].Close:
			vals[i] = vals[i-1] + candles[i].Volume
		case candles[i].Close < candles[i-1].Close:
			vals[i] = vals[i-1] - candles[i].Volume
		default:
			vals[i] = vals[i-1]
		}
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}

// Volume returns raw volume values. No params, no warm-up.
// The frontend colours bars green (close >= open) or red (close < open).
func Volume(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	n := len(candles)
	vals := make([]float64, n)
	for i, c := range candles {
		vals[i] = c.Volume
	}
	return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
}
```

**Step 4: Run all panel tests**
```bash
cd backend && go test ./internal/indicator/... -v
```
Expected: all PASS.

**Step 5: Commit**
```bash
git add backend/internal/indicator/panel.go backend/internal/indicator/panel_test.go
git commit -m "feat(phase5): add OBV and Volume panel indicators — Wave 1 complete"
```

---

## MERGE POINT

After both worktrees finish:

```bash
# Merge overlay worktree
git merge phase5/overlay --no-ff -m "merge(phase5): overlay indicators from worktree"

# Merge panel worktree
git merge phase5/panel --no-ff -m "merge(phase5): panel indicators from worktree"

# Verify everything passes together
cd backend && go test ./internal/indicator/... -v
```

Expected: 0 conflicts (different files), all tests PASS.

---

## WAVE 2 — Sequential

---

### Task 1 — Indicator Registry (`registry.go`)

**Files:**
- Create: `backend/internal/indicator/registry.go`

No tests needed — this is pure data (catalog wiring). Verified implicitly by the API integration test in Task 3.

```go
package indicator

import (
	"fmt"

	"github.com/trader-claude/backend/internal/registry"
)

// entry binds metadata to a calculation function.
type entry struct {
	Meta IndicatorMeta
	Fn   CalcFunc
}

var catalog []entry

func init() {
	p := func(name, typ, def string, min, max interface{}, desc string, required bool) registry.ParamDefinition {
		return registry.ParamDefinition{Name: name, Type: typ, Default: def, Min: min, Max: max, Description: desc, Required: required}
	}
	o := func(name, color string) OutputDef { return OutputDef{Name: name, Color: color} }

	catalog = []entry{
		// Overlay — Trend
		{IndicatorMeta{"sma", "SMA", "Simple Moving Average", "overlay", "trend",
			[]registry.ParamDefinition{p("period", "int", "20", 2, 500, "Lookback period", true)},
			[]OutputDef{o("value", "#2962FF")}}, SMA},
		{IndicatorMeta{"ema", "EMA", "Exponential Moving Average", "overlay", "trend",
			[]registry.ParamDefinition{p("period", "int", "20", 2, 500, "Lookback period", true)},
			[]OutputDef{o("value", "#FF6D00")}}, EMA},
		{IndicatorMeta{"wma", "WMA", "Weighted Moving Average", "overlay", "trend",
			[]registry.ParamDefinition{p("period", "int", "20", 2, 500, "Lookback period", true)},
			[]OutputDef{o("value", "#6200EA")}}, WMA},
		{IndicatorMeta{"bollinger_bands", "BB", "Bollinger Bands", "overlay", "volatility",
			[]registry.ParamDefinition{
				p("period", "int", "20", 2, 500, "SMA period", true),
				p("std_dev", "float", "2.0", 0.1, 10, "Standard deviation multiplier", true),
			},
			[]OutputDef{o("upper", "#F44336"), o("middle", "#2962FF"), o("lower", "#4CAF50")}}, BollingerBands},
		{IndicatorMeta{"vwap", "VWAP", "Volume-Weighted Average Price", "overlay", "trend",
			nil,
			[]OutputDef{o("value", "#E91E63")}}, VWAP},
		{IndicatorMeta{"parabolic_sar", "SAR", "Parabolic SAR", "overlay", "trend",
			[]registry.ParamDefinition{
				p("step", "float", "0.02", 0.001, 0.5, "Acceleration step", true),
				p("max", "float", "0.2", 0.01, 1.0, "Maximum acceleration", true),
			},
			[]OutputDef{o("value", "#FF9800")}}, ParabolicSAR},
		{IndicatorMeta{"ichimoku", "Ichimoku", "Ichimoku Cloud", "overlay", "trend",
			[]registry.ParamDefinition{
				p("tenkan", "int", "9", 2, 100, "Tenkan-sen period", true),
				p("kijun", "int", "26", 2, 200, "Kijun-sen period", true),
				p("senkou_b", "int", "52", 2, 500, "Senkou Span B period", true),
				p("displacement", "int", "26", 1, 100, "Cloud displacement", true),
			},
			[]OutputDef{
				o("tenkan", "#E91E63"), o("kijun", "#2962FF"),
				o("senkou_a", "#4CAF50"), o("senkou_b", "#F44336"), o("chikou", "#9C27B0"),
			}}, Ichimoku},
		// Panel — Momentum
		{IndicatorMeta{"rsi", "RSI", "Relative Strength Index", "panel", "momentum",
			[]registry.ParamDefinition{p("period", "int", "14", 2, 200, "Lookback period", true)},
			[]OutputDef{o("value", "#7B1FA2")}}, RSI},
		{IndicatorMeta{"macd", "MACD", "MACD", "panel", "momentum",
			[]registry.ParamDefinition{
				p("fast", "int", "12", 2, 200, "Fast EMA period", true),
				p("slow", "int", "26", 2, 500, "Slow EMA period", true),
				p("signal", "int", "9", 2, 200, "Signal EMA period", true),
			},
			[]OutputDef{o("macd", "#2962FF"), o("signal", "#FF6D00"), o("histogram", "#26A69A")}}, MACD},
		{IndicatorMeta{"stochastic", "Stoch", "Stochastic Oscillator", "panel", "momentum",
			[]registry.ParamDefinition{
				p("k_period", "int", "14", 2, 200, "%K lookback", true),
				p("d_period", "int", "3", 1, 50, "%D smoothing", true),
				p("smooth", "int", "3", 1, 50, "%K smoothing", true),
			},
			[]OutputDef{o("k", "#2962FF"), o("d", "#FF6D00")}}, Stochastic},
		// Panel — Volatility
		{IndicatorMeta{"atr", "ATR", "Average True Range", "panel", "volatility",
			[]registry.ParamDefinition{p("period", "int", "14", 2, 200, "Lookback period", true)},
			[]OutputDef{o("value", "#FF6D00")}}, ATR},
		// Panel — Volume
		{IndicatorMeta{"obv", "OBV", "On-Balance Volume", "panel", "volume",
			nil,
			[]OutputDef{o("value", "#2962FF")}}, OBV},
		{IndicatorMeta{"volume", "Volume", "Volume", "panel", "volume",
			nil,
			[]OutputDef{o("value", "#26A69A")}}, Volume},
	}
}

// All returns the full catalog metadata (no CalcFunc exposed).
func All() []IndicatorMeta {
	out := make([]IndicatorMeta, len(catalog))
	for i, e := range catalog {
		out[i] = e.Meta
	}
	return out
}

// Get returns the CalcFunc for the given indicator ID, or an error.
func Get(id string) (CalcFunc, error) {
	for _, e := range catalog {
		if e.Meta.ID == id {
			return e.Fn, nil
		}
	}
	return nil, fmt.Errorf("unknown indicator: %q", id)
}
```

**Commit:**
```bash
git add backend/internal/indicator/registry.go
git commit -m "feat(phase5): add indicator registry wiring all 13 indicators"
```

---

### Task 2 — Fiber Handler + Routes

**Files:**
- Create: `backend/internal/indicator/handler.go`
- Modify: `backend/internal/api/routes.go`

**Step 1: Write the handler**

```go
// handler.go
package indicator

import (
	"math"

	"github.com/gofiber/fiber/v2"
	"github.com/trader-claude/backend/internal/registry"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

// ListIndicators handles GET /api/v1/indicators
func (h *Handler) ListIndicators(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"indicators": All()})
}

// calculateRequest is the body for POST /api/v1/indicators/calculate
type calculateRequest struct {
	IndicatorID string                   `json:"indicator_id"`
	Params      map[string]interface{}   `json:"params"`
	Candles     []candleInput            `json:"candles"`
}

type candleInput struct {
	Timestamp int64   `json:"timestamp"` // Unix seconds
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// Calculate handles POST /api/v1/indicators/calculate
func (h *Handler) Calculate(c *fiber.Ctx) error {
	var req calculateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.IndicatorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "indicator_id is required"})
	}
	if len(req.Candles) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "candles must not be empty"})
	}

	fn, err := Get(req.IndicatorID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Convert input candles to registry.Candle
	candles := make([]registry.Candle, len(req.Candles))
	for i, ci := range req.Candles {
		candles[i] = registry.Candle{
			Timestamp: timeFromUnix(ci.Timestamp),
			Open: ci.Open, High: ci.High, Low: ci.Low,
			Close: ci.Close, Volume: ci.Volume,
		}
	}

	result, err := fn(candles, req.Params)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	// Serialise NaN as null
	serialised := make(map[string][]interface{}, len(result.Series))
	for name, series := range result.Series {
		out := make([]interface{}, len(series))
		for i, v := range series {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				out[i] = nil
			} else {
				out[i] = v
			}
		}
		serialised[name] = out
	}

	return c.JSON(fiber.Map{
		"timestamps": result.Timestamps,
		"series":     serialised,
	})
}
```

Add a small helper in handler.go:
```go
import "time"
func timeFromUnix(ts int64) time.Time { return time.Unix(ts, 0) }
```

**Step 2: Register routes in `routes.go`**

In `routes.go`, add the import and the handler:
```go
import "github.com/trader-claude/backend/internal/indicator"
```

After the replay routes block, add:
```go
// --- Indicators ---
ih := indicator.NewHandler()
v1.Get("/indicators", ih.ListIndicators)
v1.Post("/indicators/calculate", ih.Calculate)
```

**Step 3: Build to verify no compile errors**
```bash
cd backend && go build ./...
```
Expected: exits 0 with no output.

**Step 4: Commit**
```bash
git add backend/internal/indicator/handler.go backend/internal/api/routes.go
git commit -m "feat(phase5): add indicator API handler and register routes"
```

---

### Task 3 — API Integration Tests

**Files:**
- Create: `backend/internal/indicator/handler_test.go`

```go
// handler_test.go
package indicator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func testApp() *fiber.App {
	app := fiber.New()
	h := NewHandler()
	app.Get("/api/v1/indicators", h.ListIndicators)
	app.Post("/api/v1/indicators/calculate", h.Calculate)
	return app
}

func TestListIndicators(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest("GET", "/api/v1/indicators", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d, want 200", resp.StatusCode)
	}
	var body struct {
		Indicators []IndicatorMeta `json:"indicators"`
	}
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(body.Indicators) != 13 {
		t.Errorf("expected 13 indicators, got %d", len(body.Indicators))
	}
}

func TestCalculateEMA(t *testing.T) {
	app := testApp()
	payload := map[string]interface{}{
		"indicator_id": "ema",
		"params":       map[string]interface{}{"period": 3},
		"candles": []map[string]interface{}{
			{"timestamp": 0, "open": 1, "high": 1, "low": 1, "close": 1, "volume": 100},
			{"timestamp": 3600, "open": 2, "high": 2, "low": 2, "close": 2, "volume": 100},
			{"timestamp": 7200, "open": 3, "high": 3, "low": 3, "close": 3, "volume": 100},
			{"timestamp": 10800, "open": 4, "high": 4, "low": 4, "close": 4, "volume": 100},
			{"timestamp": 14400, "open": 5, "high": 5, "low": 5, "close": 5, "volume": 100},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/indicators/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, data)
	}
	var result struct {
		Timestamps []int64                    `json:"timestamps"`
		Series     map[string][]interface{}   `json:"series"`
	}
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(result.Timestamps) != 5 {
		t.Errorf("expected 5 timestamps, got %d", len(result.Timestamps))
	}
	vals, ok := result.Series["value"]
	if !ok {
		t.Fatal("missing 'value' series")
	}
	// First two are null (NaN)
	if vals[0] != nil || vals[1] != nil {
		t.Errorf("expected null for warm-up, got %v %v", vals[0], vals[1])
	}
	// Third value ≈ 2.0
	if v, ok := vals[2].(float64); !ok || v < 1.9 || v > 2.1 {
		t.Errorf("vals[2]=%v, want ~2.0", vals[2])
	}
}

func TestCalculate_UnknownIndicator(t *testing.T) {
	app := testApp()
	payload := map[string]interface{}{
		"indicator_id": "not_real",
		"params":       map[string]interface{}{},
		"candles": []map[string]interface{}{
			{"timestamp": 0, "open": 1, "high": 1, "low": 1, "close": 1, "volume": 100},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/indicators/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
```

**Run tests:**
```bash
cd backend && go test ./internal/indicator/... -v
```
Expected: all PASS (including handler tests).

**Commit:**
```bash
git add backend/internal/indicator/handler_test.go
git commit -m "test(phase5): add API integration tests for indicator endpoints"
```

---

### Task 4 — Frontend Types + API Client

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/api/indicators.ts`

**Step 1: Add types to `types/index.ts`**

Append at the end of the file:

```ts
// ── Indicator types (Phase 5) ───────────────────────────────────────────────

export type IndicatorType = 'overlay' | 'panel'
export type IndicatorGroup = 'trend' | 'momentum' | 'volatility' | 'volume'

export interface OutputDef {
  name: string
  color: string
}

export interface IndicatorMeta {
  id: string
  name: string
  full_name: string
  type: IndicatorType
  group: IndicatorGroup
  params: ParamDefinition[]   // reuses existing ParamDefinition
  outputs: OutputDef[]
}

export interface CalcResult {
  timestamps: number[]
  series: Record<string, (number | null)[]>
}

export interface ActiveIndicator {
  meta: IndicatorMeta
  params: Record<string, unknown>
  result?: CalcResult
}

export interface CalculateRequest {
  indicator_id: string
  params: Record<string, unknown>
  candles: Array<{
    timestamp: number
    open: number
    high: number
    low: number
    close: number
    volume: number
  }>
}
```

**Step 2: Create `api/indicators.ts`**

```ts
import { api } from './client'
import type { IndicatorMeta, CalcResult, CalculateRequest } from '../types'

export async function fetchIndicators(): Promise<IndicatorMeta[]> {
  const { data } = await api.get<{ indicators: IndicatorMeta[] }>('/indicators')
  return data.indicators
}

export async function calculateIndicator(req: CalculateRequest): Promise<CalcResult> {
  const { data } = await api.post<CalcResult>('/indicators/calculate', req)
  return data
}
```

**Step 3: Commit**
```bash
git add frontend/src/types/index.ts frontend/src/api/indicators.ts
git commit -m "feat(phase5): add indicator types and API client"
```

---

### Task 5 — IndicatorModal + IndicatorParamForm + IndicatorChips

**Files:**
- Create: `frontend/src/components/chart/IndicatorModal.tsx`
- Create: `frontend/src/components/chart/IndicatorParamForm.tsx`
- Create: `frontend/src/components/chart/IndicatorChips.tsx`

**Step 1: IndicatorParamForm.tsx**

This reuses the same `ParamDefinition` schema as the Backtest param form. Keep it minimal — slider + number for numeric, toggle for bool.

```tsx
import { ParamDefinition } from '../../types'

interface Props {
  params: ParamDefinition[]
  values: Record<string, unknown>
  onChange: (key: string, value: unknown) => void
}

export function IndicatorParamForm({ params, values, onChange }: Props) {
  if (!params || params.length === 0) {
    return <p className="text-sm text-muted-foreground">No parameters</p>
  }
  return (
    <div className="space-y-3">
      {params.map((p) => (
        <div key={p.name}>
          <label className="text-sm font-medium">{p.name}</label>
          {p.description && (
            <p className="text-xs text-muted-foreground mb-1">{p.description}</p>
          )}
          {(p.type === 'int' || p.type === 'float') && (
            <div className="flex items-center gap-2">
              <input
                type="range"
                min={p.min as number ?? 1}
                max={p.max as number ?? 500}
                step={p.type === 'float' ? 0.1 : 1}
                value={Number(values[p.name] ?? p.default)}
                onChange={(e) => onChange(p.name, p.type === 'int' ? parseInt(e.target.value) : parseFloat(e.target.value))}
                className="flex-1"
              />
              <input
                type="number"
                min={p.min as number ?? 1}
                max={p.max as number ?? 500}
                step={p.type === 'float' ? 0.1 : 1}
                value={Number(values[p.name] ?? p.default)}
                onChange={(e) => onChange(p.name, p.type === 'int' ? parseInt(e.target.value) : parseFloat(e.target.value))}
                className="w-20 border rounded px-2 py-1 text-sm"
              />
            </div>
          )}
          {p.type === 'bool' && (
            <input
              type="checkbox"
              checked={Boolean(values[p.name] ?? p.default)}
              onChange={(e) => onChange(p.name, e.target.checked)}
            />
          )}
          {p.type === 'select' && p.options && (
            <select
              value={String(values[p.name] ?? p.default)}
              onChange={(e) => onChange(p.name, e.target.value)}
              className="border rounded px-2 py-1 text-sm"
            >
              {p.options.map((o) => <option key={o} value={o}>{o}</option>)}
            </select>
          )}
        </div>
      ))}
    </div>
  )
}
```

**Step 2: IndicatorModal.tsx**

```tsx
import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, X } from 'lucide-react'
import { fetchIndicators } from '../../api/indicators'
import { IndicatorMeta, ActiveIndicator } from '../../types'
import { IndicatorParamForm } from './IndicatorParamForm'

interface Props {
  open: boolean
  onClose: () => void
  active: ActiveIndicator[]
  onAdd: (indicator: ActiveIndicator) => void
}

const GROUPS: { id: string; label: string }[] = [
  { id: 'trend', label: 'Trend' },
  { id: 'momentum', label: 'Momentum' },
  { id: 'volatility', label: 'Volatility' },
  { id: 'volume', label: 'Volume' },
]

export function IndicatorModal({ open, onClose, active, onAdd }: Props) {
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<IndicatorMeta | null>(null)
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})

  const { data: indicators = [] } = useQuery({
    queryKey: ['indicators'],
    queryFn: fetchIndicators,
    staleTime: Infinity,
  })

  const filtered = useMemo(() =>
    indicators.filter((ind) =>
      ind.full_name.toLowerCase().includes(search.toLowerCase()) ||
      ind.name.toLowerCase().includes(search.toLowerCase())
    ), [indicators, search])

  const grouped = useMemo(() =>
    GROUPS.map((g) => ({
      ...g,
      items: filtered.filter((ind) => ind.group === g.id),
    })), [filtered])

  function selectIndicator(meta: IndicatorMeta) {
    setSelected(meta)
    const defaults: Record<string, unknown> = {}
    meta.params?.forEach((p) => { defaults[p.name] = p.default })
    setParamValues(defaults)
  }

  function handleAdd() {
    if (!selected) return
    onAdd({ meta: selected, params: paramValues })
    setSelected(null)
    setParamValues({})
    onClose()
  }

  const isActive = (id: string) => active.some((a) => a.meta.id === id)

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-xl w-[560px] max-h-[80vh] flex flex-col">
        <div className="flex items-center justify-between px-4 py-3 border-b">
          <h2 className="font-semibold">Indicators</h2>
          <button onClick={onClose}><X className="w-4 h-4" /></button>
        </div>

        <div className="px-4 py-2 border-b">
          <div className="flex items-center gap-2 border rounded px-3 py-1.5">
            <Search className="w-4 h-4 text-muted-foreground" />
            <input
              className="flex-1 bg-transparent text-sm outline-none"
              placeholder="Search indicators..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              autoFocus
            />
          </div>
        </div>

        <div className="flex flex-1 overflow-hidden">
          {/* List */}
          <div className="w-56 border-r overflow-y-auto py-2">
            {grouped.map((g) =>
              g.items.length === 0 ? null : (
                <div key={g.id}>
                  <p className="px-4 py-1 text-xs font-semibold uppercase text-muted-foreground">{g.label}</p>
                  {g.items.map((ind) => (
                    <button
                      key={ind.id}
                      onClick={() => selectIndicator(ind)}
                      className={`w-full text-left px-4 py-2 text-sm hover:bg-accent flex items-center justify-between
                        ${selected?.id === ind.id ? 'bg-accent' : ''}
                        ${isActive(ind.id) ? 'text-muted-foreground' : ''}`}
                    >
                      <span>{ind.full_name}</span>
                      {isActive(ind.id) && <span className="text-xs text-primary">●</span>}
                    </button>
                  ))}
                </div>
              )
            )}
          </div>

          {/* Param form */}
          <div className="flex-1 p-4 overflow-y-auto">
            {selected ? (
              <div className="space-y-4">
                <div>
                  <h3 className="font-medium">{selected.full_name}</h3>
                  <p className="text-xs text-muted-foreground capitalize">{selected.group} · {selected.type}</p>
                </div>
                <IndicatorParamForm
                  params={selected.params ?? []}
                  values={paramValues}
                  onChange={(k, v) => setParamValues((prev) => ({ ...prev, [k]: v }))}
                />
                <button
                  onClick={handleAdd}
                  className="w-full bg-primary text-primary-foreground rounded px-4 py-2 text-sm font-medium hover:bg-primary/90"
                >
                  Add to Chart
                </button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Select an indicator</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
```

**Step 3: IndicatorChips.tsx**

```tsx
import { X, Settings2 } from 'lucide-react'
import { ActiveIndicator } from '../../types'

interface Props {
  indicators: ActiveIndicator[]
  onRemove: (id: string) => void
  onEdit: (indicator: ActiveIndicator) => void
}

export function IndicatorChips({ indicators, onRemove, onEdit }: Props) {
  if (indicators.length === 0) return null
  return (
    <div className="flex flex-wrap gap-1">
      {indicators.map((ind) => (
        <div
          key={ind.meta.id}
          className="flex items-center gap-1 bg-secondary text-secondary-foreground rounded px-2 py-0.5 text-xs"
        >
          <span
            className="w-2 h-2 rounded-full inline-block"
            style={{ background: ind.meta.outputs[0]?.color ?? '#888' }}
          />
          <span>{ind.meta.name}</span>
          <button onClick={() => onEdit(ind)} className="hover:text-primary">
            <Settings2 className="w-3 h-3" />
          </button>
          <button onClick={() => onRemove(ind.meta.id)} className="hover:text-destructive">
            <X className="w-3 h-3" />
          </button>
        </div>
      ))}
    </div>
  )
}
```

**Step 4: Commit**
```bash
git add frontend/src/components/chart/IndicatorModal.tsx \
        frontend/src/components/chart/IndicatorParamForm.tsx \
        frontend/src/components/chart/IndicatorChips.tsx
git commit -m "feat(phase5): add IndicatorModal, IndicatorParamForm, IndicatorChips components"
```

---

### Task 6 — PanelChart Component

**Files:**
- Create: `frontend/src/components/chart/PanelChart.tsx`

A self-contained lightweight-charts instance for one panel indicator.

```tsx
import { useEffect, useRef } from 'react'
import { createChart, ColorType, IChartApi, ISeriesApi } from 'lightweight-charts'
import { X } from 'lucide-react'
import { ActiveIndicator } from '../../types'

interface Props {
  indicator: ActiveIndicator
  onClose: () => void
  isDark: boolean
}

export function PanelChart({ indicator, onClose, isDark }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)

  useEffect(() => {
    if (!containerRef.current) return
    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: 120,
      layout: {
        background: { type: ColorType.Solid, color: isDark ? '#0f0f0f' : '#ffffff' },
        textColor: isDark ? '#d1d5db' : '#374151',
      },
      grid: {
        vertLines: { color: isDark ? '#1f2937' : '#f3f4f6' },
        horzLines: { color: isDark ? '#1f2937' : '#f3f4f6' },
      },
      rightPriceScale: { borderVisible: false },
      timeScale: { borderVisible: false, visible: false },
      crosshair: { horzLine: { visible: false } },
    })
    chartRef.current = chart

    const result = indicator.result
    if (result) {
      indicator.meta.outputs.forEach((output) => {
        const series = result.series[output.name]
        if (!series) return

        let s: ISeriesApi<'Line'> | ISeriesApi<'Histogram'>

        if (output.name === 'histogram') {
          s = chart.addHistogramSeries({ color: output.color }) as ISeriesApi<'Histogram'>
        } else {
          s = chart.addLineSeries({ color: output.color, lineWidth: 1 })
        }

        const points = result.timestamps
          .map((ts, i) => ({ time: ts as unknown as import('lightweight-charts').Time, value: series[i] }))
          .filter((p) => p.value !== null && p.value !== undefined) as Array<{ time: import('lightweight-charts').Time; value: number }>

        s.setData(points)
      })
    }

    const ro = new ResizeObserver(() => {
      if (containerRef.current) {
        chart.applyOptions({ width: containerRef.current.clientWidth })
      }
    })
    ro.observe(containerRef.current)

    return () => {
      ro.disconnect()
      chart.remove()
      chartRef.current = null
    }
  }, [indicator, isDark])

  const paramSummary = Object.entries(indicator.params)
    .map(([k, v]) => `${k}:${v}`)
    .join(', ')

  return (
    <div className="border-t">
      <div className="flex items-center justify-between px-3 py-1 text-xs text-muted-foreground">
        <span>
          <span className="font-medium text-foreground">{indicator.meta.name}</span>
          {paramSummary && ` (${paramSummary})`}
        </span>
        <button onClick={onClose} className="hover:text-destructive">
          <X className="w-3 h-3" />
        </button>
      </div>
      <div ref={containerRef} />
    </div>
  )
}
```

**Commit:**
```bash
git add frontend/src/components/chart/PanelChart.tsx
git commit -m "feat(phase5): add PanelChart component for panel indicators"
```

---

### Task 7 — Wire Chart Page: Overlay Rendering + localStorage

**Files:**
- Modify: `frontend/src/pages/Chart.tsx`

Read `Chart.tsx` first, then make these targeted additions:

**Additions to the Chart page:**

1. **Imports** — add at top:
```tsx
import { useState, useEffect, useCallback } from 'react'
import { BarChart2 } from 'lucide-react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { fetchIndicators, calculateIndicator } from '../api/indicators'
import { IndicatorModal } from '../components/chart/IndicatorModal'
import { IndicatorChips } from '../components/chart/IndicatorChips'
import { PanelChart } from '../components/chart/PanelChart'
import type { ActiveIndicator, OHLCVCandle } from '../types'
```

2. **State** — add inside the Chart component:
```tsx
const [indicatorModalOpen, setIndicatorModalOpen] = useState(false)
const [activeIndicators, setActiveIndicators] = useState<ActiveIndicator[]>(() => {
  try {
    const stored = localStorage.getItem(`indicators:${symbol}:${timeframe}`)
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
})
```

3. **Persist to localStorage** — add after state:
```tsx
useEffect(() => {
  const persisted = activeIndicators.map(({ meta, params }) => ({ meta, params }))
  localStorage.setItem(`indicators:${symbol}:${timeframe}`, JSON.stringify(persisted))
}, [activeIndicators, symbol, timeframe])
```

4. **Calculate mutation** — add after state:
```tsx
const { mutateAsync: calcIndicator } = useMutation({ mutationFn: calculateIndicator })

// Re-calculate all active indicators when candles change
useEffect(() => {
  if (!candles || candles.length === 0 || activeIndicators.length === 0) return
  const candlePayload = candles.map((c: OHLCVCandle) => ({
    timestamp: c.timestamp,
    open: c.open, high: c.high, low: c.low, close: c.close, volume: c.volume,
  }))
  activeIndicators.forEach(async (ind, idx) => {
    try {
      const result = await calcIndicator({
        indicator_id: ind.meta.id,
        params: ind.params,
        candles: candlePayload,
      })
      setActiveIndicators((prev) =>
        prev.map((a, i) => (i === idx ? { ...a, result } : a))
      )
    } catch { /* silent: chart still works without indicators */ }
  })
}, [candles]) // eslint-disable-line react-hooks/exhaustive-deps
```

5. **Handle add/remove/edit**:
```tsx
const handleAddIndicator = useCallback(async (ind: ActiveIndicator) => {
  if (!candles || candles.length === 0) { setActiveIndicators((prev) => [...prev, ind]); return }
  const candlePayload = candles.map((c: OHLCVCandle) => ({
    timestamp: c.timestamp, open: c.open, high: c.high, low: c.low, close: c.close, volume: c.volume,
  }))
  try {
    const result = await calcIndicator({ indicator_id: ind.meta.id, params: ind.params, candles: candlePayload })
    setActiveIndicators((prev) => [...prev, { ...ind, result }])
  } catch {
    setActiveIndicators((prev) => [...prev, ind])
  }
}, [candles, calcIndicator])

const handleRemoveIndicator = useCallback((id: string) => {
  setActiveIndicators((prev) => prev.filter((a) => a.meta.id !== id))
}, [])
```

6. **Overlay series on the main chart** — add inside the chart `useEffect` (after `candleSeries.setData(...)`) or in a separate effect watching `activeIndicators`:
```tsx
// Keep a ref to active overlay series so we can remove them on cleanup
const overlaySeriesRef = useRef<ISeriesApi<'Line'>[]>([])

useEffect(() => {
  const chart = chartRef.current
  if (!chart) return
  // Remove previous overlay series
  overlaySeriesRef.current.forEach((s) => { try { chart.removeSeries(s) } catch {} })
  overlaySeriesRef.current = []

  activeIndicators
    .filter((ind) => ind.meta.type === 'overlay' && ind.result)
    .forEach((ind) => {
      ind.meta.outputs.forEach((output) => {
        const series = ind.result!.series[output.name]
        if (!series) return
        const s = chart.addLineSeries({ color: output.color, lineWidth: 1 })
        const points = ind.result!.timestamps
          .map((ts, i) => ({ time: ts as unknown as Time, value: series[i] }))
          .filter((p) => p.value !== null && p.value !== undefined) as Array<{ time: Time; value: number }>
        s.setData(points)
        overlaySeriesRef.current.push(s)
      })
    })
}, [activeIndicators])
```

7. **Toolbar** — add "Indicators" button and chips next to the existing timeframe buttons:
```tsx
<button
  onClick={() => setIndicatorModalOpen(true)}
  className="flex items-center gap-1.5 px-3 py-1.5 rounded text-sm border hover:bg-accent"
>
  <BarChart2 className="w-4 h-4" />
  Indicators
</button>
<IndicatorChips
  indicators={activeIndicators}
  onRemove={handleRemoveIndicator}
  onEdit={(ind) => { /* TODO: open modal pre-selected */ }}
/>
```

8. **Panel charts** — add below the main chart container:
```tsx
{activeIndicators
  .filter((ind) => ind.meta.type === 'panel')
  .map((ind) => (
    <PanelChart
      key={ind.meta.id}
      indicator={ind}
      onClose={() => handleRemoveIndicator(ind.meta.id)}
      isDark={isDark}
    />
  ))
}
```

9. **Modal** — add at end of JSX:
```tsx
<IndicatorModal
  open={indicatorModalOpen}
  onClose={() => setIndicatorModalOpen(false)}
  active={activeIndicators}
  onAdd={handleAddIndicator}
/>
```

**Commit:**
```bash
git add frontend/src/pages/Chart.tsx
git commit -m "feat(phase5): wire indicator overlay + panel rendering into Chart page"
```

---

### Task 8 — Bollinger Bands + Ichimoku Special Rendering + Smoke Test

**Files:**
- Modify: `frontend/src/pages/Chart.tsx`

Bollinger Bands and Ichimoku need special rendering in the overlay effect (Task 7's overlay `useEffect`).

**BBands area fill** — after setting all 3 line series, add a filled area series between upper and lower using lightweight-charts `addAreaSeries` with custom high/low line data. The simplest approach that works without a custom plugin is to render 3 line series (upper, middle, lower) at different opacity levels — the fill is optional cosmetic polish and can be done with CSS overlay tricks or skipped in V1. For V1, 3 coloured lines is sufficient and correct.

**Ichimoku cloud fill** — same reasoning. Render all 5 lines as `LineSeries`. Cloud coloring (A vs B fill) requires a custom series plugin. Defer to a follow-up; 5 lines deliver the signal information.

Add a special case for volume (histogram with up/down colouring):
```tsx
// In the overlay effect, for volume panel indicator:
if (ind.meta.id === 'volume' && ind.result) {
  const candles = ... // access from outer state
  const points = ind.result.timestamps.map((ts, i) => ({
    time: ts as unknown as Time,
    value: ind.result!.series['value'][i] as number,
    color: candles[i]?.close >= candles[i]?.open ? '#26A69A' : '#EF5350',
  })).filter(p => p.value !== null)
  // pass to PanelChart via prop or handle inside PanelChart
}
```

For clean separation: pass `rawCandles` as an optional prop to `PanelChart` so it can colour volume bars.

**Smoke test — manual verification checklist:**
```
1. Start backend: cd backend && go run cmd/server/main.go
2. Start frontend: cd frontend && npm run dev
3. Navigate to Chart page
4. Click "Indicators" button → modal opens
5. Search "EMA" → select → set period=20 → click "Add to Chart"
   Expected: EMA chip appears in toolbar, orange line appears on main chart
6. Add "RSI" → panel appears below chart
7. Add "Bollinger Bands" → three lines appear on main chart
8. Add "MACD" → second panel appears
9. Remove EMA chip → line disappears
10. Reload page → indicators restored from localStorage
11. Switch symbol → indicators recalculate for new candles
```

**Final commit:**
```bash
git add frontend/src/pages/Chart.tsx frontend/src/components/chart/PanelChart.tsx
git commit -m "feat(phase5): add Bollinger Bands multi-series and volume histogram colouring"
```

---

### Task 9 — Run Full Test Suite + Final Commit

**Step 1: Backend tests**
```bash
cd backend && go test ./... -v 2>&1 | tail -30
```
Expected: all PASS, 0 FAIL.

**Step 2: Frontend lint**
```bash
cd frontend && npm run lint
```
Expected: 0 errors.

**Step 3: Update phases.md**

In `.claude/docs/phases.md`, change Phase 5 status lines from `🔲` to `✅` and add `✅ COMPLETE` to the Phase 5 header.

**Step 4: Final commit**
```bash
git add .claude/docs/phases.md
git commit -m "docs: mark Phase 5 technical indicators complete"
```

---

## Summary of Files Changed

| File | Action |
|------|--------|
| `backend/internal/indicator/types.go` | Create |
| `backend/internal/indicator/overlay.go` | Create (Worktree A) |
| `backend/internal/indicator/overlay_test.go` | Create (Worktree A) |
| `backend/internal/indicator/panel.go` | Create (Worktree B) |
| `backend/internal/indicator/panel_test.go` | Create (Worktree B) |
| `backend/internal/indicator/registry.go` | Create (Wave 2) |
| `backend/internal/indicator/handler.go` | Create (Wave 2) |
| `backend/internal/indicator/handler_test.go` | Create (Wave 2) |
| `backend/internal/api/routes.go` | Modify (Wave 2) |
| `frontend/src/types/index.ts` | Modify (Wave 2) |
| `frontend/src/api/indicators.ts` | Create (Wave 2) |
| `frontend/src/components/chart/IndicatorModal.tsx` | Create (Wave 2) |
| `frontend/src/components/chart/IndicatorParamForm.tsx` | Create (Wave 2) |
| `frontend/src/components/chart/IndicatorChips.tsx` | Create (Wave 2) |
| `frontend/src/components/chart/PanelChart.tsx` | Create (Wave 2) |
| `frontend/src/pages/Chart.tsx` | Modify (Wave 2) |
| `.claude/docs/phases.md` | Modify (Wave 2) |
