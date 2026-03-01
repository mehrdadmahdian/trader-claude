package analytics

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// CompareRunEntry holds one backtest run's data for comparison.
type CompareRunEntry struct {
	RunID       int64                  `json:"run_id"`
	Name        string                 `json:"name"`
	Strategy    string                 `json:"strategy"`
	Symbol      string                 `json:"symbol"`
	Timeframe   string                 `json:"timeframe"`
	Metrics     map[string]interface{} `json:"metrics"`
	EquityCurve models.JSONArray       `json:"equity_curve"`
}

// CompareResult holds the comparison of multiple backtest runs.
type CompareResult struct {
	Runs []CompareRunEntry `json:"runs"`
}

// CompareRuns loads multiple backtest runs and returns their metrics side-by-side.
func CompareRuns(ctx context.Context, db *gorm.DB, runIDs []int64) (*CompareResult, error) {
	if len(runIDs) == 0 {
		return nil, fmt.Errorf("analytics: no run IDs provided")
	}
	if len(runIDs) > 5 {
		return nil, fmt.Errorf("analytics: maximum 5 runs for comparison")
	}

	var backtests []models.Backtest
	if err := db.WithContext(ctx).Where("id IN ?", runIDs).Find(&backtests).Error; err != nil {
		return nil, fmt.Errorf("analytics: load backtests: %w", err)
	}

	result := &CompareResult{
		Runs: make([]CompareRunEntry, 0, len(backtests)),
	}

	for _, bt := range backtests {
		// Metrics are stored in the Metrics JSON field
		metrics := make(map[string]interface{})
		if bt.Metrics != nil {
			metrics = (map[string]interface{})(bt.Metrics)
		}

		result.Runs = append(result.Runs, CompareRunEntry{
			RunID:       bt.ID,
			Name:        bt.Name,
			Strategy:    bt.StrategyName,
			Symbol:      bt.Symbol,
			Timeframe:   bt.Timeframe,
			Metrics:     metrics,
			EquityCurve: bt.EquityCurve,
		})
	}

	return result, nil
}
