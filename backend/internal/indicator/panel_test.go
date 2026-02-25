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
			t.Errorf("expected NaN at index %d, got %f", i, res.Series["value"][i])
		}
	}
	// After warm-up: RSI = 100 (no losses)
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
	// With rising prices, MACD line should be > 0 once slow EMA is warmed up
	for i := 26; i < 40; i++ {
		if !math.IsNaN(res.Series["macd"][i]) && res.Series["macd"][i] < 0 {
			t.Errorf("MACD[%d]=%f, expected >= 0 for rising prices", i, res.Series["macd"][i])
		}
	}
}
