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
}
