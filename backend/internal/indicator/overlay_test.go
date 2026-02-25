package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/trader-claude/backend/internal/registry"
)

// makeCandles builds a minimal candle slice from close prices.
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
			t.Errorf("SMA[%d] got %f, want %f", i, res.Series["value"][i], w)
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
	// WMA[4] = (1*3 + 2*4 + 3*5)/6 = (3+8+15)/6 = 26/6
	if !approxEq(res.Series["value"][4], 26.0/6.0, 0.001) {
		t.Errorf("WMA[4] got %f, want %f", res.Series["value"][4], 26.0/6.0)
	}
}

func TestSMAEdgeCases(t *testing.T) {
	// period=0 must error
	_, err := SMA(makeCandles([]float64{1, 2, 3}), map[string]interface{}{"period": 0})
	if err == nil {
		t.Error("expected error for period=0, got nil")
	}

	// period=1: every output equals its close
	res, err := SMA(makeCandles([]float64{1, 2, 3}), map[string]interface{}{"period": 1})
	if err != nil {
		t.Fatalf("unexpected error for period=1: %v", err)
	}
	for i, want := range []float64{1, 2, 3} {
		if !approxEq(res.Series["value"][i], want, 0.001) {
			t.Errorf("SMA period=1 [%d] got %f, want %f", i, res.Series["value"][i], want)
		}
	}

	// n=0: must return empty result without panic
	res, err = SMA(makeCandles(nil), map[string]interface{}{"period": 3})
	if err != nil {
		t.Fatalf("unexpected error for n=0: %v", err)
	}
	if len(res.Series["value"]) != 0 {
		t.Errorf("expected empty series for n=0, got %d elements", len(res.Series["value"]))
	}
}

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
	// index 2 onwards: all equal to 2.0
	for i := 2; i < 5; i++ {
		if !approxEq(res.Series["middle"][i], 2.0, 0.001) {
			t.Errorf("middle[%d]=%f, want 2.0", i, res.Series["middle"][i])
		}
		if !approxEq(res.Series["upper"][i], 2.0, 0.001) {
			t.Errorf("upper[%d]=%f, want 2.0", i, res.Series["upper"][i])
		}
		if !approxEq(res.Series["lower"][i], 2.0, 0.001) {
			t.Errorf("lower[%d]=%f, want 2.0", i, res.Series["lower"][i])
		}
	}
}

func TestBollingerBandsSpread(t *testing.T) {
	// period=3, std_dev=2 on [1,2,3]
	// mean=2.0, E[X²]=(1+4+9)/3=14/3, variance=14/3-4=2/3, std=sqrt(2/3)≈0.8165
	// upper = 2.0 + 2*0.8165 ≈ 3.633, lower ≈ 0.367
	candles := makeCandles([]float64{1, 2, 3})
	res, err := BollingerBands(candles, map[string]interface{}{"period": 3, "std_dev": 2.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	std := math.Sqrt(2.0 / 3.0)
	wantUpper := 2.0 + 2*std
	wantLower := 2.0 - 2*std
	if !approxEq(res.Series["upper"][2], wantUpper, 0.001) {
		t.Errorf("upper[2] got %f, want %f", res.Series["upper"][2], wantUpper)
	}
	if !approxEq(res.Series["middle"][2], 2.0, 0.001) {
		t.Errorf("middle[2] got %f, want 2.0", res.Series["middle"][2])
	}
	if !approxEq(res.Series["lower"][2], wantLower, 0.001) {
		t.Errorf("lower[2] got %f, want %f", res.Series["lower"][2], wantLower)
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
