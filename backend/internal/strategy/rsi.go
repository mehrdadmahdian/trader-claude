package strategy

import (
	"github.com/trader-claude/backend/internal/registry"
)

// RSIStrategy implements Wilder's Relative Strength Index strategy.
// A long signal fires when RSI crosses below the oversold threshold (enters oversold zone),
// a short signal fires when RSI crosses above the overbought threshold (enters overbought zone).
type RSIStrategy struct {
	// Parameters
	period         int
	overbought     float64
	oversold       float64
	useDivergence  bool

	// Internal Wilder smoothing state
	prevClose  float64
	avgGain    float64
	avgLoss    float64
	prevRSI    float64
	warmup     int  // counts processed candles
	seeded     bool // true once the initial SMA seed is computed
}

// Name returns the unique strategy identifier.
func (r *RSIStrategy) Name() string { return "rsi" }

// Description returns a human-readable description.
func (r *RSIStrategy) Description() string {
	return "RSI strategy: long when RSI crosses below oversold threshold, short when RSI crosses above overbought threshold."
}

// Params returns the list of configurable parameters.
func (r *RSIStrategy) Params() []registry.ParamDefinition {
	return []registry.ParamDefinition{
		{
			Name:        "period",
			Type:        "int",
			Default:     14,
			Min:         2,
			Max:         100,
			Description: "RSI period (Wilder smoothing)",
			Required:    false,
		},
		{
			Name:        "overbought",
			Type:        "float",
			Default:     70.0,
			Min:         50.0,
			Max:         100.0,
			Description: "RSI level considered overbought",
			Required:    false,
		},
		{
			Name:        "oversold",
			Type:        "float",
			Default:     30.0,
			Min:         0.0,
			Max:         50.0,
			Description: "RSI level considered oversold",
			Required:    false,
		},
		{
			Name:        "use_divergence",
			Type:        "bool",
			Default:     false,
			Description: "Enable divergence detection (not yet implemented)",
			Required:    false,
		},
	}
}

// Init initialises the strategy with the provided parameters and resets state.
func (r *RSIStrategy) Init(params map[string]interface{}) error {
	r.period = paramInt(params, "period", 14)
	r.overbought = paramFloat(params, "overbought", 70.0)
	r.oversold = paramFloat(params, "oversold", 30.0)
	r.useDivergence = paramBool(params, "use_divergence", false)

	r.Reset()
	return nil
}

// Reset clears all internal running state without changing parameters.
func (r *RSIStrategy) Reset() {
	r.prevClose = 0
	r.avgGain = 0
	r.avgLoss = 0
	r.prevRSI = 50
	r.warmup = 0
	r.seeded = false
}

// rsi computes the RSI value from the current avgGain / avgLoss state.
func (r *RSIStrategy) rsi() float64 {
	if r.avgLoss == 0 {
		if r.avgGain == 0 {
			return 50
		}
		return 100
	}
	rs := r.avgGain / r.avgLoss
	return 100 - (100 / (1 + rs))
}

// OnCandle processes a new candle and optionally returns a signal.
func (r *RSIStrategy) OnCandle(candle registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	price := candle.Close
	r.warmup++

	if r.warmup == 1 {
		// First candle: just store the close, no change to compute.
		r.prevClose = price
		return nil, nil
	}

	change := price - r.prevClose
	gain := 0.0
	loss := 0.0
	if change > 0 {
		gain = change
	} else {
		loss = -change
	}

	if !r.seeded {
		// Accumulate for the initial SMA seed (need period changes = period+1 candles).
		r.avgGain += gain
		r.avgLoss += loss

		if r.warmup == r.period+1 {
			// Compute initial SMA averages.
			r.avgGain /= float64(r.period)
			r.avgLoss /= float64(r.period)
			r.seeded = true
			r.prevRSI = r.rsi()
		}

		r.prevClose = price
		return nil, nil
	}

	// Wilder's smoothing.
	r.avgGain = (r.avgGain*float64(r.period-1) + gain) / float64(r.period)
	r.avgLoss = (r.avgLoss*float64(r.period-1) + loss) / float64(r.period)

	currentRSI := r.rsi()
	prevRSI := r.prevRSI

	r.prevRSI = currentRSI
	r.prevClose = price

	// Long signal: RSI crosses below oversold (enters oversold zone going down).
	// prevRSI was at or above the oversold level; currentRSI dropped below it.
	if prevRSI >= r.oversold && currentRSI < r.oversold {
		strength := clamp(1.0-currentRSI/100.0, 0.01, 1.0)
		return &registry.Signal{
			Symbol:    candle.Symbol,
			Market:    candle.Market,
			Direction: "long",
			Strength:  strength,
			Price:     candle.Close,
			Timestamp: candle.Timestamp,
			Metadata: map[string]interface{}{
				"rsi": currentRSI,
			},
		}, nil
	}

	// Short signal: RSI crosses above overbought (enters overbought zone going up).
	// prevRSI was at or below the overbought level; currentRSI rose above it.
	if prevRSI <= r.overbought && currentRSI > r.overbought {
		strength := clamp(currentRSI/100.0, 0.01, 1.0)
		return &registry.Signal{
			Symbol:    candle.Symbol,
			Market:    candle.Market,
			Direction: "short",
			Strength:  strength,
			Price:     candle.Close,
			Timestamp: candle.Timestamp,
			Metadata: map[string]interface{}{
				"rsi": currentRSI,
			},
		}, nil
	}

	return nil, nil
}

// OnTick returns nil — this strategy does not operate on ticks.
func (r *RSIStrategy) OnTick(_ registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}

// clamp returns v clamped to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
