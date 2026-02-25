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
		if x != math.Trunc(x) {
			return def, fmt.Errorf("param %q must be an integer, got %v", key, x)
		}
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

// BollingerBands computes BB with middle (SMA), upper, and lower bands.
// params: period int (default 20), std_dev float (default 2.0)
// Outputs: "upper", "middle", "lower"
func BollingerBands(candles []registry.Candle, params map[string]interface{}) (CalcResult, error) {
	period, err := intParam(params, "period", 20)
	if err != nil {
		return CalcResult{}, err
	}
	stdMult, err := floatParam(params, "std_dev", 2.0)
	if err != nil {
		return CalcResult{}, err
	}
	if period < 1 {
		return CalcResult{}, fmt.Errorf("period must be >= 1")
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

// ParabolicSAR computes Parabolic Stop-And-Reverse.
// params: step float (default 0.02), max float (default 0.2)
// Output: "value". Index 0 is NaN; index 1 onward is finite.
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

	// Initial trend: bullish if close[1] > close[0]
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
// All series have length == len(candles).
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
		if i >= tenkan-1 {
			hi, lo := highLow(candles, i-tenkan+1, i)
			tenkanVals[i] = (hi + lo) / 2
		}
		if i >= kijun-1 {
			hi, lo := highLow(candles, i-kijun+1, i)
			kijunVals[i] = (hi + lo) / 2
		}
		// Senkou A = (tenkan + kijun) / 2, plotted disp periods ahead
		if i >= kijun-1 {
			t := tenkanVals[i]
			k := kijunVals[i]
			if !math.IsNaN(t) && !math.IsNaN(k) {
				target := i + disp
				if target < n {
					senkouAVals[target] = (t + k) / 2
				}
			}
		}
		// Senkou B, plotted disp periods ahead
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
