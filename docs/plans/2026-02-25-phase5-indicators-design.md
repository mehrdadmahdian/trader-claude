# Phase 5 — Technical Indicators: Design Document

**Date:** 2026-02-25
**Status:** Approved
**Execution strategy:** 2-Wave Parallel (Approach A)

---

## Overview

Add 13 technical indicators (7 overlay, 6 panel) to the Chart page. Calculations are stateless and run on the backend. The frontend renders overlay indicators as line series on the main chart and panel indicators in separate panes below.

---

## Architecture

```
backend/internal/indicator/
  types.go         shared types (IndicatorMeta, CalcResult, CalcFunc, OutputDef)
  overlay.go       SMA, EMA, WMA, BollingerBands, VWAP, ParabolicSAR, Ichimoku
  overlay_test.go
  panel.go         RSI, MACD, Stochastic, ATR, OBV, Volume
  panel_test.go
  registry.go      catalog: id → (meta + CalcFunc)
  handler.go       GET /indicators, POST /indicators/calculate

frontend/src/
  api/indicators.ts
  components/chart/
    IndicatorModal.tsx
    IndicatorParamForm.tsx
    IndicatorChips.tsx
    PanelChart.tsx
```

---

## Backend — Shared Types

The indicator package accepts `[]registry.Candle` to avoid duplicating the existing type.

```go
// CalcFunc is the signature every indicator calculation must implement.
type CalcFunc func(candles []registry.Candle, params map[string]interface{}) (CalcResult, error)

// CalcResult holds parallel arrays of timestamps and named output series.
// Leading NaN values represent the warm-up period before the indicator stabilises.
type CalcResult struct {
    Timestamps []int64              // Unix seconds
    Series     map[string][]float64 // named outputs: "value", "upper", "lower", etc.
}

// IndicatorMeta describes one indicator to the frontend.
type IndicatorMeta struct {
    ID       string
    Name     string   // short name, e.g. "EMA"
    FullName string   // e.g. "Exponential Moving Average"
    Type     string   // "overlay" | "panel"
    Group    string   // "trend" | "momentum" | "volatility" | "volume"
    Params   []registry.ParamDefinition
    Outputs  []OutputDef
}

// OutputDef describes one series within a CalcResult (name + default chart colour).
type OutputDef struct {
    Name  string // e.g. "value", "signal", "histogram"
    Color string // default hex colour for the frontend
}
```

---

## Indicators

### Overlay (rendered on main chart pane)

| ID | Name | Key Params | Outputs |
|----|------|-----------|---------|
| `sma` | SMA | period | value |
| `ema` | EMA | period | value |
| `wma` | WMA | period | value |
| `bollinger_bands` | Bollinger Bands | period, std_dev | upper, middle, lower |
| `vwap` | VWAP | — | value |
| `parabolic_sar` | Parabolic SAR | step, max | value |
| `ichimoku` | Ichimoku Cloud | tenkan, kijun, senkou_b, displacement | tenkan, kijun, senkou_a, senkou_b, chikou |

### Panel (rendered in separate panes below main chart)

| ID | Name | Key Params | Outputs |
|----|------|-----------|---------|
| `rsi` | RSI | period | value |
| `macd` | MACD | fast, slow, signal | macd, signal, histogram |
| `stochastic` | Stochastic | k_period, d_period, smooth | k, d |
| `atr` | ATR | period | value |
| `obv` | OBV | — | value |
| `volume` | Volume | — | value (coloured up/down) |

---

## API Contract

### GET /api/v1/indicators
Returns full indicator catalog with metadata and param schemas.

```json
{
  "indicators": [
    {
      "id": "ema",
      "name": "EMA",
      "full_name": "Exponential Moving Average",
      "type": "overlay",
      "group": "trend",
      "params": [
        { "Name": "period", "Type": "int", "Default": 20, "Min": 2, "Max": 500, "Description": "Lookback period" }
      ],
      "outputs": [
        { "name": "value", "color": "#2962FF" }
      ]
    }
  ]
}
```

### POST /api/v1/indicators/calculate
Accepts candles from the client (already in React Query cache — no second DB round-trip).

Request:
```json
{
  "indicator_id": "bollinger_bands",
  "params": { "period": 20, "std_dev": 2.0 },
  "candles": [
    { "timestamp": 1700000000, "open": 42000, "high": 43000, "low": 41500, "close": 42500, "volume": 1234.5 }
  ]
}
```

Response:
```json
{
  "timestamps": [1700000000, 1700003600],
  "series": {
    "upper":  [null, 44200.5],
    "middle": [null, 43100.0],
    "lower":  [null, 42000.5]
  }
}
```

NaN values in the warm-up period are serialised as `null` in JSON. The frontend skips null points when building chart series.

---

## Frontend Flow

1. Chart page toolbar gains an **"Indicators" button**.
2. Clicking opens a **searchable modal** grouped by: Trend, Momentum, Volatility, Volume.
3. Selecting an indicator shows an **auto-generated param form** (reuses the `ParamDefinition` schema, same pattern as the Backtest param form).
4. Confirming adds an **active indicator chip** to the toolbar. Chips are clickable (re-open param form to edit) and have an × to remove.
5. Active indicators are **persisted to `localStorage`** as `[{id, params}]` keyed by `symbol:timeframe`.
6. On candle load or indicator change, the frontend POSTs to `/indicators/calculate` per active indicator (React Query, keyed by `[indicator_id, params, candles_hash]`).
7. Results are rendered:
   - **Overlay**: `chart.addLineSeries()` per output. BBands adds 3 line series + filled area between upper/lower. Ichimoku adds 5 lines + Kumo cloud fill between senkou_a and senkou_b.
   - **Panel**: each panel indicator gets its own `<PanelChart>` component (separate lightweight-charts instance) stacked below the main chart. Each panel has a header (name + params summary) and a close button. MACD uses `addHistogramSeries()` for the histogram output.

### State model

```ts
interface ActiveIndicator {
  meta: IndicatorMeta
  params: Record<string, unknown>
  result?: CalcResult       // populated after fetch, undefined while loading
}
```

State lives in React `useState` on the Chart page (page-local, not Zustand). Persisted subset `{id, params}[]` goes to `localStorage`.

---

## Execution Plan — 2-Wave Parallel

### Pre-step (main session)
- Write `internal/indicator/types.go` so both agents share the same foundation.

### Wave 1 — Parallel worktrees

**Agent A** (`worktree-overlay`):
- `internal/indicator/overlay.go` — all 7 overlay functions
- `internal/indicator/overlay_test.go` — known-input/expected-output tests for each

**Agent B** (`worktree-panel`):
- `internal/indicator/panel.go` — all 6 panel functions
- `internal/indicator/panel_test.go` — known-input/expected-output tests for each

Both agents import `types.go` from the same package but write to separate files — zero merge conflict risk.

### Wave 2 — Sequential (after Wave 1 merge)

1. `internal/indicator/registry.go` — wire all 13 indicators into the catalog
2. `internal/indicator/handler.go` — Fiber handlers
3. Add routes to `internal/api/routes.go`
4. API integration tests
5. Frontend: `types/index.ts` additions, `api/indicators.ts`, `IndicatorModal`, `IndicatorChips`, `IndicatorParamForm`, `PanelChart`
6. Wire Chart page: overlay rendering + panel panes

---

## Testing

- `overlay_test.go`: each function tested with a hand-calculated known sequence
- `panel_test.go`: same pattern, including RSI divergence, MACD crossover, Stochastic overbought/oversold
- API integration test: POST with minimal candle set, assert response shape and series length
- Frontend: localStorage persistence, modal open/close, chip add/remove

---

## Non-goals (deferred)

- Server-side caching of calculated indicator values (not needed at this scale)
- Streaming indicator updates via WebSocket (calculate on demand is sufficient)
- Drawing tools / annotations on chart
