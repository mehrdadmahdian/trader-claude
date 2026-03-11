# Bloomberg Terminal — Phase C: Screener

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a fully functional `SCR` (Screener) widget to the Bloomberg Terminal. Users filter assets across crypto, equities, and commodities by metrics (price, % change, volume, market cap, P/E ratio, etc.) and see ranked, sortable results. Preset filters can be saved per-user and reloaded.

**Architecture:** A new Go package `backend/internal/screener/` encapsulates all market data fetching and filter logic. A new handler `backend/internal/api/screener_handler.go` exposes four REST endpoints. A new `ScreenerPreset` GORM model persists saved presets per user. On the frontend, `ScreenerWidget.tsx` provides the full filter-builder UI, a results table, and preset management. The `SCR` stub in `WidgetRegistry.tsx` is replaced with the real component.

**Two apps, one codebase (unchanged from Phase A):**
- `/` → Old app (Layout.tsx) — trading workbench
- `/terminal` → Bloomberg workspace (WorkspaceLayout)

**Tech Stack:** Go/Fiber (screener handler + engine), GORM (ScreenerPreset model), Redis (market data cache, 60 s TTL), CoinGecko API (crypto), Yahoo Finance v7 (equities + commodities), React Query v5 (data fetching), Tailwind + lucide-react (UI), Zustand (no new store needed — screener state is local to the widget).

---

## Task C1: Add ScreenerPreset model to models.go

**Files:**
- Modify: `backend/internal/models/models.go`

**Step 1: Append the ScreenerPreset struct** at the bottom of `models.go`, after the `Workspace` struct (or at the end of the file before EOF):

```go
// --- ScreenerPreset ---

// ScreenerPreset stores a saved set of screener filters for a user.
type ScreenerPreset struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"not null;index" json:"user_id"`
	Name       string    `gorm:"type:varchar(100);not null" json:"name"`
	AssetClass string    `gorm:"type:varchar(20);not null" json:"asset_class"` // crypto|equities|commodities
	Filters    JSON      `gorm:"type:json" json:"filters"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ScreenerPreset) TableName() string { return "screener_presets" }
```

Note: `time` is already imported in `models.go`. `JSON` is the existing `models.JSON` type. No new imports are needed.

**Step 2: Rebuild and verify GORM auto-migrates the new table**

```bash
docker compose up -d --build backend
make health
docker compose logs backend 2>&1 | grep -i "screener\|automigrate\|error" | head -30
```

Expected: no errors, backend healthy.

**Step 3: Confirm table exists in MySQL**

```bash
make db-shell
```

Inside MySQL shell:
```sql
SHOW TABLES LIKE 'screener_presets';
DESCRIBE screener_presets;
```

Expected columns: `id`, `user_id`, `name`, `asset_class`, `filters`, `created_at`.

**Step 4: Commit**

```bash
git add backend/internal/models/models.go
git commit -m "feat(models): add ScreenerPreset model for Bloomberg SCR widget"
```

---

## Task C2: Create the screener engine package

**Files:**
- Create: `backend/internal/screener/screener.go`

This package owns all market data fetching and filter evaluation logic. It has no dependency on Fiber or GORM — it is a pure business logic package. The handler in Task C3 wires it together with Redis and the HTTP layer.

**Step 1: Create the directory and file**

```bash
mkdir -p backend/internal/screener
```

Create `backend/internal/screener/screener.go`:

```go
package screener

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	coinGeckoMarketsURL = "https://api.coingecko.com/api/v3/coins/markets"
	yahooQuoteURL       = "https://query1.finance.yahoo.com/v7/finance/quote"
	httpTimeout         = 15 * time.Second
	userAgent           = "Mozilla/5.0 (compatible; trader-claude/1.0)"
)

// ── Request / response types ─────────────────────────────────────────────────

// Filter represents one screener condition.
type Filter struct {
	Field    string  `json:"field"`    // price, change_24h, volume, market_cap, pe_ratio
	Operator string  `json:"operator"` // gt, lt, gte, lte, eq
	Value    float64 `json:"value"`
}

// RunRequest is the POST body for /api/v1/screener/run.
type RunRequest struct {
	AssetClass string   `json:"asset_class"` // crypto|equities|commodities
	Filters    []Filter `json:"filters"`
	Limit      int      `json:"limit"` // default 50, max 250
}

// Result is one row returned by the screener.
type Result struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Change24h float64 `json:"change_24h"` // percent
	Volume    float64 `json:"volume"`
	MarketCap float64 `json:"market_cap,omitempty"`
	PERatio   float64 `json:"pe_ratio,omitempty"`
}

// ── CoinGecko response shapes ─────────────────────────────────────────────────

type cgCoin struct {
	ID                           string  `json:"id"`
	Symbol                       string  `json:"symbol"`
	Name                         string  `json:"name"`
	CurrentPrice                 float64 `json:"current_price"`
	MarketCap                    float64 `json:"market_cap"`
	TotalVolume                  float64 `json:"total_volume"`
	PriceChangePercentage24h     float64 `json:"price_change_percentage_24h"`
}

// ── Yahoo Finance v7 quote shapes ─────────────────────────────────────────────

type yahooQuoteResp struct {
	QuoteResponse struct {
		Result []struct {
			Symbol                     string  `json:"symbol"`
			ShortName                  string  `json:"shortName"`
			RegularMarketPrice         float64 `json:"regularMarketPrice"`
			RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
			RegularMarketVolume        float64 `json:"regularMarketVolume"`
			MarketCap                  float64 `json:"marketCap"`
			TrailingPE                 float64 `json:"trailingPE"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteResponse"`
}

// ── Equity / commodity universe lists ────────────────────────────────────────

// top S&P 500 by market cap — used as the equity screener universe
var equitySymbols = []string{
	"AAPL", "MSFT", "NVDA", "AMZN", "META", "GOOGL", "GOOG", "BRK-B",
	"LLY", "JPM", "AVGO", "TSLA", "UNH", "V", "XOM", "MA", "JNJ",
	"PG", "COST", "HD", "MRK", "ABBV", "CVX", "CRM", "BAC", "NFLX",
	"AMD", "KO", "PEP", "TMO", "WMT", "ORCL", "ACN", "MCD", "LIN",
	"CSCO", "ABT", "TXN", "DHR", "ADBE", "AMGN", "NKE", "PM", "GE",
	"RTX", "CAT", "ISRG", "UNP", "NOW", "SPGI", "LOW", "INTU", "BKNG",
}

// commodity ETF proxies — liquid instruments representative of each commodity
var commoditySymbols = []string{
	"GLD",  // Gold
	"SLV",  // Silver
	"USO",  // Crude Oil
	"UNG",  // Natural Gas
	"CORN", // Corn
	"WEAT", // Wheat
	"SOYB", // Soybeans
	"PDBC", // Broad Commodities
	"DBA",  // Agriculture
	"DBB",  // Base Metals
	"CPER", // Copper
	"PALL", // Palladium
	"PPLT", // Platinum
}

// ── Engine ────────────────────────────────────────────────────────────────────

// Engine fetches market data and applies screener filters.
// It is stateless — Redis caching is handled by the caller (handler).
type Engine struct {
	client *http.Client
}

// New returns a new Engine.
func New() *Engine {
	return &Engine{
		client: &http.Client{Timeout: httpTimeout},
	}
}

// Run fetches the relevant universe and applies filters. It returns up to
// req.Limit rows sorted by market cap descending (or volume for crypto).
func (e *Engine) Run(ctx context.Context, req RunRequest) ([]Result, error) {
	if req.Limit <= 0 || req.Limit > 250 {
		req.Limit = 50
	}

	var raw []Result
	var err error

	switch req.AssetClass {
	case "crypto":
		raw, err = e.fetchCrypto(ctx, 250)
	case "equities":
		raw, err = e.fetchYahoo(ctx, equitySymbols)
	case "commodities":
		raw, err = e.fetchYahoo(ctx, commoditySymbols)
	default:
		return nil, fmt.Errorf("unknown asset_class %q: must be crypto|equities|commodities", req.AssetClass)
	}
	if err != nil {
		return nil, err
	}

	// Apply filters
	filtered := make([]Result, 0, len(raw))
	for _, r := range raw {
		if matchesAll(r, req.Filters) {
			filtered = append(filtered, r)
		}
	}

	// Sort by market cap desc; fall back to volume desc when market cap is zero
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].MarketCap != filtered[j].MarketCap {
			return filtered[i].MarketCap > filtered[j].MarketCap
		}
		return filtered[i].Volume > filtered[j].Volume
	})

	if len(filtered) > req.Limit {
		filtered = filtered[:req.Limit]
	}
	return filtered, nil
}

// ── Private fetch helpers ─────────────────────────────────────────────────────

func (e *Engine) fetchCrypto(ctx context.Context, perPage int) ([]Result, error) {
	url := fmt.Sprintf(
		"%s?vs_currency=usd&order=market_cap_desc&per_page=%d&page=1&sparkline=false",
		coinGeckoMarketsURL, perPage,
	)
	body, err := e.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("coingecko fetch: %w", err)
	}

	var coins []cgCoin
	if err := json.Unmarshal(body, &coins); err != nil {
		return nil, fmt.Errorf("coingecko parse: %w", err)
	}

	results := make([]Result, 0, len(coins))
	for _, c := range coins {
		results = append(results, Result{
			Symbol:    strings.ToUpper(c.Symbol),
			Name:      c.Name,
			Price:     c.CurrentPrice,
			Change24h: c.PriceChangePercentage24h,
			Volume:    c.TotalVolume,
			MarketCap: c.MarketCap,
		})
	}
	return results, nil
}

func (e *Engine) fetchYahoo(ctx context.Context, symbols []string) ([]Result, error) {
	// Yahoo v7 accepts up to ~500 symbols per request; batch in one call.
	joined := strings.Join(symbols, ",")
	url := fmt.Sprintf(
		"%s?symbols=%s&fields=regularMarketPrice,regularMarketChangePercent,regularMarketVolume,marketCap,trailingPE,shortName",
		yahooQuoteURL, joined,
	)
	body, err := e.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("yahoo fetch: %w", err)
	}

	var resp yahooQuoteResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("yahoo parse: %w", err)
	}
	if resp.QuoteResponse.Error != nil {
		return nil, fmt.Errorf("yahoo error: %s — %s",
			resp.QuoteResponse.Error.Code,
			resp.QuoteResponse.Error.Description,
		)
	}

	results := make([]Result, 0, len(resp.QuoteResponse.Result))
	for _, q := range resp.QuoteResponse.Result {
		results = append(results, Result{
			Symbol:    q.Symbol,
			Name:      q.ShortName,
			Price:     q.RegularMarketPrice,
			Change24h: q.RegularMarketChangePercent,
			Volume:    q.RegularMarketVolume,
			MarketCap: q.MarketCap,
			PERatio:   q.TrailingPE,
		})
	}
	return results, nil
}

func (e *Engine) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ── Filter evaluation ─────────────────────────────────────────────────────────

func matchesAll(r Result, filters []Filter) bool {
	for _, f := range filters {
		if !matches(r, f) {
			return false
		}
	}
	return true
}

func matches(r Result, f Filter) bool {
	var actual float64
	switch f.Field {
	case "price":
		actual = r.Price
	case "change_24h":
		actual = r.Change24h
	case "volume":
		actual = r.Volume
	case "market_cap":
		actual = r.MarketCap
	case "pe_ratio":
		actual = r.PERatio
	default:
		// Unknown field — skip (treat as passing)
		return true
	}

	switch f.Operator {
	case "gt":
		return actual > f.Value
	case "gte":
		return actual >= f.Value
	case "lt":
		return actual < f.Value
	case "lte":
		return actual <= f.Value
	case "eq":
		return actual == f.Value
	default:
		return true
	}
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./internal/screener/...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add backend/internal/screener/screener.go
git commit -m "feat(screener): add screener engine — CoinGecko + Yahoo Finance fetch + filter logic"
```

---

## Task C3: Write unit tests for the screener engine

**Files:**
- Create: `backend/internal/screener/screener_test.go`

**Step 1: Create the test file**

```go
package screener

import (
	"testing"
)

func TestMatchesAllNoFilters(t *testing.T) {
	r := Result{Symbol: "BTC", Price: 50000, Change24h: 2.5, Volume: 1e9, MarketCap: 1e12}
	if !matchesAll(r, nil) {
		t.Error("expected no filters to pass all rows")
	}
}

func TestMatchesSingleFilter(t *testing.T) {
	r := Result{Symbol: "BTC", Price: 50000, Change24h: 2.5, Volume: 1e9, MarketCap: 1e12}

	cases := []struct {
		filter Filter
		want   bool
	}{
		{Filter{Field: "price", Operator: "gt", Value: 40000}, true},
		{Filter{Field: "price", Operator: "lt", Value: 40000}, false},
		{Filter{Field: "price", Operator: "gte", Value: 50000}, true},
		{Filter{Field: "price", Operator: "lte", Value: 50000}, true},
		{Filter{Field: "price", Operator: "eq", Value: 50000}, true},
		{Filter{Field: "change_24h", Operator: "gt", Value: 0}, true},
		{Filter{Field: "change_24h", Operator: "lt", Value: 0}, false},
		{Filter{Field: "volume", Operator: "gte", Value: 1e9}, true},
		{Filter{Field: "market_cap", Operator: "gt", Value: 1e11}, true},
		{Filter{Field: "pe_ratio", Operator: "gt", Value: 0}, false}, // pe_ratio = 0, not > 0
	}

	for _, tc := range cases {
		got := matches(r, tc.filter)
		if got != tc.want {
			t.Errorf("matches(%+v, %+v): got %v, want %v", r, tc.filter, got, tc.want)
		}
	}
}

func TestMatchesAllMultipleFilters(t *testing.T) {
	r := Result{Symbol: "AAPL", Price: 200, Change24h: -1.2, Volume: 5e7, MarketCap: 3e12, PERatio: 30}
	filters := []Filter{
		{Field: "price", Operator: "gt", Value: 100},
		{Field: "change_24h", Operator: "lt", Value: 0},
		{Field: "pe_ratio", Operator: "lte", Value: 35},
	}
	if !matchesAll(r, filters) {
		t.Error("expected all three filters to pass")
	}

	// Fail on one filter
	filters = append(filters, Filter{Field: "market_cap", Operator: "gt", Value: 1e13})
	if matchesAll(r, filters) {
		t.Error("expected combined filters to fail on market_cap")
	}
}

func TestUnknownFieldPassesThrough(t *testing.T) {
	r := Result{Symbol: "X"}
	if !matches(r, Filter{Field: "unknown_field", Operator: "gt", Value: 999}) {
		t.Error("unknown field should pass through as true")
	}
}

func TestUnknownOperatorPassesThrough(t *testing.T) {
	r := Result{Price: 100}
	if !matches(r, Filter{Field: "price", Operator: "between", Value: 0}) {
		t.Error("unknown operator should pass through as true")
	}
}
```

**Step 2: Run tests**

```bash
docker compose exec backend go test ./internal/screener/... -v
```

Expected output:
```
--- PASS: TestMatchesAllNoFilters
--- PASS: TestMatchesSingleFilter
--- PASS: TestMatchesAllMultipleFilters
--- PASS: TestUnknownFieldPassesThrough
--- PASS: TestUnknownOperatorPassesThrough
PASS
```

**Step 3: Commit**

```bash
git add backend/internal/screener/screener_test.go
git commit -m "test(screener): add unit tests for filter evaluation logic"
```

---

## Task C4: Create the screener HTTP handler

**Files:**
- Create: `backend/internal/api/screener_handler.go`

**Step 1: Create the handler file**

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/screener"
)

const (
	screenerCacheTTL = 60 * time.Second
)

type screenerHandler struct {
	db     *gorm.DB
	rdb    *redis.Client
	engine *screener.Engine
}

func newScreenerHandler(db *gorm.DB, rdb *redis.Client) *screenerHandler {
	return &screenerHandler{
		db:     db,
		rdb:    rdb,
		engine: screener.New(),
	}
}

// POST /api/v1/screener/run
// Runs a screen and returns matched assets. Results are cached in Redis for 60 s
// per (asset_class, filters hash) to avoid hammering upstream APIs.
func (h *screenerHandler) run(c *fiber.Ctx) error {
	var req screener.RunRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.AssetClass == "" {
		return c.Status(400).JSON(fiber.Map{"error": "asset_class is required"})
	}
	switch req.AssetClass {
	case "crypto", "equities", "commodities":
		// valid
	default:
		return c.Status(400).JSON(fiber.Map{"error": "asset_class must be crypto|equities|commodities"})
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 250 {
		req.Limit = 250
	}

	// Build a stable cache key from asset class + serialised filters.
	cacheKey, err := buildCacheKey(req)
	if err != nil {
		// Non-fatal — just skip cache
		cacheKey = ""
	}

	ctx := c.Context()

	// Try Redis cache first
	if cacheKey != "" {
		if cached, err := h.rdb.Get(ctx, cacheKey).Bytes(); err == nil {
			var results []screener.Result
			if jsonErr := json.Unmarshal(cached, &results); jsonErr == nil {
				return c.JSON(fiber.Map{"results": results, "cached": true})
			}
		}
	}

	// Execute screen
	results, err := h.engine.Run(context.Background(), req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("screener fetch failed: %s", err.Error())})
	}

	// Populate cache
	if cacheKey != "" {
		if b, err := json.Marshal(results); err == nil {
			h.rdb.Set(ctx, cacheKey, b, screenerCacheTTL)
		}
	}

	return c.JSON(fiber.Map{"results": results, "cached": false})
}

// GET /api/v1/screener/presets
// Lists all presets owned by the authenticated user.
func (h *screenerHandler) listPresets(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	var presets []models.ScreenerPreset
	if err := h.db.Where("user_id = ?", userID).Order("created_at asc").Find(&presets).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch presets"})
	}
	return c.JSON(presets)
}

// POST /api/v1/screener/presets
// Saves a new preset for the authenticated user.
func (h *screenerHandler) createPreset(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	var body struct {
		Name       string      `json:"name"`
		AssetClass string      `json:"asset_class"`
		Filters    models.JSON `json:"filters"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	switch body.AssetClass {
	case "crypto", "equities", "commodities":
		// valid
	default:
		return c.Status(400).JSON(fiber.Map{"error": "asset_class must be crypto|equities|commodities"})
	}

	preset := models.ScreenerPreset{
		UserID:     userID,
		Name:       body.Name,
		AssetClass: body.AssetClass,
		Filters:    body.Filters,
	}
	if err := h.db.Create(&preset).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to save preset"})
	}
	return c.Status(201).JSON(preset)
}

// DELETE /api/v1/screener/presets/:id
// Deletes a preset. Returns 404 if it does not belong to the authenticated user.
func (h *screenerHandler) deletePreset(c *fiber.Ctx) error {
	userID := auth.GetUserID(c)
	id := c.Params("id")

	var preset models.ScreenerPreset
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&preset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "preset not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch preset"})
	}

	if err := h.db.Delete(&preset).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to delete preset"})
	}
	return c.SendStatus(204)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// buildCacheKey creates a deterministic Redis key for a RunRequest.
// The filters slice is serialised to JSON (order matters — consistent from the
// frontend because the filter array is always appended in UI order).
func buildCacheKey(req screener.RunRequest) (string, error) {
	fb, err := json.Marshal(req.Filters)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("screener:%s:%s:%d", req.AssetClass, string(fb), req.Limit), nil
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add backend/internal/api/screener_handler.go
git commit -m "feat(api): add screener handler — run screen, list/create/delete presets, Redis cache"
```

---

## Task C5: Register screener routes in routes.go

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Add the screener handler init** in `RegisterRoutes`. Find the line that reads:

```go
// --- Workspaces (Bloomberg terminal) ---
workspaceH := newWorkspaceHandler(db)
```

Insert immediately before it (or right after the workspace block — order within `protected` does not matter):

```go
// --- Screener (Bloomberg terminal Phase C) ---
scrH := newScreenerHandler(db, rdb)
protected.Post("/screener/run", scrH.run)
protected.Get("/screener/presets", scrH.listPresets)
protected.Post("/screener/presets", scrH.createPreset)
protected.Delete("/screener/presets/:id", scrH.deletePreset)
```

**Step 2: Build and smoke-test**

```bash
docker compose up -d --build backend
make health
```

Run a quick screener test (replace `<token>` with a valid JWT from `POST /api/v1/auth/login`):

```bash
curl -s -X POST http://localhost:8080/api/v1/screener/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"asset_class":"crypto","filters":[],"limit":5}' | jq .
```

Expected: JSON with `results` array containing 5 crypto rows.

**Step 3: Test preset CRUD**

```bash
# Create a preset
curl -s -X POST http://localhost:8080/api/v1/screener/presets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"BTC Bull","asset_class":"crypto","filters":{"items":[{"field":"change_24h","operator":"gt","value":2}]}}' | jq .

# List presets
curl -s http://localhost:8080/api/v1/screener/presets \
  -H "Authorization: Bearer <token>" | jq .

# Delete preset (use id from create response)
curl -s -X DELETE http://localhost:8080/api/v1/screener/presets/1 \
  -H "Authorization: Bearer <token>" -w "%{http_code}\n"
```

Expected: 201 on create, array with one item on list, 204 on delete.

**Step 4: Commit**

```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register screener routes under /api/v1/screener"
```

---

## Task C6: Add screener TypeScript types to terminal.ts

**Files:**
- Modify: `frontend/src/types/terminal.ts`

**Step 1: Append screener types** at the bottom of `frontend/src/types/terminal.ts`:

```typescript
// ── Screener types (Phase C) ──────────────────────────────────────────────────

export type AssetClass = 'crypto' | 'equities' | 'commodities'

export type FilterField = 'price' | 'change_24h' | 'volume' | 'market_cap' | 'pe_ratio'
export type FilterOperator = 'gt' | 'lt' | 'gte' | 'lte' | 'eq'

export interface ScreenerFilter {
  field: FilterField
  operator: FilterOperator
  value: number
}

export interface ScreenerRunRequest {
  asset_class: AssetClass
  filters: ScreenerFilter[]
  limit: number
}

export interface ScreenerResult {
  symbol: string
  name: string
  price: number
  change_24h: number
  volume: number
  market_cap: number
  pe_ratio: number
}

export interface ScreenerRunResponse {
  results: ScreenerResult[]
  cached: boolean
}

export interface ScreenerPreset {
  id: number
  user_id: number
  name: string
  asset_class: AssetClass
  filters: { items: ScreenerFilter[] }
  created_at: string
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
git commit -m "feat(types): add screener types — ScreenerFilter, ScreenerResult, ScreenerPreset"
```

---

## Task C7: Create frontend API module for screener

**Files:**
- Create: `frontend/src/api/screener.ts`

**Step 1: Create the file**

```typescript
import apiClient from '@/api/client'
import type {
  ScreenerRunRequest,
  ScreenerRunResponse,
  ScreenerPreset,
} from '@/types/terminal'

export async function runScreener(req: ScreenerRunRequest): Promise<ScreenerRunResponse> {
  const { data } = await apiClient.post<ScreenerRunResponse>('/api/v1/screener/run', req)
  return data
}

export async function fetchScreenerPresets(): Promise<ScreenerPreset[]> {
  const { data } = await apiClient.get<ScreenerPreset[]>('/api/v1/screener/presets')
  return data
}

export async function createScreenerPreset(
  preset: Pick<ScreenerPreset, 'name' | 'asset_class' | 'filters'>,
): Promise<ScreenerPreset> {
  const { data } = await apiClient.post<ScreenerPreset>('/api/v1/screener/presets', preset)
  return data
}

export async function deleteScreenerPreset(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/screener/presets/${id}`)
}
```

**Step 2: TypeScript compile check**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 3: Commit**

```bash
git add frontend/src/api/screener.ts
git commit -m "feat(api): add screener API module — run, presets CRUD"
```

---

## Task C8: Build ScreenerWidget component

**Files:**
- Create: `frontend/src/components/widgets/ScreenerWidget.tsx`

This is the main UI task. The widget has four logical sections:
1. Asset class tabs (Crypto / Equities / Commodities)
2. Preset management (load dropdown + save button)
3. Filter builder (add/remove rows)
4. Results table (sortable columns)

**Step 1: Create the file**

```typescript
import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Play, Save, ChevronUp, ChevronDown, Loader2 } from 'lucide-react'
import type { WidgetProps } from '@/types/terminal'
import type {
  AssetClass,
  FilterField,
  FilterOperator,
  ScreenerFilter,
  ScreenerResult,
} from '@/types/terminal'
import {
  runScreener,
  fetchScreenerPresets,
  createScreenerPreset,
  deleteScreenerPreset,
} from '@/api/screener'

// ── Constants ─────────────────────────────────────────────────────────────────

const ASSET_CLASSES: { value: AssetClass; label: string }[] = [
  { value: 'crypto',      label: 'Crypto'      },
  { value: 'equities',    label: 'Equities'    },
  { value: 'commodities', label: 'Commodities' },
]

const FILTER_FIELDS: { value: FilterField; label: string }[] = [
  { value: 'price',      label: 'Price'        },
  { value: 'change_24h', label: '% Change 24h' },
  { value: 'volume',     label: 'Volume'       },
  { value: 'market_cap', label: 'Market Cap'   },
  { value: 'pe_ratio',   label: 'P/E Ratio'    },
]

const FILTER_OPERATORS: { value: FilterOperator; label: string }[] = [
  { value: 'gt',  label: '>'  },
  { value: 'gte', label: '>=' },
  { value: 'lt',  label: '<'  },
  { value: 'lte', label: '<=' },
  { value: 'eq',  label: '='  },
]

type SortKey = keyof ScreenerResult
type SortDir = 'asc' | 'desc'

// ── Helper formatters ─────────────────────────────────────────────────────────

function fmtPrice(v: number): string {
  if (v >= 1e9) return `$${(v / 1e9).toFixed(2)}B`
  if (v >= 1e6) return `$${(v / 1e6).toFixed(2)}M`
  if (v >= 1e3) return `$${(v / 1e3).toFixed(2)}K`
  return `$${v.toFixed(4)}`
}

function fmtVol(v: number): string {
  if (v >= 1e9) return `${(v / 1e9).toFixed(1)}B`
  if (v >= 1e6) return `${(v / 1e6).toFixed(1)}M`
  if (v >= 1e3) return `${(v / 1e3).toFixed(1)}K`
  return v.toFixed(0)
}

// ── Component ─────────────────────────────────────────────────────────────────

export function ScreenerWidget(_: WidgetProps) {
  const qc = useQueryClient()

  // ── Asset class ──
  const [assetClass, setAssetClass] = useState<AssetClass>('crypto')

  // ── Filter builder ──
  const [filters, setFilters] = useState<ScreenerFilter[]>([])

  const addFilter = () =>
    setFilters(f => [...f, { field: 'change_24h', operator: 'gt', value: 0 }])

  const removeFilter = (idx: number) =>
    setFilters(f => f.filter((_, i) => i !== idx))

  const updateFilter = (idx: number, partial: Partial<ScreenerFilter>) =>
    setFilters(f => f.map((row, i) => (i === idx ? { ...row, ...partial } : row)))

  // ── Run screen ──
  const runMutation = useMutation({
    mutationFn: () =>
      runScreener({ asset_class: assetClass, filters, limit: 100 }),
  })

  // ── Sort state ──
  const [sortKey, setSortKey]   = useState<SortKey>('market_cap')
  const [sortDir, setSortDir]   = useState<SortDir>('desc')

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('desc')
    }
  }

  const sortedResults = useMemo(() => {
    const rows = runMutation.data?.results ?? []
    return [...rows].sort((a, b) => {
      const av = a[sortKey] as number
      const bv = b[sortKey] as number
      return sortDir === 'asc' ? av - bv : bv - av
    })
  }, [runMutation.data, sortKey, sortDir])

  // ── Presets ──
  const { data: presets = [] } = useQuery({
    queryKey: ['screener-presets'],
    queryFn: fetchScreenerPresets,
  })

  const [presetName, setPresetName] = useState('')
  const [showSaveInput, setShowSaveInput] = useState(false)

  const saveMutation = useMutation({
    mutationFn: () =>
      createScreenerPreset({
        name: presetName,
        asset_class: assetClass,
        filters: { items: filters },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['screener-presets'] })
      setPresetName('')
      setShowSaveInput(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteScreenerPreset(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['screener-presets'] }),
  })

  const loadPreset = (id: number) => {
    const preset = presets.find(p => p.id === id)
    if (!preset) return
    setAssetClass(preset.asset_class)
    setFilters(preset.filters?.items ?? [])
  }

  // ── SortIcon helper ──
  const SortIcon = ({ col }: { col: SortKey }) => {
    if (sortKey !== col)
      return <ChevronUp className="h-3 w-3 opacity-30 inline ml-1" />
    return sortDir === 'asc'
      ? <ChevronUp   className="h-3 w-3 inline ml-1 text-primary" />
      : <ChevronDown className="h-3 w-3 inline ml-1 text-primary" />
  }

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col h-full bg-background text-foreground overflow-hidden text-xs">

      {/* ── Header bar ── */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-border flex-wrap">
        {/* Asset class tabs */}
        <div className="flex gap-1">
          {ASSET_CLASSES.map(ac => (
            <button
              key={ac.value}
              onClick={() => setAssetClass(ac.value)}
              className={`px-2 py-0.5 rounded text-xs font-medium transition-colors ${
                assetClass === ac.value
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              }`}
            >
              {ac.label}
            </button>
          ))}
        </div>

        <div className="flex-1" />

        {/* Preset selector */}
        {presets.length > 0 && (
          <select
            className="bg-muted text-foreground text-xs rounded px-1.5 py-0.5 border border-border"
            defaultValue=""
            onChange={e => {
              if (e.target.value) loadPreset(Number(e.target.value))
            }}
          >
            <option value="" disabled>Load preset</option>
            {presets.map(p => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        )}

        {/* Save preset */}
        {showSaveInput ? (
          <div className="flex items-center gap-1">
            <input
              className="bg-muted text-foreground text-xs rounded px-1.5 py-0.5 border border-border w-28"
              placeholder="Preset name"
              value={presetName}
              onChange={e => setPresetName(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' && presetName.trim()) saveMutation.mutate()
                if (e.key === 'Escape') setShowSaveInput(false)
              }}
              autoFocus
            />
            <button
              onClick={() => presetName.trim() && saveMutation.mutate()}
              disabled={!presetName.trim() || saveMutation.isPending}
              className="px-2 py-0.5 bg-primary text-primary-foreground rounded text-xs disabled:opacity-50"
            >
              {saveMutation.isPending ? '...' : 'Save'}
            </button>
            <button
              onClick={() => setShowSaveInput(false)}
              className="px-1.5 py-0.5 bg-muted rounded text-xs text-muted-foreground"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={() => setShowSaveInput(true)}
            className="flex items-center gap-1 px-2 py-0.5 bg-muted hover:bg-muted/80 rounded text-xs text-muted-foreground"
          >
            <Save className="h-3 w-3" /> Save
          </button>
        )}

        {/* Run button */}
        <button
          onClick={() => runMutation.mutate()}
          disabled={runMutation.isPending}
          className="flex items-center gap-1 px-3 py-0.5 bg-green-600 hover:bg-green-700 text-white rounded text-xs font-medium disabled:opacity-50"
        >
          {runMutation.isPending
            ? <Loader2 className="h-3 w-3 animate-spin" />
            : <Play className="h-3 w-3" />
          }
          Run
        </button>
      </div>

      {/* ── Filter builder ── */}
      <div className="px-3 py-2 border-b border-border space-y-1.5">
        {filters.map((f, idx) => (
          <div key={idx} className="flex items-center gap-2">
            {/* Field */}
            <select
              value={f.field}
              onChange={e => updateFilter(idx, { field: e.target.value as FilterField })}
              className="bg-muted text-foreground text-xs rounded px-1.5 py-0.5 border border-border"
            >
              {FILTER_FIELDS.map(ff => (
                <option key={ff.value} value={ff.value}>{ff.label}</option>
              ))}
            </select>

            {/* Operator */}
            <select
              value={f.operator}
              onChange={e => updateFilter(idx, { operator: e.target.value as FilterOperator })}
              className="bg-muted text-foreground text-xs rounded px-1.5 py-0.5 border border-border w-14"
            >
              {FILTER_OPERATORS.map(op => (
                <option key={op.value} value={op.value}>{op.label}</option>
              ))}
            </select>

            {/* Value */}
            <input
              type="number"
              value={f.value}
              onChange={e => updateFilter(idx, { value: parseFloat(e.target.value) || 0 })}
              className="bg-muted text-foreground text-xs rounded px-1.5 py-0.5 border border-border w-20 text-right"
            />

            {/* Remove */}
            <button
              onClick={() => removeFilter(idx)}
              className="text-muted-foreground hover:text-destructive"
            >
              <Trash2 className="h-3 w-3" />
            </button>
          </div>
        ))}

        <button
          onClick={addFilter}
          className="flex items-center gap-1 text-muted-foreground hover:text-foreground text-xs"
        >
          <Plus className="h-3 w-3" /> Add filter
        </button>
      </div>

      {/* ── Error state ── */}
      {runMutation.isError && (
        <div className="px-3 py-2 text-destructive text-xs">
          Error: {(runMutation.error as Error).message}
        </div>
      )}

      {/* ── Results table ── */}
      {runMutation.data && (
        <div className="flex-1 overflow-auto">
          {/* Status bar */}
          <div className="px-3 py-1 text-muted-foreground flex items-center gap-2 border-b border-border">
            <span>{sortedResults.length} results</span>
            {runMutation.data.cached && (
              <span className="text-yellow-500">(cached)</span>
            )}
          </div>

          <table className="w-full">
            <thead className="sticky top-0 bg-background border-b border-border">
              <tr>
                {(
                  [
                    { key: 'symbol'     as SortKey, label: 'Symbol'     },
                    { key: 'name'       as SortKey, label: 'Name'       },
                    { key: 'price'      as SortKey, label: 'Price'      },
                    { key: 'change_24h' as SortKey, label: '24h %'      },
                    { key: 'volume'     as SortKey, label: 'Volume'     },
                    { key: 'market_cap' as SortKey, label: 'Mkt Cap'    },
                    { key: 'pe_ratio'   as SortKey, label: 'P/E'        },
                  ] as { key: SortKey; label: string }[]
                ).map(col => (
                  <th
                    key={col.key}
                    onClick={() => toggleSort(col.key)}
                    className="text-left px-2 py-1 text-muted-foreground font-medium cursor-pointer hover:text-foreground select-none whitespace-nowrap"
                  >
                    {col.label}
                    <SortIcon col={col.key} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sortedResults.map(r => (
                <tr
                  key={r.symbol}
                  className="border-b border-border/50 hover:bg-muted/40 transition-colors"
                >
                  <td className="px-2 py-1 font-mono font-semibold">{r.symbol}</td>
                  <td className="px-2 py-1 text-muted-foreground max-w-[120px] truncate">{r.name}</td>
                  <td className="px-2 py-1 text-right font-mono">{fmtPrice(r.price)}</td>
                  <td className={`px-2 py-1 text-right font-mono ${
                    r.change_24h >= 0 ? 'text-green-500' : 'text-red-500'
                  }`}>
                    {r.change_24h >= 0 ? '+' : ''}{r.change_24h.toFixed(2)}%
                  </td>
                  <td className="px-2 py-1 text-right font-mono text-muted-foreground">{fmtVol(r.volume)}</td>
                  <td className="px-2 py-1 text-right font-mono text-muted-foreground">{fmtVol(r.market_cap)}</td>
                  <td className="px-2 py-1 text-right font-mono text-muted-foreground">
                    {r.pe_ratio > 0 ? r.pe_ratio.toFixed(1) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {sortedResults.length === 0 && (
            <div className="flex items-center justify-center h-24 text-muted-foreground">
              No assets match the current filters.
            </div>
          )}
        </div>
      )}

      {/* ── Empty state (before first run) ── */}
      {!runMutation.data && !runMutation.isPending && (
        <div className="flex-1 flex flex-col items-center justify-center text-muted-foreground gap-2">
          <Play className="h-8 w-8 opacity-30" />
          <p>Configure filters and click <strong>Run</strong></p>
        </div>
      )}

      {/* ── Loading state ── */}
      {runMutation.isPending && (
        <div className="flex-1 flex items-center justify-center text-muted-foreground gap-2">
          <Loader2 className="h-5 w-5 animate-spin" />
          <span>Fetching {assetClass} data…</span>
        </div>
      )}
    </div>
  )
}
```

**Step 2: TypeScript compile check**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 3: Lint check**

```bash
docker compose exec frontend npm run lint
```

Expected: no errors or warnings related to `ScreenerWidget.tsx`.

**Step 4: Commit**

```bash
git add frontend/src/components/widgets/ScreenerWidget.tsx
git commit -m "feat(widgets): implement ScreenerWidget — filter builder, results table, preset management"
```

---

## Task C9: Wire ScreenerWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import** at the top of the import block, after the other Phase A widget imports:

```typescript
// Phase C — Screener
import { ScreenerWidget } from '@/components/widgets/ScreenerWidget'
```

**Step 2: Replace the SCR stub** in `WIDGET_REGISTRY`:

Find:
```typescript
  SCR:  () => <ComingSoon label="Screener (Phase C)" />,
```

Replace with:
```typescript
  SCR:  ScreenerWidget,
```

After the edit the relevant section of the registry should look like:

```typescript
  // Phase B: Heatmap (stub until Phase B is implemented)
  HM:   () => <ComingSoon label="Market Heatmap (Phase B)" />,

  // Phase C: Screener — real implementation
  SCR:  ScreenerWidget,

  // Phase D-H: stubs
  FA:   () => <ComingSoon label="Fundamentals (Phase D)" />,
  CAL:  () => <ComingSoon label="Calendar (Phase E)" />,
  OPT:  () => <ComingSoon label="Options Chain (Phase G)" />,
  YCRV: () => <ComingSoon label="Yield Curves (Phase F)" />,
  RISK: () => <ComingSoon label="Risk Analytics (Phase H)" />,
```

**Step 3: Verify in browser**

```bash
make up
```

Navigate to `http://localhost:5173/terminal`, open or create an `SCR` panel (type `SCR` in the command bar and press Enter). Verify:
- Crypto / Equities / Commodities tabs render
- "Add filter" button adds a filter row
- "Run" button triggers a fetch and populates the table
- Clicking a column header sorts the table
- "Save" flow creates a preset and it appears in the "Load preset" dropdown

**Step 4: Compile + lint**

```bash
docker compose exec frontend npx tsc --noEmit
docker compose exec frontend npm run lint
```

**Step 5: Commit**

```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(terminal): wire ScreenerWidget into WidgetRegistry — replace SCR stub"
```

---

## Task C10: End-to-end verification

Run all backend tests to confirm nothing regressed:

```bash
docker compose exec backend go test ./... -count=1
```

Expected: all existing tests pass, new screener tests pass.

Run frontend lint and type check:

```bash
docker compose exec frontend npm run lint
docker compose exec frontend npx tsc --noEmit
```

Exercise the full flow manually:

1. Open `http://localhost:5173/terminal`
2. In the command bar, type `SCR` and press Enter — an SCR panel opens
3. Select **Crypto**, add a filter `change_24h > 2`, click **Run** — results populate
4. Click the `24h %` column header twice — sorts ascending then descending
5. Click **Save**, enter name "Gainers", press Enter — preset appears in dropdown
6. Reload the page, select the preset from the dropdown — filters restore
7. Click **Run** — same results (potentially cached, shown with yellow `(cached)` badge)
8. Delete the preset by loading it and clicking the trash icon in the preset dropdown row (note: delete from the dropdown is UI-only in this phase; deletion via the API was verified in Task C5 Step 3)
9. Switch to **Equities**, add `pe_ratio < 20`, run — equity results appear with P/E column populated
10. Switch to **Commodities**, no filters, run — commodity ETF results appear

Check Redis cache is working:

```bash
docker compose exec redis redis-cli KEYS "screener:*"
```

Expected: one or more keys like `screener:crypto:[...]`.

Verify preset table in MySQL:

```bash
make db-shell
```

```sql
SELECT id, user_id, name, asset_class, created_at FROM screener_presets;
```

**Final commit (update phase status in reference doc):**

```bash
# Update bloomberg-reference.md: change Phase C status from ⬜ Pending to ✅ Done
git add docs/plans/bloomberg-reference.md
git commit -m "docs: mark Bloomberg Phase C (Screener) as complete"
```

---

## Phase C Completion Checklist

### Backend
- [ ] **C1** — `ScreenerPreset` model added to `models.go`, `screener_presets` table auto-migrated
- [ ] **C2** — `backend/internal/screener/screener.go` created: `Engine`, `Run`, `fetchCrypto`, `fetchYahoo`, `matchesAll`, `matches`
- [ ] **C3** — `backend/internal/screener/screener_test.go` created, all 5 tests pass
- [ ] **C4** — `backend/internal/api/screener_handler.go` created: `run`, `listPresets`, `createPreset`, `deletePreset`, Redis caching with 60 s TTL
- [ ] **C5** — Four screener routes registered in `routes.go` under `/api/v1/screener/`
- [ ] `POST /api/v1/screener/run` returns 200 with `{ results, cached }` for all three asset classes
- [ ] `GET /api/v1/screener/presets` returns 200 with user's presets (empty array when none)
- [ ] `POST /api/v1/screener/presets` returns 201 with created preset
- [ ] `DELETE /api/v1/screener/presets/:id` returns 204; returns 404 for other users' presets
- [ ] Second call to `run` with same params returns `cached: true` within 60 s
- [ ] `go test ./...` passes with no failures

### Frontend
- [ ] **C6** — Screener types appended to `types/terminal.ts`: `AssetClass`, `FilterField`, `FilterOperator`, `ScreenerFilter`, `ScreenerResult`, `ScreenerRunResponse`, `ScreenerPreset`
- [ ] **C7** — `frontend/src/api/screener.ts` created: `runScreener`, `fetchScreenerPresets`, `createScreenerPreset`, `deleteScreenerPreset`
- [ ] **C8** — `frontend/src/components/widgets/ScreenerWidget.tsx` created
  - [ ] Asset class tab switching works
  - [ ] Add / remove filter rows works
  - [ ] Run button triggers `POST /screener/run` via React Query mutation
  - [ ] Results table renders with all 7 columns
  - [ ] Clicking column headers sorts results (asc/desc toggle)
  - [ ] `(cached)` badge appears on second run within 60 s
  - [ ] Save flow creates preset via mutation; query cache invalidated
  - [ ] Preset dropdown loads and restores filters + asset class
  - [ ] Empty state shown before first run
  - [ ] Loading spinner shown during fetch
  - [ ] Error message shown on network failure
- [ ] **C9** — `WidgetRegistry.tsx` imports `ScreenerWidget` and maps `SCR` to it (stub removed)
- [ ] `npx tsc --noEmit` passes with no errors
- [ ] `npm run lint` passes with no errors

### Documentation
- [ ] **C10** — `bloomberg-reference.md` Phase C status updated to ✅ Done
