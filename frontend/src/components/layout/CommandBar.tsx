import { useState, useMemo, useRef, useEffect } from 'react'
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
import { useSymbols } from '@/hooks/useMarketData'
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

  const selectedSymbol    = useMarketStore((s) => s.selectedSymbol)
  const selectedMarket    = useMarketStore((s) => s.selectedMarket)
  const selectedTimeframe = useMarketStore((s) => s.selectedTimeframe)
  const setSelectedSymbol    = useMarketStore((s) => s.setSelectedSymbol)
  const setSelectedTimeframe = useMarketStore((s) => s.setSelectedTimeframe)

  const adapter = 'binance'
  const { data: symbols }  = useSymbols(adapter, selectedMarket)

  const [showSymbolSearch, setShowSymbolSearch] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const symbolPickerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!showSymbolSearch) return
    function handlePointerDown(e: PointerEvent) {
      if (symbolPickerRef.current && !symbolPickerRef.current.contains(e.target as Node)) {
        setShowSymbolSearch(false)
        setSearchQuery('')
      }
    }
    document.addEventListener('pointerdown', handlePointerDown)
    return () => document.removeEventListener('pointerdown', handlePointerDown)
  }, [showSymbolSearch])

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
      <div className="relative mr-1" ref={symbolPickerRef}>
        <button
          onClick={() => setShowSymbolSearch((v) => !v)}
          className="flex items-center gap-1.5 h-8 px-3 rounded-lg border border-slate-200 bg-white text-sm shadow-sm hover:border-slate-300 hover:shadow-md transition-all duration-150 min-w-[130px]"
          aria-expanded={showSymbolSearch}
          aria-haspopup="listbox"
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
                onKeyDown={(e) => {
                  if (e.key === 'Escape') {
                    setShowSymbolSearch(false)
                    setSearchQuery('')
                  }
                }}
              />
            </div>
            <ul className="max-h-60 overflow-y-auto" role="listbox">
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
              className="flex justify-center items-center px-4 py-2.5 text-xs text-primary hover:text-primary/80 hover:bg-slate-50 transition-colors cursor-pointer outline-none"
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
