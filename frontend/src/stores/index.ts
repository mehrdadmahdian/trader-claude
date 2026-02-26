import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type {
  Alert,
  Backtest,
  Notification,
  Portfolio,
  Position,
  PortfolioSummary,
  PortfolioUpdateMsg,
  ReplayCandle,
  ReplayEquityPoint,
  ReplayServerMsg,
  ReplayState,
  ReplayStatus,
  Symbol,
  Tick,
  Trade,
} from '@/types'

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

  // Replay state
  replayActive: boolean
  replayId: string | null
  replayState: ReplayStatus
  replayIndex: number
  replayTotal: number
  replaySpeed: number
  replayCandles: ReplayCandle[]
  replayEquity: ReplayEquityPoint[]
  replayTrades: Trade[]
  replayOpen: boolean

  setReplayActive: (active: boolean, replayId?: string) => void
  setReplayOpen: (open: boolean) => void
  applyReplayMsg: (msg: ReplayServerMsg) => void
  resetReplay: () => void
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

  // Replay initial state
  replayActive: false,
  replayId: null,
  replayState: 'idle',
  replayIndex: 0,
  replayTotal: 0,
  replaySpeed: 1,
  replayCandles: [],
  replayEquity: [],
  replayTrades: [],
  replayOpen: false,

  setReplayActive: (active, replayId) =>
    set((s) => ({
      replayActive: active,
      replayId: replayId ?? s.replayId,
      replayCandles: active ? s.replayCandles : [],
      replayEquity: active ? s.replayEquity : [],
      replayTrades: active ? s.replayTrades : [],
      replayState: active ? 'idle' : 'idle',
      replayIndex: active ? 0 : 0,
    })),

  setReplayOpen: (replayOpen) => set({ replayOpen }),

  applyReplayMsg: (msg) =>
    set((s) => {
      switch (msg.type) {
        case 'status': {
          const d = msg.data as ReplayState
          return { replayState: d.state, replayIndex: d.index, replayTotal: d.total, replaySpeed: d.speed }
        }
        case 'candle': {
          const candle = msg.data as ReplayCandle
          return { replayCandles: [...s.replayCandles, candle] }
        }
        case 'equity_update': {
          const eq = msg.data as ReplayEquityPoint
          return { replayEquity: [...s.replayEquity, eq] }
        }
        case 'trade_open':
        case 'trade_close': {
          const ev = msg.data as { trade: Trade }
          const exists = s.replayTrades.some((t) => t.id === ev.trade.id)
          return exists
            ? { replayTrades: s.replayTrades.map((t) => (t.id === ev.trade.id ? ev.trade : t)) }
            : { replayTrades: [...s.replayTrades, ev.trade] }
        }
        case 'seek_snapshot': {
          const snap = msg.data as { candles: ReplayCandle[]; equity: ReplayEquityPoint[]; trades: Trade[] }
          return { replayCandles: snap.candles, replayEquity: snap.equity, replayTrades: snap.trades }
        }
        default:
          return {}
      }
    }),

  resetReplay: () =>
    set({
      replayActive: false,
      replayId: null,
      replayState: 'idle',
      replayIndex: 0,
      replayTotal: 0,
      replaySpeed: 1,
      replayCandles: [],
      replayEquity: [],
      replayTrades: [],
      replayOpen: false,
    }),
}))

// ── Portfolio store ─────────────────────────────────────────────────────────

interface PortfolioStore {
  portfolios: Portfolio[]
  activePortfolioId: number | null
  positions: Position[]
  summary: PortfolioSummary | null
  setPortfolios: (portfolios: Portfolio[]) => void
  setActivePortfolioId: (id: number | null) => void
  setPositions: (positions: Position[]) => void
  setSummary: (summary: PortfolioSummary | null) => void
  applyLiveUpdate: (msg: PortfolioUpdateMsg) => void
}

export const usePortfolioStore = create<PortfolioStore>()((set) => ({
  portfolios: [],
  activePortfolioId: null,
  positions: [],
  summary: null,
  setPortfolios: (portfolios) => set({ portfolios }),
  setActivePortfolioId: (id) => set({ activePortfolioId: id }),
  setPositions: (positions) => set({ positions }),
  setSummary: (summary) => set({ summary }),
  applyLiveUpdate: (msg) =>
    set((state) => ({
      summary: state.summary
        ? {
            ...state.summary,
            total_value: msg.total_value,
            total_pnl: msg.total_pnl,
            total_pnl_pct: msg.total_pnl_pct,
          }
        : null,
      positions: state.positions.map((pos) => {
        const update = msg.positions.find((p) => p.id === pos.id)
        if (!update) return pos
        return {
          ...pos,
          current_price: update.current_price,
          unrealized_pnl: update.unrealized_pnl,
          unrealized_pnl_pct: update.unrealized_pnl_pct,
          current_value: pos.quantity * update.current_price,
        }
      }),
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
