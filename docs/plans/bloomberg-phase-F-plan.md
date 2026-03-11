# Bloomberg Terminal — Phase F: Yield Curves (YCRV)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `YCRV` yield curves widget. After this phase, users can open a panel with function code `YCRV` and see US Treasury yield data fetched from the FRED API, displayed as two recharts views: a historical line chart (yield over time, one line per maturity) and a snapshot curve chart (maturity on x-axis, most recent yield on y-axis).

**Architecture:**
- A new Go handler `yieldcurve_handler.go` fetches up to 4 FRED series in parallel using goroutines + `sync.WaitGroup`, caches responses in Redis for 4 hours, and returns a JSON array of `YieldSeries`.
- The route is registered under the `protected` group in `routes.go`.
- The frontend API client in `api/yieldcurves.ts` calls this route with React Query.
- `YieldCurveWidget.tsx` renders two tabs: **Historical** and **Snapshot**, with date range and series toggle controls.
- `WidgetRegistry.tsx` is updated to replace the `YCRV` stub with the real component.

**Tech Stack:** Go/Fiber, `sync.WaitGroup`, `net/http`, Redis (`go-redis/v9`), React 18, React Query v5, recharts 2.x, Tailwind, lucide-react.

**FRED series used:**

| Series ID | Label    | Maturity |
|-----------|----------|----------|
| `DGS2`    | 2-Year   | 2Y       |
| `DGS5`    | 5-Year   | 5Y       |
| `DGS10`   | 10-Year  | 10Y      |
| `DGS30`   | 30-Year  | 30Y      |

---

## Task F1: Add FRED API key to config and .env.example

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `.env.example`

**Step 1: Add an `ExternalAPIs` config struct and `FREDKey` field**

In `backend/internal/config/config.go`, add the new struct and field. The full diff is:

```go
// Add this new struct after the existing CORSConfig struct definition
type ExternalAPIsConfig struct {
	FREDKey string
}
```

Then add the field to `Config`:

```go
type Config struct {
	App          AppConfig
	DB           DBConfig
	Redis        RedisConfig
	Worker       WorkerConfig
	CORS         CORSConfig
	ExternalAPIs ExternalAPIsConfig   // ← add this line
}
```

Then populate it in the `Load()` function, inside the `cfg := &Config{...}` block, after the existing `CORS` field:

```go
ExternalAPIs: ExternalAPIsConfig{
    FREDKey: getEnv("FRED_API_KEY", ""),
},
```

**Step 2: Add `FRED_API_KEY` to `.env.example`**

Append the following block after the `# ── Notifications` section and before `# ── Frontend`:

```bash
# ── External Market Data APIs ────────────────────────────────────────────────
# FRED (Federal Reserve Economic Data) — required for Yield Curves widget (YCRV)
# Register free at https://fred.stlouisfed.org/docs/api/api_key.html
FRED_API_KEY=
```

**Step 3: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 4: Commit**

```bash
git add backend/internal/config/config.go .env.example
git commit -m "feat(config): add FREDKey config field and FRED_API_KEY env var for yield curves"
```

---

## Task F2: Pass config to RegisterRoutes

**Files:**
- Modify: `backend/internal/api/routes.go`
- Modify: `backend/cmd/server/main.go`

**Context:** The yield curve handler needs `cfg.ExternalAPIs.FREDKey` and `rdb`. `RegisterRoutes` already receives `rdb`. We need to pass `cfg` (or just the FRED key string) to `RegisterRoutes`.

**Step 1: Check the current `RegisterRoutes` signature**

```bash
grep -n "func RegisterRoutes" backend/internal/api/routes.go
```

The current signature is:

```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager, authSvc *auth.AuthService, corsOrigins string)
```

**Step 2: Add `fredAPIKey string` parameter to `RegisterRoutes`**

In `backend/internal/api/routes.go`, update the function signature — add `fredAPIKey string` as the last parameter before the closing `)`:

```go
func RegisterRoutes(app *fiber.App, db *gorm.DB, rdb *redis.Client, hub *ws.Hub, version string, pool *worker.WorkerPool, ds *adapter.DataService, mgr *replay.Manager, monMgr *monitor.Manager, authSvc *auth.AuthService, corsOrigins string, fredAPIKey string)
```

**Step 3: Update the call site in `main.go`**

```bash
grep -n "RegisterRoutes" backend/cmd/server/main.go
```

Find the `RegisterRoutes(...)` call and append `cfg.ExternalAPIs.FREDKey` as the last argument:

```go
api.RegisterRoutes(app, db, rdb, hub, cfg.App.Version, pool, ds, replayMgr, monMgr, authSvc, cfg.CORS.Origins, cfg.ExternalAPIs.FREDKey)
```

**Step 4: Build check**

```bash
docker compose exec backend go build ./...
```

Expected: no errors.

**Step 5: Commit**

```bash
git add backend/internal/api/routes.go backend/cmd/server/main.go
git commit -m "feat(routes): thread fredAPIKey through RegisterRoutes for yield curve handler"
```

---

## Task F3: Create the yield curve backend handler

**Files:**
- Create: `backend/internal/api/yieldcurve_handler.go`

**Step 1: Create the handler file with the following content**

```go
package api

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const yieldCacheTTL = 4 * time.Hour

// yieldCurveHandler fetches US Treasury yield series from the FRED API.
type yieldCurveHandler struct {
	fredAPIKey string
	rdb        *redis.Client
	httpClient *http.Client
}

func newYieldCurveHandler(fredAPIKey string, rdb *redis.Client) *yieldCurveHandler {
	return &yieldCurveHandler{
		fredAPIKey: fredAPIKey,
		rdb:        rdb,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// YieldPoint is a single date/value observation.
type YieldPoint struct {
	Date  string  `json:"date"`  // YYYY-MM-DD
	Value float64 `json:"value"` // yield percent, e.g. 4.52
}

// YieldSeries is one FRED series with its parsed observations.
type YieldSeries struct {
	SeriesID string       `json:"series_id"` // e.g. "DGS10"
	Label    string       `json:"label"`     // e.g. "10-Year"
	Data     []YieldPoint `json:"data"`
}

// seriesMeta maps FRED series IDs to human-readable labels.
var seriesMeta = map[string]string{
	"DGS2":  "2-Year",
	"DGS5":  "5-Year",
	"DGS10": "10-Year",
	"DGS30": "30-Year",
}

// validSeriesIDs is the allow-list of permitted series.
var validSeriesIDs = map[string]bool{
	"DGS2":  true,
	"DGS5":  true,
	"DGS10": true,
	"DGS30": true,
}

// GET /api/v1/yield-curves?series=DGS2,DGS5,DGS10,DGS30&from=2024-01-01&to=2024-12-31
func (h *yieldCurveHandler) get(c *fiber.Ctx) error {
	if h.fredAPIKey == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "FRED_API_KEY is not configured",
		})
	}

	// Parse and validate query params
	seriesParam := c.Query("series", "DGS2,DGS5,DGS10,DGS30")
	from := c.Query("from", "")
	to := c.Query("to", "")

	if from == "" {
		// Default: 1 year ago
		from = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}

	// Validate date formats (YYYY-MM-DD)
	if _, err := time.Parse("2006-01-02", from); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date, expected YYYY-MM-DD"})
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date, expected YYYY-MM-DD"})
	}

	// Build validated series list
	requestedIDs := strings.Split(seriesParam, ",")
	var seriesIDs []string
	for _, id := range requestedIDs {
		id = strings.TrimSpace(strings.ToUpper(id))
		if validSeriesIDs[id] {
			seriesIDs = append(seriesIDs, id)
		}
	}
	if len(seriesIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no valid series IDs provided"})
	}

	// Build cache key from sorted series IDs hash + date range
	hash := fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(seriesIDs, ","))))
	cacheKey := fmt.Sprintf("yield-curves:%s:%s:%s", hash, from, to)

	// Check Redis cache
	if h.rdb != nil {
		cached, err := h.rdb.Get(c.Context(), cacheKey).Bytes()
		if err == nil {
			c.Set("X-Cache", "HIT")
			c.Set("Content-Type", "application/json")
			return c.Send(cached)
		}
	}

	// Fetch all series in parallel
	results := make([]YieldSeries, len(seriesIDs))
	errs := make([]error, len(seriesIDs))
	var wg sync.WaitGroup

	for i, id := range seriesIDs {
		wg.Add(1)
		go func(idx int, seriesID string) {
			defer wg.Done()
			series, err := h.fetchFREDSeries(c.Context(), seriesID, from, to)
			if err != nil {
				errs[idx] = err
				return
			}
			results[idx] = series
		}(i, id)
	}
	wg.Wait()

	// Check for any fetch errors
	for i, err := range errs {
		if err != nil {
			log.Printf("yieldcurve: failed to fetch series %s: %v", seriesIDs[i], err)
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to fetch series %s from FRED", seriesIDs[i]),
			})
		}
	}

	// Cache and respond
	payload, err := json.Marshal(results)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	if h.rdb != nil {
		if setErr := h.rdb.Set(c.Context(), cacheKey, payload, yieldCacheTTL).Err(); setErr != nil {
			log.Printf("yieldcurve: redis Set failed: %v", setErr)
		}
	}

	c.Set("X-Cache", "MISS")
	c.Set("Content-Type", "application/json")
	return c.Send(payload)
}

// fetchFREDSeries calls the FRED observations endpoint for one series.
func (h *yieldCurveHandler) fetchFREDSeries(ctx context.Context, seriesID, from, to string) (YieldSeries, error) {
	url := fmt.Sprintf(
		"https://api.stlouisfed.org/fred/series/observations?series_id=%s&observation_start=%s&observation_end=%s&api_key=%s&file_type=json",
		seriesID, from, to, h.fredAPIKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return YieldSeries{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return YieldSeries{}, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return YieldSeries{}, fmt.Errorf("FRED returned HTTP %d for series %s", resp.StatusCode, seriesID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return YieldSeries{}, fmt.Errorf("read body: %w", err)
	}

	// FRED response shape:
	// { "observations": [ { "date": "2024-01-02", "value": "4.43" }, ... ] }
	var fredResp struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(body, &fredResp); err != nil {
		return YieldSeries{}, fmt.Errorf("unmarshal: %w", err)
	}

	label, ok := seriesMeta[seriesID]
	if !ok {
		label = seriesID
	}

	points := make([]YieldPoint, 0, len(fredResp.Observations))
	for _, obs := range fredResp.Observations {
		// FRED uses "." for missing values — skip them
		if obs.Value == "." || obs.Value == "" {
			continue
		}
		val, err := strconv.ParseFloat(obs.Value, 64)
		if err != nil {
			continue
		}
		points = append(points, YieldPoint{Date: obs.Date, Value: val})
	}

	return YieldSeries{
		SeriesID: seriesID,
		Label:    label,
		Data:     points,
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
git add backend/internal/api/yieldcurve_handler.go
git commit -m "feat(api): add yield curve handler — parallel FRED series fetch with Redis cache"
```

---

## Task F4: Register the yield curve route

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Instantiate the handler** in `RegisterRoutes`, after the workspace handler init line (`workspaceH := newWorkspaceHandler(db)`):

```go
ycH := newYieldCurveHandler(fredAPIKey, rdb)
```

**Step 2: Register the route** in the `protected` group, after the workspace routes block:

```go
// Yield Curves (Bloomberg YCRV widget)
protected.Get("/yield-curves", ycH.get)
```

**Step 3: Build + smoke test**

```bash
docker compose exec backend go build ./...
make up
# Wait for backend to start, then test (replace <token> with a valid JWT):
curl -s "http://localhost:8080/api/v1/yield-curves?series=DGS10&from=2024-01-01&to=2024-03-31" \
  -H "Authorization: Bearer <token>" | jq '.[0] | {series_id, label, count: (.data | length)}'
```

Expected (if FRED_API_KEY is set):
```json
{
  "series_id": "DGS10",
  "label": "10-Year",
  "count": 63
}
```

If `FRED_API_KEY` is not set, the response should be:
```json
{ "error": "FRED_API_KEY is not configured" }
```
with HTTP 503 — this is correct behavior.

**Step 4: Verify cache header**

```bash
# First call — should be MISS
curl -si "http://localhost:8080/api/v1/yield-curves?series=DGS10&from=2024-01-01&to=2024-03-31" \
  -H "Authorization: Bearer <token>" | grep "X-Cache"
# Second call — should be HIT
curl -si "http://localhost:8080/api/v1/yield-curves?series=DGS10&from=2024-01-01&to=2024-03-31" \
  -H "Authorization: Bearer <token>" | grep "X-Cache"
```

**Step 5: Commit**

```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register GET /api/v1/yield-curves under protected group"
```

---

## Task F5: Create the frontend API client

**Files:**
- Create: `frontend/src/api/yieldcurves.ts`

**Step 1: Create the file**

```typescript
import apiClient from '@/api/client'
import { useQuery } from '@tanstack/react-query'

// Mirror of backend YieldPoint struct
export interface YieldPoint {
  date: string   // YYYY-MM-DD
  value: number  // yield percent, e.g. 4.52
}

// Mirror of backend YieldSeries struct
export interface YieldSeries {
  series_id: string  // e.g. "DGS10"
  label: string      // e.g. "10-Year"
  data: YieldPoint[]
}

export interface YieldCurveParams {
  series: string[]     // e.g. ["DGS2", "DGS5", "DGS10", "DGS30"]
  from: string         // YYYY-MM-DD
  to: string           // YYYY-MM-DD
}

export async function fetchYieldCurves(params: YieldCurveParams): Promise<YieldSeries[]> {
  const { data } = await apiClient.get<YieldSeries[]>('/api/v1/yield-curves', {
    params: {
      series: params.series.join(','),
      from: params.from,
      to: params.to,
    },
  })
  return data
}

export function useYieldCurves(params: YieldCurveParams, enabled = true) {
  return useQuery({
    queryKey: ['yield-curves', params.series, params.from, params.to],
    queryFn: () => fetchYieldCurves(params),
    enabled,
    staleTime: 4 * 60 * 60 * 1000,  // 4 hours — matches backend cache TTL
    retry: 2,
  })
}
```

**Step 2: Verify TypeScript compiles**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 3: Commit**

```bash
git add frontend/src/api/yieldcurves.ts
git commit -m "feat(api): add yieldcurves API client with useYieldCurves React Query hook"
```

---

## Task F6: Create the YieldCurveWidget component

**Files:**
- Create: `frontend/src/components/widgets/YieldCurveWidget.tsx`

**Step 1: Create the file**

```typescript
import { useState, useMemo } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { TrendingUp, AlertCircle, Loader2 } from 'lucide-react'
import { useYieldCurves, type YieldSeries } from '@/api/yieldcurves'
import type { WidgetProps } from '@/types/terminal'

// ─── Constants ────────────────────────────────────────────────────────────────

const ALL_SERIES = ['DGS2', 'DGS5', 'DGS10', 'DGS30'] as const
type SeriesID = (typeof ALL_SERIES)[number]

const SERIES_COLORS: Record<SeriesID, string> = {
  DGS2:  '#60a5fa',  // blue-400
  DGS5:  '#34d399',  // emerald-400
  DGS10: '#f59e0b',  // amber-500
  DGS30: '#f87171',  // red-400
}

const SERIES_LABELS: Record<SeriesID, string> = {
  DGS2:  '2Y',
  DGS5:  '5Y',
  DGS10: '10Y',
  DGS30: '30Y',
}

type RangeKey = '1M' | '3M' | '1Y' | '5Y'

const DATE_RANGES: Record<RangeKey, number> = {
  '1M': 1,
  '3M': 3,
  '1Y': 12,
  '5Y': 60,
}

function subtractMonths(months: number): string {
  const d = new Date()
  d.setMonth(d.getMonth() - months)
  return d.toISOString().slice(0, 10)
}

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function HistoricalChart({ series }: { series: YieldSeries[] }) {
  // Merge all series into a single array of { date, DGS2?, DGS5?, DGS10?, DGS30? }
  // keyed by date for recharts LineChart
  const merged = useMemo(() => {
    const map = new Map<string, Record<string, number>>()
    for (const s of series) {
      for (const pt of s.data) {
        if (!map.has(pt.date)) map.set(pt.date, { date_str: 0 })
        map.get(pt.date)![s.series_id] = pt.value
      }
    }
    return Array.from(map.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, values]) => ({ date, ...values }))
  }, [series])

  if (merged.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
        No data available for the selected range.
      </div>
    )
  }

  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={merged} margin={{ top: 8, right: 16, left: 0, bottom: 8 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.08)" />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 10, fill: '#9ca3af' }}
          tickFormatter={(v: string) => v.slice(0, 7)}  // show YYYY-MM
          interval="preserveStartEnd"
        />
        <YAxis
          tick={{ fontSize: 10, fill: '#9ca3af' }}
          tickFormatter={(v: number) => `${v.toFixed(2)}%`}
          domain={['auto', 'auto']}
          width={52}
        />
        <Tooltip
          contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: 6 }}
          labelStyle={{ color: '#e5e7eb', fontSize: 11 }}
          itemStyle={{ fontSize: 11 }}
          formatter={(value: number, name: string) => [`${value.toFixed(2)}%`, name]}
        />
        <Legend wrapperStyle={{ fontSize: 11, paddingTop: 4 }} />
        {series.map((s) => (
          <Line
            key={s.series_id}
            type="monotone"
            dataKey={s.series_id}
            name={s.label}
            stroke={SERIES_COLORS[s.series_id as SeriesID] ?? '#9ca3af'}
            dot={false}
            strokeWidth={1.5}
            connectNulls
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  )
}

function SnapshotChart({ series }: { series: YieldSeries[] }) {
  // Build curve snapshot: for each series, take the most recent data point
  const snapshotData = useMemo(() => {
    const points: { maturity: string; yield: number; order: number }[] = []
    const order: Record<SeriesID, number> = { DGS2: 0, DGS5: 1, DGS10: 2, DGS30: 3 }
    for (const s of series) {
      if (s.data.length === 0) continue
      const latest = s.data[s.data.length - 1]
      points.push({
        maturity: SERIES_LABELS[s.series_id as SeriesID] ?? s.label,
        yield: latest.value,
        order: order[s.series_id as SeriesID] ?? 99,
      })
    }
    return points.sort((a, b) => a.order - b.order)
  }, [series])

  const latestDate = useMemo(() => {
    const dates = series.flatMap((s) => s.data.map((p) => p.date))
    if (dates.length === 0) return ''
    return dates.sort().at(-1) ?? ''
  }, [series])

  if (snapshotData.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
        No snapshot data available.
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full gap-1">
      {latestDate && (
        <p className="text-xs text-muted-foreground px-1">
          Most recent observation: <span className="text-foreground font-mono">{latestDate}</span>
        </p>
      )}
      <div className="flex-1 min-h-0">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={snapshotData} margin={{ top: 8, right: 16, left: 0, bottom: 8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.08)" />
            <XAxis
              dataKey="maturity"
              tick={{ fontSize: 11, fill: '#9ca3af' }}
            />
            <YAxis
              tick={{ fontSize: 10, fill: '#9ca3af' }}
              tickFormatter={(v: number) => `${v.toFixed(2)}%`}
              domain={['auto', 'auto']}
              width={52}
            />
            <Tooltip
              contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: 6 }}
              labelStyle={{ color: '#e5e7eb', fontSize: 11 }}
              itemStyle={{ fontSize: 11 }}
              formatter={(value: number) => [`${value.toFixed(2)}%`, 'Yield']}
            />
            <Line
              type="monotone"
              dataKey="yield"
              name="Yield"
              stroke="#f59e0b"
              dot={{ r: 4, fill: '#f59e0b' }}
              strokeWidth={2}
              connectNulls
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

// ─── Main Widget ──────────────────────────────────────────────────────────────

export function YieldCurveWidget(_props: WidgetProps) {
  const [activeTab, setActiveTab] = useState<'historical' | 'snapshot'>('historical')
  const [range, setRange] = useState<RangeKey>('1Y')
  const [enabledSeries, setEnabledSeries] = useState<Set<SeriesID>>(new Set(ALL_SERIES))

  const from = subtractMonths(DATE_RANGES[range])
  const to = today()

  const { data, isLoading, isError, error } = useYieldCurves({
    series: Array.from(enabledSeries),
    from,
    to,
  })

  function toggleSeries(id: SeriesID) {
    setEnabledSeries((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        // Don't allow deselecting all series
        if (next.size === 1) return prev
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  return (
    <div className="flex flex-col h-full bg-background text-foreground">
      {/* Header toolbar */}
      <div className="flex items-center justify-between gap-2 px-3 py-2 border-b border-border flex-shrink-0">
        {/* Left: icon + title + tabs */}
        <div className="flex items-center gap-3">
          <TrendingUp className="w-4 h-4 text-amber-500 flex-shrink-0" />
          <span className="text-xs font-semibold text-foreground tracking-wide">US TREASURY YIELD CURVES</span>
          <div className="flex gap-0.5">
            {(['historical', 'snapshot'] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`px-2 py-0.5 text-xs rounded font-medium transition-colors ${
                  activeTab === tab
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {tab === 'historical' ? 'Historical' : 'Snapshot'}
              </button>
            ))}
          </div>
        </div>

        {/* Right: range selector + series toggles */}
        <div className="flex items-center gap-3">
          {/* Series toggles — only visible in historical view */}
          {activeTab === 'historical' && (
            <div className="flex gap-1">
              {ALL_SERIES.map((id) => (
                <button
                  key={id}
                  onClick={() => toggleSeries(id)}
                  className={`px-1.5 py-0.5 text-xs rounded border transition-colors font-mono ${
                    enabledSeries.has(id)
                      ? 'border-transparent text-white'
                      : 'border-border text-muted-foreground opacity-40'
                  }`}
                  style={
                    enabledSeries.has(id)
                      ? { backgroundColor: SERIES_COLORS[id] + '33', borderColor: SERIES_COLORS[id] + '80', color: SERIES_COLORS[id] }
                      : undefined
                  }
                >
                  {SERIES_LABELS[id]}
                </button>
              ))}
            </div>
          )}

          {/* Date range buttons */}
          <div className="flex gap-0.5">
            {(Object.keys(DATE_RANGES) as RangeKey[]).map((r) => (
              <button
                key={r}
                onClick={() => setRange(r)}
                className={`px-2 py-0.5 text-xs rounded font-medium transition-colors ${
                  range === r
                    ? 'bg-muted text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {r}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Chart area */}
      <div className="flex-1 min-h-0 p-2">
        {isLoading && (
          <div className="flex items-center justify-center h-full gap-2 text-muted-foreground text-sm">
            <Loader2 className="w-4 h-4 animate-spin" />
            Loading yield data…
          </div>
        )}
        {isError && (
          <div className="flex flex-col items-center justify-center h-full gap-2 text-red-400 text-sm">
            <AlertCircle className="w-5 h-5" />
            <span>{(error as Error)?.message ?? 'Failed to load yield curves'}</span>
          </div>
        )}
        {!isLoading && !isError && data && data.length > 0 && (
          activeTab === 'historical'
            ? <HistoricalChart series={data.filter((s) => enabledSeries.has(s.series_id as SeriesID))} />
            : <SnapshotChart series={data} />
        )}
        {!isLoading && !isError && data && data.length === 0 && (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
            No yield data returned for the selected range.
          </div>
        )}
      </div>
    </div>
  )
}
```

**Step 2: Verify TypeScript compiles**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 3: Commit**

```bash
git add frontend/src/components/widgets/YieldCurveWidget.tsx
git commit -m "feat(widgets): add YieldCurveWidget with Historical and Snapshot recharts views"
```

---

## Task F7: Wire YieldCurveWidget into WidgetRegistry

**Files:**
- Modify: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Add the import** after the Phase A imports block and before the `// Stub for future phases` comment:

```typescript
// Phase F — real widgets
import { YieldCurveWidget } from '@/components/widgets/YieldCurveWidget'
```

**Step 2: Replace the YCRV stub entry** in `WIDGET_REGISTRY`:

Find this line:
```typescript
  YCRV: () => <ComingSoon label="Yield Curves (Phase F)" />,
```

Replace it with:
```typescript
  YCRV: YieldCurveWidget,
```

**Step 3: Verify TypeScript compiles**

```bash
docker compose exec frontend npx tsc --noEmit
```

Expected: no errors.

**Step 4: Smoke test in browser**

```bash
make up
# Navigate to http://localhost:5173/terminal
# In the command bar, type: YCRV  (no ticker needed for yield curves)
# Press Enter — a new panel should open showing the YieldCurveWidget
# Verify:
#   - "US TREASURY YIELD CURVES" header is visible
#   - Historical tab is active by default
#   - Loading spinner appears, then the recharts line chart renders
#   - 2Y/5Y/10Y/30Y toggle buttons are shown
#   - Range buttons 1M/3M/1Y/5Y are shown
#   - Clicking "Snapshot" tab renders the curve shape chart
```

**Step 5: Commit**

```bash
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(terminal): wire YieldCurveWidget into WidgetRegistry — replaces YCRV stub"
```

---

## Task F8: End-to-end integration test

This task verifies the full stack works together.

**Step 1: Verify backend route is reachable and returns correct shape**

```bash
# Get a valid JWT first (register or login):
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"your@email.com","password":"yourpassword"}' | jq -r '.access_token')

# Fetch all 4 series for a 3-month window
curl -s "http://localhost:8080/api/v1/yield-curves?series=DGS2,DGS5,DGS10,DGS30&from=2024-01-01&to=2024-03-31" \
  -H "Authorization: Bearer $TOKEN" | jq 'map({series_id, label, points: (.data | length)})'
```

Expected (with valid FRED API key):
```json
[
  { "series_id": "DGS2",  "label": "2-Year",  "points": 63 },
  { "series_id": "DGS5",  "label": "5-Year",  "points": 63 },
  { "series_id": "DGS10", "label": "10-Year", "points": 63 },
  { "series_id": "DGS30", "label": "30-Year", "points": 63 }
]
```

**Step 2: Verify Redis cache is populated**

```bash
docker compose exec redis redis-cli KEYS "yield-curves:*"
```

Expected: one or more keys matching `yield-curves:<hash>:<from>:<to>`.

```bash
docker compose exec redis redis-cli TTL "yield-curves:<key-from-above>"
```

Expected: a value between 1 and 14400 (4 hours = 14400 seconds).

**Step 3: Verify cache HIT on second request**

```bash
curl -si "http://localhost:8080/api/v1/yield-curves?series=DGS2,DGS5,DGS10,DGS30&from=2024-01-01&to=2024-03-31" \
  -H "Authorization: Bearer $TOKEN" | grep "X-Cache"
```

Expected: `X-Cache: HIT`

**Step 4: Verify frontend widget renders**

Open a browser at `http://localhost:5173/terminal`.

- Open a `YCRV` panel from the command bar.
- Confirm the Historical tab loads a multi-line recharts chart.
- Toggle off the 2Y series — its line should disappear.
- Switch to the Snapshot tab — a single curve line should appear with 4 data points.
- Change the date range to 5Y — a fresh API call should be made (cache MISS for new params).
- Check the browser DevTools Network tab — the request URL should be `GET /api/v1/yield-curves?series=...`.

**Step 5: Verify graceful error state**

```bash
# Temporarily remove FRED key from .env and restart backend
# FRED_API_KEY=   (leave blank)
docker compose restart backend
```

Expected in the widget: the `AlertCircle` error UI shows "FRED_API_KEY is not configured".

Restore the key and restart:
```bash
docker compose restart backend
```

**Step 6: Commit**

No new code in this task — it is verification only. If any fixes were required during verification, commit them now with:

```bash
git add <fixed files>
git commit -m "fix(yieldcurve): <describe what was fixed>"
```

---

## Phase F Completion Checklist

### Backend
- [ ] `FRED_API_KEY` env var added to `.env.example` with comment
- [ ] `ExternalAPIsConfig.FREDKey` field added to `backend/internal/config/config.go`
- [ ] `fredAPIKey string` parameter added to `RegisterRoutes` signature
- [ ] `main.go` passes `cfg.ExternalAPIs.FREDKey` to `RegisterRoutes`
- [ ] `backend/internal/api/yieldcurve_handler.go` created with:
  - [ ] `YieldPoint` and `YieldSeries` types defined
  - [ ] Allow-list validation of series IDs (`validSeriesIDs`)
  - [ ] Date format validation (`YYYY-MM-DD`)
  - [ ] Parallel goroutine fetch via `sync.WaitGroup`
  - [ ] Missing value (`"."`) skipped during parse
  - [ ] Redis cache check before fetch (TTL 4 hours)
  - [ ] Redis cache write after successful fetch
  - [ ] `X-Cache: HIT` / `X-Cache: MISS` response headers set
  - [ ] 503 returned when `FRED_API_KEY` is empty
  - [ ] 502 returned when FRED fetch fails
- [ ] `GET /api/v1/yield-curves` registered in `routes.go` under `protected` group
- [ ] `docker compose exec backend go build ./...` passes with zero errors

### Frontend
- [ ] `frontend/src/api/yieldcurves.ts` created with:
  - [ ] `YieldPoint` and `YieldSeries` interfaces
  - [ ] `fetchYieldCurves` function using `apiClient`
  - [ ] `useYieldCurves` React Query hook with 4-hour stale time
- [ ] `frontend/src/components/widgets/YieldCurveWidget.tsx` created with:
  - [ ] `HistoricalChart` sub-component (recharts `LineChart`, one line per series)
  - [ ] `SnapshotChart` sub-component (recharts `LineChart`, maturity on x-axis)
  - [ ] Tab switcher: Historical / Snapshot
  - [ ] Date range buttons: 1M / 3M / 1Y / 5Y (default 1Y)
  - [ ] Series toggle buttons: 2Y / 5Y / 10Y / 30Y (all on by default; minimum 1 active)
  - [ ] Loading spinner state
  - [ ] Error state with `AlertCircle`
  - [ ] Empty state message
  - [ ] Color-coded series using `SERIES_COLORS` map
- [ ] `WidgetRegistry.tsx` updated: `YCRV` stub replaced with `YieldCurveWidget`
- [ ] `npx tsc --noEmit` passes with zero errors

### Integration
- [ ] `GET /api/v1/yield-curves` returns correct `YieldSeries[]` shape
- [ ] Redis key `yield-curves:<hash>:<from>:<to>` is set after first call
- [ ] Second identical request returns `X-Cache: HIT`
- [ ] Widget renders Historical and Snapshot charts in the browser
- [ ] Series toggle and date range buttons work correctly
- [ ] Error UI renders when FRED key is missing
