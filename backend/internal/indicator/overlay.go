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
