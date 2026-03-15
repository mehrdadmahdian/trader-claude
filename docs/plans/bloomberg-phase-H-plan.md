# Bloomberg Terminal — Phase H: Risk Analytics

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `RISK` widget to the Bloomberg terminal that calculates and displays key portfolio risk metrics — VaR, CVaR, Sharpe ratio, Sortino ratio, max drawdown, annualized volatility, per-symbol breakdowns, stress test scenarios, and a return distribution histogram — all computed in a pure-Go analytics package using historical candle data already stored in the database.

**Architecture:** A new `backend/internal/analytics/risk.go` package performs all math using only the standard `math` package (no external dependencies). A new `backend/internal/api/risk_handler.go` handler resolves symbols (from portfolio ID or direct list), fetches daily candles from the DB, delegates to the analytics package, and returns structured JSON. A new `frontend/src/api/risk.ts` wraps the single `POST /api/v1/risk/analyze` call as a React Query `useMutation`. `frontend/src/components/widgets/RiskWidget.tsx` renders the full UI. `WidgetRegistry.tsx` is updated to wire in the real component in place of the `ComingSoon` stub.

**Tech Stack:** Go `math` package (pure math, no new Go dependencies), GORM (candle queries), Fiber v2 (handler), React Query `useMutation`, recharts `BarChart` (histogram — already installed), lucide-react icons, Tailwind CSS.

---

## Task H1: Create the Go risk analytics package

**Files:**
- Create: `backend/internal/analytics/risk.go`

**Step 1: Create the file**

```go
package analytics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

// ── Request / Response types ──────────────────────────────────────────────

// RiskRequest is the validated input to Analyze (fields already defaulted by handler).
type RiskRequest struct {
	Symbols         []string
	Weights         []float64
	LookbackDays    int
	ConfidenceLevel float64 // e.g. 0.95
}

// SymbolRisk holds per-asset metrics.
type SymbolRisk struct {
	Symbol      string  `json:"symbol"`
	Weight      float64 `json:"weight"`
	VaR95       float64 `json:"var_95"`
	CVaR95      float64 `json:"cvar_95"`
	Sharpe      float64 `json:"sharpe"`
	Sortino     float64 `json:"sortino"`
	MaxDrawdown float64 `json:"max_drawdown"`
	Volatility  float64 `json:"volatility"`
}

// StressResult holds the estimated portfolio impact of one named scenario.
type StressResult struct {
	Name            string  `json:"name"`
	ScenarioReturn  float64 `json:"scenario_return"`  // e.g. -0.34 = -34%
	DurationDays    int     `json:"duration_days"`
	PortfolioImpact float64 `json:"portfolio_impact"` // weighted sum
	Description     string  `json:"description"`
}

// ReturnBucket is one bar in the daily-return histogram.
type ReturnBucket struct {
	RangeLabel string  `json:"range_label"` // e.g. "-3% to -2%"
	MidPoint   float64 `json:"mid_point"`   // centre of bucket as fraction
	Count      int     `json:"count"`
}

// RiskMetrics is the full response payload.
type RiskMetrics struct {
	Symbols         []string       `json:"symbols"`
	Weights         []float64      `json:"weights"`
	LookbackDays    int            `json:"lookback_days"`
	VaR95           float64        `json:"var_95"`
	CVaR95          float64        `json:"cvar_95"`
	SharpeRatio     float64        `json:"sharpe_ratio"`
	SortinoRatio    float64        `json:"sortino_ratio"`
	MaxDrawdown     float64        `json:"max_drawdown"`
	Volatility      float64        `json:"volatility"`
	SymbolMetrics   []SymbolRisk   `json:"symbol_metrics"`
	StressScenarios []StressResult `json:"stress_scenarios"`
	ReturnBuckets   []ReturnBucket `json:"return_buckets"`
}

// ── Hardcoded stress scenarios ─────────────────────────────────────────────

var stressScenarios = []struct {
	name            string
	scenarioReturn  float64
	durationDays    int
	description     string
}{
	{
		name:           "2020 COVID Crash",
		scenarioReturn: -0.34,
		durationDays:   33,
		description:    "S&P 500 peak-to-trough decline, Feb–Mar 2020",
	},
	{
		name:           "2008 GFC",
		scenarioReturn: -0.57,
		durationDays:   517,
		description:    "Global Financial Crisis peak-to-trough, Oct 2007–Mar 2009",
	},
	{
		name:           "2022 Bear Market",
		scenarioReturn: -0.25,
		durationDays:   282,
		description:    "Fed rate-hike bear market, Jan–Oct 2022",
	},
}

// ── Public entry point ─────────────────────────────────────────────────────

// Analyze fetches daily candles from the DB and computes all risk metrics.
func Analyze(ctx context.Context, db *gorm.DB, req RiskRequest) (*RiskMetrics, error) {
	if len(req.Symbols) == 0 {
		return nil, fmt.Errorf("risk: at least one symbol is required")
	}
	if len(req.Weights) != len(req.Symbols) {
		return nil, fmt.Errorf("risk: weights length %d does not match symbols length %d",
			len(req.Weights), len(req.Symbols))
	}

	from := time.Now().UTC().AddDate(0, 0, -req.LookbackDays)
	to := time.Now().UTC()

	// Fetch close prices for every symbol
	seriesMap := make(map[string][]float64, len(req.Symbols))
	for _, sym := range req.Symbols {
		prices, err := fetchClosePrices(ctx, db, sym, from, to)
		if err != nil {
			return nil, fmt.Errorf("risk: fetch prices for %s: %w", sym, err)
		}
		if len(prices) < 2 {
			return nil, fmt.Errorf("risk: not enough daily candles for %s (got %d, need ≥2)", sym, len(prices))
		}
		seriesMap[sym] = prices
	}

	// Build portfolio daily return series (weighted sum of individual returns)
	portfolioReturns := portfolioReturnSeries(req.Symbols, req.Weights, seriesMap)

	// Per-symbol metrics
	symbolMetrics := make([]SymbolRisk, len(req.Symbols))
	for i, sym := range req.Symbols {
		returns := dailyReturns(seriesMap[sym])
		symbolMetrics[i] = SymbolRisk{
			Symbol:      sym,
			Weight:      req.Weights[i],
			VaR95:       computeVaR(returns, req.ConfidenceLevel),
			CVaR95:      computeCVaR(returns, req.ConfidenceLevel),
			Sharpe:      computeSharpe(returns),
			Sortino:     computeSortino(returns),
			MaxDrawdown: computeMaxDrawdown(seriesMap[sym]),
			Volatility:  computeVolatility(returns),
		}
	}

	// Portfolio-level metrics
	portfolioVar := computeVaR(portfolioReturns, req.ConfidenceLevel)
	portfolioCVar := computeCVaR(portfolioReturns, req.ConfidenceLevel)
	portfolioSharpe := computeSharpe(portfolioReturns)
	portfolioSortino := computeSortino(portfolioReturns)
	portfolioVol := computeVolatility(portfolioReturns)

	// Max drawdown on portfolio equity curve (start at 1.0)
	portfolioEquity := returnsToEquityCurve(portfolioReturns)
	portfolioMDD := computeMaxDrawdown(portfolioEquity)

	// Stress scenarios
	stressResults := make([]StressResult, len(stressScenarios))
	for i, sc := range stressScenarios {
		impact := 0.0
		for j, w := range req.Weights {
			_ = req.Symbols[j] // weight applied uniformly per scenario
			impact += w * sc.scenarioReturn
		}
		stressResults[i] = StressResult{
			Name:            sc.name,
			ScenarioReturn:  sc.scenarioReturn,
			DurationDays:    sc.durationDays,
			PortfolioImpact: impact,
			Description:     sc.description,
		}
	}

	// Return distribution histogram (portfolio returns)
	buckets := buildHistogram(portfolioReturns, 20)

	return &RiskMetrics{
		Symbols:         req.Symbols,
		Weights:         req.Weights,
		LookbackDays:    req.LookbackDays,
		VaR95:           portfolioVar,
		CVaR95:          portfolioCVar,
		SharpeRatio:     portfolioSharpe,
		SortinoRatio:    portfolioSortino,
		MaxDrawdown:     portfolioMDD,
		Volatility:      portfolioVol,
		SymbolMetrics:   symbolMetrics,
		StressScenarios: stressResults,
		ReturnBuckets:   buckets,
	}, nil
}

// ── DB helper ──────────────────────────────────────────────────────────────

func fetchClosePrices(ctx context.Context, db *gorm.DB, symbol string, from, to time.Time) ([]float64, error) {
	var candles []models.Candle
	if err := db.WithContext(ctx).
		Where("symbol = ? AND timeframe = ? AND timestamp BETWEEN ? AND ?", symbol, "1d", from, to).
		Order("timestamp ASC").
		Find(&candles).Error; err != nil {
		return nil, err
	}
	prices := make([]float64, len(candles))
	for i, c := range candles {
		prices[i] = c.Close
	}
	return prices, nil
}

// ── Math helpers ───────────────────────────────────────────────────────────

// dailyReturns computes (p[t]-p[t-1])/p[t-1] for each consecutive pair.
func dailyReturns(prices []float64) []float64 {
	if len(prices) < 2 {
		return nil
	}
	r := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] != 0 {
			r[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
		}
	}
	return r
}

// portfolioReturnSeries blends per-symbol daily return slices by weight.
// Uses only the minimum common length across all series.
func portfolioReturnSeries(symbols []string, weights []float64, seriesMap map[string][]float64) []float64 {
	minLen := math.MaxInt32
	returnMaps := make([][]float64, len(symbols))
	for i, sym := range symbols {
		r := dailyReturns(seriesMap[sym])
		returnMaps[i] = r
		if len(r) < minLen {
			minLen = len(r)
		}
	}
	if minLen == math.MaxInt32 || minLen == 0 {
		return nil
	}
	combined := make([]float64, minLen)
	for t := 0; t < minLen; t++ {
		for i, w := range weights {
			combined[t] += w * returnMaps[i][t]
		}
	}
	return combined
}

// computeVaR returns the historical Value at Risk at the given confidence level
// as a negative fraction. E.g. -0.023 means 2.3% 1-day VaR.
func computeVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	sorted := make([]float64, len(returns))
	copy(sorted, returns)
	sort.Float64s(sorted)
	// (1 - confidence) percentile from the left tail
	idx := int(math.Floor((1.0 - confidence) * float64(len(sorted))))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// computeCVaR returns the Expected Shortfall — mean of returns below the VaR threshold.
func computeCVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	varThreshold := computeVaR(returns, confidence)
	sum := 0.0
	count := 0
	for _, r := range returns {
		if r <= varThreshold {
			sum += r
			count++
		}
	}
	if count == 0 {
		return varThreshold
	}
	return sum / float64(count)
}

// computeSharpe returns the annualized Sharpe ratio using a 5% risk-free rate.
func computeSharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	const riskFreeRate = 0.05
	mean := meanOf(returns)
	std := stdDevOf(returns)
	if std == 0 {
		return 0
	}
	annualizedReturn := mean * 252
	annualizedStd := std * math.Sqrt(252)
	return (annualizedReturn - riskFreeRate) / annualizedStd
}

// computeSortino returns the annualized Sortino ratio using a 5% risk-free rate.
// Only negative returns contribute to the downside deviation.
func computeSortino(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	const riskFreeRate = 0.05
	mean := meanOf(returns)

	sumSqNeg := 0.0
	countNeg := 0
	for _, r := range returns {
		if r < 0 {
			sumSqNeg += r * r
			countNeg++
		}
	}
	if countNeg == 0 {
		return 0
	}
	downsideStd := math.Sqrt(sumSqNeg / float64(len(returns)))
	if downsideStd == 0 {
		return 0
	}
	annualizedReturn := mean * 252
	annualizedDownside := downsideStd * math.Sqrt(252)
	return (annualizedReturn - riskFreeRate) / annualizedDownside
}

// computeMaxDrawdown returns the maximum peak-to-trough decline as a negative fraction
// given a price (or equity) series.
func computeMaxDrawdown(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}
	peak := prices[0]
	maxDD := 0.0
	for _, p := range prices {
		if p > peak {
			peak = p
		}
		if peak != 0 {
			dd := (p - peak) / peak
			if dd < maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

// computeVolatility returns annualized standard deviation of daily returns.
func computeVolatility(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	return stdDevOf(returns) * math.Sqrt(252)
}

// returnsToEquityCurve converts a daily return series to a cumulative equity
// curve starting at 1.0.
func returnsToEquityCurve(returns []float64) []float64 {
	curve := make([]float64, len(returns)+1)
	curve[0] = 1.0
	for i, r := range returns {
		curve[i+1] = curve[i] * (1 + r)
	}
	return curve
}

// buildHistogram divides the return series into numBuckets equal-width bins
// and returns a slice of ReturnBucket for the bar chart.
func buildHistogram(returns []float64, numBuckets int) []ReturnBucket {
	if len(returns) == 0 || numBuckets <= 0 {
		return nil
	}
	min, max := returns[0], returns[0]
	for _, r := range returns {
		if r < min {
			min = r
		}
		if r > max {
			max = r
		}
	}
	if min == max {
		return nil
	}
	bucketWidth := (max - min) / float64(numBuckets)
	counts := make([]int, numBuckets)
	for _, r := range returns {
		idx := int((r - min) / bucketWidth)
		if idx >= numBuckets {
			idx = numBuckets - 1
		}
		counts[idx]++
	}
	buckets := make([]ReturnBucket, numBuckets)
	for i := 0; i < numBuckets; i++ {
		lo := min + float64(i)*bucketWidth
		hi := lo + bucketWidth
		mid := (lo + hi) / 2
		buckets[i] = ReturnBucket{
			RangeLabel: fmt.Sprintf("%.1f%% to %.1f%%", lo*100, hi*100),
			MidPoint:   mid,
			Count:      counts[i],
		}
	}
	return buckets
}

// ── Stat helpers ───────────────────────────────────────────────────────────

func meanOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stdDevOf(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := meanOf(xs)
	s := 0.0
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./internal/analytics/...
```
Expected: no errors.

**Step 3: Commit**
```bash
git add backend/internal/analytics/risk.go
git commit -m "feat(analytics): add pure-Go risk analytics package — VaR, CVaR, Sharpe, Sortino, drawdown, stress tests"
```

---

## Task H2: Write unit tests for the risk package

**Files:**
- Create: `backend/internal/analytics/risk_test.go`

**Step 1: Create the test file**

```go
package analytics

import (
	"math"
	"testing"
)

func TestDailyReturns(t *testing.T) {
	prices := []float64{100, 110, 99}
	r := dailyReturns(prices)
	if len(r) != 2 {
		t.Fatalf("expected 2 returns, got %d", len(r))
	}
	// (110 - 100) / 100 = 0.10
	if math.Abs(r[0]-0.10) > 1e-9 {
		t.Errorf("first return: got %f, want 0.10", r[0])
	}
	// (99 - 110) / 110 ≈ -0.1
	want := (99.0 - 110.0) / 110.0
	if math.Abs(r[1]-want) > 1e-9 {
		t.Errorf("second return: got %f, want %f", r[1], want)
	}
}

func TestComputeVaR(t *testing.T) {
	// 10 returns; 5th percentile = worst return in tail
	returns := []float64{-0.05, -0.04, -0.03, -0.02, -0.01, 0.01, 0.02, 0.03, 0.04, 0.05}
	v := computeVaR(returns, 0.95)
	if v >= 0 {
		t.Errorf("VaR should be negative, got %f", v)
	}
}

func TestComputeCVaR_LessThanVaR(t *testing.T) {
	returns := []float64{-0.10, -0.08, -0.05, -0.03, -0.01, 0.01, 0.02, 0.03, 0.04, 0.05}
	v := computeVaR(returns, 0.95)
	cv := computeCVaR(returns, 0.95)
	if cv > v {
		t.Errorf("CVaR (%f) should be <= VaR (%f)", cv, v)
	}
}

func TestComputeMaxDrawdown(t *testing.T) {
	prices := []float64{100, 120, 90, 110, 80}
	dd := computeMaxDrawdown(prices)
	// Peak = 120, trough = 80: (80-120)/120 ≈ -0.3333
	want := (80.0 - 120.0) / 120.0
	if math.Abs(dd-want) > 1e-9 {
		t.Errorf("max drawdown: got %f, want %f", dd, want)
	}
}

func TestComputeSharpe_ZeroStd(t *testing.T) {
	// All returns the same — std = 0 → should return 0, not panic
	returns := []float64{0.01, 0.01, 0.01, 0.01}
	s := computeSharpe(returns)
	if s != 0 {
		t.Errorf("Sharpe with zero std should be 0, got %f", s)
	}
}

func TestComputeVolatility(t *testing.T) {
	returns := make([]float64, 252)
	for i := range returns {
		returns[i] = 0.01
	}
	vol := computeVolatility(returns)
	// std of constant returns is 0
	if vol != 0 {
		t.Errorf("volatility of constant returns should be 0, got %f", vol)
	}
}

func TestBuildHistogram(t *testing.T) {
	returns := make([]float64, 100)
	for i := range returns {
		returns[i] = float64(i-50) * 0.001
	}
	buckets := buildHistogram(returns, 10)
	if len(buckets) != 10 {
		t.Fatalf("expected 10 buckets, got %d", len(buckets))
	}
	total := 0
	for _, b := range buckets {
		total += b.Count
	}
	if total != 100 {
		t.Errorf("bucket counts should sum to 100, got %d", total)
	}
}

func TestPortfolioReturnSeries(t *testing.T) {
	seriesMap := map[string][]float64{
		"A": {100, 110, 99},
		"B": {200, 210, 220},
	}
	pr := portfolioReturnSeries([]string{"A", "B"}, []float64{0.5, 0.5}, seriesMap)
	if len(pr) != 2 {
		t.Fatalf("expected 2 portfolio returns, got %d", len(pr))
	}
}
```

**Step 2: Run tests**

```bash
docker compose exec backend go test ./internal/analytics/... -run TestDaily -v
docker compose exec backend go test ./internal/analytics/... -run TestCompute -v
docker compose exec backend go test ./internal/analytics/... -run TestBuild -v
docker compose exec backend go test ./internal/analytics/... -run TestPortfolio -v
```
Expected: all tests PASS.

**Step 3: Run full analytics suite to confirm no regressions**

```bash
docker compose exec backend go test ./internal/analytics/... -v
```
Expected: all existing tests (montecarlo, heatmap, walkforward, compare) plus new risk tests pass.

**Step 4: Commit**
```bash
git add backend/internal/analytics/risk_test.go
git commit -m "test(analytics): add unit tests for risk analytics package"
```

---

## Task H3: Create the risk handler

**Files:**
- Create: `backend/internal/api/risk_handler.go`

**Step 1: Create the handler file**

```go
package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/analytics"
	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
)

type riskHandler struct{ db *gorm.DB }

func newRiskHandler(db *gorm.DB) *riskHandler {
	return &riskHandler{db: db}
}

// RiskAnalyzeRequest is the JSON body for POST /api/v1/risk/analyze.
type RiskAnalyzeRequest struct {
	PortfolioID     *int64    `json:"portfolio_id"`      // optional — resolve symbols from portfolio
	Symbols         []string  `json:"symbols"`           // alternative: supply symbols directly
	Weights         []float64 `json:"weights"`           // portfolio weights; must sum ≈ 1.0; if empty, equal weight
	LookbackDays    int       `json:"lookback_days"`     // default 252
	ConfidenceLevel float64   `json:"confidence_level"`  // default 0.95
}

// POST /api/v1/risk/analyze
func (h *riskHandler) analyze(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)

	var req RiskAnalyzeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Apply defaults
	if req.LookbackDays <= 0 {
		req.LookbackDays = 252
	}
	if req.ConfidenceLevel <= 0 || req.ConfidenceLevel >= 1 {
		req.ConfidenceLevel = 0.95
	}

	// Resolve symbols — portfolio path
	if req.PortfolioID != nil {
		var portfolio models.Portfolio
		if err := h.db.
			Where("id = ? AND user_id = ?", *req.PortfolioID, userID).
			Preload("Positions").
			First(&portfolio).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "portfolio not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load portfolio"})
		}
		if len(portfolio.Positions) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "portfolio has no positions"})
		}
		// Extract unique symbols from positions
		seen := map[string]bool{}
		for _, pos := range portfolio.Positions {
			seen[pos.Symbol] = true
		}
		req.Symbols = make([]string, 0, len(seen))
		for sym := range seen {
			req.Symbols = append(req.Symbols, sym)
		}
	}

	// Validate symbols
	if len(req.Symbols) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbols are required (or provide portfolio_id)"})
	}
	if len(req.Symbols) > 20 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "maximum 20 symbols per request"})
	}

	// Resolve weights — equal weight if not provided or wrong length
	if len(req.Weights) != len(req.Symbols) {
		n := float64(len(req.Symbols))
		req.Weights = make([]float64, len(req.Symbols))
		for i := range req.Weights {
			req.Weights[i] = 1.0 / n
		}
	} else {
		// Validate weights sum to ≈ 1.0
		sum := 0.0
		for _, w := range req.Weights {
			if w < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "weights must be non-negative"})
			}
			sum += w
		}
		if sum < 0.99 || sum > 1.01 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "weights must sum to 1.0"})
		}
	}

	result, err := analytics.Analyze(c.Context(), h.db, analytics.RiskRequest{
		Symbols:         req.Symbols,
		Weights:         req.Weights,
		LookbackDays:    req.LookbackDays,
		ConfidenceLevel: req.ConfidenceLevel,
	})
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(result)
}
```

**Step 2: Check that `auth.GetUserID` exists (same pattern as workspace_handler.go)**

```bash
grep -n "func GetUserID" backend/internal/auth/
```
Expected: the function is present. If the function is named differently, check:
```bash
grep -rn "func Get" backend/internal/auth/
```
Adapt the import call to match whatever the real function name is.

**Step 3: Check that `models.Portfolio` has a `Positions` association**

```bash
grep -n "Portfolio\|Position" backend/internal/models/models.go | head -30
```
If the Portfolio model does not have a `Positions` field with a `has many` GORM relationship, remove the `Preload("Positions")` line and instead fetch positions separately with:
```go
var positions []models.Position
h.db.Where("portfolio_id = ?", *req.PortfolioID).Find(&positions)
```
Adjust symbol extraction accordingly.

**Step 4: Build check**

```bash
docker compose exec backend go build ./...
```
Expected: no errors.

**Step 5: Commit**
```bash
git add backend/internal/api/risk_handler.go
git commit -m "feat(api): add risk analysis handler — POST /api/v1/risk/analyze"
```

---

## Task H4: Register the risk route

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Add the handler init** in `RegisterRoutes`, immediately after the workspace handler block (around line 163):

```go
// --- Risk Analytics (Bloomberg terminal) ---
riskH := newRiskHandler(db)
```

**Step 2: Add the route** in the `protected` group, after the workspaces block:

```go
// Risk Analytics
protected.Post("/risk/analyze", mutationLimiter, riskH.analyze)
```

**Step 3: Build + smoke test**

```bash
docker compose exec backend go build ./...
make up
# After rebuild, verify route exists (will get 401 without token, not 404):
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/api/v1/risk/analyze
```
Expected: `401` (unauthorized — not 404, confirming route is registered).

**Step 4: Commit**
```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register POST /api/v1/risk/analyze under protected group"
```

---

## Task H5: Integration smoke test — end-to-end backend

This task verifies the backend pipeline with a real auth token. It does not require production data — it exercises the error paths.

**Step 1: Get an auth token**

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"your@email.com","password":"yourpassword"}' | jq -r '.access_token')
echo $TOKEN
```

**Step 2: Test with missing symbols (expect 400)**

```bash
curl -s -X POST http://localhost:8080/api/v1/risk/analyze \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' | jq .
```
Expected: `{"error": "symbols are required (or provide portfolio_id)"}`

**Step 3: Test with invalid weights (expect 400)**

```bash
curl -s -X POST http://localhost:8080/api/v1/risk/analyze \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"symbols":["BTCUSDT"],"weights":[0.5]}' | jq .
```
Expected: `{"error": "weights must sum to 1.0"}` or, because length mismatch triggers equal-weight fallback, the request is reprocessed with equal weight (1.0 for a single symbol). Either is acceptable — weights are silently corrected to equal when length mismatches.

**Step 4: Test with a real symbol (will fail gracefully if no daily candles in DB)**

```bash
curl -s -X POST http://localhost:8080/api/v1/risk/analyze \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"symbols":["BTCUSDT"],"lookback_days":30}' | jq .
```
If candles are present: full `RiskMetrics` JSON is returned.
If not: `{"error":"risk: not enough daily candles for BTCUSDT (got 0, need ≥2)"}`

Both outcomes confirm the handler is wired correctly.

**Step 5: Commit**
No code changes in this task. If any bugs were found and fixed, commit them with:
```bash
git add backend/internal/api/risk_handler.go backend/internal/analytics/risk.go
git commit -m "fix(risk): correct handler/analytics bugs found in smoke test"
```

---

## Task H6: Add TypeScript types for risk

**Files:**
- Modify: `frontend/src/types/terminal.ts`

**Step 1: Append the risk types** at the end of `frontend/src/types/terminal.ts`:

```typescript
// ── Risk Analytics (Phase H) ────────────────────────────────────────────

export interface RiskAnalyzeRequest {
  portfolio_id?: number
  symbols?: string[]
  weights?: number[]
  lookback_days?: number       // default 252
  confidence_level?: number    // default 0.95
}

export interface SymbolRisk {
  symbol: string
  weight: number
  var_95: number
  cvar_95: number
  sharpe: number
  sortino: number
  max_drawdown: number
  volatility: number
}

export interface StressResult {
  name: string
  scenario_return: number   // e.g. -0.34 = -34%
  duration_days: number
  portfolio_impact: number
  description: string
}

export interface ReturnBucket {
  range_label: string
  mid_point: number
  count: number
}

export interface RiskMetrics {
  symbols: string[]
  weights: number[]
  lookback_days: number
  var_95: number
  cvar_95: number
  sharpe_ratio: number
  sortino_ratio: number
  max_drawdown: number
  volatility: number
  symbol_metrics: SymbolRisk[]
  stress_scenarios: StressResult[]
  return_buckets: ReturnBucket[]
}
```

**Step 2: TypeScript compile check**

```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 3: Commit**
```bash
git add frontend/src/types/terminal.ts
git commit -m "feat(types): add RiskMetrics, SymbolRisk, StressResult, ReturnBucket interfaces for Phase H"
```

---

## Task H7: Create the risk API client

**Files:**
- Create: `frontend/src/api/risk.ts`

**Step 1: Create the file**

```typescript
import apiClient from '@/api/client'
import { useMutation } from '@tanstack/react-query'
import type { RiskAnalyzeRequest, RiskMetrics } from '@/types/terminal'

export async function analyzeRisk(req: RiskAnalyzeRequest): Promise<RiskMetrics> {
  const { data } = await apiClient.post<RiskMetrics>('/risk/analyze', req)
  return data
}

export function useRiskAnalyze() {
  return useMutation<RiskMetrics, Error, RiskAnalyzeRequest>({
    mutationFn: analyzeRisk,
  })
}
```

**Step 2: TypeScript compile check**

```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 3: Commit**
```bash
git add frontend/src/api/risk.ts
git commit -m "feat(api): add risk API client and useRiskAnalyze mutation hook"
```

---

## Task H8: Create the RiskWidget component

**Files:**
- Create: `frontend/src/components/widgets/RiskWidget.tsx`

**Step 1: Create the file**

The component is divided into four logical sections:

1. **Input bar** — symbol input(s), weights, lookback selector (3M / 1Y / 2Y / 5Y), Analyze button
2. **Metrics grid** — 6 cards: VaR, CVaR, Sharpe, Sortino, Max Drawdown, Volatility
3. **Return distribution** — recharts BarChart histogram of daily returns
4. **Stress test table** — scenario name, duration, estimated portfolio impact, description

```typescript
import { useState } from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
  Cell,
} from 'recharts'
import { AlertTriangle, TrendingDown, Activity, BarChart2, Loader2, X, Plus } from 'lucide-react'
import { useRiskAnalyze } from '@/api/risk'
import type { WidgetProps } from '@/types/terminal'
import type { RiskMetrics, SymbolRisk, StressResult } from '@/types/terminal'

// ── helpers ───────────────────────────────────────────────────────────────

function pct(v: number, decimals = 2): string {
  return (v * 100).toFixed(decimals) + '%'
}

function colorForValue(v: number, reverse = false): string {
  if (reverse) v = -v
  if (v > 0) return 'text-green-400'
  if (v < 0) return 'text-red-400'
  return 'text-muted-foreground'
}

const LOOKBACK_OPTIONS: { label: string; days: number }[] = [
  { label: '3M',  days: 63 },
  { label: '1Y',  days: 252 },
  { label: '2Y',  days: 504 },
  { label: '5Y',  days: 1260 },
]

// ── Sub-components ─────────────────────────────────────────────────────────

function MetricCard({
  label,
  value,
  subLabel,
  icon: Icon,
  valueClass,
}: {
  label: string
  value: string
  subLabel?: string
  icon: React.ElementType
  valueClass?: string
}) {
  return (
    <div className="bg-zinc-900 border border-zinc-700 rounded p-3 flex flex-col gap-1">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Icon className="w-3 h-3" />
        {label}
      </div>
      <div className={`text-lg font-mono font-semibold ${valueClass ?? ''}`}>{value}</div>
      {subLabel && <div className="text-[10px] text-muted-foreground">{subLabel}</div>}
    </div>
  )
}

function SymbolTable({ rows }: { rows: SymbolRisk[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-zinc-700 text-muted-foreground">
            <th className="text-left pb-1 pr-3">Symbol</th>
            <th className="text-right pb-1 pr-3">Weight</th>
            <th className="text-right pb-1 pr-3">VaR 95%</th>
            <th className="text-right pb-1 pr-3">Sharpe</th>
            <th className="text-right pb-1 pr-3">Sortino</th>
            <th className="text-right pb-1 pr-3">Max DD</th>
            <th className="text-right pb-1">Vol (ann.)</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.symbol} className="border-b border-zinc-800 hover:bg-zinc-800/40">
              <td className="py-1 pr-3 font-mono">{r.symbol}</td>
              <td className="py-1 pr-3 text-right">{pct(r.weight)}</td>
              <td className={`py-1 pr-3 text-right font-mono ${colorForValue(r.var_95)}`}>
                {pct(r.var_95)}
              </td>
              <td className={`py-1 pr-3 text-right font-mono ${colorForValue(r.sharpe)}`}>
                {r.sharpe.toFixed(2)}
              </td>
              <td className={`py-1 pr-3 text-right font-mono ${colorForValue(r.sortino)}`}>
                {r.sortino.toFixed(2)}
              </td>
              <td className={`py-1 pr-3 text-right font-mono ${colorForValue(r.max_drawdown)}`}>
                {pct(r.max_drawdown)}
              </td>
              <td className="py-1 text-right font-mono">{pct(r.volatility)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function StressTable({ rows }: { rows: StressResult[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-zinc-700 text-muted-foreground">
            <th className="text-left pb-1 pr-3">Scenario</th>
            <th className="text-right pb-1 pr-3">Duration</th>
            <th className="text-right pb-1 pr-3">Hist. Return</th>
            <th className="text-right pb-1 pr-3">Portfolio Impact</th>
            <th className="text-left pb-1">Notes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-b border-zinc-800 hover:bg-zinc-800/40">
              <td className="py-1 pr-3 font-semibold">{r.name}</td>
              <td className="py-1 pr-3 text-right text-muted-foreground">{r.duration_days}d</td>
              <td className={`py-1 pr-3 text-right font-mono ${colorForValue(r.scenario_return)}`}>
                {pct(r.scenario_return)}
              </td>
              <td className={`py-1 pr-3 text-right font-mono font-semibold ${colorForValue(r.portfolio_impact)}`}>
                {pct(r.portfolio_impact)}
              </td>
              <td className="py-1 text-muted-foreground">{r.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── Main widget ────────────────────────────────────────────────────────────

export function RiskWidget({ ticker }: WidgetProps) {
  // Seed the symbol list from the ticker prop if provided
  const [symbols, setSymbols] = useState<string[]>(ticker ? [ticker.toUpperCase()] : [])
  const [weights, setWeights] = useState<string[]>(ticker ? ['1'] : [])
  const [newSymbol, setNewSymbol] = useState('')
  const [lookbackDays, setLookbackDays] = useState(252)

  const { mutate: analyze, data: result, isPending, error, reset } = useRiskAnalyze()

  // ── Symbol management ──────────────────────────────────────────────────

  function addSymbol() {
    const sym = newSymbol.trim().toUpperCase()
    if (!sym || symbols.includes(sym)) return
    setSymbols((s) => [...s, sym])
    setWeights((w) => [...w, ''])
    setNewSymbol('')
  }

  function removeSymbol(idx: number) {
    setSymbols((s) => s.filter((_, i) => i !== idx))
    setWeights((w) => w.filter((_, i) => i !== idx))
  }

  function updateWeight(idx: number, val: string) {
    setWeights((w) => w.map((v, i) => (i === idx ? val : v)))
  }

  // ── Compute normalized weights ─────────────────────────────────────────

  function resolvedWeights(): number[] | undefined {
    const parsed = weights.map((w) => parseFloat(w))
    if (parsed.some(isNaN)) return undefined  // trigger equal-weight fallback on backend
    const sum = parsed.reduce((a, b) => a + b, 0)
    if (sum === 0) return undefined
    return parsed.map((w) => w / sum)
  }

  // ── Submit ─────────────────────────────────────────────────────────────

  function handleAnalyze() {
    if (symbols.length === 0) return
    reset()
    analyze({
      symbols,
      weights: resolvedWeights(),
      lookback_days: lookbackDays,
      confidence_level: 0.95,
    })
  }

  // ── Render ─────────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col gap-4 p-3 h-full overflow-y-auto text-sm">

      {/* ── Input bar ─────────────────────────────────────────────────── */}
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap gap-2 items-end">
          {/* Symbol chips */}
          {symbols.map((sym, idx) => (
            <div key={sym} className="flex items-center gap-1 bg-zinc-800 border border-zinc-600 rounded px-2 py-1">
              <span className="font-mono text-xs font-semibold">{sym}</span>
              <input
                type="number"
                placeholder="wt"
                value={weights[idx]}
                onChange={(e) => updateWeight(idx, e.target.value)}
                className="w-10 bg-transparent text-xs text-right focus:outline-none text-muted-foreground"
                min="0"
                step="0.1"
              />
              <button
                onClick={() => removeSymbol(idx)}
                className="text-muted-foreground hover:text-red-400 ml-0.5"
                aria-label={`Remove ${sym}`}
              >
                <X className="w-3 h-3" />
              </button>
            </div>
          ))}

          {/* Add symbol */}
          <div className="flex items-center gap-1">
            <input
              type="text"
              placeholder="Add symbol"
              value={newSymbol}
              onChange={(e) => setNewSymbol(e.target.value.toUpperCase())}
              onKeyDown={(e) => { if (e.key === 'Enter') addSymbol() }}
              className="bg-zinc-800 border border-zinc-600 rounded px-2 py-1 text-xs font-mono w-24 focus:outline-none focus:border-zinc-400"
            />
            <button
              onClick={addSymbol}
              className="p-1 bg-zinc-700 hover:bg-zinc-600 rounded"
              aria-label="Add symbol"
            >
              <Plus className="w-3 h-3" />
            </button>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Lookback selector */}
          <div className="flex gap-1">
            {LOOKBACK_OPTIONS.map((opt) => (
              <button
                key={opt.label}
                onClick={() => setLookbackDays(opt.days)}
                className={`px-2 py-0.5 text-xs rounded border ${
                  lookbackDays === opt.days
                    ? 'bg-zinc-600 border-zinc-400 text-white'
                    : 'bg-zinc-800 border-zinc-700 text-muted-foreground hover:border-zinc-500'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>

          <button
            onClick={handleAnalyze}
            disabled={isPending || symbols.length === 0}
            className="px-3 py-1 text-xs bg-orange-600 hover:bg-orange-500 disabled:opacity-50 disabled:cursor-not-allowed rounded font-semibold flex items-center gap-1"
          >
            {isPending && <Loader2 className="w-3 h-3 animate-spin" />}
            {isPending ? 'Calculating…' : 'Analyze'}
          </button>
        </div>
      </div>

      {/* ── Error state ────────────────────────────────────────────────── */}
      {error && (
        <div className="flex items-center gap-2 text-xs text-red-400 bg-red-950/30 border border-red-800 rounded p-2">
          <AlertTriangle className="w-3 h-3 flex-shrink-0" />
          {error.message}
        </div>
      )}

      {/* ── Empty state ────────────────────────────────────────────────── */}
      {!result && !isPending && !error && (
        <div className="flex flex-col items-center justify-center flex-1 text-muted-foreground text-xs gap-1">
          <Activity className="w-8 h-8 mb-1 opacity-30" />
          Add symbols and click Analyze to compute risk metrics
        </div>
      )}

      {/* ── Results ────────────────────────────────────────────────────── */}
      {result && <RiskResults metrics={result} />}
    </div>
  )
}

// ── Results section (extracted to keep main component readable) ────────────

function RiskResults({ metrics }: { metrics: RiskMetrics }) {
  return (
    <div className="flex flex-col gap-5">

      {/* Portfolio metrics grid */}
      <section>
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          Portfolio Metrics ({metrics.lookback_days}d lookback)
        </h3>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
          <MetricCard
            label="VaR 95% (1-day)"
            value={pct(metrics.var_95)}
            subLabel="Hist. simulation"
            icon={TrendingDown}
            valueClass={colorForValue(metrics.var_95)}
          />
          <MetricCard
            label="CVaR 95% (ES)"
            value={pct(metrics.cvar_95)}
            subLabel="Expected shortfall"
            icon={AlertTriangle}
            valueClass={colorForValue(metrics.cvar_95)}
          />
          <MetricCard
            label="Sharpe Ratio"
            value={metrics.sharpe_ratio.toFixed(2)}
            subLabel="Ann., rf = 5%"
            icon={Activity}
            valueClass={colorForValue(metrics.sharpe_ratio)}
          />
          <MetricCard
            label="Sortino Ratio"
            value={metrics.sortino_ratio.toFixed(2)}
            subLabel="Downside deviation"
            icon={Activity}
            valueClass={colorForValue(metrics.sortino_ratio)}
          />
          <MetricCard
            label="Max Drawdown"
            value={pct(metrics.max_drawdown)}
            subLabel="Peak-to-trough"
            icon={TrendingDown}
            valueClass={colorForValue(metrics.max_drawdown)}
          />
          <MetricCard
            label="Volatility (Ann.)"
            value={pct(metrics.volatility)}
            subLabel="Std dev × √252"
            icon={BarChart2}
          />
        </div>
      </section>

      {/* Return distribution histogram */}
      {metrics.return_buckets.length > 0 && (
        <section>
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
            Daily Return Distribution
          </h3>
          <div className="h-40">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={metrics.return_buckets} margin={{ top: 4, right: 4, bottom: 4, left: 0 }}>
                <XAxis
                  dataKey="mid_point"
                  tickFormatter={(v) => `${(v * 100).toFixed(1)}%`}
                  tick={{ fontSize: 9, fill: '#71717a' }}
                  interval="preserveStartEnd"
                />
                <YAxis tick={{ fontSize: 9, fill: '#71717a' }} width={28} />
                <Tooltip
                  formatter={(v: number) => [v, 'Days']}
                  labelFormatter={(l: number) => `Return: ${(l * 100).toFixed(2)}%`}
                  contentStyle={{ background: '#18181b', border: '1px solid #3f3f46', fontSize: 11 }}
                />
                <ReferenceLine x={0} stroke="#52525b" strokeDasharray="3 3" />
                <Bar dataKey="count" radius={[2, 2, 0, 0]}>
                  {metrics.return_buckets.map((b, i) => (
                    <Cell key={i} fill={b.mid_point < 0 ? '#ef4444' : '#22c55e'} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </section>
      )}

      {/* Per-symbol breakdown */}
      {metrics.symbol_metrics.length > 1 && (
        <section>
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
            Per-Symbol Breakdown
          </h3>
          <SymbolTable rows={metrics.symbol_metrics} />
        </section>
      )}

      {/* Stress tests */}
      <section>
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          Stress Test Scenarios
        </h3>
        <StressTable rows={metrics.stress_scenarios} />
      </section>
    </div>
  )
}
```

**Step 2: TypeScript compile check**

```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 3: Verify recharts is already installed (no new dependency needed)**

```bash
docker compose exec frontend npm ls recharts
```
Expected: `recharts@2.x.x` — already present from Phase A or earlier.

If recharts is missing:
```bash
docker compose exec frontend npm install recharts
```

**Step 4: Commit**
```bash
git add frontend/src/components/widgets/RiskWidget.tsx
git commit -m "feat(widget): add RiskWidget — VaR/Sharpe metrics, histogram, stress tests, symbol breakdown"
```

---

## Task H9: Wire RiskWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import** directly after the `AIChatWidget` import line:

```typescript
import { RiskWidget }    from '@/components/widgets/RiskWidget'
```

**Step 2: Replace the RISK stub** in `WIDGET_REGISTRY`:

Find:
```typescript
  RISK: () => <ComingSoon label="Risk Analytics (Phase H)" />,
```

Replace with:
```typescript
  RISK: RiskWidget,
```

**Step 3: TypeScript compile check**

```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 4: Manual smoke test in browser**

1. Navigate to `http://localhost:5173/terminal`
2. In the command bar, type `BTCUSDT RISK` and press Enter
3. A new panel opens with the RISK widget
4. Type `BTCUSDT` in the symbol input and click Analyze
5. If daily candles exist in DB: full metrics display
6. If no candles: error message "not enough daily candles for BTCUSDT" is shown correctly

**Step 5: Commit**
```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(terminal): wire RiskWidget into WidgetRegistry — replaces Phase H stub"
```

---

## Task H10: Update the bloomberg-reference.md phase status

**Files:**
- Modify: `docs/plans/bloomberg-reference.md`

**Step 1: Update Phase H row** in the Phase Status table.

Find:
```
| **H** | `bloomberg-phase-H-plan.md` | ⬜ Pending | VaR/Sharpe Go engine, RISK widget |
```

Replace with:
```
| **H** | `bloomberg-phase-H-plan.md` | ✅ Done | VaR/Sharpe Go engine, RISK widget |
```

**Step 2: Commit**
```bash
git add docs/plans/bloomberg-reference.md
git commit -m "docs: mark Bloomberg Phase H (Risk Analytics) as done"
```

---

## Phase H Completion Checklist

### Backend
- [ ] `backend/internal/analytics/risk.go` created — all math in pure Go standard library
  - [ ] `dailyReturns` — `(p[t]-p[t-1])/p[t-1]`
  - [ ] `computeVaR` — historical simulation at configurable confidence level
  - [ ] `computeCVaR` — mean of returns at or below VaR threshold
  - [ ] `computeSharpe` — annualized, rf = 5%
  - [ ] `computeSortino` — downside deviation only
  - [ ] `computeMaxDrawdown` — peak-to-trough on price/equity series
  - [ ] `computeVolatility` — `std_dev * sqrt(252)`
  - [ ] `portfolioReturnSeries` — weighted blend of per-symbol return slices
  - [ ] `buildHistogram` — 20-bucket return distribution for histogram
  - [ ] 3 hardcoded stress scenarios: 2020 COVID, 2008 GFC, 2022 Bear Market
- [ ] `backend/internal/analytics/risk_test.go` created — all tests pass
- [ ] `backend/internal/api/risk_handler.go` created
  - [ ] Parses and validates `RiskAnalyzeRequest`
  - [ ] Defaults: `lookback_days=252`, `confidence_level=0.95`, equal weights when not provided
  - [ ] Portfolio ID path: resolves symbols from positions, correct ownership check
  - [ ] Max 20 symbols guard
  - [ ] Weight sum validation (0.99–1.01)
  - [ ] `analytics.Analyze` called with resolved request
  - [ ] Correct Fiber error format: `c.Status(xxx).JSON(fiber.Map{"error": "..."})`
- [ ] `backend/internal/api/routes.go` updated — `POST /api/v1/risk/analyze` registered under `protected` group with `mutationLimiter`
- [ ] `docker compose exec backend go build ./...` passes
- [ ] `docker compose exec backend go test ./internal/analytics/...` — all tests pass (no regressions)

### Frontend
- [ ] `frontend/src/types/terminal.ts` updated — `RiskAnalyzeRequest`, `RiskMetrics`, `SymbolRisk`, `StressResult`, `ReturnBucket` added
- [ ] `frontend/src/api/risk.ts` created — `analyzeRisk` function + `useRiskAnalyze` mutation hook
- [ ] `frontend/src/components/widgets/RiskWidget.tsx` created
  - [ ] Symbol chip input with individual weight fields
  - [ ] Add/remove symbols
  - [ ] Lookback selector: 3M / 1Y / 2Y / 5Y (maps to 63 / 252 / 504 / 1260 days)
  - [ ] Analyze button triggers `useRiskAnalyze` mutation
  - [ ] Loading spinner during calculation
  - [ ] Error banner if request fails
  - [ ] Empty state with instructional message
  - [ ] 6 metric cards: VaR, CVaR, Sharpe, Sortino, Max Drawdown, Volatility
  - [ ] Return distribution recharts `BarChart` — negative bars red, positive bars green
  - [ ] Per-symbol breakdown table (only shown when >1 symbol)
  - [ ] Stress test table with scenario name, duration, historical return, portfolio impact, description
  - [ ] All colors follow Tailwind dark theme (zinc palette)
  - [ ] No inline styles; no CSS modules
- [ ] `frontend/src/components/terminal/WidgetRegistry.tsx` updated — `RISK` entry replaced from stub to real `RiskWidget`
- [ ] `docker compose exec frontend npx tsc --noEmit` passes
- [ ] Browser smoke test: `BTCUSDT RISK` opens widget, Analyze button executes, results render or graceful error shown

### Documentation
- [ ] `docs/plans/bloomberg-reference.md` Phase H status updated to `✅ Done`
