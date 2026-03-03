# UI Redesign Design — 2026-03-03

## Overview

Full redesign of the trader-claude frontend: new design system, top command bar replacing the sidebar, and a Bloomberg Terminal-style single-view dashboard with four always-visible panels. All other pages (Backtest, Monitor, Portfolio, etc.) keep their routing but get the new shell.

---

## 1. Design System & Tokens

### Color Palette (light mode primary, dark toggle supported)

| Token | Value | Usage |
|---|---|---|
| Background | `#f8fafc` (slate-50) | Page canvas |
| Surface | `#ffffff` | Cards, panels |
| Surface-2 | `#f1f5f9` (slate-100) | Inner panel backgrounds, hover states |
| Border | `#e2e8f0` (slate-200) | Dividers |
| Text primary | `#0f172a` (slate-900) | Headings, key numbers |
| Text muted | `#64748b` (slate-500) | Labels, secondary info |
| Accent | `#1e40af` (blue-800) | Active states, CTAs |
| Accent-light | `#eff6ff` (blue-50) | Accent hover backgrounds |
| Green | `#16a34a` (green-600) | Price up, profit |
| Red | `#dc2626` (red-600) | Price down, loss |
| Amber | `#d97706` (amber-600) | Warnings, pending signals |

### CSS Variable Mapping (index.css)

```css
:root {
  --background: 210 40% 98%;        /* slate-50 */
  --foreground: 222 84% 5%;         /* slate-900 */
  --card: 0 0% 100%;                /* white */
  --card-foreground: 222 84% 5%;
  --border: 214 32% 91%;            /* slate-200 */
  --input: 214 32% 91%;
  --primary: 226 71% 40%;           /* blue-800 */
  --primary-foreground: 210 40% 98%;
  --muted: 210 40% 96%;             /* slate-100 */
  --muted-foreground: 215 16% 47%;  /* slate-500 */
  --accent: 210 40% 96%;
  --accent-foreground: 222 84% 5%;
  --destructive: 0 84% 60%;
  --radius: 0.75rem;                /* rounded-xl default */
}
```

### Typography

- **Font**: Inter (system stack fallback)
- **Numbers / prices / P&L**: `font-mono` — clean data feel
- **Panel headers**: `text-xs font-semibold uppercase tracking-wider text-slate-400`
- **Body**: `text-sm text-slate-600`
- **Headings**: `text-sm font-semibold text-slate-900 tracking-tight`

### Radius & Shadows

| Element | Class |
|---|---|
| Page cards / panels | `rounded-2xl shadow-sm border border-slate-100` |
| Inner panels | `rounded-xl` |
| Buttons | `rounded-lg` |
| Inputs / selects | `rounded-lg` |
| Modals | `rounded-2xl shadow-xl` |
| Hover elevation | `hover:shadow-md transition-shadow duration-150` |

---

## 2. Layout & Navigation

### Top Command Bar (52px, replaces sidebar)

```
┌───────────────────────────────────────────────────────────────────────────┐
│  ◈ Trader Claude  │  BTC/USDT ▾  1h ▾  │  Chart  Backtest  Monitor ...  │  🔔 ☀ avatar │
└───────────────────────────────────────────────────────────────────────────┘
```

**Zones:**
1. **Left** — Logo mark + "Trader Claude" wordmark
2. **Center-left** — Global symbol picker (`Combobox`) + timeframe selector (`Select`) — persists across all pages via `marketStore`
3. **Center** — Page tabs: `Chart | Backtest | Monitor | Portfolio | News | Alerts | Settings` — active tab: `border-b-2 border-blue-800 text-blue-800 bg-blue-50/50`
4. **Right** — Notification bell (badge), theme toggle, user avatar dropdown

Bar styles: `bg-white border-b border-slate-200 shadow-sm`

### Dashboard 3-Column Layout

Only the Dashboard route uses this layout. All other pages render full-width below the command bar.

```
┌─────────────────────────────────────────────────────────────────┐
│                     COMMAND BAR (52px)                          │
├──────────────┬────────────────────────────────┬─────────────────┤
│  Left Rail   │       Center (flex-1)          │   Right Rail    │
│   220px      │                                │    280px        │
│              │  ┌─ Chart ──────────────────┐  │  ┌─ News ─────┐ │
│ ┌─ Watch ──┐ │  │                          │  │  │ item 1     │ │
│ │BTC  +2%  │ │  │  CandlestickChart        │  │  │ item 2     │ │
│ │ETH  -1%  │ │  │  (~65% of center height) │  │  └────────────┘ │
│ │AAPL +1%  │ │  └──────────────────────────┘  │  ┌─ Alerts ───┐ │
│ └──────────┘ │  ┌─ Indicator panels ────────┐  │  │ ⚠ signal   │ │
│              │  │  RSI / MACD / etc.         │  │  │ ⚠ alert    │ │
│ ┌─ Port ───┐ │  └──────────────────────────┘  │  └────────────┘ │
│ │$12k +3%  │ │                                │                 │
│ │3 positions│ │                                │                 │
│ └──────────┘ │                                │                 │
└──────────────┴────────────────────────────────┴─────────────────┘
```

**Column details:**
- Left rail (`w-[220px] shrink-0`): Watchlist panel + Portfolio summary panel, stacked vertically with `gap-3`
- Center (`flex-1 min-w-0`): Chart panel (dominant) + indicator sub-panels below, `flex flex-col gap-3`
- Right rail (`w-[280px] shrink-0`): News panel (scrollable) + Alerts panel (scrollable), stacked with `gap-3`

**Panel anatomy:**
```
rounded-2xl bg-white shadow-sm border border-slate-100
  ├── header: px-4 py-3 border-b border-slate-100
  │     label: text-xs font-semibold uppercase tracking-wider text-slate-400
  │     actions: icon buttons (ghost)
  └── body: p-3 (or p-0 for full-bleed charts)
```

### Other Pages

Backtest, Monitor, Portfolio, News, Alerts, Settings render below the command bar with:
```
<main className="flex-1 overflow-y-auto">
  <div className="max-w-7xl mx-auto px-6 py-6">
    <Outlet />
  </div>
</main>
```

---

## 3. Component Specifications

### Form Controls

**Select / Dropdown** (replace raw `<select>`):
- Use shadcn `Select` component (Radix primitive)
- Trigger: `h-8 rounded-lg border-slate-200 bg-white text-sm shadow-sm px-3 hover:border-slate-300 focus:ring-2 focus:ring-blue-500/20 transition`

**Search / Combobox** (symbol picker):
- Trigger pill shows selected symbol or "Search symbol…" in muted text
- Popover with search input + scrollable listbox
- Input: `rounded-lg border-slate-200 pl-9 (search icon inset) focus:ring-2 focus:ring-blue-500/20`

**Timeframe pills:**
- Container: `inline-flex items-center gap-0.5 bg-slate-100 rounded-lg p-0.5`
- Active: `bg-white text-slate-900 shadow-sm rounded-md`
- Inactive: `text-slate-500 hover:text-slate-700 rounded-md`

### Buttons

| Variant | Classes |
|---|---|
| Primary | `bg-blue-800 hover:bg-blue-700 text-white rounded-lg px-4 py-2 text-sm font-medium shadow-sm transition` |
| Secondary | `bg-white border border-slate-200 hover:border-slate-300 hover:shadow-sm text-slate-700 rounded-lg px-4 py-2 text-sm font-medium transition` |
| Ghost | `hover:bg-slate-100 text-slate-600 hover:text-slate-900 rounded-lg px-3 py-2 text-sm transition` |
| Destructive | `bg-red-600 hover:bg-red-700 text-white rounded-lg px-4 py-2 text-sm font-medium transition` |

### Watchlist Rows

```
rounded-lg px-3 py-2 flex items-center gap-3
hover:bg-slate-50 cursor-pointer transition
active: border-l-2 border-blue-800 bg-blue-50/40

├── symbol: font-mono text-sm font-semibold text-slate-900
├── price:  font-mono text-sm text-slate-700 ml-auto
└── delta:  rounded-full text-xs font-mono px-1.5 py-0.5
            green: bg-green-50 text-green-600
            red:   bg-red-50 text-red-600
```

### Portfolio Stat Cards (summary)

```
rounded-xl bg-white shadow-sm p-4 border border-slate-100
├── label: text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1
├── value: font-mono text-xl font-bold text-slate-900
└── delta: rounded-full text-xs font-mono px-2 py-0.5 mt-1
           inline-flex items-center gap-1
```

### News Items

```
px-4 py-3 border-b border-slate-100 last:border-0
hover:bg-slate-50 hover:translate-x-0.5 transition-all duration-150

├── source + time: text-xs text-slate-400
├── headline:      text-sm font-medium text-slate-800 line-clamp-2 mt-0.5
└── sentiment chip (optional): rounded-full text-xs
```

### Alerts / Signal Items

```
px-4 py-3 flex items-start gap-3 border-b border-slate-100 last:border-0

├── icon: rounded-full w-6 h-6 flex items-center justify-center
│         warning: bg-amber-50 text-amber-600
│         signal:  bg-blue-50 text-blue-600
│         error:   bg-red-50 text-red-600
├── content:
│     title: text-sm font-medium text-slate-800
│     body:  text-xs text-slate-500 mt-0.5 line-clamp-2
└── time: text-xs text-slate-400 ml-auto shrink-0
```

### Modals

- Overlay: `bg-slate-900/40 backdrop-blur-sm`
- Dialog: `rounded-2xl bg-white shadow-xl border border-slate-100 w-full max-w-lg`
- Enter animation: `scale-95 opacity-0 → scale-100 opacity-100 duration-150`

---

## 4. Micro-interactions

| Interaction | Implementation |
|---|---|
| Price tick flash (up) | `@keyframes flash-green` — bg pulses `green-50 → transparent` over 400ms |
| Price tick flash (down) | `@keyframes flash-red` — bg pulses `red-50 → transparent` over 400ms |
| Button press | `active:scale-95 transition-transform duration-75` |
| All hover transitions | `transition-all duration-150` |
| Modal open/close | `data-[state=open]:animate-in data-[state=closed]:animate-out scale-95/100 fade-in/out duration-150` |
| News item hover | `hover:translate-x-0.5 hover:bg-slate-50 duration-150` |
| Panel shadow on hover | `hover:shadow-md duration-150` |

---

## 5. Files to Create / Modify

### New files
- `frontend/src/components/layout/CommandBar.tsx` — top command bar with tabs + global controls
- `frontend/src/components/dashboard/WatchlistPanel.tsx` — live watchlist with price ticks
- `frontend/src/components/dashboard/PortfolioSummaryPanel.tsx` — compact stat cards
- `frontend/src/components/dashboard/NewsFeedPanel.tsx` — news items, auto-filtered
- `frontend/src/components/dashboard/AlertsFeedPanel.tsx` — recent alerts/signals

### Modified files
- `frontend/src/index.css` — new CSS variable tokens, flash keyframes, scrollbar
- `frontend/src/components/layout/Layout.tsx` — remove Sidebar, add CommandBar, restructure main
- `frontend/src/components/layout/TopBar.tsx` — absorbed into CommandBar (can be deleted)
- `frontend/src/components/layout/Sidebar.tsx` — removed
- `frontend/src/pages/Dashboard.tsx` — rebuilt as 3-column workstation
- `frontend/src/components/chart/CandlestickChart.tsx` — minor: remove outer padding (panel provides it)
- All page files — remove page-level `h1` headings (command bar tabs provide context), apply new max-w layout wrapper

---

## 6. Out of Scope (v1)

- Resizable panels (drag handles) — v2 enhancement
- Keyboard-driven navigation (cmd+K command palette) — v2
- Dark mode redesign — functional but not priority-polished
- Replacing all existing page internals (Backtest form, Monitor table, etc.) — they get the new shell but internal layouts are untouched in this phase
