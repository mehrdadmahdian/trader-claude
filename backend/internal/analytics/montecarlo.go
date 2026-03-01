package analytics

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// MonteCarloResult holds the output of a Monte Carlo simulation.
type MonteCarloResult struct {
	NumSimulations  int               `json:"num_simulations"`
	Percentiles     []PercentileCurve `json:"percentiles"`
	ProbabilityRuin float64           `json:"probability_of_ruin"`
	MedianReturn    float64           `json:"median_return"`
	MinReturn       float64           `json:"min_return"`
	MaxReturn       float64           `json:"max_return"`
}

// PercentileCurve holds one percentile line across trade indices.
type PercentileCurve struct {
	Percentile int       `json:"percentile"`
	Values     []float64 `json:"values"`
}

// RunMonteCarlo runs numSims Monte Carlo simulations by resampling trade returns.
func RunMonteCarlo(ctx context.Context, db *gorm.DB, backtestID int64, numSims int, ruinThreshold float64) (*MonteCarloResult, error) {
	if numSims <= 0 {
		numSims = 1000
	}
	if ruinThreshold <= 0 {
		ruinThreshold = 0.5
	}

	var trades []models.Trade
	if err := db.WithContext(ctx).Where("backtest_id = ?", backtestID).Order("entry_time ASC").Find(&trades).Error; err != nil {
		return nil, fmt.Errorf("montecarlo: load trades: %w", err)
	}
	if len(trades) < 2 {
		return nil, fmt.Errorf("montecarlo: need at least 2 trades, got %d", len(trades))
	}

	returns := make([]float64, 0, len(trades))
	for _, t := range trades {
		if t.PnLPercent != nil {
			returns = append(returns, *t.PnLPercent/100.0)
		}
	}
	if len(returns) < 2 {
		return nil, fmt.Errorf("montecarlo: not enough trades with PnL data")
	}

	numTrades := len(returns)
	initialCapital := 10000.0
	ruinLevel := initialCapital * ruinThreshold

	equities := make([][]float64, numSims)
	finalReturns := make([]float64, numSims)
	ruinCount := 0

	rng := rand.New(rand.NewSource(42))

	for sim := 0; sim < numSims; sim++ {
		equity := make([]float64, numTrades+1)
		equity[0] = initialCapital
		ruined := false

		for i := 0; i < numTrades; i++ {
			idx := rng.Intn(len(returns))
			equity[i+1] = equity[i] * (1 + returns[idx])
			if equity[i+1] < ruinLevel {
				ruined = true
			}
		}

		equities[sim] = equity
		finalReturns[sim] = (equity[numTrades] - initialCapital) / initialCapital
		if ruined {
			ruinCount++
		}
	}

	percentileValues := []int{5, 25, 50, 75, 95}
	curves := make([]PercentileCurve, len(percentileValues))

	for pi, pct := range percentileValues {
		curve := make([]float64, numTrades+1)
		for step := 0; step <= numTrades; step++ {
			vals := make([]float64, numSims)
			for sim := 0; sim < numSims; sim++ {
				vals[sim] = equities[sim][step]
			}
			sort.Float64s(vals)
			idx := int(math.Round(float64(pct) / 100.0 * float64(numSims-1)))
			if idx >= numSims {
				idx = numSims - 1
			}
			curve[step] = vals[idx]
		}
		curves[pi] = PercentileCurve{Percentile: pct, Values: curve}
	}

	sort.Float64s(finalReturns)

	return &MonteCarloResult{
		NumSimulations:  numSims,
		Percentiles:     curves,
		ProbabilityRuin: float64(ruinCount) / float64(numSims),
		MedianReturn:    finalReturns[numSims/2],
		MinReturn:       finalReturns[0],
		MaxReturn:       finalReturns[numSims-1],
	}, nil
}
