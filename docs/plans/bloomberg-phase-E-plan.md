# Bloomberg Terminal — Phase E: Economic Calendar (CAL)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an economic calendar widget (`CAL`) to the Bloomberg terminal workspace. After this phase, users can open a `CAL` panel showing upcoming earnings dates for individual stocks and macro economic events (Fed FOMC meetings, CPI, GDP, NFP, and other FRED data releases) in a clean date-grouped agenda view.

**Architecture:** Two new backend routes (`/api/v1/calendar/earnings` and `/api/v1/calendar/macro`) fetch from Alpha Vantage (earnings CSV) and FRED API (release calendar), with Redis caching at 1h and 4h TTL respectively. A set of hardcoded FOMC meeting dates supplement FRED data for 2024–2026. The frontend `CalendarWidget.tsx` renders two tabs (Earnings / Macro) with date-grouped scrollable agenda lists. The stub in `WidgetRegistry.tsx` is replaced with the real component.

**Tech Stack:** Go `net/http` (external API calls), `encoding/csv` (Alpha Vantage CSV parsing), `go-redis` (caching), React Query, Tailwind, lucide-react, shadcn/ui Tabs.

**Data sources:**
- Earnings: Alpha Vantage `EARNINGS_CALENDAR` — returns CSV, parsed in Go
- Macro: FRED `releases/dates` API + hardcoded FOMC meeting dates
- Both degrade gracefully (return empty arrays) when API keys are not configured

---

## Task E1: Add API keys to config

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `.env.example`

**Step 1: Add an `ExternalAPIs` config struct** to `config.go`.

Add a new struct after the `CORSConfig` struct:

```go
type ExternalAPIsConfig struct {
	AlphaVantageKey string
	FREDKey         string
}
```

**Step 2: Add the field to the top-level `Config` struct.**

Find:
```go
type Config struct {
	App    AppConfig
	DB     DBConfig
	Redis  RedisConfig
	Worker WorkerConfig
	CORS   CORSConfig
}
```

Replace with:
```go
type Config struct {
	App         AppConfig
	DB          DBConfig
	Redis       RedisConfig
	Worker      WorkerConfig
	CORS        CORSConfig
	ExternalAPIs ExternalAPIsConfig
}
```

**Step 3: Populate the new struct** in the `Load()` function, after the `CORS` block:

```go
ExternalAPIs: ExternalAPIsConfig{
    AlphaVantageKey: getEnv("ALPHA_VANTAGE_API_KEY", ""),
    FREDKey:         getEnv("FRED_API_KEY", ""),
},
```

**Step 4: Add the new keys to `.env.example`** after the `TELEGRAM_CHAT_ID=` line:

```
# ── Bloomberg Calendar (Phase E) ─────────────────────────────────────────────
# Alpha Vantage — earnings calendar (https://www.alphavantage.co/support/#api-key)
ALPHA_VANTAGE_API_KEY=
# FRED — macro economic releases (https://fred.stlouisfed.org/docs/api/api_key.html)
FRED_API_KEY=
```

**Step 5: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 6: Commit**
```bash
git add backend/internal/config/config.go .env.example
git commit -m "feat(config): add ALPHA_VANTAGE_API_KEY and FRED_API_KEY to ExternalAPIs config"
```

---

## Task E2: Create the calendar handler

**Files:**
- Create: `backend/internal/api/calendar_handler.go`

**Step 1: Create the handler file** with the full implementation below.

```go
package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/trader-claude/backend/internal/config"
)

// --- Response types ---

// EarningsEvent represents a single upcoming earnings date from Alpha Vantage.
type EarningsEvent struct {
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	ReportDate   string `json:"report_date"`  // YYYY-MM-DD
	FiscalPeriod string `json:"fiscal_period"`
	EPSEstimate  string `json:"eps_estimate"`
	Currency     string `json:"currency"`
}

// MacroEvent represents a macro economic event (FRED release or FOMC meeting).
type MacroEvent struct {
	Name        string `json:"name"`
	Date        string `json:"date"`        // YYYY-MM-DD
	Category    string `json:"category"`    // "fed" | "inflation" | "employment" | "gdp" | "other"
	Description string `json:"description"`
	Importance  string `json:"importance"`  // "high" | "medium" | "low"
}

// --- Handler ---

type calendarHandler struct {
	rdb  *redis.Client
	cfg  *config.Config
	http *http.Client
}

func newCalendarHandler(rdb *redis.Client, cfg *config.Config) *calendarHandler {
	return &calendarHandler{
		rdb: rdb,
		cfg: cfg,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// GET /api/v1/calendar/earnings?from=2024-01-01&to=2024-01-31&symbol=AAPL
func (h *calendarHandler) earnings(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	symbol := strings.ToUpper(c.Query("symbol"))

	// Default date range: today + 3 months
	if from == "" {
		from = time.Now().Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().AddDate(0, 3, 0).Format("2006-01-02")
	}

	cacheKey := fmt.Sprintf("calendar:earnings:%s:%s:%s", from, to, symbol)
	ctx := context.Background()

	// Check cache
	if cached, err := h.rdb.Get(ctx, cacheKey).Result(); err == nil {
		c.Set("Content-Type", "application/json")
		return c.SendString(cached)
	}

	// If no API key, return empty list gracefully
	if h.cfg.ExternalAPIs.AlphaVantageKey == "" {
		return c.JSON([]EarningsEvent{})
	}

	events, err := h.fetchEarnings(from, to, symbol)
	if err != nil {
		log.Printf("calendar: earnings fetch error: %v", err)
		return c.JSON([]EarningsEvent{})
	}

	// Cache result
	if b, err := json.Marshal(events); err == nil {
		h.rdb.Set(ctx, cacheKey, string(b), time.Hour)
	}

	return c.JSON(events)
}

// fetchEarnings calls Alpha Vantage EARNINGS_CALENDAR, parses the CSV, and
// filters by date range and optional symbol.
func (h *calendarHandler) fetchEarnings(from, to, symbol string) ([]EarningsEvent, error) {
	url := fmt.Sprintf(
		"https://www.alphavantage.co/query?function=EARNINGS_CALENDAR&horizon=3month&apikey=%s",
		h.cfg.ExternalAPIs.AlphaVantageKey,
	)

	resp, err := h.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("alphavantage request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alphavantage returned HTTP %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	// Read header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("csv read header: %w", err)
	}

	// Build column index map for robustness against field reordering
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}

	// Required columns: symbol, name, reportDate, fiscalDateEnding, estimate, currency
	required := []string{"symbol", "name", "reportDate", "fiscalDateEnding", "estimate", "currency"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("alphavantage CSV missing expected column %q", col)
		}
	}

	var events []EarningsEvent
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		reportDate := strings.TrimSpace(row[colIdx["reportDate"]])
		sym := strings.ToUpper(strings.TrimSpace(row[colIdx["symbol"]]))

		// Filter by symbol if provided
		if symbol != "" && sym != symbol {
			continue
		}

		// Filter by date range
		if reportDate < from || reportDate > to {
			continue
		}

		events = append(events, EarningsEvent{
			Symbol:       sym,
			Name:         strings.TrimSpace(row[colIdx["name"]]),
			ReportDate:   reportDate,
			FiscalPeriod: strings.TrimSpace(row[colIdx["fiscalDateEnding"]]),
			EPSEstimate:  strings.TrimSpace(row[colIdx["estimate"]]),
			Currency:     strings.TrimSpace(row[colIdx["currency"]]),
		})
	}

	if events == nil {
		events = []EarningsEvent{}
	}
	return events, nil
}

// GET /api/v1/calendar/macro?from=2024-01-01&to=2024-12-31
func (h *calendarHandler) macro(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")

	// Default: current month through 6 months ahead
	if from == "" {
		from = time.Now().Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().AddDate(0, 6, 0).Format("2006-01-02")
	}

	cacheKey := fmt.Sprintf("calendar:macro:%s:%s", from, to)
	ctx := context.Background()

	// Check cache
	if cached, err := h.rdb.Get(ctx, cacheKey).Result(); err == nil {
		c.Set("Content-Type", "application/json")
		return c.SendString(cached)
	}

	var events []MacroEvent

	// 1. Hardcoded FOMC meetings (always available, regardless of API key)
	events = append(events, fomcEventsInRange(from, to)...)

	// 2. FRED release calendar (optional — degrades gracefully)
	if h.cfg.ExternalAPIs.FREDKey != "" {
		fredEvents, err := h.fetchFREDReleases(from, to)
		if err != nil {
			log.Printf("calendar: FRED fetch error: %v", err)
		} else {
			events = append(events, fredEvents...)
		}
	}

	// Deduplicate by (name + date), sort by date
	events = dedupAndSortMacroEvents(events)

	if events == nil {
		events = []MacroEvent{}
	}

	// Cache for 4 hours
	if b, err := json.Marshal(events); err == nil {
		h.rdb.Set(ctx, cacheKey, string(b), 4*time.Hour)
	}

	return c.JSON(events)
}

// fredReleasesResponse is the top-level FRED API response for releases/dates.
type fredReleasesResponse struct {
	ReleaseDates []struct {
		ReleaseID   int    `json:"release_id"`
		ReleaseName string `json:"release_name"`
		Date        string `json:"date"`
	} `json:"release_dates"`
}

// fetchFREDReleases calls the FRED releases/dates endpoint and maps results to MacroEvent.
func (h *calendarHandler) fetchFREDReleases(from, to string) ([]MacroEvent, error) {
	url := fmt.Sprintf(
		"https://api.stlouisfed.org/fred/releases/dates?realtime_start=%s&realtime_end=%s&api_key=%s&file_type=json&sort_order=asc",
		from, to,
		h.cfg.ExternalAPIs.FREDKey,
	)

	resp, err := h.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("FRED request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FRED returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5 MB cap
	if err != nil {
		return nil, fmt.Errorf("FRED response read: %w", err)
	}

	var parsed fredReleasesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("FRED JSON parse: %w", err)
	}

	var events []MacroEvent
	for _, rd := range parsed.ReleaseDates {
		category, importance := classifyFREDRelease(rd.ReleaseName)
		events = append(events, MacroEvent{
			Name:        rd.ReleaseName,
			Date:        rd.Date,
			Category:    category,
			Description: fmt.Sprintf("FRED data release: %s", rd.ReleaseName),
			Importance:  importance,
		})
	}
	return events, nil
}

// classifyFREDRelease maps a FRED release name to a (category, importance) pair.
func classifyFREDRelease(name string) (string, string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "consumer price index") || strings.Contains(lower, "cpi"):
		return "inflation", "high"
	case strings.Contains(lower, "personal consumption") || strings.Contains(lower, "pce"):
		return "inflation", "high"
	case strings.Contains(lower, "producer price") || strings.Contains(lower, "ppi"):
		return "inflation", "medium"
	case strings.Contains(lower, "nonfarm payroll") || strings.Contains(lower, "employment situation"):
		return "employment", "high"
	case strings.Contains(lower, "unemployment"):
		return "employment", "high"
	case strings.Contains(lower, "initial claims") || strings.Contains(lower, "jobless"):
		return "employment", "medium"
	case strings.Contains(lower, "gross domestic product") || strings.Contains(lower, "gdp"):
		return "gdp", "high"
	case strings.Contains(lower, "retail sales"):
		return "other", "high"
	case strings.Contains(lower, "industrial production"):
		return "other", "medium"
	case strings.Contains(lower, "housing starts") || strings.Contains(lower, "building permits"):
		return "other", "medium"
	case strings.Contains(lower, "ism") || strings.Contains(lower, "manufacturing"):
		return "other", "medium"
	default:
		return "other", "low"
	}
}

// fomcEventsInRange returns hardcoded FOMC meeting dates that fall within [from, to].
// Dates are the statement-release dates (day 2 of each 2-day meeting).
func fomcEventsInRange(from, to string) []MacroEvent {
	allDates := []string{
		// 2024
		"2024-01-31", "2024-03-20", "2024-05-01",
		"2024-06-12", "2024-07-31", "2024-09-18",
		"2024-11-07", "2024-12-18",
		// 2025
		"2025-01-29", "2025-03-19", "2025-05-07",
		"2025-06-18", "2025-07-30", "2025-09-17",
		"2025-11-05", "2025-12-17",
		// 2026
		"2026-01-28", "2026-03-18", "2026-04-29",
		"2026-06-17", "2026-07-29", "2026-09-16",
		"2026-11-04", "2026-12-16",
	}

	var events []MacroEvent
	for _, d := range allDates {
		if d >= from && d <= to {
			events = append(events, MacroEvent{
				Name:        "FOMC Meeting",
				Date:        d,
				Category:    "fed",
				Description: "Federal Open Market Committee interest rate decision and statement",
				Importance:  "high",
			})
		}
	}
	return events
}

// dedupAndSortMacroEvents deduplicates by (name+date) and sorts ascending by date.
func dedupAndSortMacroEvents(events []MacroEvent) []MacroEvent {
	seen := make(map[string]struct{})
	var deduped []MacroEvent
	for _, e := range events {
		key := e.Date + "|" + e.Name
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			deduped = append(deduped, e)
		}
	}

	// Insertion sort is fine for a few hundred events per request
	for i := 1; i < len(deduped); i++ {
		for j := i; j > 0 && deduped[j].Date < deduped[j-1].Date; j-- {
			deduped[j], deduped[j-1] = deduped[j-1], deduped[j]
		}
	}
	return deduped
}
```

**Step 2: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors. If there are import errors, verify the module path is `github.com/trader-claude/backend`.

**Step 3: Commit**

```bash
git add backend/internal/api/calendar_handler.go
git commit -m "feat(api): add calendar handler — earnings (Alpha Vantage) and macro (FRED + FOMC)"
```

---

## Task E3: Register calendar routes

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Update the `RegisterRoutes` function signature** to accept `cfg *config.Config`.

Find the existing signature:
```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager, authSvc *auth.AuthService, corsOrigins string) {
```

Replace with:
```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager, authSvc *auth.AuthService, corsOrigins string, cfg *config.Config) {
```

**Step 2: Add the calendar handler init** just after the workspace handler init block (after `workspaceH := newWorkspaceHandler(db)`):

```go
// --- Calendar (Bloomberg Phase E) ---
calH := newCalendarHandler(rdb, cfg)
```

**Step 3: Register the routes** in the `protected` group, after the workspace routes block:

```go
// --- Calendar ---
protected.Get("/calendar/earnings", calH.earnings)
protected.Get("/calendar/macro", calH.macro)
```

**Step 4: Update the `config` import** at the top of `routes.go`. Add it to the import block if not already present:

```go
"github.com/trader-claude/backend/internal/config"
```

**Step 5: Update the call site in `main.go`** to pass `cfg` as the final argument.

First, find the `RegisterRoutes` call:
```bash
grep -n "RegisterRoutes" backend/cmd/server/main.go
```

Update that call to append `, cfg` as the last argument. For example, if the existing call is:
```go
api.RegisterRoutes(app, db, rdb, hub, cfg.App.Version, pool, ds, replayMgr, monMgr, authSvc, cfg.CORS.Origins)
```

Replace with:
```go
api.RegisterRoutes(app, db, rdb, hub, cfg.App.Version, pool, ds, replayMgr, monMgr, authSvc, cfg.CORS.Origins, cfg)
```

**Step 6: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 7: Smoke test** (requires running stack + auth token):

```bash
# Obtain a token first
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}' | jq -r '.access_token')

# Earnings — returns [] when no API key configured
curl -s "http://localhost:8080/api/v1/calendar/earnings" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Macro — returns FOMC dates even without FRED key
curl -s "http://localhost:8080/api/v1/calendar/macro?from=2026-01-01&to=2026-12-31" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: earnings returns `[]`, macro returns an array of FOMC events with `"category":"fed"`.

**Step 8: Commit**

```bash
git add backend/internal/api/routes.go backend/cmd/server/main.go
git commit -m "feat(routes): register /api/v1/calendar/earnings and /api/v1/calendar/macro routes"
```

---

## Task E4: Create frontend API module

**Files:**
- Create: `frontend/src/api/calendar.ts`

**Step 1: Create the file**

```typescript
import apiClient from '@/api/client'

export interface EarningsEvent {
  symbol: string
  name: string
  report_date: string    // YYYY-MM-DD
  fiscal_period: string
  eps_estimate: string
  currency: string
}

export interface MacroEvent {
  name: string
  date: string           // YYYY-MM-DD
  category: 'fed' | 'inflation' | 'employment' | 'gdp' | 'other'
  description: string
  importance: 'high' | 'medium' | 'low'
}

export interface EarningsParams {
  from?: string
  to?: string
  symbol?: string
}

export interface MacroParams {
  from?: string
  to?: string
}

export async function fetchEarnings(params: EarningsParams = {}): Promise<EarningsEvent[]> {
  const query = new URLSearchParams()
  if (params.from) query.set('from', params.from)
  if (params.to) query.set('to', params.to)
  if (params.symbol) query.set('symbol', params.symbol)
  const qs = query.toString()
  const { data } = await apiClient.get<EarningsEvent[]>(
    `/api/v1/calendar/earnings${qs ? `?${qs}` : ''}`,
  )
  return data
}

export async function fetchMacroEvents(params: MacroParams = {}): Promise<MacroEvent[]> {
  const query = new URLSearchParams()
  if (params.from) query.set('from', params.from)
  if (params.to) query.set('to', params.to)
  const qs = query.toString()
  const { data } = await apiClient.get<MacroEvent[]>(
    `/api/v1/calendar/macro${qs ? `?${qs}` : ''}`,
  )
  return data
}
```

**Step 2: Verify TypeScript compiles**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 3: Commit**

```bash
git add frontend/src/api/calendar.ts
git commit -m "feat(api): add calendar.ts API module for earnings and macro endpoints"
```

---

## Task E5: Create CalendarWidget component

**Files:**
- Create: `frontend/src/components/widgets/CalendarWidget.tsx`

**Step 1: Create the file**

The widget has two tabs: Earnings and Macro. Each tab shows a date-grouped agenda list. Earnings can be filtered by a ticker symbol input (pre-populated from `WidgetProps.ticker`). Macro events are color-coded by importance.

```tsx
import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Calendar, TrendingUp, AlertCircle, Loader2 } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { fetchEarnings, fetchMacroEvents } from '@/api/calendar'
import type { EarningsEvent, MacroEvent } from '@/api/calendar'
import type { WidgetProps } from '@/types/terminal'

// --- Helpers ---

function todayStr(): string {
  return new Date().toISOString().split('T')[0]
}

function sixMonthsAhead(): string {
  const d = new Date()
  d.setMonth(d.getMonth() + 6)
  return d.toISOString().split('T')[0]
}

function threeMonthsAhead(): string {
  const d = new Date()
  d.setMonth(d.getMonth() + 3)
  return d.toISOString().split('T')[0]
}

/** Format a YYYY-MM-DD string for display, e.g. "Mon, Jan 13 2026" */
function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split('-').map(Number)
  return new Date(y, m - 1, d).toLocaleDateString('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

/** Group a sorted list of items by their date field. */
function groupByDate<T extends { report_date?: string; date?: string }>(
  items: T[],
  dateKey: keyof T,
): Map<string, T[]> {
  const groups = new Map<string, T[]>()
  for (const item of items) {
    const key = item[dateKey] as string
    const existing = groups.get(key) ?? []
    existing.push(item)
    groups.set(key, existing)
  }
  return groups
}

// --- Importance styling ---

const importanceConfig = {
  high:   { label: 'High',   className: 'bg-red-500/20 text-red-400 border-red-500/30' },
  medium: { label: 'Med',    className: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30' },
  low:    { label: 'Low',    className: 'bg-zinc-700 text-zinc-400 border-zinc-600' },
} as const

const categoryConfig = {
  fed:        { label: 'FED',        className: 'bg-blue-500/20 text-blue-400 border-blue-500/30' },
  inflation:  { label: 'CPI/PCE',   className: 'bg-orange-500/20 text-orange-400 border-orange-500/30' },
  employment: { label: 'JOBS',       className: 'bg-green-500/20 text-green-400 border-green-500/30' },
  gdp:        { label: 'GDP',        className: 'bg-purple-500/20 text-purple-400 border-purple-500/30' },
  other:      { label: 'OTHER',      className: 'bg-zinc-700 text-zinc-400 border-zinc-600' },
} as const

// --- Sub-components ---

function DateGroupHeader({ dateStr }: { dateStr: string }) {
  return (
    <div className="sticky top-0 z-10 px-3 py-1.5 bg-zinc-900/95 backdrop-blur-sm border-b border-zinc-800">
      <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">
        {formatDate(dateStr)}
      </span>
    </div>
  )
}

function EarningsRow({ event }: { event: EarningsEvent }) {
  return (
    <div className="flex items-start gap-3 px-3 py-2.5 hover:bg-zinc-800/50 transition-colors border-b border-zinc-800/50 last:border-0">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm font-semibold text-amber-400">
            {event.symbol}
          </span>
          <span className="text-xs text-zinc-400 truncate">{event.name}</span>
        </div>
        <div className="flex items-center gap-2 mt-0.5">
          {event.eps_estimate && event.eps_estimate !== 'None' && (
            <span className="text-xs text-zinc-500">
              Est. EPS: <span className="text-zinc-300">{event.eps_estimate}</span>
              {event.currency && event.currency !== 'USD' && (
                <span className="text-zinc-500"> {event.currency}</span>
              )}
            </span>
          )}
          {event.fiscal_period && (
            <span className="text-xs text-zinc-600">{event.fiscal_period}</span>
          )}
        </div>
      </div>
    </div>
  )
}

function MacroRow({ event }: { event: MacroEvent }) {
  const imp = importanceConfig[event.importance] ?? importanceConfig.low
  const cat = categoryConfig[event.category] ?? categoryConfig.other

  return (
    <div className="flex items-start gap-3 px-3 py-2.5 hover:bg-zinc-800/50 transition-colors border-b border-zinc-800/50 last:border-0">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-sm font-medium text-zinc-100">{event.name}</span>
          <Badge variant="outline" className={`text-[10px] px-1.5 py-0 h-4 ${cat.className}`}>
            {cat.label}
          </Badge>
          <Badge variant="outline" className={`text-[10px] px-1.5 py-0 h-4 ${imp.className}`}>
            {imp.label}
          </Badge>
        </div>
        {event.description && (
          <p className="text-xs text-zinc-500 mt-0.5 truncate">{event.description}</p>
        )}
      </div>
    </div>
  )
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center h-32 gap-2 text-zinc-500">
      <Calendar className="w-8 h-8 opacity-40" />
      <span className="text-sm">{message}</span>
    </div>
  )
}

function LoadingState() {
  return (
    <div className="flex items-center justify-center h-32 gap-2 text-zinc-500">
      <Loader2 className="w-5 h-5 animate-spin" />
      <span className="text-sm">Loading…</span>
    </div>
  )
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="flex items-center justify-center h-32 gap-2 text-red-400">
      <AlertCircle className="w-5 h-5" />
      <span className="text-sm">{message}</span>
    </div>
  )
}

// --- Earnings tab ---

function EarningsTab({ initialSymbol }: { initialSymbol: string }) {
  const [symbolFilter, setSymbolFilter] = useState(initialSymbol)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['calendar', 'earnings', symbolFilter],
    queryFn: () =>
      fetchEarnings({
        from: todayStr(),
        to: threeMonthsAhead(),
        symbol: symbolFilter || undefined,
      }),
    staleTime: 60 * 60 * 1000, // 1 hour — matches backend cache TTL
  })

  const grouped = useMemo(() => {
    if (!data) return new Map<string, EarningsEvent[]>()
    const sorted = [...data].sort((a, b) => a.report_date.localeCompare(b.report_date))
    return groupByDate(sorted, 'report_date')
  }, [data])

  return (
    <div className="flex flex-col h-full">
      {/* Symbol filter input */}
      <div className="px-3 py-2 border-b border-zinc-800">
        <Input
          placeholder="Filter by symbol (e.g. AAPL)"
          value={symbolFilter}
          onChange={(e) => setSymbolFilter(e.target.value.toUpperCase())}
          className="h-7 text-xs bg-zinc-800 border-zinc-700 focus-visible:ring-amber-500/30"
        />
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading && <LoadingState />}
        {isError && <ErrorState message="Failed to load earnings data" />}
        {!isLoading && !isError && grouped.size === 0 && (
          <EmptyState message="No earnings events found" />
        )}
        {!isLoading &&
          !isError &&
          Array.from(grouped.entries()).map(([date, events]) => (
            <div key={date}>
              <DateGroupHeader dateStr={date} />
              {events.map((event, i) => (
                <EarningsRow key={`${event.symbol}-${date}-${i}`} event={event} />
              ))}
            </div>
          ))}
      </div>

      {/* Footer count */}
      {data && data.length > 0 && (
        <div className="px-3 py-1.5 border-t border-zinc-800 text-xs text-zinc-500">
          {data.length} event{data.length !== 1 ? 's' : ''} · next 3 months
        </div>
      )}
    </div>
  )
}

// --- Macro tab ---

function MacroTab() {
  const [importanceFilter, setImportanceFilter] = useState<'all' | 'high' | 'medium' | 'low'>('all')

  const { data, isLoading, isError } = useQuery({
    queryKey: ['calendar', 'macro'],
    queryFn: () =>
      fetchMacroEvents({
        from: todayStr(),
        to: sixMonthsAhead(),
      }),
    staleTime: 4 * 60 * 60 * 1000, // 4 hours — matches backend cache TTL
  })

  const filtered = useMemo(() => {
    if (!data) return []
    if (importanceFilter === 'all') return data
    return data.filter((e) => e.importance === importanceFilter)
  }, [data, importanceFilter])

  const grouped = useMemo(() => {
    const sorted = [...filtered].sort((a, b) => a.date.localeCompare(b.date))
    return groupByDate(sorted, 'date')
  }, [filtered])

  return (
    <div className="flex flex-col h-full">
      {/* Importance filter */}
      <div className="px-3 py-2 border-b border-zinc-800 flex gap-1.5">
        {(['all', 'high', 'medium', 'low'] as const).map((level) => (
          <button
            key={level}
            onClick={() => setImportanceFilter(level)}
            className={[
              'text-[10px] px-2 py-0.5 rounded border font-medium transition-colors',
              importanceFilter === level
                ? 'bg-amber-500/20 text-amber-400 border-amber-500/40'
                : 'bg-zinc-800 text-zinc-400 border-zinc-700 hover:border-zinc-600',
            ].join(' ')}
          >
            {level === 'all' ? 'All' : level.charAt(0).toUpperCase() + level.slice(1)}
          </button>
        ))}
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading && <LoadingState />}
        {isError && <ErrorState message="Failed to load macro events" />}
        {!isLoading && !isError && grouped.size === 0 && (
          <EmptyState message="No macro events found" />
        )}
        {!isLoading &&
          !isError &&
          Array.from(grouped.entries()).map(([date, events]) => (
            <div key={date}>
              <DateGroupHeader dateStr={date} />
              {events.map((event, i) => (
                <MacroRow key={`${event.name}-${date}-${i}`} event={event} />
              ))}
            </div>
          ))}
      </div>

      {/* Footer count */}
      {data && data.length > 0 && (
        <div className="px-3 py-1.5 border-t border-zinc-800 text-xs text-zinc-500">
          {filtered.length} event{filtered.length !== 1 ? 's' : ''} · next 6 months
        </div>
      )}
    </div>
  )
}

// --- Root widget ---

export function CalendarWidget({ ticker }: WidgetProps) {
  return (
    <div className="flex flex-col h-full bg-zinc-950 text-zinc-100 overflow-hidden">
      <Tabs defaultValue="earnings" className="flex flex-col h-full">
        <TabsList className="w-full rounded-none border-b border-zinc-800 bg-zinc-900 h-8 shrink-0">
          <TabsTrigger
            value="earnings"
            className="flex-1 text-xs data-[state=active]:bg-zinc-800 data-[state=active]:text-amber-400 rounded-none"
          >
            <TrendingUp className="w-3 h-3 mr-1.5" />
            Earnings
          </TabsTrigger>
          <TabsTrigger
            value="macro"
            className="flex-1 text-xs data-[state=active]:bg-zinc-800 data-[state=active]:text-amber-400 rounded-none"
          >
            <Calendar className="w-3 h-3 mr-1.5" />
            Macro
          </TabsTrigger>
        </TabsList>

        <TabsContent value="earnings" className="flex-1 overflow-hidden mt-0">
          <EarningsTab initialSymbol={ticker ?? ''} />
        </TabsContent>

        <TabsContent value="macro" className="flex-1 overflow-hidden mt-0">
          <MacroTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
```

**Step 2: Install shadcn Badge component** if not already present:

```bash
# Check if Badge already exists
ls frontend/src/components/ui/badge.tsx 2>/dev/null && echo "exists" || echo "missing"
```

If missing:
```bash
docker compose exec frontend npx shadcn-ui@latest add badge
```

**Step 3: Verify TypeScript compiles**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors. If `@/components/ui/badge` is missing, re-run the shadcn add command above.

**Step 4: Commit**

```bash
git add frontend/src/components/widgets/CalendarWidget.tsx
git commit -m "feat(widgets): add CalendarWidget with Earnings and Macro tabs"
```

---

## Task E6: Wire CalendarWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import** at the top of the file, after the `AIChatWidget` import line:

```typescript
import { CalendarWidget }   from '@/components/widgets/CalendarWidget'
```

**Step 2: Replace the CAL stub** in the `WIDGET_REGISTRY` object.

Find:
```typescript
  CAL:  () => <ComingSoon label="Calendar (Phase E)" />,
```

Replace with:
```typescript
  CAL:  CalendarWidget,
```

**Step 3: Verify the full registry object** now reads:

```typescript
export const WIDGET_REGISTRY: Record<FunctionCode, React.ComponentType<WidgetProps>> = {
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

  // Phase E
  CAL:  CalendarWidget,

  // Phases B, C, D, F, G, H: stubbed for now
  HM:   () => <ComingSoon label="Market Heatmap (Phase B)" />,
  SCR:  () => <ComingSoon label="Screener (Phase C)" />,
  FA:   () => <ComingSoon label="Fundamentals (Phase D)" />,
  YCRV: () => <ComingSoon label="Yield Curves (Phase F)" />,
  OPT:  () => <ComingSoon label="Options Chain (Phase G)" />,
  RISK: () => <ComingSoon label="Risk Analytics (Phase H)" />,
}
```

**Step 4: TypeScript and build check**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 5: Commit**

```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(terminal): wire CalendarWidget into WidgetRegistry — replaces Phase E stub"
```

---

## Task E7: End-to-end verification

**Step 1: Rebuild and start the full stack**

```bash
docker compose up --build -d
```

**Step 2: Confirm backend health**

```bash
make health
```

Expected: `{"status":"ok","db":"ok","redis":"ok","version":"..."}`

**Step 3: Test earnings endpoint (no API key — graceful empty)**

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}' | jq -r '.access_token')

curl -s "http://localhost:8080/api/v1/calendar/earnings" \
  -H "Authorization: Bearer $TOKEN" | jq 'type, length'
```

Expected: `"array"` and `0` (empty array, no crash).

**Step 4: Test macro endpoint (FOMC dates hardcoded — always works)**

```bash
curl -s "http://localhost:8080/api/v1/calendar/macro?from=2026-01-01&to=2026-12-31" \
  -H "Authorization: Bearer $TOKEN" | jq '.[0]'
```

Expected: a JSON object with `"name":"FOMC Meeting"`, `"category":"fed"`, `"importance":"high"`.

**Step 5: Test Redis caching (second request should hit cache)**

```bash
# Call twice — second call should return instantly from Redis
time curl -s "http://localhost:8080/api/v1/calendar/macro?from=2026-01-01&to=2026-12-31" \
  -H "Authorization: Bearer $TOKEN" > /dev/null

time curl -s "http://localhost:8080/api/v1/calendar/macro?from=2026-01-01&to=2026-12-31" \
  -H "Authorization: Bearer $TOKEN" > /dev/null
```

Expected: second call noticeably faster.

**Step 6: Test CAL widget in the terminal UI**

1. Open `http://localhost:5173/terminal` in the browser.
2. Type `CAL` in the command bar and press Enter.
3. Confirm the panel opens showing the Earnings tab with a symbol filter input.
4. Switch to the Macro tab and confirm FOMC meeting dates appear.
5. Test the importance filter buttons (All / High / Med / Low).
6. Confirm the component does not crash when the earnings list is empty (no Alpha Vantage key).

**Step 7: (Optional) Test with real API keys**

Add to your local `.env` file:
```
ALPHA_VANTAGE_API_KEY=your_real_key_here
FRED_API_KEY=your_real_key_here
```

Then restart:
```bash
docker compose restart backend
```

Verify:
```bash
curl -s "http://localhost:8080/api/v1/calendar/earnings?symbol=AAPL" \
  -H "Authorization: Bearer $TOKEN" | jq 'length'
```

Expected: a positive number of earnings events.

**Step 8: Commit**

No new code changes in this task — if all tests pass, proceed to final commit.

```bash
git add -A
git status  # confirm nothing unexpected is staged
```

---

## Phase E Completion Checklist

### Backend
- [ ] `ExternalAPIsConfig` struct added to `config.go` with `AlphaVantageKey` and `FREDKey` fields
- [ ] Both fields populated from `ALPHA_VANTAGE_API_KEY` and `FRED_API_KEY` env vars with empty string defaults
- [ ] `.env.example` updated with the two new commented-out keys
- [ ] `backend/internal/api/calendar_handler.go` created with:
  - [ ] `EarningsEvent` and `MacroEvent` response structs
  - [ ] `calendarHandler` struct wrapping `redis.Client` and `*config.Config`
  - [ ] `earnings` handler: date-range + symbol filtering, Alpha Vantage CSV parsing, 1h Redis TTL
  - [ ] `macro` handler: FRED releases + hardcoded FOMC dates, 4h Redis TTL
  - [ ] Graceful degradation: returns empty array if API key is not set
  - [ ] 5 MB response body cap on FRED response (`io.LimitReader`)
  - [ ] `classifyFREDRelease` maps release names to category + importance
  - [ ] `dedupAndSortMacroEvents` deduplicates and sorts ascending by date
  - [ ] FOMC dates hardcoded for 2024, 2025, 2026
- [ ] `routes.go` updated:
  - [ ] `RegisterRoutes` signature extended with `cfg *config.Config`
  - [ ] `GET /api/v1/calendar/earnings` registered under `protected` group
  - [ ] `GET /api/v1/calendar/macro` registered under `protected` group
- [ ] `main.go` updated to pass `cfg` to `RegisterRoutes`
- [ ] `docker compose exec backend go build ./...` passes with no errors

### Frontend
- [ ] `frontend/src/api/calendar.ts` created with:
  - [ ] `EarningsEvent` and `MacroEvent` TypeScript interfaces
  - [ ] `fetchEarnings(params)` function using `apiClient`
  - [ ] `fetchMacroEvents(params)` function using `apiClient`
- [ ] `frontend/src/components/widgets/CalendarWidget.tsx` created with:
  - [ ] Accepts `WidgetProps` (`ticker`, `market`, `timeframe`, `params`)
  - [ ] `ticker` pre-populates the Earnings symbol filter
  - [ ] Two tabs: Earnings / Macro
  - [ ] Earnings tab: date-grouped agenda list, symbol filter input, 1h `staleTime`
  - [ ] Macro tab: date-grouped agenda list, importance filter buttons, 4h `staleTime`
  - [ ] `EarningsRow` shows symbol (amber monospace), name, EPS estimate, fiscal period
  - [ ] `MacroRow` shows name, category badge, importance badge (red/yellow/gray), description
  - [ ] Loading, empty, and error states handled in both tabs
  - [ ] Footer showing event count and date range
  - [ ] No inline styles — Tailwind classes only
  - [ ] Icons from `lucide-react` only
- [ ] shadcn `Badge` component present at `frontend/src/components/ui/badge.tsx`
- [ ] `WidgetRegistry.tsx` updated: `CAL` maps to `CalendarWidget` (stub removed)
- [ ] `npx tsc --noEmit` passes with no errors

### Integration
- [ ] `GET /api/v1/calendar/earnings` returns `[]` when `ALPHA_VANTAGE_API_KEY` is not set (no 500 error)
- [ ] `GET /api/v1/calendar/macro` returns FOMC events without any API key configured
- [ ] Redis cache: second request for same parameters is served from cache
- [ ] Opening `CAL` in the terminal command bar renders `CalendarWidget` (not the "coming soon" stub)
- [ ] Earnings tab shows symbol filter and gracefully shows empty state
- [ ] Macro tab shows FOMC dates in the High importance tier
- [ ] Importance filter buttons work: toggling to "High" hides Medium/Low events
- [ ] No console errors in browser DevTools when widget is open
