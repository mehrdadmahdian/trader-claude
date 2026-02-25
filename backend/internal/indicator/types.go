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
	ID       string                     `json:"id"`
	Name     string                     `json:"name"`
	FullName string                     `json:"full_name"`
	Type     string                     `json:"type"`    // "overlay" | "panel"
	Group    string                     `json:"group"`   // "trend" | "momentum" | "volatility" | "volume"
	Params   []registry.ParamDefinition `json:"params"`
	Outputs  []OutputDef                `json:"outputs"`
}

// OutputDef describes one named series within a CalcResult.
type OutputDef struct {
	Name  string `json:"name"`
	Color string `json:"color"` // default hex colour, e.g. "#2962FF"
}
