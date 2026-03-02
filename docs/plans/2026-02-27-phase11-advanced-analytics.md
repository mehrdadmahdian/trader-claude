# Phase 11 — Advanced Analytics — Atomic Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add four advanced analytical tools to completed backtests: parameter sensitivity heatmap, Monte Carlo simulation, walk-forward analysis, and run comparison. All computations run server-side inside the existing worker pool with async job tracking.

**Architecture:** New `internal/analytics/` package with pure computation functions. Results stored in `analytics_results` MySQL table. The frontend polls `GET /analytics/jobs/:id` until completion and renders the results in an "Advanced Analytics" tab on the Backtest results panel.

**Tech Stack:** Go 1.24 (Fiber v2, GORM, worker pool), React 18 (React Query v5, Recharts, Tailwind, shadcn/ui). No new dependencies.

**Design doc:** `docs/plans/2026-02-27-phase11-advanced-analytics-design.md`

---

## BACKEND TASKS

---

### Task B1: Add AnalyticsResult model + autoMigrate

**Read first:**
- `backend/internal/models/models.go` (last 30 lines)
- `backend/cmd/server/main.go` (autoMigrate function)

**Files to modify:**
- `backend/internal/models/models.go`
- `backend/cmd/server/main.go`

---

**Step 1: Append AnalyticsResult struct to models.go**

```go
// --- AnalyticsResult ---

// AnalyticsResult stores the output of an advanced analytics computation.
type AnalyticsResult struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	BacktestRunID int64      `gorm:"index;not null" json:"backtest_run_id"`
	Type          string     `gorm:"type:varchar(30);not null;index" json:"type"`
	Status        string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Params        JSON       `gorm:"type:json" json:"params"`
	Result        JSON       `gorm:"type:json" json:"result,omitempty"`
	ErrorMessage  string     `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func (AnalyticsResult) TableName() string { return "analytics_results" }

const (
	AnalyticsTypeHeatmap     = "heatmap"
	AnalyticsTypeMonteCarlo  = "monte_carlo"
	AnalyticsTypeWalkForward = "walk_forward"

	AnalyticsStatusPending   = "pending"
	AnalyticsStatusRunning   = "running"
	AnalyticsStatusCompleted = "completed"
	AnalyticsStatusFailed    = "failed"
)
```

---

**Step 2: Add to autoMigrate in main.go**

---

**Step 3: Verify compile + commit**

```bash
make backend-fmt && docker compose exec backend go build ./...
git add backend/internal/models/models.go backend/cmd/server/main.go
git commit -m "feat(phase11): add AnalyticsResult model and autoMigrate"
```

---

### Task B2: Create internal/analytics/compare.go + tests

**Read first:**
- `backend/internal/models/models.go` (Backtest, Trade structs)

**Files to create:**
- `backend/internal/analytics/compare.go`
- `backend/internal/analytics/compare_test.go`

---

**Step 1: Create `compare.go`**

```go
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
	EquityCurve models.JSON            `json:"equity_curve"`
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
		metrics := map[string]interface{}{
			"total_return":  bt.TotalReturn,
			"sharpe_ratio":  bt.SharpeRatio,
			"max_drawdown":  bt.MaxDrawdown,
			"win_rate":      bt.WinRate,
			"total_trades":  bt.TotalTrades,
			"profit_factor": bt.ProfitFactor,
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
```

---

**Step 2: Create `compare_test.go`**

```go
package analytics

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Backtest{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCompareRuns_Success(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Backtest{Name: "Run1", StrategyName: "ema", Symbol: "BTCUSDT", Timeframe: "1h", Status: models.BacktestStatusCompleted, TotalReturn: 0.34})
	db.Create(&models.Backtest{Name: "Run2", StrategyName: "rsi", Symbol: "ETHUSDT", Timeframe: "4h", Status: models.BacktestStatusCompleted, TotalReturn: -0.05})

	result, err := CompareRuns(context.Background(), db, []int64{1, 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(result.Runs))
	}
}

func TestCompareRuns_MaxFiveRuns(t *testing.T) {
	db := setupTestDB(t)
	_, err := CompareRuns(context.Background(), db, []int64{1, 2, 3, 4, 5, 6})
	if err == nil {
		t.Fatal("expected error for > 5 runs")
	}
}

func TestCompareRuns_EmptyIDs(t *testing.T) {
	db := setupTestDB(t)
	_, err := CompareRuns(context.Background(), db, []int64{})
	if err == nil {
		t.Fatal("expected error for empty IDs")
	}
}
```

---

**Step 3: Run tests + commit**

```bash
docker compose exec backend go test ./internal/analytics/... -v
git add backend/internal/analytics/
git commit -m "feat(phase11): add CompareRuns analytics function"
```

---

### Task B3: Create internal/analytics/montecarlo.go + tests

**Read first:**
- `backend/internal/models/models.go` (Trade struct — PnLPercent field)

**Files to create:**
- `backend/internal/analytics/montecarlo.go`
- `backend/internal/analytics/montecarlo_test.go`

---

**Step 1: Create `montecarlo.go`**

```go
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
// ruinThreshold is the fraction of initial capital below which a sim is "ruined" (e.g., 0.5 = 50%).
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
			idx := int(math.Round(float64(pct)/100.0*float64(numSims-1)))
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
```

---

**Step 2: Create `montecarlo_test.go`**

```go
package analytics

import (
	"context"
	"testing"

	"github.com/trader-claude/backend/internal/models"
)

func TestRunMonteCarlo_Success(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Trade{})
	db.Create(&models.Backtest{Name: "BT", StrategyName: "ema", Symbol: "BTCUSDT", Timeframe: "1h", Status: models.BacktestStatusCompleted})

	pnl1, pnl2, pnl3 := 5.0, -3.0, 8.0
	pnlPct1, pnlPct2, pnlPct3 := 5.0, -3.0, 8.0
	db.Create(&models.Trade{BacktestID: 1, Symbol: "BTCUSDT", Direction: "long", EntryPrice: 100, PnL: &pnl1, PnLPercent: &pnlPct1})
	db.Create(&models.Trade{BacktestID: 1, Symbol: "BTCUSDT", Direction: "long", EntryPrice: 105, PnL: &pnl2, PnLPercent: &pnlPct2})
	db.Create(&models.Trade{BacktestID: 1, Symbol: "BTCUSDT", Direction: "long", EntryPrice: 102, PnL: &pnl3, PnLPercent: &pnlPct3})

	result, err := RunMonteCarlo(context.Background(), db, 1, 100, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumSimulations != 100 {
		t.Errorf("expected 100 sims, got %d", result.NumSimulations)
	}
	if len(result.Percentiles) != 5 {
		t.Errorf("expected 5 percentile curves, got %d", len(result.Percentiles))
	}
	if result.ProbabilityRuin < 0 || result.ProbabilityRuin > 1 {
		t.Errorf("ruin probability out of bounds: %f", result.ProbabilityRuin)
	}
}

func TestRunMonteCarlo_TooFewTrades(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&models.Trade{})
	db.Create(&models.Backtest{Name: "BT", StrategyName: "ema", Symbol: "BTCUSDT", Timeframe: "1h", Status: models.BacktestStatusCompleted})

	pnl := 5.0
	db.Create(&models.Trade{BacktestID: 1, Symbol: "BTCUSDT", Direction: "long", EntryPrice: 100, PnL: &pnl, PnLPercent: &pnl})

	_, err := RunMonteCarlo(context.Background(), db, 1, 100, 0.5)
	if err == nil {
		t.Fatal("expected error for too few trades")
	}
}
```

---

**Step 3: Run tests + commit**

```bash
docker compose exec backend go test ./internal/analytics/... -v
git add backend/internal/analytics/montecarlo.go backend/internal/analytics/montecarlo_test.go
git commit -m "feat(phase11): add Monte Carlo simulation with percentile curves and ruin probability"
```

---

### Task B4: Create internal/analytics/heatmap.go + tests

**Read first:**
- `backend/internal/registry/interfaces.go` (Strategy, ParamDefinition)
- `backend/internal/worker/pool.go` (Submit pattern)

**Files to create:**
- `backend/internal/analytics/heatmap.go`
- `backend/internal/analytics/heatmap_test.go`

---

**Step 1: Create `heatmap.go`**

Generates a grid of mini-backtests by varying two numeric parameters. Uses `sync.WaitGroup` to run cells concurrently.

Key types: `HeatmapResult`, `HeatmapCell` (as defined in design doc).

Core algorithm:
1. Load original backtest + strategy + candles
2. Generate `linspace(min, max, gridSize)` for X and Y params
3. For each (x, y): clone params, override x/y values, run backtest engine
4. Collect results into `cells[y][x]` matrix

---

**Step 2: Create `heatmap_test.go`**

Test with mock strategy + small grid (3×3 = 9 cells). Verify grid dimensions and that all cells have valid Sharpe values.

---

**Step 3: Run tests + commit**

```bash
docker compose exec backend go test ./internal/analytics/... -v
git add backend/internal/analytics/heatmap.go backend/internal/analytics/heatmap_test.go
git commit -m "feat(phase11): add parameter sensitivity heatmap with parallel grid execution"
```

---

### Task B5: Create internal/analytics/walkforward.go + tests

**Read first:**
- `backend/internal/analytics/heatmap.go` (for backtest re-run pattern)

**Files to create:**
- `backend/internal/analytics/walkforward.go`
- `backend/internal/analytics/walkforward_test.go`

---

**Step 1: Create `walkforward.go`**

Splits the candle date range into N windows (70/30 train/test). For each window, runs the strategy on train and test periods separately.

Key types: `WalkForwardResult`, `WalkForwardWindow`, `WalkForwardSummary`.

---

**Step 2: Create `walkforward_test.go`**

Test window splitting logic (5 windows over 100 days). Verify each window has correct train/test date ranges and non-overlapping test periods.

---

**Step 3: Run tests + commit**

```bash
docker compose exec backend go test ./internal/analytics/... -v
git add backend/internal/analytics/walkforward.go backend/internal/analytics/walkforward_test.go
git commit -m "feat(phase11): add walk-forward analysis with sliding windows"
```

---

### Task B6: Create api/analytics_handler.go + wire routes

**Read first:**
- `backend/internal/api/backtest.go` (async job pattern)
- `backend/internal/analytics/` (all function signatures)

**Files to create:**
- `backend/internal/api/analytics_handler.go`

**Files to modify:**
- `backend/internal/api/routes.go`

---

**Step 1: Create `analytics_handler.go`**

Handler struct holds `db *gorm.DB` and `pool *worker.WorkerPool`.

Endpoints:
- `GET /api/v1/backtest/runs/:id/param-heatmap` — creates async job, returns `{job_id, status}`
- `POST /api/v1/backtest/runs/:id/monte-carlo` — creates async job
- `GET /api/v1/backtest/runs/:id/walk-forward` — creates async job
- `POST /api/v1/backtest/compare` — synchronous, returns CompareResult
- `GET /api/v1/analytics/jobs/:jobId` — polls job status + result

Async job pattern:
1. Insert `AnalyticsResult` with `status: "pending"`
2. Submit worker pool job that:
   a. Updates status to `"running"`
   b. Calls analytics function
   c. On success: stores JSON result, sets `status: "completed"`
   d. On error: stores error message, sets `status: "failed"`
3. Return `{job_id, status: "pending"}` immediately

---

**Step 2: Wire routes**

```go
// --- Analytics ---
anah := newAnalyticsHandler(db, pool)
v1.Get("/backtest/runs/:id/param-heatmap", anah.paramHeatmap)
v1.Post("/backtest/runs/:id/monte-carlo", anah.monteCarlo)
v1.Get("/backtest/runs/:id/walk-forward", anah.walkForward)
v1.Post("/backtest/compare", anah.compareRuns)
v1.Get("/analytics/jobs/:jobId", anah.getJob)
```

---

**Step 3: Verify compile + tests + commit**

```bash
make backend-fmt && docker compose exec backend go build ./...
make backend-test
git add backend/internal/api/analytics_handler.go backend/internal/api/routes.go
git commit -m "feat(phase11): add analytics API endpoints with async job tracking"
```

---

## FRONTEND TASKS

---

### Task F1: Add analytics types + api/analytics.ts

**Files to modify/create:**
- `frontend/src/types/index.ts` — append analytics types
- `frontend/src/api/analytics.ts` — create API client

---

### Task F2: Create hooks/useAnalytics.ts with polling

React Query hooks with `refetchInterval` for polling:
- `useParamHeatmap(runId, xParam, yParam, gridSize)` — starts job, then polls
- `useMonteCarlo(runId, numSims)` — starts job, then polls
- `useWalkForward(runId, windows)` — starts job, then polls
- `useCompareRuns(runIds)` — synchronous query
- `useAnalyticsJob(jobId)` — polls every 2s until complete

---

### Task F3: Create components/analytics/CompareRuns.tsx

Side-by-side metrics table + overlaid equity curves (Recharts LineChart with multiple lines).

---

### Task F4: Create components/analytics/MonteCarloChart.tsx

Fan chart using Recharts AreaChart with stacked transparent areas for percentile bands. Show ruin probability and median return as stat cards above the chart.

---

### Task F5: Create components/analytics/ParamHeatmap.tsx

Grid of colored cells. Color scale: red (bad Sharpe) → yellow → green (good Sharpe). Hover tooltip shows all metrics for the cell. Parameter selectors (dropdowns) above the grid.

---

### Task F6: Create components/analytics/WalkForwardChart.tsx

Grouped bar chart (Recharts BarChart). Train Sharpe in blue, test Sharpe in orange, per window. Summary stats (avg test Sharpe, consistency ratio) as header cards.

---

### Task F7: Create components/analytics/AnalyticsTab.tsx

Container component that wraps all 4 analytics sections in collapsible cards. Each section has a "Run" button that triggers the computation and shows a loading spinner until results arrive.

---

### Task F8: Add "Advanced Analytics" tab to Backtest results

**Files to modify:**
- `frontend/src/pages/Backtest.tsx`

Add a new tab alongside Overview, Trades, Chart, Bookmarks:
```tsx
<button onClick={() => setActiveTab('analytics')} className={tabClass('analytics')}>
  Advanced Analytics
</button>
```

Mount `<AnalyticsTab runId={selectedRunId} />` when active.

---

**Final Verification:**

```bash
make backend-test
make frontend-lint
make frontend-test
```

---

## Summary

| Task | Files | Description |
|---|---|---|
| B1 | models.go, main.go | AnalyticsResult model + autoMigrate |
| B2 | analytics/compare.go | Compare runs — load and normalize metrics |
| B3 | analytics/montecarlo.go | Monte Carlo simulation with resampled trade returns |
| B4 | analytics/heatmap.go | Parameter sensitivity heatmap with parallel grid |
| B5 | analytics/walkforward.go | Walk-forward analysis with sliding windows |
| B6 | api/analytics_handler.go, routes.go | API endpoints + async job tracking |
| F1 | types/index.ts, api/analytics.ts | Analytics types + API client |
| F2 | hooks/useAnalytics.ts | React Query hooks with polling |
| F3 | components/analytics/CompareRuns.tsx | Side-by-side comparison table + equity overlay |
| F4 | components/analytics/MonteCarloChart.tsx | Fan chart with percentile bands |
| F5 | components/analytics/ParamHeatmap.tsx | Color-coded parameter grid |
| F6 | components/analytics/WalkForwardChart.tsx | Train vs test bar chart |
| F7 | components/analytics/AnalyticsTab.tsx | Container for all 4 sections |
| F8 | pages/Backtest.tsx | Wire "Advanced Analytics" tab |
