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
	p := func(name, typ string, def interface{}, min, max interface{}, desc string, required bool) registry.ParamDefinition {
		return registry.ParamDefinition{Name: name, Type: typ, Default: def, Min: min, Max: max, Description: desc, Required: required}
	}
	o := func(name, color string) OutputDef { return OutputDef{Name: name, Color: color} }

	catalog = []entry{
		// ── Overlay — Trend ──────────────────────────────────────────────────
		{IndicatorMeta{"sma", "SMA", "Simple Moving Average", "overlay", "trend",
			[]registry.ParamDefinition{p("period", "int", 20, 2, 500, "Lookback period", true)},
			[]OutputDef{o("value", "#2962FF")}}, SMA},
		{IndicatorMeta{"ema", "EMA", "Exponential Moving Average", "overlay", "trend",
			[]registry.ParamDefinition{p("period", "int", 20, 2, 500, "Lookback period", true)},
			[]OutputDef{o("value", "#FF6D00")}}, EMA},
		{IndicatorMeta{"wma", "WMA", "Weighted Moving Average", "overlay", "trend",
			[]registry.ParamDefinition{p("period", "int", 20, 2, 500, "Lookback period", true)},
			[]OutputDef{o("value", "#6200EA")}}, WMA},
		{IndicatorMeta{"bollinger_bands", "BB", "Bollinger Bands", "overlay", "volatility",
			[]registry.ParamDefinition{
				p("period", "int", 20, 2, 500, "SMA period", true),
				p("std_dev", "float", 2.0, 0.1, 10.0, "Standard deviation multiplier", true),
			},
			[]OutputDef{o("upper", "#F44336"), o("middle", "#2962FF"), o("lower", "#4CAF50")}}, BollingerBands},
		{IndicatorMeta{"vwap", "VWAP", "Volume-Weighted Average Price", "overlay", "trend",
			nil,
			[]OutputDef{o("value", "#E91E63")}}, VWAP},
		{IndicatorMeta{"parabolic_sar", "SAR", "Parabolic SAR", "overlay", "trend",
			[]registry.ParamDefinition{
				p("step", "float", 0.02, 0.001, 0.5, "Acceleration step", true),
				p("max", "float", 0.2, 0.01, 1.0, "Maximum acceleration", true),
			},
			[]OutputDef{o("value", "#FF9800")}}, ParabolicSAR},
		{IndicatorMeta{"ichimoku", "Ichimoku", "Ichimoku Cloud", "overlay", "trend",
			[]registry.ParamDefinition{
				p("tenkan", "int", 9, 2, 100, "Tenkan-sen period", true),
				p("kijun", "int", 26, 2, 200, "Kijun-sen period", true),
				p("senkou_b", "int", 52, 2, 500, "Senkou Span B period", true),
				p("displacement", "int", 26, 1, 100, "Cloud displacement", true),
			},
			[]OutputDef{
				o("tenkan", "#E91E63"), o("kijun", "#2962FF"),
				o("senkou_a", "#4CAF50"), o("senkou_b", "#F44336"), o("chikou", "#9C27B0"),
			}}, Ichimoku},
		// ── Panel — Momentum ─────────────────────────────────────────────────
		{IndicatorMeta{"rsi", "RSI", "Relative Strength Index", "panel", "momentum",
			[]registry.ParamDefinition{p("period", "int", 14, 2, 200, "Lookback period", true)},
			[]OutputDef{o("value", "#7B1FA2")}}, RSI},
		{IndicatorMeta{"macd", "MACD", "MACD", "panel", "momentum",
			[]registry.ParamDefinition{
				p("fast", "int", 12, 2, 200, "Fast EMA period", true),
				p("slow", "int", 26, 2, 500, "Slow EMA period", true),
				p("signal", "int", 9, 2, 200, "Signal EMA period", true),
			},
			[]OutputDef{o("macd", "#2962FF"), o("signal", "#FF6D00"), o("histogram", "#26A69A")}}, MACD},
		{IndicatorMeta{"stochastic", "Stoch", "Stochastic Oscillator", "panel", "momentum",
			[]registry.ParamDefinition{
				p("k_period", "int", 14, 2, 200, "%K lookback period", true),
				p("d_period", "int", 3, 1, 50, "%D smoothing period", true),
				p("smooth", "int", 3, 1, 50, "%K smoothing period", true),
			},
			[]OutputDef{o("k", "#2962FF"), o("d", "#FF6D00")}}, Stochastic},
		// ── Panel — Volatility ───────────────────────────────────────────────
		{IndicatorMeta{"atr", "ATR", "Average True Range", "panel", "volatility",
			[]registry.ParamDefinition{p("period", "int", 14, 2, 200, "Lookback period", true)},
			[]OutputDef{o("value", "#FF6D00")}}, ATR},
		// ── Panel — Volume ───────────────────────────────────────────────────
		{IndicatorMeta{"obv", "OBV", "On-Balance Volume", "panel", "volume",
			nil,
			[]OutputDef{o("value", "#2962FF")}}, OBV},
		{IndicatorMeta{"volume", "Volume", "Volume", "panel", "volume",
			nil,
			[]OutputDef{o("value", "#26A69A")}}, Volume},
	}
}

// All returns the full catalog metadata.
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
