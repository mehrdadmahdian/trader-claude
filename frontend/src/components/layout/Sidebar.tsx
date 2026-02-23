import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  CandlestickChart,
  FlaskConical,
  Briefcase,
  Activity,
  Newspaper,
  Bell,
  Settings,
  ChevronLeft,
  ChevronRight,
  TrendingUp,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useSidebarStore } from '@/stores'

const navItems = [
  { to: '/',           icon: LayoutDashboard, label: 'Dashboard'    },
  { to: '/chart',      icon: CandlestickChart, label: 'Chart'       },
  { to: '/backtest',   icon: FlaskConical,     label: 'Backtest'    },
  { to: '/portfolio',  icon: Briefcase,        label: 'Portfolio'   },
  { to: '/monitor',    icon: Activity,         label: 'Monitor'     },
  { to: '/news',       icon: Newspaper,        label: 'News'        },
  { to: '/alerts',     icon: Bell,             label: 'Alerts'      },
  { to: '/settings',   icon: Settings,         label: 'Settings'    },
]

export function Sidebar() {
  const { collapsed, toggle } = useSidebarStore()

  return (
    <aside
      className={cn(
        'flex flex-col h-screen bg-card border-r border-border transition-all duration-300 shrink-0',
        collapsed ? 'w-16' : 'w-56',
      )}
    >
      {/* Logo */}
      <div className={cn('flex items-center gap-2 px-4 h-16 border-b border-border', collapsed && 'justify-center px-0')}>
        <TrendingUp className="w-6 h-6 text-primary shrink-0" />
        {!collapsed && (
          <span className="font-semibold text-sm truncate">Trader Claude</span>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 py-4 space-y-1 overflow-y-auto overflow-x-hidden">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 mx-2 px-3 py-2 rounded-md text-sm font-medium transition-colors',
                'hover:bg-accent hover:text-accent-foreground',
                isActive
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground',
                collapsed && 'justify-center px-0 mx-1',
              )
            }
            title={collapsed ? label : undefined}
          >
            <Icon className="w-4 h-4 shrink-0" />
            {!collapsed && <span className="truncate">{label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* Collapse toggle */}
      <button
        onClick={toggle}
        className={cn(
          'flex items-center justify-center h-10 border-t border-border',
          'text-muted-foreground hover:text-foreground hover:bg-accent transition-colors',
        )}
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
      >
        {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
      </button>
    </aside>
  )
}
