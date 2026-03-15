# Bloomberg Terminal — Phase D: Fundamentals (FA Widget)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `FA` (Fundamentals) widget for the Bloomberg terminal workspace. After this phase, typing `AAPL FA` or `BTC FA` in the command bar loads a rich fundamentals panel: equity data (P/E, EPS, revenue, balance sheet) from Yahoo Finance `quoteSummary`, and crypto data (market cap, supply, ATH, rank) from CoinGecko. Results are Redis-cached for 5 minutes.

**Architecture:** A new Go handler (`fundamentals_handler.go`) fetches from Yahoo Finance or CoinGecko based on the `?market=` query param, normalises the response into a shared `FundamentalsData` struct, caches in Redis (TTL 300s), and serves it via `GET /api/v1/fundamentals/:symbol`. The frontend `api/fundamentals.ts` calls this route; `FundamentalsWidget.tsx` renders conditional sections for equity vs crypto. The `WidgetRegistry.tsx` replaces the Phase D stub with the real component.

**Tech Stack:** Go 1.24, Fiber v2, go-redis v9, standard `net/http` (10s timeout), React 18, TypeScript, React Query v5, Tailwind CSS, lucide-react.

**Prerequisite:** Phase A complete — `WidgetRegistry.tsx`, `WorkspaceLayout`, and the protected route group in `routes.go` all exist.

---

## Task D1: Define FundamentalsData response struct

**Files:**
- Modify: `backend/internal/api/fundamentals_handler.go` (create new)

**Step 1: Create the handler file with the response struct and CoinGecko ID map**

Create `backend/internal/api/fundamentals_handler.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const (
	fundamentalsTTL        = 300 * time.Second
	fundamentalsHTTPTimeout = 10 * time.Second
	yahooQuoteSummaryURL   = "https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=financialData,defaultKeyStatistics,summaryDetail,assetProfile"
	coingeckoCoinsURL      = "https://api.coingecko.com/api/v3/coins/%s?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false"
	fundamentalsUserAgent  = "Mozilla/5.0 (compatible; trader-claude/1.0)"
)

// coingeckoIDMap maps uppercase ticker symbols to CoinGecko coin IDs.
var coingeckoIDMap = map[string]string{
	"BTC":   "bitcoin",
	"ETH":   "ethereum",
	"BNB":   "binancecoin",
	"SOL":   "solana",
	"XRP":   "ripple",
	"ADA":   "cardano",
	"AVAX":  "avalanche-2",
	"DOT":   "polkadot",
	"MATIC": "matic-network",
	"LINK":  "chainlink",
	"UNI":   "uniswap",
	"ATOM":  "cosmos",
	"LTC":   "litecoin",
	"BCH":   "bitcoin-cash",
	"ALGO":  "algorand",
	"XLM":   "stellar",
	"NEAR":  "near",
	"FTM":   "fantom",
	"SAND":  "the-sandbox",
	"MANA":  "decentraland",
}

// FundamentalsData is the unified response shape for both equity and crypto.
type FundamentalsData struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Market    string `json:"market"` // "yahoo" | "coingecko"
	Price     float64 `json:"price"`
	MarketCap float64 `json:"market_cap"`
	// Equity fields (zero for crypto)
	PE            float64 `json:"pe"`
	ForwardPE     float64 `json:"forward_pe"`
	EPS           float64 `json:"eps"`
	Revenue       float64 `json:"revenue"`
	GrossProfit   float64 `json:"gross_profit"`
	TotalDebt     float64 `json:"total_debt"`
	TotalCash     float64 `json:"total_cash"`
	DividendYield float64 `json:"dividend_yield"`
	Beta          float64 `json:"beta"`
	Week52High    float64 `json:"week_52_high"`
	Week52Low     float64 `json:"week_52_low"`
	// Crypto fields (zero for equity)
	CirculatingSupply float64 `json:"circulating_supply"`
	TotalSupply       float64 `json:"total_supply"`
	MaxSupply         float64 `json:"max_supply"`
	ATH               float64 `json:"ath"`
	ATHChangePercent  float64 `json:"ath_change_percent"`
	MarketCapRank     int     `json:"market_cap_rank"`
	Change24h         float64 `json:"change_24h"`
	Volume24h         float64 `json:"volume_24h"`
	Description       string  `json:"description"`
}

type fundamentalsHandler struct {
	rdb        *redis.Client
	httpClient *http.Client
}

func newFundamentalsHandler(rdb *redis.Client) *fundamentalsHandler {
	return &fundamentalsHandler{
		rdb: rdb,
		httpClient: &http.Client{
			Timeout: fundamentalsHTTPTimeout,
		},
	}
}
```

**Step 2: Build check (struct only — no handler method yet)**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add backend/internal/api/fundamentals_handler.go
git commit -m "feat(fundamentals): add FundamentalsData struct and handler skeleton"
```

---

## Task D2: Implement the Yahoo Finance fetch

**Files:**
- Modify: `backend/internal/api/fundamentals_handler.go`

**Step 1: Add Yahoo Finance response structs and fetch function**

Append the following to `backend/internal/api/fundamentals_handler.go` after the `fundamentalsHandler` struct:

```go
// --- Yahoo Finance quoteSummary response shapes ---

type yahooQuoteSummaryResponse struct {
	QuoteSummary struct {
		Result []struct {
			FinancialData struct {
				CurrentPrice      yahooValue `json:"currentPrice"`
				TotalRevenue      yahooValue `json:"totalRevenue"`
				GrossProfits      yahooValue `json:"grossProfits"`
				OperatingCashflow yahooValue `json:"operatingCashflow"`
				TotalDebt         yahooValue `json:"totalDebt"`
				TotalCash         yahooValue `json:"totalCash"`
			} `json:"financialData"`
			DefaultKeyStatistics struct {
				TrailingEps  yahooValue `json:"trailingEps"`
				ForwardPE    yahooValue `json:"forwardPE"`
				TrailingPE   yahooValue `json:"trailingPE"`
				MarketCap    yahooValue `json:"marketCap"`
			} `json:"defaultKeyStatistics"`
			SummaryDetail struct {
				DividendYield  yahooValue `json:"dividendYield"`
				Beta           yahooValue `json:"beta"`
				FiftyTwoWeekHigh yahooValue `json:"fiftyTwoWeekHigh"`
				FiftyTwoWeekLow  yahooValue `json:"fiftyTwoWeekLow"`
			} `json:"summaryDetail"`
			AssetProfile struct {
				LongBusinessSummary string `json:"longBusinessSummary"`
				ShortName           string `json:"shortName"`
				LongName            string `json:"longName"`
			} `json:"assetProfile"`
			Price struct {
				ShortName string `json:"shortName"`
				LongName  string `json:"longName"`
			} `json:"price"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteSummary"`
}

// yahooValue represents a Yahoo Finance numeric field that may be null.
type yahooValue struct {
	Raw float64 `json:"raw"`
}

func (h *fundamentalsHandler) fetchYahoo(ctx context.Context, symbol string) (*FundamentalsData, error) {
	url := fmt.Sprintf(yahooQuoteSummaryURL, symbol)
	body, err := h.httpGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("yahoo quoteSummary GET: %w", err)
	}

	var resp yahooQuoteSummaryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("yahoo quoteSummary parse: %w", err)
	}
	if resp.QuoteSummary.Error != nil {
		return nil, fmt.Errorf("yahoo quoteSummary API error %s: %s",
			resp.QuoteSummary.Error.Code, resp.QuoteSummary.Error.Description)
	}
	if len(resp.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("yahoo quoteSummary: no result for symbol %q", symbol)
	}

	r := resp.QuoteSummary.Result[0]

	name := r.AssetProfile.LongName
	if name == "" {
		name = r.AssetProfile.ShortName
	}
	if name == "" {
		name = r.Price.LongName
	}
	if name == "" {
		name = r.Price.ShortName
	}

	// currentPrice may come from financialData
	price := r.FinancialData.CurrentPrice.Raw

	return &FundamentalsData{
		Symbol:        strings.ToUpper(symbol),
		Name:          name,
		Market:        "yahoo",
		Price:         price,
		MarketCap:     r.DefaultKeyStatistics.MarketCap.Raw,
		PE:            r.DefaultKeyStatistics.TrailingPE.Raw,
		ForwardPE:     r.DefaultKeyStatistics.ForwardPE.Raw,
		EPS:           r.DefaultKeyStatistics.TrailingEps.Raw,
		Revenue:       r.FinancialData.TotalRevenue.Raw,
		GrossProfit:   r.FinancialData.GrossProfits.Raw,
		TotalDebt:     r.FinancialData.TotalDebt.Raw,
		TotalCash:     r.FinancialData.TotalCash.Raw,
		DividendYield: r.SummaryDetail.DividendYield.Raw,
		Beta:          r.SummaryDetail.Beta.Raw,
		Week52High:    r.SummaryDetail.FiftyTwoWeekHigh.Raw,
		Week52Low:     r.SummaryDetail.FiftyTwoWeekLow.Raw,
		Description:   r.AssetProfile.LongBusinessSummary,
	}, nil
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add backend/internal/api/fundamentals_handler.go
git commit -m "feat(fundamentals): implement Yahoo Finance quoteSummary fetch"
```

---

## Task D3: Implement the CoinGecko fetch

**Files:**
- Modify: `backend/internal/api/fundamentals_handler.go`

**Step 1: Add CoinGecko response structs and fetch function**

Append the following to `backend/internal/api/fundamentals_handler.go`:

```go
// --- CoinGecko /coins/{id} response shapes ---

type coingeckoCoinResponse struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Description struct {
		En string `json:"en"`
	} `json:"description"`
	MarketCapRank int `json:"market_cap_rank"`
	MarketData    struct {
		CurrentPrice map[string]float64 `json:"current_price"`
		MarketCap    map[string]float64 `json:"market_cap"`
		TotalVolume  map[string]float64 `json:"total_volume"`
		PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
		CirculatingSupply float64 `json:"circulating_supply"`
		TotalSupply       *float64 `json:"total_supply"`
		MaxSupply         *float64 `json:"max_supply"`
		ATH map[string]float64 `json:"ath"`
		ATHChangePercentage map[string]float64 `json:"ath_change_percentage"`
	} `json:"market_data"`
}

func (h *fundamentalsHandler) fetchCoingecko(ctx context.Context, symbol string) (*FundamentalsData, error) {
	coinID, ok := coingeckoIDMap[strings.ToUpper(symbol)]
	if !ok {
		return nil, fmt.Errorf("coingecko: no ID mapping for symbol %q", symbol)
	}

	url := fmt.Sprintf(coingeckoCoinsURL, coinID)
	body, err := h.httpGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("coingecko coins GET: %w", err)
	}

	var resp coingeckoCoinResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("coingecko coins parse: %w", err)
	}

	// CoinGecko returns HTTP 200 even for errors in some cases.
	// A missing MarketData.CurrentPrice map is the signal for a bad response.
	if len(resp.MarketData.CurrentPrice) == 0 {
		return nil, fmt.Errorf("coingecko: empty market data for %q", symbol)
	}

	var totalSupply, maxSupply float64
	if resp.MarketData.TotalSupply != nil {
		totalSupply = *resp.MarketData.TotalSupply
	}
	if resp.MarketData.MaxSupply != nil {
		maxSupply = *resp.MarketData.MaxSupply
	}

	// Trim description to 500 chars to keep the JSON payload reasonable.
	desc := resp.Description.En
	if len(desc) > 500 {
		desc = desc[:500] + "…"
	}

	return &FundamentalsData{
		Symbol:            strings.ToUpper(symbol),
		Name:              resp.Name,
		Market:            "coingecko",
		Price:             resp.MarketData.CurrentPrice["usd"],
		MarketCap:         resp.MarketData.MarketCap["usd"],
		MarketCapRank:     resp.MarketCapRank,
		Change24h:         resp.MarketData.PriceChangePercentage24h,
		Volume24h:         resp.MarketData.TotalVolume["usd"],
		CirculatingSupply: resp.MarketData.CirculatingSupply,
		TotalSupply:       totalSupply,
		MaxSupply:         maxSupply,
		ATH:               resp.MarketData.ATH["usd"],
		ATHChangePercent:  resp.MarketData.ATHChangePercentage["usd"],
		Description:       desc,
	}, nil
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add backend/internal/api/fundamentals_handler.go
git commit -m "feat(fundamentals): implement CoinGecko /coins fetch with ID map"
```

---

## Task D4: Add shared HTTP helper, Redis cache, and the Fiber handler method

**Files:**
- Modify: `backend/internal/api/fundamentals_handler.go`

**Step 1: Add the httpGet helper, Redis cache wrappers, and the main `get` handler**

Append the following to `backend/internal/api/fundamentals_handler.go`:

```go
// httpGet performs an authenticated GET with the project User-Agent.
func (h *fundamentalsHandler) httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", fundamentalsUserAgent)
	resp, err := h.httpClient.Do(req)
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

// cacheKey returns the Redis key for a fundamentals result.
func fundamentalsCacheKey(market, symbol string) string {
	return fmt.Sprintf("fundamentals:%s:%s", strings.ToLower(market), strings.ToUpper(symbol))
}

// GET /api/v1/fundamentals/:symbol?market=yahoo|coingecko
func (h *fundamentalsHandler) get(c *fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol is required"})
	}

	market := strings.ToLower(c.Query("market", "yahoo"))
	if market != "yahoo" && market != "coingecko" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "market must be yahoo or coingecko"})
	}

	cacheKey := fundamentalsCacheKey(market, symbol)

	// Try Redis cache first.
	if h.rdb != nil {
		cached, err := h.rdb.Get(c.Context(), cacheKey).Bytes()
		if err == nil {
			// Cache hit — return immediately.
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Send(cached)
		}
		// err == redis.Nil means cache miss — proceed to fetch.
	}

	var (
		data *FundamentalsData
		err  error
	)
	switch market {
	case "coingecko":
		data, err = h.fetchCoingecko(c.Context(), symbol)
	default:
		data, err = h.fetchYahoo(c.Context(), symbol)
	}

	if err != nil {
		log.Printf("[fundamentals] fetch %s/%s error: %v", market, symbol, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to fetch fundamentals data"})
	}

	// Marshal and store in Redis.
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("[fundamentals] marshal error for %s/%s: %v", market, symbol, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
	if h.rdb != nil {
		if setErr := h.rdb.Set(c.Context(), cacheKey, jsonBytes, fundamentalsTTL).Err(); setErr != nil {
			// Cache write failure is non-fatal; log and continue.
			log.Printf("[fundamentals] redis SET error for %s: %v", cacheKey, setErr)
		}
	}

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Send(jsonBytes)
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 3: Smoke-test the compile locally**

```bash
docker compose exec backend go vet ./internal/api/...
```

Expected: no output (no warnings).

**Step 4: Commit**

```bash
git add backend/internal/api/fundamentals_handler.go
git commit -m "feat(fundamentals): add Redis cache, HTTP helper, and Fiber get handler"
```

---

## Task D5: Register the fundamentals route in routes.go

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Instantiate the handler**

In `RegisterRoutes`, after the workspace handler init block (around line 163), add:

```go
// --- Fundamentals (Bloomberg Phase D) ---
fundH := newFundamentalsHandler(rdb)
```

**Step 2: Register the route**

In the `protected` group section, after the Workspaces block, add:

```go
// Fundamentals
protected.Get("/fundamentals/:symbol", fundH.get)
```

**Step 3: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 4: End-to-end smoke test**

Start the stack and request an equity and a crypto symbol:

```bash
make up
# Obtain a JWT token first (log in via the app or use curl /auth/login).
# Then test Yahoo:
curl -s "http://localhost:8080/api/v1/fundamentals/AAPL?market=yahoo" \
  -H "Authorization: Bearer <token>" | jq '{symbol,name,price,pe,eps}'

# Test CoinGecko:
curl -s "http://localhost:8080/api/v1/fundamentals/BTC?market=coingecko" \
  -H "Authorization: Bearer <token>" | jq '{symbol,name,price,market_cap_rank,ath}'
```

Expected (Yahoo): `{"symbol":"AAPL","name":"Apple Inc.","price":<number>,"pe":<number>,"eps":<number>}`
Expected (CoinGecko): `{"symbol":"BTC","name":"Bitcoin","price":<number>,"market_cap_rank":1,"ath":<number>}`

**Step 5: Verify Redis caching**

```bash
# Run the same request twice and confirm the second is served from cache:
docker compose exec redis redis-cli GET "fundamentals:yahoo:AAPL"
```

Expected: the full JSON payload (not nil).

**Step 6: Commit**

```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register GET /api/v1/fundamentals/:symbol route"
```

---

## Task D6: Add FundamentalsData TypeScript type

**Files:**
- Modify: `frontend/src/types/terminal.ts`

**Step 1: Append the interface** to `frontend/src/types/terminal.ts` after the existing `FUNCTION_META` constant:

```typescript
// Fundamentals (Phase D)
export interface FundamentalsData {
  symbol: string
  name: string
  market: 'yahoo' | 'coingecko'
  price: number
  market_cap: number
  // Equity fields (0 for crypto)
  pe: number
  forward_pe: number
  eps: number
  revenue: number
  gross_profit: number
  total_debt: number
  total_cash: number
  dividend_yield: number
  beta: number
  week_52_high: number
  week_52_low: number
  // Crypto fields (0 for equity)
  circulating_supply: number
  total_supply: number
  max_supply: number
  ath: number
  ath_change_percent: number
  market_cap_rank: number
  change_24h: number
  volume_24h: number
  description: string
}
```

**Step 2: TypeScript check**

```bash
docker compose exec frontend npm run type-check 2>&1 | head -20
```

Expected: no new errors related to `FundamentalsData`.

**Step 3: Commit**

```bash
git add frontend/src/types/terminal.ts
git commit -m "feat(types): add FundamentalsData interface for FA widget"
```

---

## Task D7: Create the frontend API module

**Files:**
- Create: `frontend/src/api/fundamentals.ts`

**Step 1: Create the file**

```typescript
import { useQuery } from '@tanstack/react-query'
import apiClient from '@/api/client'
import type { FundamentalsData } from '@/types/terminal'

export async function fetchFundamentals(
  symbol: string,
  market: 'yahoo' | 'coingecko',
): Promise<FundamentalsData> {
  const { data } = await apiClient.get<FundamentalsData>(
    `/api/v1/fundamentals/${encodeURIComponent(symbol)}`,
    { params: { market } },
  )
  return data
}

export function useFundamentals(symbol: string, market: 'yahoo' | 'coingecko') {
  return useQuery({
    queryKey: ['fundamentals', symbol, market],
    queryFn: () => fetchFundamentals(symbol, market),
    enabled: Boolean(symbol),
    staleTime: 5 * 60 * 1000,    // 5 minutes — matches server TTL
    gcTime:    10 * 60 * 1000,   // 10 minutes
    retry: 1,
  })
}
```

**Step 2: TypeScript check**

```bash
docker compose exec frontend npm run type-check 2>&1 | head -20
```

Expected: no errors in `fundamentals.ts`.

**Step 3: Commit**

```bash
git add frontend/src/api/fundamentals.ts
git commit -m "feat(api): add fundamentals fetch + useFundamentals React Query hook"
```

---

## Task D8: Create FundamentalsWidget.tsx

**Files:**
- Create: `frontend/src/components/widgets/FundamentalsWidget.tsx`

**Step 1: Create the widget**

```tsx
import { TrendingUp, TrendingDown, Info } from 'lucide-react'
import { useFundamentals } from '@/api/fundamentals'
import type { FundamentalsData, WidgetProps } from '@/types/terminal'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function fmt(value: number, opts?: Intl.NumberFormatOptions): string {
  if (!value) return '—'
  return new Intl.NumberFormat('en-US', opts).format(value)
}

function fmtPrice(value: number): string {
  return fmt(value, { style: 'currency', currency: 'USD', maximumFractionDigits: 2 })
}

function fmtLarge(value: number): string {
  if (!value) return '—'
  if (value >= 1e12) return `$${(value / 1e12).toFixed(2)}T`
  if (value >= 1e9)  return `$${(value / 1e9).toFixed(2)}B`
  if (value >= 1e6)  return `$${(value / 1e6).toFixed(2)}M`
  return fmtPrice(value)
}

function fmtSupply(value: number): string {
  if (!value) return '—'
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)}B`
  if (value >= 1e6) return `${(value / 1e6).toFixed(2)}M`
  if (value >= 1e3) return `${(value / 1e3).toFixed(2)}K`
  return value.toLocaleString()
}

function fmtPct(value: number): string {
  if (!value && value !== 0) return '—'
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

interface MetricCardProps {
  label: string
  value: string
  highlight?: boolean
  positive?: boolean
}

function MetricCard({ label, value, highlight, positive }: MetricCardProps) {
  return (
    <div className="flex flex-col gap-0.5 rounded-md bg-muted/40 px-3 py-2">
      <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
      <span
        className={[
          'text-sm font-semibold tabular-nums',
          highlight && positive  && 'text-green-400',
          highlight && !positive && 'text-red-400',
          !highlight             && 'text-foreground',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {value}
      </span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Equity section
// ---------------------------------------------------------------------------

function EquityMetrics({ d }: { d: FundamentalsData }) {
  return (
    <>
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Valuation
        </h3>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <MetricCard label="P/E (TTM)"    value={d.pe         ? d.pe.toFixed(2)         : '—'} />
          <MetricCard label="Forward P/E"  value={d.forward_pe ? d.forward_pe.toFixed(2) : '—'} />
          <MetricCard label="EPS (TTM)"    value={d.eps        ? fmtPrice(d.eps)          : '—'} />
          <MetricCard label="Market Cap"   value={fmtLarge(d.market_cap)} />
          <MetricCard label="Revenue"      value={fmtLarge(d.revenue)} />
          <MetricCard label="Gross Profit" value={fmtLarge(d.gross_profit)} />
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Balance Sheet
        </h3>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <MetricCard label="Total Cash" value={fmtLarge(d.total_cash)} />
          <MetricCard label="Total Debt" value={fmtLarge(d.total_debt)} />
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Market Data
        </h3>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <MetricCard label="52W High"       value={fmtPrice(d.week_52_high)} />
          <MetricCard label="52W Low"        value={fmtPrice(d.week_52_low)} />
          <MetricCard label="Beta"           value={d.beta           ? d.beta.toFixed(2)           : '—'} />
          <MetricCard label="Dividend Yield" value={d.dividend_yield ? fmtPct(d.dividend_yield * 100) : '—'} />
        </div>
      </section>
    </>
  )
}

// ---------------------------------------------------------------------------
// Crypto section
// ---------------------------------------------------------------------------

function CryptoMetrics({ d }: { d: FundamentalsData }) {
  const positive24h = d.change_24h >= 0
  return (
    <>
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Market
        </h3>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <MetricCard label="Market Cap"  value={fmtLarge(d.market_cap)} />
          <MetricCard label="CMC Rank"    value={d.market_cap_rank ? `#${d.market_cap_rank}` : '—'} />
          <MetricCard label="Volume 24h"  value={fmtLarge(d.volume_24h)} />
          <MetricCard
            label="Change 24h"
            value={fmtPct(d.change_24h)}
            highlight
            positive={positive24h}
          />
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Supply
        </h3>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <MetricCard label="Circulating" value={fmtSupply(d.circulating_supply)} />
          <MetricCard label="Total"       value={fmtSupply(d.total_supply)} />
          <MetricCard label="Max"         value={fmtSupply(d.max_supply)} />
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          All-Time High
        </h3>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <MetricCard label="ATH Price"      value={fmtPrice(d.ath)} />
          <MetricCard
            label="From ATH"
            value={fmtPct(d.ath_change_percent)}
            highlight
            positive={d.ath_change_percent >= 0}
          />
        </div>
      </section>
    </>
  )
}

// ---------------------------------------------------------------------------
// Root widget
// ---------------------------------------------------------------------------

export function FundamentalsWidget({ ticker, market = 'yahoo' }: WidgetProps) {
  const resolvedMarket = (market === 'coingecko' ? 'coingecko' : 'yahoo') as 'yahoo' | 'coingecko'

  const { data, isLoading, isError, error } = useFundamentals(ticker, resolvedMarket)

  if (!ticker) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        Type a ticker in the command bar (e.g. AAPL FA or BTC FA)
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        Loading fundamentals for {ticker}…
      </div>
    )
  }

  if (isError || !data) {
    const msg = error instanceof Error ? error.message : 'Failed to load fundamentals data.'
    return (
      <div className="flex h-full items-center justify-center text-sm text-red-400">
        {msg}
      </div>
    )
  }

  const isCrypto = data.market === 'coingecko'
  const priceChange = isCrypto ? data.change_24h : null

  return (
    <div className="flex h-full flex-col overflow-y-auto p-3 gap-4 text-sm">
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div>
          <div className="flex items-center gap-2">
            <span className="text-lg font-bold text-foreground">{data.symbol}</span>
            {isCrypto && priceChange !== null && (
              priceChange >= 0
                ? <TrendingUp className="h-4 w-4 text-green-400" />
                : <TrendingDown className="h-4 w-4 text-red-400" />
            )}
          </div>
          <p className="text-xs text-muted-foreground">{data.name}</p>
        </div>
        <div className="text-right">
          <p className="text-base font-semibold tabular-nums">{fmtPrice(data.price)}</p>
          {isCrypto && priceChange !== null && (
            <p
              className={[
                'text-xs font-medium',
                priceChange >= 0 ? 'text-green-400' : 'text-red-400',
              ].join(' ')}
            >
              {fmtPct(priceChange)} (24h)
            </p>
          )}
        </div>
      </div>

      {/* Metric sections */}
      {isCrypto ? <CryptoMetrics d={data} /> : <EquityMetrics d={data} />}

      {/* Description */}
      {data.description && (
        <section>
          <h3 className="mb-1.5 flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            <Info className="h-3 w-3" />
            About
          </h3>
          <p className="text-xs leading-relaxed text-muted-foreground">
            {data.description}
          </p>
        </section>
      )}
    </div>
  )
}
```

**Step 2: TypeScript check**

```bash
docker compose exec frontend npm run type-check 2>&1 | head -30
```

Expected: no errors in `FundamentalsWidget.tsx`.

**Step 3: Lint check**

```bash
docker compose exec frontend npm run lint 2>&1 | grep -i "fundamentals"
```

Expected: no lint errors for the new file.

**Step 4: Commit**

```bash
git add frontend/src/components/widgets/FundamentalsWidget.tsx
git commit -m "feat(widgets): implement FundamentalsWidget with equity and crypto views"
```

---

## Task D9: Wire FundamentalsWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import** at the top of the imports block, after the Phase A imports:

```typescript
// Phase D
import { FundamentalsWidget } from '@/components/widgets/FundamentalsWidget'
```

**Step 2: Replace the FA stub entry** in `WIDGET_REGISTRY`:

Find:
```typescript
  FA:   () => <ComingSoon label="Fundamentals (Phase D)" />,
```

Replace with:
```typescript
  FA:   FundamentalsWidget,
```

After the change the relevant section of `WidgetRegistry.tsx` should read:

```typescript
// Phase A: research & market widgets
GP:   ChartWidget,
NEWS: NewsWidget,
PORT: PortfolioWidget,
WL:   WatchlistWidget,

// Read-only views of workbench tools
ALRT: AlertsWidget,
BT:   BacktestWidget,
MON:  MonitorWidget,
AI:   AIChatWidget,

// Phase D
FA:   FundamentalsWidget,

// Phases B, C, E-H: stubbed for now
HM:   () => <ComingSoon label="Market Heatmap (Phase B)" />,
SCR:  () => <ComingSoon label="Screener (Phase C)" />,
CAL:  () => <ComingSoon label="Calendar (Phase E)" />,
OPT:  () => <ComingSoon label="Options Chain (Phase G)" />,
YCRV: () => <ComingSoon label="Yield Curves (Phase F)" />,
RISK: () => <ComingSoon label="Risk Analytics (Phase H)" />,
```

**Step 3: TypeScript check**

```bash
docker compose exec frontend npm run type-check 2>&1 | head -20
```

Expected: no errors.

**Step 4: Full lint pass**

```bash
docker compose exec frontend npm run lint 2>&1 | tail -5
```

Expected: no warnings or errors.

**Step 5: Commit**

```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(registry): wire FundamentalsWidget into FA slot (Phase D)"
```

---

## Task D10: End-to-end integration test

This task verifies the full flow: command bar → backend → Redis → frontend panel.

**Step 1: Bring up the full stack**

```bash
make up
make health
```

Expected: `{"status":"ok","db":"ok","redis":"ok"}`

**Step 2: Test the backend routes directly**

```bash
# Log in and capture token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"<your_email>","password":"<your_password>"}' \
  | jq -r '.access_token')

# Equity test
curl -s "http://localhost:8080/api/v1/fundamentals/MSFT?market=yahoo" \
  -H "Authorization: Bearer $TOKEN" | jq '{symbol,name,price,pe,eps,revenue}'

# Crypto test
curl -s "http://localhost:8080/api/v1/fundamentals/ETH?market=coingecko" \
  -H "Authorization: Bearer $TOKEN" | jq '{symbol,name,price,market_cap_rank,ath,circulating_supply}'
```

Expected MSFT: all numeric fields populated, `market` = `"yahoo"`.
Expected ETH:  all numeric fields populated, `market` = `"coingecko"`.

**Step 3: Confirm Redis caching**

```bash
docker compose exec redis redis-cli TTL "fundamentals:yahoo:MSFT"
```

Expected: a positive integer (e.g. 287), confirming the key was set with TTL.

**Step 4: Bad market param**

```bash
curl -s "http://localhost:8080/api/v1/fundamentals/AAPL?market=binance" \
  -H "Authorization: Bearer $TOKEN"
```

Expected: `{"error":"market must be yahoo or coingecko"}` with HTTP 400.

**Step 5: Unknown crypto symbol**

```bash
curl -s "http://localhost:8080/api/v1/fundamentals/FAKECOIN?market=coingecko" \
  -H "Authorization: Bearer $TOKEN"
```

Expected: `{"error":"failed to fetch fundamentals data"}` with HTTP 502.

**Step 6: Browser smoke-test**

1. Open `http://localhost:5173/terminal`
2. In the command bar type `AAPL FA` and press Enter.
3. Verify: the FA panel loads, shows the AAPL name, price, P/E, EPS, revenue, balance sheet, and the About description.
4. In the command bar type `BTC FA` (set market to `coingecko` if the panel has a market selector, or open a new panel via `Shift+Enter`).
5. Verify: the FA panel loads, shows Bitcoin name, price, 24h change, market cap rank, ATH, supply fields, and About.

**Step 7: Commit**

```bash
git add .
git commit -m "test(fundamentals): end-to-end Phase D integration verified"
```

---

## Phase D Completion Checklist

### Backend
- [ ] `backend/internal/api/fundamentals_handler.go` exists and compiles
- [ ] `FundamentalsData` struct covers all equity and crypto fields
- [ ] `coingeckoIDMap` contains all 20 top coins (BTC through MANA)
- [ ] `fetchYahoo` calls `quoteSummary?modules=financialData,defaultKeyStatistics,summaryDetail,assetProfile`
- [ ] `fetchCoingecko` maps ticker → CoinGecko ID before fetching
- [ ] HTTP client uses a 10s timeout
- [ ] Redis cache key format: `fundamentals:{market}:{SYMBOL}`, TTL 300s
- [ ] Cache hit returns immediately without upstream call
- [ ] Cache write failure is logged but does not return an error to the client
- [ ] Error responses use `fiber.Map{"error": "..."}` — no internal error details leaked
- [ ] Route `GET /api/v1/fundamentals/:symbol` registered under `protected` group in `routes.go`
- [ ] `go build ./...` passes with no errors
- [ ] `go vet ./internal/api/...` passes with no warnings

### Frontend
- [ ] `FundamentalsData` interface added to `frontend/src/types/terminal.ts`
- [ ] `frontend/src/api/fundamentals.ts` exports `fetchFundamentals` and `useFundamentals`
- [ ] `useFundamentals` uses `staleTime: 5 * 60 * 1000` to match server TTL
- [ ] `frontend/src/components/widgets/FundamentalsWidget.tsx` exists
- [ ] Widget renders equity metrics section (P/E, EPS, revenue, balance sheet, beta, 52W range, dividend yield) when `market === 'yahoo'`
- [ ] Widget renders crypto metrics sections (market, supply, ATH) when `market === 'coingecko'`
- [ ] Widget shows loading state while query is in-flight
- [ ] Widget shows error state when query fails (no raw error details shown to user)
- [ ] Widget shows empty/prompt state when `ticker` is empty
- [ ] `WidgetRegistry.tsx` maps `FA` to `FundamentalsWidget` (stub removed)
- [ ] `npm run type-check` passes with no errors
- [ ] `npm run lint` passes with no warnings

### Integration
- [ ] `AAPL FA` in command bar loads equity panel with populated fields
- [ ] `BTC FA` in command bar loads crypto panel with populated fields
- [ ] Second request within 5 minutes is served from Redis cache (verify with `redis-cli TTL`)
- [ ] Invalid market param returns HTTP 400
- [ ] Unknown crypto symbol returns HTTP 502 (not 500 or a Go panic)
- [ ] `make backend-test` passes (no regressions)
