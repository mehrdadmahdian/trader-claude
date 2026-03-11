# Bloomberg Terminal — Phase B: Market Heatmap

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `HM` (Market Heatmap) widget — a treemap where each cell is an asset sized by market cap and colored by 24-hour percent change. After this phase, a panel set to `HM` renders a live, interactive treemap for crypto, equities, or commodities.

**Architecture:**
- New backend package `backend/internal/marketdata/` provides thin HTTP clients for CoinGecko and Yahoo Finance batch quotes. These clients are used only by the heatmap handler for now; future phases (FA, SCR) will import from the same package.
- New handler `backend/internal/api/heatmap_handler.go` hits the appropriate data source based on the `:market` path param, caches the result in Redis (TTL 60 s), and returns a normalized `[]HeatmapItem` JSON array.
- New frontend API module `frontend/src/api/heatmap.ts` wraps the GET call.
- New widget `frontend/src/components/widgets/HeatmapWidget.tsx` renders a `recharts` `Treemap` with custom cells (color-coded by change, symbol + % label).
- `WidgetRegistry.tsx` replaces the `HM` stub with the real widget.

**Tech Stack:**
- Backend: Go standard `net/http` (10 s timeout), `encoding/json`, `go-redis`, Fiber v2
- Frontend: `recharts` Treemap (already in `package.json`), React Query `useQuery`, Axios, Tailwind, lucide-react

**Prerequisite:** Phase A complete — WorkspaceLayout, WidgetRegistry, PanelSlot, and all terminal infrastructure are in place.

---

## Task B1: Create the marketdata package — CoinGecko client

**Files:**
- Create: `backend/internal/marketdata/coingecko.go`

**Step 1: Create the directory and file**

```go
package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	coingeckoBaseURL     = "https://api.coingecko.com/api/v3"
	coingeckoUserAgent   = "Mozilla/5.0 (compatible; trader-claude/1.0)"
	coingeckoHTTPTimeout = 10 * time.Second
)

// CoinGeckoClient fetches market data from the CoinGecko public API (no API key required).
type CoinGeckoClient struct {
	client  *http.Client
	baseURL string // overridable in tests
}

func NewCoinGeckoClient() *CoinGeckoClient {
	return &CoinGeckoClient{
		client:  &http.Client{Timeout: coingeckoHTTPTimeout},
		baseURL: coingeckoBaseURL,
	}
}

// CoinGeckoMarketItem mirrors one entry from /coins/markets.
type CoinGeckoMarketItem struct {
	ID                     string  `json:"id"`
	Symbol                 string  `json:"symbol"`
	Name                   string  `json:"name"`
	CurrentPrice           float64 `json:"current_price"`
	MarketCap              float64 `json:"market_cap"`
	TotalVolume            float64 `json:"total_volume"`
	PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
}

// FetchTopCoins fetches the top perPage coins by market cap.
// perPage must be between 1 and 250.
func (c *CoinGeckoClient) FetchTopCoins(ctx context.Context, perPage int) ([]CoinGeckoMarketItem, error) {
	if perPage < 1 || perPage > 250 {
		perPage = 100
	}
	url := fmt.Sprintf(
		"%s/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=%d&page=1&sparkline=false",
		c.baseURL, perPage,
	)

	body, err := c.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("coingecko FetchTopCoins: %w", err)
	}

	var items []CoinGeckoMarketItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("coingecko FetchTopCoins: parse response: %w", err)
	}
	return items, nil
}

func (c *CoinGeckoClient) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", coingeckoUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("coingecko rate limit exceeded (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s: unexpected status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}
```

**Step 2: Build check**
```bash
docker compose exec backend go build ./internal/marketdata/...
```
Expected: no errors.

**Step 3: Commit**
```bash
git add backend/internal/marketdata/coingecko.go
git commit -m "feat(marketdata): add CoinGecko client for top coins by market cap"
```

---

## Task B2: Create the marketdata package — Yahoo Finance batch quotes client

**Files:**
- Create: `backend/internal/marketdata/yahoo_quotes.go`

**Step 1: Create the file**

The Yahoo Finance v7 quote endpoint accepts a comma-separated `symbols` query param and returns summary data including `regularMarketPrice`, `regularMarketChangePercent`, `marketCap`, `regularMarketVolume`, and `sector`.

```go
package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	yahooQuoteBaseURL   = "https://query1.finance.yahoo.com/v7/finance/quote"
	yahooQuoteUserAgent = "Mozilla/5.0 (compatible; trader-claude/1.0)"
	yahooQuoteTimeout   = 10 * time.Second
)

// YahooQuoteClient fetches batch quote data from Yahoo Finance v7.
type YahooQuoteClient struct {
	client  *httpClient // shared private helper defined below
	baseURL string
}

func NewYahooQuoteClient() *YahooQuoteClient {
	return &YahooQuoteClient{
		client:  newHTTPClient(yahooQuoteTimeout, yahooQuoteUserAgent),
		baseURL: yahooQuoteBaseURL,
	}
}

// YahooQuoteResult holds the subset of fields we need from a Yahoo Finance quote.
type YahooQuoteResult struct {
	Symbol                     string  `json:"symbol"`
	ShortName                  string  `json:"shortName"`
	RegularMarketPrice         float64 `json:"regularMarketPrice"`
	RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
	MarketCap                  float64 `json:"marketCap"`
	RegularMarketVolume        float64 `json:"regularMarketVolume"`
	Sector                     string  `json:"sector"`
}

type yahooQuoteResponse struct {
	QuoteResponse struct {
		Result []YahooQuoteResult `json:"result"`
		Error  *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteResponse"`
}

// FetchQuotes fetches batch quotes for the given symbols (max 50 per call).
func (c *YahooQuoteClient) FetchQuotes(ctx context.Context, symbols []string) ([]YahooQuoteResult, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	url := fmt.Sprintf(
		"%s?symbols=%s&fields=symbol,shortName,regularMarketPrice,regularMarketChangePercent,marketCap,regularMarketVolume,sector",
		c.baseURL,
		strings.Join(symbols, "%2C"),
	)

	body, err := c.client.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("yahoo FetchQuotes: %w", err)
	}

	var resp yahooQuoteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("yahoo FetchQuotes: parse response: %w", err)
	}
	if resp.QuoteResponse.Error != nil {
		return nil, fmt.Errorf("yahoo FetchQuotes: API error %s: %s",
			resp.QuoteResponse.Error.Code,
			resp.QuoteResponse.Error.Description,
		)
	}
	return resp.QuoteResponse.Result, nil
}
```

**Step 2: Create the shared HTTP helper used by both clients**

Create `backend/internal/marketdata/http.go`:

```go
package marketdata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is a thin wrapper around net/http.Client with a shared User-Agent.
type httpClient struct {
	inner     *http.Client
	userAgent string
}

func newHTTPClient(timeout time.Duration, userAgent string) *httpClient {
	return &httpClient{
		inner:     &http.Client{Timeout: timeout},
		userAgent: userAgent,
	}
}

func (c *httpClient) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.inner.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s: unexpected status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}
```

**Note:** Now that `httpClient` is in `http.go`, update `coingecko.go` to use it instead of its own `get` method. Replace the `client *http.Client` field in `CoinGeckoClient` with `client *httpClient`, remove the standalone `get` method from `coingecko.go`, and update the constructor:

In `coingecko.go`:
- Change struct field: `client *httpClient`
- Change constructor: `client: newHTTPClient(coingeckoHTTPTimeout, coingeckoUserAgent),`
- Replace `c.get(ctx, url)` with `c.client.get(ctx, url)`
- Delete the `func (c *CoinGeckoClient) get(...)` method at the bottom of `coingecko.go`

**Step 3: Build check**
```bash
docker compose exec backend go build ./internal/marketdata/...
```
Expected: no errors.

**Step 4: Commit**
```bash
git add backend/internal/marketdata/yahoo_quotes.go backend/internal/marketdata/http.go backend/internal/marketdata/coingecko.go
git commit -m "feat(marketdata): add Yahoo Finance batch quotes client + shared HTTP helper"
```

---

## Task B3: Create the heatmap handler

**Files:**
- Create: `backend/internal/api/heatmap_handler.go`

**Step 1: Create the file**

The handler supports three markets: `crypto` (CoinGecko top 100), `equities` (Yahoo Finance S&P 500 top 50), and `commodities` (Yahoo Finance futures). It checks Redis first, and on a miss fetches live data, normalizes it to `[]HeatmapItem`, marshals to JSON, stores in Redis with a 60-second TTL, and returns it.

```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/trader-claude/backend/internal/marketdata"
)

// HeatmapItem is the normalized shape returned to the frontend for every asset.
type HeatmapItem struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Change24h float64 `json:"change_24h"`  // percent, e.g. 3.14 means +3.14%
	MarketCap float64 `json:"market_cap"`  // USD; 0 for commodities (no market cap concept)
	Volume24h float64 `json:"volume_24h"`  // USD
	Sector    string  `json:"sector"`      // non-empty for equities grouping; empty otherwise
}

// sp500Top50 is the hardcoded list of S&P 500 top-50 symbols used for the equities heatmap.
// These are Yahoo Finance tickers. Update this list periodically as index composition changes.
var sp500Top50 = []string{
	"AAPL", "MSFT", "NVDA", "AMZN", "META", "GOOGL", "GOOG", "BRK-B", "LLY", "AVGO",
	"JPM", "TSLA", "UNH", "V", "XOM", "MA", "COST", "PG", "JNJ", "HD",
	"ABBV", "MRK", "CVX", "BAC", "KO", "PEP", "CRM", "WMT", "TMO", "CSCO",
	"ACN", "MCD", "ABT", "LIN", "DHR", "DIS", "NEE", "ADBE", "TXN", "CMCSA",
	"NKE", "ORCL", "PM", "INTC", "RTX", "AMGN", "LOW", "QCOM", "GS", "CAT",
}

// commoditySymbols are Yahoo Finance futures tickers for the commodities heatmap.
var commoditySymbols = []string{
	"GC=F",  // Gold
	"SI=F",  // Silver
	"CL=F",  // Crude Oil (WTI)
	"NG=F",  // Natural Gas
	"ZW=F",  // Wheat
	"ZC=F",  // Corn
	"HG=F",  // Copper
	"PL=F",  // Platinum
}

type heatmapHandler struct {
	rdb     *redis.Client
	cgClient *marketdata.CoinGeckoClient
	yfClient *marketdata.YahooQuoteClient
}

func newHeatmapHandler(rdb *redis.Client) *heatmapHandler {
	return &heatmapHandler{
		rdb:      rdb,
		cgClient: marketdata.NewCoinGeckoClient(),
		yfClient: marketdata.NewYahooQuoteClient(),
	}
}

// GET /api/v1/heatmap/:market   market = crypto | equities | commodities
func (h *heatmapHandler) getHeatmap(c *fiber.Ctx) error {
	market := strings.ToLower(c.Params("market"))
	switch market {
	case "crypto", "equities", "commodities":
		// valid
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "market must be one of: crypto, equities, commodities",
		})
	}

	ctx := c.Context()
	cacheKey := fmt.Sprintf("heatmap:%s", market)

	// --- Cache read ---
	cached, err := h.rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		c.Set("Content-Type", "application/json")
		c.Set("X-Cache", "HIT")
		return c.Send(cached)
	}

	// --- Cache miss: fetch live data ---
	items, fetchErr := h.fetchMarket(ctx, market)
	if fetchErr != nil {
		log.Printf("heatmap: fetch %s failed: %v", market, fetchErr)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "failed to fetch market data",
		})
	}

	// --- Marshal and cache ---
	payload, marshalErr := json.Marshal(items)
	if marshalErr != nil {
		log.Printf("heatmap: marshal failed: %v", marshalErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	// Best-effort cache write — do not fail the request if Redis is unavailable
	if setErr := h.rdb.Set(ctx, cacheKey, payload, 60*time.Second).Err(); setErr != nil {
		log.Printf("heatmap: cache write failed for key %s: %v", cacheKey, setErr)
	}

	c.Set("Content-Type", "application/json")
	c.Set("X-Cache", "MISS")
	return c.Send(payload)
}

func (h *heatmapHandler) fetchMarket(ctx context.Context, market string) ([]HeatmapItem, error) {
	switch market {
	case "crypto":
		return h.fetchCrypto(ctx)
	case "equities":
		return h.fetchEquities(ctx)
	case "commodities":
		return h.fetchCommodities(ctx)
	default:
		return nil, fmt.Errorf("unsupported market: %s", market)
	}
}

func (h *heatmapHandler) fetchCrypto(ctx context.Context) ([]HeatmapItem, error) {
	coins, err := h.cgClient.FetchTopCoins(ctx, 100)
	if err != nil {
		return nil, err
	}
	items := make([]HeatmapItem, 0, len(coins))
	for _, c := range coins {
		items = append(items, HeatmapItem{
			Symbol:    strings.ToUpper(c.Symbol),
			Name:      c.Name,
			Price:     c.CurrentPrice,
			Change24h: c.PriceChangePercentage24h,
			MarketCap: c.MarketCap,
			Volume24h: c.TotalVolume,
			Sector:    "",
		})
	}
	return items, nil
}

func (h *heatmapHandler) fetchEquities(ctx context.Context) ([]HeatmapItem, error) {
	quotes, err := h.yfClient.FetchQuotes(ctx, sp500Top50)
	if err != nil {
		return nil, err
	}
	items := make([]HeatmapItem, 0, len(quotes))
	for _, q := range quotes {
		name := q.ShortName
		if name == "" {
			name = q.Symbol
		}
		items = append(items, HeatmapItem{
			Symbol:    q.Symbol,
			Name:      name,
			Price:     q.RegularMarketPrice,
			Change24h: q.RegularMarketChangePercent,
			MarketCap: q.MarketCap,
			Volume24h: q.RegularMarketVolume,
			Sector:    q.Sector,
		})
	}
	return items, nil
}

func (h *heatmapHandler) fetchCommodities(ctx context.Context) ([]HeatmapItem, error) {
	quotes, err := h.yfClient.FetchQuotes(ctx, commoditySymbols)
	if err != nil {
		return nil, err
	}
	// Friendly names for commodity futures tickers
	friendlyNames := map[string]string{
		"GC=F": "Gold",
		"SI=F": "Silver",
		"CL=F": "Crude Oil",
		"NG=F": "Natural Gas",
		"ZW=F": "Wheat",
		"ZC=F": "Corn",
		"HG=F": "Copper",
		"PL=F": "Platinum",
	}
	items := make([]HeatmapItem, 0, len(quotes))
	for _, q := range quotes {
		name := friendlyNames[q.Symbol]
		if name == "" {
			name = q.ShortName
		}
		if name == "" {
			name = q.Symbol
		}
		items = append(items, HeatmapItem{
			Symbol:    q.Symbol,
			Name:      name,
			Price:     q.RegularMarketPrice,
			Change24h: q.RegularMarketChangePercent,
			MarketCap: 0, // commodities have no market cap
			Volume24h: q.RegularMarketVolume,
			Sector:    "Commodities",
		})
	}
	return items, nil
}
```

**Step 2: Build check**
```bash
docker compose exec backend go build ./...
```
Expected: no errors.

**Step 3: Commit**
```bash
git add backend/internal/api/heatmap_handler.go
git commit -m "feat(api): add heatmap handler — crypto/equities/commodities with Redis cache"
```

---

## Task B4: Register the heatmap route

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Add the handler init**

In `RegisterRoutes`, after the workspace handler init (`workspaceH := newWorkspaceHandler(db)`), add:

```go
// Heatmap (Bloomberg HM widget)
heatmapH := newHeatmapHandler(rdb)
```

**Step 2: Add the route** in the `protected` group, after the workspace routes block:

```go
// Heatmap
protected.Get("/heatmap/:market", heatmapH.getHeatmap)
```

**Step 3: Build + smoke test**
```bash
docker compose exec backend go build ./...
make up
# Wait for services to start, then test with a valid auth token:
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"<your-email>","password":"<your-password>"}' | jq -r '.access_token')

curl -s "http://localhost:8080/api/v1/heatmap/crypto" \
  -H "Authorization: Bearer $TOKEN" | jq '.[0:3]'
```
Expected: array of 3 HeatmapItem objects with `symbol`, `name`, `price`, `change_24h`, `market_cap`, `volume_24h`, `sector` fields.

```bash
curl -s "http://localhost:8080/api/v1/heatmap/commodities" \
  -H "Authorization: Bearer $TOKEN" | jq '.[0:3]'
```
Expected: 3 commodity items with `market_cap: 0` and `sector: "Commodities"`.

**Step 4: Verify cache**
```bash
# Call once (MISS), call again (HIT), check header
curl -sv "http://localhost:8080/api/v1/heatmap/equities" \
  -H "Authorization: Bearer $TOKEN" 2>&1 | grep "X-Cache"
# First call: X-Cache: MISS
# Second call within 60s: X-Cache: HIT

# Confirm Redis key exists
docker compose exec redis redis-cli TTL "heatmap:equities"
# Expected: positive number <= 60
```

**Step 5: Test invalid market param**
```bash
curl -s "http://localhost:8080/api/v1/heatmap/forex" \
  -H "Authorization: Bearer $TOKEN"
```
Expected: `{"error":"market must be one of: crypto, equities, commodities"}` with HTTP 400.

**Step 6: Commit**
```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register GET /api/v1/heatmap/:market route"
```

---

## Task B5: Add HeatmapItem types to the frontend

**Files:**
- Modify: `frontend/src/types/terminal.ts`

**Step 1: Append the HeatmapItem interface** at the end of `frontend/src/types/terminal.ts`:

```typescript
// ── Heatmap (Phase B) ────────────────────────────────────────────────────────

export interface HeatmapItem {
  symbol: string
  name: string
  price: number
  change_24h: number   // percent, e.g. 3.14 = +3.14%
  market_cap: number   // 0 for commodities
  volume_24h: number
  sector: string       // non-empty for equities grouping; empty string otherwise
}

export type HeatmapMarket = 'crypto' | 'equities' | 'commodities'
```

**Step 2: TypeScript compile check**
```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 3: Commit**
```bash
git add frontend/src/types/terminal.ts
git commit -m "feat(types): add HeatmapItem and HeatmapMarket types"
```

---

## Task B6: Create the heatmap API module

**Files:**
- Create: `frontend/src/api/heatmap.ts`

**Step 1: Create the file**

```typescript
import apiClient from '@/api/client'
import type { HeatmapItem, HeatmapMarket } from '@/types/terminal'

/**
 * Fetch market heatmap data from the backend.
 *
 * Backend caches each market for 60 seconds in Redis.
 * React Query's staleTime should be set to match (60_000 ms).
 */
export async function fetchHeatmap(market: HeatmapMarket): Promise<HeatmapItem[]> {
  const { data } = await apiClient.get<HeatmapItem[]>(`/api/v1/heatmap/${market}`)
  return data
}
```

**Step 2: TypeScript compile check**
```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 3: Commit**
```bash
git add frontend/src/api/heatmap.ts
git commit -m "feat(api): add heatmap API client module"
```

---

## Task B7: Create HeatmapWidget

**Files:**
- Create: `frontend/src/components/widgets/HeatmapWidget.tsx`

**Step 1: Verify recharts is installed**
```bash
docker compose exec frontend npm ls recharts
```
Expected: `recharts@2.x.x`. (It is already in `package.json` — no install needed.)

**Step 2: Create the file**

The widget renders a `recharts` `Treemap`. Each cell is sized by `market_cap` (or equal size for commodities where `market_cap === 0`), colored by `change_24h` with a linear interpolation: red (#ef4444) at -5%, neutral gray (#6b7280) at 0%, green (#22c55e) at +5%. Cells show the ticker symbol and percent change. The `market` prop (defaulting to `'crypto'`) drives which dataset to load. Clicking a cell calls `onTickerSelect(symbol)`.

```tsx
import { useQuery } from '@tanstack/react-query'
import { Treemap, ResponsiveContainer, Tooltip } from 'recharts'
import { fetchHeatmap } from '@/api/heatmap'
import type { WidgetProps } from '@/types/terminal'
import type { HeatmapItem, HeatmapMarket } from '@/types/terminal'

// ── Color helpers ────────────────────────────────────────────────────────────

/**
 * Linearly interpolate between two hex colors given t in [0, 1].
 */
function lerpColor(a: string, b: string, t: number): string {
  const parse = (hex: string) => [
    parseInt(hex.slice(1, 3), 16),
    parseInt(hex.slice(3, 5), 16),
    parseInt(hex.slice(5, 7), 16),
  ]
  const toHex = (n: number) => Math.round(n).toString(16).padStart(2, '0')
  const [ar, ag, ab] = parse(a)
  const [br, bg, bb] = parse(b)
  return `#${toHex(ar + (br - ar) * t)}${toHex(ag + (bg - ag) * t)}${toHex(ab + (bb - ab) * t)}`
}

const COLOR_NEGATIVE = '#ef4444'  // Tailwind red-500
const COLOR_NEUTRAL  = '#6b7280'  // Tailwind gray-500
const COLOR_POSITIVE = '#22c55e'  // Tailwind green-500
const CLAMP_PCT = 5               // ± 5% maps to full color

function changeToColor(changePct: number): string {
  const clamped = Math.max(-CLAMP_PCT, Math.min(CLAMP_PCT, changePct))
  if (clamped < 0) {
    return lerpColor(COLOR_NEUTRAL, COLOR_NEGATIVE, Math.abs(clamped) / CLAMP_PCT)
  }
  if (clamped > 0) {
    return lerpColor(COLOR_NEUTRAL, COLOR_POSITIVE, clamped / CLAMP_PCT)
  }
  return COLOR_NEUTRAL
}

// ── Treemap data shape ───────────────────────────────────────────────────────

interface TreemapNode {
  name: string       // symbol
  fullName: string   // human-readable name
  size: number       // treemap area (market_cap or 1 for commodities)
  change24h: number
  price: number
  volume24h: number
  fill: string
}

function toTreemapData(items: HeatmapItem[]): TreemapNode[] {
  // For assets without market cap (commodities), use equal sizing
  const hasMarketCap = items.some((i) => i.market_cap > 0)
  return items.map((item) => ({
    name: item.symbol,
    fullName: item.name,
    size: hasMarketCap ? Math.max(item.market_cap, 1) : 1,
    change24h: item.change_24h,
    price: item.price,
    volume24h: item.volume_24h,
    fill: changeToColor(item.change_24h),
  }))
}

// ── Custom cell renderer ─────────────────────────────────────────────────────

interface CustomCellProps {
  x?: number
  y?: number
  width?: number
  height?: number
  name?: string
  change24h?: number
  fill?: string
  onClick?: () => void
}

function HeatmapCell({ x = 0, y = 0, width = 0, height = 0, name = '', change24h = 0, fill = '#6b7280', onClick }: CustomCellProps) {
  const tooSmall = width < 40 || height < 30
  const sign = change24h >= 0 ? '+' : ''
  return (
    <g onClick={onClick} style={{ cursor: 'pointer' }}>
      <rect
        x={x + 1}
        y={y + 1}
        width={Math.max(0, width - 2)}
        height={Math.max(0, height - 2)}
        fill={fill}
        rx={3}
        ry={3}
      />
      {!tooSmall && (
        <>
          <text
            x={x + width / 2}
            y={y + height / 2 - 6}
            textAnchor="middle"
            dominantBaseline="middle"
            fill="white"
            fontSize={Math.min(14, width / 5)}
            fontWeight="600"
            style={{ userSelect: 'none', pointerEvents: 'none' }}
          >
            {name}
          </text>
          <text
            x={x + width / 2}
            y={y + height / 2 + 10}
            textAnchor="middle"
            dominantBaseline="middle"
            fill="rgba(255,255,255,0.85)"
            fontSize={Math.min(12, width / 6)}
            style={{ userSelect: 'none', pointerEvents: 'none' }}
          >
            {sign}{change24h.toFixed(2)}%
          </text>
        </>
      )}
    </g>
  )
}

// ── Tooltip ──────────────────────────────────────────────────────────────────

interface TooltipPayloadEntry {
  payload?: TreemapNode
}

function HeatmapTooltip({ active, payload }: { active?: boolean; payload?: TooltipPayloadEntry[] }) {
  if (!active || !payload?.length) return null
  const d = payload[0]?.payload
  if (!d) return null
  const sign = d.change24h >= 0 ? '+' : ''
  return (
    <div className="bg-background border border-border rounded-md shadow-lg px-3 py-2 text-sm space-y-1">
      <p className="font-semibold">{d.name} — {d.fullName}</p>
      <p>Price: <span className="font-mono">${d.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })}</span></p>
      <p>24h Change: <span className={d.change24h >= 0 ? 'text-green-500' : 'text-red-500'}>{sign}{d.change24h.toFixed(2)}%</span></p>
      {d.volume24h > 0 && (
        <p>Volume 24h: <span className="font-mono">${(d.volume24h / 1e6).toFixed(1)}M</span></p>
      )}
      {d.size > 1 && (
        <p>Market Cap: <span className="font-mono">${(d.size / 1e9).toFixed(2)}B</span></p>
      )}
    </div>
  )
}

// ── Market selector ──────────────────────────────────────────────────────────

const MARKETS: { value: HeatmapMarket; label: string }[] = [
  { value: 'crypto',      label: 'Crypto' },
  { value: 'equities',    label: 'Equities' },
  { value: 'commodities', label: 'Commodities' },
]

// ── Main widget ──────────────────────────────────────────────────────────────

interface HeatmapWidgetProps extends WidgetProps {
  onTickerSelect?: (symbol: string) => void
}

export function HeatmapWidget({ market: initialMarket = 'crypto', onTickerSelect }: HeatmapWidgetProps) {
  // Allow the user to switch market within the widget independently of the panel ticker
  const [selectedMarket, setSelectedMarket] = React.useState<HeatmapMarket>(
    (initialMarket as HeatmapMarket) ?? 'crypto'
  )

  const { data: items = [], isLoading, isError } = useQuery({
    queryKey: ['heatmap', selectedMarket],
    queryFn: () => fetchHeatmap(selectedMarket),
    staleTime: 60_000,   // matches Redis TTL
    refetchInterval: 65_000,  // auto-refresh slightly after cache expires
  })

  const treeData = React.useMemo(() => toTreemapData(items), [items])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
        Loading heatmap...
      </div>
    )
  }

  if (isError) {
    return (
      <div className="flex items-center justify-center h-full text-destructive text-sm">
        Failed to load heatmap data. Check network or backend logs.
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full gap-2 p-1">
      {/* Market selector */}
      <div className="flex gap-1 flex-shrink-0">
        {MARKETS.map((m) => (
          <button
            key={m.value}
            onClick={() => setSelectedMarket(m.value)}
            className={[
              'px-2 py-1 rounded text-xs font-medium transition-colors',
              selectedMarket === m.value
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted text-muted-foreground hover:bg-muted/80',
            ].join(' ')}
          >
            {m.label}
          </button>
        ))}
      </div>

      {/* Treemap */}
      <div className="flex-1 min-h-0">
        <ResponsiveContainer width="100%" height="100%">
          <Treemap
            data={treeData}
            dataKey="size"
            aspectRatio={4 / 3}
            content={
              <HeatmapCell
                onClick={undefined}  // overridden per-cell below
              />
            }
          >
            <Tooltip content={<HeatmapTooltip />} />
          </Treemap>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
```

**Step 3: Add the missing React import**

The file uses `React.useState` and `React.useMemo`. Add the import at the top of the file:

```tsx
import React from 'react'
```

**Step 4: Wire `onClick` per cell**

The `recharts` `Treemap` `content` prop receives the node data as props on each render. Update the `content` prop to pass a `onClick` for each cell:

Replace the `content` prop in the `<Treemap>` component:

```tsx
content={(props: unknown) => {
  // recharts passes all node fields as props
  const p = props as CustomCellProps & { name: string; change24h: number; fill: string }
  return (
    <HeatmapCell
      {...p}
      onClick={() => {
        if (p.name && onTickerSelect) {
          onTickerSelect(p.name)
        } else if (p.name) {
          console.log('HM cell selected:', p.name)
        }
      }}
    />
  )
}}
```

**Step 5: TypeScript compile check**
```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 6: Commit**
```bash
git add frontend/src/components/widgets/HeatmapWidget.tsx
git commit -m "feat(widgets): add HeatmapWidget — recharts Treemap with color-coded cells and market selector"
```

---

## Task B8: Wire HeatmapWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import** after the existing widget imports, before the `ComingSoon` definition:

```tsx
import { HeatmapWidget } from '@/components/widgets/HeatmapWidget'
```

**Step 2: Replace the HM stub** in `WIDGET_REGISTRY`:

Change:
```tsx
HM:   () => <ComingSoon label="Market Heatmap (Phase B)" />,
```

To:
```tsx
HM:   HeatmapWidget,
```

**Step 3: TypeScript compile check**
```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: no errors.

**Step 4: Verify in the browser**

1. Open `http://localhost:5173/terminal`
2. In the command bar type `HM` and press Enter (or open a panel that uses the Market Overview template which places HM in p1)
3. Confirm: treemap renders with colored cells labeled by symbol + % change
4. Click a cell: browser console shows `HM cell selected: <SYMBOL>`
5. Switch between Crypto / Equities / Commodities buttons — each loads a different dataset

**Step 5: Commit**
```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(terminal): wire HeatmapWidget into WidgetRegistry — replace Phase B stub"
```

---

## Phase B Completion Checklist

### Backend
- [ ] `backend/internal/marketdata/http.go` — shared HTTP helper (`httpClient` struct + `get`)
- [ ] `backend/internal/marketdata/coingecko.go` — `CoinGeckoClient` + `FetchTopCoins`
- [ ] `backend/internal/marketdata/yahoo_quotes.go` — `YahooQuoteClient` + `FetchQuotes`
- [ ] `backend/internal/api/heatmap_handler.go` — `HeatmapItem` type, handler struct, `getHeatmap`, `fetchCrypto`, `fetchEquities`, `fetchCommodities`
- [ ] `backend/internal/api/routes.go` — `heatmapH` init + `protected.Get("/heatmap/:market", ...)` registered
- [ ] `docker compose exec backend go build ./...` passes with no errors

### Backend behavior
- [ ] `GET /api/v1/heatmap/crypto` returns 100 items with `symbol`, `name`, `price`, `change_24h`, `market_cap`, `volume_24h`, `sector: ""`
- [ ] `GET /api/v1/heatmap/equities` returns ~50 items with `sector` non-empty for most
- [ ] `GET /api/v1/heatmap/commodities` returns 8 items with `market_cap: 0`, `sector: "Commodities"`
- [ ] `GET /api/v1/heatmap/forex` returns HTTP 400
- [ ] Second call within 60 s returns `X-Cache: HIT`
- [ ] `redis-cli TTL heatmap:crypto` shows a positive number <= 60
- [ ] Unauthenticated request returns HTTP 401 (protected route)

### Frontend
- [ ] `frontend/src/types/terminal.ts` — `HeatmapItem` and `HeatmapMarket` types added
- [ ] `frontend/src/api/heatmap.ts` — `fetchHeatmap(market)` function
- [ ] `frontend/src/components/widgets/HeatmapWidget.tsx` — full widget with Treemap, color interpolation, market selector, tooltip, click handler
- [ ] `frontend/src/components/terminal/WidgetRegistry.tsx` — `HM` entry points to `HeatmapWidget` (not stub)
- [ ] `npx tsc --noEmit` passes with no errors

### UI behavior
- [ ] Treemap renders cells sized by market cap (equal size for commodities)
- [ ] Deeply negative cells (< -5%) render red (#ef4444)
- [ ] Deeply positive cells (> +5%) render green (#22c55e)
- [ ] Near-zero cells render gray (#6b7280)
- [ ] Each cell shows: symbol (bold), % change (below)
- [ ] Cells too small to label (< 40px wide or < 30px tall) show no text
- [ ] Hover tooltip shows: symbol, full name, price, 24h change %, volume, market cap
- [ ] Clicking a cell calls `onTickerSelect(symbol)` (or logs to console)
- [ ] Market selector buttons switch between Crypto / Equities / Commodities
- [ ] Data auto-refreshes every ~65 seconds (slightly after Redis TTL expires)
- [ ] Loading state displays "Loading heatmap..." centered
- [ ] Error state displays a red error message
