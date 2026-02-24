// Package backtest provides a single-asset backtesting engine that replays
// historical candles through a registry.Strategy, tracks positions, and
// computes a full suite of performance metrics.
package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

// ---- Public types -------------------------------------------------------

// RunConfig holds everything the engine needs to execute one backtest.
type RunConfig struct {
	BacktestID  int64
	Strategy    registry.Strategy
	Candles     []registry.Candle
	InitialCash float64 // default 10 000
	Commission  float64 // fraction per side, default 0.001
	Slippage    float64 // fraction per side, default 0.0005
}

// EquityPoint is one sample on the equity curve.
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Metrics is the full set of performance statistics computed after a run.
type Metrics struct {
	TotalReturn         float64       `json:"total_return"`
	AnnualizedReturn    float64       `json:"annualized_return"`
	SharpeRatio         float64       `json:"sharpe_ratio"`
	SortinoRatio        float64       `json:"sortino_ratio"`
	MaxDrawdown         float64       `json:"max_drawdown"`
	MaxDrawdownDuration time.Duration `json:"max_drawdown_duration"`
	WinRate             float64       `json:"win_rate"`
	ProfitFactor        float64       `json:"profit_factor"`
	AvgWin              float64       `json:"avg_win"`
	AvgLoss             float64       `json:"avg_loss"`
	TotalTrades         int           `json:"total_trades"`
	WinningTrades       int           `json:"winning_trades"`
	LosingTrades        int           `json:"losing_trades"`
	LargestWin          float64       `json:"largest_win"`
	LargestLoss         float64       `json:"largest_loss"`
}

// Result is returned by Run upon successful completion.
type Result struct {
	BacktestID  int64
	Trades      []models.Trade
	EquityCurve []EquityPoint
	Metrics     Metrics
}

// ---- internal position tracker -----------------------------------------

type openPosition struct {
	direction  models.TradeDirection
	entryPrice float64
	quantity   float64
	entryTime  time.Time
}

// ---- Engine entry point -------------------------------------------------

// Run executes the backtest described by cfg.
//
// db and rdb are optional: pass nil in unit tests to skip persistence and
// Redis publishing without any change in engine logic.
func Run(ctx context.Context, cfg RunConfig, db *gorm.DB, rdb *redis.Client) (*Result, error) {
	// --- Validate -----------------------------------------------------------
	if cfg.Strategy == nil {
		return nil, fmt.Errorf("backtest: strategy must not be nil")
	}
	if len(cfg.Candles) == 0 {
		return nil, fmt.Errorf("backtest: candles must not be empty")
	}
	if cfg.InitialCash <= 0 {
		return nil, fmt.Errorf("backtest: InitialCash must be > 0")
	}

	// --- Apply defaults -----------------------------------------------------
	if cfg.Commission == 0 {
		cfg.Commission = 0.001
	}
	if cfg.Slippage == 0 {
		cfg.Slippage = 0.0005
	}

	// --- Initialise state ---------------------------------------------------
	cash := cfg.InitialCash
	var pos *openPosition
	var closedTrades []models.Trade
	equityCurve := make([]EquityPoint, 0, len(cfg.Candles))

	state := &registry.StrategyState{
		StrategyID: cfg.Strategy.Name(),
		State:      make(map[string]interface{}),
		UpdatedAt:  time.Now(),
	}
	if len(cfg.Candles) > 0 {
		state.Symbol = cfg.Candles[0].Symbol
		state.Market = cfg.Candles[0].Market
	}

	nCandles := len(cfg.Candles)

	// --- Main loop ----------------------------------------------------------
	for i, candle := range cfg.Candles {
		// Give strategy the current candle
		signal, err := cfg.Strategy.OnCandle(candle, state)
		if err != nil {
			return nil, fmt.Errorf("backtest: strategy error on candle %d: %w", i, err)
		}

		// --- Process signal -------------------------------------------------
		if signal != nil {
			switch signal.Direction {

			case "long":
				if pos == nil {
					// Open long position
					entryPrice := candle.Close * (1 + cfg.Slippage)
					quantity := (cash * 0.99) / entryPrice
					entryFee := quantity * entryPrice * cfg.Commission
					cash -= quantity*entryPrice + entryFee

					pos = &openPosition{
						direction:  models.TradeDirectionLong,
						entryPrice: entryPrice,
						quantity:   quantity,
						entryTime:  candle.Timestamp,
					}
				}

			case "short", "flat":
				if pos != nil {
					// Close existing position
					trade, newCash := closePosition(cfg, pos, candle, cash)
					closedTrades = append(closedTrades, trade)
					cash = newCash
					pos = nil
				}
			}
		}

		// --- Compute current equity -----------------------------------------
		equity := cash
		if pos != nil {
			equity += pos.quantity * candle.Close
		}
		equityCurve = append(equityCurve, EquityPoint{
			Timestamp: candle.Timestamp,
			Value:     equity,
		})

		// --- Publish progress to Redis (best-effort) ------------------------
		if rdb != nil {
			progress := int(float64(i+1) / float64(nCandles) * 100)
			key := fmt.Sprintf("backtest:%d:progress", cfg.BacktestID)
			_ = rdb.Set(ctx, key, progress, 24*time.Hour)
		}
	}

	// --- Close any remaining open position at last candle close -------------
	if pos != nil && nCandles > 0 {
		lastCandle := cfg.Candles[nCandles-1]
		trade, newCash := closePosition(cfg, pos, lastCandle, cash)
		closedTrades = append(closedTrades, trade)
		cash = newCash
		pos = nil
		// Update last equity point
		if len(equityCurve) > 0 {
			equityCurve[len(equityCurve)-1].Value = cash
		}
	}

	// --- Compute metrics ----------------------------------------------------
	finalEquity := cash
	if len(equityCurve) > 0 {
		finalEquity = equityCurve[len(equityCurve)-1].Value
	}
	metrics := computeMetrics(closedTrades, equityCurve, cfg.InitialCash, finalEquity)

	// --- Persist trades to DB (best-effort, skip if nil) -------------------
	if db != nil && len(closedTrades) > 0 {
		if err := db.WithContext(ctx).Create(&closedTrades).Error; err != nil {
			return nil, fmt.Errorf("backtest: failed to save trades: %w", err)
		}
	}

	// --- Publish final result to Redis --------------------------------------
	if rdb != nil {
		progressKey := fmt.Sprintf("backtest:%d:progress", cfg.BacktestID)
		_ = rdb.Set(ctx, progressKey, 100, 24*time.Hour)

		if resultJSON, err := json.Marshal(metrics); err == nil {
			resultKey := fmt.Sprintf("backtest:%d:result", cfg.BacktestID)
			_ = rdb.Set(ctx, resultKey, string(resultJSON), 24*time.Hour)
		}
	}

	return &Result{
		BacktestID:  cfg.BacktestID,
		Trades:      closedTrades,
		EquityCurve: equityCurve,
		Metrics:     metrics,
	}, nil
}

// ---- Position closing helper -------------------------------------------

// closePosition builds a closed models.Trade, updates cash, and returns both.
func closePosition(cfg RunConfig, pos *openPosition, candle registry.Candle, cash float64) (models.Trade, float64) {
	exitPrice := candle.Close * (1 - cfg.Slippage)
	exitTime := candle.Timestamp

	// Gross proceeds from selling the position
	grossProceeds := pos.quantity * exitPrice

	// Commission on entry side was already deducted when we opened.
	// Commission on exit side:
	exitFee := grossProceeds * cfg.Commission

	// Total fee charged across both sides of the trade
	entryFee := pos.quantity * pos.entryPrice * cfg.Commission
	totalFee := entryFee + exitFee

	// PnL = proceeds - cost - total fees
	cost := pos.quantity * pos.entryPrice
	pnl := grossProceeds - cost - totalFee
	pnlPct := pnl / cost * 100

	// Update cash
	newCash := cash + grossProceeds - exitFee

	backtestID := cfg.BacktestID
	trade := models.Trade{
		BacktestID: &backtestID,
		Symbol:     candle.Symbol,
		Market:     candle.Market,
		Direction:  pos.direction,
		EntryPrice: pos.entryPrice,
		ExitPrice:  &exitPrice,
		Quantity:   pos.quantity,
		EntryTime:  pos.entryTime,
		ExitTime:   &exitTime,
		PnL:        &pnl,
		PnLPercent: &pnlPct,
		Fee:        totalFee,
	}
	return trade, newCash
}

// ---- Metrics computation -----------------------------------------------

func computeMetrics(trades []models.Trade, equityCurve []EquityPoint, initialCash, finalEquity float64) Metrics {
	m := Metrics{}

	// --- Trade-based stats -------------------------------------------------
	m.TotalTrades = len(trades)
	var grossProfit, grossLoss float64
	var sumWin, sumLoss float64
	m.LargestWin = math.Inf(-1)
	m.LargestLoss = math.Inf(1)

	for _, t := range trades {
		if t.PnL == nil {
			continue
		}
		pnl := *t.PnL
		if pnl >= 0 {
			m.WinningTrades++
			grossProfit += pnl
			sumWin += pnl
			if pnl > m.LargestWin {
				m.LargestWin = pnl
			}
		} else {
			m.LosingTrades++
			grossLoss += -pnl // store as positive
			sumLoss += pnl
			if pnl < m.LargestLoss {
				m.LargestLoss = pnl
			}
		}
	}

	// Reset Inf sentinels if no trades in that category
	if math.IsInf(m.LargestWin, -1) {
		m.LargestWin = 0
	}
	if math.IsInf(m.LargestLoss, 1) {
		m.LargestLoss = 0
	}

	if m.TotalTrades > 0 {
		m.WinRate = float64(m.WinningTrades) / float64(m.TotalTrades)
	}
	if m.WinningTrades > 0 {
		m.AvgWin = sumWin / float64(m.WinningTrades)
	}
	if m.LosingTrades > 0 {
		m.AvgLoss = sumLoss / float64(m.LosingTrades) // negative
	}
	if grossLoss > 0 {
		m.ProfitFactor = grossProfit / grossLoss
	}

	// --- Return-based stats -----------------------------------------------
	m.TotalReturn = (finalEquity - initialCash) / initialCash

	// Annualised return: compute duration from equity curve endpoints
	if len(equityCurve) >= 2 {
		duration := equityCurve[len(equityCurve)-1].Timestamp.Sub(equityCurve[0].Timestamp)
		years := duration.Hours() / (24 * 365.25)
		if years > 0 {
			// CAGR formula: (1 + TotalReturn)^(1/years) - 1
			m.AnnualizedReturn = math.Pow(1+m.TotalReturn, 1/years) - 1
		}
	}

	// --- Sharpe and Sortino -----------------------------------------------
	dailyReturns := dailyEquityReturns(equityCurve)
	if len(dailyReturns) > 1 {
		mean, std := meanStd(dailyReturns)
		if std > 0 {
			m.SharpeRatio = mean / std * math.Sqrt(252)
		}
		downsideStd := downsideDeviation(dailyReturns, 0)
		if downsideStd > 0 {
			m.SortinoRatio = mean / downsideStd * math.Sqrt(252)
		}
	}

	// --- Max drawdown -----------------------------------------------------
	m.MaxDrawdown, m.MaxDrawdownDuration = computeMaxDrawdown(equityCurve)

	return m
}

// dailyEquityReturns converts an equity curve into a slice of period returns.
func dailyEquityReturns(curve []EquityPoint) []float64 {
	if len(curve) < 2 {
		return nil
	}
	returns := make([]float64, 0, len(curve)-1)
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Value
		curr := curve[i].Value
		if prev > 0 {
			returns = append(returns, (curr-prev)/prev)
		}
	}
	return returns
}

// meanStd computes the mean and population standard deviation of xs.
func meanStd(xs []float64) (mean, std float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	var variance float64
	for _, x := range xs {
		d := x - mean
		variance += d * d
	}
	variance /= float64(len(xs))
	std = math.Sqrt(variance)
	return mean, std
}

// downsideDeviation computes the standard deviation of returns below threshold.
func downsideDeviation(returns []float64, threshold float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	var sum float64
	for _, r := range returns {
		if r < threshold {
			d := r - threshold
			sum += d * d
		}
	}
	return math.Sqrt(sum / float64(len(returns)))
}

// computeMaxDrawdown finds the maximum peak-to-trough percentage drop and
// the duration of that drawdown period.
func computeMaxDrawdown(curve []EquityPoint) (maxDD float64, maxDur time.Duration) {
	if len(curve) == 0 {
		return 0, 0
	}
	peak := curve[0].Value
	peakTime := curve[0].Timestamp
	var curDD float64
	var drawStart time.Time

	for _, ep := range curve {
		if ep.Value > peak {
			peak = ep.Value
			peakTime = ep.Timestamp
			drawStart = time.Time{} // reset drawdown start
		}
		if peak > 0 {
			curDD = (peak - ep.Value) / peak
		}
		if curDD > maxDD {
			maxDD = curDD
			if drawStart.IsZero() {
				drawStart = peakTime
			}
			dur := ep.Timestamp.Sub(drawStart)
			if dur > maxDur {
				maxDur = dur
			}
		}
	}
	return maxDD, maxDur
}
