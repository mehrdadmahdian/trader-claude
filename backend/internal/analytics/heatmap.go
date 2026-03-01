package analytics

import (
	"context"
	"fmt"
	"math"
	"sync"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/backtest"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/registry"
)

// HeatmapResult holds the grid output of a parameter sensitivity heatmap.
type HeatmapResult struct {
	XParam  string          `json:"x_param"`
	YParam  string          `json:"y_param"`
	XValues []float64       `json:"x_values"`
	YValues []float64       `json:"y_values"`
	Cells   [][]HeatmapCell `json:"cells"` // [y][x]
}

// HeatmapCell holds the metrics for one (x, y) parameter combination.
type HeatmapCell struct {
	XValue      float64 `json:"x_value"`
	YValue      float64 `json:"y_value"`
	SharpeRatio float64 `json:"sharpe_ratio"`
	TotalReturn float64 `json:"total_return"`
	MaxDrawdown float64 `json:"max_drawdown"`
	TotalTrades int     `json:"total_trades"`
}

// linspace returns gridSize evenly-spaced values from min to max (inclusive).
func linspace(min, max float64, n int) []float64 {
	if n <= 1 {
		return []float64{min}
	}
	step := (max - min) / float64(n-1)
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = min + float64(i)*step
	}
	return vals
}

// toRegistryCandles converts model candles to registry candles.
func toRegistryCandles(mc []models.Candle) []registry.Candle {
	rc := make([]registry.Candle, len(mc))
	for i, c := range mc {
		rc[i] = registry.Candle{
			Symbol:    c.Symbol,
			Market:    c.Market,
			Timeframe: c.Timeframe,
			Timestamp: c.Timestamp,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		}
	}
	return rc
}

// RunHeatmap runs a grid of mini-backtests varying xParam and yParam.
// gridSize must be between 2 and 20.
func RunHeatmap(ctx context.Context, db *gorm.DB, backtestID int64, xParam, yParam string, gridSize int) (*HeatmapResult, error) {
	if gridSize < 2 {
		gridSize = 2
	}
	if gridSize > 20 {
		gridSize = 20
	}

	// Load original backtest
	var bt models.Backtest
	if err := db.WithContext(ctx).First(&bt, backtestID).Error; err != nil {
		return nil, fmt.Errorf("heatmap: load backtest: %w", err)
	}

	// Get strategy and its param definitions
	strat, err := registry.Strategies().Create(bt.StrategyName)
	if err != nil {
		return nil, fmt.Errorf("heatmap: create strategy: %w", err)
	}
	paramDefs := strat.Params()

	// Find x and y param definitions
	var xDef, yDef *registry.ParamDefinition
	for i := range paramDefs {
		if paramDefs[i].Name == xParam {
			p := paramDefs[i]
			xDef = &p
		}
		if paramDefs[i].Name == yParam {
			p := paramDefs[i]
			yDef = &p
		}
	}
	if xDef == nil {
		return nil, fmt.Errorf("heatmap: param %q not found in strategy", xParam)
	}
	if yDef == nil {
		return nil, fmt.Errorf("heatmap: param %q not found in strategy", yParam)
	}

	// Extract numeric min/max for x
	xMin, xMax, err := extractMinMax(xDef)
	if err != nil {
		return nil, fmt.Errorf("heatmap: x param: %w", err)
	}
	yMin, yMax, err := extractMinMax(yDef)
	if err != nil {
		return nil, fmt.Errorf("heatmap: y param: %w", err)
	}

	xValues := linspace(xMin, xMax, gridSize)
	yValues := linspace(yMin, yMax, gridSize)

	// Load candles
	var mCandles []models.Candle
	q := db.WithContext(ctx).
		Where("symbol = ? AND market = ? AND timeframe = ?", bt.Symbol, bt.Market, bt.Timeframe).
		Where("timestamp >= ? AND timestamp <= ?", bt.StartDate, bt.EndDate).
		Order("timestamp ASC").
		Find(&mCandles)
	if q.Error != nil {
		return nil, fmt.Errorf("heatmap: load candles: %w", q.Error)
	}
	if len(mCandles) == 0 {
		return nil, fmt.Errorf("heatmap: no candles found for %s/%s/%s", bt.Symbol, bt.Market, bt.Timeframe)
	}
	candles := toRegistryCandles(mCandles)

	// Base params from the original backtest
	baseParams := map[string]interface{}{}
	if bt.Params != nil {
		for k, v := range bt.Params {
			baseParams[k] = v
		}
	}

	// Allocate result grid
	cells := make([][]HeatmapCell, gridSize)
	for i := range cells {
		cells[i] = make([]HeatmapCell, gridSize)
	}

	// Run grid cells concurrently
	type cellResult struct {
		yi, xi int
		cell   HeatmapCell
	}
	results := make(chan cellResult, gridSize*gridSize)
	var wg sync.WaitGroup

	for yi, yVal := range yValues {
		for xi, xVal := range xValues {
			wg.Add(1)
			go func(xi, yi int, xVal, yVal float64) {
				defer wg.Done()

				// Clone params and override
				params := make(map[string]interface{}, len(baseParams)+2)
				for k, v := range baseParams {
					params[k] = v
				}
				params[xParam] = xVal
				params[yParam] = yVal

				cell := HeatmapCell{XValue: xVal, YValue: yVal}

				// Create and init strategy for this cell
				s, err := registry.Strategies().Create(bt.StrategyName)
				if err == nil {
					if err := s.Init(params); err == nil {
						cfg := backtest.RunConfig{
							BacktestID: 0, // ephemeral
							Strategy:   s,
							Candles:    candles,
							Timeframe:  bt.Timeframe,
						}
						if res, err := backtest.Run(ctx, cfg, nil, nil); err == nil {
							cell.SharpeRatio = res.Metrics.SharpeRatio
							cell.TotalReturn = res.Metrics.TotalReturn
							cell.MaxDrawdown = res.Metrics.MaxDrawdown
							cell.TotalTrades = res.Metrics.TotalTrades
						}
					}
				}

				results <- cellResult{yi: yi, xi: xi, cell: cell}
			}(xi, yi, xVal, yVal)
		}
	}

	wg.Wait()
	close(results)

	for r := range results {
		cells[r.yi][r.xi] = r.cell
	}

	return &HeatmapResult{
		XParam:  xParam,
		YParam:  yParam,
		XValues: xValues,
		YValues: yValues,
		Cells:   cells,
	}, nil
}

// extractMinMax extracts numeric min/max from a ParamDefinition.
func extractMinMax(p *registry.ParamDefinition) (float64, float64, error) {
	toFloat := func(v interface{}) (float64, bool) {
		switch n := v.(type) {
		case float64:
			return n, true
		case float32:
			return float64(n), true
		case int:
			return float64(n), true
		case int64:
			return float64(n), true
		case int32:
			return float64(n), true
		}
		return 0, false
	}

	minVal, ok := toFloat(p.Min)
	if !ok {
		return 0, 0, fmt.Errorf("param %q has no numeric Min", p.Name)
	}
	maxVal, ok := toFloat(p.Max)
	if !ok {
		return 0, 0, fmt.Errorf("param %q has no numeric Max", p.Name)
	}
	if math.IsNaN(minVal) || math.IsNaN(maxVal) || minVal >= maxVal {
		return 0, 0, fmt.Errorf("param %q: invalid range [%v, %v]", p.Name, minVal, maxVal)
	}
	return minVal, maxVal, nil
}
