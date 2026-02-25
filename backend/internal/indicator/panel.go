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

// Stochastic computes Fast Stochastic Oscillator (%K and %D).
// params: k_period int (14), d_period int (3), smooth int (3)
// Outputs: "k", "d"
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

	// %K smoothed = SMA(rawK, smooth)
	kVals := nanSlice(n)
	for i := kPeriod + smooth - 2; i < n; i++ {
		sum, valid := 0.0, true
		for j := i - smooth + 1; j <= i; j++ {
			if math.IsNaN(rawK[j]) {
				valid = false
				break
			}
			sum += rawK[j]
		}
		if valid {
			kVals[i] = sum / float64(smooth)
		}
	}

	// %D = SMA(%K, dPeriod)
	dVals := nanSlice(n)
	for i := kPeriod + smooth + dPeriod - 3; i < n; i++ {
		sum, count := 0.0, 0
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
// Output: "value". First `period` values are NaN.
func ATR(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 14)
	if err != nil {
		return CalcResult{}, err
	}
	if period < 1 {
		return CalcResult{}, fmt.Errorf("period must be >= 1")
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

	// Need at least period+1 candles for first ATR
	if n < period+1 {
		return CalcResult{Timestamps: timestamps(candles), Series: map[string][]float64{"value": vals}}, nil
	}

	// Seed: simple average of first `period` TRs
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
