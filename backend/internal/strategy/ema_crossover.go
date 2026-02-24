package strategy

import (
	"fmt"
	"math"

	"github.com/trader-claude/backend/internal/registry"
)

// EMACrossover implements a classic dual-EMA crossover strategy.
// A long signal is emitted when the fast EMA crosses above the slow EMA and
// a short signal when it crosses below.
type EMACrossover struct {
	// Parameters
	fastPeriod    int
	slowPeriod    int
	signalOnClose bool

	// Internal state — rolling close prices and computed EMAs
	closes   []float64
	fastEMA  float64
	slowEMA  float64
	prevFast float64
	prevSlow float64
	warmup   int // counts how many candles have been processed
}

// Name returns the unique strategy identifier.
func (e *EMACrossover) Name() string { return "ema_crossover" }

// Description returns a human-readable description.
func (e *EMACrossover) Description() string {
	return "Dual EMA crossover: long when fast EMA crosses above slow EMA, short when it crosses below."
}

// Params returns the list of configurable parameters.
func (e *EMACrossover) Params() []registry.ParamDefinition {
	return []registry.ParamDefinition{
		{
			Name:        "fast_period",
			Type:        "int",
			Default:     9,
			Min:         2,
			Max:         200,
			Description: "Fast EMA period",
			Required:    false,
		},
		{
			Name:        "slow_period",
			Type:        "int",
			Default:     21,
			Min:         5,
			Max:         500,
			Description: "Slow EMA period",
			Required:    false,
		},
		{
			Name:        "signal_on_close",
			Type:        "bool",
			Default:     true,
			Description: "Generate signal on candle close",
			Required:    false,
		},
	}
}

// Init initialises the strategy with the provided parameters and resets state.
func (e *EMACrossover) Init(params map[string]interface{}) error {
	e.fastPeriod = paramInt(params, "fast_period", 9)
	e.slowPeriod = paramInt(params, "slow_period", 21)
	e.signalOnClose = paramBool(params, "signal_on_close", true)

	if e.fastPeriod >= e.slowPeriod {
		return fmt.Errorf("fast_period (%d) must be less than slow_period (%d)", e.fastPeriod, e.slowPeriod)
	}

	e.Reset()
	return nil
}

// Reset clears all internal running state without changing parameters.
func (e *EMACrossover) Reset() {
	e.closes = nil
	e.fastEMA = 0
	e.slowEMA = 0
	e.prevFast = 0
	e.prevSlow = 0
	e.warmup = 0
}

// OnCandle processes a new candle and optionally returns a signal.
func (e *EMACrossover) OnCandle(candle registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	price := candle.Close
	if !e.signalOnClose {
		price = candle.Open
	}
	e.closes = append(e.closes, price)
	e.warmup++

	kFast := 2.0 / float64(e.fastPeriod+1)
	kSlow := 2.0 / float64(e.slowPeriod+1)

	switch {
	case e.warmup == 1:
		// Seed both EMAs with the first price.
		e.fastEMA = price
		e.slowEMA = price
		e.prevFast = price
		e.prevSlow = price
		return nil, nil

	case e.warmup < e.slowPeriod:
		// During warmup accumulate EMA without emitting signals.
		e.fastEMA = price*kFast + e.fastEMA*(1-kFast)
		e.slowEMA = price*kSlow + e.slowEMA*(1-kSlow)
		e.prevFast = e.fastEMA
		e.prevSlow = e.slowEMA
		return nil, nil

	case e.warmup == e.slowPeriod:
		// Boundary candle: seed EMAs once and return without signal.
		e.fastEMA = price*kFast + e.fastEMA*(1-kFast)
		e.slowEMA = price*kSlow + e.slowEMA*(1-kSlow)
		e.prevFast = e.fastEMA
		e.prevSlow = e.slowEMA
		return nil, nil
	}

	// From here e.warmup >= slowPeriod.
	prevFast := e.fastEMA
	prevSlow := e.slowEMA

	e.fastEMA = price*kFast + e.fastEMA*(1-kFast)
	e.slowEMA = price*kSlow + e.slowEMA*(1-kSlow)

	// Detect cross.
	wasFastBelow := prevFast <= prevSlow
	isFastAbove := e.fastEMA > e.slowEMA

	wasFastAbove := prevFast >= prevSlow
	isFastBelow := e.fastEMA < e.slowEMA

	var direction string
	switch {
	case wasFastBelow && isFastAbove:
		direction = "long"
	case wasFastAbove && isFastBelow:
		direction = "short"
	default:
		return nil, nil
	}

	strength := math.Abs(e.fastEMA-e.slowEMA) / e.slowEMA
	if strength > 1 {
		strength = 1
	}
	if strength <= 0 {
		strength = 0.01
	}

	return &registry.Signal{
		Symbol:    candle.Symbol,
		Market:    candle.Market,
		Direction: direction,
		Strength:  strength,
		Price:     candle.Close,
		Timestamp: candle.Timestamp,
		Metadata: map[string]interface{}{
			"fast_ema": e.fastEMA,
			"slow_ema": e.slowEMA,
		},
	}, nil
}

// OnTick returns nil — this strategy does not operate on ticks.
func (e *EMACrossover) OnTick(_ registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}

// --- param helpers ---

// paramInt extracts an int parameter from params, with a fallback default.
// Accepts both int and float64 values (JSON unmarshalling uses float64).
func paramInt(params map[string]interface{}, key string, def int) int {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	}
	return def
}

// paramFloat extracts a float64 parameter from params, with a fallback default.
func paramFloat(params map[string]interface{}, key string, def float64) float64 {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	}
	return def
}

// paramBool extracts a bool parameter from params, with a fallback default.
func paramBool(params map[string]interface{}, key string, def bool) bool {
	v, ok := params[key]
	if !ok {
		return def
	}
	if val, ok := v.(bool); ok {
		return val
	}
	return def
}

// calcEMA computes EMA over a slice using the standard formula.
// Returns the final EMA value.
func calcEMA(prices []float64, period int) float64 {
	if len(prices) == 0 || period <= 0 {
		return 0
	}
	k := 2.0 / float64(period+1)
	ema := prices[0]
	for _, p := range prices[1:] {
		ema = p*k + ema*(1-k)
	}
	return ema
}

// ensure calcEMA is referenced (used by MACD)
var _ = calcEMA
