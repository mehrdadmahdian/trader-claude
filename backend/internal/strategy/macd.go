package strategy

import (
	"math"

	"github.com/trader-claude/backend/internal/registry"
)

// MACDSignal implements the Moving Average Convergence/Divergence strategy.
//
// MACD line     = EMA(close, fast) - EMA(close, slow)
// Signal line   = EMA(MACD line, signal)
// Histogram     = MACD line - signal line
//
// A long signal fires when MACD crosses above the signal line (and
// abs(histogram) > histogram_threshold). A short signal fires on the reverse.
type MACDSignal struct {
	// Parameters
	fastPeriod         int
	slowPeriod         int
	signalPeriod       int
	histogramThreshold float64

	// Internal EMA state
	fastEMA    float64
	slowEMA    float64
	signalEMA  float64

	// Cross detection
	prevMACD   float64
	prevSignal float64

	// Warmup tracking
	warmup      int  // candles processed
	macdWarmup  int  // candles processed after MACD can first be calculated
	sigSeeded   bool // true once signal EMA is warmed up
}

// Name returns the unique strategy identifier.
func (m *MACDSignal) Name() string { return "macd" }

// Description returns a human-readable description.
func (m *MACDSignal) Description() string {
	return "MACD strategy: long on bullish MACD/signal cross, short on bearish cross."
}

// Params returns the list of configurable parameters.
func (m *MACDSignal) Params() []registry.ParamDefinition {
	return []registry.ParamDefinition{
		{
			Name:        "fast",
			Type:        "int",
			Default:     12,
			Min:         2,
			Max:         100,
			Description: "Fast EMA period for MACD line",
			Required:    false,
		},
		{
			Name:        "slow",
			Type:        "int",
			Default:     26,
			Min:         5,
			Max:         200,
			Description: "Slow EMA period for MACD line",
			Required:    false,
		},
		{
			Name:        "signal",
			Type:        "int",
			Default:     9,
			Min:         2,
			Max:         50,
			Description: "Signal EMA period",
			Required:    false,
		},
		{
			Name:        "histogram_threshold",
			Type:        "float",
			Default:     0.0,
			Min:         -1.0,
			Max:         1.0,
			Description: "Minimum absolute histogram value required to fire a signal",
			Required:    false,
		},
	}
}

// Init initialises the strategy with the provided parameters and resets state.
func (m *MACDSignal) Init(params map[string]interface{}) error {
	m.fastPeriod = paramInt(params, "fast", 12)
	m.slowPeriod = paramInt(params, "slow", 26)
	m.signalPeriod = paramInt(params, "signal", 9)
	m.histogramThreshold = paramFloat(params, "histogram_threshold", 0.0)

	m.Reset()
	return nil
}

// Reset clears all internal running state without changing parameters.
func (m *MACDSignal) Reset() {
	m.fastEMA = 0
	m.slowEMA = 0
	m.signalEMA = 0
	m.prevMACD = 0
	m.prevSignal = 0
	m.warmup = 0
	m.macdWarmup = 0
	m.sigSeeded = false
}

// OnCandle processes a new candle and optionally returns a signal.
func (m *MACDSignal) OnCandle(candle registry.Candle, _ *registry.StrategyState) (*registry.Signal, error) {
	price := candle.Close
	m.warmup++

	kFast := 2.0 / float64(m.fastPeriod+1)
	kSlow := 2.0 / float64(m.slowPeriod+1)
	kSig := 2.0 / float64(m.signalPeriod+1)

	if m.warmup == 1 {
		m.fastEMA = price
		m.slowEMA = price
		return nil, nil
	}

	// Update fast and slow EMAs continuously.
	m.fastEMA = price*kFast + m.fastEMA*(1-kFast)
	m.slowEMA = price*kSlow + m.slowEMA*(1-kSlow)

	// Only consider slow EMA warmed up after slowPeriod candles.
	if m.warmup < m.slowPeriod {
		return nil, nil
	}

	macdLine := m.fastEMA - m.slowEMA
	m.macdWarmup++

	if m.macdWarmup == 1 {
		// Seed the signal EMA with the first MACD value.
		m.signalEMA = macdLine
		m.prevMACD = macdLine
		m.prevSignal = macdLine
		return nil, nil
	}

	// Update signal EMA.
	m.signalEMA = macdLine*kSig + m.signalEMA*(1-kSig)

	// Only fire signals once signal EMA has warmed up.
	if m.macdWarmup < m.signalPeriod {
		m.prevMACD = macdLine
		m.prevSignal = m.signalEMA
		return nil, nil
	}

	if !m.sigSeeded {
		m.sigSeeded = true
		// On the first fully warmed-up candle, record state and skip signal.
		m.prevMACD = macdLine
		m.prevSignal = m.signalEMA
		return nil, nil
	}

	prevMACD := m.prevMACD
	prevSig := m.prevSignal

	histogram := macdLine - m.signalEMA

	m.prevMACD = macdLine
	m.prevSignal = m.signalEMA

	// Check histogram threshold.
	if math.Abs(histogram) <= m.histogramThreshold {
		return nil, nil
	}

	// Detect MACD/signal line cross.
	wasMACDBelow := prevMACD <= prevSig
	isMACDAbove := macdLine > m.signalEMA

	wasMACDAbove := prevMACD >= prevSig
	isMACDBelow := macdLine < m.signalEMA

	var direction string
	switch {
	case wasMACDBelow && isMACDAbove:
		direction = "long"
	case wasMACDAbove && isMACDBelow:
		direction = "short"
	default:
		return nil, nil
	}

	strength := clamp(math.Abs(histogram)/math.Abs(macdLine+1e-10), 0.01, 1.0)

	return &registry.Signal{
		Symbol:    candle.Symbol,
		Market:    candle.Market,
		Direction: direction,
		Strength:  strength,
		Price:     candle.Close,
		Timestamp: candle.Timestamp,
		Metadata: map[string]interface{}{
			"macd":      macdLine,
			"signal":    m.signalEMA,
			"histogram": histogram,
		},
	}, nil
}

// OnTick returns nil — this strategy does not operate on ticks.
func (m *MACDSignal) OnTick(_ registry.Tick, _ *registry.StrategyState) (*registry.Signal, error) {
	return nil, nil
}
