# Phase 11 — Advanced Analytics: Design Document

**Date:** 2026-02-27
**Status:** Draft

---

## Overview

Phase 11 adds four advanced analytical tools on top of completed backtests: a **parameter sensitivity heatmap**, a **Monte Carlo simulation**, a **walk-forward analysis**, and a **run comparison** view. All computations run server-side inside the existing worker pool; the frontend adds an "Advanced Analytics" tab to the Backtest results panel.

---

## Decisions Made

| Topic | Decision |
|---|---|
| Heatmap grid | Up to 20×20 cells = 400 backtests; each cell runs in the worker pool concurrently |
| Monte Carlo sims | 1000 simulations using trade-return resampling with replacement |
| Walk-forward windows | User-configurable N windows (default 5); sliding window, no overlap |
| Compare runs | Up to 5 runs side-by-side; reuses existing backtest result data |
| Computation model | Synchronous with progress — POST starts job, returns job_id; GET polls for result |
| Result storage | Stored as JSON in a new `analytics_results` table; keyed by backtest_run_id + type |
| Heatmap strategy params | Client picks two numeric params from the strategy's param schema; backend iterates grid |
| Probability of ruin | Defined as equity dropping below 50% of initial capital at any point in a simulation |
| Fan chart percentiles | 5th, 25th, 50th (median), 75th, 95th percentiles across 1000 equity curves |

---

## Section 1: New DB Model

### `analytics_results` table

```go
type AnalyticsResult struct {
    ID            int64          `gorm:"primaryKey;autoIncrement" json:"id"`
    BacktestRunID int64          `gorm:"index;not null" json:"backtest_run_id"`
    Type          string         `gorm:"type:varchar(30);not null;index" json:"type"` // "heatmap", "monte_carlo", "walk_forward"
    Status        string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"` // "pending", "running", "completed", "failed"
    Params        JSON           `gorm:"type:json" json:"params"`
    Result        JSON           `gorm:"type:json" json:"result,omitempty"`
    ErrorMessage  string         `gorm:"type:text" json:"error_message,omitempty"`
    CreatedAt     time.Time      `json:"created_at"`
    CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

func (AnalyticsResult) TableName() string { return "analytics_results" }
```

---

## Section 2: Backend — `internal/analytics/` Package

### File layout

```
internal/analytics/
  heatmap.go          RunHeatmap(ctx, db, pool, backtestID, xParam, yParam, gridSize) → HeatmapResult
  heatmap_test.go     Grid correctness, parallel execution
  montecarlo.go       RunMonteCarlo(ctx, db, backtestID, numSims, ruinThreshold) → MonteCarloResult
  montecarlo_test.go  Distribution shape, ruin probability bounds
  walkforward.go      RunWalkForward(ctx, db, pool, ds, backtestID, numWindows) → WalkForwardResult
  walkforward_test.go Window splitting logic
  compare.go          CompareRuns(ctx, db, runIDs) → CompareResult
  compare_test.go     Side-by-side metrics correctness
```

### Heatmap

```go
type HeatmapResult struct {
    XParam   string          `json:"x_param"`
    YParam   string          `json:"y_param"`
    XValues  []float64       `json:"x_values"`
    YValues  []float64       `json:"y_values"`
    Cells    [][]HeatmapCell `json:"cells"` // [y][x]
}

type HeatmapCell struct {
    XValue      float64 `json:"x_value"`
    YValue      float64 `json:"y_value"`
    SharpeRatio float64 `json:"sharpe_ratio"`
    TotalReturn float64 `json:"total_return"`
    MaxDrawdown float64 `json:"max_drawdown"`
    TotalTrades int     `json:"total_trades"`
}
```

Algorithm:
1. Load original backtest params + candle data
2. Generate grid: linspace(x_min, x_max, gridSize) × linspace(y_min, y_max, gridSize)
3. For each (x, y) cell: submit a mini-backtest to the worker pool with overridden params
4. Collect results into the grid matrix
5. Use `sync.WaitGroup` to wait for all cells

### Monte Carlo

```go
type MonteCarloResult struct {
    NumSimulations  int               `json:"num_simulations"`
    Percentiles     []PercentileCurve `json:"percentiles"` // 5th, 25th, 50th, 75th, 95th
    ProbabilityRuin float64           `json:"probability_of_ruin"` // 0.0–1.0
    MedianReturn    float64           `json:"median_return"`
    MinReturn       float64           `json:"min_return"`
    MaxReturn       float64           `json:"max_return"`
}

type PercentileCurve struct {
    Percentile int       `json:"percentile"`
    Values     []float64 `json:"values"` // equity at each trade index
}
```

Algorithm:
1. Load trades from the completed backtest
2. Extract trade return percentages: `[pnl_pct_1, pnl_pct_2, ...]`
3. For each simulation (1000x):
   a. Shuffle trade returns (sampling with replacement, same length)
   b. Walk through shuffled returns starting at initial_capital
   c. Record equity curve
   d. Check if equity ever dropped below 50% of initial (ruin)
4. Compute percentile curves at each trade index
5. Compute probability of ruin = count(ruined) / total_sims

### Walk-Forward

```go
type WalkForwardResult struct {
    Windows []WalkForwardWindow `json:"windows"`
    Summary WalkForwardSummary  `json:"summary"`
}

type WalkForwardWindow struct {
    WindowIndex   int       `json:"window_index"`
    TrainStart    time.Time `json:"train_start"`
    TrainEnd      time.Time `json:"train_end"`
    TestStart     time.Time `json:"test_start"`
    TestEnd       time.Time `json:"test_end"`
    TrainSharpe   float64   `json:"train_sharpe"`
    TestSharpe    float64   `json:"test_sharpe"`
    TrainReturn   float64   `json:"train_return"`
    TestReturn    float64   `json:"test_return"`
    TrainTrades   int       `json:"train_trades"`
    TestTrades    int       `json:"test_trades"`
}

type WalkForwardSummary struct {
    AvgTestSharpe    float64 `json:"avg_test_sharpe"`
    AvgTestReturn    float64 `json:"avg_test_return"`
    ConsistencyRatio float64 `json:"consistency_ratio"` // % of windows where test_sharpe > 0
}
```

Algorithm:
1. Load backtest candle date range + strategy params
2. Split time range into N windows (default 5). Each window: 70% train, 30% test
3. For each window: run backtest on train period, run backtest on test period (same params)
4. Collect per-window metrics

### Compare Runs

```go
type CompareResult struct {
    Runs []CompareRunEntry `json:"runs"`
}

type CompareRunEntry struct {
    RunID        int64                  `json:"run_id"`
    Name         string                 `json:"name"`
    Strategy     string                 `json:"strategy"`
    Symbol       string                 `json:"symbol"`
    Timeframe    string                 `json:"timeframe"`
    Metrics      map[string]interface{} `json:"metrics"`
    EquityCurve  []EquityPoint          `json:"equity_curve"`
}
```

No computation — just loads existing backtest results and normalizes equity curves for overlay.

---

## Section 3: Analytics API

### New file: `internal/api/analytics_handler.go`

```
GET  /api/v1/backtest/runs/:id/param-heatmap?x_param=fast_period&y_param=slow_period&grid_size=10
     → starts async job, returns { job_id, status: "pending" }

POST /api/v1/backtest/runs/:id/monte-carlo
     → { num_simulations?: 1000, ruin_threshold?: 0.5 }
     → starts async job, returns { job_id, status: "pending" }

GET  /api/v1/backtest/runs/:id/walk-forward?windows=5
     → starts async job, returns { job_id, status: "pending" }

POST /api/v1/backtest/compare
     → { run_ids: [1, 2, 3] }
     → synchronous, returns CompareResult

GET  /api/v1/analytics/jobs/:jobId
     → { status, result?, error? }
```

Job lifecycle:
1. POST/GET creates an `AnalyticsResult` record with `status: "pending"`
2. Submits computation to worker pool
3. On completion: updates record to `status: "completed"` with `result` JSON
4. On error: updates to `status: "failed"` with `error_message`
5. Client polls `GET /analytics/jobs/:jobId` until complete

---

## Section 4: Frontend

### New TypeScript types

```ts
export interface HeatmapResult {
  x_param: string
  y_param: string
  x_values: number[]
  y_values: number[]
  cells: HeatmapCell[][]
}

export interface HeatmapCell {
  x_value: number
  y_value: number
  sharpe_ratio: number
  total_return: number
  max_drawdown: number
  total_trades: number
}

export interface MonteCarloResult {
  num_simulations: number
  percentiles: Array<{ percentile: number; values: number[] }>
  probability_of_ruin: number
  median_return: number
  min_return: number
  max_return: number
}

export interface WalkForwardResult {
  windows: WalkForwardWindow[]
  summary: { avg_test_sharpe: number; avg_test_return: number; consistency_ratio: number }
}

export interface WalkForwardWindow {
  window_index: number
  train_sharpe: number
  test_sharpe: number
  train_return: number
  test_return: number
}

export interface CompareResult {
  runs: Array<{
    run_id: number
    name: string
    strategy: string
    symbol: string
    metrics: Record<string, unknown>
    equity_curve: Array<{ timestamp: string; value: number }>
  }>
}
```

### "Advanced Analytics" Tab

Added as a new tab in the Backtest results panel (alongside Overview, Trades, Chart, Bookmarks).

Layout:
```
┌─── Advanced Analytics ────────────────────────────────────────┐
│                                                               │
│  ┌─ Param Heatmap ─────────────────────────────────────────┐ │
│  │  X axis: [fast_period ▾]   Y axis: [slow_period ▾]     │ │
│  │  Grid size: [10]                    [Generate]          │ │
│  │                                                         │ │
│  │  Color-coded grid (red → green = Sharpe)                │ │
│  │  Hover: tooltip with all metrics                        │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌─ Monte Carlo ───────────────────────────────────────────┐ │
│  │  Simulations: [1000]          [Run Simulation]          │ │
│  │                                                         │ │
│  │  Fan chart (5th–95th percentile shaded area)            │ │
│  │  Probability of ruin: 3.2%                              │ │
│  │  Median return: +28.4%                                  │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌─ Walk-Forward ──────────────────────────────────────────┐ │
│  │  Windows: [5]                 [Run Analysis]            │ │
│  │                                                         │ │
│  │  Bar chart: train Sharpe (blue) vs test Sharpe (orange) │ │
│  │  Consistency ratio: 80%                                 │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌─ Compare Runs ──────────────────────────────────────────┐ │
│  │  [Select runs: ✓ Run 1  ✓ Run 3  ✓ Run 5]  [Compare]  │ │
│  │                                                         │ │
│  │  Comparison table + overlaid equity curves               │ │
│  └─────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
```

### New Frontend Files

```
frontend/src/api/analytics.ts                        API client functions
frontend/src/hooks/useAnalytics.ts                   React Query hooks with polling
frontend/src/components/analytics/ParamHeatmap.tsx    Heatmap grid with color scale
frontend/src/components/analytics/MonteCarloChart.tsx Fan chart (Recharts area)
frontend/src/components/analytics/WalkForwardChart.tsx Bar chart (Recharts)
frontend/src/components/analytics/CompareRuns.tsx     Table + overlaid equity chart
frontend/src/components/analytics/AnalyticsTab.tsx    Container for all 4 sections
```

---

## Implementation Order

```
B1: Add AnalyticsResult model → autoMigrate                     (no deps)
B2: analytics/compare.go + tests                                 (needs B1 — simplest, no async)
B3: analytics/montecarlo.go + tests                              (no deps on B2)
B4: analytics/heatmap.go + tests                                 (needs worker pool understanding)
B5: analytics/walkforward.go + tests                             (needs B4 pattern)
B6: api/analytics_handler.go — all endpoints + job polling       (needs B1-B5)
B7: Wire routes + main.go                                        (needs B6)
F1: types + api/analytics.ts                                     (no deps)
F2: hooks/useAnalytics.ts (with polling for async jobs)          (needs F1)
F3: components/analytics/CompareRuns.tsx                          (needs F2)
F4: components/analytics/MonteCarloChart.tsx                      (needs F2)
F5: components/analytics/ParamHeatmap.tsx                         (needs F2)
F6: components/analytics/WalkForwardChart.tsx                     (needs F2)
F7: components/analytics/AnalyticsTab.tsx — wire all together     (needs F3-F6)
F8: Add "Advanced Analytics" tab to Backtest results              (needs F7)
```

---

## Testing Requirements

| Task | Tests |
|---|---|
| B2 | Compare: loads N runs, returns correct metrics and equity data |
| B3 | Monte Carlo: 1000 sims produce expected percentile count, ruin probability in [0,1] |
| B4 | Heatmap: 5×5 grid produces 25 cells, parallel execution completes, Sharpe values valid |
| B5 | Walk-forward: 5 windows split correctly, each has train+test metrics |
| B6 | API endpoints: async job creation, polling returns completed result |
| F5 | Heatmap: renders grid, hover shows tooltip |
| F4 | Monte Carlo: fan chart renders, ruin stat displayed |
