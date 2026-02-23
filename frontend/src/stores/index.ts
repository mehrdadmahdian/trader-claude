import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Alert, Backtest, Notification, Portfolio, Symbol, Tick } from '@/types'

// ── Theme store ────────────────────────────────────────────────────────────

interface ThemeStore {
  theme: 'light' | 'dark'
  toggleTheme: () => void
  setTheme: (theme: 'light' | 'dark') => void
}

export const useThemeStore = create<ThemeStore>()(
  persist(
    (set) => ({
      theme: 'dark',
      toggleTheme: () =>
        set((state) => {
          const next = state.theme === 'dark' ? 'light' : 'dark'
          document.documentElement.classList.toggle('dark', next === 'dark')
          return { theme: next }
        }),
      setTheme: (theme) => {
        document.documentElement.classList.toggle('dark', theme === 'dark')
        set({ theme })
      },
    }),
    { name: 'trader-theme' },
  ),
)

// ── Sidebar store ──────────────────────────────────────────────────────────

interface SidebarStore {
  collapsed: boolean
  toggle: () => void
  setCollapsed: (v: boolean) => void
}

export const useSidebarStore = create<SidebarStore>()(
  persist(
    (set) => ({
      collapsed: false,
      toggle: () => set((s) => ({ collapsed: !s.collapsed })),
      setCollapsed: (v) => set({ collapsed: v }),
    }),
    { name: 'trader-sidebar' },
  ),
)

// ── Market data store ──────────────────────────────────────────────────────

interface MarketStore {
  ticks: Record<string, Tick>     // keyed by "symbol:market"
  symbols: Symbol[]
  selectedSymbol: string | null
  selectedMarket: string
  selectedTimeframe: string
  updateTick: (tick: Tick) => void
  setSymbols: (symbols: Symbol[]) => void
  setSelectedSymbol: (symbol: string | null) => void
  setSelectedMarket: (market: string) => void
  setSelectedTimeframe: (tf: string) => void
}

export const useMarketStore = create<MarketStore>()((set) => ({
  ticks: {},
  symbols: [],
  selectedSymbol: null,
  selectedMarket: 'crypto',
  selectedTimeframe: '1h',
  updateTick: (tick) =>
    set((s) => ({
      ticks: { ...s.ticks, [`${tick.symbol}:${tick.market}`]: tick },
    })),
  setSymbols: (symbols) => set({ symbols }),
  setSelectedSymbol: (symbol) => set({ selectedSymbol: symbol }),
  setSelectedMarket: (market) => set({ selectedMarket: market }),
  setSelectedTimeframe: (tf) => set({ selectedTimeframe: tf }),
}))

// ── Backtest store ─────────────────────────────────────────────────────────

interface BacktestStore {
  backtests: Backtest[]
  activeBacktest: Backtest | null
  setBacktests: (b: Backtest[]) => void
  setActiveBacktest: (b: Backtest | null) => void
  updateBacktest: (b: Backtest) => void
}

export const useBacktestStore = create<BacktestStore>()((set) => ({
  backtests: [],
  activeBacktest: null,
  setBacktests: (backtests) => set({ backtests }),
  setActiveBacktest: (activeBacktest) => set({ activeBacktest }),
  updateBacktest: (b) =>
    set((s) => ({
      backtests: s.backtests.map((x) => (x.id === b.id ? b : x)),
      activeBacktest: s.activeBacktest?.id === b.id ? b : s.activeBacktest,
    })),
}))

// ── Portfolio store ────────────────────────────────────────────────────────

interface PortfolioStore {
  portfolios: Portfolio[]
  activePortfolio: Portfolio | null
  setPortfolios: (p: Portfolio[]) => void
  setActivePortfolio: (p: Portfolio | null) => void
  updatePortfolio: (p: Portfolio) => void
}

export const usePortfolioStore = create<PortfolioStore>()((set) => ({
  portfolios: [],
  activePortfolio: null,
  setPortfolios: (portfolios) => set({ portfolios }),
  setActivePortfolio: (activePortfolio) => set({ activePortfolio }),
  updatePortfolio: (p) =>
    set((s) => ({
      portfolios: s.portfolios.map((x) => (x.id === p.id ? p : x)),
      activePortfolio: s.activePortfolio?.id === p.id ? p : s.activePortfolio,
    })),
}))

// ── Alert store ────────────────────────────────────────────────────────────

interface AlertStore {
  alerts: Alert[]
  setAlerts: (a: Alert[]) => void
  addAlert: (a: Alert) => void
  removeAlert: (id: number) => void
}

export const useAlertStore = create<AlertStore>()((set) => ({
  alerts: [],
  setAlerts: (alerts) => set({ alerts }),
  addAlert: (a) => set((s) => ({ alerts: [a, ...s.alerts] })),
  removeAlert: (id) => set((s) => ({ alerts: s.alerts.filter((x) => x.id !== id) })),
}))

// ── Notification store ─────────────────────────────────────────────────────

interface NotificationStore {
  notifications: Notification[]
  unreadCount: number
  setNotifications: (n: Notification[]) => void
  addNotification: (n: Notification) => void
  markRead: (id: number) => void
  markAllRead: () => void
}

export const useNotificationStore = create<NotificationStore>()((set) => ({
  notifications: [],
  unreadCount: 0,
  setNotifications: (notifications) =>
    set({
      notifications,
      unreadCount: notifications.filter((n) => !n.read).length,
    }),
  addNotification: (n) =>
    set((s) => ({
      notifications: [n, ...s.notifications],
      unreadCount: s.unreadCount + (n.read ? 0 : 1),
    })),
  markRead: (id) =>
    set((s) => ({
      notifications: s.notifications.map((n) => (n.id === id ? { ...n, read: true } : n)),
      unreadCount: Math.max(0, s.unreadCount - 1),
    })),
  markAllRead: () =>
    set((s) => ({
      notifications: s.notifications.map((n) => ({ ...n, read: true })),
      unreadCount: 0,
    })),
}))
