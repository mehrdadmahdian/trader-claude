package analytics

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

// WalkForwardResult holds the full walk-forward analysis output.
type WalkForwardResult struct {
	Windows []WalkForwardWindow `json:"windows"`
	Summary WalkForwardSummary  `json:"summary"`
}

// WalkForwardWindow holds metrics for one train/test window pair.
type WalkForwardWindow struct {
	WindowIndex int       `json:"window_index"`
	TrainStart  time.Time `json:"train_start"`
	TrainEnd    time.Time `json:"train_end"`
	TestStart   time.Time `json:"test_start"`
	TestEnd     time.Time `json:"test_end"`
	TrainSharpe float64   `json:"train_sharpe"`
	TestSharpe  float64   `json:"test_sharpe"`
	TrainReturn float64   `json:"train_return"`
	TestReturn  float64   `json:"test_return"`
	TrainTrades int       `json:"train_trades"`
	TestTrades  int       `json:"test_trades"`
}

// WalkForwardSummary aggregates metrics across all windows.
type WalkForwardSummary struct {
	AvgTestSharpe    float64 `json:"avg_test_sharpe"`
	AvgTestReturn    float64 `json:"avg_test_return"`
	ConsistencyRatio float64 `json:"consistency_ratio"` // fraction of windows where test_sharpe > 0
}

// RunWalkForward performs a sliding-window walk-forward analysis on a backtest.
// It splits the date range into numWindows equal intervals, runs the strategy on
// a 70% train / 30% test split for each window, and returns performance metrics.
func RunWalkForward(ctx context.Context, db *gorm.DB, backtestID int64, numWindows int) (*WalkForwardResult, error) {
	// Clamp numWindows to reasonable range
	if numWindows <= 0 {
		numWindows = 5
	}
	if numWindows > 20 {
		numWindows = 20
	}

	// Load original backtest from DB
	var bt models.Backtest
	if err := db.WithContext(ctx).First(&bt, backtestID).Error; err != nil {
		return nil, fmt.Errorf("walkforward: load backtest: %w", err)
	}

	// Load all candles for this symbol/market/timeframe within the date range
	var mCandles []models.Candle
	if err := db.WithContext(ctx).
		Where("symbol = ? AND market = ? AND timeframe = ?", bt.Symbol, bt.Market, bt.Timeframe).
		Where("timestamp >= ? AND timestamp <= ?", bt.StartDate, bt.EndDate).
		Order("timestamp ASC").
		Find(&mCandles).Error; err != nil {
		return nil, fmt.Errorf("walkforward: load candles: %w", err)
	}

	if len(mCandles) == 0 {
		return nil, fmt.Errorf("walkforward: no candles found for %s/%s/%s", bt.Symbol, bt.Market, bt.Timeframe)
	}

	// Convert model candles to registry candles
	candles := toRegistryCandles(mCandles)

	// Compute total duration and window boundaries
	totalDuration := bt.EndDate.Sub(bt.StartDate)
	windowDuration := totalDuration / time.Duration(numWindows)

	// Base strategy params from the backtest
	baseParams := make(map[string]interface{})
	if bt.Params != nil {
		for k, v := range bt.Params {
			baseParams[k] = v
		}
	}

	// Run analysis for each window
	windows := make([]WalkForwardWindow, 0, numWindows)
	var positiveTestSharpes int

	for i := 0; i < numWindows; i++ {
		// Compute window boundaries
		windowStart := bt.StartDate.Add(time.Duration(i) * windowDuration)
		windowEnd := bt.StartDate.Add(time.Duration(i+1) * windowDuration)

		// Train/test split: 70% train, 30% test
		trainTestBoundary := windowStart.Add(windowDuration * 7 / 10)

		// Filter candles for train period [windowStart, trainTestBoundary)
		trainCandles := filterCandlesByTimeRange(candles, windowStart, trainTestBoundary)

		// Filter candles for test period [trainTestBoundary, windowEnd)
		testCandles := filterCandlesByTimeRange(candles, trainTestBoundary, windowEnd)

		wf := WalkForwardWindow{
			WindowIndex: i,
			TrainStart:  windowStart,
			TrainEnd:    trainTestBoundary,
			TestStart:   trainTestBoundary,
			TestEnd:     windowEnd,
		}

		// Skip window if either period has no candles
		if len(trainCandles) == 0 || len(testCandles) == 0 {
			windows = append(windows, wf)
			continue
		}

		// Create strategy and init with base params
		strat, err := registry.Strategies().Create(bt.StrategyName)
		if err != nil {
			return nil, fmt.Errorf("walkforward: create strategy: %w", err)
		}
		if err := strat.Init(baseParams); err != nil {
			return nil, fmt.Errorf("walkforward: init strategy: %w", err)
		}

		// Run backtest on train period
		trainCfg := backtest.RunConfig{
			BacktestID: 0, // ephemeral
			Strategy:   strat,
			Candles:    trainCandles,
			Timeframe:  bt.Timeframe,
		}
		trainResult, err := backtest.Run(ctx, trainCfg, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("walkforward: run train: %w", err)
		}

		wf.TrainSharpe = trainResult.Metrics.SharpeRatio
		wf.TrainReturn = trainResult.Metrics.TotalReturn
		wf.TrainTrades = trainResult.Metrics.TotalTrades

		// Reset strategy for test period
		strat, err = registry.Strategies().Create(bt.StrategyName)
		if err != nil {
			return nil, fmt.Errorf("walkforward: create strategy for test: %w", err)
		}
		if err := strat.Init(baseParams); err != nil {
			return nil, fmt.Errorf("walkforward: init strategy for test: %w", err)
		}

		// Run backtest on test period
		testCfg := backtest.RunConfig{
			BacktestID: 0, // ephemeral
			Strategy:   strat,
			Candles:    testCandles,
			Timeframe:  bt.Timeframe,
		}
		testResult, err := backtest.Run(ctx, testCfg, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("walkforward: run test: %w", err)
		}

		wf.TestSharpe = testResult.Metrics.SharpeRatio
		wf.TestReturn = testResult.Metrics.TotalReturn
		wf.TestTrades = testResult.Metrics.TotalTrades

		if wf.TestSharpe > 0 {
			positiveTestSharpes++
		}

		windows = append(windows, wf)
	}

	// Compute summary statistics
	summary := computeWalkForwardSummary(windows, positiveTestSharpes)

	return &WalkForwardResult{
		Windows: windows,
		Summary: summary,
	}, nil
}

// filterCandlesByTimeRange returns candles within [start, end).
func filterCandlesByTimeRange(candles []registry.Candle, start, end time.Time) []registry.Candle {
	var filtered []registry.Candle
	for _, c := range candles {
		if !c.Timestamp.Before(start) && c.Timestamp.Before(end) {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// computeWalkForwardSummary aggregates window metrics into summary stats.
func computeWalkForwardSummary(windows []WalkForwardWindow, positiveTestSharpes int) WalkForwardSummary {
	var sumTestSharpe, sumTestReturn float64
	for _, w := range windows {
		sumTestSharpe += w.TestSharpe
		sumTestReturn += w.TestReturn
	}

	nWindows := len(windows)
	summary := WalkForwardSummary{}

	if nWindows > 0 {
		summary.AvgTestSharpe = sumTestSharpe / float64(nWindows)
		summary.AvgTestReturn = sumTestReturn / float64(nWindows)
		summary.ConsistencyRatio = float64(positiveTestSharpes) / float64(nWindows)
	}

	return summary
}
