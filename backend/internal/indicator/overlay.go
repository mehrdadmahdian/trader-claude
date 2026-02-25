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
