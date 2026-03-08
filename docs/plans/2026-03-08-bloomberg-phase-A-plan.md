# Bloomberg Terminal — Phase A: Terminal Foundation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Bloomberg Terminal-style multi-panel workspace at `/terminal` alongside the existing app. After this phase, users can switch between the old trading workbench and the new Bloomberg panel viewer.

**Architecture:** The existing Layout.tsx and all current routes (`/`, `/chart`, `/backtest`, etc.) are untouched. A new `/terminal` route is added, served by WorkspaceLayout. A "Terminal" link is added to the existing CommandBar sidebar nav. react-grid-layout drives the panel grid. Workspace layouts persisted per-user in the backend DB.

**Two apps, one codebase:**
- `/` → Old app (Layout.tsx) — trading workbench: backtest, strategy design, alerts, portfolio, monitors
- `/terminal` → Bloomberg workspace (WorkspaceLayout) — research & market viewer: panels, charts, data, fundamentals, screener

**Tech Stack:** react-grid-layout, Zustand (workspaceStore), React Query, Go/Fiber (workspace handler), GORM (Workspace model)

---

## Task A1: Install react-grid-layout

**Files:**
- Modify: `frontend/package.json`

**Step 1: Install the package**

Run inside the frontend container or locally:
```bash
docker compose exec frontend npm install react-grid-layout
docker compose exec frontend npm install --save-dev @types/react-grid-layout
```

**Step 2: Verify install**

```bash
docker compose exec frontend npm ls react-grid-layout
```
Expected: `react-grid-layout@1.x.x`

**Step 3: Commit**
```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore: install react-grid-layout for Bloomberg panel grid"
```

---

## Task A2: Add Workspace model to models.go

**Files:**
- Modify: `backend/internal/models/models.go`

**Step 1: Add the Workspace struct** at the end of `models.go`, before the final closing brace:

```go
// --- Workspace ---

// Workspace stores a user's Bloomberg-style panel layout
type Workspace struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64          `gorm:"not null;index" json:"user_id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	IsTemplate  bool           `gorm:"default:false" json:"is_template"`
	Layout      JSON           `gorm:"type:json" json:"layout"`
	PanelStates JSON           `gorm:"type:json" json:"panel_states"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
```

**Step 2: Verify GORM auto-migrates on startup**

```bash
make up
make health
docker compose logs backend 2>&1 | grep -i "workspace\|automigrate\|error"
```
Expected: no errors, backend healthy.

**Step 3: Confirm table exists**

```bash
make db-shell
# in MySQL:
SHOW TABLES LIKE 'workspaces';
DESCRIBE workspaces;
```
Expected: table with columns id, user_id, name, is_template, layout, panel_states, created_at, updated_at.

**Step 4: Commit**
```bash
git add backend/internal/models/models.go
git commit -m "feat(models): add Workspace model for Bloomberg panel layout persistence"
```

---

## Task A3: Create workspace backend handler

**Files:**
- Create: `backend/internal/api/workspace_handler.go`

**Step 1: Create the handler file**

```go
package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
	"github.com/trader-claude/backend/internal/models"
)

type workspaceHandler struct{ db *gorm.DB }

func newWorkspaceHandler(db *gorm.DB) *workspaceHandler {
	return &workspaceHandler{db: db}
}

func (h *workspaceHandler) list(c *fiber.Ctx) error {
	userID := auth.UserIDFromCtx(c)
	var workspaces []models.Workspace
	if err := h.db.Where("user_id = ?", userID).Order("created_at asc").Find(&workspaces).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch workspaces"})
	}
	return c.JSON(workspaces)
}

func (h *workspaceHandler) get(c *fiber.Ctx) error {
	userID := auth.UserIDFromCtx(c)
	id := c.Params("id")
	var ws models.Workspace
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&ws).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}
	return c.JSON(ws)
}

func (h *workspaceHandler) create(c *fiber.Ctx) error {
	userID := auth.UserIDFromCtx(c)
	var body struct {
		Name        string      `json:"name"`
		Layout      models.JSON `json:"layout"`
		PanelStates models.JSON `json:"panel_states"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	ws := models.Workspace{
		UserID:      userID,
		Name:        body.Name,
		Layout:      body.Layout,
		PanelStates: body.PanelStates,
	}
	if err := h.db.Create(&ws).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create workspace"})
	}
	return c.Status(201).JSON(ws)
}

func (h *workspaceHandler) update(c *fiber.Ctx) error {
	userID := auth.UserIDFromCtx(c)
	id := c.Params("id")
	var ws models.Workspace
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&ws).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}
	var body struct {
		Name        string      `json:"name"`
		Layout      models.JSON `json:"layout"`
		PanelStates models.JSON `json:"panel_states"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name != "" {
		ws.Name = body.Name
	}
	if body.Layout != nil {
		ws.Layout = body.Layout
	}
	if body.PanelStates != nil {
		ws.PanelStates = body.PanelStates
	}
	if err := h.db.Save(&ws).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update workspace"})
	}
	return c.JSON(ws)
}

func (h *workspaceHandler) delete(c *fiber.Ctx) error {
	userID := auth.UserIDFromCtx(c)
	id := c.Params("id")
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Workspace{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to delete workspace"})
	}
	return c.SendStatus(204)
}
```

**Step 2: Check that `auth.UserIDFromCtx` exists**

```bash
grep -r "UserIDFromCtx" backend/internal/auth/
```
If it doesn't exist, find the correct function name:
```bash
grep -r "func.*Ctx" backend/internal/auth/
```
Use whatever function extracts the user ID from the JWT context — adapt the handler accordingly.

**Step 3: Build check**
```bash
docker compose exec backend go build ./...
```
Expected: no errors.

**Step 4: Commit**
```bash
git add backend/internal/api/workspace_handler.go
git commit -m "feat(api): add workspace CRUD handler"
```

---

## Task A4: Register workspace routes

**Files:**
- Modify: `backend/internal/api/routes.go`

**Step 1: Add workspace handler init** in `RegisterRoutes` after the existing handler inits:

```go
wsH := newWorkspaceHandler(db)
```

**Step 2: Add routes** in the `protected` group section:

```go
// Workspaces
protected.Get("/workspaces", wsH.list)
protected.Post("/workspaces", wsH.create)
protected.Get("/workspaces/:id", wsH.get)
protected.Put("/workspaces/:id", wsH.update)
protected.Delete("/workspaces/:id", wsH.delete)
```

**Step 3: Build + smoke test**
```bash
docker compose exec backend go build ./...
# After rebuild, test with auth token:
curl -s http://localhost:8080/api/v1/workspaces \
  -H "Authorization: Bearer <token>" | jq .
```
Expected: `[]` (empty array).

**Step 4: Commit**
```bash
git add backend/internal/api/routes.go
git commit -m "feat(routes): register workspace CRUD routes under /api/v1/workspaces"
```

---

## Task A5: Create terminal TypeScript types

**Files:**
- Create: `frontend/src/types/terminal.ts`

**Step 1: Create the file**

```typescript
// All Bloomberg terminal-specific types
// Shared trading types remain in types/index.ts

export type FunctionCode =
  | 'GP'    // Chart
  | 'HM'    // Heatmap
  | 'FA'    // Fundamentals
  | 'NEWS'  // News
  | 'PORT'  // Portfolio
  | 'WL'   // Watchlist
  | 'SCR'  // Screener
  | 'CAL'  // Calendar
  | 'OPT'  // Options Chain
  | 'YCRV' // Yield Curves
  | 'RISK' // Risk Analytics
  | 'BT'   // Backtest
  | 'ALRT' // Alerts
  | 'MON'  // Monitor
  | 'AI'   // AI Chat

export type LinkGroup = 'red' | 'blue' | 'green' | 'yellow' | null

export interface PanelConfig {
  id: string
  functionCode: FunctionCode
  ticker: string
  market?: string
  timeframe?: string
  params?: Record<string, unknown>
  linkGroup?: LinkGroup
  maximized?: boolean
}

// react-grid-layout grid item shape
export interface GridItem {
  i: string   // panel id
  x: number
  y: number
  w: number
  h: number
  minW?: number
  minH?: number
}

export interface WorkspaceConfig {
  id?: number
  name: string
  layout: GridItem[]
  panels: Record<string, PanelConfig>  // panelId → PanelConfig
}

// Props contract every widget component must satisfy
export interface WidgetProps {
  ticker: string
  market?: string
  timeframe?: string
  params?: Record<string, unknown>
}

// Command bar autocomplete suggestion
export interface CommandSuggestion {
  type: 'ticker' | 'function'
  value: string
  label: string
  description?: string
}

export const FUNCTION_META: Record<FunctionCode, { label: string; description: string }> = {
  GP:   { label: 'Chart',          description: 'Candlestick chart with indicators' },
  HM:   { label: 'Heatmap',        description: 'Market heatmap by asset class' },
  FA:   { label: 'Fundamentals',   description: 'P/E, EPS, revenue, balance sheet' },
  NEWS: { label: 'News',           description: 'Asset-specific news feed' },
  PORT: { label: 'Portfolio',      description: 'Positions, PnL, transactions' },
  WL:   { label: 'Watchlist',      description: 'Multi-column watchlist' },
  SCR:  { label: 'Screener',       description: 'Filter by any metric' },
  CAL:  { label: 'Calendar',       description: 'Earnings & macro events' },
  OPT:  { label: 'Options Chain',  description: 'Put/call chain with Greeks' },
  YCRV: { label: 'Yield Curves',   description: 'US Treasury yield curves' },
  RISK: { label: 'Risk Analytics', description: 'VaR, Sharpe, stress tests' },
  BT:   { label: 'Backtest',       description: 'Strategy backtest runner' },
  ALRT: { label: 'Alerts',         description: 'Price & volume alert rules' },
  MON:  { label: 'Monitor',        description: 'Live strategy monitor' },
  AI:   { label: 'AI Chat',        description: 'AI assistant panel' },
}
```

**Step 2: Commit**
```bash
git add frontend/src/types/terminal.ts
git commit -m "feat(types): add Bloomberg terminal types — FunctionCode, PanelConfig, WorkspaceConfig, WidgetProps"
```

---

## Task A6: Create workspaceStore

**Files:**
- Create: `frontend/src/stores/workspaceStore.ts`

**Step 1: Create the file**

```typescript
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { WorkspaceConfig, PanelConfig, GridItem, FunctionCode } from '@/types/terminal'

// ── Default workspace templates ────────────────────────────────────────────

const makePanel = (id: string, fn: FunctionCode, ticker: string, market = '', timeframe = '1h'): PanelConfig => ({
  id, functionCode: fn, ticker, market, timeframe, linkGroup: null,
})

export const WORKSPACE_TEMPLATES: WorkspaceConfig[] = [
  {
    name: 'Market Overview',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 7, h: 14, minW: 3, minH: 4 },
      { i: 'p2', x: 7, y: 0, w: 5, h: 7,  minW: 3, minH: 4 },
      { i: 'p3', x: 7, y: 7, w: 5, h: 7,  minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'HM',   '',        ''),
      p2: makePanel('p2', 'NEWS', '',        ''),
      p3: makePanel('p3', 'WL',  '',        ''),
    },
  },
  {
    name: 'Trader',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 8, h: 10, minW: 3, minH: 4 },
      { i: 'p2', x: 8, y: 0, w: 4, h: 10, minW: 3, minH: 4 },
      { i: 'p3', x: 0, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
      { i: 'p4', x: 6, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'GP',   'BTCUSDT', 'binance', '1h'),
      p2: makePanel('p2', 'NEWS', '',        ''),
      p3: makePanel('p3', 'PORT', '',        ''),
      p4: makePanel('p4', 'ALRT', '',        ''),
    },
  },
  {
    name: 'Crypto',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 6, h: 10, minW: 3, minH: 4 },
      { i: 'p2', x: 6, y: 0, w: 6, h: 10, minW: 3, minH: 4 },
      { i: 'p3', x: 0, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
      { i: 'p4', x: 6, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'GP', 'BTCUSDT', 'binance', '1h'),
      p2: makePanel('p2', 'GP', 'ETHUSDT', 'binance', '1h'),
      p3: makePanel('p3', 'NEWS', '', ''),
      p4: makePanel('p4', 'WL',  '', ''),
    },
  },
  {
    name: 'Analyst',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 6, h: 12, minW: 3, minH: 4 },
      { i: 'p2', x: 6, y: 0, w: 6, h: 12, minW: 3, minH: 4 },
      { i: 'p3', x: 0, y: 12, w: 12, h: 8, minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'FA',  'AAPL', 'yahoo'),
      p2: makePanel('p2', 'CAL', '',     ''),
      p3: makePanel('p3', 'SCR', '',     ''),
    },
  },
]

// ── Store ──────────────────────────────────────────────────────────────────

interface WorkspaceStore {
  workspaces: WorkspaceConfig[]
  activeIndex: number
  activePanelId: string | null

  // Panel focus
  setActivePanel: (id: string | null) => void

  // Workspace tabs
  setActiveWorkspace: (index: number) => void
  addWorkspace: (name?: string) => void
  removeWorkspace: (index: number) => void
  renameWorkspace: (index: number, name: string) => void

  // Layout persistence
  updateLayout: (layout: GridItem[]) => void

  // Panel CRUD
  addPanel: (config: Omit<PanelConfig, 'id'>) => string  // returns new panel id
  updatePanel: (panelId: string, update: Partial<PanelConfig>) => void
  removePanel: (panelId: string) => void

  // Sync from server
  loadServerWorkspaces: (workspaces: WorkspaceConfig[]) => void
}

export const useWorkspaceStore = create<WorkspaceStore>()(
  persist(
    (set, get) => ({
      workspaces: WORKSPACE_TEMPLATES,
      activeIndex: 0,
      activePanelId: null,

      setActivePanel: (id) => set({ activePanelId: id }),

      setActiveWorkspace: (index) => set({ activeIndex: index }),

      addWorkspace: (name = 'New Workspace') =>
        set((s) => ({
          workspaces: [...s.workspaces, { name, layout: [], panels: {} }],
          activeIndex: s.workspaces.length,
        })),

      removeWorkspace: (index) =>
        set((s) => {
          if (s.workspaces.length <= 1) return s
          const next = s.workspaces.filter((_, i) => i !== index)
          return { workspaces: next, activeIndex: Math.min(s.activeIndex, next.length - 1) }
        }),

      renameWorkspace: (index, name) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          workspaces[index] = { ...workspaces[index], name }
          return { workspaces }
        }),

      updateLayout: (layout) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          workspaces[s.activeIndex] = { ...workspaces[s.activeIndex], layout }
          return { workspaces }
        }),

      addPanel: (config) => {
        const id = crypto.randomUUID()
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          const newPanel: PanelConfig = { ...config, id }
          // Place new panel at bottom-left by default
          const maxY = ws.layout.reduce((m, item) => Math.max(m, item.y + item.h), 0)
          workspaces[s.activeIndex] = {
            ...ws,
            layout: [...ws.layout, { i: id, x: 0, y: maxY, w: 6, h: 10, minW: 3, minH: 4 }],
            panels: { ...ws.panels, [id]: newPanel },
          }
          return { workspaces }
        })
        return id
      },

      updatePanel: (panelId, update) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          workspaces[s.activeIndex] = {
            ...ws,
            panels: { ...ws.panels, [panelId]: { ...ws.panels[panelId], ...update } },
          }
          return { workspaces }
        }),

      removePanel: (panelId) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          const { [panelId]: _, ...panels } = ws.panels
          workspaces[s.activeIndex] = {
            ...ws,
            layout: ws.layout.filter((item) => item.i !== panelId),
            panels,
          }
          return { workspaces }
        }),

      loadServerWorkspaces: (serverWorkspaces) =>
        set({ workspaces: serverWorkspaces.length > 0 ? serverWorkspaces : WORKSPACE_TEMPLATES }),
    }),
    { name: 'trader-workspaces' },
  ),
)
```

**Step 2: Commit**
```bash
git add frontend/src/stores/workspaceStore.ts
git commit -m "feat(store): add workspaceStore with panel CRUD, workspace tabs, default templates"
```

---

## Task A7: Create terminal API client

**Files:**
- Create: `frontend/src/api/terminal.ts`

**Step 1: Create the file**

```typescript
import apiClient from '@/api/client'
import type { WorkspaceConfig } from '@/types/terminal'

// Shape returned by backend (layout + panel_states are JSON objects)
interface ServerWorkspace {
  id: number
  name: string
  is_template: boolean
  layout: WorkspaceConfig['layout']
  panel_states: WorkspaceConfig['panels']
  created_at: string
  updated_at: string
}

function toLocal(sw: ServerWorkspace): WorkspaceConfig {
  return {
    id: sw.id,
    name: sw.name,
    layout: sw.layout ?? [],
    panels: sw.panel_states ?? {},
  }
}

export async function fetchWorkspaces(): Promise<WorkspaceConfig[]> {
  const { data } = await apiClient.get<ServerWorkspace[]>('/workspaces')
  return data.map(toLocal)
}

export async function createWorkspace(ws: WorkspaceConfig): Promise<WorkspaceConfig> {
  const { data } = await apiClient.post<ServerWorkspace>('/workspaces', {
    name: ws.name,
    layout: ws.layout,
    panel_states: ws.panels,
  })
  return toLocal(data)
}

export async function updateWorkspace(id: number, ws: WorkspaceConfig): Promise<WorkspaceConfig> {
  const { data } = await apiClient.put<ServerWorkspace>(`/workspaces/${id}`, {
    name: ws.name,
    layout: ws.layout,
    panel_states: ws.panels,
  })
  return toLocal(data)
}

export async function deleteWorkspace(id: number): Promise<void> {
  await apiClient.delete(`/workspaces/${id}`)
}
```

**Step 2: Commit**
```bash
git add frontend/src/api/terminal.ts
git commit -m "feat(api): add terminal API client for workspace CRUD"
```

---

## Task A8: Create WidgetRegistry

**Files:**
- Create: `frontend/src/components/terminal/WidgetRegistry.tsx`

**Step 1: Create the file** (stub widgets for phases B-H; real widgets for A):

```tsx
import React from 'react'
import type { FunctionCode, WidgetProps } from '@/types/terminal'

// Phase A — real widgets (adapted wrappers created in Task A13)
import { ChartWidget }     from '@/components/widgets/ChartWidget'
import { NewsWidget }      from '@/components/widgets/NewsWidget'
import { PortfolioWidget } from '@/components/widgets/PortfolioWidget'
import { WatchlistWidget } from '@/components/widgets/WatchlistWidget'
import { AlertsWidget }    from '@/components/widgets/AlertsWidget'
import { BacktestWidget }  from '@/components/widgets/BacktestWidget'
import { MonitorWidget }   from '@/components/widgets/MonitorWidget'
import { AIChatWidget }    from '@/components/widgets/AIChatWidget'

// Stub for future phases
const ComingSoon = ({ label }: { label: string }) => (
  <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
    {label} — coming soon
  </div>
)

// Bloomberg terminal focuses on research/market viewing.
// BT, MON, ALRT are read-only summary views here — full management stays in the old app.
export const WIDGET_REGISTRY: Record<FunctionCode, React.ComponentType<WidgetProps>> = {
  // ── Phase A: research & market widgets ──
  GP:   ChartWidget,       // candlestick chart
  NEWS: NewsWidget,        // news feed
  PORT: PortfolioWidget,   // read-only portfolio summary
  WL:   WatchlistWidget,   // watchlist

  // ── Read-only views of workbench tools (link back to old app for full control) ──
  ALRT: AlertsWidget,
  BT:   BacktestWidget,
  MON:  MonitorWidget,
  AI:   AIChatWidget,

  // ── Phases B-H: new research widgets (stubbed for now) ──
  HM:   () => <ComingSoon label="Market Heatmap (Phase B)" />,
  FA:   () => <ComingSoon label="Fundamentals (Phase D)" />,
  SCR:  () => <ComingSoon label="Screener (Phase C)" />,
  CAL:  () => <ComingSoon label="Calendar (Phase E)" />,
  OPT:  () => <ComingSoon label="Options Chain (Phase G)" />,
  YCRV: () => <ComingSoon label="Yield Curves (Phase F)" />,
  RISK: () => <ComingSoon label="Risk Analytics (Phase H)" />,
}

export function getWidget(code: FunctionCode): React.ComponentType<WidgetProps> {
  return WIDGET_REGISTRY[code] ?? (() => <ComingSoon label={code} />)
}
```

**Step 2: Commit** (after widgets are created in A13 — come back and commit together)

---

## Task A9: Create PanelSlot (panel container)

**Files:**
- Create: `frontend/src/components/terminal/PanelSlot.tsx`

**Step 1: Create the file**

```tsx
import { useState } from 'react'
import { X, Maximize2, Minimize2, Link } from 'lucide-react'
import { cn } from '@/lib/utils'
import { FUNCTION_META } from '@/types/terminal'
import { getWidget } from './WidgetRegistry'
import type { PanelConfig, LinkGroup } from '@/types/terminal'

const LINK_COLORS: Record<NonNullable<LinkGroup>, string> = {
  red:    'bg-red-500',
  blue:   'bg-blue-500',
  green:  'bg-green-500',
  yellow: 'bg-yellow-500',
}

interface PanelSlotProps {
  config: PanelConfig
  isActive: boolean
  onFocus: () => void
  onClose: () => void
  onUpdate: (update: Partial<PanelConfig>) => void
}

export function PanelSlot({ config, isActive, onFocus, onClose, onUpdate }: PanelSlotProps) {
  const [maximized, setMaximized] = useState(false)
  const meta = FUNCTION_META[config.functionCode]
  const Widget = getWidget(config.functionCode)

  return (
    <div
      className={cn(
        'flex flex-col h-full rounded border bg-background overflow-hidden',
        isActive ? 'border-blue-500/60' : 'border-border',
        maximized && 'fixed inset-2 z-50 shadow-2xl',
      )}
      onClick={onFocus}
    >
      {/* Header */}
      <div className="flex items-center gap-1.5 px-2 py-1 border-b border-border bg-muted/40 shrink-0">
        {/* Link group indicator */}
        {config.linkGroup && (
          <span className={cn('w-2 h-2 rounded-full shrink-0', LINK_COLORS[config.linkGroup])} />
        )}

        {/* Ticker badge */}
        {config.ticker && (
          <span className="text-xs font-mono font-semibold text-foreground bg-muted px-1.5 py-0.5 rounded">
            {config.ticker}
          </span>
        )}

        {/* Function label */}
        <span className="text-xs text-muted-foreground">{meta.label}</span>

        <div className="ml-auto flex items-center gap-1">
          <button
            className="text-muted-foreground hover:text-foreground p-0.5 rounded"
            onClick={(e) => { e.stopPropagation(); setMaximized((m) => !m) }}
            title={maximized ? 'Restore' : 'Maximize'}
          >
            {maximized ? <Minimize2 size={12} /> : <Maximize2 size={12} />}
          </button>
          <button
            className="text-muted-foreground hover:text-red-500 p-0.5 rounded"
            onClick={(e) => { e.stopPropagation(); onClose() }}
            title="Close panel"
          >
            <X size={12} />
          </button>
        </div>
      </div>

      {/* Widget content */}
      <div className="flex-1 overflow-hidden">
        <Widget
          ticker={config.ticker}
          market={config.market}
          timeframe={config.timeframe}
          params={config.params}
        />
      </div>
    </div>
  )
}
```

**Step 2: Commit** (together with PanelGrid in A10)

---

## Task A10: Create PanelGrid

**Files:**
- Create: `frontend/src/components/terminal/PanelGrid.tsx`

**Step 1: Create the file**

```tsx
import { useCallback } from 'react'
import GridLayout from 'react-grid-layout'
import 'react-grid-layout/css/styles.css'
import 'react-resizable/css/styles.css'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { PanelSlot } from './PanelSlot'
import type { GridItem } from '@/types/terminal'

const COLS = 12
const ROW_HEIGHT = 30

export function PanelGrid() {
  const workspaces   = useWorkspaceStore((s) => s.workspaces)
  const activeIndex  = useWorkspaceStore((s) => s.activeIndex)
  const activePanelId = useWorkspaceStore((s) => s.activePanelId)
  const updateLayout = useWorkspaceStore((s) => s.updateLayout)
  const removePanel  = useWorkspaceStore((s) => s.removePanel)
  const updatePanel  = useWorkspaceStore((s) => s.updatePanel)
  const setActive    = useWorkspaceStore((s) => s.setActivePanel)

  const ws = workspaces[activeIndex]
  if (!ws) return null

  const onLayoutChange = useCallback(
    (layout: GridItem[]) => updateLayout(layout),
    [updateLayout],
  )

  return (
    <GridLayout
      className="w-full"
      layout={ws.layout}
      cols={COLS}
      rowHeight={ROW_HEIGHT}
      width={window.innerWidth}
      onLayoutChange={onLayoutChange}
      draggableHandle=".drag-handle"
      resizeHandles={['se']}
      margin={[4, 4]}
    >
      {ws.layout.map((item) => {
        const config = ws.panels[item.i]
        if (!config) return null
        return (
          <div key={item.i} className="drag-handle">
            <PanelSlot
              config={config}
              isActive={activePanelId === item.i}
              onFocus={() => setActive(item.i)}
              onClose={() => removePanel(item.i)}
              onUpdate={(update) => updatePanel(item.i, update)}
            />
          </div>
        )
      })}
    </GridLayout>
  )
}
```

**Step 2: Commit** (together with PanelSlot)
```bash
git add frontend/src/components/terminal/PanelSlot.tsx frontend/src/components/terminal/PanelGrid.tsx
git commit -m "feat(terminal): add PanelSlot and PanelGrid components"
```

---

## Task A11: Create WorkspaceTabs

**Files:**
- Create: `frontend/src/components/terminal/WorkspaceTabs.tsx`

**Step 1: Create the file**

```tsx
import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useWorkspaceStore } from '@/stores/workspaceStore'

export function WorkspaceTabs() {
  const { workspaces, activeIndex, setActiveWorkspace, addWorkspace, removeWorkspace, renameWorkspace } = useWorkspaceStore()
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [editValue, setEditValue] = useState('')

  return (
    <div className="flex items-center gap-1 px-2 overflow-x-auto shrink-0 border-b border-border bg-muted/30">
      {workspaces.map((ws, i) => (
        <div
          key={i}
          className={cn(
            'flex items-center gap-1 px-3 py-1.5 text-xs rounded-t cursor-pointer select-none shrink-0',
            i === activeIndex
              ? 'bg-background border border-b-background border-border text-foreground font-medium'
              : 'text-muted-foreground hover:text-foreground',
          )}
          onClick={() => setActiveWorkspace(i)}
          onDoubleClick={() => { setEditingIndex(i); setEditValue(ws.name) }}
        >
          {editingIndex === i ? (
            <input
              autoFocus
              className="bg-transparent outline-none w-24 text-xs"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onBlur={() => { renameWorkspace(i, editValue || ws.name); setEditingIndex(null) }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') { renameWorkspace(i, editValue || ws.name); setEditingIndex(null) }
                if (e.key === 'Escape') setEditingIndex(null)
              }}
              onClick={(e) => e.stopPropagation()}
            />
          ) : (
            <span>{ws.name}</span>
          )}
          {workspaces.length > 1 && (
            <button
              className="text-muted-foreground hover:text-red-500 ml-1"
              onClick={(e) => { e.stopPropagation(); removeWorkspace(i) }}
            >
              <X size={10} />
            </button>
          )}
        </div>
      ))}
      <button
        className="p-1.5 text-muted-foreground hover:text-foreground shrink-0"
        onClick={() => addWorkspace()}
        title="New workspace"
      >
        <Plus size={14} />
      </button>
    </div>
  )
}
```

**Step 2: Commit**
```bash
git add frontend/src/components/terminal/WorkspaceTabs.tsx
git commit -m "feat(terminal): add WorkspaceTabs component with rename and add/remove"
```

---

## Task A12: Create Bloomberg-style CommandBar

**Files:**
- Create: `frontend/src/components/terminal/CommandBar.tsx`
- Note: This replaces the existing `components/layout/CommandBar.tsx` (keep old file, new one is in `components/terminal/`)

**Step 1: Create the file**

```tsx
import { useState, useRef, useEffect, useCallback } from 'react'
import { Terminal } from 'lucide-react'
import { cn } from '@/lib/utils'
import { FUNCTION_META, type FunctionCode, type CommandSuggestion } from '@/types/terminal'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useMarketStore } from '@/stores'

const FUNCTION_CODES = Object.keys(FUNCTION_META) as FunctionCode[]

export function CommandBar() {
  const [input, setInput] = useState('')
  const [suggestions, setSuggestions] = useState<CommandSuggestion[]>([])
  const [selectedIdx, setSelectedIdx] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const addPanel = useWorkspaceStore((s) => s.addPanel)
  const symbols  = useMarketStore((s) => s.symbols)

  const buildSuggestions = useCallback((value: string) => {
    const parts = value.trim().toUpperCase().split(/\s+/)
    const tickerPart = parts[0] ?? ''
    const fnPart     = parts[1] ?? ''

    if (!tickerPart) { setSuggestions([]); return }

    // If we have both ticker and start of function code
    if (tickerPart && fnPart !== undefined) {
      const fnSuggestions = FUNCTION_CODES
        .filter((code) => code.startsWith(fnPart))
        .slice(0, 8)
        .map((code) => ({
          type: 'function' as const,
          value: `${tickerPart} ${code}`,
          label: code,
          description: FUNCTION_META[code].description,
        }))
      setSuggestions(fnSuggestions)
      return
    }

    // Ticker suggestions only
    const tickerSuggestions = symbols
      .filter((s) => s.symbol.toUpperCase().startsWith(tickerPart))
      .slice(0, 6)
      .map((s) => ({
        type: 'ticker' as const,
        value: s.symbol,
        label: s.symbol,
        description: s.name ?? '',
      }))
    setSuggestions(tickerSuggestions)
  }, [symbols])

  useEffect(() => {
    buildSuggestions(input)
    setSelectedIdx(0)
  }, [input, buildSuggestions])

  const execute = useCallback((command: string) => {
    const parts = command.trim().toUpperCase().split(/\s+/)
    const ticker = parts[0] ?? ''
    const fnCode = parts[1] as FunctionCode | undefined

    if (!ticker || !fnCode || !FUNCTION_META[fnCode]) return

    addPanel({ functionCode: fnCode, ticker, market: '', timeframe: '1h' })
    setInput('')
    setSuggestions([])
  }, [addPanel])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') { setSelectedIdx((i) => Math.min(i + 1, suggestions.length - 1)); e.preventDefault() }
    if (e.key === 'ArrowUp')   { setSelectedIdx((i) => Math.max(i - 1, 0)); e.preventDefault() }
    if (e.key === 'Enter') {
      const cmd = suggestions[selectedIdx]?.value ?? input
      execute(cmd)
      e.preventDefault()
    }
    if (e.key === 'Escape') { setSuggestions([]); setInput('') }
  }

  return (
    <div className="relative flex items-center gap-2 px-3 py-1.5 border-b border-border bg-background">
      <Terminal size={14} className="text-muted-foreground shrink-0" />

      <div className="flex-1 relative">
        <input
          ref={inputRef}
          className="w-full bg-transparent text-sm font-mono outline-none placeholder:text-muted-foreground/50"
          placeholder="BTC GP — type ticker + function (e.g. AAPL FA, BTC NEWS)"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKeyDown}
          spellCheck={false}
          autoComplete="off"
        />

        {suggestions.length > 0 && (
          <div className="absolute top-full left-0 z-50 mt-1 w-96 rounded-md border border-border bg-popover shadow-lg overflow-hidden">
            {suggestions.map((s, i) => (
              <div
                key={s.value}
                className={cn(
                  'flex items-center gap-3 px-3 py-2 cursor-pointer text-sm',
                  i === selectedIdx ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50',
                )}
                onClick={() => execute(s.value)}
              >
                <span className="font-mono font-semibold w-16 shrink-0">{s.label}</span>
                <span className="text-muted-foreground text-xs truncate">{s.description}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
```

**Step 2: Commit**
```bash
git add frontend/src/components/terminal/CommandBar.tsx
git commit -m "feat(terminal): add Bloomberg-style CommandBar with ticker+function autocomplete"
```

---

## Task A13: Create 8 widget wrappers

**Files:**
- Create: `frontend/src/components/widgets/ChartWidget.tsx`
- Create: `frontend/src/components/widgets/NewsWidget.tsx`
- Create: `frontend/src/components/widgets/PortfolioWidget.tsx`
- Create: `frontend/src/components/widgets/WatchlistWidget.tsx`
- Create: `frontend/src/components/widgets/AlertsWidget.tsx`
- Create: `frontend/src/components/widgets/BacktestWidget.tsx`
- Create: `frontend/src/components/widgets/MonitorWidget.tsx`
- Create: `frontend/src/components/widgets/AIChatWidget.tsx`

**Step 1: ChartWidget.tsx**
```tsx
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import type { WidgetProps } from '@/types/terminal'

export function ChartWidget({ ticker, market, timeframe }: WidgetProps) {
  return <CandlestickChart symbol={ticker} market={market ?? 'binance'} timeframe={timeframe ?? '1h'} />
}
```

Note: Check `CandlestickChart`'s actual prop names at `frontend/src/components/chart/CandlestickChart.tsx` and adapt accordingly.

**Step 2: NewsWidget.tsx**
```tsx
import { NewsFeedPanel } from '@/components/dashboard/NewsFeedPanel'
import type { WidgetProps } from '@/types/terminal'

export function NewsWidget({ ticker }: WidgetProps) {
  return <NewsFeedPanel symbol={ticker || undefined} />
}
```

Check `NewsFeedPanel`'s actual props and adapt.

**Step 3: PortfolioWidget.tsx**
```tsx
import { Portfolio } from '@/pages/Portfolio'
import type { WidgetProps } from '@/types/terminal'

// Embed the Portfolio page directly — it manages its own state
export function PortfolioWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto">
      <Portfolio />
    </div>
  )
}
```

**Step 4: WatchlistWidget.tsx**
```tsx
import { WatchlistPanel } from '@/components/dashboard/WatchlistPanel'
import type { WidgetProps } from '@/types/terminal'

export function WatchlistWidget(_: WidgetProps) {
  return <WatchlistPanel />
}
```

**Step 5: AlertsWidget.tsx**
```tsx
import { Alerts } from '@/pages/Alerts'
import type { WidgetProps } from '@/types/terminal'

export function AlertsWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto p-3">
      <Alerts />
    </div>
  )
}
```

**Step 6: BacktestWidget.tsx**
```tsx
import { Backtest } from '@/pages/Backtest'
import type { WidgetProps } from '@/types/terminal'

export function BacktestWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto">
      <Backtest />
    </div>
  )
}
```

**Step 7: MonitorWidget.tsx**
```tsx
import { Monitor } from '@/pages/Monitor'
import type { WidgetProps } from '@/types/terminal'

export function MonitorWidget(_: WidgetProps) {
  return (
    <div className="h-full overflow-y-auto">
      <Monitor />
    </div>
  )
}
```

**Step 8: AIChatWidget.tsx**
```tsx
import { ChatPanel } from '@/components/ai/ChatPanel'
import type { WidgetProps } from '@/types/terminal'

export function AIChatWidget({ ticker }: WidgetProps) {
  // Pass ticker as context to the AI so it knows what asset the user is looking at
  return <ChatPanel isOpen={true} onClose={() => {}} contextTicker={ticker} />
}
```

Note: `ChatPanel` may not have a `contextTicker` prop yet. If not, just render without it for now.

**Step 9: Commit all widgets**
```bash
git add frontend/src/components/widgets/
git add frontend/src/components/terminal/WidgetRegistry.tsx
git commit -m "feat(widgets): add 8 panel widget wrappers (Chart, News, Portfolio, Watchlist, Alerts, Backtest, Monitor, AIChat)"
```

---

## Task A14: Create WorkspaceLayout

**Files:**
- Create: `frontend/src/components/terminal/WorkspaceLayout.tsx`

**Step 1: Create the file**

```tsx
import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'
import { CommandBar } from './CommandBar'
import { WorkspaceTabs } from './WorkspaceTabs'
import { PanelGrid } from './PanelGrid'

export function WorkspaceLayout() {
  useNotificationWS()

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <CommandBar />
      <WorkspaceTabs />
      <main className="flex-1 overflow-hidden relative">
        <PanelGrid />
      </main>
      <SignalToast />
    </div>
  )
}
```

**Step 2: Commit**
```bash
git add frontend/src/components/terminal/WorkspaceLayout.tsx
git commit -m "feat(terminal): add WorkspaceLayout root shell"
```

---

## Task A15: Add /terminal route to App.tsx (keep all existing routes)

**Files:**
- Modify: `frontend/src/App.tsx`

**Step 1: Add the `/terminal` route** — existing routes must NOT be touched

```tsx
import { Routes, Route } from 'react-router-dom'
import { useEffect } from 'react'
import { Layout } from '@/components/layout/Layout'                    // existing — unchanged
import { WorkspaceLayout } from '@/components/terminal/WorkspaceLayout' // new
import ProtectedRoute from '@/components/auth/ProtectedRoute'
import { Dashboard }     from '@/pages/Dashboard'
import { Chart }         from '@/pages/Chart'
import { Backtest }      from '@/pages/Backtest'
import { Portfolio }     from '@/pages/Portfolio'
import { Monitor }       from '@/pages/Monitor'
import { News }          from '@/pages/News'
import { Alerts }        from '@/pages/Alerts'
import { Settings }      from '@/pages/Settings'
import { Notifications } from '@/pages/Notifications'
import Login    from '@/pages/Login'
import Register from '@/pages/Register'
import { useAuthStore } from '@/stores/authStore'

export function App() {
  const initialize = useAuthStore((s) => s.initialize)

  useEffect(() => {
    initialize()
  }, [initialize])

  return (
    <Routes>
      {/* Public routes — unchanged */}
      <Route path="/login"    element={<Login />} />
      <Route path="/register" element={<Register />} />

      {/* ── Old app: trading workbench (Layout + sidebar) — ALL UNCHANGED ── */}
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route path="/"             element={<Dashboard />} />
        <Route path="/chart"        element={<Chart />} />
        <Route path="/backtest"     element={<Backtest />} />
        <Route path="/portfolio"    element={<Portfolio />} />
        <Route path="/monitor"      element={<Monitor />} />
        <Route path="/news"         element={<News />} />
        <Route path="/alerts"       element={<Alerts />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/settings"     element={<Settings />} />
      </Route>

      {/* ── Bloomberg terminal: research & market viewer ── */}
      <Route
        path="/terminal"
        element={
          <ProtectedRoute>
            <WorkspaceLayout />
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}
```

**Step 2: Add "Terminal" link to the existing CommandBar nav**

In `frontend/src/components/layout/CommandBar.tsx`, find the `navItems` array and add the terminal entry:

```tsx
import { Monitor as TerminalIcon } from 'lucide-react'  // or use 'LayoutGrid'

const navItems = [
  { to: '/',          icon: LayoutDashboard,  label: 'Dashboard' },
  { to: '/chart',     icon: CandlestickChart, label: 'Chart'     },
  { to: '/backtest',  icon: FlaskConical,     label: 'Backtest'  },
  { to: '/portfolio', icon: Briefcase,        label: 'Portfolio' },
  { to: '/monitor',   icon: Activity,         label: 'Monitor'   },
  { to: '/news',      icon: Newspaper,        label: 'News'      },
  { to: '/alerts',    icon: Bell,             label: 'Alerts'    },
  { to: '/settings',  icon: Settings,         label: 'Settings'  },
  { to: '/terminal',  icon: LayoutGrid,       label: 'Terminal'  },  // ← ADD THIS
]
```

Import `LayoutGrid` from `lucide-react` at the top of the file (it's already in the package).

**Step 3: Verify both apps work**
```bash
make up
# Navigate to http://localhost:5173 → old app (Dashboard) loads normally
# Click "Terminal" in sidebar → /terminal loads Bloomberg workspace
# Navigate back to / → old app still works perfectly
# All existing routes (/chart, /backtest, etc.) still work
```

**Step 4: Commit**
```bash
git add frontend/src/App.tsx frontend/src/components/layout/CommandBar.tsx
git commit -m "feat(app): add /terminal Bloomberg workspace alongside existing trading workbench"
```

---

## Task A16: Smoke test Phase A end-to-end

**Step 1: Start the stack**
```bash
make up && make health
```

**Step 2: Login and verify terminal loads**
- Navigate to `http://localhost:5173`
- Login — should redirect to `/terminal`
- Expected: command bar at top, workspace tabs below it, panel grid fills rest of screen

**Step 3: Test command bar**
- Type `BTC GP` → press Enter
- Expected: new GP panel appears with BTCUSDT candlestick chart
- Type `AAPL NEWS` → press Shift+Enter (or Enter)
- Expected: NEWS panel appears

**Step 4: Test panel drag/resize**
- Drag a panel by its header → should reposition
- Drag panel corner → should resize

**Step 5: Test workspace tabs**
- Click `+` → new workspace tab created
- Double-click tab name → rename it
- Click X on tab → tab removed

**Step 6: Test panel close**
- Click X in panel header → panel removed from grid

**Step 7: Test workspace persistence**
- Refresh browser
- Expected: panels and layout restored from localStorage

---

## Phase A Completion Checklist

- [ ] `react-grid-layout` installed
- [ ] `Workspace` model in models.go, table auto-created
- [ ] Workspace CRUD API working (`/api/v1/workspaces`)
- [ ] `types/terminal.ts` with all 15 function codes + WidgetProps
- [ ] `workspaceStore.ts` with 4 default templates
- [ ] `api/terminal.ts` workspace API client
- [ ] `WidgetRegistry.tsx` mapping all 15 codes (8 real, 7 stub)
- [ ] `PanelSlot.tsx` with header (ticker badge, function label, maximize, close)
- [ ] `PanelGrid.tsx` with drag/resize
- [ ] `WorkspaceTabs.tsx` with add/remove/rename
- [ ] `CommandBar.tsx` with `TICKER FUNCTION` autocomplete
- [ ] `WorkspaceLayout.tsx` assembling all pieces
- [ ] 8 widget wrappers created
- [ ] `App.tsx` routing to `/terminal`, legacy routes redirect
- [ ] App boots and command bar successfully opens panels
