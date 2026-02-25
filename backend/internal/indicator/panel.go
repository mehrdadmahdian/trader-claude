package indicator

import (
	"fmt"
	"math"

	"github.com/trader-claude/backend/internal/registry"
)

// panelEMA is an internal EMA helper for panel calculations.
// Returns parallel array; first (period-1) values are NaN.
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
// Output: "value" (0–100 scale). First `period` values are NaN.
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
	// RSI needs `period` price-changes, requiring at least period+1 candles.
	// Use <= (not <) to guard the seed loop that accesses candles[period].
	if n <= period {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}

	// Seed: average gain/loss over first `period` changes
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

	// Wilder smoothing for subsequent candles
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

	signalLine := nanSlice(n)
	histogram := nanSlice(n)

	start := slow - 1
	if start >= n || start+sig-1 >= n {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{
			"macd": macdLine, "signal": signalLine, "histogram": histogram,
		}}, nil
	}

	// Seed signal EMA from first `sig` MACD values
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

	return CalcResult{
		Timestamps: timestamps(candles),
		Series:     map[string][]float64{"macd": macdLine, "signal": signalLine, "histogram": histogram},
	}, nil
}
