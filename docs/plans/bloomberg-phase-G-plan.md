# Bloomberg Terminal — Phase G: Options Chain (OPT)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add the `OPT` (Options Chain) widget to the Bloomberg Terminal workspace. After this phase, typing `AAPL OPT` in the command bar loads a side-by-side put/call chain for any US equity — with expiration date selector, bid/ask, implied volatility, volume, open interest, and ATM/ITM row highlighting. Data is fetched from the Yahoo Finance v7 options API.

**Architecture:** Two new backend routes proxy Yahoo Finance and return typed JSON. A new Go handler (`options_handler.go`) fetches and normalises the Yahoo response. On the frontend, a new `OptionsWidget.tsx` renders the chain using an expiry dropdown + a three-column table (Calls | Strike | Puts). No Redux changes needed — the widget uses local React state for the selected expiry and a React Query `useQuery` for each fetch. `WidgetRegistry.tsx` is updated to replace the stub with the real component.

**Note on Greeks:** Yahoo Finance v7 does not return delta, gamma, theta, or vega. Phase G displays only the fields Yahoo provides: bid, ask, implied volatility, volume, and open interest. Greeks can be added in a future phase by implementing a Black-Scholes pricing engine server-side and injecting the values into the `OptionContract` struct.

**Tech Stack:** Go `net/http` (Yahoo Finance v7), Fiber v2 handler, React Query `useQuery`, Tailwind CSS, `lucide-react`

---

## Task G1: Add Go types for options data

**Files:**
- Modify: `backend/internal/api/options_handler.go` *(create new)*

**Step 1: Create the handler file with types only**

Create `backend/internal/api/options_handler.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

// OptionContract represents a single call or put contract as returned by Yahoo Finance.
// Greeks (delta, gamma, theta, vega) are intentionally omitted — Yahoo v7 does not
// provide them. They can be added in a future phase via Black-Scholes calculation.
type OptionContract struct {
	Strike            float64 `json:"strike"`
	LastPrice         float64 `json:"last_price"`
	Bid               float64 `json:"bid"`
	Ask               float64 `json:"ask"`
	Change            float64 `json:"change"`
	PercentChange     float64 `json:"percent_change"`
	Volume            int     `json:"volume"`
	OpenInterest      int     `json:"open_interest"`
	ImpliedVolatility float64 `json:"implied_volatility"`
	InTheMoney        bool    `json:"in_the_money"`
	ContractSymbol    string  `json:"contract_symbol"`
	Expiration        int64   `json:"expiration"` // Unix timestamp
}

// OptionsChain is the top-level response for GET /api/v1/options/:symbol
type OptionsChain struct {
	Symbol          string           `json:"symbol"`
	Expiry          string           `json:"expiry"` // YYYY-MM-DD
	UnderlyingPrice float64          `json:"underlying_price"`
	Calls           []OptionContract `json:"calls"`
	Puts            []OptionContract `json:"puts"`
}

// ExpirationsResponse is the top-level response for GET /api/v1/options/:symbol/expirations
type ExpirationsResponse struct {
	Symbol      string   `json:"symbol"`
	Expirations []string `json:"expirations"` // YYYY-MM-DD strings, sorted ascending
}

// yahooOptionsResponse mirrors the relevant parts of the Yahoo Finance v7 JSON structure.
// Only the fields we parse are included; the rest are ignored.
type yahooOptionsResponse struct {
	OptionChain struct {
		Result []struct {
			UnderlyingSymbol string  `json:"underlyingSymbol"`
			ExpirationDates  []int64 `json:"expirationDates"` // Unix timestamps
			Quote            struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"quote"`
			Options []struct {
				ExpirationDate int64 `json:"expirationDate"`
				Calls          []struct {
					ContractSymbol     string  `json:"contractSymbol"`
					Strike             float64 `json:"strike"`
					Currency           string  `json:"currency"`
					LastPrice          float64 `json:"lastPrice"`
					Change             float64 `json:"change"`
					PercentChange      float64 `json:"percentChange"`
					Volume             int     `json:"volume"`
					OpenInterest       int     `json:"openInterest"`
					Bid                float64 `json:"bid"`
					Ask                float64 `json:"ask"`
					ImpliedVolatility  float64 `json:"impliedVolatility"`
					InTheMoney         bool    `json:"inTheMoney"`
					Expiration         int64   `json:"expiration"`
				} `json:"calls"`
				Puts []struct {
					ContractSymbol     string  `json:"contractSymbol"`
					Strike             float64 `json:"strike"`
					Currency           string  `json:"currency"`
					LastPrice          float64 `json:"lastPrice"`
					Change             float64 `json:"change"`
					PercentChange      float64 `json:"percentChange"`
					Volume             int     `json:"volume"`
					OpenInterest       int     `json:"openInterest"`
					Bid                float64 `json:"bid"`
					Ask                float64 `json:"ask"`
					ImpliedVolatility  float64 `json:"impliedVolatility"`
					InTheMoney         bool    `json:"inTheMoney"`
					Expiration         int64   `json:"expiration"`
				} `json:"puts"`
			} `json:"options"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"optionChain"`
}

// yahooHTTPClient is a package-level client reused across requests.
// Timeout is 10 s; Yahoo Finance requires a browser-like User-Agent.
var yahooHTTPClient = &http.Client{Timeout: 10 * time.Second}

// fetchYahooOptions calls Yahoo Finance v7 and returns the raw parsed response.
// Pass expiry = 0 to fetch the default (nearest) expiry plus the full expirations list.
// Pass expiry > 0 (Unix timestamp) to fetch a specific expiry's chain.
func fetchYahooOptions(symbol string, expiry int64) (*yahooOptionsResponse, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/options/%s", symbol)
	if expiry > 0 {
		url = fmt.Sprintf("%s?date=%d", url, expiry)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// Yahoo Finance returns 401/403 without a realistic User-Agent.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := yahooHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var data yahooOptionsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(data.OptionChain.Result) == 0 {
		return nil, fmt.Errorf("no option chain data for symbol %s", symbol)
	}

	return &data, nil
}

// unixToDate converts a Unix timestamp to a YYYY-MM-DD string in UTC.
func unixToDate(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02")
}

// GetOptionsExpirations handles GET /api/v1/options/:symbol/expirations
// It fetches the default Yahoo chain (no expiry param) solely to extract the
// list of available expiration dates and returns them as YYYY-MM-DD strings.
func GetOptionsExpirations(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol is required"})
	}

	data, err := fetchYahooOptions(symbol, 0)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to fetch options data: " + err.Error()})
	}

	result := data.OptionChain.Result[0]

	dates := make([]string, 0, len(result.ExpirationDates))
	for _, ts := range result.ExpirationDates {
		dates = append(dates, unixToDate(ts))
	}

	return c.JSON(ExpirationsResponse{
		Symbol:      symbol,
		Expirations: dates,
	})
}

// GetOptionsChain handles GET /api/v1/options/:symbol?expiry=YYYY-MM-DD
// If no expiry query param is given, returns the nearest-expiry chain.
// If expiry is provided, parses it to a Unix timestamp and fetches that specific chain.
func GetOptionsChain(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol is required"})
	}

	var expiryUnix int64
	expiryParam := c.Query("expiry") // YYYY-MM-DD or empty
	if expiryParam != "" {
		t, err := time.Parse("2006-01-02", expiryParam)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "expiry must be in YYYY-MM-DD format"})
		}
		expiryUnix = t.Unix()
	}

	data, err := fetchYahooOptions(symbol, expiryUnix)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to fetch options data: " + err.Error()})
	}

	result := data.OptionChain.Result[0]

	if len(result.Options) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no options found for the requested expiry"})
	}

	chain := result.Options[0]

	// Determine the expiry label for the response.
	// Use the requested date if provided; otherwise derive from the chain data.
	expiryLabel := expiryParam
	if expiryLabel == "" && chain.ExpirationDate > 0 {
		expiryLabel = unixToDate(chain.ExpirationDate)
	}

	// Map Yahoo calls to our typed OptionContract slice.
	calls := make([]OptionContract, 0, len(chain.Calls))
	for _, c := range chain.Calls {
		calls = append(calls, OptionContract{
			Strike:            c.Strike,
			LastPrice:         c.LastPrice,
			Bid:               c.Bid,
			Ask:               c.Ask,
			Change:            c.Change,
			PercentChange:     c.PercentChange,
			Volume:            c.Volume,
			OpenInterest:      c.OpenInterest,
			ImpliedVolatility: c.ImpliedVolatility,
			InTheMoney:        c.InTheMoney,
			ContractSymbol:    c.ContractSymbol,
			Expiration:        c.Expiration,
		})
	}

	// Map Yahoo puts to our typed OptionContract slice.
	puts := make([]OptionContract, 0, len(chain.Puts))
	for _, p := range chain.Puts {
		puts = append(puts, OptionContract{
			Strike:            p.Strike,
			LastPrice:         p.LastPrice,
			Bid:               p.Bid,
			Ask:               p.Ask,
			Change:            p.Change,
			PercentChange:     p.PercentChange,
			Volume:            p.Volume,
			OpenInterest:      p.OpenInterest,
			ImpliedVolatility: p.ImpliedVolatility,
			InTheMoney:        p.InTheMoney,
			ContractSymbol:    p.ContractSymbol,
			Expiration:        p.Expiration,
		})
	}

	return c.JSON(OptionsChain{
		Symbol:          symbol,
		Expiry:          expiryLabel,
		UnderlyingPrice: result.Quote.RegularMarketPrice,
		Calls:           calls,
		Puts:            puts,
	})
}
```

**Verification:**

```bash
# Confirm the file compiles with no errors (run from repo root)
docker compose exec backend go build ./internal/api/...
```

Expected: no output (clean compile).

**Commit:**
```bash
git add backend/internal/api/options_handler.go
git commit -m "feat(backend): add options handler types and Yahoo Finance fetch logic"
```

---

## Task G2: Register options routes in routes.go

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Locate the protected route group**

Open `backend/internal/api/routes.go` and find the block where `protected` group routes are registered (e.g. near the existing `GET /api/v1/fundamentals` or `GET /api/v1/yield-curves` registrations from previous phases).

**Step 2: Add the two options routes**

Inside the `protected` group, add the following two lines. The expirations route must be registered **before** the chain route to prevent Fiber from matching `/expirations` as the `expiry` query on the chain route:

```go
// Options Chain (Phase G)
protected.Get("/options/:symbol/expirations", GetOptionsExpirations)
protected.Get("/options/:symbol", GetOptionsChain)
```

The full block in context:

```go
// ...existing routes...

// Options Chain (Phase G)
protected.Get("/options/:symbol/expirations", GetOptionsExpirations)
protected.Get("/options/:symbol", GetOptionsChain)
```

**Verification:**

```bash
# Rebuild and confirm routes are registered
docker compose exec backend go build ./...

# With services running, verify route responds (substitute a valid JWT if auth is enforced)
curl -s "http://localhost:8080/api/v1/options/AAPL/expirations" | jq '.expirations[:3]'
```

Expected output (dates will vary):
```json
[
  "2024-01-19",
  "2024-01-26",
  "2024-02-02"
]
```

```bash
curl -s "http://localhost:8080/api/v1/options/AAPL" | jq '{symbol,expiry,underlying_price,call_count: (.calls | length),put_count: (.puts | length)}'
```

Expected output (values will vary):
```json
{
  "symbol": "AAPL",
  "expiry": "2024-01-19",
  "underlying_price": 185.92,
  "call_count": 47,
  "put_count": 47
}
```

**Commit:**
```bash
git add backend/internal/api/routes.go
git commit -m "feat(backend): register GET /options/:symbol and /options/:symbol/expirations routes"
```

---

## Task G3: Add TypeScript types for options

**Files:**
- Modify: `frontend/src/types/terminal.ts`

**Step 1: Append options types**

Open `frontend/src/types/terminal.ts` and add the following type definitions at the end of the file (before any closing braces, or simply appended to the file):

```typescript
// ---------------------------------------------------------------------------
// Phase G — Options Chain (OPT)
// ---------------------------------------------------------------------------

/**
 * A single call or put option contract as returned by the backend.
 * Greeks are intentionally absent — Yahoo Finance v7 does not return them.
 * They will be added in a future phase via server-side Black-Scholes calculation.
 */
export interface OptionContract {
  strike: number
  last_price: number
  bid: number
  ask: number
  change: number
  percent_change: number
  volume: number
  open_interest: number
  implied_volatility: number
  in_the_money: boolean
  contract_symbol: string
  expiration: number // Unix timestamp
}

/** Full chain for one expiry date. */
export interface OptionsChain {
  symbol: string
  expiry: string // YYYY-MM-DD
  underlying_price: number
  calls: OptionContract[]
  puts: OptionContract[]
}

/** Response from GET /api/v1/options/:symbol/expirations */
export interface ExpirationsResponse {
  symbol: string
  expirations: string[] // YYYY-MM-DD strings, sorted ascending
}
```

**Verification:**

```bash
# TypeScript compilation check
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Commit:**
```bash
git add frontend/src/types/terminal.ts
git commit -m "feat(frontend): add OptionContract, OptionsChain, ExpirationsResponse types"
```

---

## Task G4: Create the options API client

**Files:**
- Create: `frontend/src/api/options.ts`

**Step 1: Create the file**

```typescript
import { useQuery } from '@tanstack/react-query'
import client from './client'
import type { OptionsChain, ExpirationsResponse } from '@/types/terminal'

// ---------------------------------------------------------------------------
// Raw fetch functions (called by React Query hooks below)
// ---------------------------------------------------------------------------

/**
 * Fetch available expiration dates for a symbol.
 * Calls GET /api/v1/options/:symbol/expirations
 */
async function fetchExpirations(symbol: string): Promise<ExpirationsResponse> {
  const { data } = await client.get<ExpirationsResponse>(
    `/api/v1/options/${encodeURIComponent(symbol)}/expirations`
  )
  return data
}

/**
 * Fetch the full options chain for a symbol + expiry.
 * Calls GET /api/v1/options/:symbol?expiry=YYYY-MM-DD
 * If expiry is undefined or empty, backend returns nearest-expiry chain.
 */
async function fetchOptionsChain(
  symbol: string,
  expiry?: string
): Promise<OptionsChain> {
  const params = expiry ? { expiry } : {}
  const { data } = await client.get<OptionsChain>(
    `/api/v1/options/${encodeURIComponent(symbol)}`,
    { params }
  )
  return data
}

// ---------------------------------------------------------------------------
// React Query hooks
// ---------------------------------------------------------------------------

/**
 * Hook: list of available expiration dates.
 * Refetches every 5 minutes; expiry dates change rarely during a trading day.
 */
export function useOptionsExpirations(symbol: string) {
  return useQuery<ExpirationsResponse, Error>({
    queryKey: ['options-expirations', symbol],
    queryFn: () => fetchExpirations(symbol),
    enabled: Boolean(symbol),
    staleTime: 5 * 60 * 1000,  // 5 minutes
    refetchInterval: 5 * 60 * 1000,
  })
}

/**
 * Hook: full options chain for one expiry.
 * Refetches every 30 seconds — options quotes change frequently.
 */
export function useOptionsChain(symbol: string, expiry?: string) {
  return useQuery<OptionsChain, Error>({
    queryKey: ['options-chain', symbol, expiry ?? 'default'],
    queryFn: () => fetchOptionsChain(symbol, expiry),
    enabled: Boolean(symbol),
    staleTime: 30 * 1000,       // 30 seconds
    refetchInterval: 30 * 1000,
  })
}
```

**Verification:**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Commit:**
```bash
git add frontend/src/api/options.ts
git commit -m "feat(frontend): add useOptionsExpirations and useOptionsChain React Query hooks"
```

---

## Task G5: Build the OptionsWidget component

**Files:**
- Modify: `frontend/src/components/widgets/OptionsWidget.tsx` *(replace existing stub)*

**Step 1: Replace the stub with the full component**

Open `frontend/src/components/widgets/OptionsWidget.tsx` and replace its entire contents with:

```tsx
import { useState } from 'react'
import { AlertCircle, Loader2 } from 'lucide-react'
import { useOptionsExpirations, useOptionsChain } from '@/api/options'
import type { WidgetProps } from '@/types/terminal'
import type { OptionContract } from '@/types/terminal'

// ---------------------------------------------------------------------------
// Helper: format a number to N decimal places, return '—' for zero/undefined
// ---------------------------------------------------------------------------
function fmt(value: number | undefined, decimals = 2): string {
  if (value === undefined || value === null || value === 0) return '—'
  return value.toFixed(decimals)
}

// ---------------------------------------------------------------------------
// Helper: format implied volatility (Yahoo returns it as a decimal, e.g. 0.35)
// ---------------------------------------------------------------------------
function fmtIV(iv: number | undefined): string {
  if (iv === undefined || iv === null || iv === 0) return '—'
  return (iv * 100).toFixed(1) + '%'
}

// ---------------------------------------------------------------------------
// Helper: format large integers (volume, OI) with comma separators
// ---------------------------------------------------------------------------
function fmtInt(n: number | undefined): string {
  if (n === undefined || n === null || n === 0) return '—'
  return n.toLocaleString()
}

// ---------------------------------------------------------------------------
// Column header row (shared by calls and puts sides)
// ---------------------------------------------------------------------------
function ChainHeader({ side }: { side: 'calls' | 'puts' }) {
  // Calls: columns read left-to-right toward the strike.
  // Puts:  columns read left-to-right away from the strike.
  const cols =
    side === 'calls'
      ? ['Bid', 'Ask', 'IV', 'Vol', 'OI']
      : ['Bid', 'Ask', 'IV', 'Vol', 'OI']

  return (
    <div className="grid grid-cols-5 gap-x-1 px-1 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
      {cols.map((col) => (
        <span key={col} className="text-right">
          {col}
        </span>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// A single call row (5 cells)
// ---------------------------------------------------------------------------
function CallRow({
  contract,
  isAtm,
}: {
  contract: OptionContract
  isAtm: boolean
}) {
  const itm = contract.in_the_money
  return (
    <div
      className={[
        'grid grid-cols-5 gap-x-1 px-1 py-[3px] text-[11px] tabular-nums',
        itm
          ? 'bg-emerald-950/30 dark:bg-emerald-900/20'
          : 'hover:bg-neutral-800/40',
        isAtm ? 'ring-1 ring-inset ring-yellow-500/50' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <span className="text-right text-neutral-300">{fmt(contract.bid)}</span>
      <span className="text-right text-neutral-300">{fmt(contract.ask)}</span>
      <span className="text-right text-sky-400">{fmtIV(contract.implied_volatility)}</span>
      <span className="text-right text-neutral-400">{fmtInt(contract.volume)}</span>
      <span className="text-right text-neutral-400">{fmtInt(contract.open_interest)}</span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// A single put row (5 cells)
// ---------------------------------------------------------------------------
function PutRow({
  contract,
  isAtm,
}: {
  contract: OptionContract
  isAtm: boolean
}) {
  const itm = contract.in_the_money
  return (
    <div
      className={[
        'grid grid-cols-5 gap-x-1 px-1 py-[3px] text-[11px] tabular-nums',
        itm
          ? 'bg-rose-950/30 dark:bg-rose-900/20'
          : 'hover:bg-neutral-800/40',
        isAtm ? 'ring-1 ring-inset ring-yellow-500/50' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <span className="text-right text-neutral-300">{fmt(contract.bid)}</span>
      <span className="text-right text-neutral-300">{fmt(contract.ask)}</span>
      <span className="text-right text-sky-400">{fmtIV(contract.implied_volatility)}</span>
      <span className="text-right text-neutral-400">{fmtInt(contract.volume)}</span>
      <span className="text-right text-neutral-400">{fmtInt(contract.open_interest)}</span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Strike cell (center column)
// ---------------------------------------------------------------------------
function StrikeCell({
  strike,
  isAtm,
}: {
  strike: number
  isAtm: boolean
}) {
  return (
    <div
      className={[
        'flex items-center justify-center px-1 py-[3px] text-[11px] font-semibold tabular-nums',
        isAtm
          ? 'bg-yellow-500/20 text-yellow-300'
          : 'bg-neutral-900/60 text-neutral-300',
      ].join(' ')}
    >
      {strike.toFixed(2)}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main widget
// ---------------------------------------------------------------------------
export function OptionsWidget({ ticker }: WidgetProps) {
  const symbol = ticker.toUpperCase()

  // Expiry state — undefined means "use backend default (nearest expiry)"
  const [selectedExpiry, setSelectedExpiry] = useState<string | undefined>(undefined)

  const {
    data: expirationsData,
    isLoading: expLoading,
    isError: expError,
  } = useOptionsExpirations(symbol)

  const {
    data: chain,
    isLoading: chainLoading,
    isError: chainError,
    error: chainErr,
  } = useOptionsChain(symbol, selectedExpiry)

  // Build a lookup map of puts by strike for quick pairing with calls
  const putsByStrike = new Map<number, OptionContract>()
  chain?.puts.forEach((p) => putsByStrike.set(p.strike, p))

  // Determine ATM strike = strike closest to underlying price
  const underlyingPrice = chain?.underlying_price ?? 0
  let atmStrike = 0
  if (chain && underlyingPrice > 0) {
    let minDiff = Infinity
    chain.calls.forEach((c) => {
      const diff = Math.abs(c.strike - underlyingPrice)
      if (diff < minDiff) {
        minDiff = diff
        atmStrike = c.strike
      }
    })
  }

  // ---------------------------------------------------------------------------
  // Loading state
  // ---------------------------------------------------------------------------
  if (expLoading || chainLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-neutral-500" />
        <span className="ml-2 text-sm text-neutral-500">Loading options…</span>
      </div>
    )
  }

  // ---------------------------------------------------------------------------
  // Error state
  // ---------------------------------------------------------------------------
  if (expError || chainError) {
    const msg =
      chainErr instanceof Error
        ? chainErr.message
        : 'Failed to load options data'
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 p-4">
        <AlertCircle className="h-6 w-6 text-red-500" />
        <p className="text-sm text-red-400">{msg}</p>
        <p className="text-xs text-neutral-500">
          Options are only available for US equities.
        </p>
      </div>
    )
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------
  return (
    <div className="flex h-full flex-col overflow-hidden bg-neutral-950 text-neutral-200">
      {/* ── Top bar: underlying price + expiry selector ── */}
      <div className="flex flex-shrink-0 items-center justify-between border-b border-neutral-800 px-3 py-2">
        <div className="flex items-center gap-3">
          <span className="text-xs font-semibold uppercase tracking-widest text-neutral-400">
            {symbol}
          </span>
          {underlyingPrice > 0 && (
            <span className="text-sm font-semibold text-neutral-100">
              ${underlyingPrice.toFixed(2)}
            </span>
          )}
          <span className="text-[10px] text-neutral-600">underlying</span>
        </div>

        {/* Expiry dropdown */}
        <div className="flex items-center gap-2">
          <label
            htmlFor="opt-expiry"
            className="text-[10px] uppercase tracking-wide text-neutral-500"
          >
            Expiry
          </label>
          <select
            id="opt-expiry"
            value={selectedExpiry ?? chain?.expiry ?? ''}
            onChange={(e) => setSelectedExpiry(e.target.value || undefined)}
            className="rounded border border-neutral-700 bg-neutral-900 px-2 py-0.5 text-xs text-neutral-200 focus:border-sky-500 focus:outline-none"
          >
            {expirationsData?.expirations.map((date) => (
              <option key={date} value={date}>
                {date}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* ── Greeks notice ── */}
      <div className="flex-shrink-0 border-b border-neutral-800/60 bg-neutral-900/40 px-3 py-1 text-[10px] text-neutral-600">
        Greeks (delta, gamma, theta, vega) will be added in a future phase via Black-Scholes calculation.
        ITM calls highlighted green · ITM puts highlighted red · ATM strike highlighted yellow
      </div>

      {/* ── Chain header ── */}
      <div className="flex-shrink-0 border-b border-neutral-800">
        <div className="grid grid-cols-[1fr_80px_1fr]">
          {/* Calls side header */}
          <div className="border-r border-neutral-800">
            <div className="px-1 py-0.5 text-center text-[10px] font-bold uppercase tracking-widest text-emerald-500">
              Calls
            </div>
            <ChainHeader side="calls" />
          </div>

          {/* Strike header */}
          <div className="flex items-end justify-center pb-1 text-[10px] font-semibold uppercase tracking-wide text-neutral-500">
            Strike
          </div>

          {/* Puts side header */}
          <div className="border-l border-neutral-800">
            <div className="px-1 py-0.5 text-center text-[10px] font-bold uppercase tracking-widest text-rose-500">
              Puts
            </div>
            <ChainHeader side="puts" />
          </div>
        </div>
      </div>

      {/* ── Chain rows (scrollable) ── */}
      <div className="flex-1 overflow-y-auto">
        {chain && chain.calls.length === 0 && (
          <div className="py-8 text-center text-sm text-neutral-600">
            No contracts found for this expiry.
          </div>
        )}

        {chain?.calls.map((call) => {
          const put = putsByStrike.get(call.strike)
          const isAtm = call.strike === atmStrike

          return (
            <div
              key={call.contract_symbol}
              className="grid grid-cols-[1fr_80px_1fr] border-b border-neutral-800/50"
            >
              {/* Call side */}
              <div className="border-r border-neutral-800/50">
                <CallRow contract={call} isAtm={isAtm} />
              </div>

              {/* Strike center */}
              <StrikeCell strike={call.strike} isAtm={isAtm} />

              {/* Put side */}
              <div className="border-l border-neutral-800/50">
                {put ? (
                  <PutRow contract={put} isAtm={isAtm} />
                ) : (
                  <div className="px-1 py-[3px] text-center text-[11px] text-neutral-700">
                    —
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>

      {/* ── Footer: contract count ── */}
      {chain && (
        <div className="flex-shrink-0 border-t border-neutral-800 px-3 py-1 text-[10px] text-neutral-600">
          {chain.calls.length} calls · {chain.puts.length} puts · expiry{' '}
          {chain.expiry}
        </div>
      )}
    </div>
  )
}
```

**Verification:**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no TypeScript errors.

```bash
# Confirm ESLint passes
docker compose exec frontend npx eslint src/components/widgets/OptionsWidget.tsx --max-warnings 0
```

Expected: no warnings.

**Commit:**
```bash
git add frontend/src/components/widgets/OptionsWidget.tsx
git commit -m "feat(frontend): implement OptionsWidget with expiry selector and ATM/ITM highlighting"
```

---

## Task G6: Wire OptionsWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import**

Find the existing imports section in `WidgetRegistry.tsx` and add:

```tsx
import { OptionsWidget } from '@/components/widgets/OptionsWidget'
```

**Step 2: Register the widget**

Find the registry object/map where function codes map to components. It will look similar to:

```tsx
const registry: Record<FunctionCode, React.ComponentType<WidgetProps>> = {
  GP:   ChartWidget,
  HM:   HeatmapWidget,
  // ...
  OPT:  OptionsStub,   // ← replace this line
  // ...
}
```

Replace the stub entry:

```tsx
OPT: OptionsWidget,
```

**Verification:**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

Open the terminal at `http://localhost:5173/terminal`, type `AAPL OPT` in the command bar, and press Enter. The active panel should load the OptionsWidget.

**Commit:**
```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(frontend): wire OptionsWidget into WidgetRegistry for OPT function code"
```

---

## Task G7: End-to-end smoke test

This task has no new code. Its purpose is to verify the full stack path from command bar to rendered chain.

**Step 1: Verify backend routes**

```bash
# Health check first
curl -s http://localhost:8080/health | jq .

# Expirations — should return list of YYYY-MM-DD strings
curl -s "http://localhost:8080/api/v1/options/AAPL/expirations" | jq '{symbol, count: (.expirations | length), first: .expirations[0], last: .expirations[-1]}'

# Default (nearest) chain
curl -s "http://localhost:8080/api/v1/options/AAPL" | jq '{symbol, expiry, underlying_price, calls: (.calls | length), puts: (.puts | length)}'

# Specific expiry chain (substitute a date from the expirations response)
curl -s "http://localhost:8080/api/v1/options/AAPL?expiry=2024-03-15" | jq '{symbol, expiry, calls: (.calls | length)}'

# Error case — invalid symbol
curl -s "http://localhost:8080/api/v1/options/INVALID_SYM_XYZ" | jq .
```

Expected for invalid symbol:
```json
{ "error": "failed to fetch options data: no option chain data for symbol INVALID_SYM_XYZ" }
```

**Step 2: Verify frontend renders correctly**

1. Open `http://localhost:5173/terminal`
2. In the command bar enter `AAPL OPT` and press Enter
3. Check:
   - Underlying price is displayed in the top bar
   - Expiry dropdown is populated with multiple dates
   - Three-column table renders (Calls | Strike | Puts)
   - ITM call rows have a green tint
   - ITM put rows have a red tint
   - ATM strike row has a yellow border/highlight
   - Footer shows contract counts and expiry date
   - Changing the expiry dropdown triggers a new fetch and re-renders the table
4. Open browser DevTools Network tab. Confirm:
   - One request to `/api/v1/options/AAPL/expirations`
   - One request to `/api/v1/options/AAPL` (or with `?expiry=...` after changing dropdown)
   - No 4xx/5xx responses

**Step 3: Test dark/light mode**

Toggle the theme in the top bar. Confirm the options table remains readable in both modes (Tailwind `dark:` variants apply correctly).

**Step 4: Test with a crypto or commodity ticker**

```bash
curl -s "http://localhost:8080/api/v1/options/BTC-USD" | jq .
```

Expected: an error response indicating no chain data (crypto has no options on Yahoo). The frontend widget should show the error state with the message "Options are only available for US equities."

**Commit:**
```bash
git commit --allow-empty -m "test(phase-g): end-to-end options chain smoke test passed"
```

---

## Phase G Completion Checklist

Work through this checklist in order before marking Phase G as done.

### Backend
- [ ] `backend/internal/api/options_handler.go` created with `OptionContract`, `OptionsChain`, `ExpirationsResponse`, `yahooOptionsResponse` structs
- [ ] `fetchYahooOptions` sets `User-Agent: Mozilla/5.0` header (Yahoo requires it)
- [ ] `fetchYahooOptions` uses a 10 s `http.Client` timeout
- [ ] `GetOptionsExpirations` returns `ExpirationsResponse` with YYYY-MM-DD strings
- [ ] `GetOptionsChain` handles missing `expiry` param (falls back to nearest expiry)
- [ ] `GetOptionsChain` returns `400` for malformed `expiry` format
- [ ] `GetOptionsChain` returns `502` (Bad Gateway) for Yahoo fetch failures — never leaks raw error detail beyond the message
- [ ] Routes registered in `routes.go`: `GET /options/:symbol/expirations` before `GET /options/:symbol`
- [ ] `go build ./...` passes with no errors
- [ ] `curl /api/v1/options/AAPL/expirations` returns an array of date strings
- [ ] `curl /api/v1/options/AAPL` returns chain with `calls` and `puts` arrays
- [ ] `curl /api/v1/options/INVALID_SYM_XYZ` returns a JSON error (not a panic / 500)

### Frontend
- [ ] `frontend/src/types/terminal.ts` has `OptionContract`, `OptionsChain`, `ExpirationsResponse` interfaces
- [ ] `frontend/src/api/options.ts` has `useOptionsExpirations` (5 min stale) and `useOptionsChain` (30 s stale) hooks
- [ ] `OptionsWidget.tsx` accepts `WidgetProps` (ticker, market, timeframe, params)
- [ ] Expiry dropdown populated from `useOptionsExpirations`
- [ ] Chain table renders three columns: Calls | Strike | Puts
- [ ] Call columns: Bid, Ask, IV, Vol, OI
- [ ] Put columns: Bid, Ask, IV, Vol, OI
- [ ] ATM strike highlighted yellow (nearest strike to `underlying_price`)
- [ ] ITM call rows highlighted green, ITM put rows highlighted red
- [ ] Loading spinner shown while fetching
- [ ] Error state shown for failed fetch (e.g. crypto/commodity ticker)
- [ ] Greeks notice displayed informing user they are a future feature
- [ ] Footer shows call count, put count, expiry date
- [ ] `WidgetRegistry.tsx` maps `OPT` to `OptionsWidget` (not stub)
- [ ] `npx tsc --noEmit` passes
- [ ] `npx eslint src/components/widgets/OptionsWidget.tsx --max-warnings 0` passes

### Integration
- [ ] Typing `AAPL OPT` in command bar loads the widget in the active panel
- [ ] Changing the expiry dropdown refetches and re-renders without full panel remount
- [ ] Theme toggle (dark/light) keeps the chain readable
- [ ] Crypto/commodity ticker (e.g. `BTC-USD OPT`) shows the error state gracefully
- [ ] No console errors in browser DevTools during normal widget use

### Docs
- [ ] Update `docs/plans/bloomberg-reference.md` Phase G status from `⬜ Pending` to `✅ Done`

---

## Future Work (not in Phase G)

- **Greeks via Black-Scholes:** Implement a Go function in `backend/internal/analytics/` that calculates delta, gamma, theta, vega from strike, underlying price, time-to-expiry, risk-free rate, and implied volatility. Inject the values into `OptionContract` before returning from `GetOptionsChain`. FRED API can supply the risk-free rate (3-month T-bill: series `DTB3`).
- **Redis caching:** If Yahoo rate-limits requests at scale, add a 30 s TTL cache key `options:{symbol}:{expiry_unix}` in Redis. The handler should attempt `GET` from Redis first and fall back to Yahoo on cache miss.
- **IV surface chart:** Add a second view mode to `OptionsWidget` that renders a 2-D implied volatility surface (strike × expiry) using `recharts` or `lightweight-charts`.
- **Unusual options activity:** Aggregate high-OI / high-volume contracts and surface them as a summary row at the top of the chain.
