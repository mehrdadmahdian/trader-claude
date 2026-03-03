# UI Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the generic sidebar+topbar shell with a Bloomberg-style single-view workstation: top command bar navigation, 3-column dashboard (watchlist + chart + news/alerts), and a clean light/neutral pro design system.

**Architecture:** The design system lives in `index.css` CSS variables + `tailwind.config.js` keyframes. A new `CommandBar.tsx` replaces both `Sidebar.tsx` and `TopBar.tsx`. `Layout.tsx` is restructured so the Dashboard route renders the 3-column workstation while all other pages render full-width below the command bar.

**Tech Stack:** React 18, Tailwind CSS v3, shadcn/ui (Radix primitives), Zustand stores, React Query, lucide-react icons, existing hooks (`useMarketStore`, `usePortfolioStore`, `useAlertStore`, `useNotificationStore`, `useNews`)

---

## Task 1: Update CSS Design Tokens

**Files:**
- Modify: `frontend/src/index.css`

**Context:** The CSS variables drive every shadcn/ui component color. We're switching from the generic dark-blue defaults to a light/neutral pro palette (slate-50 background, white cards, blue-800 accent). We also add `flash-up` and `flash-down` keyframes for price tick animations.

**Step 1: Replace the entire contents of `frontend/src/index.css`**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    /* Surfaces */
    --background: 210 40% 98%;          /* slate-50  #f8fafc */
    --foreground: 222 47% 11%;          /* slate-900 #0f172a */
    --card: 0 0% 100%;                  /* white */
    --card-foreground: 222 47% 11%;
    --popover: 0 0% 100%;
    --popover-foreground: 222 47% 11%;

    /* Primary = blue-800 */
    --primary: 226 71% 40%;             /* #1e40af */
    --primary-foreground: 210 40% 98%;

    /* Secondary / muted */
    --secondary: 210 40% 96%;           /* slate-100 */
    --secondary-foreground: 222 47% 11%;
    --muted: 210 40% 96%;               /* slate-100 */
    --muted-foreground: 215 16% 47%;    /* slate-500 #64748b */

    /* Accent (hover backgrounds) */
    --accent: 210 40% 96%;              /* slate-100 */
    --accent-foreground: 222 47% 11%;

    /* Destructive */
    --destructive: 0 84% 60%;           /* red-500 */
    --destructive-foreground: 0 0% 100%;

    /* Borders & inputs */
    --border: 214 32% 91%;              /* slate-200 #e2e8f0 */
    --input: 214 32% 91%;
    --ring: 226 71% 40%;                /* blue-800 */

    /* Radius — slightly larger for modern feel */
    --radius: 0.625rem;                 /* 10px */
  }

  .dark {
    --background: 222 47% 6%;
    --foreground: 210 40% 98%;
    --card: 222 47% 9%;
    --card-foreground: 210 40% 98%;
    --popover: 222 47% 9%;
    --popover-foreground: 210 40% 98%;
    --primary: 217 91% 60%;
    --primary-foreground: 222 47% 11%;
    --secondary: 217 33% 17%;
    --secondary-foreground: 210 40% 98%;
    --muted: 217 33% 17%;
    --muted-foreground: 215 20% 65%;
    --accent: 217 33% 17%;
    --accent-foreground: 210 40% 98%;
    --destructive: 0 63% 31%;
    --destructive-foreground: 210 40% 98%;
    --border: 217 33% 17%;
    --input: 217 33% 17%;
    --ring: 217 91% 60%;
  }
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
    font-feature-settings: "rlig" 1, "calt" 1;
  }
}

/* ── Price tick flash animations ─────────────────────────────────────────── */
@keyframes flash-up {
  0%   { background-color: rgb(220 252 231); }   /* green-100 */
  100% { background-color: transparent; }
}
@keyframes flash-down {
  0%   { background-color: rgb(254 226 226); }   /* red-100 */
  100% { background-color: transparent; }
}
.flash-up   { animation: flash-up   0.4s ease-out; }
.flash-down { animation: flash-down 0.4s ease-out; }

/* ── Scrollbar ───────────────────────────────────────────────────────────── */
::-webkit-scrollbar        { width: 4px; height: 4px; }
::-webkit-scrollbar-track  { background: transparent; }
::-webkit-scrollbar-thumb  { @apply bg-border rounded-full; }
::-webkit-scrollbar-thumb:hover { @apply bg-muted-foreground/30; }
```

**Step 2: Start the dev environment and verify**

```bash
make up
```

Open `http://localhost:5173`. The app should now have a lighter background (slate-50) and blue-800 primary color. The sidebar and any primary buttons should look noticeably different.

**Step 3: Commit**

```bash
git add frontend/src/index.css
git commit -m "style: update design tokens to light/neutral pro palette with flash keyframes"
```

---

## Task 2: Update Tailwind Config

**Files:**
- Modify: `frontend/tailwind.config.js`

**Context:** We add `flash-up`/`flash-down` to the `animation` map (so `animate-flash-up` utility works), tweak the border-radius to match `--radius: 0.625rem`, and keep everything else intact.

**Step 1: Edit `frontend/tailwind.config.js`**

Replace the `keyframes` and `animation` blocks (lines 61–79) with:

```js
keyframes: {
  'accordion-down': {
    from: { height: '0' },
    to: { height: 'var(--radix-accordion-content-height)' },
  },
  'accordion-up': {
    from: { height: 'var(--radix-accordion-content-height)' },
    to: { height: '0' },
  },
  'fade-in': {
    from: { opacity: '0', transform: 'translateY(4px)' },
    to: { opacity: '1', transform: 'translateY(0)' },
  },
  'flash-up': {
    '0%':   { backgroundColor: 'rgb(220 252 231)' },
    '100%': { backgroundColor: 'transparent' },
  },
  'flash-down': {
    '0%':   { backgroundColor: 'rgb(254 226 226)' },
    '100%': { backgroundColor: 'transparent' },
  },
},
animation: {
  'accordion-down': 'accordion-down 0.2s ease-out',
  'accordion-up':   'accordion-up 0.2s ease-out',
  'fade-in':        'fade-in 0.2s ease-out',
  'flash-up':       'flash-up 0.4s ease-out',
  'flash-down':     'flash-down 0.4s ease-out',
},
```

**Step 2: Commit**

```bash
git add frontend/tailwind.config.js
git commit -m "style: add flash-up/flash-down animations to tailwind config"
```

---

## Task 3: Update Button Component

**Files:**
- Modify: `frontend/src/components/ui/button.tsx`

**Context:** The current button uses `rounded-md` and generic hover colors. We update to `rounded-lg`, add `active:scale-95` press feel, and tweak secondary/ghost/outline variants to match the new slate palette.

**Step 1: Replace the `buttonVariants` definition**

Replace lines 6–30 in `frontend/src/components/ui/button.tsx` with:

```tsx
const buttonVariants = cva(
  'inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 active:scale-95',
  {
    variants: {
      variant: {
        default:     'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90',
        destructive: 'bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90',
        outline:     'border border-input bg-background shadow-sm hover:border-slate-300 hover:shadow-md text-foreground',
        secondary:   'bg-white border border-slate-200 text-slate-700 shadow-sm hover:border-slate-300 hover:shadow-md',
        ghost:       'hover:bg-accent hover:text-accent-foreground',
        link:        'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm:      'h-7 rounded-md px-3 text-xs',
        lg:      'h-11 rounded-lg px-8',
        icon:    'h-9 w-9',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)
```

**Step 2: Verify**

Check any existing page that uses `<Button>` — it should now feel snappier on press and have slightly softer corners.

**Step 3: Commit**

```bash
git add frontend/src/components/ui/button.tsx
git commit -m "style: update button variants — rounded-lg, active:scale-95, new secondary style"
```

---

## Task 4: Install shadcn Select Component

**Files:**
- Create: `frontend/src/components/ui/select.tsx` (generated by shadcn CLI)

**Context:** The Chart page uses raw `<select>` elements. We need a styled shadcn Select to replace them. Run the shadcn CLI inside the frontend container.

**Step 1: Install the component**

```bash
make frontend-shell
# Inside the container:
npx shadcn-ui@latest add select
# Answer prompts: yes to overwrite if asked
exit
```

**Step 2: Verify the file exists**

```bash
ls frontend/src/components/ui/select.tsx
```

Expected: file exists with Radix `SelectPrimitive` exports.

**Step 3: Commit**

```bash
git add frontend/src/components/ui/select.tsx
git commit -m "feat: add shadcn Select component"
```

---

## Task 5: Create CommandBar Component

**Files:**
- Create: `frontend/src/components/layout/CommandBar.tsx`

**Context:** This replaces both `Sidebar.tsx` and `TopBar.tsx`. It is a slim 52px horizontal bar at the top of the screen containing:
1. Logo (left)
2. Global symbol combobox + timeframe selector (center-left) — reads/writes `useMarketStore`
3. Page navigation tabs (center) — `NavLink` components
4. Notification bell + theme toggle + user avatar (right)

The symbol combobox reuses the search dropdown pattern from `Chart.tsx` but lives globally so it works from any page.

**Step 1: Create `frontend/src/components/layout/CommandBar.tsx`**

```tsx
import { useState, useMemo } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import {
  TrendingUp, LayoutDashboard, CandlestickChart, FlaskConical,
  Briefcase, Activity, Newspaper, Bell, Settings, Sun, Moon,
  Search, ChevronDown, CheckCheck, LogOut, ChevronRight,
} from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { cn } from '@/lib/utils'
import { useThemeStore, useNotificationStore, useMarketStore } from '@/stores'
import { useAuthStore } from '@/stores/authStore'
import { useMarkAllRead } from '@/hooks/useNotifications'
import { useMarkets, useSymbols } from '@/hooks/useMarketData'
import type { MarketSymbol } from '@/types'

const TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w']

const navItems = [
  { to: '/',          icon: LayoutDashboard,  label: 'Dashboard' },
  { to: '/chart',     icon: CandlestickChart, label: 'Chart'     },
  { to: '/backtest',  icon: FlaskConical,     label: 'Backtest'  },
  { to: '/portfolio', icon: Briefcase,        label: 'Portfolio' },
  { to: '/monitor',   icon: Activity,         label: 'Monitor'   },
  { to: '/news',      icon: Newspaper,        label: 'News'      },
  { to: '/alerts',    icon: Bell,             label: 'Alerts'    },
  { to: '/settings',  icon: Settings,         label: 'Settings'  },
]

export function CommandBar() {
  const { theme, toggleTheme } = useThemeStore()
  const { unreadCount, notifications } = useNotificationStore()
  const { mutate: markAllRead } = useMarkAllRead()
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()

  // Global symbol / timeframe controls (shared with Chart page via store)
  const selectedSymbol    = useMarketStore((s) => s.selectedSymbol)
  const selectedMarket    = useMarketStore((s) => s.selectedMarket)
  const selectedTimeframe = useMarketStore((s) => s.selectedTimeframe)
  const setSelectedSymbol    = useMarketStore((s) => s.setSelectedSymbol)
  const setSelectedMarket    = useMarketStore((s) => s.setSelectedMarket)
  const setSelectedTimeframe = useMarketStore((s) => s.setSelectedTimeframe)

  const [adapter] = useState('binance')
  const { data: adapters } = useMarkets()
  const { data: symbols }  = useSymbols(adapter, selectedMarket)

  const [showSymbolSearch, setShowSymbolSearch] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const filteredSymbols = useMemo(() => {
    if (!symbols) return []
    if (!searchQuery.trim()) return symbols.slice(0, 50)
    const q = searchQuery.toLowerCase()
    return symbols.filter(
      (s) => s.id.toLowerCase().includes(q) || s.description?.toLowerCase().includes(q),
    )
  }, [symbols, searchQuery])

  function handleSymbolSelect(sym: MarketSymbol) {
    setSelectedSymbol(sym.id)
    setShowSymbolSearch(false)
    setSearchQuery('')
  }

  const recentNotifications = notifications.slice(0, 5)

  return (
    <header className="h-[52px] bg-white border-b border-slate-200 shadow-sm flex items-center gap-1 px-4 shrink-0 z-40">
      {/* ── Logo ── */}
      <div className="flex items-center gap-2 mr-4 shrink-0">
        <div className="w-7 h-7 rounded-lg bg-primary flex items-center justify-center">
          <TrendingUp className="w-4 h-4 text-white" />
        </div>
        <span className="font-semibold text-sm text-slate-900 tracking-tight hidden sm:block">
          Trader Claude
        </span>
      </div>

      {/* ── Global symbol picker ── */}
      <div className="relative mr-1">
        <button
          onClick={() => setShowSymbolSearch((v) => !v)}
          className="flex items-center gap-1.5 h-8 px-3 rounded-lg border border-slate-200 bg-white text-sm shadow-sm hover:border-slate-300 hover:shadow-md transition-all duration-150 min-w-[130px]"
          aria-expanded={showSymbolSearch}
        >
          <Search className="w-3.5 h-3.5 text-slate-400 shrink-0" />
          <span className={cn('truncate', selectedSymbol ? 'text-slate-900 font-medium font-mono' : 'text-slate-400')}>
            {selectedSymbol ?? 'Symbol…'}
          </span>
          <ChevronDown className="w-3.5 h-3.5 text-slate-400 ml-auto shrink-0" />
        </button>

        {showSymbolSearch && (
          <div className="absolute z-50 top-full mt-1 w-72 bg-white border border-slate-200 rounded-xl shadow-xl">
            <div className="p-2 border-b border-slate-100">
              <input
                autoFocus
                type="text"
                placeholder="Search symbols…"
                className="w-full rounded-lg border border-slate-200 px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
            <ul className="max-h-60 overflow-y-auto">
              {filteredSymbols.length === 0 ? (
                <li className="px-3 py-4 text-sm text-slate-400 text-center">No symbols found</li>
              ) : (
                filteredSymbols.map((sym) => (
                  <li key={sym.id}>
                    <button
                      className="w-full px-3 py-2.5 text-left text-sm hover:bg-slate-50 transition-colors flex items-center justify-between"
                      onClick={() => handleSymbolSelect(sym)}
                    >
                      <span className="font-mono font-semibold text-slate-900">{sym.id}</span>
                      {sym.description && (
                        <span className="text-slate-400 text-xs ml-2 truncate max-w-[100px]">
                          {sym.description}
                        </span>
                      )}
                    </button>
                  </li>
                ))
              )}
            </ul>
          </div>
        )}
      </div>

      {/* ── Timeframe pills ── */}
      <div className="hidden md:flex items-center gap-0.5 bg-slate-100 rounded-lg p-0.5 mr-3">
        {TIMEFRAMES.map((tf) => (
          <button
            key={tf}
            onClick={() => setSelectedTimeframe(tf)}
            className={cn(
              'px-2 py-1 rounded-md text-xs font-medium transition-all duration-150',
              selectedTimeframe === tf
                ? 'bg-white text-slate-900 shadow-sm'
                : 'text-slate-500 hover:text-slate-700',
            )}
          >
            {tf}
          </button>
        ))}
      </div>

      {/* ── Page tabs ── */}
      <nav className="hidden lg:flex items-center gap-0.5 flex-1 overflow-x-auto">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium transition-all duration-150 whitespace-nowrap',
                isActive
                  ? 'bg-primary/10 text-primary border border-primary/20'
                  : 'text-slate-500 hover:text-slate-800 hover:bg-slate-100',
              )
            }
          >
            <Icon className="w-3.5 h-3.5 shrink-0" />
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="flex-1 lg:flex-none" />

      {/* ── Theme toggle ── */}
      <button
        onClick={toggleTheme}
        className="p-2 rounded-lg text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-all duration-150"
        aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      >
        {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
      </button>

      {/* ── Notification bell ── */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button
            className="relative p-2 rounded-lg text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-all duration-150"
            aria-label="Notifications"
          >
            <Bell className="w-4 h-4" />
            {unreadCount > 0 && (
              <span className="absolute top-1 right-1 min-w-[1rem] h-4 px-0.5 flex items-center justify-center text-[10px] font-bold text-white bg-red-500 rounded-full">
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className="z-50 w-80 rounded-xl border border-slate-200 bg-white shadow-xl animate-in fade-in-0 zoom-in-95 duration-150"
          >
            <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100">
              <span className="text-sm font-semibold text-slate-900">Notifications</span>
              {unreadCount > 0 && (
                <button
                  onClick={() => markAllRead()}
                  className="flex items-center gap-1 text-xs text-slate-400 hover:text-slate-700 transition-colors"
                >
                  <CheckCheck className="w-3.5 h-3.5" />
                  Mark all read
                </button>
              )}
            </div>
            {recentNotifications.length === 0 ? (
              <div className="px-4 py-6 text-center text-sm text-slate-400">No notifications yet</div>
            ) : (
              recentNotifications.map((n) => (
                <DropdownMenu.Item
                  key={n.id}
                  className={cn(
                    'flex flex-col gap-0.5 px-4 py-3 border-b border-slate-100 last:border-0 cursor-default select-none outline-none hover:bg-slate-50 transition-colors',
                    !n.read && 'bg-blue-50/40',
                  )}
                >
                  <div className="flex items-start gap-2">
                    {!n.read && <span className="mt-1.5 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />}
                    <div className={cn('min-w-0', n.read && 'pl-3.5')}>
                      <p className="text-xs font-medium text-slate-800 truncate">{n.title}</p>
                      <p className="text-xs text-slate-500 line-clamp-2 mt-0.5">{n.body}</p>
                      <p className="text-[10px] text-slate-400 mt-1">
                        {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                      </p>
                    </div>
                  </div>
                </DropdownMenu.Item>
              ))
            )}
            <DropdownMenu.Item
              className="flex justify-center px-4 py-2.5 text-xs text-primary hover:text-primary/80 hover:bg-slate-50 transition-colors cursor-pointer outline-none"
              onSelect={() => navigate('/notifications')}
            >
              View all notifications <ChevronRight className="w-3.5 h-3.5 ml-1" />
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>

      {/* ── User avatar ── */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button
            className="flex items-center justify-center w-8 h-8 rounded-full bg-primary/10 text-primary text-sm font-semibold hover:bg-primary/20 transition-all duration-150 ml-1 shrink-0"
            aria-label="User menu"
          >
            {user ? (user.display_name || user.email).charAt(0).toUpperCase() : '?'}
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className="z-50 w-48 rounded-xl border border-slate-200 bg-white shadow-xl animate-in fade-in-0 zoom-in-95 duration-150"
          >
            {user && (
              <div className="px-3 py-2.5 border-b border-slate-100">
                <p className="text-sm font-medium text-slate-900 truncate">{user.display_name || user.email}</p>
                <p className="text-xs text-slate-400 capitalize">{user.role}</p>
              </div>
            )}
            <DropdownMenu.Item
              className="flex items-center gap-2 px-3 py-2.5 text-sm text-red-600 hover:bg-red-50 cursor-pointer outline-none transition-colors"
              onSelect={() => logout()}
            >
              <LogOut className="w-4 h-4" />
              Logout
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </header>
  )
}
```

**Step 2: Commit**

```bash
git add frontend/src/components/layout/CommandBar.tsx
git commit -m "feat: add CommandBar — top nav replacing sidebar + topbar"
```

---

## Task 6: Update Layout.tsx

**Files:**
- Modify: `frontend/src/components/layout/Layout.tsx`

**Context:** `Layout.tsx` currently renders `<Sidebar>` + `<TopBar>` + `<Outlet>`. We replace that with `<CommandBar>` + `<Outlet>`. The Dashboard page will handle its own 3-column grid internally — Layout just provides the shell.

**Step 1: Replace `frontend/src/components/layout/Layout.tsx`**

```tsx
import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { CommandBar } from './CommandBar'
import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'
import { AIButton } from '@/components/ai/AIButton'
import { ChatPanel } from '@/components/ai/ChatPanel'

export function Layout() {
  const [isChatOpen, setIsChatOpen] = useState(false)
  useNotificationWS()

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <CommandBar />
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
      <SignalToast />
      <AIButton onClick={() => setIsChatOpen((o) => !o)} isOpen={isChatOpen} />
      <ChatPanel isOpen={isChatOpen} onClose={() => setIsChatOpen(false)} />
    </div>
  )
}
```

**Step 2: Verify the app still loads**

Open `http://localhost:5173`. You should see the new CommandBar across the top (no left sidebar). All pages should still be reachable via the nav tabs. The existing page content renders below.

**Step 3: Commit**

```bash
git add frontend/src/components/layout/Layout.tsx
git commit -m "feat: update Layout to use CommandBar, remove Sidebar + TopBar"
```

---

## Task 7: Build WatchlistPanel

**Files:**
- Create: `frontend/src/components/dashboard/WatchlistPanel.tsx`

**Context:** Shows live price ticks from `useMarketStore`. When no ticks are in the store (no live WS feed yet), falls back to showing the searched symbols. The active symbol (from `useMarketStore`) is highlighted with a blue left border. Price changes trigger the `flash-up` / `flash-down` CSS class for 400ms.

**Step 1: Create `frontend/src/components/dashboard/WatchlistPanel.tsx`**

```tsx
import { useRef, useEffect, useState } from 'react'
import { Star, TrendingUp } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useMarketStore } from '@/stores'

// A static default watchlist shown when no ticks are streaming
const DEFAULT_WATCH = [
  'BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT',
  'XRPUSDT', 'ADAUSDT', 'DOGEUSDT', 'AVAXUSDT',
]

function PriceRow({
  symbol,
  price,
  prevPrice,
  isActive,
  onClick,
}: {
  symbol: string
  price: number | null
  prevPrice: number | null
  isActive: boolean
  onClick: () => void
}) {
  const [flashClass, setFlashClass] = useState('')
  const prevRef = useRef<number | null>(prevPrice)

  useEffect(() => {
    if (price === null || prevRef.current === null) {
      prevRef.current = price
      return
    }
    if (price > prevRef.current) {
      setFlashClass('flash-up')
    } else if (price < prevRef.current) {
      setFlashClass('flash-down')
    }
    prevRef.current = price
    const t = setTimeout(() => setFlashClass(''), 400)
    return () => clearTimeout(t)
  }, [price])

  // Fake percentage change display (real data would come from a 24h price endpoint)
  const pct = price && prevPrice && prevPrice !== 0
    ? ((price - prevPrice) / prevPrice) * 100
    : null

  return (
    <button
      onClick={onClick}
      className={cn(
        'w-full flex items-center gap-2 px-3 py-2 rounded-lg text-left transition-all duration-150',
        'hover:bg-slate-50',
        isActive && 'border-l-2 border-primary bg-primary/5',
        flashClass,
      )}
    >
      <span className={cn('font-mono text-xs font-semibold truncate flex-1', isActive ? 'text-primary' : 'text-slate-800')}>
        {symbol}
      </span>
      {price !== null ? (
        <>
          <span className="font-mono text-xs text-slate-700 shrink-0">
            {price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: price > 1000 ? 2 : 5 })}
          </span>
          {pct !== null && (
            <span className={cn(
              'text-[10px] font-mono rounded-full px-1.5 py-0.5 shrink-0',
              pct >= 0 ? 'bg-green-50 text-green-600' : 'bg-red-50 text-red-600',
            )}>
              {pct >= 0 ? '+' : ''}{pct.toFixed(2)}%
            </span>
          )}
        </>
      ) : (
        <span className="text-xs text-slate-300">—</span>
      )}
    </button>
  )
}

export function WatchlistPanel() {
  const ticks = useMarketStore((s) => s.ticks)
  const selectedSymbol = useMarketStore((s) => s.selectedSymbol)
  const setSelectedSymbol = useMarketStore((s) => s.setSelectedSymbol)

  // Build display list: ticking symbols first, then defaults
  const tickingSymbols = Object.keys(ticks).map((key) => key.split(':')[0])
  const displaySymbols = tickingSymbols.length > 0
    ? Array.from(new Set([...tickingSymbols, ...DEFAULT_WATCH])).slice(0, 20)
    : DEFAULT_WATCH

  return (
    <div className="flex flex-col rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-2">
          <Star className="w-3.5 h-3.5 text-slate-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Watchlist</span>
        </div>
        <TrendingUp className="w-3.5 h-3.5 text-slate-300" />
      </div>

      {/* Rows */}
      <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
        {displaySymbols.map((sym) => {
          const tickKey = Object.keys(ticks).find((k) => k.startsWith(sym + ':'))
          const tick = tickKey ? ticks[tickKey] : null
          return (
            <PriceRow
              key={sym}
              symbol={sym}
              price={tick?.price ?? null}
              prevPrice={null}
              isActive={selectedSymbol === sym}
              onClick={() => setSelectedSymbol(sym)}
            />
          )
        })}
      </div>
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add frontend/src/components/dashboard/WatchlistPanel.tsx
git commit -m "feat: add WatchlistPanel with price tick flash animations"
```

---

## Task 8: Build PortfolioSummaryPanel

**Files:**
- Create: `frontend/src/components/dashboard/PortfolioSummaryPanel.tsx`

**Context:** Shows compact stat cards from `usePortfolioStore`. If no portfolio is active, shows a placeholder. Data from the store is already populated by the Portfolio page when visited.

**Step 1: Create `frontend/src/components/dashboard/PortfolioSummaryPanel.tsx`**

```tsx
import { Briefcase, TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { cn } from '@/lib/utils'
import { usePortfolioStore } from '@/stores'

function StatCard({
  label,
  value,
  delta,
  deltaPositive,
}: {
  label: string
  value: string
  delta?: string
  deltaPositive?: boolean
}) {
  return (
    <div className="flex flex-col gap-0.5 p-3 rounded-xl bg-slate-50 border border-slate-100">
      <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">{label}</span>
      <span className="font-mono text-sm font-bold text-slate-900 truncate">{value}</span>
      {delta !== undefined && (
        <span className={cn(
          'inline-flex items-center gap-1 text-[10px] font-mono rounded-full w-fit px-1.5 py-0.5 mt-0.5',
          deltaPositive ? 'bg-green-50 text-green-600' : 'bg-red-50 text-red-600',
        )}>
          {deltaPositive
            ? <TrendingUp className="w-2.5 h-2.5" />
            : <TrendingDown className="w-2.5 h-2.5" />
          }
          {delta}
        </span>
      )}
    </div>
  )
}

export function PortfolioSummaryPanel() {
  const { summary, positions, activePortfolioId } = usePortfolioStore()

  const formatUSD = (n: number) =>
    new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 2 }).format(n)

  const formatPct = (n: number) => `${n >= 0 ? '+' : ''}${n.toFixed(2)}%`

  return (
    <div className="flex flex-col rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-2">
          <Briefcase className="w-3.5 h-3.5 text-slate-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Portfolio</span>
        </div>
        {activePortfolioId && (
          <span className="text-[10px] text-slate-300 font-mono">#{activePortfolioId}</span>
        )}
      </div>

      {/* Stats */}
      <div className="p-3 space-y-2">
        {!activePortfolioId || !summary ? (
          <div className="flex flex-col items-center gap-2 py-4 text-center">
            <Minus className="w-6 h-6 text-slate-200" />
            <p className="text-xs text-slate-400">No portfolio selected</p>
            <a href="/portfolio" className="text-xs text-primary hover:underline">Open Portfolio →</a>
          </div>
        ) : (
          <>
            <StatCard
              label="Total Value"
              value={formatUSD(summary.total_value)}
            />
            <div className="grid grid-cols-2 gap-2">
              <StatCard
                label="P&L"
                value={formatUSD(summary.total_pnl)}
                delta={formatPct(summary.total_pnl_pct)}
                deltaPositive={summary.total_pnl >= 0}
              />
              <StatCard
                label="Positions"
                value={String(positions.length)}
              />
            </div>
          </>
        )}
      </div>
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add frontend/src/components/dashboard/PortfolioSummaryPanel.tsx
git commit -m "feat: add PortfolioSummaryPanel with stat cards"
```

---

## Task 9: Build NewsFeedPanel

**Files:**
- Create: `frontend/src/components/dashboard/NewsFeedPanel.tsx`

**Context:** Shows news items filtered to the selected symbol, using `useNewsBySymbol`. When no symbol is selected, shows recent global news via `useNews`. Items have the hover-slide micro-interaction.

**Step 1: Create `frontend/src/components/dashboard/NewsFeedPanel.tsx`**

```tsx
import { Newspaper, ExternalLink } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { cn } from '@/lib/utils'
import { useMarketStore } from '@/stores'
import { useNewsBySymbol } from '@/hooks/useNews'

export function NewsFeedPanel() {
  const selectedSymbol = useMarketStore((s) => s.selectedSymbol)

  const { data, isFetching } = useNewsBySymbol(selectedSymbol, 20)
  const items = data?.data ?? []

  return (
    <div className="flex flex-col rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-2">
          <Newspaper className="w-3.5 h-3.5 text-slate-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">News</span>
          {selectedSymbol && (
            <span className="text-[10px] font-mono bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded-full">
              {selectedSymbol}
            </span>
          )}
        </div>
        {isFetching && (
          <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" />
        )}
      </div>

      {/* Items */}
      <div className="flex-1 overflow-y-auto divide-y divide-slate-100">
        {items.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 gap-2">
            <Newspaper className="w-6 h-6 text-slate-200" />
            <p className="text-xs text-slate-400">
              {isFetching ? 'Loading news…' : 'No news available'}
            </p>
          </div>
        ) : (
          items.map((item) => (
            <a
              key={item.id}
              href={item.url ?? '#'}
              target="_blank"
              rel="noopener noreferrer"
              className={cn(
                'flex flex-col gap-1 px-4 py-3 hover:bg-slate-50',
                'hover:translate-x-0.5 transition-all duration-150',
                'no-underline group',
              )}
            >
              <div className="flex items-center gap-2">
                <span className="text-[10px] text-slate-400 truncate flex-1">
                  {item.source} · {formatDistanceToNow(new Date(item.published_at), { addSuffix: true })}
                </span>
                <ExternalLink className="w-2.5 h-2.5 text-slate-300 group-hover:text-slate-500 shrink-0 transition-colors" />
              </div>
              <p className="text-xs font-medium text-slate-800 line-clamp-2 leading-relaxed">
                {item.title}
              </p>
              {item.sentiment && (
                <span className={cn(
                  'self-start text-[10px] font-medium rounded-full px-1.5 py-0.5 mt-0.5',
                  item.sentiment === 'positive' ? 'bg-green-50 text-green-600' :
                  item.sentiment === 'negative' ? 'bg-red-50 text-red-600' :
                  'bg-slate-100 text-slate-500',
                )}>
                  {item.sentiment}
                </span>
              )}
            </a>
          ))
        )}
      </div>
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add frontend/src/components/dashboard/NewsFeedPanel.tsx
git commit -m "feat: add NewsFeedPanel with symbol-filtered news"
```

---

## Task 10: Build AlertsFeedPanel

**Files:**
- Create: `frontend/src/components/dashboard/AlertsFeedPanel.tsx`

**Context:** Shows active alerts from `useAlertStore` and recent notifications from `useNotificationStore`. Groups them with a small icon by type (warning, signal, info).

**Step 1: Create `frontend/src/components/dashboard/AlertsFeedPanel.tsx`**

```tsx
import { Bell, AlertTriangle, Activity, Info } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { cn } from '@/lib/utils'
import { useAlertStore, useNotificationStore } from '@/stores'
import { useAlerts } from '@/hooks/useAlerts'

function iconForType(type: string) {
  if (type === 'signal') return <Activity className="w-3 h-3" />
  if (type === 'warning' || type === 'price') return <AlertTriangle className="w-3 h-3" />
  return <Info className="w-3 h-3" />
}

function bgForType(type: string) {
  if (type === 'signal') return 'bg-blue-50 text-blue-600'
  if (type === 'warning' || type === 'price') return 'bg-amber-50 text-amber-600'
  return 'bg-slate-100 text-slate-500'
}

export function AlertsFeedPanel() {
  useAlerts() // populates useAlertStore

  const alerts = useAlertStore((s) => s.alerts)
  const notifications = useNotificationStore((s) => s.notifications)

  // Show latest 3 triggered notifications + up to 5 active alerts
  const recentNotifs = notifications.slice(0, 3)
  const activeAlerts = alerts.filter((a) => a.active).slice(0, 5)

  const isEmpty = recentNotifs.length === 0 && activeAlerts.length === 0

  return (
    <div className="flex flex-col rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-100 shrink-0">
        <div className="flex items-center gap-2">
          <Bell className="w-3.5 h-3.5 text-slate-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Alerts</span>
        </div>
        {alerts.length > 0 && (
          <span className="text-[10px] font-mono bg-blue-50 text-blue-600 px-1.5 py-0.5 rounded-full">
            {alerts.filter((a) => a.active).length} active
          </span>
        )}
      </div>

      {/* Items */}
      <div className="divide-y divide-slate-100">
        {isEmpty ? (
          <div className="flex flex-col items-center justify-center py-6 gap-2">
            <Bell className="w-6 h-6 text-slate-200" />
            <p className="text-xs text-slate-400">No alerts yet</p>
          </div>
        ) : (
          <>
            {recentNotifs.map((n) => (
              <div key={`notif-${n.id}`} className="flex items-start gap-3 px-4 py-3 hover:bg-slate-50 transition-colors">
                <span className={cn('flex items-center justify-center w-5 h-5 rounded-full shrink-0 mt-0.5', bgForType('signal'))}>
                  {iconForType('signal')}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-slate-800 truncate">{n.title}</p>
                  <p className="text-[10px] text-slate-400 mt-0.5">
                    {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                  </p>
                </div>
                {!n.read && <span className="w-1.5 h-1.5 rounded-full bg-primary mt-1.5 shrink-0" />}
              </div>
            ))}
            {activeAlerts.map((a) => (
              <div key={`alert-${a.id}`} className="flex items-start gap-3 px-4 py-3 hover:bg-slate-50 transition-colors">
                <span className={cn('flex items-center justify-center w-5 h-5 rounded-full shrink-0 mt-0.5', bgForType('price'))}>
                  {iconForType('price')}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-slate-800 truncate">{a.symbol} — {a.condition}</p>
                  <p className="text-[10px] text-slate-400 mt-0.5 font-mono">{a.threshold}</p>
                </div>
                <span className="text-[10px] bg-green-50 text-green-600 rounded-full px-1.5 py-0.5 shrink-0">on</span>
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  )
}
```

**Step 2: Check the Alert type shape** in `frontend/src/types/index.ts` to confirm field names (`symbol`, `condition`, `threshold`, `active`). Adjust field names in the component if they differ.

**Step 3: Commit**

```bash
git add frontend/src/components/dashboard/AlertsFeedPanel.tsx
git commit -m "feat: add AlertsFeedPanel with active alerts and recent notifications"
```

---

## Task 11: Rebuild Dashboard Page

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

**Context:** This is the centerpiece — the Bloomberg-style 3-column workstation. It embeds:
- **Left rail** (220px): `WatchlistPanel` (flex-1 scrollable) + `PortfolioSummaryPanel` (fixed height)
- **Center** (flex-1): `CandlestickChart` (reuses the existing component) + any indicator panels
- **Right rail** (280px): `NewsFeedPanel` (flex-1 scrollable) + `AlertsFeedPanel` (fixed height)

The chart here is a simplified version — no toolbar (the global symbol/timeframe is in CommandBar). Indicator controls can be added in a later iteration.

**Step 1: Replace `frontend/src/pages/Dashboard.tsx`**

```tsx
import { useMemo } from 'react'
import { subDays, formatISO } from 'date-fns'
import { useCandles } from '@/hooks/useMarketData'
import { CandlestickChart } from '@/components/chart/CandlestickChart'
import { WatchlistPanel } from '@/components/dashboard/WatchlistPanel'
import { PortfolioSummaryPanel } from '@/components/dashboard/PortfolioSummaryPanel'
import { NewsFeedPanel } from '@/components/dashboard/NewsFeedPanel'
import { AlertsFeedPanel } from '@/components/dashboard/AlertsFeedPanel'
import { useMarketStore, useThemeStore } from '@/stores'
import { CandlestickChart as ChartIcon } from 'lucide-react'

function defaultDateRange(timeframe: string) {
  const daysBack: Record<string, number> = {
    '1m': 1, '5m': 3, '15m': 7, '30m': 14,
    '1h': 30, '4h': 60, '1d': 365, '1w': 730,
  }
  const to = new Date()
  return {
    from: formatISO(subDays(to, daysBack[timeframe] ?? 30)),
    to: formatISO(to),
  }
}

export function Dashboard() {
  const selectedSymbol    = useMarketStore((s) => s.selectedSymbol)
  const selectedMarket    = useMarketStore((s) => s.selectedMarket)
  const selectedTimeframe = useMarketStore((s) => s.selectedTimeframe)
  const theme = useThemeStore((s) => s.theme)

  const { from, to } = useMemo(() => defaultDateRange(selectedTimeframe), [selectedTimeframe])

  const { data: candles, isFetching } = useCandles({
    adapter: 'binance',
    symbol: selectedSymbol ?? '',
    timeframe: selectedTimeframe,
    from,
    to,
    market: selectedMarket,
    enabled: !!selectedSymbol,
  })

  return (
    <div className="flex h-full gap-3 p-3 overflow-hidden">
      {/* ── Left rail ── */}
      <div className="flex flex-col gap-3 w-[220px] shrink-0 overflow-hidden">
        <div className="flex-1 min-h-0 overflow-hidden">
          <WatchlistPanel />
        </div>
        <div className="shrink-0">
          <PortfolioSummaryPanel />
        </div>
      </div>

      {/* ── Center: Chart ── */}
      <div className="flex-1 min-w-0 overflow-hidden">
        <div className="h-full rounded-2xl bg-white shadow-sm border border-slate-100 overflow-hidden">
          {!selectedSymbol ? (
            <div className="flex flex-col items-center justify-center h-full gap-3 text-slate-300">
              <ChartIcon className="w-12 h-12" />
              <p className="text-sm font-medium text-slate-500">Select a symbol to view chart</p>
              <p className="text-xs text-slate-400">Use the symbol picker in the top bar</p>
            </div>
          ) : (
            <CandlestickChart
              candles={candles ?? []}
              overlayIndicators={[]}
              isLoading={isFetching}
              className="h-full"
            />
          )}
        </div>
      </div>

      {/* ── Right rail ── */}
      <div className="flex flex-col gap-3 w-[280px] shrink-0 overflow-hidden">
        <div className="flex-1 min-h-0 overflow-hidden">
          <NewsFeedPanel />
        </div>
        <div className="shrink-0 max-h-[280px] overflow-y-auto">
          <AlertsFeedPanel />
        </div>
      </div>
    </div>
  )
}
```

**Step 2: Check `useCandles` signature** in `frontend/src/hooks/useMarketData.ts`. If it doesn't accept an `enabled` prop, remove that option and add a guard: `enabled: !!selectedSymbol` as a React Query `enabled` flag. Adjust to match actual hook signature.

**Step 3: Verify visually**

Open `http://localhost:5173/`. You should see:
- Left rail: watchlist with default symbols
- Center: "Select a symbol" empty state (until you pick one from CommandBar)
- Right rail: news + alerts panels

Select a symbol from the command bar — the center chart should load.

**Step 4: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat: rebuild Dashboard as 3-column Bloomberg-style workstation"
```

---

## Task 12: Update Chart Page Toolbar Styling

**Files:**
- Modify: `frontend/src/pages/Chart.tsx`

**Context:** The Chart page still has its own toolbar with raw `<select>` and plain styles. Since the global symbol + timeframe are now in the CommandBar, the Chart page toolbar can be simplified to just: adapter selector + indicators button + news toggle + refresh. Update styles to match new design tokens.

**Step 1: Update the toolbar div in `Chart.tsx`** (lines ~209–349)

Replace the outer toolbar `<div className="flex flex-wrap items-center gap-3">` wrapper and its children with:

```tsx
{/* ── Toolbar ── */}
<div className="flex flex-wrap items-center gap-2 px-1">
  {/* Adapter selector */}
  <div className="relative">
    <select
      className="appearance-none h-8 rounded-lg border border-slate-200 bg-white pl-3 pr-7 text-sm shadow-sm text-slate-700 hover:border-slate-300 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition cursor-pointer"
      value={selectedAdapter}
      onChange={handleAdapterChange}
      aria-label="Select adapter"
    >
      {(adapters ?? [{ id: 'binance', markets: ['crypto'] }]).map((a) => (
        <option key={a.id} value={a.id}>
          {a.id.charAt(0).toUpperCase() + a.id.slice(1)}
        </option>
      ))}
    </select>
    <ChevronDown className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-slate-400" />
  </div>

  {/* Symbol search */}
  <div className="relative">
    <button
      className="flex items-center gap-1.5 h-8 bg-white border border-slate-200 rounded-lg px-3 text-sm min-w-[140px] hover:border-slate-300 hover:shadow-sm transition-all duration-150 shadow-sm"
      onClick={() => setShowSearch((v) => !v)}
      aria-expanded={showSearch}
    >
      <Search className="h-3.5 w-3.5 text-slate-400 flex-shrink-0" />
      <span className={selectedSymbol ? 'font-mono font-medium text-slate-900' : 'text-slate-400'}>
        {selectedSymbol ?? 'Search symbol…'}
      </span>
      <ChevronDown className="h-3.5 w-3.5 text-slate-400 ml-auto" />
    </button>

    {showSearch && (
      <div className="absolute z-50 top-full mt-1 w-72 bg-white border border-slate-200 rounded-xl shadow-xl">
        <div className="p-2 border-b border-slate-100">
          <input
            autoFocus
            type="text"
            placeholder="Search…"
            className="w-full rounded-lg border border-slate-200 px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
        <ul role="listbox" className="max-h-64 overflow-y-auto">
          {filteredSymbols.length === 0 ? (
            <li className="px-3 py-4 text-sm text-slate-400 text-center">No symbols found</li>
          ) : (
            filteredSymbols.map((sym) => (
              <li key={sym.id}>
                <button
                  role="option"
                  aria-selected={sym.id === selectedSymbol}
                  className="w-full px-3 py-2.5 text-left text-sm hover:bg-slate-50 transition-colors flex items-center justify-between"
                  onClick={() => handleSymbolSelect(sym)}
                >
                  <span className="font-mono font-semibold text-slate-900">{sym.id}</span>
                  {sym.description && (
                    <span className="text-slate-400 text-xs ml-2 truncate max-w-[100px]">{sym.description}</span>
                  )}
                </button>
              </li>
            ))
          )}
        </ul>
      </div>
    )}
  </div>

  {/* Timeframe pills */}
  <div className="flex items-center gap-0.5 bg-slate-100 rounded-lg p-0.5">
    {timeframes.map((tf) => (
      <button
        key={tf}
        onClick={() => setSelectedTimeframe(tf)}
        className={cn(
          'px-2.5 py-1 rounded-md text-xs font-medium transition-all duration-150',
          selectedTimeframe === tf
            ? 'bg-white text-slate-900 shadow-sm'
            : 'text-slate-500 hover:text-slate-700',
        )}
      >
        {tf}
      </button>
    ))}
  </div>

  {/* Indicators button */}
  <button
    onClick={() => setIndicatorModalOpen(true)}
    className="flex items-center gap-1.5 h-8 px-3 text-sm bg-white border border-slate-200 rounded-lg shadow-sm hover:border-slate-300 hover:shadow-md transition-all duration-150 text-slate-700"
  >
    <BarChart2 className="h-3.5 w-3.5 text-slate-400" />
    Indicators
  </button>

  {/* News toggle */}
  <button
    onClick={() => setNewsOpen((v) => !v)}
    className={cn(
      'flex items-center gap-1.5 h-8 px-3 text-sm rounded-lg border transition-all duration-150 shadow-sm',
      newsOpen
        ? 'bg-primary text-white border-primary'
        : 'bg-white border-slate-200 text-slate-700 hover:border-slate-300 hover:shadow-md',
    )}
    aria-pressed={newsOpen}
  >
    <Newspaper className="h-3.5 w-3.5" />
    News
  </button>

  {/* Indicator chips */}
  {activeIndicators.length > 0 && (
    <IndicatorChips
      indicators={activeIndicators}
      onRemove={handleRemoveIndicator}
      onEdit={() => setIndicatorModalOpen(true)}
    />
  )}

  {/* Refresh */}
  <button
    onClick={() => refetch()}
    disabled={isFetching}
    className="ml-auto flex items-center gap-1.5 h-8 px-3 text-sm bg-white border border-slate-200 rounded-lg shadow-sm hover:border-slate-300 hover:shadow-md transition-all duration-150 disabled:opacity-50 text-slate-700"
  >
    <RefreshCw className={cn('h-3.5 w-3.5 text-slate-400', isFetching && 'animate-spin')} />
    Refresh
  </button>
</div>
```

Also update the outer container to use consistent padding: change `<div className="flex flex-col h-[calc(100vh-4rem)] gap-4 p-4">` to:

```tsx
<div className="flex flex-col h-full gap-3 px-4 py-3">
```

And update the chart card: `<div className="flex-1 bg-card border border-border rounded-lg overflow-hidden min-h-0">` → `<div className="flex-1 bg-white border border-slate-100 rounded-2xl shadow-sm overflow-hidden min-h-0">`

**Step 2: Commit**

```bash
git add frontend/src/pages/Chart.tsx
git commit -m "style: update Chart page toolbar to new design tokens"
```

---

## Task 13: Add Page Wrapper to Other Pages

**Files:**
- Modify: `frontend/src/components/layout/Layout.tsx`

**Context:** All non-Dashboard pages currently render inside `<main className="flex-1 overflow-hidden">`. We need to give them a proper scrollable wrapper with consistent padding. Dashboard needs no padding (it manages its own layout). All other pages use a standard `overflow-y-auto` + max-width wrapper.

**Step 1: Update `Layout.tsx` to use `useLocation` for conditional wrapper**

```tsx
import { useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { CommandBar } from './CommandBar'
import { useNotificationWS } from '@/hooks/useNotifications'
import { SignalToast } from '@/components/SignalToast'
import { AIButton } from '@/components/ai/AIButton'
import { ChatPanel } from '@/components/ai/ChatPanel'
import { cn } from '@/lib/utils'

export function Layout() {
  const [isChatOpen, setIsChatOpen] = useState(false)
  const { pathname } = useLocation()
  useNotificationWS()

  const isDashboard = pathname === '/'

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <CommandBar />
      <main className={cn('flex-1 overflow-hidden', !isDashboard && 'overflow-y-auto')}>
        {isDashboard ? (
          <Outlet />
        ) : (
          <div className="max-w-7xl mx-auto px-6 py-6 animate-fade-in">
            <Outlet />
          </div>
        )}
      </main>
      <SignalToast />
      <AIButton onClick={() => setIsChatOpen((o) => !o)} isOpen={isChatOpen} />
      <ChatPanel isOpen={isChatOpen} onClose={() => setIsChatOpen(false)} />
    </div>
  )
}
```

**Step 2: Remove page-level `<h1>` headings that are redundant**

The pages `Portfolio`, `Backtest`, `Monitor`, `Alerts`, `News`, `Settings` each have an `<h1 className="text-2xl font-bold ...">` at the top. These still make sense — update the class to match the new design system: `text-xl font-semibold text-slate-900 tracking-tight`.

Do a search-and-replace across pages:

```
old: text-2xl font-bold
new: text-xl font-semibold text-slate-900 tracking-tight
```

Files to update: `Portfolio.tsx`, `Backtest.tsx`, `Monitor.tsx`, `Alerts.tsx`, `News.tsx`, `Settings.tsx`, `Notifications.tsx`

**Step 3: Commit**

```bash
git add frontend/src/components/layout/Layout.tsx \
        frontend/src/pages/Portfolio.tsx \
        frontend/src/pages/Backtest.tsx \
        frontend/src/pages/Monitor.tsx \
        frontend/src/pages/Alerts.tsx \
        frontend/src/pages/News.tsx \
        frontend/src/pages/Settings.tsx \
        frontend/src/pages/Notifications.tsx
git commit -m "style: add consistent page wrapper, update heading styles across pages"
```

---

## Task 14: Update Default Theme to Light

**Files:**
- Modify: `frontend/src/stores/index.ts`

**Context:** The theme store currently defaults to `'dark'`. Since our primary design is light/neutral pro, we change the default to `'light'`. The user can still toggle to dark.

**Step 1: Change the default theme in `useThemeStore`** (line 33)

```ts
// Change:
theme: 'dark',
// To:
theme: 'light',
```

**Step 2: Update the `useEffect` in `frontend/src/main.tsx`** (if it exists) or confirm the `setTheme` initialization on app load correctly applies the `dark` class. The store's `persist` middleware will restore the user's preference from localStorage — new users will start with light.

**Step 3: Commit**

```bash
git add frontend/src/stores/index.ts
git commit -m "style: change default theme to light mode"
```

---

## Task 15: Remove Old Layout Files

**Files:**
- Delete: `frontend/src/components/layout/Sidebar.tsx`
- Delete: `frontend/src/components/layout/TopBar.tsx`

**Context:** Both files are now replaced by `CommandBar.tsx`. Confirm no other file imports them before deleting.

**Step 1: Verify no remaining imports**

```bash
grep -r "from.*Sidebar" frontend/src/
grep -r "from.*TopBar" frontend/src/
```

Expected: no matches (only `CommandBar` imports from layout).

**Step 2: Delete the files**

```bash
rm frontend/src/components/layout/Sidebar.tsx
rm frontend/src/components/layout/TopBar.tsx
```

**Step 3: Verify the build compiles**

```bash
make frontend-shell
# Inside container:
npm run build
exit
```

Expected: no TypeScript errors.

**Step 4: Commit**

```bash
git add -u frontend/src/components/layout/
git commit -m "chore: remove Sidebar.tsx and TopBar.tsx (replaced by CommandBar)"
```

---

## Task 16: Final Visual Verification Checklist

Start the full dev environment and do a complete walkthrough:

```bash
make up
make health
```

Open `http://localhost:5173` and verify:

**Command Bar:**
- [ ] 52px tall, white background, subtle shadow
- [ ] Logo + wordmark on left
- [ ] Symbol picker opens dropdown with search
- [ ] Timeframe pills — active one is white/raised, inactive is flat slate
- [ ] Nav tabs — active tab has blue-800 underline + blue-50 background
- [ ] Notification bell shows dropdown with correct notifications
- [ ] Theme toggle switches light ↔ dark correctly
- [ ] User avatar opens logout dropdown

**Dashboard (/):**
- [ ] 3-column layout: watchlist (left 220px) + chart (center) + news+alerts (right 280px)
- [ ] Watchlist shows default symbols
- [ ] Chart shows empty state with "Select a symbol" message
- [ ] Select a symbol → chart loads
- [ ] Portfolio panel shows "No portfolio selected" when none is active
- [ ] News panel shows news items (or empty state)
- [ ] Alerts panel shows alerts (or empty state)

**Chart page (/chart):**
- [ ] Full-width below command bar
- [ ] Toolbar uses new pill styles + rounded controls
- [ ] Chart area has `rounded-2xl` card

**Other pages (Portfolio, Backtest, etc.):**
- [ ] Render within `max-w-7xl` centered wrapper
- [ ] Headings use `text-xl font-semibold text-slate-900`
- [ ] No left sidebar present

**Commit:**

```bash
git add -A
git commit -m "chore: final cleanup after UI redesign"
```

---

## Summary

| Task | Component | Key Change |
|---|---|---|
| 1 | `index.css` | New CSS variables, flash keyframes |
| 2 | `tailwind.config.js` | flash-up/down animations |
| 3 | `button.tsx` | rounded-lg, active:scale-95 |
| 4 | `select.tsx` | shadcn Select install |
| 5 | `CommandBar.tsx` | New — replaces Sidebar + TopBar |
| 6 | `Layout.tsx` | Remove Sidebar/TopBar, add CommandBar |
| 7 | `WatchlistPanel.tsx` | New — live ticks with flash |
| 8 | `PortfolioSummaryPanel.tsx` | New — compact stat cards |
| 9 | `NewsFeedPanel.tsx` | New — symbol-filtered news |
| 10 | `AlertsFeedPanel.tsx` | New — active alerts feed |
| 11 | `Dashboard.tsx` | Rebuilt — 3-column workstation |
| 12 | `Chart.tsx` | Toolbar restyled |
| 13 | `Layout.tsx` | Dashboard vs page wrappers |
| 14 | `stores/index.ts` | Default theme → light |
| 15 | `Sidebar.tsx`, `TopBar.tsx` | Deleted |
| 16 | — | Final visual QA |
