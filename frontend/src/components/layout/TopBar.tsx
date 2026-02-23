import { Menu, Moon, Sun, Bell } from 'lucide-react'
import { useThemeStore, useNotificationStore, useSidebarStore } from '@/stores'
import { cn } from '@/lib/utils'

export function TopBar() {
  const { theme, toggleTheme } = useThemeStore()
  const { unreadCount } = useNotificationStore()
  const { toggle } = useSidebarStore()

  return (
    <header className="h-16 border-b border-border bg-card flex items-center gap-4 px-4 shrink-0">
      {/* Mobile hamburger */}
      <button
        onClick={toggle}
        className="lg:hidden p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label="Toggle sidebar"
      >
        <Menu className="w-5 h-5" />
      </button>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      >
        {theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
      </button>

      {/* Notification bell */}
      <button
        className="relative p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        aria-label="Notifications"
      >
        <Bell className="w-5 h-5" />
        {unreadCount > 0 && (
          <span
            className={cn(
              'absolute top-1 right-1 min-w-[1rem] h-4 px-0.5',
              'flex items-center justify-center',
              'text-[10px] font-bold text-white bg-destructive rounded-full',
            )}
          >
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>
    </header>
  )
}
